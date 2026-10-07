package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/middlewares"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func setupOwnedDownloaderTest(t *testing.T) *downloader {
	t.Helper()
	previous := Downloader
	d := NewDownloader()
	// Hold the download slots so controller tests do not make network requests.
	for i := 0; i < cap(d.semaphore); i++ {
		d.semaphore <- struct{}{}
	}
	Downloader = d
	t.Cleanup(func() {
		for _, task := range d.tasks {
			task.Cancel()
			if task.done != nil {
				select {
				case <-task.done:
				case <-time.After(5 * time.Second):
					t.Error("download did not stop")
				}
			}
		}
		Downloader = previous
	})
	return d
}

func createMapDownloadCode(t *testing.T, store *logic.AuthCodeStore) logic.CreatedAuthCode {
	t.Helper()
	created, err := store.Create(logic.CreateAuthCodeRequest{AccessType: logic.AuthAccessMapUpload})
	if err != nil {
		t.Fatal(err)
	}
	return created
}

func ownedDownloadContext(ownerID, path string, fields url.Values) (*gin.Context, *httptest.ResponseRecorder) {
	c, response := newFormTestContext(path, fields)
	c.Set("role", middlewares.RoleMapUploader)
	c.Set("auth_code_id", ownerID)
	return c, response
}

func ownedDownloadTestTask(ownerID string, status DOWNLOAD_STATUS) *downloadTask {
	task := newDownloadConcurrencyTestTask(status)
	task.id = uuid.NewString()
	task.authCodeID = ownerID
	return task
}

func TestMapUploaderDownloadListAndActionsStayWithinOwner(t *testing.T) {
	store := setupAuthControllerStore(t)
	d := setupOwnedDownloaderTest(t)
	owner := createMapDownloadCode(t, store)
	other := createMapDownloadCode(t, store)
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{Total: 100 << 30, Free: 90 << 30, UsedPercent: 10})
	ownFinished := ownedDownloadTestTask(owner.ID, DOWNLOAD_STATUS_COMPLETED)
	ownPending := ownedDownloadTestTask(owner.ID, DOWNLOAD_STATUS_PENDING)
	otherFinished := ownedDownloadTestTask(other.ID, DOWNLOAD_STATUS_FAILED)
	unowned := ownedDownloadTestTask("", DOWNLOAD_STATUS_PENDING)
	d.tasks = []*downloadTask{otherFinished, ownFinished, unowned, ownPending}

	c, response := ownedDownloadContext(owner.ID, "/download/list", nil)
	GetDownloadTasksInfo(c)
	var listed []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || len(listed) != 2 || listed[0]["id"] != ownFinished.id || listed[1]["id"] != ownPending.id {
		t.Fatalf("owner list: %d %s", response.Code, response.Body.String())
	}

	for _, forbidden := range []*downloadTask{otherFinished, unowned} {
		for _, handler := range []gin.HandlerFunc{CancelDownloadTask, RestartDownloadTask} {
			c, response = ownedDownloadContext(owner.ID, "/download/action", url.Values{"id": {forbidden.id}})
			handler(c)
			if response.Code != http.StatusForbidden {
				t.Fatalf("other download accessible: %d %s", response.Code, response.Body.String())
			}
		}
	}

	c, response = ownedDownloadContext(owner.ID, "/download/cancel", url.Values{"index": {"0"}})
	CancelDownloadTask(c)
	if response.Code != http.StatusBadRequest {
		t.Fatal("restricted role accepted an unscoped list index")
	}
	c, response = ownedDownloadContext(owner.ID, "/download/cancel", url.Values{"id": {ownPending.id}})
	CancelDownloadTask(c)
	if response.Code != 200 || !ownPending.cancelled {
		t.Fatalf("own cancel: %d %s", response.Code, response.Body.String())
	}
	c, response = ownedDownloadContext(owner.ID, "/download/clear", nil)
	ClearTasks(c)
	if response.Code != 200 || len(d.tasks) != 3 || d.tasks[0] != otherFinished {
		t.Fatal("clearing own records changed another owner's downloads")
	}

	for _, role := range []string{middlewares.RoleAdmin, middlewares.RoleGuest} {
		c, response = newFormTestContext("/download/list", nil)
		c.Set("role", role)
		GetDownloadTasksInfo(c)
		if err := json.Unmarshal(response.Body.Bytes(), &listed); err != nil {
			t.Fatal(err)
		}
		if response.Code != 200 || len(listed) != 3 {
			t.Fatalf("existing %s download visibility changed", role)
		}
	}
}

func TestMapUploaderAddsOwnedBatchAndParsedDownloads(t *testing.T) {
	store := setupAuthControllerStore(t)
	d := setupOwnedDownloaderTest(t)
	owner := createMapDownloadCode(t, store)
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{Total: 100 << 30, Free: 90 << 30, UsedPercent: 10})
	c, response := ownedDownloadContext(owner.ID, "/download/add", url.Values{
		"url": {"https://example.invalid/one.vpk\nhttps://example.invalid/two.zip"},
	})
	AddDownloadTask(c)
	if response.Code != 200 || len(d.tasks) != 2 {
		t.Fatalf("batch add: %d %s", response.Code, response.Body.String())
	}
	c, response = ownedDownloadContext(owner.ID, "/download/add", url.Values{
		"url": {"https://example.invalid/file"}, "filename": {"解析地图.vpk"}, "referer": {"https://qfile.qq.com/q/example"},
	})
	AddDownloadTask(c)
	if response.Code != 200 || len(d.tasks) != 3 {
		t.Fatalf("parsed add: %d %s", response.Code, response.Body.String())
	}
	ids := make(map[string]bool)
	for _, task := range d.tasks {
		if task.authCodeID != owner.ID || task.id == "" || ids[task.id] {
			t.Fatal("download did not receive a distinct ID and owner")
		}
		ids[task.id] = true
	}
	if d.tasks[2].preferredFilename != "解析地图.vpk" || d.tasks[2].referer != "https://qfile.qq.com/q/example" {
		t.Fatal("parsed download lost its filename or Referer")
	}
	c, response = ownedDownloadContext(owner.ID, "/download/add", url.Values{"url": {"not a download link"}})
	AddDownloadTask(c)
	if response.Code != 400 || len(d.tasks) != 3 {
		t.Fatal("invalid input reported a successful download")
	}
}

func TestMapUploaderDownloadCannotBypassDiskLimit(t *testing.T) {
	store := setupAuthControllerStore(t)
	d := setupOwnedDownloaderTest(t)
	owner := createMapDownloadCode(t, store)
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{Total: 100 << 30, Free: 4 << 30, UsedPercent: 96})
	c, response := ownedDownloadContext(owner.ID, "/download/add", url.Values{
		"url": {"https://example.invalid/map.vpk"}, "force": {"true"},
	})
	AddDownloadTask(c)
	if response.Code != http.StatusInsufficientStorage || len(d.tasks) != 0 {
		t.Fatalf("disk limit bypassed: %d %s", response.Code, response.Body.String())
	}
	payload := decodeDiskLimitPayload(t, response.Body.Bytes())
	if payload.CanForce {
		t.Fatal("restricted download offered disk limit override")
	}
}

func TestDownloadFixedIDSurvivesCleanupAndRetry(t *testing.T) {
	store := setupAuthControllerStore(t)
	d := setupOwnedDownloaderTest(t)
	owner := createMapDownloadCode(t, store)
	other := createMapDownloadCode(t, store)
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{Total: 100 << 30, Free: 90 << 30, UsedPercent: 10})
	finished := ownedDownloadTestTask(other.ID, DOWNLOAD_STATUS_COMPLETED)
	retry := ownedDownloadTestTask(owner.ID, DOWNLOAD_STATUS_FAILED)
	retry.preferredFilename = "campaign.vpk"
	retry.referer = "https://qfile.qq.com/q/example"
	d.tasks = []*downloadTask{finished, retry}
	c, response := ownedDownloadContext(other.ID, "/download/clear", nil)
	ClearTasks(c)
	if response.Code != 200 || len(d.tasks) != 1 || d.tasks[0].id != retry.id {
		t.Fatal("cleanup changed the target task identity")
	}
	c, response = ownedDownloadContext(owner.ID, "/download/restart", url.Values{"id": {retry.id}})
	RestartDownloadTask(c)
	if response.Code != 200 {
		t.Fatalf("retry: %d %s", response.Code, response.Body.String())
	}
	current := d.tasks[0]
	if current == retry || current.id != retry.id || current.authCodeID != owner.ID || current.preferredFilename != "campaign.vpk" || current.referer != retry.referer {
		t.Fatal("retry changed task identity, ownership, filename or Referer")
	}
	c, response = ownedDownloadContext(owner.ID, "/download/cancel", url.Values{"id": {current.id}})
	CancelDownloadTask(c)
	if response.Code != 200 {
		t.Fatal("fixed ID no longer addresses the retried download")
	}
	c, response = newFormTestContext("/download/cancel", url.Values{"index": {"0"}})
	c.Set("role", middlewares.RoleAdmin)
	CancelDownloadTask(c)
	if response.Code != 200 {
		t.Fatal("legacy index API stopped working")
	}
}

func TestMapDownloadCodeRecreationCannotInheritDownloads(t *testing.T) {
	store := setupAuthControllerStore(t)
	d := setupOwnedDownloaderTest(t)
	owner := createMapDownloadCode(t, store)
	task := ownedDownloadTestTask(owner.ID, DOWNLOAD_STATUS_FAILED)
	d.tasks = []*downloadTask{task}
	if _, err := store.Delete(owner.ID); err != nil {
		t.Fatal(err)
	}
	recreated, err := store.Create(logic.CreateAuthCodeRequest{Code: owner.Code, AccessType: logic.AuthAccessMapUpload})
	if err != nil {
		t.Fatal(err)
	}
	c, response := ownedDownloadContext(recreated.ID, "/download/list", nil)
	GetDownloadTasksInfo(c)
	if response.Code != 200 || strings.TrimSpace(response.Body.String()) != "[]" {
		t.Fatal("recreated credential inherited the old download list")
	}
	c, response = ownedDownloadContext(recreated.ID, "/download/cancel", url.Values{"id": {task.id}})
	CancelDownloadTask(c)
	if response.Code != 403 {
		t.Fatal("recreated credential took over an old download")
	}
	if _, err := store.Revoke(recreated.ID); err != nil {
		t.Fatal(err)
	}
	for _, handler := range []gin.HandlerFunc{GetDownloadTasksInfo, CancelDownloadTask, ClearTasks, RestartDownloadTask} {
		c, response = ownedDownloadContext(recreated.ID, "/download/action", url.Values{"id": {task.id}})
		handler(c)
		if response.Code != 401 {
			t.Fatalf("revoked download authorization accepted: %d", response.Code)
		}
	}
}

func TestQueuedMapDownloadRejectsExpiredAuthorization(t *testing.T) {
	store := setupAuthControllerStore(t)
	owner := createMapDownloadCode(t, store)
	past := time.Now().Add(-time.Minute)
	if _, err := store.Update(logic.UpdateAuthCodeRequest{ID: owner.ID, ExpiresAt: &past}); err != nil {
		t.Fatal(err)
	}
	task := newDownloadTaskForOwner("https://example.invalid/expired.vpk", "", "", owner.ID, "", make(chan struct{}, 1))
	select {
	case <-task.done:
	case <-time.After(5 * time.Second):
		task.Cancel()
		t.Fatal("expired download did not stop before network access")
	}
	if task.GetStatus() != DOWNLOAD_STATUS_FAILED || !strings.Contains(task.GetMessage(), "无法开始下载") {
		t.Fatalf("expired download: %d %s", task.GetStatus(), task.GetMessage())
	}
}

func TestMapDownloadRevokedDuringTransferDoesNotInstall(t *testing.T) {
	store := setupAuthControllerStore(t)
	owner := createMapDownloadCode(t, store)
	setupChunkUploadTestPaths(t)
	payload := []byte("map payload downloaded before authorization is revoked")
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		_, _ = w.Write(payload[:4])
		w.(http.Flusher).Flush()
		close(started)
		<-release
		_, _ = w.Write(payload[4:])
	}))
	task := newDownloadTaskForOwner(server.URL+"/revoked.vpk", "", "", owner.ID, "", make(chan struct{}, 1))
	t.Cleanup(func() {
		unblock()
		task.Cancel()
		select {
		case <-task.done:
		case <-time.After(5 * time.Second):
			t.Error("download did not finish cleanup")
		}
		server.Close()
	})
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("download did not begin")
	}
	if _, err := store.Revoke(owner.ID); err != nil {
		t.Fatal(err)
	}
	unblock()
	select {
	case <-task.done:
	case <-time.After(5 * time.Second):
		t.Fatal("download did not finish")
	}
	if task.GetStatus() != DOWNLOAD_STATUS_FAILED || !strings.Contains(task.GetMessage(), "无法安装") {
		t.Fatalf("revoked transfer: %d %s", task.GetStatus(), task.GetMessage())
	}
	if _, err := os.Stat(filepath.Join(consts.AddonsBasePath, "revoked.vpk")); !os.IsNotExist(err) {
		t.Fatal("revoked download was installed")
	}
	entries, err := os.ReadDir(filepath.Join(consts.AddonsBasePath, "temp"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary files survived revoked download: %v %v", entries, err)
	}
}
