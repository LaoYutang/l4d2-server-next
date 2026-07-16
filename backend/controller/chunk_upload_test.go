package controller

import (
	"bytes"
	"fmt"
	"l4d2-manager-next/consts"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestUploadChunkAcceptsFrontendFiveMBChunkWhenAverageChunkIsSmaller(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldAddonsBasePath := consts.AddonsBasePath
	consts.AddonsBasePath = filepath.Join(t.TempDir(), "addons")
	t.Cleanup(func() {
		consts.AddonsBasePath = oldAddonsBasePath
	})

	uploadId := uuid.New().String()
	tempPath := getUploadTempPath(uploadId)
	if err := os.MkdirAll(tempPath, 0755); err != nil {
		t.Fatalf("create temp path: %v", err)
	}

	fileSize := int64(5<<20) + 1
	totalChunks := 2
	meta := fmt.Sprintf("addon.zip\n%d\n%d\n", fileSize, totalChunks)
	if err := os.WriteFile(filepath.Join(tempPath, ".meta"), []byte(meta), 0644); err != nil {
		t.Fatalf("write meta: %v", err)
	}

	req, err := newChunkUploadRequest(uploadId, 0, 5<<20)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	UploadChunk(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(tempPath, "0")); err != nil {
		t.Fatalf("saved chunk missing: %v", err)
	}
}

func TestChunkUploadEndpointsRejectInvalidUploadIds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupChunkUploadTestPaths(t)

	validId := uuid.New().String()
	invalidIds := []string{
		"..",
		"../escape",
		`..\escape`,
		`C:\temp\escape`,
		"not-a-uuid",
		strings.ReplaceAll(validId, "-", ""),
		"urn:uuid:" + validId,
		"{" + validId + "}",
		strings.ToUpper(validId),
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
	}
	endpoints := []struct {
		name    string
		path    string
		handler gin.HandlerFunc
		fields  url.Values
	}{
		{
			name:    "chunk",
			path:    "/upload/chunk",
			handler: UploadChunk,
			fields:  url.Values{"chunkIndex": {"0"}},
		},
		{
			name:    "status",
			path:    "/upload/status",
			handler: UploadStatus,
			fields:  url.Values{},
		},
		{
			name:    "merge",
			path:    "/upload/merge",
			handler: UploadMerge,
			fields:  url.Values{"filename": {"map.vpk"}},
		},
		{
			name:    "cancel",
			path:    "/upload/cancel",
			handler: UploadCancel,
			fields:  url.Values{},
		},
	}

	for _, endpoint := range endpoints {
		for _, uploadId := range invalidIds {
			t.Run(endpoint.name+"/"+uploadId, func(t *testing.T) {
				fields := cloneFormValues(endpoint.fields)
				fields.Set("uploadId", uploadId)
				c, w := newFormTestContext(endpoint.path, fields)

				endpoint.handler(c)

				if w.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, body = %q, want 400", w.Code, w.Body.String())
				}
			})
		}
	}
}

func TestUploadCancelRejectsTraversalWithoutDeletingFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rootDir, addonsPath := setupChunkUploadTestPaths(t)

	addonsSentinel := filepath.Join(addonsPath, "keep.vpk")
	outsideSentinel := filepath.Join(rootDir, "outside.txt")
	if err := os.WriteFile(addonsSentinel, []byte("keep"), 0644); err != nil {
		t.Fatalf("write addons sentinel: %v", err)
	}
	if err := os.WriteFile(outsideSentinel, []byte("keep"), 0644); err != nil {
		t.Fatalf("write outside sentinel: %v", err)
	}

	c, w := newFormTestContext("/upload/cancel", url.Values{"uploadId": {".."}})
	UploadCancel(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %q, want 400", w.Code, w.Body.String())
	}
	for _, path := range []string{addonsSentinel, outsideSentinel} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("sentinel %s was removed: %v", path, err)
		}
	}
}

func TestUploadStatusAndCancelAcceptValidMissingUploadId(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupChunkUploadTestPaths(t)
	uploadId := uuid.New().String()

	c, statusRecorder := newFormTestContext("/upload/status", url.Values{"uploadId": {uploadId}})
	UploadStatus(c)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("status endpoint = %d, body = %q, want 200", statusRecorder.Code, statusRecorder.Body.String())
	}

	c, cancelRecorder := newFormTestContext("/upload/cancel", url.Values{"uploadId": {uploadId}})
	UploadCancel(c)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("cancel endpoint = %d, body = %q, want 200", cancelRecorder.Code, cancelRecorder.Body.String())
	}
}

func TestUploadMergeAcceptsValidUploadId(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, addonsPath := setupChunkUploadTestPaths(t)

	uploadId := uuid.New().String()
	tempPath := getUploadTempPath(uploadId)
	if err := os.MkdirAll(tempPath, 0755); err != nil {
		t.Fatalf("create temp path: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempPath, ".meta"), []byte("map.vpk\n3\n1\n"), 0644); err != nil {
		t.Fatalf("write meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempPath, "0"), []byte("vpk"), 0644); err != nil {
		t.Fatalf("write chunk: %v", err)
	}

	c, w := newFormTestContext("/upload/merge", url.Values{
		"uploadId": {uploadId},
		"filename": {"map.vpk"},
	})
	UploadMerge(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q, want 200", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(addonsPath, "map.vpk")); err != nil {
		t.Fatalf("merged map missing: %v", err)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("upload temp directory still exists: %v", err)
	}
}

func setupChunkUploadTestPaths(t *testing.T) (string, string) {
	t.Helper()

	oldAddonsBasePath := consts.AddonsBasePath
	oldMapListFilePath := consts.MapListFilePath
	rootDir := t.TempDir()
	addonsPath := filepath.Join(rootDir, "addons")
	if err := os.MkdirAll(addonsPath, 0755); err != nil {
		t.Fatalf("create addons path: %v", err)
	}
	consts.AddonsBasePath = addonsPath
	consts.MapListFilePath = filepath.Join(addonsPath, "maplist.txt")
	if err := os.WriteFile(consts.MapListFilePath, nil, 0644); err != nil {
		t.Fatalf("create maplist: %v", err)
	}
	t.Cleanup(func() {
		consts.AddonsBasePath = oldAddonsBasePath
		consts.MapListFilePath = oldMapListFilePath
	})
	return rootDir, addonsPath
}

func newFormTestContext(path string, fields url.Values) (*gin.Context, *httptest.ResponseRecorder) {
	body := strings.NewReader(fields.Encode())
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

func cloneFormValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, items := range values {
		cloned[key] = append([]string(nil), items...)
	}
	return cloned
}

func newChunkUploadRequest(uploadId string, chunkIndex int, chunkSize int) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("uploadId", uploadId); err != nil {
		return nil, err
	}
	if err := writer.WriteField("chunkIndex", fmt.Sprintf("%d", chunkIndex)); err != nil {
		return nil, err
	}
	part, err := writer.CreateFormFile("chunk", "chunk.part")
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(make([]byte, chunkSize)); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req := httptest.NewRequest(http.MethodPost, "/upload/chunk", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}
