package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"l4d2-manager-next/middlewares"
	"l4d2-manager-next/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func runGetTempAuthCodeTest(t *testing.T, role string, privateKey []byte, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/auth/getTempAuthCode", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = request
	context.Set("role", role)
	context.Set("privateKey", privateKey)
	GetTempAuthCode(context)
	return response
}

func parseGeneratedTempToken(t *testing.T, tokenString string, privateKey []byte) *middlewares.TempAuthClaims {
	t.Helper()
	claims := &middlewares.TempAuthClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return privateKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		t.Fatalf("parse generated token: %v", err)
	}
	return claims
}

func TestGetTempAuthCodeGeneratesSelectedAccessType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	privateKey := []byte("test-private-key")
	oldEnqueueAuditLog := enqueueAuditLog
	enqueueAuditLog = func(model.AuditLog) {}
	t.Cleanup(func() { enqueueAuditLog = oldEnqueueAuditLog })

	tests := []struct {
		name              string
		accessType        string
		wantMapUploadOnly bool
	}{
		{name: "missing defaults to temporary", wantMapUploadOnly: false},
		{name: "temporary", accessType: tempAccessTypeTemporary, wantMapUploadOnly: false},
		{name: "map uploader", accessType: tempAccessTypeMapUploader, wantMapUploadOnly: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values := url.Values{"expired": {"6"}}
			if test.accessType != "" {
				values.Set("access_type", test.accessType)
			}
			response := runGetTempAuthCodeTest(t, middlewares.RoleAdmin, privateKey, values)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
			}
			claims := parseGeneratedTempToken(t, response.Body.String(), privateKey)
			if claims.MapUploadOnly != test.wantMapUploadOnly {
				t.Fatalf("map_upload_only = %v, want %v", claims.MapUploadOnly, test.wantMapUploadOnly)
			}
		})
	}
}

func TestGetTempAuthCodeValidatesRoleAccessTypeAndExpiration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	privateKey := []byte("test-private-key")
	oldEnqueueAuditLog := enqueueAuditLog
	enqueueAuditLog = func(model.AuditLog) {}
	t.Cleanup(func() { enqueueAuditLog = oldEnqueueAuditLog })

	tests := []struct {
		name       string
		role       string
		values     url.Values
		wantStatus int
	}{
		{
			name:       "guest cannot generate",
			role:       middlewares.RoleGuest,
			values:     url.Values{"expired": {"1"}},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "unknown access type",
			role:       middlewares.RoleAdmin,
			values:     url.Values{"expired": {"1"}, "access_type": {"unknown"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non numeric expiration",
			role:       middlewares.RoleAdmin,
			values:     url.Values{"expired": {"invalid"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "thirty day expiration",
			role:       middlewares.RoleAdmin,
			values:     url.Values{"expired": {"720"}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "expiration above thirty days",
			role:       middlewares.RoleAdmin,
			values:     url.Values{"expired": {"721"}},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := runGetTempAuthCodeTest(t, test.role, privateKey, test.values)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}
