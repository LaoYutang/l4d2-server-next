package controller

import (
	"l4d2-manager-next/logic"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetMapMissionDetail(c *gin.Context) {
	mapName := strings.TrimSpace(c.PostForm("map"))
	if mapName == "" {
		FailWithError(c, http.StatusBadRequest, "地图名称不能为空")
		return
	}

	campaigns, err := logic.GetMapMissionDetail(mapName)
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "解析地图详情失败: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":      mapName,
		"campaigns": campaigns,
	})
}
