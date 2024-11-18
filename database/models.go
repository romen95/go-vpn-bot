package database

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID         uint    `gorm:"primaryKey"`
	TelegramID int64   `gorm:"uniqueIndex"`
	Username   string  `gorm:"size:100"`
	Balance    float64 `gorm:"default:0"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func Migrate(db *gorm.DB) {
	db.AutoMigrate(&User{})
}
