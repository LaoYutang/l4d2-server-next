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

	mutex.RLock()
	scripts, revision, err := logic.GetMapGlobalScriptContentsWithRevision(mapName)
	mutex.RUnlock()
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
		"map":      mapName,
		"revision": revision,
		"scripts":  scripts,
	})
}

func UpdateMapGlobalScript(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	var request struct {
		Map              string `json:"map"`
		Path             string `json:"path"`
		Content          string `json:"content"`
		Encoding         string `json:"encoding"`
		ExpectedRevision string `json:"expected_revision"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		FailWithError(c, http.StatusBadRequest, "参数错误或请求内容过大")
		return
	}

	mapName, err := logic.NormalizeMapVPKName(strings.TrimSpace(request.Map))
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "地图名称无效")
		return
	}
	scriptPath := strings.TrimSpace(request.Path)
	defer LogOp(c, "编辑地图全局脚本: "+mapName+" / "+scriptPath)()

	mutex.Lock()
	result, err := logic.UpdateMapGlobalScript(
		mapName,
		scriptPath,
		request.Content,
		request.Encoding,
		request.ExpectedRevision,
	)
	mutex.Unlock()
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrMapRevisionConflict),
			errors.Is(err, logic.ErrMapInspectionStale):
			FailWithError(c, http.StatusConflict, "地图文件已发生变化，请重新打开脚本后再试")
		case errors.Is(err, logic.ErrMapInspectionNotFound),
			errors.Is(err, logic.ErrMapRecordNotFound),
			errors.Is(err, logic.ErrNoGlobalScripts),
			errors.Is(err, logic.ErrGlobalScriptNotFound),
			os.IsNotExist(err):
			FailWithError(c, http.StatusNotFound, "%v", err)
		case errors.Is(err, logic.ErrGlobalScriptPathInvalid),
			errors.Is(err, logic.ErrGlobalScriptNotEditable),
			errors.Is(err, logic.ErrGlobalScriptContentTooLarge),
			errors.Is(err, logic.ErrGlobalScriptRepackUnsupported),
			errors.Is(err, logic.ErrGlobalScriptDuplicateEntry):
			FailWithError(c, http.StatusUnprocessableEntity, "%v", err)
		default:
			FailWithError(c, http.StatusInternalServerError, "保存全局脚本失败: %v", err)
		}
		return
	}

	c.JSON(http.StatusOK, result)
}
