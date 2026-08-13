package controller

import (
	"bytes"
	"encoding/json"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/model"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSourceModLogManagementRequiresAdmin(t *testing.T) {
	logsDir := setupSourceModLogControllerTestDir(t)
	logPath := filepath.Join(logsDir, "old.log")
	if err := os.WriteFile(logPath, []byte("old"), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	previewContext, previewRecorder := newSourceModLogJSONContext(
		"guest",
		"/logs/cleanup/preview",
		map[string]any{"categories": []string{"other"}, "retention_days": 0},
	)
	PreviewSourceModLogCleanup(previewContext)
	if previewRecorder.Code != http.StatusForbidden {
		t.Fatalf("preview status = %d, want 403", previewRecorder.Code)
	}

	deleteContext, deleteRecorder := newSourceModLogJSONContext(
		"guest",
		"/logs/delete",
		map[string]any{"files": []map[string]string{{"name": "old.log", "version": "0:0"}}},
	)
	DeleteSourceModLogs(deleteContext)
	if deleteRecorder.Code != http.StatusForbidden {
		t.Fatalf("delete status = %d, want 403", deleteRecorder.Code)
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("guest changed log file: %v", err)
	}
}

func TestPreviewAndDeleteSourceModLogsAsAdmin(t *testing.T) {
	logsDir := setupSourceModLogControllerTestDir(t)
	now := time.Now()
	logPath := filepath.Join(logsDir, "old.log")
	if err := os.WriteFile(logPath, []byte("old log"), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	oldTime := now.Add(-90 * 24 * time.Hour)
	if err := os.Chtimes(logPath, oldTime, oldTime); err != nil {
		t.Fatalf("set log time: %v", err)
	}

	previewContext, previewRecorder := newSourceModLogJSONContext(
		"admin",
		"/logs/cleanup/preview",
		map[string]any{"categories": []string{"other"}, "retention_days": 30},
	)
	PreviewSourceModLogCleanup(previewContext)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body = %q", previewRecorder.Code, previewRecorder.Body.String())
	}
	var preview logic.SourceModLogCleanupPreview
	if err := json.Unmarshal(previewRecorder.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.Count != 1 || preview.Candidates[0].Name != "old.log" {
		t.Fatalf("unexpected preview: %+v", preview)
	}

	oldEnqueueAuditLog := enqueueAuditLog
	enqueueAuditLog = func(model.AuditLog) {}
	t.Cleanup(func() { enqueueAuditLog = oldEnqueueAuditLog })

	deleteContext, deleteRecorder := newSourceModLogJSONContext(
		"admin",
		"/logs/delete",
		map[string]any{"files": []logic.SourceModLogDeleteTarget{{Name: "old.log", Version: preview.Candidates[0].Version}}},
	)
	DeleteSourceModLogs(deleteContext)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %q", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	var result logic.SourceModLogDeleteResult
	if err := json.Unmarshal(deleteRecorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode delete result: %v", err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "old.log" || result.FreedBytes != int64(len("old log")) {
		t.Fatalf("unexpected delete result: %+v", result)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("old.log still exists: %v", err)
	}
}

func TestPreviewSourceModLogCleanupRejectsInvalidFilter(t *testing.T) {
	setupSourceModLogControllerTestDir(t)
	context, recorder := newSourceModLogJSONContext(
		"admin",
		"/logs/cleanup/preview",
		map[string]any{"categories": []string{"other"}, "retention_days": 14},
	)
	PreviewSourceModLogCleanup(context)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %q, want 400", recorder.Code, recorder.Body.String())
	}
}

func setupSourceModLogControllerTestDir(t *testing.T) string {
	t.Helper()
	oldGamePath := consts.GamePath
	gamePath := t.TempDir()
	logsDir := filepath.Join(gamePath, "addons", "sourcemod", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("create SourceMod logs directory: %v", err)
	}
	consts.GamePath = gamePath
	t.Cleanup(func() { consts.GamePath = oldGamePath })
	return logsDir
}

func newSourceModLogJSONContext(role, path string, body any) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	payload, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	context.Request = request
	context.Set("role", role)
	return context, recorder
}
