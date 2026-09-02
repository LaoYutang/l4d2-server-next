package controller

import (
	"encoding/json"
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

func TestBackupServerConfigMasksSteamGroupForGuest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storePath := t.TempDir()
	t.Setenv(logic.PluginStorePathEnv, storePath)
	const steamGroup = "123456789"
	backupData := `backups:
  - name: secret-test
    created_at: 1
    plugins: []
    server_config:
      hidden: false
      lobby_connect_only: true
      steam_group: "` + steamGroup + `"
`
	if err := os.WriteFile(filepath.Join(storePath, logic.BackupFileName), []byte(backupData), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		role string
		want string
	}{
		{name: "guest is masked", role: "guest", want: serverConfigValueMask},
		{name: "admin sees value", role: "admin", want: steamGroup},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("role", test.role)
				c.Next()
			})
			router.POST("/plugins/backups/detail/server_config", GetBackupServerConfigDetail)
			body := url.Values{"name": {"secret-test"}}.Encode()
			request := httptest.NewRequest(
				http.MethodPost,
				"/plugins/backups/detail/server_config",
				strings.NewReader(body),
			)
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
			}

			var response logic.BackupServerConfig
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.SteamGroup != test.want {
				t.Fatalf("steam_group = %q, want %q", response.SteamGroup, test.want)
			}
			if test.role != "admin" && strings.Contains(recorder.Body.String(), steamGroup) {
				t.Fatalf("guest response leaked Steam group: %s", recorder.Body.String())
			}
		})
	}
}

func TestBackupExportsRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		path    string
		handler gin.HandlerFunc
	}{
		{path: "/plugins/backups/export", handler: ExportBackup},
		{path: "/plugins/backups/export-all", handler: ExportAllBackups},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("role", "guest")
				c.Next()
			})
			router.POST(test.path, test.handler)
			body := url.Values{"name": {"secret-test"}}.Encode()
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(body))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
			}
		})
	}
}
