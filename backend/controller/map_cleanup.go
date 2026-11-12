package controller

import (
	"errors"
	"fmt"
	"l4d2-manager-next/logic"
	"net/http"

	"github.com/gin-gonic/gin"
)

func requireMapLifecycleAdmin(c *gin.Context) bool {
	if c.GetString("role") == "admin" {
		return true
	}
	FailWithError(c, http.StatusForbidden, "需要管理员权限")
	return false
}

func GetMapCleanupConfig(c *gin.Context) {
	if !requireMapLifecycleAdmin(c) {
		return
	}
	config, err := logic.GetMapCleanupConfig()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "读取地图自动清理配置失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, config)
}

func SetMapCleanupConfig(c *gin.Context) {
	if !requireMapLifecycleAdmin(c) {
		return
	}
	var request struct {
		Enabled       *bool `json:"enabled"`
		RetentionDays *int  `json:"retention_days"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Enabled == nil || request.RetentionDays == nil {
		FailWithError(c, http.StatusBadRequest, "请提供自动清理开关和保留天数")
		return
	}
	if err := logic.ValidateMapCleanupConfig(*request.Enabled, *request.RetentionDays); err != nil {
		FailWithError(c, http.StatusBadRequest, "%v", err)
		return
	}
	defer LogOp(c, fmt.Sprintf("设置地图自动清理: enabled=%t，保留%d天", *request.Enabled, *request.RetentionDays))()
	config, err := logic.SetMapCleanupConfig(*request.Enabled, *request.RetentionDays)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, logic.ErrInvalidMapCleanupConfig) {
			status = http.StatusBadRequest
		}
		FailWithError(c, status, "%v", err)
		return
	}
	c.JSON(http.StatusOK, config)
}
