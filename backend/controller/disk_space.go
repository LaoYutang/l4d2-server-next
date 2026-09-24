package controller

import (
	"fmt"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/middlewares"
	"net/http"

	"github.com/gin-gonic/gin"
)

// diskLimitPayload 是磁盘检查被拒绝时的响应体。code 区分两类拦截：
// disk_usage_exceeded 只是超过配置上限，管理员可以二次确认后继续；
// disk_space_insufficient 是预判会写满，任何角色都不能继续。
type diskLimitPayload struct {
	Code          string  `json:"code"`
	Message       string  `json:"message"`
	UsedPercent   float64 `json:"used_percent"`
	LimitPercent  int     `json:"limit_percent"`
	FreeBytes     uint64  `json:"free_bytes"`
	TotalBytes    uint64  `json:"total_bytes"`
	RequiredBytes uint64  `json:"required_bytes"`
	CanForce      bool    `json:"can_force"`
}

// queryUploadDiskUsage 便于测试注入磁盘快照，避免测试依赖真实磁盘状态。
var queryUploadDiskUsage = logic.QueryAddonsDiskUsage

// ensureUploadDiskSpace 检查 addons 目录所在分区的可用空间。
// allowForce 为真时允许管理员带 force=true 越过使用率上限（预判写满仍然拦截）。
// 返回 false 时已写入 507 响应，调用方必须立即返回。
func ensureUploadDiskSpace(c *gin.Context, fileSize int64, allowForce bool) (ok bool, forced bool) {
	usage, err := queryUploadDiskUsage()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "获取磁盘使用信息失败: %v", err)
		return false, false
	}

	limitPercent := logic.GetDiskUsageLimitPercent()
	role, _ := c.Get("role")
	isAdmin := role == middlewares.RoleAdmin
	forced = allowForce && isAdmin && c.PostForm("force") == "true"

	decision := logic.EvaluateDiskSpace(usage, limitPercent, fileSize, forced)
	if !decision.Blocked() {
		return true, forced
	}

	canForce := allowForce && isAdmin && decision.BlockReason == logic.DiskBlockUsageExceeded
	payload := diskLimitPayload{
		Code:          string(decision.BlockReason),
		Message:       diskLimitMessage(decision),
		UsedPercent:   decision.UsedPercent,
		LimitPercent:  decision.LimitPercent,
		FreeBytes:     decision.FreeBytes,
		TotalBytes:    decision.TotalBytes,
		RequiredBytes: decision.RequiredBytes,
		CanForce:      canForce,
	}

	fmt.Printf(
		"[WARN] 拒绝写入 | %s | %s/%s | 使用率 %.1f%%(上限 %d%%) | 剩余 %s | 需要 %s\n",
		role,
		c.Request.Method,
		c.Request.URL.Path,
		payload.UsedPercent,
		payload.LimitPercent,
		formatBytes(payload.FreeBytes),
		formatBytes(payload.RequiredBytes),
	)

	c.JSON(http.StatusInsufficientStorage, payload)
	return false, false
}

func diskLimitMessage(decision logic.DiskSpaceDecision) string {
	if decision.BlockReason == logic.DiskBlockFreeInsufficient {
		return fmt.Sprintf(
			"磁盘空间不足：addons 所在分区剩余可用空间 %s，本次上传预计需要 %s（含分片临时文件与解压产物）",
			formatBytes(decision.FreeBytes),
			formatBytes(decision.RequiredBytes),
		)
	}

	return fmt.Sprintf(
		"磁盘空间不足：addons 所在分区当前使用率 %.1f%%，已超过限制 %d%%",
		decision.UsedPercent,
		decision.LimitPercent,
	)
}

func formatBytes(size uint64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	value := float64(size)
	index := -1
	for value >= unit && index < len(units)-1 {
		value /= unit
		index++
	}

	return fmt.Sprintf("%.1f %s", value, units[index])
}
