package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetMapMissionDetailRejectsMissingMapName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodPost, "/maps/detail", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	GetMapMissionDetail(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %q", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "地图名称不能为空") {
		t.Fatalf("body = %q, want missing map name message", w.Body.String())
	}
}
