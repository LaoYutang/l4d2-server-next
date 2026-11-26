package controller

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/middlewares"

	"github.com/gin-gonic/gin"
)

func selfServiceTestRouter() *gin.Engine {
	router := gin.New()
	router.POST("/self-service/status", GetSelfServiceStatus)
	router.POST("/self-service/generate", GenerateSelfServiceCode)
	router.POST("/config/self-service", middlewares.Auth(), SetSelfServiceConfig)
	router.POST("/auth", middlewares.Auth(), Auth)
	router.POST("/upload/init", middlewares.Auth(), UploadInit)
	router.POST("/download/add", middlewares.Auth(), AddDownloadTask)
	router.POST("/download/list", middlewares.Auth(), GetDownloadTasksInfo)
	router.POST("/download/cancel", middlewares.Auth(), CancelDownloadTask)
	router.POST("/upload/chunk", middlewares.Auth(), UploadChunk)
	router.POST("/upload/status", middlewares.Auth(), UploadStatus)
	router.POST("/upload/cancel", middlewares.Auth(), UploadCancel)
	router.POST("/admins/list", middlewares.Auth(), GetAdmins)
	router.POST("/plugins/list", middlewares.Auth(), GetPlugins)
	router.POST("/maps/hot-reload", middlewares.Auth(), HotReloadMaps)
	router.POST("/maps/hot-reload/status", middlewares.Auth(), GetMapHotReloadStatus)
	router.POST("/maps/hot-reload/config/update", middlewares.Auth(), SetMapHotReloadConfig)
	return router
}

func postSelfServiceJSON(t *testing.T, router http.Handler, path, credential, body string, expected int, result any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	checkSelfServiceResponse(t, router, req, credential, expected, result)
}

func checkSelfServiceResponse(t *testing.T, router http.Handler, req *http.Request, credential string, expected int, result any) {
	t.Helper()
	path := req.URL.Path
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != expected {
		t.Fatalf("%s: status %d, want %d", path, response.Code, expected)
	}
	if result != nil {
		if err := json.Unmarshal(response.Body.Bytes(), result); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
}

func TestSelfServiceConfigCompatibility(t *testing.T) {
	setupAuthControllerStore(t)
	setupDiskSpaceControllerTest(t)
	router := selfServiceTestRouter()
	check := func(enabled bool, accessType string) {
		t.Helper()
		var status struct {
			Enabled    bool   `json:"enabled"`
			AccessType string `json:"access_type"`
		}
		postSelfServiceJSON(t, router, "/self-service/status", "", "", 200, &status)
		if status.Enabled != enabled || status.AccessType != accessType {
			t.Fatalf("enabled/access_type = %t/%q, want %t/%q", status.Enabled, status.AccessType, enabled, accessType)
		}
	}
	check(false, logic.AuthAccessTemporary)
	if err := consts.EnsureManagerDataPath(); err != nil {
		t.Fatal(err)
	}
	for _, config := range []struct {
		json    string
		enabled bool
	}{
		{`{"enable_self_service":true,"self_service_access_type":null}`, false},
		{`{"enable_self_service":true,"self_service_access_type":"unknown"}`, false},
		{`{"enable_self_service":true,"self_service_access_type":"admin"}`, false},
		{`{"enable_self_service":true}`, true},
	} {
		if err := os.WriteFile(consts.ManagerConfigPath, []byte(config.json), 0600); err != nil {
			t.Fatal(err)
		}
		logic.LoadManagerConfig()
		check(config.enabled, logic.AuthAccessTemporary)
		if !config.enabled {
			postSelfServiceJSON(t, router, "/self-service/generate", "", "", 403, nil)
		}
	}

	admin := logic.AdministratorPassword()
	postSelfServiceJSON(t, router, "/config/self-service", "", `{"enable":false}`, 401, nil)
	postSelfServiceJSON(t, router, "/config/self-service", admin, `{"access_type":"map_upload_only"}`, 200, nil)
	check(true, logic.AuthAccessMapUpload)
	postSelfServiceJSON(t, router, "/config/self-service", admin, `{"enable":false}`, 200, nil)
	raw, err := os.ReadFile(consts.ManagerConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var stored struct {
		AccessType string `json:"self_service_access_type"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil || stored.AccessType != logic.AuthAccessMapUpload {
		t.Fatalf("stored self_service_access_type = %q, decode error %v", stored.AccessType, err)
	}
	logic.LoadManagerConfig()
	check(false, logic.AuthAccessMapUpload)
	postSelfServiceJSON(t, router, "/self-service/generate", "", "", 403, nil)
	postSelfServiceJSON(t, router, "/config/self-service", admin, `{"enable":true,"access_type":"unknown"}`, 400, nil)
	postSelfServiceJSON(t, router, "/config/self-service", admin, `{}`, 400, nil)
	postSelfServiceJSON(t, router, "/config/self-service", admin, `{"enable":true,"access_type":null}`, 400, nil)
	check(false, logic.AuthAccessMapUpload)
	postSelfServiceJSON(t, router, "/config/self-service", admin, `{"enable":null,"access_type":"temporary"}`, 400, nil)
	check(false, logic.AuthAccessMapUpload)
}

func TestSelfServiceIssuanceFollowsAdminSelection(t *testing.T) {
	store := setupAuthControllerStore(t)
	setupDiskSpaceControllerTest(t)
	router := selfServiceTestRouter()
	admin := logic.AdministratorPassword()
	configure := func(accessType string) {
		t.Helper()
		postSelfServiceJSON(t, router, "/config/self-service", admin, `{"enable":true,"access_type":"`+accessType+`"}`, 200, nil)
	}
	claim := func(requested, expected string) logic.CreatedAuthCode {
		t.Helper()
		var grant logic.CreatedAuthCode
		postSelfServiceJSON(t, router, "/self-service/generate", "", `{"access_type":"`+requested+`"}`, 200, &grant)
		if grant.AccessType != expected || grant.Source != "self_service" || grant.ExpiresAt.Sub(grant.CreatedAt) != time.Hour {
			t.Fatalf("issued type %q, source %q, duration %s", grant.AccessType, grant.Source, grant.ExpiresAt.Sub(grant.CreatedAt))
		}
		return grant
	}
	login := func(grant logic.CreatedAuthCode, wantRole string) {
		t.Helper()
		var result struct {
			Role      string    `json:"role"`
			ExpiresAt time.Time `json:"expires_at"`
		}
		postSelfServiceJSON(t, router, "/auth", grant.Code, "", 200, &result)
		if result.Role != wantRole || !result.ExpiresAt.Equal(grant.ExpiresAt) {
			t.Fatalf("login role/expiry = %q/%s, want %q/%s", result.Role, result.ExpiresAt, wantRole, grant.ExpiresAt)
		}
	}
	configure(logic.AuthAccessTemporary)
	temporary := claim(logic.AuthAccessMapUpload, logic.AuthAccessTemporary)
	configure(logic.AuthAccessMapUpload)
	manual, err := store.Create(logic.CreateAuthCodeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if manual.Source != "manual" || manual.AccessType != logic.AuthAccessTemporary {
		t.Fatal("self-service setting changed the default for manually issued codes")
	}
	login(temporary, middlewares.RoleGuest)
	postSelfServiceJSON(t, router, "/config/self-service", temporary.Code, `{"enable":false}`, 403, nil)
	postSelfServiceJSON(t, router, "/self-service/generate", "", "", 429, nil)
	if _, err := store.Revoke(temporary.ID); err != nil {
		t.Fatal(err)
	}
	maps := claim(logic.AuthAccessTemporary, logic.AuthAccessMapUpload)
	configure(logic.AuthAccessTemporary)
	postSelfServiceJSON(t, router, "/auth", temporary.Code, "", 401, nil)
	revoked, err := store.List(logic.AuthCodeListFilter{Status: "revoked", Source: "self_service"})
	if err != nil || len(revoked.Items) != 1 {
		t.Fatalf("expected one revoked self-service record, got %d: %v", len(revoked.Items), err)
	}
	old := revoked.Items[0]
	if old.ID != temporary.ID || old.AccessType != temporary.AccessType || !old.ExpiresAt.Equal(temporary.ExpiresAt) {
		t.Fatal("switching the issuance setting changed a revoked authorization")
	}
	login(maps, middlewares.RoleMapUploader)
	postSelfServiceJSON(t, router, "/config/self-service", maps.Code, `{"enable":false}`, 403, nil)
	postSelfServiceJSON(t, router, "/self-service/generate", "", "", 429, nil)

}

func TestSelfServiceMapCodeUsesExistingPermissions(t *testing.T) {
	store := setupAuthControllerStore(t)
	setupChunkUploadTestPaths(t)
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{Total: 100 << 30, Free: 90 << 30, UsedPercent: 10})
	downloader := setupOwnedDownloaderTest(t)
	router := selfServiceTestRouter()
	admin := logic.AdministratorPassword()
	postSelfServiceJSON(t, router, "/config/self-service", admin, `{"enable":true,"access_type":"map_upload_only"}`, 200, nil)
	var grant logic.CreatedAuthCode
	postSelfServiceJSON(t, router, "/self-service/generate", "", "", 200, &grant)
	for _, path := range []string{"/admins/list", "/plugins/list"} {
		postSelfServiceJSON(t, router, path, grant.Code, "", 403, nil)
	}

	t.Run("upload and download ownership", func(t *testing.T) {
		postForm := func(path, code string, fields url.Values, want int, result any) {
			t.Helper()
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(fields.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			checkSelfServiceResponse(t, router, req, code, want, result)
		}
		other := createMapDownloadCode(t, store)
		for _, owner := range []logic.CreatedAuthCode{grant, other} {
			var upload struct {
				ID string `json:"uploadId"`
			}
			postForm("/upload/init", owner.Code, url.Values{"filename": {"sample.vpk"}, "fileSize": {"1"}, "totalChunks": {"1"}}, 200, &upload)
			fields := url.Values{"uploadId": {upload.ID}}
			if owner.ID != grant.ID {
				postForm("/upload/cancel", grant.Code, fields, 403, nil)
			}
			req, err := newChunkUploadRequest(upload.ID, 0, 1)
			if err != nil {
				t.Fatal(err)
			}
			checkSelfServiceResponse(t, router, req, owner.Code, 200, nil)
			var status struct {
				UploadedChunks []int `json:"uploadedChunks"`
			}
			postForm("/upload/status", owner.Code, fields, 200, &status)
			if len(status.UploadedChunks) != 1 || status.UploadedChunks[0] != 0 {
				t.Fatal("owner could not complete a chunk on the original upload")
			}
			postForm("/upload/cancel", owner.Code, fields, 200, nil)
			postForm("/download/add", owner.Code, url.Values{"url": {"https://example.invalid/sample.vpk"}}, 200, nil)
		}
		if len(downloader.tasks) != 2 || downloader.tasks[0].authCodeID != grant.ID || downloader.tasks[1].authCodeID != other.ID {
			t.Fatal("downloads did not keep their respective owners")
		}
		var listed []struct {
			ID string `json:"id"`
		}
		postForm("/download/list", grant.Code, nil, 200, &listed)
		if len(listed) != 1 || listed[0].ID != downloader.tasks[0].id {
			t.Fatal("self-service map code must see only its own download")
		}
		fields := url.Values{"id": {downloader.tasks[1].id}}
		postForm("/download/cancel", grant.Code, fields, 403, nil)
		select {
		case <-downloader.tasks[1].cancel:
			t.Fatal("denied request cancelled the other owner's download")
		default:
		}
		postForm("/download/cancel", other.Code, fields, 200, nil)
	})

	t.Run("administrator controls the hot reload command", func(t *testing.T) {
		// The existing loopback RCON fixture accepts this harmless read-only command.
		const command = "sm plugins list"
		postSelfServiceJSON(t, router, "/maps/hot-reload/config/update", admin, `{"command":"sm plugins list"}`, 200, nil)
		postSelfServiceJSON(t, router, "/maps/hot-reload/config/update", grant.Code, `{"command":"sm_update_vpk"}`, 403, nil)
		if logic.GetMapHotReloadCommand() != command {
			t.Fatal("map code changed the configured command")
		}
		var status map[string]any
		postSelfServiceJSON(t, router, "/maps/hot-reload/status", grant.Code, "", 200, &status)
		if _, disclosed := status["command"]; disclosed || status["using_default"] != false {
			t.Fatal("hot reload status must report the custom setting without exposing its command")
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { listener.Close() })
		t.Setenv("L4D2_RCON_URL", listener.Addr().String())
		t.Setenv("L4D2_RCON_PASSWORD", "test-password")
		done := make(chan error, 1)
		go func() { done <- serveConsoleRCON(listener, "ok") }()
		postSelfServiceJSON(t, router, "/maps/hot-reload", grant.Code, "", 200, nil)
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("configured command did not reach the test RCON server")
		}
	})
}
