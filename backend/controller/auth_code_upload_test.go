package controller

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"l4d2-manager-next/logic"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestAuthCodeUploadOwnershipAndLifecycle(t *testing.T) {
	store := setupAuthControllerStore(t)
	setupChunkUploadTestPaths(t)
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{Total: 100 << 30, Free: 90 << 30, UsedPercent: 10})
	owner, err := store.Create(logic.CreateAuthCodeRequest{Code: "uploadOwner", AccessType: logic.AuthAccessMapUpload})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.Create(logic.CreateAuthCodeRequest{Code: "otherOwner"})
	if err != nil {
		t.Fatal(err)
	}
	c, response := newFormTestContext("/upload/init", url.Values{"filename": {"test.vpk"}, "fileSize": {"8"}, "totalChunks": {"1"}})
	c.Set("role", "map_uploader")
	c.Set("auth_code_id", owner.ID)
	UploadInit(c)
	if response.Code != 200 {
		t.Fatalf("init: %d %s", response.Code, response.Body.String())
	}
	var initialized struct {
		UploadID string `json:"uploadId"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &initialized); err != nil {
		t.Fatal(err)
	}
	ownerFile, err := os.ReadFile(filepath.Join(getUploadTempPath(initialized.UploadID), ".auth_owner"))
	if err != nil || string(ownerFile) != owner.ID {
		t.Fatalf("stored owner: %q %v", ownerFile, err)
	}
	fields := url.Values{"uploadId": {initialized.UploadID}, "chunkIndex": {"0"}, "filename": {"test.vpk"}}
	for _, handler := range []gin.HandlerFunc{UploadChunk, UploadStatus, UploadMerge, UploadCancel} {
		c, response = newFormTestContext("/upload/test", fields)
		c.Set("role", "guest")
		c.Set("auth_code_id", other.ID)
		handler(c)
		if response.Code != 403 {
			t.Fatalf("other credential accepted: %d %s", response.Code, response.Body.String())
		}
	}
	request, err := newChunkUploadRequest(initialized.UploadID, 0, 8)
	if err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(response)
	c.Request = request
	c.Set("role", "map_uploader")
	c.Set("auth_code_id", owner.ID)
	UploadChunk(c)
	if response.Code != 200 {
		t.Fatalf("own chunk: %d %s", response.Code, response.Body.String())
	}
	past := time.Now().Add(-time.Hour)
	if _, err := store.Update(logic.UpdateAuthCodeRequest{ID: owner.ID, ExpiresAt: &past}); err != nil {
		t.Fatal(err)
	}
	c, response = newFormTestContext("/upload/merge", fields)
	c.Set("role", "map_uploader")
	c.Set("auth_code_id", owner.ID)
	UploadMerge(c)
	if response.Code != 401 {
		t.Fatalf("expired merge accepted: %d %s", response.Code, response.Body.String())
	}
	if _, err := os.Stat(filepath.Join(getUploadTempPath(initialized.UploadID), "0")); err != nil {
		t.Fatal("failed merge removed resumable chunk")
	}
	future := time.Now().Add(time.Hour)
	if _, err := store.Update(logic.UpdateAuthCodeRequest{ID: owner.ID, ExpiresAt: &future}); err != nil {
		t.Fatal(err)
	}
	c, response = newFormTestContext("/upload/status", fields)
	c.Set("role", "map_uploader")
	c.Set("auth_code_id", owner.ID)
	UploadStatus(c)
	if response.Code != 200 {
		t.Fatal("renewed owner could not resume")
	}
	if _, err := store.Delete(owner.ID); err != nil {
		t.Fatal(err)
	}
	recreated, err := store.Create(logic.CreateAuthCodeRequest{Code: owner.Code})
	if err != nil {
		t.Fatal(err)
	}
	c, response = newFormTestContext("/upload/status", fields)
	c.Set("role", "guest")
	c.Set("auth_code_id", recreated.ID)
	UploadStatus(c)
	if response.Code != 403 {
		t.Fatal("recreated credential inherited old upload")
	}
	c, response = newFormTestContext("/upload/cancel", fields)
	UploadCancel(c)
	if response.Code != 200 {
		t.Fatal("administrator could not cancel another upload")
	}
	if _, err := os.Stat(getUploadTempPath(initialized.UploadID)); !os.IsNotExist(err) {
		t.Fatal("cancel retained upload directory")
	}
}

func TestAuthCodeCannotAccessLegacyUnownedUpload(t *testing.T) {
	store := setupAuthControllerStore(t)
	setupChunkUploadTestPaths(t)
	created, err := store.Create(logic.CreateAuthCodeRequest{Code: "newUpload"})
	if err != nil {
		t.Fatal(err)
	}
	uploadID := uuid.NewString()
	if err := os.MkdirAll(getUploadTempPath(uploadID), 0755); err != nil {
		t.Fatal(err)
	}
	c, response := newFormTestContext("/upload/status", url.Values{"uploadId": {uploadID}})
	c.Set("role", "guest")
	c.Set("auth_code_id", created.ID)
	UploadStatus(c)
	if response.Code != 403 {
		t.Fatalf("legacy ownerless task accepted: %d", response.Code)
	}
	c, response = newFormTestContext("/upload/status", url.Values{"uploadId": {uploadID}})
	UploadStatus(c)
	if response.Code != 200 {
		t.Fatal("administrator denied legacy upload")
	}
}
