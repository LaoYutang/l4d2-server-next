package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"l4d2-manager-next/consts"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestUploadRepeatedInterruptionsResumeCompletedChunks(t *testing.T) {
	_, addons := setupMapImportTest(t)
	data := bytes.Repeat([]byte("X"), 5<<20)
	id, path := uploadTaskFixture(t, int64(2*len(data)+1), 3)
	c, w := chunkDataContext(t, id, 0, data)
	UploadChunk(c)
	assertUploadResponse(t, w, 200, "OK")
	previousCopy := copyUploadData
	t.Cleanup(func() { copyUploadData = previousCopy })
	for attempt, deficit := range []int64{16 << 10, 16 << 10, 16 << 10, 16 << 10, 96 << 10} {
		c, w = chunkDataContext(t, id, 1, data)
		ctx, abort := context.WithCancel(c.Request.Context())
		c.Request = c.Request.WithContext(ctx)
		copyUploadData = func(dst io.Writer, src io.Reader) (int64, error) {
			n, err := io.CopyN(dst, src, int64(len(data))-deficit)
			abort()
			if err != nil {
				return n, err
			}
			rest, err := previousCopy(dst, src)
			return n + rest, err
		}
		UploadChunk(c)
		abort()
		assertUploadResponse(t, w, 400, "中断")
		assertUploadedChunks(t, id, 0)
		assertUploadFile(t, filepath.Join(path, "0"), data)
		assertNoUploadTemps(t, path)
		t.Logf("resume attempt %d kept completed chunks after interruption %d bytes before EOF", attempt+1, deficit)
	}
	copyUploadData = previousCopy
	for i, content := range [][]byte{data, []byte("Z")} {
		c, w = chunkDataContext(t, id, i+1, content)
		UploadChunk(c)
		assertUploadResponse(t, w, 200, "OK")
	}
	assertUploadedChunks(t, id, 0, 1, 2)
	c, w = uploadActionContext(id, "merge")
	UploadMerge(c)
	assertUploadResponse(t, w, 200, "上传成功！")
	want := append(append(append([]byte(nil), data...), data...), 'Z')
	assertUploadFile(t, filepath.Join(addons, "map.vpk"), want)
}

func TestUploadMergeRejectsObservedByteDeficits(t *testing.T) {
	// The incident logs report aggregate deficits, not which request caused
	// them. These cases verify rejection without attributing a root cause.
	for _, deficit := range []int{16 << 10, 96 << 10} {
		t.Run(strconv.Itoa(deficit), func(t *testing.T) {
			_, addons := setupMapImportTest(t)
			id, _ := uploadTaskFixture(t, (5<<20)+1, 2)
			for i, content := range [][]byte{make([]byte, (5<<20)-deficit), {1}} {
				c, w := chunkDataContext(t, id, i, content)
				UploadChunk(c)
				// There is no negotiated chunk size/hash in the existing API.
				assertUploadResponse(t, w, 200, "OK")
			}
			c, w := uploadActionContext(id, "merge")
			UploadMerge(c)
			assertUploadResponse(t, w, 500, "重新上传")
			if _, err := os.Stat(filepath.Join(addons, "map.vpk")); !os.IsNotExist(err) {
				t.Fatal("size-mismatched upload was imported")
			}
			assertUploadFile(t, consts.MapListFilePath, nil)
		})
	}
}

func TestUploadChunkValidationAndCompletedConflict(t *testing.T) {
	for _, test := range []struct {
		name  string
		index int
		data  []byte
	}{
		{"negative_index", -1, []byte("DATA")},
		{"out_of_range_index", 1, []byte("DATA")},
		{"empty", 0, nil},
		{"larger_than_file", 0, []byte("LARGER")},
		{"over_limit", 0, make([]byte, maxUploadChunkSize+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupChunkUploadTestPaths(t)
			id, path := uploadTaskFixture(t, 4, 1)
			c, w := chunkDataContext(t, id, test.index, test.data)
			UploadChunk(c)
			assertUploadResponse(t, w, 400, "")
			assertUploadedChunks(t, id)
			assertNoUploadTemps(t, path)
		})
	}
	t.Run("completed_size_conflict", func(t *testing.T) {
		setupChunkUploadTestPaths(t)
		id, path := uploadTaskFixture(t, 8, 2)
		if err := os.WriteFile(filepath.Join(path, "0"), []byte("KEEP"), 0644); err != nil {
			t.Fatal(err)
		}
		c, w := chunkDataContext(t, id, 0, []byte("short"))
		UploadChunk(c)
		assertUploadResponse(t, w, 400, "重新上传")
		assertUploadFile(t, filepath.Join(path, "0"), []byte("KEEP"))
		assertNoUploadTemps(t, path)
	})
}

func TestUploadTruncatedMultipartDoesNotPublishChunk(t *testing.T) {
	setupChunkUploadTestPaths(t)
	id, path := uploadTaskFixture(t, 8, 2)
	if err := os.WriteFile(filepath.Join(path, "1"), []byte("KEEP"), 0644); err != nil {
		t.Fatal(err)
	}
	c, w := chunkDataContext(t, id, 0, []byte("DATA"))
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		t.Fatal(err)
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body[:len(body)-30]))
	UploadChunk(c)
	assertUploadResponse(t, w, http.StatusBadRequest, "")
	assertUploadedChunks(t, id, 1)
	assertUploadFile(t, filepath.Join(path, "1"), []byte("KEEP"))
	assertNoUploadTemps(t, path)
}

func TestUploadSizeConflictResumeRequiresFreshTaskWithoutRepairingChunk(t *testing.T) {
	_, addons := setupMapImportTest(t)
	id, path := uploadTaskFixture(t, 8, 2)
	if err := os.WriteFile(filepath.Join(path, "0"), []byte("BAD"), 0644); err != nil {
		t.Fatal(err)
	}
	c, w := chunkDataContext(t, id, 0, []byte("GOOD"))
	UploadChunk(c)
	assertUploadResponse(t, w, 400, "请重新上传")
	assertUploadFile(t, filepath.Join(path, "0"), []byte("BAD"))
	// The old client offers Continue after a chunk error, then skips completed
	// indices from status. Reproduce that exact sequence: no hidden deletion,
	// replacement, or bad-chunk repair can make the old task appear successful.
	assertUploadedChunks(t, id, 0)
	c, w = chunkDataContext(t, id, 1, []byte("REST"))
	UploadChunk(c)
	assertUploadResponse(t, w, 200, "OK")
	c, w = uploadActionContext(id, "merge")
	UploadMerge(c)
	assertUploadResponse(t, w, 500, "请重新上传")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("size-mismatched task was not discarded")
	}
	if _, err := os.Stat(filepath.Join(addons, "map.vpk")); !os.IsNotExist(err) {
		t.Fatal("continuing the conflicting upload installed a map")
	}
	c, w = chunkDataContext(t, id, 0, []byte("GOOD"))
	UploadChunk(c)
	assertUploadResponse(t, w, 400, "请重新上传")
	freshID, _ := uploadTaskFixture(t, 8, 2)
	for i, data := range [][]byte{[]byte("GOOD"), []byte("REST")} {
		c, w = chunkDataContext(t, freshID, i, data)
		UploadChunk(c)
		assertUploadResponse(t, w, 200, "OK")
	}
	c, w = uploadActionContext(freshID, "merge")
	UploadMerge(c)
	assertUploadResponse(t, w, 200, "上传成功！")
	assertUploadFile(t, filepath.Join(addons, "map.vpk"), []byte("GOODREST"))
}

func TestUploadMergePreflightRejectsInvalidChunks(t *testing.T) {
	for _, test := range []struct {
		name string
		size int64
		code int
		kind string
	}{
		{"missing", 4, 400, "missing"},
		{"short", 8, 500, "file"},
		{"long", 2, 500, "file"},
		{"empty", 1, 400, "empty"},
		{"directory", 4, 400, "directory"},
		{"symlink", 4, 400, "symlink"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, addons := setupMapImportTest(t)
			id, path := uploadTaskFixture(t, test.size, 1)
			outside := filepath.Join(root, "outside.vpk")
			if err := os.WriteFile(outside, []byte("KEEP"), 0644); err != nil {
				t.Fatal(err)
			}
			var err error
			switch test.kind {
			case "file":
				err = os.WriteFile(filepath.Join(path, "0"), []byte("DATA"), 0644)
			case "empty":
				err = os.WriteFile(filepath.Join(path, "0"), nil, 0644)
			case "directory":
				err = os.Mkdir(filepath.Join(path, "0"), 0755)
			case "symlink":
				err = os.Symlink(outside, filepath.Join(path, "0"))
			}
			if err != nil {
				t.Fatal(err)
			}
			c, w := uploadActionContext(id, "merge")
			UploadMerge(c)
			assertUploadResponse(t, w, test.code, "重新上传")
			if _, err := os.Stat(filepath.Join(addons, "map.vpk")); !os.IsNotExist(err) {
				t.Fatal("invalid upload was imported")
			}
			assertUploadFile(t, outside, []byte("KEEP"))
			assertNoUploadTemps(t, path)
		})
	}
}

func TestUploadRejectsMalformedMetadata(t *testing.T) {
	for _, meta := range []string{"map.vpk", "map.vpk\nwrong\n1\n", "map.vpk\n4\n0\n", "map.vpk\n-4\n1\n", "map.vpk\n4\n-1\n", "map.vpk\n4\n5\n"} {
		t.Run(fmt.Sprintf("%q", meta), func(t *testing.T) {
			setupChunkUploadTestPaths(t)
			id, path := uploadTaskFixture(t, 4, 1)
			if err := os.WriteFile(filepath.Join(path, ".meta"), []byte(meta), 0644); err != nil {
				t.Fatal(err)
			}
			c, w := chunkDataContext(t, id, 0, []byte("DATA"))
			UploadChunk(c)
			assertUploadResponse(t, w, 500, "重新上传")
			c, w = uploadActionContext(id, "merge")
			UploadMerge(c)
			assertUploadResponse(t, w, 500, "重新上传")
			assertNoUploadTemps(t, path)
		})
	}
}

func TestUploadIOFailuresDoNotPublishOrImport(t *testing.T) {
	for _, test := range []struct {
		name  string
		merge bool
		code  int
	}{
		{"chunk_write", false, 500},
		{"chunk_short_copy", false, 400},
		{"chunk_wrong_copy_count", false, 500},
		{"chunk_rename", false, 500},
		{"merge_write", true, 500},
		{"merge_rename", true, 500},
		{"merge_short_copy", true, 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, addons := setupMapImportTest(t)
			id, path := uploadTaskFixture(t, 4, 1)
			if test.merge {
				if err := os.WriteFile(filepath.Join(path, "0"), []byte("DATA"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			previousCopy, previousRename := copyUploadData, renameUploadFile
			t.Cleanup(func() { copyUploadData, renameUploadFile = previousCopy, previousRename })
			if strings.HasSuffix(test.name, "rename") {
				renameUploadFile = func(string, string) error { return os.ErrPermission }
			} else {
				copyUploadData = func(dst io.Writer, src io.Reader) (int64, error) {
					n, err := io.CopyN(dst, src, 3)
					if err != nil {
						return n, err
					}
					if strings.HasSuffix(test.name, "write") {
						return n, syscall.ENOSPC
					}
					if strings.HasSuffix(test.name, "wrong_copy_count") {
						return 4, nil
					}
					return n, nil
				}
			}
			c, w := chunkDataContext(t, id, 0, []byte("DATA"))
			if test.merge {
				c, w = uploadActionContext(id, "merge")
				UploadMerge(c)
			} else {
				UploadChunk(c)
			}
			assertUploadResponse(t, w, test.code, "")
			assertNoUploadTemps(t, path)
			if _, err := os.Stat(filepath.Join(addons, "map.vpk")); !os.IsNotExist(err) {
				t.Fatal("failed write or publication installed a map")
			}
			if !test.merge {
				assertUploadedChunks(t, id)
			} else if test.name != "merge_short_copy" {
				assertUploadedChunks(t, id, 0)
				assertUploadFile(t, filepath.Join(path, "0"), []byte("DATA"))
			}
			copyUploadData, renameUploadFile = previousCopy, previousRename
			if test.name != "merge_short_copy" {
				if !test.merge {
					c, w = chunkDataContext(t, id, 0, []byte("DATA"))
					UploadChunk(c)
					assertUploadResponse(t, w, 200, "OK")
				}
				c, w = uploadActionContext(id, "merge")
				UploadMerge(c)
				assertUploadResponse(t, w, 200, "上传成功！")
				assertUploadFile(t, filepath.Join(addons, "map.vpk"), []byte("DATA"))
			}
		})
	}
}

func TestUploadImportCannotRepeatAfterCleanupFailure(t *testing.T) {
	_, addons := setupMapImportTest(t)
	id, path := uploadTaskFixture(t, 7, 3)
	for i, data := range [][]byte{[]byte("AB"), []byte("CDEF"), []byte("G")} {
		c, w := chunkDataContext(t, id, i, data)
		UploadChunk(c)
		assertUploadResponse(t, w, 200, "OK")
	}
	previousRemove, previousInstall := removeUploadTempDir, installMapImportFile
	t.Cleanup(func() { removeUploadTempDir, installMapImportFile = previousRemove, previousInstall })
	installs := 0
	installMapImportFile = func(src, dst string) error { installs++; return previousInstall(src, dst) }
	removeUploadTempDir = func(string) error { return os.ErrPermission }
	c, w := uploadActionContext(id, "merge")
	UploadMerge(c)
	assertUploadResponse(t, w, 500, "清理临时文件失败")
	assertUploadFile(t, filepath.Join(addons, "map.vpk"), []byte("ABCDEFG"))
	if _, err := os.Stat(filepath.Join(path, uploadImportMarker)); err != nil {
		t.Fatal("cleanup failure lost the import marker")
	}
	c, w = uploadActionContext(id, "merge")
	UploadMerge(c)
	assertUploadResponse(t, w, 409, "已进入导入阶段")
	c, w = chunkDataContext(t, id, 0, []byte("AB"))
	UploadChunk(c)
	assertUploadResponse(t, w, 409, "已进入导入阶段")
	if installs != 1 {
		t.Fatalf("map installed %d times", installs)
	}
	removeUploadTempDir = previousRemove
	c, w = uploadActionContext(id, "cancel")
	UploadCancel(c)
	assertUploadResponse(t, w, 200, "已取消")
}

func TestUploadAbandonedImportMarkerDoesNotClaimSuccessOrRetryImport(t *testing.T) {
	for _, phase := range []string{"before_install", "after_install_before_record", "after_import"} {
		t.Run(phase, func(t *testing.T) {
			_, addons := setupMapImportTest(t)
			id, path := uploadTaskFixture(t, 4, 1)
			if err := os.WriteFile(filepath.Join(path, "0"), []byte("DATA"), 0644); err != nil {
				t.Fatal(err)
			}
			// There is no live task guard. Only disk state survives a manager
			// restart, and a marker does not identify which import step ran.
			if err := os.WriteFile(filepath.Join(path, uploadImportMarker), nil, 0600); err != nil {
				t.Fatal(err)
			}
			mapPath := filepath.Join(addons, "map.vpk")
			if phase != "before_install" {
				if err := os.WriteFile(mapPath, []byte("DATA"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			list := []byte(nil)
			if phase == "after_import" {
				list = []byte("map.vpk\n")
				if err := os.WriteFile(consts.MapListFilePath, list, 0644); err != nil {
					t.Fatal(err)
				}
			}
			// This response describes completed chunks, never installed maps.
			// The old client's next step is synchronous merge; it must fail
			// explicitly instead of treating the marker as a successful import.
			assertUploadedChunks(t, id, 0)
			c, w := uploadActionContext(id, "merge")
			UploadMerge(c)
			assertUploadResponse(t, w, 409, "检查地图列表")
			c, w = chunkDataContext(t, id, 0, []byte("DATA"))
			UploadChunk(c)
			assertUploadResponse(t, w, 409, "不能安全续传")
			assertUploadFile(t, consts.MapListFilePath, list)
			if phase == "before_install" {
				if _, err := os.Stat(mapPath); !os.IsNotExist(err) {
					t.Fatal("an abandoned attempt was automatically imported")
				}
			} else {
				assertUploadFile(t, mapPath, []byte("DATA"))
			}
			c, w = uploadActionContext(id, "cancel")
			UploadCancel(c)
			assertUploadResponse(t, w, 200, "已取消")
			if phase == "before_install" {
				freshID, _ := uploadTaskFixture(t, 4, 1)
				c, w = chunkDataContext(t, freshID, 0, []byte("DATA"))
				UploadChunk(c)
				assertUploadResponse(t, w, 200, "OK")
				c, w = uploadActionContext(freshID, "merge")
				UploadMerge(c)
				assertUploadResponse(t, w, 200, "上传成功！")
			}
		})
	}
}

func TestUploadMergeInterruptedBeforeImportCanRetryWithoutStaleMarker(t *testing.T) {
	_, addons := setupMapImportTest(t)
	id, path := uploadTaskFixture(t, 4, 1)
	if err := os.WriteFile(filepath.Join(path, "0"), []byte("DATA"), 0644); err != nil {
		t.Fatal(err)
	}
	c, w := uploadActionContext(id, "merge")
	ctx, abort := context.WithCancel(c.Request.Context())
	defer abort()
	c.Request = c.Request.WithContext(ctx)
	previousRename := renameUploadFile
	t.Cleanup(func() { renameUploadFile = previousRename })
	renameUploadFile = func(src, dst string) error {
		err := previousRename(src, dst)
		// The merged file is complete, but the request has disconnected
		// before the following import boundary and ProcessFile call.
		abort()
		return err
	}
	UploadMerge(c)
	assertUploadResponse(t, w, 400, "中断")
	if _, err := os.Stat(filepath.Join(path, uploadImportMarker)); !os.IsNotExist(err) {
		t.Fatal("a known unstarted import left a terminal marker")
	}
	if _, err := os.Stat(filepath.Join(addons, "map.vpk")); !os.IsNotExist(err) {
		t.Fatal("interrupted request started importing a map")
	}
	assertUploadFile(t, filepath.Join(path, "0"), []byte("DATA"))
	assertUploadedChunks(t, id, 0)
	renameUploadFile = previousRename
	c, w = uploadActionContext(id, "merge")
	UploadMerge(c)
	assertUploadResponse(t, w, 200, "上传成功！")
	assertUploadFile(t, filepath.Join(addons, "map.vpk"), []byte("DATA"))
}

func TestUploadExpiredWriterFailsAfterExistingCleanerRuns(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("exercises Linux unlink of an open private temporary file")
	}
	setupChunkUploadTestPaths(t)
	id, path := uploadTaskFixture(t, 4, 1)
	old := time.Now().Add(-7 * time.Hour)
	if err := os.Chtimes(filepath.Join(path, ".meta"), old, old); err != nil {
		t.Fatal(err)
	}
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
	c, w := chunkDataContext(t, id, 0, []byte("DATA"))
	done := startUploadHandler(t, UploadChunk, c)
	awaitUploadSignal(t, started)
	// The requested expiry policy is unchanged: the cleaner uses .meta age,
	// including for active tasks. Losing an expired directory must not turn
	// a still-open, unlinked private file into a completed chunk or HTTP 200.
	cleanStaleUploads()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("existing expiry cleaner did not remove the expired task")
	}
	releaseWriter()
	awaitUploadSignal(t, done)
	assertUploadResponse(t, w, 500, "确认分片失败")
	assertUploadedChunks(t, id)
}
