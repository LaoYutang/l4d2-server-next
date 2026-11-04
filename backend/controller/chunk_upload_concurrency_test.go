package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"l4d2-manager-next/consts"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func uploadTaskFixture(t *testing.T, size int64, chunks int) (string, string) {
	t.Helper()
	id := uuid.NewString()
	path := getUploadTempPath(id)
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, ".meta"), []byte(fmt.Sprintf("map.vpk\n%d\n%d\n", size, chunks)), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		uploadTasks.Lock()
		defer uploadTasks.Unlock()
		if _, exists := uploadTasks.entries[path]; exists {
			t.Error("task guard leaked after all requests completed")
		}
	})
	return id, path
}

func chunkDataContext(t *testing.T, id string, index int, data []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	req, err := newChunkDataUploadRequest(id, index, data)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")
	t.Cleanup(func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	})
	return c, w
}

func uploadActionContext(id, action string) (*gin.Context, *httptest.ResponseRecorder) {
	return newFormTestContext("/upload/"+action, url.Values{"uploadId": {id}, "filename": {"map.vpk"}})
}

func awaitUploadSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for upload operation")
	}
}

func startUploadHandler(t *testing.T, handler gin.HandlerFunc, c *gin.Context) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); handler(c) }()
	t.Cleanup(func() { awaitUploadSignal(t, done) })
	return done
}

func awaitUploadOperation(t *testing.T, task *uploadTask) {
	t.Helper()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	for {
		task.mu.Lock()
		active := task.operation != nil
		task.mu.Unlock()
		if active {
			return
		}
		select {
		case <-ticker.C:
		case <-timeout.C:
			t.Fatal("timed out waiting for upload operation to begin")
		}
	}
}

func uploadGate() (chan struct{}, func()) {
	gate := make(chan struct{})
	var once sync.Once
	return gate, func() { once.Do(func() { close(gate) }) }
}

func assertUploadResponse(t *testing.T, w *httptest.ResponseRecorder, code int, message string) {
	t.Helper()
	if w.Code != code || (message != "" && !strings.Contains(w.Body.String(), message)) {
		t.Fatalf("response = %d %q, want %d containing %q", w.Code, w.Body.String(), code, message)
	}
}

func assertUploadedChunks(t *testing.T, id string, expected ...int) {
	t.Helper()
	c, w := uploadActionContext(id, "status")
	UploadStatus(c)
	assertUploadResponse(t, w, 200, "")
	var payload struct {
		Chunks []int `json:"uploadedChunks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(payload.Chunks) != fmt.Sprint(expected) {
		t.Fatalf("uploadedChunks = %v, want %v", payload.Chunks, expected)
	}
}

func assertUploadFile(t *testing.T, path string, expected []byte) {
	t.Helper()
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatalf("file %s does not match expected content: size=%d, error=%v", path, len(actual), err)
	}
}

func assertNoUploadTemps(t *testing.T, path string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(path, "*.tmp"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary files remain: %v, error=%v", files, err)
	}
}

func TestUploadChunkConcurrentRetryKeepsFirstCompleted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, addons := setupMapImportTest(t)
	oldData := bytes.Repeat([]byte("A"), 5<<20)
	newData := bytes.Repeat([]byte("B"), len(oldData))
	id, path := uploadTaskFixture(t, int64(len(oldData)), 1)
	previousCopy := copyUploadData
	t.Cleanup(func() { copyUploadData = previousCopy })
	started := make(chan struct{})
	gate, releaseOld := uploadGate()
	defer releaseOld()
	var calls atomic.Int32
	var paths [2]string
	copyUploadData = func(dst io.Writer, src io.Reader) (int64, error) {
		call := calls.Add(1)
		if call <= 2 {
			paths[call-1] = dst.(*os.File).Name()
		}
		if call == 1 {
			n, err := io.CopyN(dst, src, 16<<10)
			if err != nil {
				return n, err
			}
			close(started)
			<-gate
			rest, err := previousCopy(dst, src)
			return n + rest, err
		}
		return previousCopy(dst, src)
	}
	oldContext, oldResponse := chunkDataContext(t, id, 0, oldData)
	oldDone := startUploadHandler(t, UploadChunk, oldContext)
	awaitUploadSignal(t, started)
	assertUploadedChunks(t, id)
	retryContext, retryResponse := chunkDataContext(t, id, 0, newData)
	UploadChunk(retryContext)
	assertUploadResponse(t, retryResponse, 200, "OK")
	if paths[0] == paths[1] {
		t.Fatal("concurrent requests shared a temporary path")
	}
	assertUploadFile(t, filepath.Join(path, "0"), newData)
	assertUploadedChunks(t, id, 0)
	// A fully late request is also idempotent and cannot overwrite the winner.
	lateContext, lateResponse := chunkDataContext(t, id, 0, oldData)
	UploadChunk(lateContext)
	assertUploadResponse(t, lateResponse, 200, "OK")
	assertUploadFile(t, filepath.Join(path, "0"), newData)
	task, releaseTask := acquireUploadTask(id)
	defer releaseTask()
	mergeContext, mergeResponse := uploadActionContext(id, "merge")
	mergeDone := startUploadHandler(t, UploadMerge, mergeContext)
	awaitUploadOperation(t, task)
	select {
	case <-mergeDone:
		t.Fatal("merge returned while the older request was still writing")
	default:
	}
	releaseOld()
	awaitUploadSignal(t, oldDone)
	awaitUploadSignal(t, mergeDone)
	assertUploadResponse(t, oldResponse, 200, "OK")
	assertUploadResponse(t, mergeResponse, 200, "上传成功！")
	assertUploadFile(t, filepath.Join(addons, "map.vpk"), newData)
}

func TestUploadChunkInterruptedRequestOnlyRemovesItsOwnTemp(t *testing.T) {
	setupChunkUploadTestPaths(t)
	id, path := uploadTaskFixture(t, 8, 2)
	if err := os.WriteFile(filepath.Join(path, "1"), []byte("KEPT"), 0644); err != nil {
		t.Fatal(err)
	}
	previousCopy := copyUploadData
	t.Cleanup(func() { copyUploadData = previousCopy })
	started := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
	gate0, release0 := uploadGate()
	gate1, release1 := uploadGate()
	defer release0()
	defer release1()
	gates := [2]chan struct{}{gate0, gate1}
	var calls atomic.Int32
	var paths [2]string
	copyUploadData = func(dst io.Writer, src io.Reader) (int64, error) {
		i := int(calls.Add(1)) - 1
		if i >= len(gates) {
			return previousCopy(dst, src)
		}
		paths[i] = dst.(*os.File).Name()
		n, err := io.CopyN(dst, src, 2)
		if err != nil {
			return n, err
		}
		close(started[i])
		<-gates[i]
		rest, err := previousCopy(dst, src)
		return n + rest, err
	}
	c, first := chunkDataContext(t, id, 0, []byte("STOP"))
	ctx, abort := context.WithCancel(c.Request.Context())
	defer abort()
	c.Request = c.Request.WithContext(ctx)
	firstDone := startUploadHandler(t, UploadChunk, c)
	awaitUploadSignal(t, started[0])
	c, second := chunkDataContext(t, id, 0, []byte("GOOD"))
	secondDone := startUploadHandler(t, UploadChunk, c)
	awaitUploadSignal(t, started[1])
	abort()
	release0()
	awaitUploadSignal(t, firstDone)
	assertUploadResponse(t, first, 400, "中断")
	if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
		t.Fatalf("aborted request temp remains: %v", err)
	}
	if _, err := os.Stat(paths[1]); err != nil {
		t.Fatalf("aborted request removed another request's temp: %v", err)
	}
	assertUploadedChunks(t, id, 1)
	release1()
	awaitUploadSignal(t, secondDone)
	assertUploadResponse(t, second, 200, "OK")
	assertUploadFile(t, filepath.Join(path, "0"), []byte("GOOD"))
	assertUploadFile(t, filepath.Join(path, "1"), []byte("KEPT"))
	assertNoUploadTemps(t, path)
}

func TestUploadCancelWaitsForWriterAndRejectsLatePublication(t *testing.T) {
	setupChunkUploadTestPaths(t)
	id, path := uploadTaskFixture(t, 4, 1)
	previousCopy := copyUploadData
	t.Cleanup(func() { copyUploadData = previousCopy })
	started := make(chan struct{})
	gate, releaseWriter := uploadGate()
	defer releaseWriter()
	copyUploadData = func(dst io.Writer, src io.Reader) (int64, error) {
		close(started)
		<-gate
		return previousCopy(dst, src)
	}
	c, writerResponse := chunkDataContext(t, id, 0, []byte("DATA"))
	writerDone := startUploadHandler(t, UploadChunk, c)
	awaitUploadSignal(t, started)
	task, releaseTask := acquireUploadTask(id)
	var releaseTaskOnce sync.Once
	defer releaseTaskOnce.Do(releaseTask)
	c, cancelResponse := uploadActionContext(id, "cancel")
	cancelDone := startUploadHandler(t, UploadCancel, c)
	awaitUploadSignal(t, task.ctx.Done())
	select {
	case <-cancelDone:
		t.Fatal("cancellation removed files while a writer was active")
	default:
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("task directory removed before writer finished")
	}
	c, lateResponse := chunkDataContext(t, id, 0, []byte("LATE"))
	UploadChunk(c)
	assertUploadResponse(t, lateResponse, 409, "已取消")
	releaseWriter()
	awaitUploadSignal(t, writerDone)
	awaitUploadSignal(t, cancelDone)
	assertUploadResponse(t, writerResponse, 409, "已取消")
	assertUploadResponse(t, cancelResponse, 200, "已取消")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cancelled task remains: %v", err)
	}
	assertUploadedChunks(t, id)
	releaseTaskOnce.Do(releaseTask)
	c, lateResponse = chunkDataContext(t, id, 0, []byte("LATE"))
	UploadChunk(c)
	assertUploadResponse(t, lateResponse, 400, "重新上传")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("late request recreated cancelled task")
	}
}

func TestUploadMergeIsSynchronousAndCancelWaitsForImport(t *testing.T) {
	_, addons := setupMapImportTest(t)
	id, path := uploadTaskFixture(t, 4, 1)
	if err := os.WriteFile(filepath.Join(path, "0"), []byte("DATA"), 0644); err != nil {
		t.Fatal(err)
	}
	previousInstall := installMapImportFile
	t.Cleanup(func() { installMapImportFile = previousInstall })
	entered := make(chan struct{})
	gate, releaseImport := uploadGate()
	defer releaseImport()
	var installs atomic.Int32
	installMapImportFile = func(src, dst string) error {
		installs.Add(1)
		close(entered)
		<-gate
		return previousInstall(src, dst)
	}
	c, mergeResponse := uploadActionContext(id, "merge")
	mergeDone := startUploadHandler(t, UploadMerge, c)
	awaitUploadSignal(t, entered)
	select {
	case <-mergeDone:
		t.Fatal("merge returned before the map import finished")
	default:
	}
	c, duplicateResponse := uploadActionContext(id, "merge")
	UploadMerge(c)
	assertUploadResponse(t, duplicateResponse, 409, "重复合并")
	c, lateResponse := chunkDataContext(t, id, 0, []byte("LATE"))
	UploadChunk(c)
	assertUploadResponse(t, lateResponse, 409, "请勿继续上传")
	assertUploadedChunks(t, id, 0)
	task, releaseTask := acquireUploadTask(id)
	defer releaseTask()
	c, cancelResponse := uploadActionContext(id, "cancel")
	cancelDone := startUploadHandler(t, UploadCancel, c)
	awaitUploadSignal(t, task.ctx.Done())
	select {
	case <-cancelDone:
		t.Fatal("cancel did not wait for the import")
	default:
	}
	releaseImport()
	awaitUploadSignal(t, mergeDone)
	awaitUploadSignal(t, cancelDone)
	assertUploadResponse(t, mergeResponse, 200, "上传成功！")
	assertUploadResponse(t, cancelResponse, 200, "已取消")
	if installs.Load() != 1 {
		t.Fatalf("import count = %d, want 1", installs.Load())
	}
	assertUploadFile(t, filepath.Join(addons, "map.vpk"), []byte("DATA"))
	assertUploadFile(t, consts.MapListFilePath, []byte("map.vpk\n"))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("task remains after import and cancellation: %v", err)
	}
}

func TestUploadCancelDuringMergePreventsImport(t *testing.T) {
	_, addons := setupMapImportTest(t)
	id, path := uploadTaskFixture(t, 4, 1)
	if err := os.WriteFile(filepath.Join(path, "0"), []byte("DATA"), 0644); err != nil {
		t.Fatal(err)
	}
	previousCopy := copyUploadData
	t.Cleanup(func() { copyUploadData = previousCopy })
	entered := make(chan struct{})
	gate, releaseMerge := uploadGate()
	defer releaseMerge()
	copyUploadData = func(dst io.Writer, src io.Reader) (int64, error) {
		close(entered)
		<-gate
		return previousCopy(dst, src)
	}
	c, mergeResponse := uploadActionContext(id, "merge")
	mergeDone := startUploadHandler(t, UploadMerge, c)
	awaitUploadSignal(t, entered)
	task, releaseTask := acquireUploadTask(id)
	defer releaseTask()
	c, cancelResponse := uploadActionContext(id, "cancel")
	cancelDone := startUploadHandler(t, UploadCancel, c)
	awaitUploadSignal(t, task.ctx.Done())
	releaseMerge()
	awaitUploadSignal(t, mergeDone)
	awaitUploadSignal(t, cancelDone)
	assertUploadResponse(t, mergeResponse, 409, "已取消")
	assertUploadResponse(t, cancelResponse, 200, "已取消")
	if _, err := os.Stat(filepath.Join(addons, "map.vpk")); !os.IsNotExist(err) {
		t.Fatal("cancelled merge installed a map")
	}
	assertUploadFile(t, consts.MapListFilePath, nil)
}

func TestUploadMergeWaitingForWriterCanBeInterrupted(t *testing.T) {
	for _, explicitCancel := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel_task_%t", explicitCancel), func(t *testing.T) {
			_, addons := setupMapImportTest(t)
			id, path := uploadTaskFixture(t, 4, 1)
			previousCopy := copyUploadData
			t.Cleanup(func() { copyUploadData = previousCopy })
			started := make(chan struct{})
			gate, releaseWriter := uploadGate()
			defer releaseWriter()
			var copies atomic.Int32
			copyUploadData = func(dst io.Writer, src io.Reader) (int64, error) {
				if copies.Add(1) == 1 {
					close(started)
					<-gate
				}
				return previousCopy(dst, src)
			}
			c, writerResponse := chunkDataContext(t, id, 0, []byte("DATA"))
			writerDone := startUploadHandler(t, UploadChunk, c)
			awaitUploadSignal(t, started)
			task, releaseTask := acquireUploadTask(id)
			defer releaseTask()
			c, mergeResponse := uploadActionContext(id, "merge")
			ctx, abortMerge := context.WithCancel(c.Request.Context())
			defer abortMerge()
			c.Request = c.Request.WithContext(ctx)
			mergeDone := startUploadHandler(t, UploadMerge, c)
			awaitUploadOperation(t, task)
			if copies.Load() != 1 {
				t.Fatal("merge read chunks before the writer finished")
			}
			var cancelDone <-chan struct{}
			var cancelResponse *httptest.ResponseRecorder
			if explicitCancel {
				c, cancelResponse = uploadActionContext(id, "cancel")
				cancelDone = startUploadHandler(t, UploadCancel, c)
				awaitUploadSignal(t, task.ctx.Done())
			} else {
				abortMerge()
			}
			// Neither kind of cancellation needs the blocked writer to release
			// the merge operation. Task cancellation still drains the writer.
			awaitUploadSignal(t, mergeDone)
			if explicitCancel {
				assertUploadResponse(t, mergeResponse, 409, "已取消")
			} else {
				assertUploadResponse(t, mergeResponse, 400, "中断")
			}
			if _, err := os.Stat(filepath.Join(path, uploadImportMarker)); !os.IsNotExist(err) {
				t.Fatal("interrupted merge incorrectly recorded an import attempt")
			}
			releaseWriter()
			awaitUploadSignal(t, writerDone)
			if explicitCancel {
				awaitUploadSignal(t, cancelDone)
				assertUploadResponse(t, writerResponse, 409, "已取消")
				assertUploadResponse(t, cancelResponse, 200, "已取消")
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("cancelled task still exists")
				}
			} else {
				assertUploadResponse(t, writerResponse, 200, "OK")
				assertUploadedChunks(t, id, 0)
				c, retryResponse := uploadActionContext(id, "merge")
				UploadMerge(c)
				assertUploadResponse(t, retryResponse, 200, "上传成功！")
				assertUploadFile(t, filepath.Join(addons, "map.vpk"), []byte("DATA"))
			}
		})
	}
}
