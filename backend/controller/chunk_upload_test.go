package controller

import (
	"bytes"
	"fmt"
	"l4d2-manager-next/consts"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUploadChunkAcceptsFrontendFiveMBChunkWhenAverageChunkIsSmaller(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldAddonsBasePath := consts.AddonsBasePath
	consts.AddonsBasePath = filepath.Join(t.TempDir(), "addons")
	t.Cleanup(func() {
		consts.AddonsBasePath = oldAddonsBasePath
	})

	uploadId := "upload-five-mb-boundary"
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
