package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"l4d2-manager-next/logic"
	"l4d2-manager-next/model"

	"github.com/gin-gonic/gin"
)

func setupAuthControllerStore(t *testing.T) *logic.AuthCodeStore {
	t.Helper()
	t.Setenv("L4D2_MANAGER_PASSWORD", "test-admin-password")
	dir := t.TempDir()
	store, err := logic.OpenAuthCodeStore(filepath.Join(dir, "auth.db"), filepath.Join(dir, "auth.key"), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	previous := logic.SetAuthCodeStore(store)
	previousAudit := enqueueAuditLog
	enqueueAuditLog = func(model.AuditLog) {}
	t.Cleanup(func() { logic.SetAuthCodeStore(previous); store.Close(); enqueueAuditLog = previousAudit })
	return store
}

func authJSONContext(role string, payload any) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	body, _ := json.Marshal(payload)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest("POST", "/auth-codes/test", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.RemoteAddr = "192.0.2.20:1234"
	c.Set("role", role)
	return c, response
}

func TestAuthCodeManagementRequiresAdmin(t *testing.T) {
	setupAuthControllerStore(t)
	for _, role := range []string{"guest", "map_uploader"} {
		for _, handler := range []gin.HandlerFunc{ListAuthCodes, CreateAuthCode, UpdateAuthCode, RevokeAuthCode, DeleteAuthCode, CleanupExpiredAuthCodes, SetSelfServiceConfig} {
			c, response := authJSONContext(role, map[string]any{})
			handler(c)
			if response.Code != http.StatusForbidden {
				t.Fatalf("role %s: status %d", role, response.Code)
			}
		}
	}
}

func TestAuthCodeControllerLifecycleAndAudit(t *testing.T) {
	setupAuthControllerStore(t)
	var logs []model.AuditLog
	enqueueAuditLog = func(entry model.AuditLog) { logs = append(logs, entry) }
	c, response := authJSONContext("admin", map[string]any{"code": "customCode8", "remark": "玩家上传", "access_type": "map_upload_only"})
	CreateAuthCode(c)
	if response.Code != 200 {
		t.Fatalf("create: %d %s", response.Code, response.Body.String())
	}
	var created logic.CreatedAuthCode
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Code != "customCode8" || created.Remark != "玩家上传" {
		t.Fatalf("creation: %+v", created)
	}
	if len(logs) != 1 || logs[0].AuthCodeID != created.ID || strings.Contains(logs[0].Detail, created.Code) {
		t.Fatal("incorrect audit")
	}
	c, response = authJSONContext("admin", map[string]any{"id": created.ID, "expires_at": time.Now().Add(-time.Hour)})
	UpdateAuthCode(c)
	if response.Code != 200 {
		t.Fatalf("update: %s", response.Body.String())
	}
	c, response = authJSONContext("admin", map[string]any{"keyword": "玩家"})
	ListAuthCodes(c)
	if response.Code != 200 || strings.Contains(response.Body.String(), created.Code) || !strings.Contains(response.Body.String(), "expired") {
		t.Fatalf("list: %s", response.Body.String())
	}
	c, response = authJSONContext("admin", map[string]any{})
	CleanupExpiredAuthCodes(c)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"deleted_count":1`) {
		t.Fatalf("cleanup: %s", response.Body.String())
	}
	if len(logs) != 3 {
		t.Fatalf("audit records lost: %d", len(logs))
	}
}

func TestAuthReturnsRoleAndExpiryWithoutAnotherToken(t *testing.T) {
	store := setupAuthControllerStore(t)
	created, err := store.Create(logic.CreateAuthCodeRequest{Code: "directCredential"})
	if err != nil {
		t.Fatal(err)
	}
	c, response := authJSONContext("guest", nil)
	c.Set("auth_code_id", created.ID)
	c.Set("auth_code_remark", created.Remark)
	Auth(c)
	if response.Code != 200 {
		t.Fatalf("login: %s", response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["role"] != "guest" || payload["expires_at"] == nil || payload["token"] != nil || payload["code"] != nil {
		t.Fatalf("unexpected login response: %v", payload)
	}
	listed, err := store.List(logic.AuthCodeListFilter{})
	if err != nil || listed.Items[0].LoginCount != 1 {
		t.Fatalf("login not recorded: %+v %v", listed, err)
	}
}

func TestAuthCodeControllerRejectsMalformedExpiryWithoutLeakingCredential(t *testing.T) {
	setupAuthControllerStore(t)
	c, response := authJSONContext("admin", map[string]any{"code": "secretCode", "expires_at": "wrong"})
	CreateAuthCode(c)
	if response.Code != 400 || strings.Contains(response.Body.String(), "secretCode") {
		t.Fatalf("malformed expiry: %s", response.Body.String())
	}
}
