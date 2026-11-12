package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"l4d2-manager-next/consts"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MapLifecycleMutex serializes every published map mutation, including the
// controller's existing import, rename, trim and script-edit commit boundaries.
// Locked functions require this mutex to be held for writing by their caller.
var MapLifecycleMutex sync.RWMutex

var (
	ErrMapLifecycleUnavailable     = errors.New("地图生命周期数据不可用，已停止地图变更")
	ErrMapLifecycleNotFound        = errors.New("地图记录不存在")
	ErrMapLifecycleInvalidName     = errors.New("地图名称无效")
	ErrMapLifecycleConflict        = errors.New("地图已发生变化，请刷新后重试")
	ErrMapLifecycleCommitUncertain = errors.New("地图提交结果尚待恢复确认，请勿重试或手动删除文件")
)

const mapLifecycleVersion = 1
const mapLifecycleTrash = ".map_lifecycle_trash"

// Used by fault-injection tests to exercise failures after files have moved.
var writeMapLifecycleData = atomicWriteFile
var cleanupMapLifecycleFile = func(root *os.Root, name string) error { return root.Remove(name) }

// Guarded by MapLifecycleMutex. Losing both persistent files during this
// process must not turn an already initialized instance into an unlocked one.
var mapLifecycleInitializedPaths = make(map[string]bool)

type MapLifecycleRecord struct {
	ID         string     `json:"id"`
	Revision   string     `json:"revision"`
	UploadedAt *time.Time `json:"uploaded_at"`
	Locked     bool       `json:"locked"`
	Size       int64      `json:"size"`
	ModTimeNS  int64      `json:"mod_time_ns"`
}

func (record *MapLifecycleRecord) UnmarshalJSON(data []byte) error {
	type plainRecord MapLifecycleRecord
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, field := range []string{"id", "revision", "uploaded_at", "locked", "size", "mod_time_ns"} {
		value, exists := fields[field]
		if !exists || (field != "uploaded_at" && string(value) == "null") {
			return fmt.Errorf("地图生命周期记录缺少有效字段: %s", field)
		}
	}
	return json.Unmarshal(data, (*plainRecord)(record))
}

type MapLifecycleItem struct {
	Name       string     `json:"name"`
	ID         string     `json:"id"`
	Revision   string     `json:"revision"`
	UploadedAt *time.Time `json:"uploaded_at"`
	Locked     bool       `json:"locked"`
	Size       int64      `json:"size"`
	SizeLabel  string     `json:"size_label"`
	Missing    bool       `json:"missing"`
	Error      string     `json:"error,omitempty"`
}

type MapDeleteFailure struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

type MapDeleteResult struct {
	Deleted            []string           `json:"deleted"`
	SkippedLocked      []string           `json:"skipped_locked"`
	SkippedUnknownTime []string           `json:"skipped_unknown_time"`
	SkippedChanged     []string           `json:"skipped_changed"`
	Failed             []MapDeleteFailure `json:"failed"`
	PendingCleanup     []string           `json:"pending_cleanup"`
}

type mapLifecycleStore struct {
	Version int                           `json:"version"`
	StoreID string                        `json:"store_id"`
	Maps    map[string]MapLifecycleRecord `json:"maps"`
}

type mapLifecycleMarker struct {
	Version int    `json:"version"`
	StoreID string `json:"store_id"`
}

// Only one operation can be outstanding under MapLifecycleMutex. Prepared
// operations roll back; committed operations finish their housekeeping. The
// quarantined map stays on the addons filesystem, so moving it is atomic.
type mapLifecycleJournal struct {
	Version        int               `json:"version"`
	ID             string            `json:"id"`
	Kind           string            `json:"kind"`
	Phase          string            `json:"phase"`
	AddonsPath     string            `json:"addons_path"`
	MapListPath    string            `json:"map_list_path"`
	Names          []string          `json:"names"`
	Quarantine     string            `json:"quarantine,omitempty"`
	FilePresent    bool              `json:"file_present"`
	Before         mapLifecycleStore `json:"before"`
	After          mapLifecycleStore `json:"after"`
	BeforeList     []byte            `json:"before_list"`
	BeforeListMode uint32            `json:"before_list_mode"`
	ListExisted    bool              `json:"list_existed"`
}

func mapLifecyclePath() string { return filepath.Join(consts.ManagerDataPath, "map_lifecycle.json") }
func mapLifecycleMarkerPath() string {
	return filepath.Join(consts.ManagerDataPath, "map_lifecycle.initialized")
}
func mapLifecycleJournalPath() string {
	return filepath.Join(consts.ManagerDataPath, "map_lifecycle.journal.json")
}

func mapLifecycleError(err error) error {
	return fmt.Errorf("%w: %v", ErrMapLifecycleUnavailable, err)
}

func validMapLifecycleID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == value && parsed != uuid.Nil
}

func validateMapLifecycleStore(store mapLifecycleStore) error {
	if store.Version != mapLifecycleVersion || !validMapLifecycleID(store.StoreID) || store.Maps == nil {
		return errors.New("地图生命周期文件版本或身份无效")
	}
	for name, record := range store.Maps {
		normalized, err := NormalizeMapVPKName(name)
		if err != nil || normalized != name || !validMapLifecycleID(record.ID) || !validMapLifecycleID(record.Revision) || record.Size < 0 {
			return fmt.Errorf("地图生命周期记录无效: %s", name)
		}
		if record.UploadedAt != nil && (record.UploadedAt.IsZero() || record.UploadedAt.Year() < 1970) {
			return fmt.Errorf("地图上传时间无效: %s", name)
		}
	}
	return nil
}

func cloneMapLifecycleStore(store mapLifecycleStore) mapLifecycleStore {
	copyStore := mapLifecycleStore{Version: store.Version, StoreID: store.StoreID, Maps: make(map[string]MapLifecycleRecord, len(store.Maps))}
	for name, record := range store.Maps {
		if record.UploadedAt != nil {
			at := *record.UploadedAt
			record.UploadedAt = &at
		}
		copyStore.Maps[name] = record
	}
	return copyStore
}

func writeMapLifecycleJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := writeMapLifecycleData(path, data, 0600); err != nil {
		return err
	}
	return syncMapLifecycleDirectory(filepath.Dir(path))
}

func syncMapLifecycleDirectory(path string) error {
	// Windows does not support syncing directory handles through os.File.Sync.
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func loadMapLifecycleStoreLocked() (mapLifecycleStore, error) {
	var store mapLifecycleStore
	storePath, pathErr := filepath.Abs(mapLifecyclePath())
	if pathErr != nil {
		return store, mapLifecycleError(pathErr)
	}
	data, err := os.ReadFile(mapLifecyclePath())
	markerBytes, markerErr := os.ReadFile(mapLifecycleMarkerPath())
	if err != nil && !os.IsNotExist(err) {
		return store, mapLifecycleError(err)
	}
	if markerErr != nil && !os.IsNotExist(markerErr) {
		return store, mapLifecycleError(markerErr)
	}
	if os.IsNotExist(err) {
		if markerErr == nil || mapLifecycleInitializedPaths[storePath] {
			return store, mapLifecycleError(errors.New("已初始化的地图生命周期文件丢失，请恢复完整备份"))
		}
		if _, journalErr := os.Lstat(mapLifecycleJournalPath()); !os.IsNotExist(journalErr) {
			return store, mapLifecycleError(errors.New("生命周期文件丢失但存在未完成操作"))
		}
		store = mapLifecycleStore{Version: mapLifecycleVersion, StoreID: uuid.NewString(), Maps: make(map[string]MapLifecycleRecord)}
		if err := writeMapLifecycleJSON(mapLifecyclePath(), store); err != nil {
			return store, mapLifecycleError(err)
		}
	} else if err := json.Unmarshal(data, &store); err != nil {
		return store, mapLifecycleError(err)
	}
	if err := validateMapLifecycleStore(store); err != nil {
		return store, mapLifecycleError(err)
	}
	if markerErr == nil {
		var marker mapLifecycleMarker
		if err := json.Unmarshal(markerBytes, &marker); err != nil || marker.Version != mapLifecycleVersion || marker.StoreID != store.StoreID {
			return store, mapLifecycleError(errors.New("地图生命周期初始化身份不一致"))
		}
	} else if err := writeMapLifecycleJSON(mapLifecycleMarkerPath(), mapLifecycleMarker{Version: mapLifecycleVersion, StoreID: store.StoreID}); err != nil {
		return store, mapLifecycleError(err)
	}
	mapLifecycleInitializedPaths[storePath] = true
	return store, nil
}

func readMapLifecycleList() ([]byte, os.FileMode, bool, error) {
	data, err := os.ReadFile(consts.MapListFilePath)
	if os.IsNotExist(err) {
		return nil, 0644, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	info, err := os.Stat(consts.MapListFilePath)
	if err != nil {
		return nil, 0, false, err
	}
	return data, info.Mode().Perm(), true, nil
}

func mapLifecycleNames(data []byte) []string {
	names := make([]string, 0)
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		name := strings.TrimSpace(line)
		if name != "" && !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	return names
}

func hasMapLifecycleName(data []byte, name string) bool {
	for _, current := range mapLifecycleNames(data) {
		if current == name {
			return true
		}
	}
	return false
}

func writeMapLifecycleList(data []byte, mode os.FileMode) error {
	if err := writeMapLifecycleData(consts.MapListFilePath, data, mode); err != nil {
		return err
	}
	return syncMapLifecycleDirectory(filepath.Dir(consts.MapListFilePath))
}

func mapLifecycleRecord(info os.FileInfo) MapLifecycleRecord {
	return MapLifecycleRecord{ID: uuid.NewString(), Revision: uuid.NewString(), Size: info.Size(), ModTimeNS: info.ModTime().UnixNano()}
}

func sameMapLifecycleFile(record MapLifecycleRecord, info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && record.Size == info.Size() && record.ModTimeNS == info.ModTime().UnixNano()
}

func statMapLifecycleFile(root *os.Root, name string) (os.FileInfo, error) {
	if normalized, err := NormalizeMapVPKName(name); err != nil || normalized != name {
		return nil, ErrMapLifecycleInvalidName
	}
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("地图必须是普通文件，不能是目录或符号链接")
	}
	return info, nil
}

func lifecycleItem(name string, record MapLifecycleRecord, info os.FileInfo, err error) MapLifecycleItem {
	item := MapLifecycleItem{Name: name, ID: record.ID, Revision: record.Revision, UploadedAt: record.UploadedAt, Locked: record.Locked, Size: record.Size, SizeLabel: formatMapLifecycleSize(record.Size)}
	if err != nil {
		item.Missing = os.IsNotExist(err)
		item.Error = err.Error()
	} else if info != nil {
		item.Size = info.Size()
		item.SizeLabel = formatMapLifecycleSize(info.Size())
	}
	return item
}

func formatMapLifecycleSize(size int64) string {
	for _, unit := range []struct {
		bytes int64
		label string
	}{{1 << 30, "GB"}, {1 << 20, "MB"}, {1 << 10, "KB"}} {
		if size >= unit.bytes {
			return fmt.Sprintf("%.2f%s", float64(size)/float64(unit.bytes), unit.label)
		}
	}
	return fmt.Sprintf("%dB", size)
}

func ListMapLifecycleItems() ([]MapLifecycleItem, error) {
	MapLifecycleMutex.Lock()
	defer MapLifecycleMutex.Unlock()
	return ListMapLifecycleItemsLocked()
}

func ListMapLifecycleItemsLocked() ([]MapLifecycleItem, error) {
	if err := RecoverMapLifecycleLocked(); err != nil {
		return nil, err
	}
	store, err := loadMapLifecycleStoreLocked()
	if err != nil {
		return nil, err
	}
	list, _, _, err := readMapLifecycleList()
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(consts.AddonsBasePath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	items := make([]MapLifecycleItem, 0)
	changed := false
	for _, name := range mapLifecycleNames(list) {
		info, statErr := statMapLifecycleFile(root, name)
		record, found := store.Maps[name]
		if statErr == nil && (!found || !sameMapLifecycleFile(record, info)) {
			// Unattributed filesystem changes never inherit an old upload time.
			locked := record.Locked
			record = mapLifecycleRecord(info)
			record.Locked = locked
			store.Maps[name] = record
			changed = true
		}
		items = append(items, lifecycleItem(name, record, info, statErr))
	}
	if changed {
		if err := writeMapLifecycleJSON(mapLifecyclePath(), store); err != nil {
			return nil, mapLifecycleError(err)
		}
	}
	return items, nil
}

func SetMapLocked(name string, locked bool, expectedRevision ...string) (MapLifecycleItem, error) {
	MapLifecycleMutex.Lock()
	defer MapLifecycleMutex.Unlock()
	return SetMapLockedLocked(name, locked, expectedRevision...)
}

func SetMapLockedLocked(name string, locked bool, expectedRevision ...string) (MapLifecycleItem, error) {
	var empty MapLifecycleItem
	if normalized, err := NormalizeMapVPKName(name); err != nil || normalized != name {
		return empty, ErrMapLifecycleInvalidName
	}
	items, err := ListMapLifecycleItemsLocked()
	if err != nil {
		return empty, err
	}
	var item *MapLifecycleItem
	for i := range items {
		if items[i].Name == name {
			item = &items[i]
			break
		}
	}
	if item == nil || item.ID == "" || item.Missing {
		return empty, ErrMapLifecycleNotFound
	}
	if item.Error != "" {
		return empty, fmt.Errorf("%w: %s", ErrMapLifecycleConflict, item.Error)
	}
	if len(expectedRevision) > 0 && expectedRevision[0] != "" && expectedRevision[0] != item.Revision {
		return empty, ErrMapLifecycleConflict
	}
	if item.Locked == locked {
		return *item, nil
	}
	store, err := loadMapLifecycleStoreLocked()
	if err != nil {
		return empty, err
	}
	record := store.Maps[name]
	record.Locked, record.Revision = locked, uuid.NewString()
	store.Maps[name] = record
	if err := writeMapLifecycleJSON(mapLifecyclePath(), store); err != nil {
		return empty, mapLifecycleError(err)
	}
	item.Locked, item.Revision = locked, record.Revision
	return *item, nil
}

// RecordInstalledMapsLocked is part of the import transaction, before maplist
// publication. The caller has installed exclusive new files and must roll them
// back on an ordinary error. ErrMapLifecycleCommitUncertain instead requires
// retaining them until journal recovery. publish must not expose a partial list.
func RecordInstalledMapsLocked(names []string, publish func() error) error {
	if err := RecoverMapLifecycleLocked(); err != nil {
		return err
	}
	store, err := loadMapLifecycleStoreLocked()
	if err != nil {
		return err
	}
	journal, err := newMapLifecycleJournal("import", names, store)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(consts.AddonsBasePath)
	if err != nil {
		return err
	}
	defer root.Close()
	now := time.Now().UTC()
	for _, name := range names {
		if hasMapLifecycleName(journal.BeforeList, name) {
			return fmt.Errorf("地图 %s 已经存在", name)
		}
		info, err := statMapLifecycleFile(root, name)
		if err != nil {
			return err
		}
		installed, err := root.OpenFile(name, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		syncErr := installed.Sync()
		closeErr := installed.Close()
		if err := errors.Join(syncErr, closeErr); err != nil {
			return err
		}
		record := mapLifecycleRecord(info)
		record.UploadedAt = &now
		journal.After.Maps[name] = record
	}
	return commitMapLifecycleJournalLocked(journal, func() error {
		if err := writeMapLifecycleJSON(mapLifecyclePath(), journal.After); err != nil {
			return err
		}
		return publish()
	})
}

func RenameMapLifecycleLocked(oldName, newName string, publish func() error) error {
	if oldName == newName {
		return publish()
	}
	if _, err := ListMapLifecycleItemsLocked(); err != nil {
		return err
	}
	store, err := loadMapLifecycleStoreLocked()
	if err != nil {
		return err
	}
	journal, err := newMapLifecycleJournal("rename", []string{oldName, newName}, store)
	if err != nil {
		return err
	}
	if !hasMapLifecycleName(journal.BeforeList, oldName) || hasMapLifecycleName(journal.BeforeList, newName) {
		return ErrMapLifecycleNotFound
	}
	root, err := os.OpenRoot(consts.AddonsBasePath)
	if err != nil {
		return err
	}
	defer root.Close()
	if _, err := statMapLifecycleFile(root, oldName); err != nil {
		return err
	}
	if _, err := root.Lstat(newName); !os.IsNotExist(err) {
		return fmt.Errorf("目标地图已存在或无法检查: %s", newName)
	}
	record := store.Maps[oldName]
	record.Revision = uuid.NewString()
	journal.After.Maps[newName] = record
	delete(journal.After.Maps, oldName)
	return commitMapLifecycleJournalLocked(journal, func() error {
		// Keep both names protected until the physical rename is published.
		prepared := cloneMapLifecycleStore(journal.After)
		prepared.Maps[oldName] = store.Maps[oldName]
		if err := writeMapLifecycleJSON(mapLifecyclePath(), prepared); err != nil {
			return err
		}
		if err := publish(); err != nil {
			return err
		}
		return writeMapLifecycleJSON(mapLifecyclePath(), journal.After)
	})
}

// Content maintenance preserves upload time, identity and lock. A crash between
// replacement and this update leaves a stale fingerprint, which cleanup skips.
func RefreshMapLifecycleFileLocked(name string) error {
	if err := RecoverMapLifecycleLocked(); err != nil {
		return err
	}
	store, err := loadMapLifecycleStoreLocked()
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(consts.AddonsBasePath)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := statMapLifecycleFile(root, name)
	if err != nil {
		return err
	}
	record, found := store.Maps[name]
	if !found {
		record = mapLifecycleRecord(info)
	}
	record.Size, record.ModTimeNS, record.Revision = info.Size(), info.ModTime().UnixNano(), uuid.NewString()
	store.Maps[name] = record
	return writeMapLifecycleJSON(mapLifecyclePath(), store)
}

func newMapDeleteResult() MapDeleteResult {
	return MapDeleteResult{Deleted: []string{}, SkippedLocked: []string{}, SkippedUnknownTime: []string{}, SkippedChanged: []string{}, Failed: []MapDeleteFailure{}, PendingCleanup: []string{}}
}

func DeleteMaps(names []string) (MapDeleteResult, error) {
	return DeleteMapsWithRevisions(names, nil)
}

func DeleteMapsWithRevisions(names []string, revisions map[string]string) (MapDeleteResult, error) {
	MapLifecycleMutex.Lock()
	defer MapLifecycleMutex.Unlock()
	return DeleteMapsWithRevisionsLocked(names, revisions)
}

func DeleteMapsLocked(names []string) (MapDeleteResult, error) {
	return DeleteMapsWithRevisionsLocked(names, nil)
}

func DeleteMapsWithRevisionsLocked(names []string, revisions map[string]string) (MapDeleteResult, error) {
	return deleteMapLifecycleNamesLocked(names, revisions, nil)
}

func ClearUnlockedMaps() (MapDeleteResult, error) {
	MapLifecycleMutex.Lock()
	defer MapLifecycleMutex.Unlock()
	return ClearUnlockedMapsLocked()
}

func ClearUnlockedMapsLocked() (MapDeleteResult, error) {
	if err := RecoverMapLifecycleLocked(); err != nil {
		return newMapDeleteResult(), err
	}
	data, _, _, err := readMapLifecycleList()
	if err != nil {
		return newMapDeleteResult(), err
	}
	return deleteMapLifecycleNamesLocked(mapLifecycleNames(data), nil, nil)
}

func DeleteExpiredMaps(now time.Time, retentionDays int) (MapDeleteResult, error) {
	MapLifecycleMutex.Lock()
	defer MapLifecycleMutex.Unlock()
	return DeleteExpiredMapsLocked(now, retentionDays)
}

func DeleteExpiredMapsLocked(now time.Time, retentionDays int) (MapDeleteResult, error) {
	if retentionDays < 1 || retentionDays > MaxMapCleanupRetentionDays {
		return newMapDeleteResult(), fmt.Errorf("地图保留天数必须在 1 到 %d 之间", MaxMapCleanupRetentionDays)
	}
	if err := RecoverMapLifecycleLocked(); err != nil {
		return newMapDeleteResult(), err
	}
	data, _, _, err := readMapLifecycleList()
	if err != nil {
		return newMapDeleteResult(), err
	}
	cutoff := now.UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	return deleteMapLifecycleNamesLocked(mapLifecycleNames(data), nil, &cutoff)
}

func deleteMapLifecycleNamesLocked(names []string, revisions map[string]string, cutoff *time.Time) (MapDeleteResult, error) {
	result := newMapDeleteResult()
	if err := RecoverMapLifecycleLocked(); err != nil {
		return result, err
	}
	seen := make(map[string]bool)
	for index, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		if err := RecoverMapLifecycleLocked(); err != nil {
			appendMapLifecycleBlocked(&result, names[index:], err)
			return result, nil
		}
		if normalized, err := NormalizeMapVPKName(name); err != nil || normalized != name {
			result.Failed = append(result.Failed, MapDeleteFailure{name, ErrMapLifecycleInvalidName.Error()})
			continue
		}
		store, err := loadMapLifecycleStoreLocked()
		if err != nil {
			appendMapLifecycleBlocked(&result, names[index:], err)
			return result, nil
		}
		journal, err := newMapLifecycleJournal("delete", []string{name}, store)
		if err != nil {
			return result, err
		}
		if !hasMapLifecycleName(journal.BeforeList, name) {
			result.Failed = append(result.Failed, MapDeleteFailure{name, ErrMapLifecycleNotFound.Error()})
			continue
		}
		record := store.Maps[name]
		if record.Locked {
			result.SkippedLocked = append(result.SkippedLocked, name)
			continue
		}
		if expected, supplied := revisions[name]; supplied && expected != record.Revision {
			result.SkippedChanged = append(result.SkippedChanged, name)
			continue
		}
		root, err := os.OpenRoot(consts.AddonsBasePath)
		if err != nil {
			return result, err
		}
		info, statErr := statMapLifecycleFile(root, name)
		root.Close()
		if statErr != nil && !os.IsNotExist(statErr) {
			result.Failed = append(result.Failed, MapDeleteFailure{name, statErr.Error()})
			continue
		}
		if cutoff != nil {
			if record.UploadedAt == nil {
				result.SkippedUnknownTime = append(result.SkippedUnknownTime, name)
				continue
			}
			if statErr != nil || !sameMapLifecycleFile(record, info) {
				result.SkippedChanged = append(result.SkippedChanged, name)
				continue
			}
			if record.UploadedAt.After(*cutoff) {
				continue
			}
		}
		journal.FilePresent = statErr == nil
		journal.Quarantine = journal.ID + ".vpk"
		delete(journal.After.Maps, name)
		err = commitMapLifecycleJournalLocked(journal, func() error {
			root, err := os.OpenRoot(consts.AddonsBasePath)
			if err != nil {
				return err
			}
			defer root.Close()
			if journal.FilePresent {
				if err := ensureMapLifecycleTrash(root); err != nil {
					return err
				}
				if err := root.Rename(name, filepath.Join(mapLifecycleTrash, journal.Quarantine)); err != nil {
					return err
				}
				if err := syncMapLifecycleDirectory(consts.AddonsBasePath); err != nil {
					return err
				}
				if err := syncMapLifecycleDirectory(filepath.Join(consts.AddonsBasePath, mapLifecycleTrash)); err != nil {
					return err
				}
			}
			remaining := make([]string, 0)
			for _, current := range mapLifecycleNames(journal.BeforeList) {
				if current != name {
					remaining = append(remaining, current)
				}
			}
			content := strings.Join(remaining, "\n")
			if content != "" {
				content += "\n"
			}
			if err := writeMapLifecycleList([]byte(content), os.FileMode(journal.BeforeListMode)); err != nil {
				return err
			}
			return writeMapLifecycleJSON(mapLifecyclePath(), journal.After)
		})
		if err != nil {
			result.Failed = append(result.Failed, MapDeleteFailure{name, err.Error()})
			// An unrecovered journal blocks all subsequent changes in this batch.
			if _, journalErr := os.Lstat(mapLifecycleJournalPath()); !os.IsNotExist(journalErr) {
				appendMapLifecycleBlocked(&result, names[index+1:], err)
				return result, nil
			}
			continue
		}
		result.Deleted = append(result.Deleted, name)
		if _, err := os.Lstat(mapLifecycleJournalPath()); !os.IsNotExist(err) {
			result.PendingCleanup = append(result.PendingCleanup, name)
		}
	}
	return result, nil
}

func appendMapLifecycleBlocked(result *MapDeleteResult, names []string, err error) {
	seen := make(map[string]bool)
	for _, name := range names {
		if !seen[name] {
			result.Failed = append(result.Failed, MapDeleteFailure{name, "前一操作尚未恢复，未执行: " + err.Error()})
			seen[name] = true
		}
	}
}

func newMapLifecycleJournal(kind string, names []string, before mapLifecycleStore) (mapLifecycleJournal, error) {
	journal := mapLifecycleJournal{Version: mapLifecycleVersion, ID: uuid.NewString(), Kind: kind, Phase: "prepared", Names: names, Before: cloneMapLifecycleStore(before), After: cloneMapLifecycleStore(before)}
	for _, name := range names {
		if normalized, err := NormalizeMapVPKName(name); err != nil || normalized != name {
			return journal, ErrMapLifecycleInvalidName
		}
	}
	var err error
	journal.AddonsPath, err = filepath.Abs(consts.AddonsBasePath)
	if err != nil {
		return journal, err
	}
	journal.MapListPath, err = filepath.Abs(consts.MapListFilePath)
	if err != nil {
		return journal, err
	}
	data, mode, exists, err := readMapLifecycleList()
	journal.BeforeList, journal.BeforeListMode, journal.ListExisted = data, uint32(mode), exists
	return journal, err
}

func commitMapLifecycleJournalLocked(journal mapLifecycleJournal, publish func() error) error {
	if _, err := os.Lstat(mapLifecycleJournalPath()); !os.IsNotExist(err) {
		return mapLifecycleError(errors.New("存在尚未恢复的地图操作，拒绝覆盖日志"))
	}
	if err := writeMapLifecycleJSON(mapLifecycleJournalPath(), journal); err != nil {
		return mapLifecycleError(err)
	}
	if err := publish(); err != nil {
		return errors.Join(err, recoverMapLifecycleJournalLocked(journal))
	}
	if err := syncMapLifecycleDirectory(consts.AddonsBasePath); err != nil {
		return errors.Join(err, recoverMapLifecycleJournalLocked(journal))
	}
	if err := syncMapLifecycleDirectory(filepath.Dir(consts.MapListFilePath)); err != nil {
		return errors.Join(err, recoverMapLifecycleJournalLocked(journal))
	}
	journal.Phase = "committed"
	if err := writeMapLifecycleJSON(mapLifecycleJournalPath(), journal); err != nil {
		// Rename may already have published "committed" before directory Sync
		// failed. Do not roll back using a phase that differs from the disk, and
		// tell import callers to retain their files for explicit recovery.
		return fmt.Errorf("%w: %v", ErrMapLifecycleCommitUncertain, err)
	}
	// The commit is durable. A cleanup error leaves a committed journal for the
	// next operation/startup; never report failure and let an import caller erase
	// files which were already committed successfully.
	if err := recoverMapLifecycleJournalLocked(journal); err != nil {
		log.Printf("地图操作已提交，收尾等待恢复（%s）: %v", journal.ID, err)
	}
	return nil
}

func ensureMapLifecycleTrash(root *os.Root) error {
	if err := root.Mkdir(mapLifecycleTrash, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	info, err := root.Lstat(mapLifecycleTrash)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("地图删除事务目录不是普通目录")
	}
	return nil
}

func RecoverMapLifecycle() error {
	MapLifecycleMutex.Lock()
	defer MapLifecycleMutex.Unlock()
	return RecoverMapLifecycleLocked()
}

func RecoverMapLifecycleLocked() error {
	store, err := loadMapLifecycleStoreLocked()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(mapLifecycleJournalPath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return mapLifecycleError(err)
	}
	var journal mapLifecycleJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return mapLifecycleError(err)
	}
	if err := validateMapLifecycleJournal(journal, store); err != nil {
		return mapLifecycleError(err)
	}
	return recoverMapLifecycleJournalLocked(journal)
}

func validateMapLifecycleJournal(journal mapLifecycleJournal, current mapLifecycleStore) error {
	addons, err := filepath.Abs(consts.AddonsBasePath)
	if err != nil {
		return err
	}
	listPath, err := filepath.Abs(consts.MapListFilePath)
	if err != nil {
		return err
	}
	if journal.Version != mapLifecycleVersion || !validMapLifecycleID(journal.ID) || (journal.Phase != "prepared" && journal.Phase != "committed") || journal.AddonsPath != addons || journal.MapListPath != listPath || journal.Before.StoreID != current.StoreID || journal.After.StoreID != current.StoreID {
		return errors.New("地图操作日志身份、路径或版本无效")
	}
	if err := validateMapLifecycleStore(journal.Before); err != nil {
		return err
	}
	if err := validateMapLifecycleStore(journal.After); err != nil {
		return err
	}
	for _, name := range journal.Names {
		if normalized, err := NormalizeMapVPKName(name); err != nil || normalized != name {
			return ErrMapLifecycleInvalidName
		}
	}
	switch journal.Kind {
	case "delete":
		if len(journal.Names) != 1 || journal.Quarantine != journal.ID+".vpk" {
			return errors.New("地图删除日志无效")
		}
	case "import":
		if len(journal.Names) == 0 {
			return errors.New("地图导入日志为空")
		}
	case "rename":
		if len(journal.Names) != 2 || journal.Names[0] == journal.Names[1] {
			return errors.New("地图改名日志无效")
		}
	default:
		return errors.New("未知地图操作日志类型")
	}
	return nil
}

func recoverMapLifecycleJournalLocked(journal mapLifecycleJournal) error {
	root, err := os.OpenRoot(consts.AddonsBasePath)
	if err != nil {
		return mapLifecycleError(err)
	}
	defer root.Close()
	if journal.Phase == "prepared" {
		switch journal.Kind {
		case "delete":
			if journal.FilePresent {
				quarantine := filepath.Join(mapLifecycleTrash, journal.Quarantine)
				if _, err := root.Lstat(quarantine); err == nil {
					if _, err := root.Lstat(journal.Names[0]); !os.IsNotExist(err) {
						return mapLifecycleError(errors.New("恢复目标被占用，已保留隔离文件"))
					}
					if err := root.Rename(quarantine, journal.Names[0]); err != nil {
						return mapLifecycleError(err)
					}
				} else if !os.IsNotExist(err) {
					return mapLifecycleError(err)
				} else if _, err := statMapLifecycleFile(root, journal.Names[0]); err != nil {
					return mapLifecycleError(errors.New("原地图和隔离文件均不可恢复"))
				}
			}
		case "import":
			for _, name := range journal.Names {
				info, err := statMapLifecycleFile(root, name)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil || !sameMapLifecycleFile(journal.After.Maps[name], info) {
					return mapLifecycleError(fmt.Errorf("导入回滚时地图文件变化: %s", name))
				}
				if err := root.Remove(name); err != nil {
					return mapLifecycleError(err)
				}
			}
		case "rename":
			oldName, newName := journal.Names[0], journal.Names[1]
			_, oldErr := statMapLifecycleFile(root, oldName)
			newInfo, newErr := statMapLifecycleFile(root, newName)
			if os.IsNotExist(oldErr) && newErr == nil && sameMapLifecycleFile(journal.After.Maps[newName], newInfo) {
				if err := root.Rename(newName, oldName); err != nil {
					return mapLifecycleError(err)
				}
			} else if oldErr != nil || !os.IsNotExist(newErr) {
				return mapLifecycleError(errors.New("改名恢复存在文件冲突，已停止操作"))
			}
			if err := RenameMapVPKInspection(newName, oldName); err != nil {
				return mapLifecycleError(err)
			}
		}
		if journal.ListExisted {
			if err := writeMapLifecycleList(journal.BeforeList, os.FileMode(journal.BeforeListMode)); err != nil {
				return mapLifecycleError(err)
			}
		} else if err := os.Remove(consts.MapListFilePath); err != nil && !os.IsNotExist(err) {
			return mapLifecycleError(err)
		}
		if err := writeMapLifecycleJSON(mapLifecyclePath(), journal.Before); err != nil {
			return mapLifecycleError(err)
		}
	} else if journal.Kind == "delete" {
		if err := cleanupMapLifecycleFile(root, filepath.Join(mapLifecycleTrash, journal.Quarantine)); err != nil && !os.IsNotExist(err) {
			return mapLifecycleError(err)
		}
		if err := DeleteMapVPKInspection(journal.Names[0]); err != nil {
			return mapLifecycleError(err)
		}
	}
	if err := syncMapLifecycleDirectory(consts.AddonsBasePath); err != nil {
		return mapLifecycleError(err)
	}
	if journal.Kind == "delete" && journal.FilePresent {
		// A prepared operation may have failed before creating its quarantine.
		trashPath := filepath.Join(consts.AddonsBasePath, mapLifecycleTrash)
		if _, err := os.Lstat(trashPath); err == nil {
			if err := syncMapLifecycleDirectory(trashPath); err != nil {
				return mapLifecycleError(err)
			}
		} else if !os.IsNotExist(err) {
			return mapLifecycleError(err)
		}
	}
	if err := os.Remove(mapLifecycleJournalPath()); err != nil && !os.IsNotExist(err) {
		return mapLifecycleError(err)
	}
	return syncMapLifecycleDirectory(filepath.Dir(mapLifecycleJournalPath()))
}
