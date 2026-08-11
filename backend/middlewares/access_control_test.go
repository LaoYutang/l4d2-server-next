package middlewares

import (
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupAccessControlMiddlewareTest(t *testing.T) {
	t.Helper()
	oldManagerDataPath := consts.ManagerDataPath
	oldConfigPath := consts.AccessControlConfigPath
	consts.ManagerDataPath = filepath.Join(t.TempDir(), "data")
	consts.AccessControlConfigPath = filepath.Join(consts.ManagerDataPath, "access_control.json")
	logic.InitAccessControl()
	t.Cleanup(func() {
		consts.ManagerDataPath = oldManagerDataPath
		consts.AccessControlConfigPath = oldConfigPath
		logic.InitAccessControl()
	})
}

func newClientIPTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AccessControl())
	router.GET("/ip", func(c *gin.Context) {
		c.String(http.StatusOK, GetClientIP(c))
	})
	return router
}

func TestAccessControlMiddlewareIgnoresSpoofedHeadersFromUntrustedPeer(t *testing.T) {
	setupAccessControlMiddlewareTest(t)
	router := newClientIPTestRouter()
	request := httptest.NewRequest(http.MethodGet, "/ip", nil)
	request.RemoteAddr = "203.0.113.20:45000"
	request.Header.Set("X-Forwarded-For", "127.0.0.1")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "203.0.113.20" {
		t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestAccessControlMiddlewareUsesForwardedIPFromTrustedLoopback(t *testing.T) {
	setupAccessControlMiddlewareTest(t)
	router := newClientIPTestRouter()
	request := httptest.NewRequest(http.MethodGet, "/ip", nil)
	request.RemoteAddr = "127.0.0.2:45000"
	request.Header.Set("X-Forwarded-For", "198.51.100.15")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "198.51.100.15" {
		t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestAccessControlMiddlewareRejectsMalformedHeaderFromTrustedProxy(t *testing.T) {
	setupAccessControlMiddlewareTest(t)
	router := newClientIPTestRouter()
	request := httptest.NewRequest(http.MethodGet, "/ip", nil)
	request.RemoteAddr = "127.0.0.1:45000"
	request.Header.Set("X-Forwarded-For", "invalid")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
}
