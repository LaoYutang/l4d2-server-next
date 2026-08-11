package controller

import (
	"fmt"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type mapTrimResponse struct {
	Trimmed           bool   `json:"trimmed"`
	Message           string `json:"message"`
	OriginalSize      int64  `json:"original_size"`
	TrimmedSize       int64  `json:"trimmed_size"`
	SavedSize         int64  `json:"saved_size"`
	OriginalSizeLabel string `json:"original_size_label"`
	TrimmedSizeLabel  string `json:"trimmed_size_label"`
	SavedSizeLabel    string `json:"saved_size_label"`
}

func TrimMap(c *gin.Context) {
	mapName := cleanExistingMapName(c.PostForm("map"))
	defer LogOp(c, "手动精简地图文件: "+mapName)()

	if mapName == "" {
		FailWithError(c, http.StatusBadRequest, "地图名称不能为空")
		return
	}
	if strings.ToLower(filepath.Ext(mapName)) != ".vpk" {
		FailWithError(c, http.StatusBadRequest, "只能精简VPK地图文件")
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	mapList, err := readMapList()
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "读取maplist.txt失败: %v", err)
		return
	}

	exists := false
	for _, name := range mapList {
		if name == mapName {
			exists = true
			break
		}
	}
	if !exists {
		FailWithError(c, http.StatusBadRequest, "地图记录不存在")
		return
	}

	sourcePath := filepath.Join(consts.AddonsBasePath, mapName)
	originalInfo, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			FailWithError(c, http.StatusBadRequest, "地图文件不存在，无法精简")
		} else {
			FailWithError(c, http.StatusInternalServerError, "检查地图文件失败: %v", err)
		}
		return
	}
	originalSize := originalInfo.Size()

	trimmedPath, cleanup, err := logic.TrimVPKForServer(sourcePath)
	if err != nil {
		if logic.IsVPKTrimUnsupported(err) {
			c.JSON(http.StatusOK, newMapTrimResponse(false, "当前地图格式暂不支持精简，已保留原文件", originalSize, originalSize))
			return
		}
		FailWithError(c, http.StatusInternalServerError, "精简VPK失败: %v", err)
		return
	}
	defer cleanup()

	trimmedInfo, err := os.Stat(trimmedPath)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "读取精简VPK失败: %v", err)
		return
	}
	trimmedSize := trimmedInfo.Size()
	if trimmedSize >= originalSize {
		c.JSON(http.StatusOK, newMapTrimResponse(false, "精简后文件未变小，已保留原文件", originalSize, trimmedSize))
		return
	}

	if err := replaceMapWithTrimmedVPK(mapName, sourcePath, trimmedPath); err != nil {
		FailWithError(c, http.StatusInternalServerError, "替换精简VPK失败: %v", err)
		return
	}
	if err := logic.InspectAndStoreMapVPK(mapName, sourcePath); err != nil {
		log.Printf("重新检测精简后的地图 VPK 失败（%s）: %v", mapName, err)
	}

	savedSize := originalSize - trimmedSize
	c.JSON(http.StatusOK, newMapTrimResponse(true, fmt.Sprintf("精简成功，节省 %s", formatFileSize(savedSize)), originalSize, trimmedSize))
}

func newMapTrimResponse(trimmed bool, message string, originalSize, trimmedSize int64) mapTrimResponse {
	savedSize := originalSize - trimmedSize
	if savedSize < 0 {
		savedSize = 0
	}
	return mapTrimResponse{
		Trimmed:           trimmed,
		Message:           message,
		OriginalSize:      originalSize,
		TrimmedSize:       trimmedSize,
		SavedSize:         savedSize,
		OriginalSizeLabel: formatFileSize(originalSize),
		TrimmedSizeLabel:  formatFileSize(trimmedSize),
		SavedSizeLabel:    formatFileSize(savedSize),
	}
}

func replaceMapWithTrimmedVPK(mapName, sourcePath, trimmedPath string) error {
	tempDestPath := filepath.Join(consts.AddonsBasePath, "."+uuid.NewString()+"."+mapName+".trim.tmp")
	backupPath := filepath.Join(consts.AddonsBasePath, "."+uuid.NewString()+"."+mapName+".bak")
	_ = os.Remove(tempDestPath)
	_ = os.Remove(backupPath)

	if err := moveFile(trimmedPath, tempDestPath); err != nil {
		_ = os.Remove(tempDestPath)
		return fmt.Errorf("保存精简临时文件失败: %w", err)
	}

	tempDestActive := true
	defer func() {
		if tempDestActive {
			_ = os.Remove(tempDestPath)
		}
	}()

	if err := os.Rename(sourcePath, backupPath); err != nil {
		return fmt.Errorf("备份原文件失败: %w", err)
	}

	backupActive := true
	defer func() {
		if backupActive {
			_ = os.Remove(backupPath)
		}
	}()

	if err := os.Rename(tempDestPath, sourcePath); err != nil {
		if rollbackErr := os.Rename(backupPath, sourcePath); rollbackErr != nil {
			return fmt.Errorf("写入精简文件失败: %w；原文件回滚失败: %v", err, rollbackErr)
		}
		backupActive = false
		return fmt.Errorf("写入精简文件失败，已恢复原文件: %w", err)
	}

	tempDestActive = false
	return nil
}
