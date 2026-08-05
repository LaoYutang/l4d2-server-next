package controller

import (
	"encoding/json"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupDownloadConfigControllerTest(t *testing.T) {
	t.Helper()

	oldManagerDataPath := consts.ManagerDataPath
	oldManagerConfigPath := consts.ManagerConfigPath

	consts.ManagerDataPath = filepath.Join(t.TempDir(), "data")
	consts.ManagerConfigPath = filepath.Join(consts.ManagerDataPath, "manager_config.json")
	logic.LoadManagerConfig()
	gin.SetMode(gin.TestMode)

	t.Cleanup(func() {
		consts.ManagerDataPath = oldManagerDataPath
		consts.ManagerConfigPath = oldManagerConfigPath
		logic.LoadManagerConfig()
	})
}

func newDownloadConfigTestContext(role, body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/download/config", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("role", role)
	return c, w
}

func TestDownloadConfigRequiresAdmin(t *testing.T) {
	setupDownloadConfigControllerTest(t)

	c, w := newDownloadConfigTestContext("guest", "")
	GetDownloadConfig(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("guest get config status = %d, want %d", w.Code, http.StatusForbidden)
	}

	c, w = newDownloadConfigTestContext("guest", `{"steam_cdn_ip":"192.0.2.10"}`)
	SetDownloadConfig(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("guest update config status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestDownloadConfigAdminCanSaveReadAndClear(t *testing.T) {
	setupDownloadConfigControllerTest(t)

	c, w := newDownloadConfigTestContext("admin", `{"steam_cdn_ip":" 2001:0db8::1 "}`)
	SetDownloadConfig(c)
	if w.Code != http.StatusOK {
		t.Fatalf("admin update config status = %d, body = %q", w.Code, w.Body.String())
	}

	var updateResp struct {
		Status     string `json:"status"`
		SteamCDNIP string `json:"steam_cdn_ip"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updateResp.Status != "ok" || updateResp.SteamCDNIP != "2001:db8::1" {
		t.Fatalf("update response = %#v, want normalized IP", updateResp)
	}

	logic.LoadManagerConfig()
	c, w = newDownloadConfigTestContext("admin", "")
	GetDownloadConfig(c)
	if w.Code != http.StatusOK {
		t.Fatalf("admin get config status = %d, body = %q", w.Code, w.Body.String())
	}

	var getResp struct {
		SteamCDNIP string `json:"steam_cdn_ip"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if getResp.SteamCDNIP != "2001:db8::1" {
		t.Fatalf("saved Steam CDN IP = %q, want 2001:db8::1", getResp.SteamCDNIP)
	}

	c, w = newDownloadConfigTestContext("admin", `{"steam_cdn_ip":""}`)
	SetDownloadConfig(c)
	if w.Code != http.StatusOK || logic.GetSteamCDNIP() != "" {
		t.Fatalf("clear config status = %d, IP = %q", w.Code, logic.GetSteamCDNIP())
	}
}

func TestDownloadConfigRejectsInvalidIP(t *testing.T) {
	setupDownloadConfigControllerTest(t)

	c, w := newDownloadConfigTestContext("admin", `{"steam_cdn_ip":"cdn.steamusercontent.com"}`)
	SetDownloadConfig(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid IP status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if got := logic.GetSteamCDNIP(); got != "" {
		t.Fatalf("Steam CDN IP after invalid update = %q, want empty", got)
	}
}
