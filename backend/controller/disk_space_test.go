package controller

import (
	"encoding/json"
	"errors"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupDiskSpaceControllerTest(t *testing.T) {
	t.Helper()

	oldManagerDataPath := consts.ManagerDataPath
	oldManagerConfigPath := consts.ManagerConfigPath

	consts.ManagerDataPath = filepath.Join(t.TempDir(), "data")
	consts.ManagerConfigPath = filepath.Join(consts.ManagerDataPath, "manager_config.json")
	logic.LoadManagerConfig()
	gin.SetMode(gin.TestMode)

	t.Cleanup(func() {
		consts.ManagerDataPath = oldManagerDataPath
		consts.ManagerConfigPath = oldManagerConfigPath
		logic.LoadManagerConfig()
	})
}

func stubDiskUsage(t *testing.T, usage logic.DiskUsage) {
	t.Helper()

	original := queryUploadDiskUsage
	queryUploadDiskUsage = func() (logic.DiskUsage, error) { return usage, nil }
	t.Cleanup(func() { queryUploadDiskUsage = original })
}

func stubDiskUsageError(t *testing.T, err error) {
	t.Helper()

	original := queryUploadDiskUsage
	queryUploadDiskUsage = func() (logic.DiskUsage, error) { return logic.DiskUsage{}, err }
	t.Cleanup(func() { queryUploadDiskUsage = original })
}

func newJSONTestContext(path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

func decodeDiskLimitPayload(t *testing.T, body []byte) diskLimitPayload {
	t.Helper()

	var payload diskLimitPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode disk limit payload: %v; body = %q", err, string(body))
	}
	return payload
}

func TestEnsureUploadDiskSpaceOnlyAllowsAdminConfirmation(t *testing.T) {
	const gib = 1 << 30
	cases := []struct {
		name         string
		role         string
		force        string
		allowForce   bool
		wantOK       bool
		wantForced   bool
		wantCanForce bool
	}{
		{name: "管理员未确认被拒绝", role: "admin", force: "false", allowForce: true, wantCanForce: true},
		{name: "管理员确认后放行", role: "admin", force: "true", allowForce: true, wantOK: true, wantForced: true},
		{name: "游客带确认参数仍被拒绝", role: "guest", force: "true", allowForce: true},
		{name: "地图上传授权码带确认参数仍被拒绝", role: "map_uploader", force: "true", allowForce: true},
		{name: "调用方不允许确认时管理员也被拒绝", role: "admin", force: "true", allowForce: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupDiskSpaceControllerTest(t)
			stubDiskUsage(t, logic.DiskUsage{UsedPercent: 95, Total: 500 * gib, Free: 100 * gib})

			fields := url.Values{"force": {tc.force}}
			c, w := newFormTestContext("/upload/init", fields)
			c.Set("role", tc.role)

			ok, forced := ensureUploadDiskSpace(c, 1*gib, tc.allowForce)
			if ok != tc.wantOK || forced != tc.wantForced {
				t.Fatalf("ensureUploadDiskSpace() = (%v, %v), want (%v, %v)", ok, forced, tc.wantOK, tc.wantForced)
			}

			if tc.wantOK {
				if w.Code != http.StatusOK {
					t.Fatalf("allowed request status = %d, want 200", w.Code)
				}
				return
			}

			if w.Code != http.StatusInsufficientStorage {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusInsufficientStorage)
			}
			payload := decodeDiskLimitPayload(t, w.Body.Bytes())
			if payload.Code != string(logic.DiskBlockUsageExceeded) {
				t.Fatalf("code = %q, want %q", payload.Code, logic.DiskBlockUsageExceeded)
			}
			if payload.CanForce != tc.wantCanForce {
				t.Fatalf("can_force = %v, want %v", payload.CanForce, tc.wantCanForce)
			}
			if payload.LimitPercent != logic.DefaultDiskUsageLimitPercent {
				t.Fatalf("limit_percent = %d, want %d", payload.LimitPercent, logic.DefaultDiskUsageLimitPercent)
			}
			if payload.Message == "" {
				t.Fatal("message is empty")
			}
		})
	}
}

func TestEnsureUploadDiskSpaceBlocksWhenFreeSpaceInsufficient(t *testing.T) {
	const gib = 1 << 30
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{UsedPercent: 95, Total: 500 * gib, Free: 3 * gib})

	c, w := newFormTestContext("/upload/init", url.Values{"force": {"true"}})
	c.Set("role", "admin")

	if ok, forced := ensureUploadDiskSpace(c, 4*gib, true); ok || forced {
		t.Fatalf("ensureUploadDiskSpace() = (%v, %v), want blocked", ok, forced)
	}

	if w.Code != http.StatusInsufficientStorage {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInsufficientStorage)
	}
	payload := decodeDiskLimitPayload(t, w.Body.Bytes())
	if payload.Code != string(logic.DiskBlockFreeInsufficient) {
		t.Fatalf("code = %q, want %q", payload.Code, logic.DiskBlockFreeInsufficient)
	}
	if payload.CanForce {
		t.Fatal("can_force = true, want false for insufficient free space")
	}
	if payload.RequiredBytes != 8*gib {
		t.Fatalf("required_bytes = %d, want %d", payload.RequiredBytes, 8*gib)
	}
}

func TestEnsureUploadDiskSpaceAllowsBelowLimit(t *testing.T) {
	const gib = 1 << 30
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{UsedPercent: 40, Total: 500 * gib, Free: 300 * gib})

	c, w := newFormTestContext("/upload/init", url.Values{})
	c.Set("role", "guest")

	if ok, forced := ensureUploadDiskSpace(c, 1*gib, true); !ok || forced {
		t.Fatalf("ensureUploadDiskSpace() = (%v, %v), want allowed without forcing", ok, forced)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestAddDownloadTaskRejectsWhenDiskUsageExceeded(t *testing.T) {
	const gib = 1 << 30
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{UsedPercent: 96, Total: 500 * gib, Free: 200 * gib})

	c, w := newFormTestContext("/download/add", url.Values{"url": {"https://example.com/a.vpk"}})
	c.Set("role", "admin")

	AddDownloadTask(c)

	if w.Code != http.StatusInsufficientStorage {
		t.Fatalf("status = %d, want %d; body = %q", w.Code, http.StatusInsufficientStorage, w.Body.String())
	}
	payload := decodeDiskLimitPayload(t, w.Body.Bytes())
	if payload.Code != string(logic.DiskBlockUsageExceeded) {
		t.Fatalf("code = %q, want %q", payload.Code, logic.DiskBlockUsageExceeded)
	}
	if payload.CanForce {
		t.Fatal("can_force = true, want false for download tasks")
	}
}

func TestUploadInitAllowsUploadWhenDiskIsReady(t *testing.T) {
	const gib = 1 << 30
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{UsedPercent: 40, Total: 500 * gib, Free: 300 * gib})
	rootDir, addonsPath := setupChunkUploadTestPaths(t)
	_ = rootDir

	c, w := newFormTestContext("/upload/init", url.Values{
		"filename":    {"probe.vpk"},
		"fileSize":    {"1048576"},
		"totalChunks": {"1"},
	})
	c.Set("role", "admin")

	UploadInit(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", w.Code, w.Body.String())
	}
	var resp struct {
		UploadID string `json:"uploadId"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode init response: %v; body = %q", err, w.Body.String())
	}
	if resp.UploadID == "" {
		t.Fatal("uploadId is empty")
	}
	if _, err := os.Stat(filepath.Join(addonsPath, uploadTempDir, resp.UploadID, ".meta")); err != nil {
		t.Fatalf("upload meta not created: %v", err)
	}
}

func TestUploadInitBlocksWhenUsageExceeded(t *testing.T) {
	const gib = 1 << 30
	cases := []struct {
		name         string
		role         string
		force        string
		wantCode     logic.DiskSpaceBlockReason
		wantCanForce bool
	}{
		{name: "管理员未确认被拒绝", role: "admin", wantCode: logic.DiskBlockUsageExceeded, wantCanForce: true},
		{name: "游客不能确认", role: "guest", force: "true", wantCode: logic.DiskBlockUsageExceeded},
		{name: "地图上传授权码不能确认", role: "map_uploader", force: "true", wantCode: logic.DiskBlockUsageExceeded},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupDiskSpaceControllerTest(t)
			stubDiskUsage(t, logic.DiskUsage{UsedPercent: 95, Total: 500 * gib, Free: 200 * gib})
			_, addonsPath := setupChunkUploadTestPaths(t)

			c, w := newFormTestContext("/upload/init", url.Values{
				"filename":    {"blocked.vpk"},
				"fileSize":    {"1048576"},
				"totalChunks": {"1"},
				"force":       {tc.force},
			})
			c.Set("role", tc.role)

			UploadInit(c)

			if w.Code != http.StatusInsufficientStorage {
				t.Fatalf("status = %d, want %d; body = %q", w.Code, http.StatusInsufficientStorage, w.Body.String())
			}
			payload := decodeDiskLimitPayload(t, w.Body.Bytes())
			if payload.Code != string(tc.wantCode) {
				t.Fatalf("code = %q, want %q", payload.Code, tc.wantCode)
			}
			if payload.CanForce != tc.wantCanForce {
				t.Fatalf("can_force = %v, want %v", payload.CanForce, tc.wantCanForce)
			}

			// 被拒绝时不能留下临时目录
			entries, err := os.ReadDir(filepath.Join(addonsPath, uploadTempDir))
			if err != nil && !os.IsNotExist(err) {
				t.Fatalf("read upload temp dir: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("upload temp dir not empty after rejection: %d entries", len(entries))
			}
		})
	}
}

func TestUploadInitBlocksWhenFreeSpaceInsufficient(t *testing.T) {
	const gib = 1 << 30
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{UsedPercent: 95, Total: 500 * gib, Free: 1 * gib})
	setupChunkUploadTestPaths(t)

	// 1 GiB 的文件峰值需要约 2 GiB，剩余 1 GiB 不足以完成上传
	c, w := newFormTestContext("/upload/init", url.Values{
		"filename":    {"too-big.vpk"},
		"fileSize":    {strconv.FormatInt(1*gib, 10)},
		"totalChunks": {"2"},
		"force":       {"true"},
	})
	c.Set("role", "admin")

	UploadInit(c)

	if w.Code != http.StatusInsufficientStorage {
		t.Fatalf("status = %d, want %d; body = %q", w.Code, http.StatusInsufficientStorage, w.Body.String())
	}
	payload := decodeDiskLimitPayload(t, w.Body.Bytes())
	if payload.Code != string(logic.DiskBlockFreeInsufficient) {
		t.Fatalf("code = %q, want %q", payload.Code, logic.DiskBlockFreeInsufficient)
	}
	if payload.CanForce {
		t.Fatal("can_force = true, want false")
	}
}

func TestUploadInitAllowsAdminConfirmedUpload(t *testing.T) {
	const gib = 1 << 30
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{UsedPercent: 95, Total: 500 * gib, Free: 200 * gib})
	rootDir, addonsPath := setupChunkUploadTestPaths(t)
	_ = rootDir

	c, w := newFormTestContext("/upload/init", url.Values{
		"filename":    {"confirmed.vpk"},
		"fileSize":    {"1048576"},
		"totalChunks": {"1"},
		"force":       {"true"},
	})
	c.Set("role", "admin")

	UploadInit(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", w.Code, w.Body.String())
	}
	var resp struct {
		UploadID string `json:"uploadId"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode init response: %v; body = %q", err, w.Body.String())
	}
	if resp.UploadID == "" {
		t.Fatal("uploadId is empty")
	}
	if _, err := os.Stat(filepath.Join(addonsPath, uploadTempDir, resp.UploadID, ".meta")); err != nil {
		t.Fatalf("upload meta not created: %v", err)
	}
}

func TestAddDownloadTaskPassesDiskCheckWhenReady(t *testing.T) {
	const gib = 1 << 30
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{UsedPercent: 40, Total: 500 * gib, Free: 300 * gib})

	// url 为空时返回 400，可证明磁盘检查已放行并继续执行
	c, w := newFormTestContext("/download/add", url.Values{})
	c.Set("role", "admin")

	AddDownloadTask(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %q", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "下载链接不能为空") {
		t.Fatalf("body = %q, want download url validation error", w.Body.String())
	}
}

func TestDiskUsageConfigRequiresAdmin(t *testing.T) {
	setupDiskSpaceControllerTest(t)

	c, w := newFormTestContext("/disk-usage/config", url.Values{})
	c.Set("role", "guest")
	GetDiskUsageConfig(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("guest get config status = %d, want %d", w.Code, http.StatusForbidden)
	}

	c, w = newJSONTestContext("/config/disk-usage", `{"limit_percent":85}`)
	c.Set("role", "guest")
	SetDiskUsageConfig(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("guest set config status = %d, want %d", w.Code, http.StatusForbidden)
	}

	c, w = newJSONTestContext("/config/disk-usage", `{"limit_percent":85}`)
	c.Set("role", "map_uploader")
	SetDiskUsageConfig(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("map_uploader set config status = %d, want %d", w.Code, http.StatusForbidden)
	}
	if got := logic.GetDiskUsageLimitPercent(); got != logic.DefaultDiskUsageLimitPercent {
		t.Fatalf("limit after forbidden update = %d, want %d", got, logic.DefaultDiskUsageLimitPercent)
	}
}

func TestDiskUsageConfigAdminCanSaveAndReadBack(t *testing.T) {
	setupDiskSpaceControllerTest(t)
	stubDiskUsage(t, logic.DiskUsage{UsedPercent: 42.5, Total: 500 << 30, Free: 287 << 30})

	c, w := newJSONTestContext("/config/disk-usage", `{"limit_percent":85}`)
	c.Set("role", "admin")
	SetDiskUsageConfig(c)

	if w.Code != http.StatusOK {
		t.Fatalf("set config status = %d, body = %q", w.Code, w.Body.String())
	}
	var setResp struct {
		Status       string `json:"status"`
		LimitPercent int    `json:"limit_percent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &setResp); err != nil {
		t.Fatalf("decode set response: %v", err)
	}
	if setResp.Status != "ok" || setResp.LimitPercent != 85 {
		t.Fatalf("set response = %#v, want limit 85", setResp)
	}

	logic.LoadManagerConfig()
	if got := logic.GetDiskUsageLimitPercent(); got != 85 {
		t.Fatalf("persisted limit = %d, want 85", got)
	}

	c, w = newFormTestContext("/disk-usage/config", url.Values{})
	c.Set("role", "admin")
	GetDiskUsageConfig(c)

	if w.Code != http.StatusOK {
		t.Fatalf("get config status = %d, body = %q", w.Code, w.Body.String())
	}
	var getResp struct {
		LimitPercent   int `json:"limit_percent"`
		DefaultPercent int `json:"default_percent"`
		MinPercent     int `json:"min_percent"`
		MaxPercent     int `json:"max_percent"`
		Current        *struct {
			UsedPercent float64 `json:"used_percent"`
			FreeBytes   uint64  `json:"free_bytes"`
			TotalBytes  uint64  `json:"total_bytes"`
		} `json:"current"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if getResp.LimitPercent != 85 ||
		getResp.DefaultPercent != logic.DefaultDiskUsageLimitPercent ||
		getResp.MinPercent != logic.MinDiskUsageLimitPercent ||
		getResp.MaxPercent != logic.MaxDiskUsageLimitPercent {
		t.Fatalf("get response = %#v, want range metadata with limit 85", getResp)
	}
	if getResp.Current == nil || getResp.Current.UsedPercent != 42.5 {
		t.Fatalf("current usage = %#v, want 42.5", getResp.Current)
	}
}

func TestDiskUsageConfigOmitsCurrentUsageWhenQueryFails(t *testing.T) {
	setupDiskSpaceControllerTest(t)
	stubDiskUsageError(t, errors.New("statfs failed"))

	c, w := newFormTestContext("/disk-usage/config", url.Values{})
	c.Set("role", "admin")
	GetDiskUsageConfig(c)

	if w.Code != http.StatusOK {
		t.Fatalf("get config status = %d, body = %q", w.Code, w.Body.String())
	}

	var resp struct {
		LimitPercent int             `json:"limit_percent"`
		Current      json.RawMessage `json:"current"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if resp.LimitPercent != logic.DefaultDiskUsageLimitPercent {
		t.Fatalf("limit_percent = %d, want %d", resp.LimitPercent, logic.DefaultDiskUsageLimitPercent)
	}
	if len(resp.Current) != 0 {
		t.Fatalf("current = %s, want omitted when disk query fails", string(resp.Current))
	}
}

func TestDiskUsageConfigRejectsOutOfRangeValues(t *testing.T) {
	setupDiskSpaceControllerTest(t)

	for _, body := range []string{
		`{"limit_percent":79}`,
		`{"limit_percent":100}`,
		`{"limit_percent":0}`,
	} {
		c, w := newJSONTestContext("/config/disk-usage", body)
		c.Set("role", "admin")
		SetDiskUsageConfig(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, w.Code, http.StatusBadRequest)
		}
		if got := logic.GetDiskUsageLimitPercent(); got != logic.DefaultDiskUsageLimitPercent {
			t.Fatalf("limit after rejected body %s = %d, want %d", body, got, logic.DefaultDiskUsageLimitPercent)
		}
	}
}
