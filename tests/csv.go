package main

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"time"
)

func main() {
	// 配置参数
	rowCount := 10000 * 10 // 生成的行数，例如 100万行
	// colCount := 3       // 列数，与你的 user 表匹配 (name, phone, addr)
	fileName := "data/test_data10.csv"

	// 创建文件
	file, err := os.Create(fileName)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	// 创建 CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 生成表头
	headers := []string{"name", "phone", "addr"}
	if err := writer.Write(headers); err != nil {
		fmt.Println("Error writing headers:", err)
		return
	}

	// 随机种子
	rand.Seed(time.Now().UnixNano())

	// 生成随机数据
	for i := 0; i < rowCount; i++ {
		record := generateRandomRecord(i)
		if err := writer.Write(record); err != nil {
			fmt.Println("Error writing record:", err)
			return
		}

		// 每10万行打印一次进度
		if (i+1)%100000 == 0 {
			fmt.Printf("Generated %d rows\n", i+1)
		}
	}

	fmt.Printf("Generated CSV file with %d rows at %s\n", rowCount, fileName)
}

// 生成一条随机记录
func generateRandomRecord(index int) []string {
	name := fmt.Sprintf("user_%d", index+1)
	phone := generateRandomPhone()
	addr := generateRandomAddr()

	return []string{name, phone, addr}
}

// 生成随机手机号
func generateRandomPhone() string {
	prefixes := []string{"130", "131", "132", "133", "134", "135", "136", "137", "138", "139"}
	prefix := prefixes[rand.Intn(len(prefixes))]
	number := rand.Intn(100000000)
	return fmt.Sprintf("%s%08d", prefix, number)
}

// 生成随机地址
func generateRandomAddr() string {
	cities := []string{"Beijing", "Shanghai", "Guangzhou", "Shenzhen", "Hangzhou"}
	streets := []string{"Main St", "Park Ave", "Oak Rd", "Pine Ln", "Cedar Dr"}
	city := cities[rand.Intn(len(cities))]
	street := streets[rand.Intn(len(streets))]
	number := rand.Intn(1000) + 1
	return fmt.Sprintf("%d %s, %s", number, street, city)
}
