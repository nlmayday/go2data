package model

type User struct {
	ID    uint   `gorm:"primaryKey"`
	Name  string `gorm:"type:varchar(100)"`
	Phone string `gorm:"type:varchar(20)"`
	Addr  string `gorm:"type:varchar(255)"`
}
