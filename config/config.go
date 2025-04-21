package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	DB   DBConfig   `yaml:"db"`
	Task TaskConfig `yaml:"task"`
}

type DBConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

type TaskConfig struct {
	TableName     string   `yaml:"tableName"`
	Columns       []string `yaml:"columns"`
	DataColumn    []int    `yaml:"dataColumn"`
	BatchSize     int      `yaml:"batchSize"`
	Delimiter     string   `yaml:"delimiter"`
	TxtBeginLine  int      `yaml:"txtBeginLine"`
	CsvBeginLine  int      `yaml:"csvBeginLine"`
	XlsxBeginLine int      `yaml:"xlsxBeginLine"`
	MultipleTable bool     `yaml:"mulitipleTable"`
	TableSize     int64    `yaml:"tableSize"` // 一个表最大放多少
	TableNames    []string `yaml:"tableNames"`
}

func LoadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	err = yaml.Unmarshal(data, &cfg)
	return cfg, err
}
