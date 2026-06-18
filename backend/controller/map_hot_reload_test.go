package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"

	"github.com/gin-gonic/gin"
)

func setupMapHotReloadControllerTest(t *testing.T) {
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

func newMapHotReloadTestContext(role string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("role", role)
	return c, w
}

func TestMapHotReloadConfigRequiresAdmin(t *testing.T) {
	setupMapHotReloadControllerTest(t)

	c, w := newMapHotReloadTestContext("guest", "")
	GetMapHotReloadConfig(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("guest get config status = %d, want %d", w.Code, http.StatusForbidden)
	}

	c, w = newMapHotReloadTestContext("guest", `{"command":"sm_update_vpk"}`)
	SetMapHotReloadConfig(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("guest update config status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestMapHotReloadStatusAllowsGuestAndHidesCommand(t *testing.T) {
	setupMapHotReloadControllerTest(t)

	c, w := newMapHotReloadTestContext("guest", "")
	GetMapHotReloadStatus(c)
	if w.Code != http.StatusOK {
		t.Fatalf("guest status code = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if resp["using_default"] != true {
		t.Fatalf("using_default = %#v, want true", resp["using_default"])
	}
	if _, ok := resp["command"]; ok {
		t.Fatalf("status response exposes command: %#v", resp)
	}
}

func TestMapHotReloadConfigAdminCanSaveAndRead(t *testing.T) {
	setupMapHotReloadControllerTest(t)

	c, w := newMapHotReloadTestContext("admin", `{"command":" sm_update_vpk "}`)
	SetMapHotReloadConfig(c)
	if w.Code != http.StatusOK {
		t.Fatalf("admin update config status = %d, body = %q", w.Code, w.Body.String())
	}

	var updateResp struct {
		Status  string `json:"status"`
		Command string `json:"command"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updateResp.Command != "sm_update_vpk" {
		t.Fatalf("updated command = %q, want sm_update_vpk", updateResp.Command)
	}

	c, w = newMapHotReloadTestContext("admin", "")
	GetMapHotReloadConfig(c)
	if w.Code != http.StatusOK {
		t.Fatalf("admin get config status = %d, body = %q", w.Code, w.Body.String())
	}

	var getResp struct {
		Command        string `json:"command"`
		DefaultCommand string `json:"default_command"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if getResp.Command != "sm_update_vpk" {
		t.Fatalf("config command = %q, want sm_update_vpk", getResp.Command)
	}
	if getResp.DefaultCommand != logic.DefaultMapHotReloadCommand {
		t.Fatalf("default command = %q, want %q", getResp.DefaultCommand, logic.DefaultMapHotReloadCommand)
	}
}

func TestHotReloadMapsAllowsGuestBeforeRconExecution(t *testing.T) {
	setupMapHotReloadControllerTest(t)
	t.Setenv("L4D2_RCON_URL", "")
	t.Setenv("L4D2_RCON_PASSWORD", "")

	c, w := newMapHotReloadTestContext("guest", "")
	HotReloadMaps(c)
	if w.Code == http.StatusForbidden {
		t.Fatalf("guest hot reload status = %d, want non-forbidden", w.Code)
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("guest hot reload status = %d, want %d without RCON config", w.Code, http.StatusInternalServerError)
	}
}
