package middlewares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func signAuthTestToken(t *testing.T, privateKey []byte, claims jwt.Claims, method jwt.SigningMethod) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func runAuthTestRequest(t *testing.T, privateKey []byte, method, path, credential string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	role := ""
	router := gin.New()
	router.Handle(method, path, Auth(privateKey), func(c *gin.Context) {
		roleValue, _ := c.Get("role")
		role, _ = roleValue.(string)
		c.JSON(http.StatusOK, gin.H{"role": role})
	})

	request := httptest.NewRequest(method, path, nil)
	request.RemoteAddr = "192.0.2.10:12345"
	request.Header.Set("Authorization", "Bearer "+credential)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response, role
}

func TestAuthRecognizesAdministratorAndTemporaryTokenRoles(t *testing.T) {
	privateKey := []byte("test-private-key")
	t.Setenv("L4D2_MANAGER_PASSWORD", "test-admin-password")
	expiresAt := jwt.NewNumericDate(time.Now().Add(time.Hour))

	tests := []struct {
		name       string
		credential string
		wantRole   string
	}{
		{
			name:       "administrator password",
			credential: "test-admin-password",
			wantRole:   RoleAdmin,
		},
		{
			name: "legacy token",
			credential: signAuthTestToken(t, privateKey, jwt.RegisteredClaims{
				ExpiresAt: expiresAt,
			}, jwt.SigningMethodHS256),
			wantRole: RoleGuest,
		},
		{
			name: "temporary token with explicit false flag",
			credential: signAuthTestToken(t, privateKey, TempAuthClaims{
				MapUploadOnly: false,
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: expiresAt,
				},
			}, jwt.SigningMethodHS256),
			wantRole: RoleGuest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, role := runAuthTestRequest(t, privateKey, http.MethodPost, "/list", test.credential)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
			}
			if role != test.wantRole {
				t.Fatalf("role = %q, want %q", role, test.wantRole)
			}
		})
	}
}

func TestMapUploaderTokenAllowsOnlyExplicitRequests(t *testing.T) {
	privateKey := []byte("test-private-key")
	t.Setenv("L4D2_MANAGER_PASSWORD", "test-admin-password")
	token := signAuthTestToken(t, privateKey, TempAuthClaims{
		MapUploadOnly: true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}, jwt.SigningMethodHS256)

	allowed := []string{
		"/auth",
		"/upload/init",
		"/upload/chunk",
		"/upload/status",
		"/upload/merge",
		"/upload/cancel",
		"/maps/hot-reload",
		"/maps/hot-reload/status",
	}
	for _, path := range allowed {
		t.Run("allows "+path, func(t *testing.T) {
			response, role := runAuthTestRequest(t, privateKey, http.MethodPost, path, token)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
			}
			if role != RoleMapUploader {
				t.Fatalf("role = %q, want %q", role, RoleMapUploader)
			}
		})
	}

	denied := []string{
		"/upload",
		"/list",
		"/clear",
		"/remove",
		"/maps/hot-reload/config",
		"/maps/hot-reload/config/update",
		"/rcon/getstatus",
		"/download/list",
		"/plugins/list",
		"/auth/getTempAuthCode",
	}
	for _, path := range denied {
		t.Run("denies "+path, func(t *testing.T) {
			response, _ := runAuthTestRequest(t, privateKey, http.MethodPost, path, token)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusForbidden, response.Body.String())
			}
		})
	}

	t.Run("requires exact HTTP method", func(t *testing.T) {
		response, _ := runAuthTestRequest(t, privateKey, http.MethodGet, "/upload/init", token)
		if response.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
		}
	})
}

func TestAuthRejectsExpiredAndNonHS256Tokens(t *testing.T) {
	privateKey := []byte("test-private-key")
	t.Setenv("L4D2_MANAGER_PASSWORD", "test-admin-password")

	tests := []struct {
		name  string
		token string
	}{
		{
			name: "expired",
			token: signAuthTestToken(t, privateKey, TempAuthClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
				},
			}, jwt.SigningMethodHS256),
		},
		{
			name: "HS384",
			token: signAuthTestToken(t, privateKey, TempAuthClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
			}, jwt.SigningMethodHS384),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, _ := runAuthTestRequest(t, privateKey, http.MethodPost, "/list", test.token)
			if response.Code != http.StatusUnauthorized {
				var payload map[string]any
				_ = json.Unmarshal(response.Body.Bytes(), &payload)
				t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusUnauthorized, response.Body.String())
			}
		})
	}
}
