package controller

import (
	"l4d2-manager-next/consts"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func Remove(c *gin.Context) {
	mutex.Lock()
	defer mutex.Unlock()

	mapPath := filepath.Join(consts.AddonsBasePath, c.PostForm("map"))
	err := os.Remove(mapPath)
	if err != nil {
		c.String(http.StatusBadRequest, "地图不存在")
		return
	}

	// 删除maplist.txt中的记录
	mapListPath := consts.MapListFilePath
	mapListBytes, err := os.ReadFile(mapListPath)
	if err != nil {
		c.String(http.StatusBadRequest, "删除时maplist.txt不存在")
		return
	}
	mapList := strings.Split(string(mapListBytes), "\n")
	newMapList := make([]string, 0, 20)
	for _, m := range mapList {
		if m == c.PostForm("map") {
			continue
		}
		newMapList = append(newMapList, m)
	}
	newMapListBytes := []byte(strings.Join(newMapList, "\n"))
	err = os.WriteFile(mapListPath, newMapListBytes, 0644)
	if err != nil {
		c.String(http.StatusBadRequest, "删除时写入文件失败")
		return
	}

	c.String(http.StatusOK, "删除成功！")
}
