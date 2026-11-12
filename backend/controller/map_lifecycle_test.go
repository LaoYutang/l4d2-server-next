package controller

import (
	"encoding/json"
	"errors"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/model"
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

func lifecycleControllerItems(t *testing.T) []logic.MapLifecycleItem {
	t.Helper()
	items, err := logic.ListMapLifecycleItems()
	if err != nil {
		t.Fatal(err)
	}
	return items
}

func TestMapLifecycleHTTPProtectsLockedMapsAcrossDeleteEntrypoints(t *testing.T) {
	_, addons := setupRemoveTestPaths(t)
	for _, name := range []string{"locked.vpk", "first.vpk", "second.vpk"} {
		if err := os.WriteFile(filepath.Join(addons, name), []byte("map"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(consts.MapListFilePath, []byte("locked.vpk\nfirst.vpk\nsecond.vpk\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Existing maps have no upload time but remain manually removable.
	items := lifecycleControllerItems(t)
	if len(items) != 3 || items[0].UploadedAt != nil {
		t.Fatalf("legacy map lifecycle = %#v", items)
	}
	locked, err := logic.SetMapLocked("locked.vpk", true)
	if err != nil {
		t.Fatal(err)
	}

	context, recorder := newFormTestContext("/remove", url.Values{"map": {"locked.vpk"}})
	Remove(context)
	var result logic.MapDeleteResult
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &result) != nil || len(result.SkippedLocked) != 1 {
		t.Fatalf("single deletion: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	context, recorder = newJSONControllerContext("/maps/remove-batch", "guest", `{"maps":["locked.vpk","first.vpk"],"revisions":{"locked.vpk":"`+locked.Revision+`"}}`)
	RemoveMapsBatch(context)
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &result) != nil || len(result.Deleted) != 1 || result.Deleted[0] != "first.vpk" || len(result.SkippedLocked) != 1 {
		t.Fatalf("batch deletion: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	context, recorder = newFormTestContext("/clear", nil)
	Clear(context)
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &result) != nil || len(result.Deleted) != 1 || result.Deleted[0] != "second.vpk" || len(result.SkippedLocked) != 1 {
		t.Fatalf("clear: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if _, err := os.Stat(filepath.Join(addons, "locked.vpk")); err != nil {
		t.Fatalf("locked file lost: %v", err)
	}
	list, err := os.ReadFile(consts.MapListFilePath)
	if err != nil || string(list) != "locked.vpk\n" {
		t.Fatalf("remaining map list = %q, err=%v", list, err)
	}
}

func TestMapLifecycleHTTPValidatesCompleteBatchBeforeDeleting(t *testing.T) {
	_, addons := setupRemoveTestPaths(t)
	mapPath := filepath.Join(addons, "keep.vpk")
	if err := os.WriteFile(mapPath, []byte("map"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(consts.MapListFilePath, []byte("keep.vpk\n"), 0644); err != nil {
		t.Fatal(err)
	}
	context, recorder := newJSONControllerContext("/maps/remove-batch", "guest", `{"maps":["keep.vpk","../outside.vpk"]}`)
	RemoveMapsBatch(context)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid batch: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if _, err := os.Stat(mapPath); err != nil {
		t.Fatalf("valid prefix deleted before validation: %v", err)
	}
}

func TestMapLifecycleLockHTTPPermissionsAndRevision(t *testing.T) {
	_, addons := setupRemoveTestPaths(t)
	if err := os.WriteFile(filepath.Join(addons, "map.vpk"), []byte("map"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(consts.MapListFilePath, []byte("map.vpk\n"), 0644); err != nil {
		t.Fatal(err)
	}
	items := lifecycleControllerItems(t)
	for _, role := range []string{"guest", "map_uploader"} {
		context, recorder := newJSONControllerContext("/maps/lock", role, `{"map":"map.vpk","locked":true}`)
		SetMapLocked(context)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s lock status=%d body=%s", role, recorder.Code, recorder.Body.String())
		}
	}
	context, recorder := newJSONControllerContext("/maps/lock", "admin", `{"map":"map.vpk","locked":true,"expected_revision":"`+items[0].Revision+`"}`)
	SetMapLocked(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("admin lock status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	context, recorder = newJSONControllerContext("/maps/lock", "admin", `{"map":"map.vpk","locked":false,"expected_revision":"`+items[0].Revision+`"}`)
	SetMapLocked(context)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("stale revision status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !lifecycleControllerItems(t)[0].Locked {
		t.Fatal("stale request unlocked the map")
	}
}

func TestMapImportLifecycleRecordsSuccessAndPreservesOnRename(t *testing.T) {
	root, addons := setupMapImportTest(t)
	source := filepath.Join(root, "map.vpk")
	content := mapImportVPK(t, map[string][]byte{"missions/map.txt": []byte("metadata")})
	if err := os.WriteFile(source, content, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ProcessFile(source); err != nil {
		t.Fatal(err)
	}
	items := lifecycleControllerItems(t)
	if len(items) != 1 || items[0].UploadedAt == nil {
		t.Fatalf("successful import lifecycle = %#v", items)
	}
	locked, err := logic.SetMapLocked("map.vpk", true)
	if err != nil {
		t.Fatal(err)
	}
	context, recorder := newFormTestContext("/rename", url.Values{"oldName": {"map.vpk"}, "newName": {"renamed.vpk"}})
	RenameMap(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("rename status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	renamed := lifecycleControllerItems(t)[0]
	if renamed.Name != "renamed.vpk" || renamed.ID != locked.ID || !renamed.Locked || renamed.UploadedAt == nil || !renamed.UploadedAt.Equal(*locked.UploadedAt) {
		t.Fatalf("rename lost lifecycle: before=%#v after=%#v", locked, renamed)
	}
	if _, err := os.Stat(filepath.Join(addons, "renamed.vpk")); err != nil {
		t.Fatal(err)
	}
}

func TestMapLifecycleRenameRecoversJournalBeforeReadingMapList(t *testing.T) {
	_, addons := setupRemoveTestPaths(t)
	for _, name := range []string{"restore.vpk", "rename.vpk"} {
		if err := os.WriteFile(filepath.Join(addons, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	beforeList := []byte("restore.vpk\nrename.vpk\n")
	if err := os.WriteFile(consts.MapListFilePath, beforeList, 0644); err != nil {
		t.Fatal(err)
	}
	lifecycleControllerItems(t)
	storePath := filepath.Join(consts.ManagerDataPath, "map_lifecycle.json")
	before, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	var after struct {
		Version int                                 `json:"version"`
		StoreID string                              `json:"store_id"`
		Maps    map[string]logic.MapLifecycleRecord `json:"maps"`
	}
	if err := json.Unmarshal(before, &after); err != nil {
		t.Fatal(err)
	}
	delete(after.Maps, "restore.vpk")
	operationID := uuid.NewString()
	quarantineName := operationID + ".vpk"
	quarantineDir := filepath.Join(addons, ".map_lifecycle_trash")
	if err := os.Mkdir(quarantineDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(addons, "restore.vpk"), filepath.Join(quarantineDir, quarantineName)); err != nil {
		t.Fatal(err)
	}
	// Simulate a deletion interrupted after publishing its shortened map list,
	// before the operation was committed. The next mutation must recover first.
	journal, err := json.Marshal(map[string]any{
		"version": 1, "id": operationID, "kind": "delete", "phase": "prepared",
		"addons_path": addons, "map_list_path": consts.MapListFilePath,
		"names": []string{"restore.vpk"}, "quarantine": quarantineName, "file_present": true,
		"before": json.RawMessage(before), "after": after,
		"before_list": beforeList, "before_list_mode": uint32(0644), "list_existed": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consts.ManagerDataPath, "map_lifecycle.journal.json"), journal, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(consts.MapListFilePath, []byte("rename.vpk\n"), 0644); err != nil {
		t.Fatal(err)
	}
	context, recorder := newFormTestContext("/rename", url.Values{"oldName": {"rename.vpk"}, "newName": {"renamed.vpk"}})
	RenameMap(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("rename after recovery: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	list, err := os.ReadFile(consts.MapListFilePath)
	if err != nil || string(list) != "restore.vpk\nrenamed.vpk\n" {
		t.Fatalf("rename discarded recovered map: maplist=%q err=%v", list, err)
	}
	if _, err := os.Stat(filepath.Join(addons, "restore.vpk")); err != nil {
		t.Fatalf("recovered map missing: %v", err)
	}
}

func TestMapImportLifecycleFailureRollsBackInstalledFiles(t *testing.T) {
	root, addons := setupMapImportTest(t)
	blocked := filepath.Join(root, "blocked-data")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}
	consts.ManagerDataPath = blocked
	source := filepath.Join(root, "map.vpk")
	if err := os.WriteFile(source, mapImportVPK(t, map[string][]byte{"missions/map.txt": []byte("metadata")}), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ProcessFile(source); err == nil {
		t.Fatal("import succeeded without durable lifecycle state")
	}
	if _, err := os.Stat(filepath.Join(addons, "map.vpk")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed import left installed map: %v", err)
	}
	list, err := os.ReadFile(consts.MapListFilePath)
	if err != nil || len(list) != 0 {
		t.Fatalf("failed import changed map list: %q, err=%v", list, err)
	}
}

func TestMapLifecycleMaintenancePreservesUploadTimeAndLock(t *testing.T) {
	root, _ := setupMapImportTest(t)
	source := filepath.Join(root, "editable.vpk")
	content := mapImportVPK(t, map[string][]byte{
		"missions/editable.txt":               []byte("metadata"),
		"scripts/vscripts/mapspawn_addon.nut": []byte("// original\n"),
		"sound/large.wav":                     []byte(strings.Repeat("sound", 1024)),
	})
	if err := os.WriteFile(source, content, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ProcessFile(source); err != nil {
		t.Fatal(err)
	}
	before, err := logic.SetMapLocked("editable.vpk", true)
	if err != nil {
		t.Fatal(err)
	}
	context, recorder := newFormTestContext("/maps/trim", url.Values{"map": {"editable.vpk"}})
	TrimMap(context)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"trimmed":true`) {
		t.Fatalf("trim status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	afterTrim := lifecycleControllerItems(t)[0]
	if afterTrim.ID != before.ID || !afterTrim.Locked || afterTrim.UploadedAt == nil || !afterTrim.UploadedAt.Equal(*before.UploadedAt) || afterTrim.Size >= before.Size || afterTrim.Revision == before.Revision {
		t.Fatalf("trim lifecycle changed incorrectly: before=%#v after=%#v", before, afterTrim)
	}
	_, scriptRevision, err := logic.GetMapGlobalScriptContentsWithRevision("editable.vpk")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]string{
		"map": "editable.vpk", "path": "scripts/vscripts/mapspawn_addon.nut",
		"content": "// edited longer script\n", "encoding": "utf-8", "expected_revision": scriptRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	context, recorder = newJSONControllerContext("/maps/inspection/global-scripts/update", "admin", string(body))
	UpdateMapGlobalScript(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("script edit status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	afterEdit := lifecycleControllerItems(t)[0]
	if afterEdit.ID != before.ID || !afterEdit.Locked || afterEdit.UploadedAt == nil || !afterEdit.UploadedAt.Equal(*before.UploadedAt) || afterEdit.Revision == afterTrim.Revision {
		t.Fatalf("script edit lifecycle changed incorrectly: before=%#v after=%#v", before, afterEdit)
	}
}

func TestMapLifecyclePartialDeleteKeepsResultsAndAuditsFailure(t *testing.T) {
	var records []model.AuditLog
	previous := enqueueAuditLog
	enqueueAuditLog = func(record model.AuditLog) { records = append(records, record) }
	t.Cleanup(func() { enqueueAuditLog = previous })
	context, recorder := newJSONControllerContext("/maps/remove-batch", "admin", `{}`)
	finishAudit := LogOp(context, "partial deletion")
	respondMapDeleteResult(context, logic.MapDeleteResult{
		Deleted: []string{"removed.vpk"},
		Failed:  []logic.MapDeleteFailure{{Name: "keep.vpk", Error: "permission denied"}},
	}, errors.New("transaction recovery failed"))
	finishAudit()
	if recorder.Code != http.StatusOK || len(records) != 1 || records[0].Success {
		t.Fatalf("partial result audit: status=%d records=%#v", recorder.Code, records)
	}
	var result logic.MapDeleteResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil || len(result.Deleted) != 1 || len(result.Failed) != 1 {
		t.Fatalf("partial result lost: %s", recorder.Body.String())
	}
}

func newJSONControllerContext(path, role, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("role", role)
	return context, recorder
}
