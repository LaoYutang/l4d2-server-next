package controller

import (
	"errors"
	"fmt"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/middlewares"
	"l4d2-manager-next/utility"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type accessControlPreviewRequest struct {
	Section        string             `json:"section"`
	Enabled        *bool              `json:"enabled"`
	PanelBlacklist []logic.AccessRule `json:"panel_blacklist"`
	PanelWhitelist []logic.AccessRule `json:"panel_whitelist"`
	TrustedProxies []string           `json:"trusted_proxies"`
	TestIP         string             `json:"test_ip"`
}

type panelRulesUpdateRequest struct {
	ExpectedRevision uint64             `json:"expected_revision"`
	Enabled          bool               `json:"enabled"`
	PanelBlacklist   []logic.AccessRule `json:"panel_blacklist"`
	PanelWhitelist   []logic.AccessRule `json:"panel_whitelist"`
}

type trustedProxiesUpdateRequest struct {
	ExpectedRevision uint64   `json:"expected_revision"`
	TrustedProxies   []string `json:"trusted_proxies"`
}

func GetAccessControlConfig(c *gin.Context) {
	if !requireAccessControlAdmin(c) {
		return
	}
	writeAccessControlState(c, logic.GetAccessControlState())
}

func PreviewAccessControl(c *gin.Context) {
	if !requireAccessControlAdmin(c) {
		return
	}

	var request accessControlPreviewRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "参数错误"})
		return
	}

	var (
		result logic.AccessControlPreviewResult
		err    error
	)
	switch strings.TrimSpace(request.Section) {
	case "panel_rules":
		if request.Enabled == nil || request.PanelBlacklist == nil || request.PanelWhitelist == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "面板黑白名单草稿不完整"})
			return
		}
		result, err = logic.PreviewPanelAccessRules(logic.PanelRulesUpdate{
			Enabled:        *request.Enabled,
			PanelBlacklist: request.PanelBlacklist,
			PanelWhitelist: request.PanelWhitelist,
		}, middlewares.ClientIPInput(c), request.TestIP, utility.LookupRawRegion)
	case "trusted_proxies":
		if request.TrustedProxies == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "可信代理草稿不完整"})
			return
		}
		result, err = logic.PreviewTrustedProxies(
			request.TrustedProxies,
			middlewares.ClientIPInput(c),
			request.TestIP,
			utility.LookupRawRegion,
		)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_section", "message": "无效的预览类型"})
		return
	}
	if err != nil {
		writeAccessControlError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func UpdatePanelAccessRules(c *gin.Context) {
	if !requireAccessControlAdmin(c) {
		return
	}

	var request panelRulesUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.PanelBlacklist == nil || request.PanelWhitelist == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "参数错误"})
		return
	}
	detail := fmt.Sprintf(
		"更新面板黑白名单，启用: %t，黑名单: %d 条，白名单: %d 条",
		request.Enabled,
		len(request.PanelBlacklist),
		len(request.PanelWhitelist),
	)
	defer LogOp(c, detail)()

	state, err := logic.UpdatePanelAccessRules(
		request.ExpectedRevision,
		logic.PanelRulesUpdate{
			Enabled:        request.Enabled,
			PanelBlacklist: request.PanelBlacklist,
			PanelWhitelist: request.PanelWhitelist,
		},
		middlewares.ClientIPInput(c),
		utility.LookupRawRegion,
	)
	if err != nil {
		writeAccessControlError(c, err)
		return
	}
	writeAccessControlState(c, state)
}

func UpdateAccessControlTrustedProxies(c *gin.Context) {
	if !requireAccessControlAdmin(c) {
		return
	}

	var request trustedProxiesUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.TrustedProxies == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "参数错误"})
		return
	}
	defer LogOp(c, fmt.Sprintf("更新面板可信代理，共 %d 条", len(request.TrustedProxies)))()

	state, err := logic.UpdateTrustedProxies(
		request.ExpectedRevision,
		request.TrustedProxies,
		middlewares.ClientIPInput(c),
		utility.LookupRawRegion,
	)
	if err != nil {
		writeAccessControlError(c, err)
		return
	}
	writeAccessControlState(c, state)
}

func requireAccessControlAdmin(c *gin.Context) bool {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin_required", "message": "需要管理员权限"})
		return false
	}
	return true
}

func writeAccessControlState(c *gin.Context, state logic.AccessControlState) {
	snapshot := logic.CurrentAccessControlSnapshot()
	connection, err := snapshot.ResolveClientIP(middlewares.ClientIPInput(c))
	if err != nil {
		writeAccessControlError(c, err)
		return
	}
	decision, decisionErr := snapshot.Evaluate(connection.ClientIP, utility.LookupRawRegion)
	if decisionErr != nil && !errors.Is(decisionErr, logic.ErrAccessControlRecoveryMode) {
		writeAccessControlError(c, decisionErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"config":             state.Config,
		"recovery_mode":      state.RecoveryMode,
		"load_error":         state.LoadError,
		"geoip_available":    utility.IsGeoIPAvailable(),
		"current_connection": connection,
		"current_decision":   decision,
	})
}

func writeAccessControlError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, logic.ErrAccessControlRevisionConflict):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "revision_conflict",
			"message": "访问控制配置已被其他管理员修改，请刷新后重试",
		})
	case errors.Is(err, logic.ErrAccessControlWouldLockOut):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "current_admin_blocked",
			"message": "新配置会阻止当前管理员继续访问，已拒绝保存",
			"detail":  err.Error(),
		})
	case errors.Is(err, logic.ErrAccessControlGeoIPUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "geoip_unavailable",
			"message": "GeoIP 服务不可用，无法验证地区规则",
		})
	case errors.Is(err, logic.ErrAccessControlPersist):
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "persist_failed",
			"message": "保存访问控制配置失败",
		})
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_config",
			"message": err.Error(),
		})
	}
}
