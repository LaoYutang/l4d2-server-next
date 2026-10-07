package middlewares

import (
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"l4d2-manager-next/logic"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func setupAuthTestStore(t *testing.T) *logic.AuthCodeStore {
	t.Helper()
	t.Setenv("L4D2_MANAGER_PASSWORD", "test-admin-password")
	dir := t.TempDir()
	store, err := logic.OpenAuthCodeStore(filepath.Join(dir, "auth.db"), filepath.Join(dir, "auth.key"), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	previous := logic.SetAuthCodeStore(store)
	mutex.Lock()
	ipAttempts = make(map[string]*loginAttempt)
	mutex.Unlock()
	t.Cleanup(func() { logic.SetAuthCodeStore(previous); store.Close() })
	return store
}

func runAuthTestRequest(t *testing.T, method, path, credential string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	role := ""
	router := gin.New()
	router.Handle(method, path, Auth(), func(c *gin.Context) { role = c.GetString("role"); c.JSON(200, gin.H{"role": role}) })
	request := httptest.NewRequest(method, path, nil)
	request.RemoteAddr = "192.0.2.10:12345"
	request.Header.Set("Authorization", "Bearer "+credential)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response, role
}

func TestAuthRecognizesAdministratorAndDirectAuthorizationCodes(t *testing.T) {
	store := setupAuthTestStore(t)
	created, err := store.Create(logic.CreateAuthCodeRequest{Code: "directCode"})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ code, role string }{{"test-admin-password", RoleAdmin}, {created.Code, RoleGuest}} {
		response, role := runAuthTestRequest(t, "POST", "/list", test.code)
		if response.Code != 200 || role != test.role {
			t.Fatalf("status = %d, role = %s", response.Code, role)
		}
	}
	past := time.Now().Add(-time.Hour)
	if _, err := store.Update(logic.UpdateAuthCodeRequest{ID: created.ID, ExpiresAt: &past}); err != nil {
		t.Fatal(err)
	}
	response, _ := runAuthTestRequest(t, "POST", "/list", created.Code)
	if response.Code != 401 {
		t.Fatal("expired code accepted")
	}
	future := time.Now().Add(90 * 24 * time.Hour)
	if _, err := store.Update(logic.UpdateAuthCodeRequest{ID: created.ID, ExpiresAt: &future}); err != nil {
		t.Fatal(err)
	}
	response, _ = runAuthTestRequest(t, "POST", "/list", created.Code)
	if response.Code != 200 {
		t.Fatal("extension did not restore access")
	}
	if _, err := store.Revoke(created.ID); err != nil {
		t.Fatal(err)
	}
	response, _ = runAuthTestRequest(t, "POST", "/list", created.Code)
	if response.Code != 401 {
		t.Fatal("revoked code accepted")
	}
	if _, err := store.Delete(created.ID); err != nil {
		t.Fatal(err)
	}
	response, _ = runAuthTestRequest(t, "POST", "/list", created.Code)
	if response.Code != 401 {
		t.Fatal("deleted code accepted")
	}
}

func TestMapUploaderCodeAllowsOnlyExplicitRequests(t *testing.T) {
	store := setupAuthTestStore(t)
	created, err := store.Create(logic.CreateAuthCodeRequest{AccessType: logic.AuthAccessMapUpload})
	if err != nil {
		t.Fatal(err)
	}
	allowed := []string{"/auth", "/upload/init", "/upload/chunk", "/upload/status", "/upload/merge", "/upload/cancel", "/maps/hot-reload", "/maps/hot-reload/status", "/download/add", "/download/list", "/download/cancel", "/download/restart", "/download/clear", "/download/link/parse"}
	for _, path := range allowed {
		response, role := runAuthTestRequest(t, "POST", path, created.Code)
		if response.Code != 200 || role != RoleMapUploader {
			t.Fatalf("allowed %s: %d %s", path, response.Code, role)
		}
	}
	denied := []string{"/list", "/clear", "/remove", "/maps/hot-reload/config", "/maps/queue/add", "/maps/queue/snapshot", "/rcon/getstatus", "/download/config", "/download/config/update", "/plugins/list", "/auth-codes/list", "/auth-codes/create"}
	for _, path := range denied {
		response, _ := runAuthTestRequest(t, "POST", path, created.Code)
		if response.Code != 403 {
			t.Fatalf("denied %s: %d", path, response.Code)
		}
	}
	response, _ := runAuthTestRequest(t, "GET", "/upload/init", created.Code)
	if response.Code != 403 {
		t.Fatal("wrong method accepted")
	}
}

func TestAuthRejectsAllLegacyJWTsAndPreservesPasswordWhenStoreUnavailable(t *testing.T) {
	setupAuthTestStore(t)
	for _, method := range []jwt.SigningMethod{jwt.SigningMethodHS256, jwt.SigningMethodHS384} {
		for _, mapOnly := range []bool{false, true} {
			legacy, err := jwt.NewWithClaims(method, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix(), "map_upload_only": mapOnly}).SignedString([]byte("old-valid-key"))
			if err != nil {
				t.Fatal(err)
			}
			response, _ := runAuthTestRequest(t, "POST", "/auth", legacy)
			if response.Code != 401 {
				t.Fatalf("legacy token accepted: %d", response.Code)
			}
		}
	}
	previous := logic.SetAuthCodeStore(nil)
	defer logic.SetAuthCodeStore(previous)
	response, role := runAuthTestRequest(t, "POST", "/auth", "test-admin-password")
	if response.Code != 200 || role != RoleAdmin {
		t.Fatal("administrator unavailable")
	}
	response, _ = runAuthTestRequest(t, "POST", "/auth", "unknown-code")
	if response.Code != 503 {
		t.Fatalf("unavailable store = %d", response.Code)
	}
}

func TestAuthRejectsMissingBearerAndRateLimitsFailures(t *testing.T) {
	setupAuthTestStore(t)
	for attempt := 0; attempt < 11; attempt++ {
		response, _ := runAuthTestRequest(t, "POST", "/auth", "wrong-code")
		want := 401
		if attempt == 10 {
			want = 429
		}
		if response.Code != want {
			t.Fatalf("attempt %d: status %d, want %d", attempt, response.Code, want)
		}
	}
}
