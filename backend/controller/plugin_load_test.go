package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/model"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPluginLoadEndpointsClassifyResults(t *testing.T) {
	for _, endpoint := range []struct {
		name    string
		handler gin.HandlerFunc
		set     func(func(string) error)
	}{
		{"load", LoadPlugin, func(fn func(string) error) { loadPlugin = fn }},
		{"enable-and-load", EnableAndLoadPlugin, func(fn func(string) error) { enableAndLoadPlugin = fn }},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			originalLoad, originalEnable, originalAudit := loadPlugin, enableAndLoadPlugin, enqueueAuditLog
			t.Cleanup(func() {
				loadPlugin, enableAndLoadPlugin, enqueueAuditLog = originalLoad, originalEnable, originalAudit
			})
			for _, tc := range []struct {
				name   string
				err    error
				status int
				code   string
			}{
				{"success", nil, http.StatusOK, ""},
				{"unknown", fmt.Errorf("postflight: %w", logic.ErrPluginResultUnknown), http.StatusBadGateway, "plugin_result_unknown"},
				{"busy", fmt.Errorf("load: %w", logic.ErrPluginOperationBusy), http.StatusConflict, "plugin_operation_busy"},
				{"failure", errors.New("target plugin failed to load"), http.StatusInternalServerError, ""},
			} {
				t.Run(tc.name, func(t *testing.T) {
					calls := 0
					endpoint.set(func(name string) error {
						calls++
						if name != "Example" {
							t.Fatalf("plugin name = %q", name)
						}
						return tc.err
					})
					var audit []model.AuditLog
					enqueueAuditLog = func(entry model.AuditLog) { audit = append(audit, entry) }
					recorder := pluginLoadRequest(endpoint.handler, "admin", "Example")
					if recorder.Code != tc.status {
						t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.status, recorder.Body.String())
					}
					if calls != 1 {
						t.Fatalf("logic calls = %d, want 1", calls)
					}
					if tc.code != "" {
						var body struct {
							Error   string `json:"error"`
							Message string `json:"message"`
						}
						if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
							t.Fatal(err)
						}
						if body.Error != tc.code || body.Message == "" || strings.Contains(body.Message, "加载成功") {
							t.Fatalf("response = %#v, want code %s and a non-success message", body, tc.code)
						}
					}
					if len(audit) != 1 || audit[0].Success != (tc.err == nil) {
						t.Fatalf("audit = %#v; unknown and busy must not be recorded as successful", audit)
					}
				})
			}
		})
	}
}

func TestPluginLoadEndpointsValidateBeforeLogic(t *testing.T) {
	originalLoad, originalEnable, originalAudit := loadPlugin, enableAndLoadPlugin, enqueueAuditLog
	t.Cleanup(func() {
		loadPlugin, enableAndLoadPlugin, enqueueAuditLog = originalLoad, originalEnable, originalAudit
	})
	calls := 0
	stub := func(string) error { calls++; return nil }
	loadPlugin, enableAndLoadPlugin = stub, stub
	enqueueAuditLog = func(model.AuditLog) {}
	for _, handler := range []gin.HandlerFunc{LoadPlugin, EnableAndLoadPlugin} {
		for _, tc := range []struct {
			role, name string
			status     int
		}{
			{"guest", "Example", http.StatusForbidden},
			{"", "Example", http.StatusForbidden},
			{"admin", "", http.StatusBadRequest},
		} {
			recorder := pluginLoadRequest(handler, tc.role, tc.name)
			if recorder.Code != tc.status {
				t.Fatalf("role=%q name=%q: status=%d want=%d", tc.role, tc.name, recorder.Code, tc.status)
			}
		}
	}
	if calls != 0 {
		t.Fatalf("logic calls = %d, want none for rejected requests", calls)
	}
}

func pluginLoadRequest(handler gin.HandlerFunc, role, name string) *httptest.ResponseRecorder {
	router := gin.New()
	router.POST("/plugins/load", func(c *gin.Context) {
		c.Set("role", role)
		handler(c)
	})
	request := httptest.NewRequest(http.MethodPost, "/plugins/load", strings.NewReader(url.Values{"name": {name}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.RemoteAddr = "198.51.100.20:45000"
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}
