package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUpdateMapGlobalScriptRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodPost,
		"/maps/inspection/global-scripts/update",
		strings.NewReader(`{"map":"test.vpk","path":"scripts/vscripts/mapspawn_addon.nut","content":"test","encoding":"utf-8","expected_revision":"revision"}`),
	)
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("role", "guest")

	UpdateMapGlobalScript(context)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("guest update status = %d, want %d; body = %q", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}
