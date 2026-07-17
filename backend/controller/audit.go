package controller

import (
	"errors"
	"l4d2-manager-next/logic"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type AuditListRequest struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
	Role      string `json:"role"`
	IP        string `json:"ip"`
	Path      string `json:"path"`
	Success   *bool  `json:"success"`
	Keyword   string `json:"keyword"`
}

func ListAuditLogs(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.String(http.StatusForbidden, "需要管理员权限")
		return
	}

	var req AuditListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize != 20 && req.PageSize != 50 && req.PageSize != 100 {
		if req.PageSize > 100 {
			req.PageSize = 100
		} else {
			req.PageSize = 20
		}
	}
	if req.StartTime < 0 || req.EndTime < 0 || (req.StartTime > 0 && req.EndTime > 0 && req.StartTime > req.EndTime) {
		c.String(http.StatusBadRequest, "无效的时间范围")
		return
	}
	if req.Role != "" && req.Role != "admin" && req.Role != "guest" {
		c.String(http.StatusBadRequest, "无效的角色")
		return
	}

	items, total, err := logic.ListAuditLogs(logic.AuditListFilter{
		Page:      req.Page,
		PageSize:  req.PageSize,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Role:      req.Role,
		IP:        strings.TrimSpace(req.IP),
		Path:      strings.TrimSpace(req.Path),
		Success:   req.Success,
		Keyword:   strings.TrimSpace(req.Keyword),
	})
	if errors.Is(err, logic.ErrAuditDatabaseUnavailable) {
		c.String(http.StatusServiceUnavailable, "审计数据库不可用")
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "查询审计记录失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	})
}
