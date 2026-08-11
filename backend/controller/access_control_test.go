package controller

import (
	"bytes"
	"encoding/json"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/middlewares"
	"l4d2-manager-next/model"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupAccessControlControllerTest(t *testing.T) {
	t.Helper()
	oldManagerDataPath := consts.ManagerDataPath
	oldConfigPath := consts.AccessControlConfigPath
	oldEnqueueAuditLog := enqueueAuditLog
	consts.ManagerDataPath = filepath.Join(t.TempDir(), "data")
	consts.AccessControlConfigPath = filepath.Join(consts.ManagerDataPath, "access_control.json")
	enqueueAuditLog = func(model.AuditLog) {}
	logic.InitAccessControl()
	t.Cleanup(func() {
		consts.ManagerDataPath = oldManagerDataPath
		consts.AccessControlConfigPath = oldConfigPath
		enqueueAuditLog = oldEnqueueAuditLog
		logic.InitAccessControl()
	})
}

func accessControlControllerRouter(role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middlewares.AccessControl())
	router.Use(func(c *gin.Context) {
		c.Set("role", role)
		c.Next()
	})
	router.POST("/access-control/config", GetAccessControlConfig)
	router.POST("/access-control/panel-rules/update", UpdatePanelAccessRules)
	router.POST("/access-control/game-bans/list", ListGameBans)
	router.POST("/access-control/game-bans/add", AddGameBan)
	router.POST("/access-control/game-bans/remove", RemoveGameBan)
	return router
}

func TestAccessControlConfigRequiresAdministrator(t *testing.T) {
	setupAccessControlControllerTest(t)
	router := accessControlControllerRouter("guest")
	request := httptest.NewRequest(http.MethodPost, "/access-control/config", nil)
	request.RemoteAddr = "198.51.100.20:45000"
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestAccessControlConfigReturnsCanonicalClientIPToAdministrator(t *testing.T) {
	setupAccessControlControllerTest(t)
	router := accessControlControllerRouter("admin")
	request := httptest.NewRequest(http.MethodPost, "/access-control/config", nil)
	request.RemoteAddr = "203.0.113.8:45000"
	request.Header.Set("X-Forwarded-For", "127.0.0.1")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response struct {
		CurrentConnection logic.ClientIPInfo `json:"current_connection"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.CurrentConnection.ClientIP != "203.0.113.8" {
		t.Fatalf("client IP = %q, want direct peer", response.CurrentConnection.ClientIP)
	}
}

func TestPanelRulesControllerRejectsAdministratorLockout(t *testing.T) {
	setupAccessControlControllerTest(t)
	state := logic.GetAccessControlState()
	payload := map[string]any{
		"expected_revision": state.Config.Revision,
		"enabled":           true,
		"panel_blacklist":   []any{},
		"panel_whitelist": []map[string]any{
			{
				"enabled": true,
				"type":    "ip",
				"value":   "203.0.113.99",
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	router := accessControlControllerRouter("admin")
	request := httptest.NewRequest(http.MethodPost, "/access-control/panel-rules/update", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "198.51.100.20:45000"
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusConflict, recorder.Body.String())
	}
	if currentRevision := logic.GetAccessControlState().Config.Revision; currentRevision != state.Config.Revision {
		t.Fatalf("revision changed after rejected update: got %d want %d", currentRevision, state.Config.Revision)
	}
}

func TestAuditLogUsesResolvedTrustedProxyClientIP(t *testing.T) {
	setupAccessControlControllerTest(t)
	var captured model.AuditLog
	enqueueAuditLog = func(entry model.AuditLog) {
		captured = entry
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middlewares.AccessControl())
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.GET("/audit-ip", func(c *gin.Context) {
		defer LogOp(c, "测试可信代理审计 IP")()
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/audit-ip", nil)
	request.RemoteAddr = "127.0.0.2:45000"
	request.Header.Set("X-Forwarded-For", "198.51.100.33")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if captured.IP != "198.51.100.33" {
		t.Fatalf("audit IP = %q, want resolved client IP", captured.IP)
	}
}
