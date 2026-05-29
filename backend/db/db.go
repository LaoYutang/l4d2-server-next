package db

import (
	"l4d2-manager-next/consts"
	"l4d2-manager-next/model"
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB
var PlayerStatsDB *gorm.DB

func InitDB() {
	// 检查环境变量是否开启历史监控
	if os.Getenv("L4D2_HISTORY_METRICS") != "true" {
		return
	}

	var err error
	// 使用 glebarez/sqlite (纯 Go 驱动)
	if err = consts.EnsureManagerDataPath(); err != nil {
		log.Printf("Failed to create manager data directory: %v", err)
		return
	}
	DB, err = gorm.Open(sqlite.Open(consts.MonitorDBPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		return
	}

	// 开启 WAL 模式以提高并发写入性能
	DB.Exec("PRAGMA journal_mode = WAL;")

	// 自动迁移
	err = DB.AutoMigrate(&model.SystemMetric{})
	if err != nil {
		log.Printf("Failed to migrate database: %v", err)
		return
	}

	// 启动自动清理任务
	go startCleanupTask()
}

func InitPlayerStatsDB() {
	var err error
	if err = consts.EnsureManagerDataPath(); err != nil {
		log.Printf("Failed to create manager data directory: %v", err)
		return
	}

	PlayerStatsDB, err = gorm.Open(sqlite.Open(consts.PlayerStatsDBPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Printf("Failed to connect to player stats database: %v", err)
		return
	}

	PlayerStatsDB.Exec("PRAGMA journal_mode = WAL;")

	err = PlayerStatsDB.AutoMigrate(&model.PlayerStatSnapshot{}, &model.PlayerStatPlayer{})
	if err != nil {
		log.Printf("Failed to migrate player stats database: %v", err)
		PlayerStatsDB = nil
	}
}

func startCleanupTask() {
	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		if DB == nil {
			continue
		}
		// 保留3天数据: 3 * 24 * 3600 = 259200 秒
		expiration := time.Now().Unix() - 259200
		if err := DB.Where("timestamp < ?", expiration).Delete(&model.SystemMetric{}).Error; err != nil {
			log.Printf("Failed to cleanup old metrics: %v", err)
		}
	}
}
