package controller

import (
	"encoding/json"
	"l4d2-manager-next/logic"
	"net/http"
	"testing"
)

func TestMapCleanupConfigHTTPPermissionsAndValidation(t *testing.T) {
	setupDiskSpaceControllerTest(t)
	for _, role := range []string{"guest", "map_uploader"} {
		context, recorder := newJSONControllerContext("/maps/cleanup/config", role, `{}`)
		GetMapCleanupConfig(context)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s read config status=%d", role, recorder.Code)
		}
		context, recorder = newJSONControllerContext("/config/map-cleanup", role, `{"enabled":true,"retention_days":3}`)
		SetMapCleanupConfig(context)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s update config status=%d", role, recorder.Code)
		}
	}
	for _, body := range []string{
		`{}`, `{"enabled":true}`, `{"retention_days":3}`,
		`{"enabled":true,"retention_days":0}`, `{"enabled":true,"retention_days":-1}`,
		`{"enabled":true,"retention_days":3651}`, `{"enabled":true,"retention_days":1.5}`,
	} {
		context, recorder := newJSONControllerContext("/config/map-cleanup", "admin", body)
		SetMapCleanupConfig(context)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid config %s status=%d body=%s", body, recorder.Code, recorder.Body.String())
		}
	}
}

func TestMapCleanupConfigHTTPSavesAndReportsSchedule(t *testing.T) {
	setupDiskSpaceControllerTest(t)
	context, recorder := newJSONControllerContext("/maps/cleanup/config", "admin", `{}`)
	GetMapCleanupConfig(context)
	var config logic.MapCleanupConfig
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &config) != nil || config.Enabled || config.RetentionDays != 3 {
		t.Fatalf("default config status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	context, recorder = newJSONControllerContext("/config/map-cleanup", "admin", `{"enabled":true,"retention_days":5}`)
	SetMapCleanupConfig(context)
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &config) != nil || !config.Enabled || config.RetentionDays != 5 {
		t.Fatalf("saved config status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil || payload["check_time"] != "03:00" || payload["timezone"] == "" || payload["next_check_at"] == nil {
		t.Fatalf("schedule information missing: %s", recorder.Body.String())
	}
	logic.LoadManagerConfig()
	context, recorder = newJSONControllerContext("/maps/cleanup/config", "admin", `{}`)
	GetMapCleanupConfig(context)
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &config) != nil || !config.Enabled || config.RetentionDays != 5 {
		t.Fatalf("reloaded config status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
