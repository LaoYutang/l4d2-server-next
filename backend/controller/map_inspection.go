package controller

import (
	"errors"
	"l4d2-manager-next/logic"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetMapGlobalScripts(c *gin.Context) {
	var request struct {
		Map string `json:"map"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		FailWithError(c, http.StatusBadRequest, "参数错误")
		return
	}

	mapName, err := logic.NormalizeMapVPKName(strings.TrimSpace(request.Map))
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "地图名称无效")
		return
	}

	scripts, err := logic.GetMapGlobalScriptContents(mapName)
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrMapInspectionStale):
			FailWithError(c, http.StatusConflict, "地图文件已发生变化，请重新检测后再试")
		case errors.Is(err, logic.ErrMapInspectionNotFound),
			errors.Is(err, logic.ErrMapRecordNotFound),
			errors.Is(err, logic.ErrNoGlobalScripts),
			os.IsNotExist(err):
			FailWithError(c, http.StatusNotFound, "%v", err)
		default:
			FailWithError(c, http.StatusInternalServerError, "读取全局脚本失败: %v", err)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"map":     mapName,
		"scripts": scripts,
	})
}
