package db

import (
	"os"
	"path/filepath"

	"l4d2-manager-next/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func OpenAuthDB(path string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	database, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	connection, err := database.DB()
	if err != nil {
		return nil, err
	}
	connection.SetMaxOpenConns(1)
	for _, statement := range []string{"PRAGMA journal_mode = WAL;", "PRAGMA busy_timeout = 5000;"} {
		if err := database.Exec(statement).Error; err != nil {
			connection.Close()
			return nil, err
		}
	}
	if err := database.AutoMigrate(&model.AuthCode{}, &model.AuthCodeState{}); err != nil {
		connection.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0600); err != nil {
		connection.Close()
		return nil, err
	}
	return database, nil
}
