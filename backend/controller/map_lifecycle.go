package controller

import (
	"errors"
	"fmt"
	"l4d2-manager-next/logic"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ListMapLifecycle(c *gin.Context) {
	items, err := logic.ListMapLifecycleItems()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "读取地图状态失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func SetMapLocked(c *gin.Context) {
	if !requireMapLifecycleAdmin(c) {
		return
	}
	var request struct {
		Map              string `json:"map"`
		Locked           *bool  `json:"locked"`
		ExpectedRevision string `json:"expected_revision"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Locked == nil {
		FailWithError(c, http.StatusBadRequest, "请提供地图名称和锁定状态")
		return
	}
	mapName, err := logic.NormalizeMapVPKName(request.Map)
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "地图名称无效")
		return
	}
	defer LogOp(c, fmt.Sprintf("设置地图锁定: %s，locked=%t", mapName, *request.Locked))()
	item, err := logic.SetMapLocked(mapName, *request.Locked, request.ExpectedRevision)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, logic.ErrMapLifecycleConflict):
			status = http.StatusConflict
		case errors.Is(err, logic.ErrMapLifecycleNotFound):
			status = http.StatusNotFound
		case errors.Is(err, logic.ErrMapLifecycleInvalidName):
			status = http.StatusBadRequest
		}
		FailWithError(c, status, "保存地图锁定状态失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func RemoveMapsBatch(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	var request struct {
		Maps      []string          `json:"maps"`
		Revisions map[string]string `json:"revisions"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || len(request.Maps) == 0 {
		FailWithError(c, http.StatusBadRequest, "请选择要删除的地图")
		return
	}
	// Validate the complete request before deleting any selected map.
	for i, name := range request.Maps {
		normalized, err := logic.NormalizeMapVPKName(name)
		if err != nil {
			FailWithError(c, http.StatusBadRequest, "地图名称无效: %s", name)
			return
		}
		request.Maps[i] = normalized
	}
	defer LogOp(c, fmt.Sprintf("多选删除地图: %d项", len(request.Maps)))()
	result, err := logic.DeleteMapsWithRevisions(request.Maps, request.Revisions)
	respondMapDeleteResult(c, result, err)
}

func respondMapDeleteResult(c *gin.Context, result logic.MapDeleteResult, err error) {
	if err != nil && len(result.Failed) == 0 {
		FailWithError(c, http.StatusInternalServerError, "删除地图失败: %v", err)
		return
	}
	// The service can stop after a failed transaction. Preserve all completed and
	// failed item results instead of replacing them with a generic server error.
	writeMapDeleteResult(c, result)
}

func writeMapDeleteResult(c *gin.Context, result logic.MapDeleteResult) {
	// Partial results are successful HTTP responses so clients can report every
	// selected item, including skipped maps and individual failures.
	if len(result.Failed) > 0 {
		c.Set("audit_operation_failed", true)
	}
	c.JSON(http.StatusOK, result)
}
