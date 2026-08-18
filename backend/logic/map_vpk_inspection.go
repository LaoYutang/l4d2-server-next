package logic

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/pkg/valve/vpk"
	"log"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

type DictionaryInspectionStatus string

const (
	DictionaryStatusPresent       DictionaryInspectionStatus = "present"
	DictionaryStatusMissing       DictionaryInspectionStatus = "missing"
	DictionaryStatusUnreadable    DictionaryInspectionStatus = "unreadable"
	DictionaryStatusNotApplicable DictionaryInspectionStatus = "not_applicable"
	DictionaryStatusNotChecked    DictionaryInspectionStatus = "not_checked"
)

type DictionaryChapterStatus string

const (
	DictionaryChapterPresent    DictionaryChapterStatus = "present"
	DictionaryChapterMissing    DictionaryChapterStatus = "missing"
	DictionaryChapterUnreadable DictionaryChapterStatus = "unreadable"
)

type GlobalScriptsInspectionStatus string

const (
	GlobalScriptsStatusClean      GlobalScriptsInspectionStatus = "clean"
	GlobalScriptsStatusDetected   GlobalScriptsInspectionStatus = "detected"
	GlobalScriptsStatusUnreadable GlobalScriptsInspectionStatus = "unreadable"
	GlobalScriptsStatusNotChecked GlobalScriptsInspectionStatus = "not_checked"
)

type ScriptOverridesInspectionStatus string

const (
	ScriptOverridesStatusClean      ScriptOverridesInspectionStatus = "clean"
	ScriptOverridesStatusDetected   ScriptOverridesInspectionStatus = "detected"
	ScriptOverridesStatusUnreadable ScriptOverridesInspectionStatus = "unreadable"
	ScriptOverridesStatusNotChecked ScriptOverridesInspectionStatus = "not_checked"
)

type DictionaryChapterInspection struct {
	BSPPath       string                  `json:"bsp_path"`
	ChapterCode   string                  `json:"chapter_code"`
	ChapterTitle  string                  `json:"chapter_title,omitempty"`
	CampaignTitle string                  `json:"campaign_title,omitempty"`
	Status        DictionaryChapterStatus `json:"status"`
	Message       string                  `json:"message,omitempty"`
}

type DictionaryInspection struct {
	Status   DictionaryInspectionStatus    `json:"status"`
	Chapters []DictionaryChapterInspection `json:"chapters"`
}

type GlobalScriptsInspection struct {
	Status GlobalScriptsInspectionStatus `json:"status"`
	Files  []string                      `json:"files"`
}

type ScriptOverridesInspection struct {
	Status ScriptOverridesInspectionStatus `json:"status"`
	Files  []string                        `json:"files"`
}

type MapVPKInspection struct {
	CheckedAt       string                    `json:"checked_at,omitempty"`
	Dictionary      DictionaryInspection      `json:"dictionary"`
	GlobalScripts   GlobalScriptsInspection   `json:"global_scripts"`
	ScriptOverrides ScriptOverridesInspection `json:"script_overrides"`
}

type MapScriptContent struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Encoding  string `json:"encoding"`
	Content   string `json:"content,omitempty"`
	Truncated bool   `json:"truncated"`
	Error     string `json:"error,omitempty"`
}

type GlobalScriptContent = MapScriptContent
type ScriptOverrideContent = MapScriptContent

var (
	ErrMapInspectionNotFound = errors.New("地图尚未检测")
	ErrMapInspectionStale    = errors.New("地图检测结果已失效")
	ErrMapRecordNotFound     = errors.New("地图记录不存在")
	ErrNoGlobalScripts       = errors.New("地图不存在全局脚本")
	ErrNoScriptOverrides     = errors.New("地图不存在脚本覆盖")
)

const (
	mapVPKInspectionStoreVersion = 1
	maxGlobalScriptContentBytes  = 512 << 10
	bspLumpCount                 = 64
	bspLumpSize                  = 16
	bspPakfileLump               = 40
	bspHeaderSize                = 4 + 4 + bspLumpCount*bspLumpSize + 4
)

var globalScriptEntryPaths = map[string]struct{}{
	"scripts/vscripts/mapspawn_addon.nut":      {},
	"scripts/vscripts/scriptedmode_addon.nut":  {},
	"scripts/vscripts/director_base_addon.nut": {},
}

type mapVPKInspectionRecord struct {
	Size       int64            `json:"size"`
	ModTimeNS  int64            `json:"mod_time_ns"`
	Inspection MapVPKInspection `json:"inspection"`
}

type mapVPKInspectionStoreFile struct {
	Version int                               `json:"version"`
	Maps    map[string]mapVPKInspectionRecord `json:"maps"`
}

var (
	mapVPKInspectionMu         sync.Mutex
	mapVPKInspectionLoadedPath string
	mapVPKInspectionRecords    map[string]mapVPKInspectionRecord
)

func newNotCheckedMapVPKInspection() MapVPKInspection {
	return MapVPKInspection{
		Dictionary: DictionaryInspection{
			Status:   DictionaryStatusNotChecked,
			Chapters: []DictionaryChapterInspection{},
		},
		GlobalScripts: GlobalScriptsInspection{
			Status: GlobalScriptsStatusNotChecked,
			Files:  []string{},
		},
		ScriptOverrides: ScriptOverridesInspection{
			Status: ScriptOverridesStatusNotChecked,
			Files:  []string{},
		},
	}
}

func normalizeVPKEntryPath(name string) string {
	return strings.ToLower(cleanVPKEntryPath(name))
}

func cleanVPKEntryPath(name string) string {
	return strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
}

func isGlobalScriptEntryPath(name string) bool {
	_, ok := globalScriptEntryPaths[normalizeVPKEntryPath(name)]
	return ok
}

func isScriptOverrideEntryPath(name string) bool {
	name = normalizeVPKEntryPath(name)
	if name == "scripts/gamemodes.txt" {
		return true
	}
	if pathpkg.Dir(name) != "scripts" {
		return false
	}
	base := pathpkg.Base(name)
	return strings.HasPrefix(base, "weapon_") &&
		strings.HasSuffix(base, ".txt") &&
		len(base) > len("weapon_.txt")
}

func InspectMapVPK(vpkPath string) MapVPKInspection {
	result := MapVPKInspection{
		CheckedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Dictionary: DictionaryInspection{
			Status:   DictionaryStatusNotApplicable,
			Chapters: []DictionaryChapterInspection{},
		},
		GlobalScripts: GlobalScriptsInspection{
			Status: GlobalScriptsStatusClean,
			Files:  []string{},
		},
		ScriptOverrides: ScriptOverridesInspection{
			Status: ScriptOverridesStatusClean,
			Files:  []string{},
		},
	}

	opener := vpk.Single(vpkPath)
	defer opener.Close()

	archive, err := opener.ReadArchive()
	if err != nil {
		result.Dictionary.Status = DictionaryStatusUnreadable
		result.GlobalScripts.Status = GlobalScriptsStatusUnreadable
		result.ScriptOverrides.Status = ScriptOverridesStatusUnreadable
		return result
	}

	type bspEntry struct {
		name string
		file *vpk.File
	}
	bspFiles := make([]bspEntry, 0)
	seenScripts := make(map[string]struct{}, len(globalScriptEntryPaths))
	seenOverrides := make(map[string]struct{})

	for i := range archive.Files {
		archiveFile := &archive.Files[i]
		entryPath := cleanVPKEntryPath(archiveFile.Name())
		normalizedPath := normalizeVPKEntryPath(entryPath)
		if isGlobalScriptEntryPath(normalizedPath) {
			if _, exists := seenScripts[normalizedPath]; !exists {
				seenScripts[normalizedPath] = struct{}{}
				result.GlobalScripts.Files = append(result.GlobalScripts.Files, normalizedPath)
			}
		}
		if isScriptOverrideEntryPath(normalizedPath) {
			if _, exists := seenOverrides[normalizedPath]; !exists {
				seenOverrides[normalizedPath] = struct{}{}
				result.ScriptOverrides.Files = append(result.ScriptOverrides.Files, normalizedPath)
			}
		}
		if strings.HasPrefix(normalizedPath, "maps/") && strings.HasSuffix(normalizedPath, ".bsp") {
			bspFiles = append(bspFiles, bspEntry{name: entryPath, file: archiveFile})
		}
	}

	sort.Strings(result.GlobalScripts.Files)
	if len(result.GlobalScripts.Files) > 0 {
		result.GlobalScripts.Status = GlobalScriptsStatusDetected
	}
	sort.Strings(result.ScriptOverrides.Files)
	if len(result.ScriptOverrides.Files) > 0 {
		result.ScriptOverrides.Status = ScriptOverridesStatusDetected
	}
	sort.Slice(bspFiles, func(i, j int) bool {
		return strings.ToLower(bspFiles[i].name) < strings.ToLower(bspFiles[j].name)
	})

	if len(bspFiles) == 0 {
		return result
	}

	result.Dictionary.Status = DictionaryStatusPresent
	missingFound := false
	unreadableFound := false
	for _, entry := range bspFiles {
		status, message := inspectBSPDictionary(entry.file, opener)
		chapter := DictionaryChapterInspection{
			BSPPath:     entry.name,
			ChapterCode: chapterCodeFromBSPPath(entry.name),
			Status:      status,
			Message:     message,
		}
		result.Dictionary.Chapters = append(result.Dictionary.Chapters, chapter)
		switch status {
		case DictionaryChapterMissing:
			missingFound = true
		case DictionaryChapterUnreadable:
			unreadableFound = true
		}
	}

	switch {
	case missingFound:
		result.Dictionary.Status = DictionaryStatusMissing
	case unreadableFound:
		result.Dictionary.Status = DictionaryStatusUnreadable
	default:
		result.Dictionary.Status = DictionaryStatusPresent
	}

	return result
}

func inspectBSPDictionary(file *vpk.File, opener *vpk.Opener) (DictionaryChapterStatus, string) {
	if file.Size() < bspHeaderSize {
		return DictionaryChapterUnreadable, "BSP 头部不完整"
	}

	header := make([]byte, bspHeaderSize)
	readerAt := file.OpenReaderAt(opener)
	if _, err := readerAt.ReadAt(header, 0); err != nil {
		return DictionaryChapterUnreadable, "无法读取 BSP 头部"
	}
	if string(header[:4]) != "VBSP" {
		return DictionaryChapterUnreadable, "BSP 文件标识无效"
	}

	lumpOffset := 8 + bspPakfileLump*bspLumpSize
	lumpFields := [4]int64{}
	for index := range lumpFields {
		fieldOffset := lumpOffset + index*4
		lumpFields[index] = int64(int32(binary.LittleEndian.Uint32(header[fieldOffset : fieldOffset+4])))
	}

	// Most Source BSPs store lump fields as file offset, file length, version,
	// fourCC. Some L4D2 maps store version first instead. Try both layouts and
	// accept only a range that contains a valid ZIP archive.
	type pakfileLocation struct {
		offset int64
		length int64
	}
	locations := []pakfileLocation{
		{offset: lumpFields[0], length: lumpFields[1]},
		{offset: lumpFields[1], length: lumpFields[2]},
	}
	if locations[0].length == 0 {
		return DictionaryChapterMissing, "BSP 未包含 Pakfile"
	}

	validRangeFound := false
	seenLocations := make(map[pakfileLocation]struct{}, len(locations))
	for _, location := range locations {
		if location.length == 0 {
			continue
		}
		if _, seen := seenLocations[location]; seen {
			continue
		}
		seenLocations[location] = struct{}{}

		if location.offset < 0 || location.length < 0 || location.offset > int64(file.Size()) || location.length > int64(file.Size())-location.offset {
			continue
		}
		validRangeFound = true

		pakReader := io.NewSectionReader(readerAt, location.offset, location.length)
		pak, err := zip.NewReader(pakReader, location.length)
		if err != nil {
			continue
		}
		for _, embedded := range pak.File {
			name := normalizeVPKEntryPath(embedded.Name)
			if pathpkg.Base(name) == "stringtable_dictionary.dct" {
				return DictionaryChapterPresent, ""
			}
		}
		return DictionaryChapterMissing, "未找到 stringtable_dictionary.dct"
	}

	if validRangeFound {
		return DictionaryChapterUnreadable, "BSP Pakfile 无法解析"
	}
	return DictionaryChapterUnreadable, "BSP Pakfile 范围无效"
}

func chapterCodeFromBSPPath(bspPath string) string {
	name := cleanVPKEntryPath(bspPath)
	if len(name) >= len("maps/") && strings.EqualFold(name[:len("maps/")], "maps/") {
		name = name[len("maps/"):]
	}
	if len(name) >= len(".bsp") && strings.EqualFold(name[len(name)-len(".bsp"):], ".bsp") {
		name = name[:len(name)-len(".bsp")]
	}
	return name
}

func InspectAndStoreMapVPK(mapName, vpkPath string) error {
	mapName, err := NormalizeMapVPKName(mapName)
	if err != nil {
		return err
	}

	before, err := os.Stat(vpkPath)
	if err != nil {
		return err
	}
	inspection := InspectMapVPK(vpkPath)
	info, err := os.Stat(vpkPath)
	if err != nil {
		return err
	}
	if before.Size() != info.Size() || before.ModTime().UnixNano() != info.ModTime().UnixNano() {
		return fmt.Errorf("地图文件在检测期间发生变化")
	}

	mapVPKInspectionMu.Lock()
	defer mapVPKInspectionMu.Unlock()
	loadMapVPKInspectionStoreLocked()
	previousRecords := cloneMapVPKInspectionRecords(mapVPKInspectionRecords)
	mapVPKInspectionRecords[mapName] = mapVPKInspectionRecord{
		Size:       info.Size(),
		ModTimeNS:  info.ModTime().UnixNano(),
		Inspection: cloneMapVPKInspection(inspection),
	}
	if err := persistMapVPKInspectionStoreLocked(); err != nil {
		mapVPKInspectionRecords = previousRecords
		return err
	}
	return nil
}

func GetMapVPKInspection(mapName string, info os.FileInfo) MapVPKInspection {
	mapVPKInspectionMu.Lock()
	defer mapVPKInspectionMu.Unlock()
	loadMapVPKInspectionStoreLocked()

	record, ok := mapVPKInspectionRecords[mapName]
	if !ok || info == nil || record.Size != info.Size() || record.ModTimeNS != info.ModTime().UnixNano() {
		return newNotCheckedMapVPKInspection()
	}
	return cloneMapVPKInspection(record.Inspection)
}

func RenameMapVPKInspection(oldName, newName string) error {
	oldName, err := NormalizeMapVPKName(oldName)
	if err != nil {
		return err
	}
	newName, err = NormalizeMapVPKName(newName)
	if err != nil {
		return err
	}

	mapVPKInspectionMu.Lock()
	defer mapVPKInspectionMu.Unlock()
	loadMapVPKInspectionStoreLocked()
	previousRecords := cloneMapVPKInspectionRecords(mapVPKInspectionRecords)
	record, ok := mapVPKInspectionRecords[oldName]
	if !ok {
		return nil
	}
	delete(mapVPKInspectionRecords, oldName)

	info, statErr := os.Stat(filepath.Join(consts.AddonsBasePath, newName))
	if statErr == nil && info.Size() == record.Size && info.ModTime().UnixNano() == record.ModTimeNS {
		mapVPKInspectionRecords[newName] = record
	}
	if err := persistMapVPKInspectionStoreLocked(); err != nil {
		mapVPKInspectionRecords = previousRecords
		return err
	}
	return nil
}

func DeleteMapVPKInspection(mapName string) error {
	mapName, err := NormalizeMapVPKName(mapName)
	if err != nil {
		return err
	}

	mapVPKInspectionMu.Lock()
	defer mapVPKInspectionMu.Unlock()
	loadMapVPKInspectionStoreLocked()
	if _, ok := mapVPKInspectionRecords[mapName]; !ok {
		return nil
	}
	previousRecords := cloneMapVPKInspectionRecords(mapVPKInspectionRecords)
	delete(mapVPKInspectionRecords, mapName)
	if err := persistMapVPKInspectionStoreLocked(); err != nil {
		mapVPKInspectionRecords = previousRecords
		return err
	}
	return nil
}

func RetainMapVPKInspections(mapNames []string) error {
	keep := make(map[string]struct{}, len(mapNames))
	for _, name := range mapNames {
		if normalized, err := NormalizeMapVPKName(name); err == nil {
			keep[normalized] = struct{}{}
		}
	}

	mapVPKInspectionMu.Lock()
	defer mapVPKInspectionMu.Unlock()
	loadMapVPKInspectionStoreLocked()
	previousRecords := cloneMapVPKInspectionRecords(mapVPKInspectionRecords)
	for name := range mapVPKInspectionRecords {
		if _, ok := keep[name]; !ok {
			delete(mapVPKInspectionRecords, name)
		}
	}
	if err := persistMapVPKInspectionStoreLocked(); err != nil {
		mapVPKInspectionRecords = previousRecords
		return err
	}
	return nil
}

func GetMapGlobalScriptContents(mapName string) ([]GlobalScriptContent, error) {
	scripts, _, err := GetMapGlobalScriptContentsWithRevision(mapName)
	return scripts, err
}

func GetMapGlobalScriptContentsWithRevision(mapName string) ([]GlobalScriptContent, string, error) {
	return getMapInspectedScriptContents(
		mapName,
		func(inspection MapVPKInspection) ([]string, bool) {
			return inspection.GlobalScripts.Files, inspection.GlobalScripts.Status == GlobalScriptsStatusDetected
		},
		isGlobalScriptEntryPath,
		ErrNoGlobalScripts,
	)
}

func GetMapScriptOverrideContents(mapName string) ([]ScriptOverrideContent, error) {
	scripts, _, err := getMapInspectedScriptContents(
		mapName,
		func(inspection MapVPKInspection) ([]string, bool) {
			return inspection.ScriptOverrides.Files, inspection.ScriptOverrides.Status == ScriptOverridesStatusDetected
		},
		isScriptOverrideEntryPath,
		ErrNoScriptOverrides,
	)
	return scripts, err
}

func getMapInspectedScriptContents(
	mapName string,
	selectFiles func(MapVPKInspection) ([]string, bool),
	isAllowedPath func(string) bool,
	noScriptsError error,
) ([]MapScriptContent, string, error) {
	mapName, err := NormalizeMapVPKName(mapName)
	if err != nil {
		return nil, "", err
	}
	allowedMaps, err := readAllowedMapNames()
	if err != nil {
		return nil, "", err
	}
	if !allowedMaps[mapName] {
		return nil, "", ErrMapRecordNotFound
	}

	vpkPath := filepath.Join(consts.AddonsBasePath, mapName)
	info, err := os.Stat(vpkPath)
	if err != nil {
		return nil, "", err
	}

	mapVPKInspectionMu.Lock()
	loadMapVPKInspectionStoreLocked()
	record, ok := mapVPKInspectionRecords[mapName]
	if ok {
		record.Inspection = cloneMapVPKInspection(record.Inspection)
	}
	mapVPKInspectionMu.Unlock()
	if !ok {
		return nil, "", ErrMapInspectionNotFound
	}
	if record.Size != info.Size() || record.ModTimeNS != info.ModTime().UnixNano() {
		return nil, "", ErrMapInspectionStale
	}
	recordedFiles, detected := selectFiles(record.Inspection)
	if !detected || len(recordedFiles) == 0 {
		return nil, "", noScriptsError
	}

	opener := vpk.Single(vpkPath)
	defer opener.Close()
	archive, err := opener.ReadArchive()
	if err != nil {
		return nil, "", fmt.Errorf("解析 VPK 失败: %w", err)
	}

	archiveFiles := make(map[string]*vpk.File, len(archive.Files))
	for i := range archive.Files {
		archiveFile := &archive.Files[i]
		name := normalizeVPKEntryPath(archiveFile.Name())
		if isAllowedPath(name) {
			archiveFiles[name] = archiveFile
		}
	}

	result := make([]MapScriptContent, 0, len(recordedFiles))
	for _, scriptPath := range recordedFiles {
		scriptPath = normalizeVPKEntryPath(scriptPath)
		if !isAllowedPath(scriptPath) {
			continue
		}
		item := MapScriptContent{Path: scriptPath, Encoding: "unknown"}
		archiveFile, exists := archiveFiles[scriptPath]
		if !exists {
			item.Error = "脚本文件不存在，请刷新地图列表"
			result = append(result, item)
			continue
		}

		item.Size = int64(archiveFile.Size())
		item.Truncated = item.Size > maxGlobalScriptContentBytes
		readSize := item.Size
		if readSize > maxGlobalScriptContentBytes {
			readSize = maxGlobalScriptContentBytes
		}
		data := make([]byte, int(readSize))
		if readSize > 0 {
			n, readErr := archiveFile.OpenReaderAt(opener).ReadAt(data, 0)
			if readErr != nil && !(errors.Is(readErr, io.EOF) && int64(n) == readSize) {
				item.Error = "读取脚本内容失败"
				result = append(result, item)
				continue
			}
			data = data[:n]
		}
		item.Content, item.Encoding = decodeGlobalScriptContent(data, item.Truncated)
		result = append(result, item)
	}

	afterInfo, err := os.Stat(vpkPath)
	if err != nil {
		return nil, "", err
	}
	if afterInfo.Size() != info.Size() || afterInfo.ModTime().UnixNano() != info.ModTime().UnixNano() {
		return nil, "", ErrMapInspectionStale
	}

	return result, mapVPKRevision(info), nil
}

func mapVPKRevision(info os.FileInfo) string {
	return fmt.Sprintf("%x-%x", uint64(info.Size()), uint64(info.ModTime().UnixNano()))
}

func decodeGlobalScriptContent(data []byte, truncated bool) (string, string) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(data) {
		return string(data), "utf-8"
	}
	if truncated {
		for trim := 1; trim < utf8.UTFMax && trim < len(data); trim++ {
			if utf8.Valid(data[:len(data)-trim]) {
				return string(data[:len(data)-trim]), "utf-8"
			}
		}
	}
	gbkData := data
	if truncated && !isValidGBK(gbkData) && len(gbkData) > 0 && isGBKLeadByte(gbkData[len(gbkData)-1]) && isValidGBK(gbkData[:len(gbkData)-1]) {
		gbkData = gbkData[:len(gbkData)-1]
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(gbkData)
	if isValidGBK(gbkData) && err == nil && utf8.Valid(decoded) && !bytes.Contains(decoded, []byte("�")) {
		return string(decoded), "gbk"
	}
	return strings.ToValidUTF8(string(data), "�"), "unknown"
}

func isValidGBK(data []byte) bool {
	for index := 0; index < len(data); {
		if data[index] <= 0x7f {
			index++
			continue
		}
		if !isGBKLeadByte(data[index]) || index+1 >= len(data) {
			return false
		}
		trail := data[index+1]
		if trail < 0x40 || trail > 0xfe || trail == 0x7f {
			return false
		}
		index += 2
	}
	return true
}

func isGBKLeadByte(value byte) bool {
	return value >= 0x81 && value <= 0xfe
}

func loadMapVPKInspectionStoreLocked() {
	storePath := consts.MapVPKInspectionsPath
	if mapVPKInspectionLoadedPath == storePath && mapVPKInspectionRecords != nil {
		return
	}

	mapVPKInspectionLoadedPath = storePath
	mapVPKInspectionRecords = make(map[string]mapVPKInspectionRecord)
	data, err := os.ReadFile(storePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("读取地图 VPK 检测记录失败: %v", err)
		}
		return
	}

	var stored mapVPKInspectionStoreFile
	if err := json.Unmarshal(data, &stored); err != nil {
		log.Printf("解析地图 VPK 检测记录失败: %v", err)
		return
	}
	if stored.Version != mapVPKInspectionStoreVersion {
		log.Printf("忽略不支持的地图 VPK 检测记录版本: %d", stored.Version)
		return
	}
	for name, record := range stored.Maps {
		if normalized, err := NormalizeMapVPKName(name); err == nil && normalized == name {
			mapVPKInspectionRecords[name] = record
		}
	}
}

func persistMapVPKInspectionStoreLocked() error {
	if err := os.MkdirAll(filepath.Dir(consts.MapVPKInspectionsPath), 0755); err != nil {
		return err
	}
	stored := mapVPKInspectionStoreFile{
		Version: mapVPKInspectionStoreVersion,
		Maps:    mapVPKInspectionRecords,
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(consts.MapVPKInspectionsPath, data, 0644)
}

func cloneMapVPKInspection(source MapVPKInspection) MapVPKInspection {
	clone := source
	clone.Dictionary.Chapters = append([]DictionaryChapterInspection(nil), source.Dictionary.Chapters...)
	clone.GlobalScripts.Files = append([]string(nil), source.GlobalScripts.Files...)
	clone.ScriptOverrides.Files = append([]string(nil), source.ScriptOverrides.Files...)
	if clone.Dictionary.Chapters == nil {
		clone.Dictionary.Chapters = []DictionaryChapterInspection{}
	}
	if clone.GlobalScripts.Files == nil {
		clone.GlobalScripts.Files = []string{}
	}
	if clone.ScriptOverrides.Status == "" {
		clone.ScriptOverrides.Status = ScriptOverridesStatusNotChecked
	}
	if clone.ScriptOverrides.Files == nil {
		clone.ScriptOverrides.Files = []string{}
	}
	return clone
}

func cloneMapVPKInspectionRecords(source map[string]mapVPKInspectionRecord) map[string]mapVPKInspectionRecord {
	clone := make(map[string]mapVPKInspectionRecord, len(source))
	for name, record := range source {
		record.Inspection = cloneMapVPKInspection(record.Inspection)
		clone[name] = record
	}
	return clone
}

func resetMapVPKInspectionStoreForTest() {
	mapVPKInspectionMu.Lock()
	defer mapVPKInspectionMu.Unlock()
	mapVPKInspectionLoadedPath = ""
	mapVPKInspectionRecords = nil
}
