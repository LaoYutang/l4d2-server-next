package controller

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/pkg/valve/vpk"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func setupMapImportTest(t *testing.T) (string, string) {
	t.Helper()
	root, addons := setupChunkUploadTestPaths(t)
	oldData, oldConfig := consts.ManagerDataPath, consts.ManagerConfigPath
	oldPolicy, oldTrim := logic.GetMapImportPolicy(), logic.IsVPKTrimEnabled()
	consts.ManagerDataPath = filepath.Join(root, "data")
	consts.ManagerConfigPath = filepath.Join(root, "data", "manager_config.json")
	t.Cleanup(func() {
		_ = logic.SetMapImportPolicy(oldPolicy)
		_ = logic.SetVPKTrimEnable(oldTrim)
		consts.ManagerDataPath, consts.ManagerConfigPath = oldData, oldConfig
	})
	if err := logic.SetMapImportPolicy(logic.MapImportPolicy{}); err != nil {
		t.Fatal(err)
	}
	if err := logic.SetVPKTrimEnable(false); err != nil {
		t.Fatal(err)
	}
	return root, addons
}

func mapImportVPK(t *testing.T, contents map[string][]byte) []byte {
	t.Helper()
	files := make([]vpk.File, 0, len(contents))
	for name := range contents {
		ext := pathpkg.Ext(name)
		dir := pathpkg.Dir(name)
		if dir == "." {
			dir = " "
		}
		files = append(files, vpk.File{Dir: dir, Base: strings.TrimSuffix(pathpkg.Base(name), ext), Ext: strings.TrimPrefix(ext, ".")})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Ext != files[j].Ext {
			return files[i].Ext < files[j].Ext
		}
		if files[i].Dir != files[j].Dir {
			return files[i].Dir < files[j].Dir
		}
		return files[i].Base < files[j].Base
	})
	var data bytes.Buffer
	for i := range files {
		content := contents[files[i].Name()]
		files[i].DirEntry = vpk.DirEntry{CRC: crc32.ChecksumIEEE(content), DataLocation: []vpk.DataChunk{{ArchiveIndex: 0x7fff, EntryOffset: uint32(data.Len()), EntryLength: uint32(len(content))}}}
		data.Write(content)
	}
	var out bytes.Buffer
	if err := vpk.WriteDirectory(&out, &vpk.Archive{Header: vpk.Header{Magic: vpk.Magic, Version: 1}, Files: files}); err != nil {
		t.Fatal(err)
	}
	out.Write(data.Bytes())
	return out.Bytes()
}

func mapImportBSP(t *testing.T, dictionary bool) []byte {
	t.Helper()
	var pak bytes.Buffer
	zw := zip.NewWriter(&pak)
	name := "readme.txt"
	if dictionary {
		name = "stringtable_dictionary.dct"
	}
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("test")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 4+4+64*16+4)
	copy(header, "VBSP")
	binary.LittleEndian.PutUint32(header[4:8], 21)
	const lump = 8 + 40*16
	binary.LittleEndian.PutUint32(header[lump:lump+4], uint32(len(header)))
	binary.LittleEndian.PutUint32(header[lump+4:lump+8], uint32(pak.Len()))
	return append(header, pak.Bytes()...)
}

type mapImportZipEntry struct {
	name string
	data []byte
}

func writeMapImportZip(t *testing.T, path string, entries ...mapImportZipEntry) {
	t.Helper()
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, entry := range entries {
		w, err := zw.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestMapImportRiskSwitchesGateFreshCandidates(t *testing.T) {
	for _, test := range []struct {
		name, rule string
		policy     logic.MapImportPolicy
		contents   map[string][]byte
		raw        []byte
	}{
		{name: "dictionary", rule: "missing_dictionary", policy: logic.MapImportPolicy{RejectMissingDictionary: true}, contents: map[string][]byte{"maps/chapter.bsp": mapImportBSP(t, false)}},
		{name: "global", rule: "global_scripts", policy: logic.MapImportPolicy{RejectGlobalScripts: true}, contents: map[string][]byte{"SCRIPTS/VSCRIPTS/MAPSPAWN_ADDON.NUT": []byte("script")}},
		{name: "override", rule: "script_overrides", policy: logic.MapImportPolicy{RejectScriptOverrides: true}, contents: map[string][]byte{"scripts/weapon_rifle.txt": []byte("weapon")}},
		{name: "bsp error", rule: "inspection_error", policy: logic.MapImportPolicy{RejectInspectionError: true}, contents: map[string][]byte{"maps/broken.bsp": []byte("VBSP")}},
		{name: "vpk error", rule: "inspection_error", policy: logic.MapImportPolicy{RejectInspectionError: true}, raw: []byte("not a supported vpk layout")},
	} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/enabled=%t", test.name, enabled), func(t *testing.T) {
				root, addons := setupMapImportTest(t)
				if enabled {
					if err := logic.SetMapImportPolicy(test.policy); err != nil {
						t.Fatal(err)
					}
				}
				content := test.raw
				if content == nil {
					content = mapImportVPK(t, test.contents)
				}
				source := filepath.Join(root, "map.vpk")
				if err := os.WriteFile(source, content, 0644); err != nil {
					t.Fatal(err)
				}
				_, err := ProcessFile(source)
				if !enabled {
					if err != nil {
						t.Fatal(err)
					}
					info, err := os.Stat(filepath.Join(addons, "map.vpk"))
					if err != nil {
						t.Fatal(err)
					}
					if logic.GetMapVPKInspection("map.vpk", info).CheckedAt == "" {
						t.Fatal("warning-only import lost inspection")
					}
					return
				}
				var rejection *logic.MapImportRejectedError
				if !errors.As(err, &rejection) || len(rejection.Violations) != 1 || rejection.Violations[0].Rule != test.rule {
					t.Fatalf("rejection = %v", err)
				}
				if _, err := os.Stat(filepath.Join(addons, "map.vpk")); !os.IsNotExist(err) {
					t.Fatal("rejected map installed")
				}
				list, _ := os.ReadFile(consts.MapListFilePath)
				if len(list) != 0 {
					t.Fatalf("rejected map recorded: %q", list)
				}
				if _, err := os.Stat(consts.MapVPKInspectionsPath); !os.IsNotExist(err) {
					t.Fatal("rejected import persisted inspection")
				}
			})
		}
	}
}

func TestMapImportZipRejectsWholeBatchAndSanitizedCollisions(t *testing.T) {
	for _, collision := range []bool{false, true} {
		t.Run(fmt.Sprint(collision), func(t *testing.T) {
			root, addons := setupMapImportTest(t)
			if err := logic.SetMapImportPolicy(logic.MapImportPolicy{RejectGlobalScripts: true}); err != nil {
				t.Fatal(err)
			}
			clean := mapImportVPK(t, map[string][]byte{"maps/clean.bsp": mapImportBSP(t, true)})
			risk := mapImportVPK(t, map[string][]byte{"scripts/vscripts/mapspawn_addon.nut": []byte("risk")})
			first, second := "nested/clean.vpk", "nested/risk.vpk"
			if collision {
				first, second = "one/map!.vpk", "two/map@.vpk"
			}
			archive := filepath.Join(root, "batch.zip")
			writeMapImportZip(t, archive, mapImportZipEntry{first, clean}, mapImportZipEntry{second, risk})
			if _, err := ProcessFile(archive); err == nil {
				t.Fatal("batch should fail")
			}
			files, err := filepath.Glob(filepath.Join(addons, "*.vpk"))
			if err != nil || len(files) != 0 {
				t.Fatalf("partial installation: %v %v", files, err)
			}
			list, _ := os.ReadFile(consts.MapListFilePath)
			if len(list) != 0 {
				t.Fatalf("partial map list: %q", list)
			}
			if _, err := os.Stat(consts.MapVPKInspectionsPath); !os.IsNotExist(err) {
				t.Fatal("batch published inspections")
			}
		})
	}
}

func TestMapImportCommitFailuresRestoreExistingState(t *testing.T) {
	for _, failure := range []string{"second file", "inspection store", "map list"} {
		t.Run(failure, func(t *testing.T) {
			root, addons := setupMapImportTest(t)
			clean := mapImportVPK(t, map[string][]byte{"maps/clean.bsp": mapImportBSP(t, true)})
			existing := filepath.Join(addons, "existing.vpk")
			if err := os.WriteFile(existing, clean, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(consts.MapListFilePath, []byte("existing.vpk\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := logic.InspectAndStoreMapVPK("existing.vpk", existing); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(consts.MapVPKInspectionsPath)
			originalPath := consts.MapVPKInspectionsPath
			if failure == "inspection store" {
				consts.MapVPKInspectionsPath = filepath.Join(root, "blocked", "inspections.json")
				if err := os.WriteFile(filepath.Join(root, "blocked"), []byte("keep"), 0644); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { consts.MapVPKInspectionsPath = originalPath })
			}
			oldInstall, oldPublish := installMapImportFile, publishMapImportList
			t.Cleanup(func() { installMapImportFile, publishMapImportList = oldInstall, oldPublish })
			if failure == "second file" {
				count := 0
				installMapImportFile = func(source, dest string) error {
					count++
					if count == 2 {
						return errors.New("injected install failure")
					}
					return oldInstall(source, dest)
				}
			}
			if failure == "map list" {
				publishMapImportList = func(string, []byte, os.FileMode) error { return errors.New("injected map list failure") }
			}
			archive := filepath.Join(root, "batch.zip")
			writeMapImportZip(t, archive, mapImportZipEntry{"first.vpk", clean}, mapImportZipEntry{"second.vpk", clean})
			if _, err := ProcessFile(archive); err == nil {
				t.Fatal("expected commit failure")
			}
			files, _ := filepath.Glob(filepath.Join(addons, "*.vpk"))
			if len(files) != 1 || files[0] != existing {
				t.Fatalf("files after rollback = %v", files)
			}
			list, _ := os.ReadFile(consts.MapListFilePath)
			if string(list) != "existing.vpk\n" {
				t.Fatalf("map list changed: %q", list)
			}
			after, _ := os.ReadFile(originalPath)
			if !bytes.Equal(before, after) {
				t.Fatal("existing inspection file changed")
			}
			contents, _ := os.ReadFile(existing)
			if !bytes.Equal(contents, clean) {
				t.Fatal("existing map changed")
			}
			if failure == "map list" {
				info, _ := os.Stat(existing)
				if logic.GetMapVPKInspection("existing.vpk", info).CheckedAt == "" {
					t.Fatal("existing inspection cache lost")
				}
				if got := logic.GetMapVPKInspection("first.vpk", info); got.CheckedAt != "" {
					t.Fatal("failed batch left live inspection cache")
				}
			}
		})
	}
}

func TestMapImportConcurrentNamesCannotOverwrite(t *testing.T) {
	root, addons := setupMapImportTest(t)
	sources := make([]string, 2)
	contents := [][]byte{mapImportVPK(t, map[string][]byte{"missions/first.txt": []byte("first")}), mapImportVPK(t, map[string][]byte{"missions/second.txt": []byte("second")})}
	for i := range sources {
		dir := filepath.Join(root, fmt.Sprint(i))
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
		sources[i] = filepath.Join(dir, "same.vpk")
		if err := os.WriteFile(sources[i], contents[i], 0644); err != nil {
			t.Fatal(err)
		}
	}
	type result struct {
		index int
		err   error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for i, source := range sources {
		go func() { <-start; _, err := ProcessFile(source); results <- result{i, err} }()
	}
	close(start)
	winner := -1
	for range sources {
		r := <-results
		if r.err == nil {
			if winner != -1 {
				t.Fatal("both imports succeeded")
			}
			winner = r.index
		}
	}
	if winner == -1 {
		t.Fatal("neither import succeeded")
	}
	got, err := os.ReadFile(filepath.Join(addons, "same.vpk"))
	if err != nil || !bytes.Equal(got, contents[winner]) {
		t.Fatal("winner overwritten")
	}
	list, _ := os.ReadFile(consts.MapListFilePath)
	if string(list) != "same.vpk\n" {
		t.Fatalf("map list = %q", list)
	}
}

func TestMapImportListFailureRemovesNewInspectionStore(t *testing.T) {
	root, addons := setupMapImportTest(t)
	oldPublish := publishMapImportList
	publishMapImportList = func(string, []byte, os.FileMode) error { return errors.New("injected map list failure") }
	t.Cleanup(func() { publishMapImportList = oldPublish })
	source := filepath.Join(root, "map.vpk")
	if err := os.WriteFile(source, mapImportVPK(t, map[string][]byte{"missions/map.txt": []byte("metadata")}), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ProcessFile(source); err == nil {
		t.Fatal("expected map list failure")
	}
	if _, err := os.Stat(filepath.Join(addons, "map.vpk")); !os.IsNotExist(err) {
		t.Fatal("failed import left a map")
	}
	if _, err := os.Stat(consts.MapVPKInspectionsPath); !os.IsNotExist(err) {
		t.Fatal("failed import left a new inspection store")
	}
}

func TestMapImportBatchPolicySnapshotAndFinalTrimmedInspection(t *testing.T) {
	root, addons := setupMapImportTest(t)
	if err := logic.SetVPKTrimEnable(true); err != nil {
		t.Fatal(err)
	}
	batch := newMapImportBatch()
	defer batch.cleanup()
	if err := logic.SetMapImportPolicy(logic.MapImportPolicy{RejectGlobalScripts: true}); err != nil {
		t.Fatal(err)
	}
	content := mapImportVPK(t, map[string][]byte{
		"scripts/vscripts/mapspawn_addon.nut": []byte("risk"),
		"materials/unused.vtf":                bytes.Repeat([]byte("texture"), 1024),
	})
	source := filepath.Join(root, "first.vpk")
	if err := os.WriteFile(source, content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := batch.add(source, "first.vpk"); err != nil {
		t.Fatal(err)
	}
	if batch.maps[0].info.Size() >= int64(len(content)) {
		t.Fatal("candidate was not trimmed")
	}
	if _, err := batch.commit(); err != nil {
		t.Fatalf("in-flight batch did not retain snapshot: %v", err)
	}
	info, err := os.Stat(filepath.Join(addons, "first.vpk"))
	if err != nil {
		t.Fatal(err)
	}
	inspection := logic.GetMapVPKInspection("first.vpk", info)
	if inspection.GlobalScripts.Status != logic.GlobalScriptsStatusDetected {
		t.Fatal("final candidate lost warning inspection")
	}
	source = filepath.Join(root, "next-batch.vpk")
	if err := os.WriteFile(source, content, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ProcessFile(source); err == nil {
		t.Fatal("next batch bypassed updated policy")
	} else {
		var rejection *logic.MapImportRejectedError
		if !errors.As(err, &rejection) {
			t.Fatalf("expected policy rejection, got %v", err)
		}
	}
	archive := filepath.Join(root, "clean.zip")
	clean := mapImportVPK(t, map[string][]byte{"maps/test.bsp": mapImportBSP(t, true)})
	writeMapImportZip(t, archive, mapImportZipEntry{"second.vpk", clean}, mapImportZipEntry{"third.vpk", clean})
	names, err := ProcessFile(archive)
	if err != nil || len(names) != 2 {
		t.Fatalf("clean batch failed: %v %v", names, err)
	}
}

func TestMapImportRejectsChangedCandidateAndExistingFile(t *testing.T) {
	root, addons := setupMapImportTest(t)
	content := mapImportVPK(t, map[string][]byte{"maps/clean.bsp": mapImportBSP(t, true)})
	source := filepath.Join(root, "map.vpk")
	if err := os.WriteFile(source, content, 0644); err != nil {
		t.Fatal(err)
	}
	batch := newMapImportBatch()
	defer batch.cleanup()
	if err := batch.add(source, "map.vpk"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(batch.maps[0].path, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := batch.commit(); err == nil {
		t.Fatal("changed candidate accepted")
	}
	if err := os.WriteFile(filepath.Join(addons, "map.vpk"), []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, content, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ProcessFile(source); err == nil {
		t.Fatal("existing map overwritten")
	}
	got, _ := os.ReadFile(filepath.Join(addons, "map.vpk"))
	if string(got) != "existing" {
		t.Fatal("existing contents lost")
	}
}

func TestMapImportPolicyHTTPPermissionsAndUploadRejection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupMapImportTest(t)
	body := `{"reject_missing_dictionary":false,"reject_global_scripts":true,"reject_script_overrides":false,"reject_inspection_error":false}`
	for _, role := range []string{"guest", "map_uploader", "admin"} {
		for _, handler := range []gin.HandlerFunc{GetMapImportPolicy, SetMapImportPolicy} {
			req := httptest.NewRequest(http.MethodPost, "/config/map-import-policy", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Set("role", role)
			handler(c)
			want := http.StatusForbidden
			if role == "admin" {
				want = http.StatusOK
			}
			if w.Code != want {
				t.Fatalf("%s response = %d: %s", role, w.Code, w.Body.String())
			}
		}
	}
	logic.LoadManagerConfig()
	if !logic.GetMapImportPolicy().RejectGlobalScripts {
		t.Fatal("HTTP setting not persisted")
	}
	request := httptest.NewRequest(http.MethodPost, "/config/map-import-policy", strings.NewReader(`{"reject_global_scripts":false}`))
	request.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = request
	c.Set("role", "admin")
	SetMapImportPolicy(c)
	if w.Code != http.StatusBadRequest || !logic.GetMapImportPolicy().RejectGlobalScripts {
		t.Fatal("partial request changed policy")
	}
	uploadID := uuid.NewString()
	temp := getUploadTempPath(uploadID)
	if err := os.MkdirAll(temp, 0755); err != nil {
		t.Fatal(err)
	}
	content := mapImportVPK(t, map[string][]byte{"scripts/vscripts/mapspawn_addon.nut": []byte("risk")})
	if err := os.WriteFile(filepath.Join(temp, "0"), content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(temp, ".meta"), []byte(fmt.Sprintf("map.vpk\n%d\n1\n", len(content))), 0644); err != nil {
		t.Fatal(err)
	}
	c, w = newFormTestContext("/upload/merge", url.Values{"uploadId": {uploadID}, "filename": {"map.vpk"}})
	UploadMerge(c)
	var payload struct {
		Code       string                     `json:"code"`
		Violations []logic.MapImportViolation `json:"violations"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusUnprocessableEntity || payload.Code != "map_import_rejected" || len(payload.Violations) != 1 || len(payload.Violations[0].Paths) != 1 {
		t.Fatalf("response: %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(temp); !os.IsNotExist(err) {
		t.Fatal("rejected upload retained chunks")
	}
}

type mapImportTransport func(*http.Request) (*http.Response, error)

func (transport mapImportTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestMapImportDownloadsUseSamePolicy(t *testing.T) {
	for _, preferredName := range []string{"", "workshop-map.vpk"} {
		t.Run(preferredName, func(t *testing.T) {
			_, addons := setupMapImportTest(t)
			if err := logic.SetMapImportPolicy(logic.MapImportPolicy{RejectScriptOverrides: true}); err != nil {
				t.Fatal(err)
			}
			content := mapImportVPK(t, map[string][]byte{"scripts/gamemodes.txt": []byte("risk")})
			previous := downloadHTTPClient
			downloadHTTPClient = &http.Client{Transport: mapImportTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(content)), ContentLength: int64(len(content))}, nil
			})}
			t.Cleanup(func() { downloadHTTPClient = previous })
			task := NewDownloadTaskWithFilename("https://example.invalid/map.vpk", preferredName, make(chan struct{}, 1))
			select {
			case <-task.done:
			case <-time.After(5 * time.Second):
				task.Cancel()
				t.Fatal("download did not finish")
			}
			if task.GetStatus() != DOWNLOAD_STATUS_FAILED || task.getImportRejection() == nil {
				t.Fatalf("status=%d message=%s", task.GetStatus(), task.GetMessage())
			}
			files, _ := filepath.Glob(filepath.Join(addons, "*.vpk"))
			if len(files) != 0 {
				t.Fatalf("download bypassed policy: %v", files)
			}
		})
	}
}
