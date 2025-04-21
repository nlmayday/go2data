package main

import (
	"fmt"
	"go2data/config"
	"go2data/model"
	"log"
	"path/filepath"
	"sync"
	"time"
)

func main() {
	// 加载配置文件
	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// 初始化数据库
	if err := model.InitDB(cfg.DB); err != nil {
		log.Fatal("Failed to init db:", err)
	}

	processor := &DataProcessor{
		cfg:            cfg,
		db:             model.DB,
		tableIndex:     0,
		tableNames:     cfg.Task.TableNames,
		tableIndexLock: sync.Mutex{},
		tableCounters:  sync.Map{},
	}

	startAt := time.Now()

	// 处理data目录下所有文件
	err = filepath.Walk("data", processor.processFile)
	if err != nil {
		log.Fatal("Error processing files:", err)
	}
	endAt := time.Now()
	diff := endAt.Sub(startAt)
	fmt.Println("Processing completed.", diff)
}
