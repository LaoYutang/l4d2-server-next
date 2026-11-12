package logic

import (
	"encoding/json"
	"errors"
	"l4d2-manager-next/consts"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func setupLifecycleTest(t *testing.T) string {
	t.Helper()
	oldData, oldAddons, oldList, oldInspection := consts.ManagerDataPath, consts.AddonsBasePath, consts.MapListFilePath, consts.MapVPKInspectionsPath
	oldWriter, oldCleanup := writeMapLifecycleData, cleanupMapLifecycleFile
	base := t.TempDir()
	consts.ManagerDataPath = filepath.Join(base, "data")
	consts.AddonsBasePath = filepath.Join(base, "addons")
	consts.MapListFilePath = filepath.Join(consts.AddonsBasePath, "maplist.txt")
	consts.MapVPKInspectionsPath = filepath.Join(consts.ManagerDataPath, "map_vpk_inspections.json")
	if err := os.MkdirAll(consts.AddonsBasePath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(consts.MapListFilePath, nil, 0644); err != nil {
		t.Fatal(err)
	}
	resetMapVPKInspectionStoreForTest()
	t.Cleanup(func() {
		consts.ManagerDataPath, consts.AddonsBasePath, consts.MapListFilePath, consts.MapVPKInspectionsPath = oldData, oldAddons, oldList, oldInspection
		writeMapLifecycleData, cleanupMapLifecycleFile = oldWriter, oldCleanup
		resetMapVPKInspectionStoreForTest()
	})
	return base
}

func writeLifecycleTestMap(t *testing.T, name string, imported bool) MapLifecycleItem {
	t.Helper()
	if err := os.WriteFile(filepath.Join(consts.AddonsBasePath, name), []byte("map contents: "+name), 0644); err != nil {
		t.Fatal(err)
	}
	list, err := os.ReadFile(consts.MapListFilePath)
	if err != nil {
		t.Fatal(err)
	}
	publish := func() error { return os.WriteFile(consts.MapListFilePath, append(list, []byte(name+"\n")...), 0644) }
	if imported {
		MapLifecycleMutex.Lock()
		err = RecordInstalledMapsLocked([]string{name}, publish)
		MapLifecycleMutex.Unlock()
	} else {
		err = publish()
	}
	if err != nil {
		t.Fatal(err)
	}
	items, err := ListMapLifecycleItems()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("map %s not listed", name)
	return MapLifecycleItem{}
}

func ageLifecycleTestMap(t *testing.T, name string, at time.Time) {
	t.Helper()
	store, err := loadMapLifecycleStoreLocked()
	if err != nil {
		t.Fatal(err)
	}
	record := store.Maps[name]
	record.UploadedAt = &at
	store.Maps[name] = record
	if err := writeMapLifecycleJSON(mapLifecyclePath(), store); err != nil {
		t.Fatal(err)
	}
}

func assertLifecycleFile(t *testing.T, name string, present bool) {
	t.Helper()
	_, err := os.Lstat(filepath.Join(consts.AddonsBasePath, name))
	if present && err != nil || !present && !os.IsNotExist(err) {
		t.Fatalf("file %s present=%v, stat error=%v", name, present, err)
	}
}

func TestMapLifecycleHistoryExpiryLocksAndMixedDeletion(t *testing.T) {
	setupLifecycleTest(t)
	history := writeLifecycleTestMap(t, "history.vpk", false)
	old := writeLifecycleTestMap(t, "old.vpk", true)
	locked := writeLifecycleTestMap(t, "locked.vpk", true)
	writeLifecycleTestMap(t, "recent.vpk", true)
	if history.UploadedAt != nil || old.UploadedAt == nil || history.ID == "" || history.Revision == "" {
		t.Fatalf("unexpected history/import metadata: %+v / %+v", history, old)
	}
	now := time.Now().UTC()
	ageLifecycleTestMap(t, "old.vpk", now.Add(-72*time.Hour))
	ageLifecycleTestMap(t, "locked.vpk", now.Add(-100*time.Hour))
	if _, err := SetMapLocked("locked.vpk", true, locked.Revision); err != nil {
		t.Fatal(err)
	}
	result, err := DeleteExpiredMaps(now, 3)
	if err != nil || !reflect.DeepEqual(result.Deleted, []string{"old.vpk"}) || !reflect.DeepEqual(result.SkippedLocked, []string{"locked.vpk"}) || !reflect.DeepEqual(result.SkippedUnknownTime, []string{"history.vpk"}) {
		t.Fatalf("unexpected expiration result: %+v, %v", result, err)
	}
	assertLifecycleFile(t, "old.vpk", false)
	for _, name := range []string{"history.vpk", "locked.vpk", "recent.vpk"} {
		assertLifecycleFile(t, name, true)
	}
	result, err = DeleteMaps([]string{"locked.vpk", "history.vpk", "recent.vpk", "recent.vpk"})
	if err != nil || len(result.Deleted) != 2 || len(result.SkippedLocked) != 1 {
		t.Fatalf("unexpected mixed result: %+v, %v", result, err)
	}
	result, err = ClearUnlockedMaps()
	if err != nil || len(result.Deleted) != 0 || len(result.SkippedLocked) != 1 {
		t.Fatalf("clear did not preserve lock: %+v, %v", result, err)
	}
	assertLifecycleFile(t, "locked.vpk", true)
}

func TestMapLifecycleStableIdentityCASAndMaintenance(t *testing.T) {
	setupLifecycleTest(t)
	item := writeLifecycleTestMap(t, "map.vpk", true)
	locked, err := SetMapLocked(item.Name, true, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SetMapLocked(item.Name, false, item.Revision); !errors.Is(err, ErrMapLifecycleConflict) {
		t.Fatalf("stale lock mutation returned %v", err)
	}
	MapLifecycleMutex.Lock()
	err = RenameMapLifecycleLocked("map.vpk", "renamed.vpk", func() error {
		if err := os.Rename(filepath.Join(consts.AddonsBasePath, "map.vpk"), filepath.Join(consts.AddonsBasePath, "renamed.vpk")); err != nil {
			return err
		}
		return os.WriteFile(consts.MapListFilePath, []byte("renamed.vpk\n"), 0644)
	})
	MapLifecycleMutex.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consts.AddonsBasePath, "renamed.vpk"), []byte("trimmed"), 0644); err != nil {
		t.Fatal(err)
	}
	MapLifecycleMutex.Lock()
	err = RefreshMapLifecycleFileLocked("renamed.vpk")
	MapLifecycleMutex.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	items, err := ListMapLifecycleItems()
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %+v, %v", items, err)
	}
	renamed := items[0]
	if renamed.ID != item.ID || !renamed.Locked || !renamed.UploadedAt.Equal(*item.UploadedAt) || renamed.Revision == locked.Revision {
		t.Fatalf("maintenance lost metadata: %+v", renamed)
	}
	if _, err := SetMapLocked(renamed.Name, false, renamed.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := DeleteMaps([]string{renamed.Name}); err != nil {
		t.Fatal(err)
	}
	reimported := writeLifecycleTestMap(t, renamed.Name, true)
	if reimported.ID == item.ID || reimported.Revision == renamed.Revision {
		t.Fatal("delete/reimport reused identity")
	}
	result, err := DeleteMapsWithRevisions([]string{renamed.Name}, map[string]string{renamed.Name: renamed.Revision})
	if err != nil || len(result.SkippedChanged) != 1 || len(result.Deleted) != 0 {
		t.Fatalf("stale delete touched new file: %+v, %v", result, err)
	}
}

func TestMapLifecycleFilesystemReplacementNeverInheritsTime(t *testing.T) {
	setupLifecycleTest(t)
	item := writeLifecycleTestMap(t, "map.vpk", true)
	if _, err := SetMapLocked(item.Name, true, item.Revision); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consts.AddonsBasePath, item.Name), []byte("unattributed replacement"), 0644); err != nil {
		t.Fatal(err)
	}
	items, err := ListMapLifecycleItems()
	if err != nil || len(items) != 1 || items[0].UploadedAt != nil || items[0].ID == item.ID || !items[0].Locked {
		t.Fatalf("replacement inherited upload timestamp or lost lock: %+v, %v", items, err)
	}
}

func TestMapLifecycleImportFailureAndPreparedRecovery(t *testing.T) {
	for _, crash := range []bool{false, true} {
		t.Run(fmtLifecycleBool(crash), func(t *testing.T) {
			setupLifecycleTest(t)
			writeLifecycleTestMap(t, "existing.vpk", false)
			if err := os.WriteFile(filepath.Join(consts.AddonsBasePath, "new.vpk"), []byte("new map"), 0644); err != nil {
				t.Fatal(err)
			}
			if crash {
				store, _ := loadMapLifecycleStoreLocked()
				journal, _ := newMapLifecycleJournal("import", []string{"new.vpk"}, store)
				info, _ := os.Stat(filepath.Join(consts.AddonsBasePath, "new.vpk"))
				record := mapLifecycleRecord(info)
				now := time.Now().UTC()
				record.UploadedAt = &now
				journal.After.Maps["new.vpk"] = record
				if err := writeMapLifecycleJSON(mapLifecycleJournalPath(), journal); err != nil {
					t.Fatal(err)
				}
				if err := writeMapLifecycleJSON(mapLifecyclePath(), journal.After); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(consts.MapListFilePath, []byte("existing.vpk\nnew.vpk\n"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := RecoverMapLifecycle(); err != nil {
					t.Fatal(err)
				}
			} else {
				MapLifecycleMutex.Lock()
				err := RecordInstalledMapsLocked([]string{"new.vpk"}, func() error { return errors.New("list publication failed") })
				MapLifecycleMutex.Unlock()
				if err == nil {
					t.Fatal("expected import failure")
				}
			}
			assertLifecycleFile(t, "existing.vpk", true)
			assertLifecycleFile(t, "new.vpk", false)
			items, err := ListMapLifecycleItems()
			if err != nil || len(items) != 1 || items[0].Name != "existing.vpk" {
				t.Fatalf("rollback list: %+v, %v", items, err)
			}
			store, err := loadMapLifecycleStoreLocked()
			if err != nil {
				t.Fatal(err)
			}
			if _, found := store.Maps["new.vpk"]; found {
				t.Fatal("rolled back upload retained timestamp")
			}
		})
	}
}

func fmtLifecycleBool(crash bool) string {
	if crash {
		return "crash"
	}
	return "publication_failure"
}

func TestMapLifecycleDeleteRecoveryAndWriteFailure(t *testing.T) {
	for _, mode := range []string{"prepared", "prepared_before_move", "committed", "metadata_failure"} {
		t.Run(mode, func(t *testing.T) {
			setupLifecycleTest(t)
			original := writeLifecycleTestMap(t, "map.vpk", true)
			if mode == "metadata_failure" {
				originalWriter := writeMapLifecycleData
				failed := false
				writeMapLifecycleData = func(path string, data []byte, perm os.FileMode) error {
					if path == mapLifecyclePath() && !failed && !strings.Contains(string(data), `"map.vpk"`) {
						failed = true
						return errors.New("injected disk error")
					}
					return originalWriter(path, data, perm)
				}
				result, err := DeleteMaps([]string{"map.vpk"})
				if err != nil || len(result.Deleted) != 0 || len(result.Failed) != 1 {
					t.Fatalf("failure result: %+v, %v", result, err)
				}
			} else {
				store, _ := loadMapLifecycleStoreLocked()
				journal, _ := newMapLifecycleJournal("delete", []string{"map.vpk"}, store)
				journal.Quarantine, journal.FilePresent = journal.ID+".vpk", true
				delete(journal.After.Maps, "map.vpk")
				root, err := os.OpenRoot(consts.AddonsBasePath)
				if err != nil {
					t.Fatal(err)
				}
				defer root.Close()
				if err := writeMapLifecycleJSON(mapLifecycleJournalPath(), journal); err != nil {
					t.Fatal(err)
				}
				if mode != "prepared_before_move" {
					if err := ensureMapLifecycleTrash(root); err != nil {
						t.Fatal(err)
					}
					if err := root.Rename("map.vpk", filepath.Join(mapLifecycleTrash, journal.Quarantine)); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(consts.MapListFilePath, nil, 0644); err != nil {
						t.Fatal(err)
					}
					if err := writeMapLifecycleJSON(mapLifecyclePath(), journal.After); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "committed" {
					journal.Phase = mode
					if err := writeMapLifecycleJSON(mapLifecycleJournalPath(), journal); err != nil {
						t.Fatal(err)
					}
				}
				if err := RecoverMapLifecycle(); err != nil {
					t.Fatal(err)
				}
			}
			assertLifecycleFile(t, "map.vpk", mode != "committed")
			items, err := ListMapLifecycleItems()
			if err != nil {
				t.Fatal(err)
			}
			if mode == "committed" && len(items) != 0 {
				t.Fatal("committed deletion resurrected")
			}
			if mode != "committed" && (len(items) != 1 || items[0].ID != original.ID || !items[0].UploadedAt.Equal(*original.UploadedAt)) {
				t.Fatalf("rollback metadata: %+v", items)
			}
			if _, err := os.Stat(mapLifecycleJournalPath()); !os.IsNotExist(err) {
				t.Fatalf("journal not completed: %v", err)
			}
		})
	}
}

func TestMapLifecycleCommittedCleanupCannotOverwriteJournal(t *testing.T) {
	setupLifecycleTest(t)
	writeLifecycleTestMap(t, "first.vpk", true)
	writeLifecycleTestMap(t, "second.vpk", true)
	originalCleanup := cleanupMapLifecycleFile
	cleanupMapLifecycleFile = func(root *os.Root, name string) error { return errors.New("quarantine cleanup blocked") }
	result, err := DeleteMaps([]string{"first.vpk", "second.vpk"})
	if err != nil || !reflect.DeepEqual(result.Deleted, []string{"first.vpk"}) || !reflect.DeepEqual(result.PendingCleanup, []string{"first.vpk"}) || len(result.Failed) != 1 || result.Failed[0].Name != "second.vpk" {
		t.Fatalf("pending result: %+v, %v", result, err)
	}
	data, err := os.ReadFile(mapLifecycleJournalPath())
	if err != nil {
		t.Fatal(err)
	}
	var journal mapLifecycleJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		t.Fatal(err)
	}
	if journal.Phase != "committed" || journal.Names[0] != "first.vpk" {
		t.Fatalf("journal overwritten: %+v", journal)
	}
	assertLifecycleFile(t, "second.vpk", true)
	cleanupMapLifecycleFile = originalCleanup
	if err := RecoverMapLifecycle(); err != nil {
		t.Fatal(err)
	}
	result, err = ClearUnlockedMaps()
	if err != nil || !reflect.DeepEqual(result.Deleted, []string{"second.vpk"}) {
		t.Fatalf("resumed delete: %+v, %v", result, err)
	}
}

func TestMapLifecycleCommitMarkerFailureRemainsRecoverable(t *testing.T) {
	for _, markerPublished := range []bool{false, true} {
		t.Run(fmtLifecycleBool(markerPublished), func(t *testing.T) {
			setupLifecycleTest(t)
			if err := os.WriteFile(filepath.Join(consts.AddonsBasePath, "map.vpk"), []byte("map"), 0644); err != nil {
				t.Fatal(err)
			}
			originalWriter := writeMapLifecycleData
			writeMapLifecycleData = func(path string, data []byte, mode os.FileMode) error {
				if path == mapLifecycleJournalPath() && strings.Contains(string(data), `"phase": "committed"`) {
					if markerPublished {
						if err := originalWriter(path, data, mode); err != nil {
							return err
						}
					}
					return errors.New("commit marker durability unknown")
				}
				return originalWriter(path, data, mode)
			}
			MapLifecycleMutex.Lock()
			err := RecordInstalledMapsLocked([]string{"map.vpk"}, func() error { return os.WriteFile(consts.MapListFilePath, []byte("map.vpk\n"), 0644) })
			MapLifecycleMutex.Unlock()
			if !errors.Is(err, ErrMapLifecycleCommitUncertain) {
				t.Fatalf("uncertain result: %v", err)
			}
			assertLifecycleFile(t, "map.vpk", true)
			writeMapLifecycleData = originalWriter
			if err := RecoverMapLifecycle(); err != nil {
				t.Fatal(err)
			}
			assertLifecycleFile(t, "map.vpk", markerPublished)
			items, err := ListMapLifecycleItems()
			if err != nil || len(items) != map[bool]int{false: 0, true: 1}[markerPublished] {
				t.Fatalf("recovery items: %+v, %v", items, err)
			}
		})
	}
}

func TestMapLifecycleCorruptionAndMissingStoreFailClosed(t *testing.T) {
	for _, mode := range []string{"invalid_json", "missing_store", "missing_locked", "null_locked", "bad_journal"} {
		t.Run(mode, func(t *testing.T) {
			setupLifecycleTest(t)
			item := writeLifecycleTestMap(t, "locked.vpk", true)
			if _, err := SetMapLocked(item.Name, true, item.Revision); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(mapLifecyclePath())
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "invalid_json":
				err = os.WriteFile(mapLifecyclePath(), []byte("{"), 0600)
			case "missing_store":
				err = os.Remove(mapLifecyclePath())
			case "missing_locked":
				err = os.WriteFile(mapLifecyclePath(), []byte(strings.Replace(string(data), `"locked": true,`, "", 1)), 0600)
			case "null_locked":
				err = os.WriteFile(mapLifecyclePath(), []byte(strings.Replace(string(data), `"locked": true`, `"locked": null`, 1)), 0600)
			case "bad_journal":
				err = os.WriteFile(mapLifecycleJournalPath(), []byte("{}"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ClearUnlockedMaps(); !errors.Is(err, ErrMapLifecycleUnavailable) {
				t.Fatalf("corrupt state allowed clear: %v", err)
			}
			assertLifecycleFile(t, "locked.vpk", true)
		})
	}
}

func TestMapLifecycleUnsafeEntriesNeverReachDeletion(t *testing.T) {
	base := setupLifecycleTest(t)
	outside := filepath.Join(base, "outside.vpk")
	if err := os.WriteFile(outside, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(consts.AddonsBasePath, "link.vpk")); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(consts.MapListFilePath, []byte("../outside.vpk\nlink.vpk\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ClearUnlockedMaps()
	if err != nil || len(result.Deleted) != 0 || len(result.Failed) != 2 {
		t.Fatalf("unsafe clear: %+v, %v", result, err)
	}
	data, err := os.ReadFile(outside)
	if err != nil || string(data) != "keep" {
		t.Fatalf("outside map changed: %q, %v", data, err)
	}
}

func TestMapLifecycleRenamePreparedRecoveryKeepsIdentityAndLock(t *testing.T) {
	setupLifecycleTest(t)
	item := writeLifecycleTestMap(t, "original.vpk", true)
	if _, err := SetMapLocked(item.Name, true, item.Revision); err != nil {
		t.Fatal(err)
	}
	store, err := loadMapLifecycleStoreLocked()
	if err != nil {
		t.Fatal(err)
	}
	journal, err := newMapLifecycleJournal("rename", []string{"original.vpk", "new.vpk"}, store)
	if err != nil {
		t.Fatal(err)
	}
	journal.After.Maps["new.vpk"] = store.Maps["original.vpk"]
	delete(journal.After.Maps, "original.vpk")
	if err := writeMapLifecycleJSON(mapLifecycleJournalPath(), journal); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(consts.AddonsBasePath, "original.vpk"), filepath.Join(consts.AddonsBasePath, "new.vpk")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(consts.MapListFilePath, []byte("new.vpk\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeMapLifecycleJSON(mapLifecyclePath(), journal.After); err != nil {
		t.Fatal(err)
	}
	if err := RecoverMapLifecycle(); err != nil {
		t.Fatal(err)
	}
	items, err := ListMapLifecycleItems()
	if err != nil || len(items) != 1 || items[0].Name != item.Name || items[0].ID != item.ID || !items[0].Locked || !items[0].UploadedAt.Equal(*item.UploadedAt) {
		t.Fatalf("rename recovery lost protection: %+v, %v", items, err)
	}
	assertLifecycleFile(t, "original.vpk", true)
	assertLifecycleFile(t, "new.vpk", false)
}

func TestMapLifecycleClearRecoversBeforeSelectingNames(t *testing.T) {
	setupLifecycleTest(t)
	writeLifecycleTestMap(t, "restored.vpk", true)
	store, err := loadMapLifecycleStoreLocked()
	if err != nil {
		t.Fatal(err)
	}
	journal, err := newMapLifecycleJournal("delete", []string{"restored.vpk"}, store)
	if err != nil {
		t.Fatal(err)
	}
	journal.Quarantine, journal.FilePresent = journal.ID+".vpk", true
	delete(journal.After.Maps, "restored.vpk")
	root, err := os.OpenRoot(consts.AddonsBasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := ensureMapLifecycleTrash(root); err != nil {
		t.Fatal(err)
	}
	if err := writeMapLifecycleJSON(mapLifecycleJournalPath(), journal); err != nil {
		t.Fatal(err)
	}
	if err := root.Rename("restored.vpk", filepath.Join(mapLifecycleTrash, journal.Quarantine)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(consts.MapListFilePath, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeMapLifecycleJSON(mapLifecyclePath(), journal.After); err != nil {
		t.Fatal(err)
	}
	result, err := ClearUnlockedMaps()
	if err != nil || !reflect.DeepEqual(result.Deleted, []string{"restored.vpk"}) {
		t.Fatalf("clear missed recovered map: %+v, %v", result, err)
	}
	assertLifecycleFile(t, "restored.vpk", false)
}

func TestMapLifecycleExplicitEmptyRevisionCannotDeleteNewMap(t *testing.T) {
	setupLifecycleTest(t)
	writeLifecycleTestMap(t, "new.vpk", true)
	result, err := DeleteMapsWithRevisions([]string{"new.vpk"}, map[string]string{"new.vpk": ""})
	if err != nil || len(result.Deleted) != 0 || !reflect.DeepEqual(result.SkippedChanged, []string{"new.vpk"}) {
		t.Fatalf("empty stale revision bypassed CAS: %+v, %v", result, err)
	}
	assertLifecycleFile(t, "new.vpk", true)
}

func TestMapLifecycleLosingBothFilesDuringProcessFailsClosed(t *testing.T) {
	setupLifecycleTest(t)
	item := writeLifecycleTestMap(t, "locked.vpk", true)
	if _, err := SetMapLocked(item.Name, true, item.Revision); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{mapLifecyclePath(), mapLifecycleMarkerPath()} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ClearUnlockedMaps(); !errors.Is(err, ErrMapLifecycleUnavailable) {
		t.Fatalf("losing both files reset existing protection: %v", err)
	}
	assertLifecycleFile(t, item.Name, true)
	if _, err := os.Stat(mapLifecyclePath()); !os.IsNotExist(err) {
		t.Fatalf("missing state was silently reinitialized: %v", err)
	}
}

func TestMapLifecycleLockDeleteRaceHasOneLinearizedOutcome(t *testing.T) {
	setupLifecycleTest(t)
	for i := 0; i < 12; i++ {
		item := writeLifecycleTestMap(t, "race.vpk", true)
		start := make(chan struct{})
		var group sync.WaitGroup
		var lockErr, deleteErr error
		var result MapDeleteResult
		group.Add(2)
		go func() { defer group.Done(); <-start; _, lockErr = SetMapLocked(item.Name, true, item.Revision) }()
		go func() { defer group.Done(); <-start; result, deleteErr = DeleteMaps([]string{item.Name}) }()
		close(start)
		group.Wait()
		if deleteErr != nil {
			t.Fatal(deleteErr)
		}
		if lockErr == nil {
			if len(result.SkippedLocked) != 1 || len(result.Deleted) != 0 {
				t.Fatalf("lock succeeded but deletion won: %+v", result)
			}
			assertLifecycleFile(t, item.Name, true)
			if _, err := SetMapLocked(item.Name, false); err != nil {
				t.Fatal(err)
			}
			if _, err := DeleteMaps([]string{item.Name}); err != nil {
				t.Fatal(err)
			}
		} else if !errors.Is(lockErr, ErrMapLifecycleNotFound) || len(result.Deleted) != 1 {
			t.Fatalf("invalid race outcome: %+v, lock=%v", result, lockErr)
		}
	}
}
