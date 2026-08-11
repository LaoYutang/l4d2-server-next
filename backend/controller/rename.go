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
)

func RenameMap(c *gin.Context) {
	oldName := cleanExistingMapName(c.PostForm("oldName"))
	newNameInput := strings.TrimSpace(c.PostForm("newName"))
	defer LogOp(c, "重命名地图文件: "+oldName+" -> "+newNameInput)()

	if oldName == "" {
		FailWithError(c, http.StatusBadRequest, "原地图名称不能为空")
		return
	}
	if newNameInput == "" {
		FailWithError(c, http.StatusBadRequest, "新地图名称不能为空")
		return
	}

	newName := sanitizeMapRenameFilename(newNameInput)
	if newName == "" {
		FailWithError(c, http.StatusBadRequest, "新地图名称无效")
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	mapList, err := readMapList()
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "读取maplist.txt失败: %v", err)
		return
	}

	oldIndex := -1
	for i, name := range mapList {
		if name == oldName {
			oldIndex = i
			break
		}
	}
	if oldIndex == -1 {
		FailWithError(c, http.StatusBadRequest, "地图记录不存在")
		return
	}

	if oldName == newName {
		c.JSON(http.StatusOK, gin.H{"name": newName, "message": "地图名称未变化"})
		return
	}

	for _, name := range mapList {
		if name == newName {
			FailWithError(c, http.StatusBadRequest, "地图 %s 已经存在", newName)
			return
		}
	}

	oldPath := filepath.Join(consts.AddonsBasePath, oldName)
	newPath := filepath.Join(consts.AddonsBasePath, newName)
	if _, err := os.Stat(oldPath); err != nil {
		if os.IsNotExist(err) {
			FailWithError(c, http.StatusBadRequest, "地图文件不存在，无法重命名")
		} else {
			FailWithError(c, http.StatusBadRequest, "检查地图文件失败: %v", err)
		}
		return
	}
	if _, err := os.Stat(newPath); err == nil {
		FailWithError(c, http.StatusBadRequest, "目标文件 %s 已经存在", newName)
		return
	} else if !os.IsNotExist(err) {
		FailWithError(c, http.StatusBadRequest, "检查目标文件失败: %v", err)
		return
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		FailWithError(c, http.StatusInternalServerError, "重命名文件失败: %v", err)
		return
	}

	mapList[oldIndex] = newName
	if err := writeMapList(mapList); err != nil {
		_ = os.Rename(newPath, oldPath)
		FailWithError(c, http.StatusInternalServerError, "写入地图记录失败: %v", err)
		return
	}
	if err := logic.RenameMapVPKInspection(oldName, newName); err != nil {
		log.Printf("更新地图 VPK 检测记录名称失败（%s -> %s）: %v", oldName, newName, err)
	}

	c.JSON(http.StatusOK, gin.H{"name": newName, "message": fmt.Sprintf("地图已重命名为 %s", newName)})
}

func cleanExistingMapName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	if name == "." || name == "/" {
		return ""
	}
	return name
}

func sanitizeMapRenameFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(name), ".vpk") {
		name += ".vpk"
	}
	return sanitizeFilename(name)
}

func readMapList() ([]string, error) {
	mapListBytes, err := os.ReadFile(consts.MapListFilePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(mapListBytes), "\n")
	mapList := make([]string, 0, len(lines))
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name != "" {
			mapList = append(mapList, name)
		}
	}
	return mapList, nil
}

func writeMapList(mapList []string) error {
	content := strings.Join(mapList, "\n")
	if content != "" {
		content += "\n"
	}
	return os.WriteFile(consts.MapListFilePath, []byte(content), 0644)
}
