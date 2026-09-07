package controller

import (
	"errors"
	"l4d2-manager-next/logic"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type pluginTextConfigRequest struct {
	Name             string `json:"name" binding:"required"`
	Path             string `json:"path"`
	Content          string `json:"content"`
	ExpectedRevision string `json:"expected_revision"`
}

func bindPluginTextRequest(c *gin.Context) (pluginTextConfigRequest, bool) {
	var req pluginTextConfigRequest
	if role, _ := c.Get("role"); role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return req, false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4*logic.MaxPluginTextConfigBytes)
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, http.StatusBadRequest, "配置请求无效: %v", err)
		return req, false
	}
	return req, true
}

func failPluginTextConfig(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, logic.ErrPluginConfigConflict):
		status = http.StatusConflict
	case errors.Is(err, logic.ErrPluginConfigPath):
		status = http.StatusForbidden
	case errors.Is(err, logic.ErrPluginConfigUnavailable), errors.Is(err, logic.ErrPluginConfigNotEditable):
		status = http.StatusBadRequest
	case errors.Is(err, os.ErrNotExist):
		status = http.StatusNotFound
	}
	FailWithError(c, status, "%v", err)
}

func ListPluginTextConfigs(c *gin.Context) {
	req, ok := bindPluginTextRequest(c)
	if !ok {
		return
	}
	files, err := logic.ListPluginTextConfigs(req.Name)
	if err != nil {
		failPluginTextConfig(c, err)
		return
	}
	c.JSON(http.StatusOK, files)
}

func ReadPluginTextConfig(c *gin.Context) {
	req, ok := bindPluginTextRequest(c)
	if !ok {
		return
	}
	file, err := logic.ReadPluginTextConfig(req.Name, req.Path)
	if err != nil {
		failPluginTextConfig(c, err)
		return
	}
	c.JSON(http.StatusOK, file)
}

func UpdatePluginTextConfig(c *gin.Context) {
	req, ok := bindPluginTextRequest(c)
	if !ok {
		return
	}
	defer LogOp(c, "编辑 NUT 配置: "+req.Name+" / "+req.Path)()
	file, err := logic.UpdatePluginTextConfig(req.Name, req.Path, req.Content, req.ExpectedRevision)
	if err != nil {
		failPluginTextConfig(c, err)
		return
	}
	c.JSON(http.StatusOK, file)
}
