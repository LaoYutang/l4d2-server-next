package controller

import (
	"fmt"
	"l4d2-manager-next/consts"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var mutex sync.RWMutex

func List(c *gin.Context) {
	mutex.RLock()
	defer mutex.RUnlock()

	file, err := os.ReadFile(consts.MapListFilePath)
	if err != nil {
		c.String(http.StatusInternalServerError, "读取地图列表失败")
		return
	}

	// 获取vpk文件的大小
	mapList := strings.TrimSpace(string(file))
	maps := strings.Split(mapList, "\n")
	var result strings.Builder

	for _, mapName := range maps {
		if mapName == "" {
			continue
		}

		// 构建vpk文件路径
		vpkPath := filepath.Join(consts.BasePath, mapName)

		// 获取文件大小
		fileInfo, err := os.Stat(vpkPath)
		if err != nil {
			result.WriteString(fmt.Sprintf("%s$$unknown\n", mapName))
		} else {
			sizeStr := formatFileSize(fileInfo.Size())
			result.WriteString(fmt.Sprintf("%s$$%s\n", mapName, sizeStr))
		}
	}

	c.String(http.StatusOK, result.String())
}

// 格式化文件大小
func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2fGB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2fMB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2fKB", float64(size)/KB)
	default:
		return fmt.Sprintf("%dB", size)
	}
}
