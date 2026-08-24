package controller

import (
	"bytes"
	"encoding/json"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGameBanPersistenceBlockIsHiddenFromServerCustomConfig(t *testing.T) {
	oldGamePath := consts.GamePath
	gamePath := t.TempDir()
	consts.GamePath = gamePath
	t.Cleanup(func() { consts.GamePath = oldGamePath })
	cfgDir := filepath.Join(gamePath, "cfg")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"hostname test",
		`rcon_password "do-not-leak"`,
		`sv_password "also-secret"`,
		logic.GameBanConfigMarker,
		"exec banned_user.cfg",
		"exec banned_ip.cfg",
		"",
		CustomConfigMarker,
		"sm_cvar custom_value 1",
		"",
	}, "\n")
	configPath := filepath.Join(cfgDir, "server.cfg")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/server-config/get", GetServerConfig)
	request := httptest.NewRequest(http.MethodPost, "/server-config/get", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	var response ServerConfigResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.CustomConfig) != 1 || response.CustomConfig[0] != "sm_cvar custom_value 1" {
		t.Fatalf("custom config = %#v", response.CustomConfig)
	}
	if strings.Contains(response.FixedConfig, "do-not-leak") || strings.Contains(response.FixedConfig, "also-secret") {
		t.Fatalf("fixed config leaked a password: %s", response.FixedConfig)
	}
	if !strings.Contains(response.FixedConfig, `rcon_password "********"`) || !strings.Contains(response.FixedConfig, logic.GameBanConfigMarker) {
		t.Fatalf("fixed config was not redacted/preserved: %s", response.FixedConfig)
	}

	router = gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/server-config/update", UpdateServerConfig)
	updateBody := []byte(`{"hidden":false,"lobby_connect_only":false,"steam_group":"","custom_config":["sm_cvar changed 1"]}`)
	request = httptest.NewRequest(http.MethodPost, "/server-config/update", bytes.NewReader(updateBody))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	updatedText := string(updated)
	if !strings.Contains(updatedText, logic.GameBanConfigMarker) || strings.Index(updatedText, logic.GameBanConfigMarker) > strings.Index(updatedText, CustomConfigMarker) {
		t.Fatalf("game ban block was not preserved before custom config:\n%s", updatedText)
	}
}

func TestUpdateServerConfigNormalizesCommentsAndSyncsTickFiles(t *testing.T) {
	oldGamePath := consts.GamePath
	gamePath := t.TempDir()
	consts.GamePath = gamePath
	t.Cleanup(func() { consts.GamePath = oldGamePath })
	cfgDir := filepath.Join(gamePath, "cfg")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"server.cfg":         "sm_cvar sv_minrate 60000",
		"server.cfg.60tick":  "sm_cvar sv_minrate 60000",
		"server.cfg.100tick": "sm_cvar sv_minrate 100000",
	}
	for name, fixedLine := range files {
		content := fixedLine + "\n\n" + CustomConfigMarker + "\nsm_cvar old 1\n"
		if err := os.WriteFile(filepath.Join(cfgDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/server-config/update", UpdateServerConfig)
	body := []byte(`{"hidden":true,"lobby_connect_only":false,"steam_group":"","custom_config":["// 第一行","// 第二行","sm_cvar changed \"http://example.com/a//b\" // 行尾"]}`)
	request := httptest.NewRequest(http.MethodPost, "/server-config/update", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
	}

	wantCustom := "// 第一行\n// 第二行\n// 行尾\nsm_cvar changed \"http://example.com/a//b\""
	for name, fixedLine := range files {
		updated, err := os.ReadFile(filepath.Join(cfgDir, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(updated)
		if !strings.Contains(text, fixedLine) || !strings.Contains(text, wantCustom) {
			t.Fatalf("%s was not preserved/synced:\n%s", name, text)
		}
		if strings.Contains(text, "sm_cvar old 1") {
			t.Fatalf("%s retained old custom config:\n%s", name, text)
		}
	}
}

func TestUpdateServerConfigRejectsTrailingCommentsBeforeWriting(t *testing.T) {
	oldGamePath := consts.GamePath
	gamePath := t.TempDir()
	consts.GamePath = gamePath
	t.Cleanup(func() { consts.GamePath = oldGamePath })
	cfgDir := filepath.Join(gamePath, "cfg")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(cfgDir, "server.cfg")
	original := "hostname unchanged\n"
	if err := os.WriteFile(configPath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/server-config/update", UpdateServerConfig)
	body := []byte(`{"hidden":false,"lobby_connect_only":false,"steam_group":"","custom_config":["sm_cvar value 1","// 末尾注释"]}`)
	request := httptest.NewRequest(http.MethodPost, "/server-config/update", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(updated) != original {
		t.Fatalf("config changed after validation error:\n%s", updated)
	}
}
