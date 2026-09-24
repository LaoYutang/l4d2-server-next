package controller

import (
	"fmt"
	"l4d2-manager-next/logic"
	"net/http"

	"github.com/gin-gonic/gin"
)

type updateDiskUsageConfigRequest struct {
	LimitPercent int `json:"limit_percent"`
}

func requireDiskUsageConfigAdmin(c *gin.Context) bool {
	role, _ := c.Get("role")
	if role == "admin" {
		return true
	}

	FailWithError(c, http.StatusForbidden, "需要管理员权限")
	return false
}

func GetDiskUsageConfig(c *gin.Context) {
	if !requireDiskUsageConfigAdmin(c) {
		return
	}

	payload := gin.H{
		"limit_percent":   logic.GetDiskUsageLimitPercent(),
		"default_percent": logic.DefaultDiskUsageLimitPercent,
		"min_percent":     logic.MinDiskUsageLimitPercent,
		"max_percent":     logic.MaxDiskUsageLimitPercent,
	}

	// 磁盘信息读取失败不影响设置项的读写，前端会隐藏当前使用率。
	if usage, err := queryUploadDiskUsage(); err == nil {
		payload["current"] = gin.H{
			"used_percent": usage.UsedPercent,
			"free_bytes":   usage.Free,
			"total_bytes":  usage.Total,
		}
	}

	c.JSON(http.StatusOK, payload)
}

func SetDiskUsageConfig(c *gin.Context) {
	if !requireDiskUsageConfigAdmin(c) {
		return
	}

	var req updateDiskUsageConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, http.StatusBadRequest, "参数错误: %v", err)
		return
	}

	if _, err := logic.NormalizeDiskUsageLimitPercent(req.LimitPercent); err != nil {
		FailWithError(c, http.StatusBadRequest, "%v", err)
		return
	}

	defer LogOp(c, fmt.Sprintf("设置磁盘使用率上限为 %d%%", req.LimitPercent))()

	savedPercent, err := logic.SetDiskUsageLimitPercent(req.LimitPercent)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "保存磁盘配置失败: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"limit_percent": savedPercent,
	})
}
