package controller

import (
	"errors"
	"fmt"
	"l4d2-manager-next/logic"
	"net/http"

	"github.com/gin-gonic/gin"
)

func requireMapImportPolicyAdmin(c *gin.Context) bool {
	if c.GetString("role") == "admin" {
		return true
	}
	FailWithError(c, http.StatusForbidden, "需要管理员权限")
	return false
}

func GetMapImportPolicy(c *gin.Context) {
	if !requireMapImportPolicyAdmin(c) {
		return
	}
	c.JSON(http.StatusOK, logic.GetMapImportPolicy())
}

func SetMapImportPolicy(c *gin.Context) {
	if !requireMapImportPolicyAdmin(c) {
		return
	}
	// Require the complete object: omitted fields must not silently disable rules.
	var request struct {
		Missing    *bool `json:"reject_missing_dictionary"`
		Global     *bool `json:"reject_global_scripts"`
		Overrides  *bool `json:"reject_script_overrides"`
		Inspection *bool `json:"reject_inspection_error"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Missing == nil || request.Global == nil || request.Overrides == nil || request.Inspection == nil {
		FailWithError(c, http.StatusBadRequest, "请提供全部四项风险拦截开关")
		return
	}
	policy := logic.MapImportPolicy{RejectMissingDictionary: *request.Missing, RejectGlobalScripts: *request.Global, RejectScriptOverrides: *request.Overrides, RejectInspectionError: *request.Inspection}
	defer LogOp(c, fmt.Sprintf("设置地图导入风险拦截：字典缺失=%t，全局脚本=%t，脚本覆盖=%t，检测异常=%t", *request.Missing, *request.Global, *request.Overrides, *request.Inspection))()
	if err := logic.SetMapImportPolicy(policy); err != nil {
		FailWithError(c, http.StatusInternalServerError, "保存地图导入策略失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, policy)
}

func failMapImport(c *gin.Context, err error) {
	var rejection *logic.MapImportRejectedError
	if errors.As(err, &rejection) {
		defer LogOp(c, rejection.Error())()
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": "map_import_rejected", "message": rejection.Error(), "violations": rejection.Violations})
		return
	}
	FailWithError(c, http.StatusInternalServerError, "处理文件失败: %v", err)
}
