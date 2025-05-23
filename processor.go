package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"go2data/config"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type DataProcessor struct {
	cfg        config.Config
	db         *gorm.DB
	logger     *log.Logger
	currentLog string // 新增字段，记录当前日志文件名

	tableCounters  sync.Map
	tableNames     []string
	tableIndex     int
	tableIndexLock sync.Mutex
}

type ProcessState struct {
	CurrentLine int
	Filename    string
	mu          sync.Mutex
}

func (p *DataProcessor) processFile(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}

	logFile := fmt.Sprintf("logs/%s_%s.log", info.Name(), time.Now().Format("20060102150405"))
	f, err := os.Create(logFile)
	if err != nil {
		return err
	}
	p.logger = log.New(f, "", log.LstdFlags)
	p.currentLog = logFile

	state := &ProcessState{
		Filename:    info.Name(),
		CurrentLine: 0,
	}

	err = fmt.Errorf("no processor found for file type")
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".csv":
		state.CurrentLine = p.cfg.Task.CsvBeginLine
		fmt.Println(state.Filename, state.CurrentLine)
		err = p.processCSV(path, state)
	case ".xlsx":
		state.CurrentLine = p.cfg.Task.XlsxBeginLine
		err = p.processXLSX(path, state)
	case ".txt":
		state.CurrentLine = p.cfg.Task.TxtBeginLine
		err = p.processTXT(path, state)
	default:
		log.Println(err)
		return nil
	}
	if err != nil {
		return err
	}

	// return nil
	// // 移动文件到已处理目录 backup
	// backupDir := "backup/"
	// _ = os.MkdirAll(backupDir, 0755)
	// dest := filepath.Join(backupDir, filepath.Base(path))
	// return os.Rename(path, dest)

	// 删除文件
	return os.Remove(path)
}

func (p *DataProcessor) processCSV(path string, state *ProcessState) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// reader.Comma = rune(p.cfg.Task.Delimiter[0])

	return p.processFileWithReader(reader, state)
}

func (p *DataProcessor) processTXT(path string, state *ProcessState) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	return p.processFileWithReader(&TextReader{scanner: scanner, delimiter: p.cfg.Task.Delimiter}, state)
}

func (p *DataProcessor) processXLSX(path string, state *ProcessState) error {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return err
	}
	defer f.Close()

	rows, err := f.Rows("Sheet1")
	if err != nil {
		return err
	}
	return p.processFileWithReader(&XLSXReader{rows: rows}, state)
}

type TextReader struct {
	scanner   *bufio.Scanner
	delimiter string
}

func (r *TextReader) Read() ([]string, error) {
	if !r.scanner.Scan() {
		return nil, io.EOF
	}
	return strings.Split(r.scanner.Text(), r.delimiter), nil
}

type XLSXReader struct {
	rows *excelize.Rows
}

func (r *XLSXReader) Read() ([]string, error) {
	if !r.rows.Next() {
		return nil, io.EOF
	}
	return r.rows.Columns()
}

func (p *DataProcessor) processFileWithReader(reader interface{ Read() ([]string, error) }, state *ProcessState) error {
	lastLine, _ := p.getLastProcessedLine(state.Filename)
	if lastLine > state.CurrentLine {
		state.CurrentLine = lastLine
	}

	workerCount := 4 // 单表模式下使用4个worker
	if p.cfg.Task.MultipleTable {
		workerCount = 10
	}

	workerChan := make(chan []map[string]string, workerCount)
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		tableName := p.cfg.Task.TableName
		// if p.cfg.Task.MultipleTable {
		// 	tableName = fmt.Sprintf("%s_%d", p.cfg.Task.TableName, i)
		// }
		go p.processWorker(tableName, workerChan, &wg)
	}

	err := p.readAndBatch(reader, state, workerChan)
	close(workerChan)
	wg.Wait()
	return err
}

func (p *DataProcessor) readAndBatch(reader interface{ Read() ([]string, error) }, state *ProcessState, workerChan chan<- []map[string]string) error {
	batch := make([]map[string]string, 0, p.cfg.Task.BatchSize)

	// 如果有上次处理的位置，跳过已处理的行
	currentLine := 0
	for currentLine < state.CurrentLine {
		_, err := reader.Read()
		if err == io.EOF {
			return nil // 文件已全部处理过
		}
		// if err != nil && err != csv.ParseError {
		// 	return err
		// }
		currentLine++
	}

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil && len(row) == 0 {
			return err
		}
		state.mu.Lock()
		state.CurrentLine++
		state.mu.Unlock()

		record := make(map[string]string)
		for i, colIdx := range p.cfg.Task.DataColumn {
			if colIdx < len(row) {
				record[p.cfg.Task.Columns[i]] = row[colIdx]
			} else {
				p.logger.Printf("Index out of range: colIdx=%d, row length=%d", colIdx, len(row))
			}
		}

		isAllempty := true
		for _, value := range record {
			if value != "" {
				isAllempty = false
				break
			}
		}
		if isAllempty {
			continue
		}

		batch = append(batch, record)

		if len(batch) >= p.cfg.Task.BatchSize {
			workerChan <- batch
			state.mu.Lock()
			p.logger.Printf("Processed %d lines of %s", state.CurrentLine, state.Filename)
			p.updateProgress(state.Filename, state.CurrentLine)
			state.mu.Unlock()
			batch = make([]map[string]string, 0, p.cfg.Task.BatchSize)
		}
	}
	if len(batch) > 0 {
		workerChan <- batch
		state.mu.Lock()
		p.logger.Printf("Processed %d lines of %s", state.CurrentLine, state.Filename)
		p.updateProgress(state.Filename, state.CurrentLine)
		state.mu.Unlock()
	}
	return nil
}

func (p *DataProcessor) processWorker(tableName string, recordsChan <-chan []map[string]string, wg *sync.WaitGroup) {
	defer wg.Done()

	// 如果是多表模式，动态创建表
	// if p.cfg.Task.MultipleTable {
	// 	p.db.Table(tableName).AutoMigrate(&model.User{})
	// }
	for records := range recordsChan {
		tableName = p.getCurrentTableName(tableName, p.cfg.Task.BatchSize)
		if err := p.batchInsert(records, tableName); err != nil {
			p.logger.Printf("Error inserting batch to %s: %v", tableName, err)
		}
	}
}

func (p *DataProcessor) batchInsert(records []map[string]string, tableName string) error {
	// return p.db.Transaction(func(tx *gorm.DB) error {
	// 	var users []model.User
	// 	for _, r := range records {
	// 		users = append(users, model.User{
	// 			Name:  r["name"],
	// 			Phone: r["phone"],
	// 			Addr:  r["addr"],
	// 		})
	// 	}
	// 	// users := make([]map[string]string, 0)
	// 	// users = append(users, records...)
	// 	return tx.Table(tableName).Create(&users).Error
	// })
	// 用原生SQL插入
	// 构建INSERT语句
	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES ", tableName, strings.Join(p.cfg.Task.Columns, ","))
	for _, record := range records {
		var values []string
		for _, val := range record {
			_ = val
			// 使用 ? 作为占位符，避免SQL注入
			values = append(values, "?")
		}
		sql += fmt.Sprintf("(%s),", strings.Join(values, ","))
	}
	// 去掉最后一个逗号
	sql = sql[:len(sql)-1]

	// 准备参数
	var params []interface{}
	for _, record := range records {
		// for _, val := range record {
		// 	// params = append(params, val)
		// }
		for _, val := range p.cfg.Task.Columns {
			params = append(params, record[val])
		}
	}

	// 执行INSERT语句
	result := p.db.Exec(sql, params...)
	if result.Error != nil {
		log.Println(result.Error.Error())
		return result.Error
	}

	return nil
}

func (p *DataProcessor) getLastProcessedLine(filename string) (int, error) {
	logFile := p.findLatestLog(filename)
	if logFile == "" {
		return 0, nil
	}

	f, err := os.Open(logFile)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lastLine := 0
	for scanner.Scan() {
		line := scanner.Text()
		// 查找 "Processing line" 后的数字
		parts := strings.Split(line, "Processing line ")
		if len(parts) > 1 {
			numParts := strings.Split(parts[1], " of ")
			if len(numParts) > 0 {
				if num, err := strconv.Atoi(strings.TrimSpace(numParts[0])); err == nil {
					lastLine = num
				}
			}
		}
	}
	return lastLine, scanner.Err()
}

func (p *DataProcessor) updateProgress(filename string, line int) {
	p.logger.Printf("Processing line %d of %s", line, filename)
}

func (p *DataProcessor) findLatestLog(filename string) string {
	var latestFile string
	var latestTime time.Time

	err := filepath.Walk("logs", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 排除当前正在写入的日志文件
		if !info.IsDir() && strings.Contains(path, filename) && path != p.currentLog {
			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestFile = path
			}
		}
		return nil
	})

	if err != nil {
		return ""
	}
	return latestFile
}

func (p *DataProcessor) getCurrentTableName(baseTableName string, batchSize int) string {
	if !p.cfg.Task.MultipleTable {
		return baseTableName
	}

	p.tableIndexLock.Lock()
	defer p.tableIndexLock.Unlock()

	// 获取当前表的记录数
	count, _ := p.tableCounters.LoadOrStore(baseTableName+"_"+strconv.Itoa(p.tableIndex), int64(0))
	currentCount := count.(int64)

	// 检查是否需要切换到新表
	if currentCount+int64(batchSize) > p.cfg.Task.TableSize {
		p.tableIndex++
		if p.tableIndex >= len(p.cfg.Task.TableNames) {
			p.tableIndex = len(p.cfg.Task.TableNames) - 1
		}
		p.tableCounters.Store(baseTableName+"_"+strconv.Itoa(p.tableIndex), int64(0))
		p.logger.Printf("Switching to new table: %s_%d", baseTableName, p.tableIndex)
	}
	if p.tableIndex < 0 {
		p.logger.Printf("Table index is less than 0: %d", p.tableIndex)
	}

	// 更新计数器
	p.tableCounters.Store(baseTableName+"_"+strconv.Itoa(p.tableIndex), currentCount+int64(batchSize))

	// 返回当前表名
	// return fmt.Sprintf("%s_%d", baseTableName, p.tableIndex)
	return p.cfg.Task.TableNames[p.tableIndex]
}
