package controller

import (
	"l4d2-manager-next/logic"
	"net/http"

	"github.com/gin-gonic/gin"
)

func HotReloadMaps(c *gin.Context) {
	defer LogOp(c, "热重载地图")()

	if _, err := logic.ExecuteMapHotReloadCommand(); err != nil {
		FailWithError(c, http.StatusInternalServerError, "地图热重载失败: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "地图热重载指令已发送",
	})
}

func GetMapHotReloadStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"using_default": logic.IsMapHotReloadCommandDefault(),
	})
}

func GetMapHotReloadConfig(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"command":         logic.GetMapHotReloadCommand(),
		"default_command": logic.DefaultMapHotReloadCommand,
	})
}

func SetMapHotReloadConfig(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	var req struct {
		Command string `json:"command"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, http.StatusBadRequest, "参数错误")
		return
	}

	command, err := logic.SetMapHotReloadCommand(req.Command)
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "%v", err)
		return
	}

	defer LogOp(c, "设置地图热重载命令")()
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"command": command,
	})
}
