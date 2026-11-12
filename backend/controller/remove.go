package controller

import (
	"l4d2-manager-next/logic"
	"net/http"

	"github.com/gin-gonic/gin"
)

func Remove(c *gin.Context) {
	mapName, err := logic.NormalizeMapVPKName(c.PostForm("map"))
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "地图名称无效")
		return
	}
	defer LogOp(c, "删除地图文件: "+mapName)()

	result, err := logic.DeleteMaps([]string{mapName})
	respondMapDeleteResult(c, result, err)
}
