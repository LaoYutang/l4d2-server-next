package controller

import (
	"bytes"
	"encoding/json"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPluginTextConfigAuthorizationAndRevision(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, game := t.TempDir(), t.TempDir()
	t.Setenv(logic.PluginStorePathEnv, store)
	oldGame := consts.GamePath
	consts.GamePath = game
	t.Cleanup(func() { consts.GamePath = oldGame })
	for p, content := range map[string]string{"scripts/vscripts/p.nut": "nut", "cfg/p.cfg": "value=1\n"} {
		file := filepath.Join(store, "P/left4dead2", p)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := logic.EnablePlugin("P"); err != nil {
		t.Fatal(err)
	}
	call := func(role, endpoint string, body any) *httptest.ResponseRecorder {
		t.Helper()
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("role", role) })
		r.POST("/list", ListPluginTextConfigs)
		r.POST("/read", ReadPluginTextConfig)
		r.POST("/update", UpdatePluginTextConfig)
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, endpoint, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	for _, endpoint := range []string{"/list", "/read", "/update"} {
		if w := call("guest", endpoint, map[string]any{"name": "P", "path": "cfg/p.cfg"}); w.Code != http.StatusForbidden {
			t.Fatalf("guest %s: %d", endpoint, w.Code)
		}
	}
	w := call("admin", "/read", map[string]any{"name": "P", "path": "cfg/p.cfg"})
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	var cfg logic.PluginTextConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	request := map[string]any{"name": "P", "path": cfg.Path, "content": "", "expected_revision": cfg.Revision}
	if w := call("admin", "/update", request); w.Code != http.StatusOK {
		t.Fatalf("clearing config failed: %s", w.Body.String())
	}
	if w := call("admin", "/update", request); w.Code != http.StatusConflict {
		t.Fatalf("stale update=%d %s", w.Code, w.Body.String())
	}
	if w := call("admin", "/read", map[string]any{"name": "P", "path": "cfg/server.cfg"}); w.Code != http.StatusForbidden {
		t.Fatalf("unowned read=%d", w.Code)
	}
	if w := call("admin", "/read", map[string]any{"name": "P", "path": "../secret.cfg"}); w.Code != http.StatusForbidden {
		t.Fatalf("traversal read=%d", w.Code)
	}
	if err := logic.DisablePlugin("P"); err != nil {
		t.Fatal(err)
	}
	if w := call("admin", "/list", map[string]any{"name": "P"}); w.Code != http.StatusBadRequest {
		t.Fatalf("disabled configs=%d", w.Code)
	}
}

func TestPluginBackupTextOnlyVisibleToAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := t.TempDir()
	t.Setenv(logic.PluginStorePathEnv, store)
	data := "backups:\n  - name: backup\n    plugins:\n      - name: P\n        configs: []\n        text_configs:\n          - path: ems/p/settings.cfg\n            content: private-content\n            encoding: utf-8\n"
	if err := os.WriteFile(filepath.Join(store, logic.BackupFileName), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"guest", "admin"} {
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("role", role) })
		r.POST("/detail", GetBackupPluginsDetail)
		req := httptest.NewRequest(http.MethodPost, "/detail", strings.NewReader(url.Values{"name": {"backup"}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatal(w.Body.String())
		}
		if strings.Contains(w.Body.String(), "private-content") != (role == "admin") {
			t.Fatalf("unexpected %s visibility: %s", role, w.Body.String())
		}
	}
}
