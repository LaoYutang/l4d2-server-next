package controller

import (
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func Remove(c *gin.Context) {
	mapName, err := logic.NormalizeMapVPKName(c.PostForm("map"))
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "地图名称无效")
		return
	}
	LogOp(c, nil, "删除地图文件:", mapName)

	mutex.Lock()
	defer mutex.Unlock()

	root, err := os.OpenRoot(consts.AddonsBasePath)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "打开地图目录失败: %v", err)
		return
	}
	defer root.Close()

	err = root.Remove(mapName)
	if err != nil && !os.IsNotExist(err) {
		FailWithError(c, http.StatusInternalServerError, "删除地图文件失败: %v", err)
		return
	}

	// 删除maplist.txt中的记录
	mapListPath := consts.MapListFilePath
	mapListBytes, err := os.ReadFile(mapListPath)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "删除时读取maplist.txt失败: %v", err)
		return
	}
	mapList := strings.Split(string(mapListBytes), "\n")
	newMapList := make([]string, 0, 20)
	for _, m := range mapList {
		if m == mapName {
			continue
		}
		newMapList = append(newMapList, m)
	}
	newMapListBytes := []byte(strings.Join(newMapList, "\n"))
	err = os.WriteFile(mapListPath, newMapListBytes, 0644)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "删除时写入文件失败: %v", err)
		return
	}

	c.String(http.StatusOK, "删除成功！")
}
