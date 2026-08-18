package logic

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"l4d2-manager-next/consts"
	"l4d2-manager-next/pkg/valve/vpk"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestInspectMapVPKDetectsDictionaryStateAndRiskScripts(t *testing.T) {
	vpkPath := filepath.Join(t.TempDir(), "inspection.vpk")
	writeMapInspectionTestVPK(t, vpkPath, map[string][]byte{
		"maps/C1M1_Present.bsp":                        makeMapInspectionTestBSP(t, true),
		"MAPS/C1M2_Missing.BSP":                        makeMapInspectionTestBSP(t, false),
		"maps/C1M3_Unreadable.bsp":                     []byte("VBSP"),
		"Scripts/Vscripts/MAPSPAWN_ADDON.NUT":          []byte("mapspawn"),
		"scripts/vscripts/scriptedmode_addon.nut":      []byte("scriptedmode"),
		"scripts/vscripts/director_base_addon.nut":     []byte("director"),
		"scripts/vscripts/ordinary.nut":                []byte("ordinary"),
		"scripts/vscripts/another_addon.nut":           []byte("another"),
		"scripts/vscripts/sub/mapspawn_addon.nut":      []byte("nested"),
		"scripts/vscripts/mapspawn_addon.nut.disabled": []byte("disabled"),
		"scripts/vscripts/sub/scriptedmode_addon.nut":  []byte("nested"),
		"scripts/vscripts/sub/director_base_addon.nut": []byte("nested"),
		"Scripts/WEAPON_RIFLE.TXT":                     []byte("weapon"),
		"scripts/gamemodes.txt":                        []byte("gamemodes"),
		"scripts/weapon_.txt":                          []byte("empty name"),
		"scripts/weapon_rifle.txt.disabled":            []byte("disabled"),
		"scripts/sub/weapon_smg.txt":                   []byte("nested"),
	})

	inspection := InspectMapVPK(vpkPath)
	if inspection.Dictionary.Status != DictionaryStatusMissing {
		t.Fatalf("dictionary status = %q, want %q", inspection.Dictionary.Status, DictionaryStatusMissing)
	}
	if len(inspection.Dictionary.Chapters) != 3 {
		t.Fatalf("chapter count = %d, want 3", len(inspection.Dictionary.Chapters))
	}

	chapterStatuses := make(map[string]DictionaryChapterStatus)
	chapterCodes := make(map[string]string)
	for _, chapter := range inspection.Dictionary.Chapters {
		chapterStatuses[chapter.BSPPath] = chapter.Status
		chapterCodes[chapter.BSPPath] = chapter.ChapterCode
	}
	if chapterStatuses["maps/C1M1_Present.bsp"] != DictionaryChapterPresent {
		t.Fatalf("present BSP status = %q", chapterStatuses["maps/C1M1_Present.bsp"])
	}
	if chapterStatuses["MAPS/C1M2_Missing.BSP"] != DictionaryChapterMissing {
		t.Fatalf("missing BSP status = %q", chapterStatuses["MAPS/C1M2_Missing.BSP"])
	}
	if chapterStatuses["maps/C1M3_Unreadable.bsp"] != DictionaryChapterUnreadable {
		t.Fatalf("unreadable BSP status = %q", chapterStatuses["maps/C1M3_Unreadable.bsp"])
	}
	if chapterCodes["MAPS/C1M2_Missing.BSP"] != "C1M2_Missing" {
		t.Fatalf("chapter code = %q, want original-case code", chapterCodes["MAPS/C1M2_Missing.BSP"])
	}

	wantScripts := []string{
		"scripts/vscripts/director_base_addon.nut",
		"scripts/vscripts/mapspawn_addon.nut",
		"scripts/vscripts/scriptedmode_addon.nut",
	}
	if inspection.GlobalScripts.Status != GlobalScriptsStatusDetected {
		t.Fatalf("global script status = %q, want %q", inspection.GlobalScripts.Status, GlobalScriptsStatusDetected)
	}
	if !reflect.DeepEqual(inspection.GlobalScripts.Files, wantScripts) {
		t.Fatalf("global scripts = %#v, want %#v", inspection.GlobalScripts.Files, wantScripts)
	}

	wantOverrides := []string{
		"scripts/gamemodes.txt",
		"scripts/weapon_rifle.txt",
	}
	if inspection.ScriptOverrides.Status != ScriptOverridesStatusDetected {
		t.Fatalf("script overrides status = %q, want %q", inspection.ScriptOverrides.Status, ScriptOverridesStatusDetected)
	}
	if !reflect.DeepEqual(inspection.ScriptOverrides.Files, wantOverrides) {
		t.Fatalf("script overrides = %#v, want %#v", inspection.ScriptOverrides.Files, wantOverrides)
	}
}

func TestInspectMapVPKDictionaryAggregateStates(t *testing.T) {
	t.Run("no BSP", func(t *testing.T) {
		vpkPath := filepath.Join(t.TempDir(), "no-bsp.vpk")
		writeMapInspectionTestVPK(t, vpkPath, map[string][]byte{
			"missions/test.txt": []byte("mission"),
		})

		inspection := InspectMapVPK(vpkPath)
		if inspection.Dictionary.Status != DictionaryStatusNotApplicable {
			t.Fatalf("dictionary status = %q, want not_applicable", inspection.Dictionary.Status)
		}
		if inspection.GlobalScripts.Status != GlobalScriptsStatusClean {
			t.Fatalf("global scripts status = %q, want clean", inspection.GlobalScripts.Status)
		}
		if inspection.ScriptOverrides.Status != ScriptOverridesStatusClean {
			t.Fatalf("script overrides status = %q, want clean", inspection.ScriptOverrides.Status)
		}
	})

	t.Run("all present", func(t *testing.T) {
		vpkPath := filepath.Join(t.TempDir(), "present.vpk")
		writeMapInspectionTestVPK(t, vpkPath, map[string][]byte{
			"maps/present.bsp": makeMapInspectionTestBSP(t, true),
		})

		inspection := InspectMapVPK(vpkPath)
		if inspection.Dictionary.Status != DictionaryStatusPresent {
			t.Fatalf("dictionary status = %q, want present", inspection.Dictionary.Status)
		}
	})

	t.Run("only unreadable", func(t *testing.T) {
		vpkPath := filepath.Join(t.TempDir(), "unreadable.vpk")
		writeMapInspectionTestVPK(t, vpkPath, map[string][]byte{
			"maps/broken.bsp": []byte("not a BSP"),
		})

		inspection := InspectMapVPK(vpkPath)
		if inspection.Dictionary.Status != DictionaryStatusUnreadable {
			t.Fatalf("dictionary status = %q, want unreadable", inspection.Dictionary.Status)
		}
		if inspection.ScriptOverrides.Status != ScriptOverridesStatusClean {
			t.Fatalf("script overrides status = %q, want clean", inspection.ScriptOverrides.Status)
		}
	})
}

func TestInspectMapVPKSupportsVersionFirstPakfileLump(t *testing.T) {
	vpkPath := filepath.Join(t.TempDir(), "version-first.vpk")
	writeMapInspectionTestVPK(t, vpkPath, map[string][]byte{
		"maps/version_first.bsp": makeMapInspectionTestBSPWithLayout(t, true, true),
	})

	inspection := InspectMapVPK(vpkPath)
	if inspection.Dictionary.Status != DictionaryStatusPresent {
		t.Fatalf("dictionary status = %q, want present: %+v", inspection.Dictionary.Status, inspection.Dictionary.Chapters)
	}
	if len(inspection.Dictionary.Chapters) != 1 || inspection.Dictionary.Chapters[0].Status != DictionaryChapterPresent {
		t.Fatalf("chapters = %+v, want one present chapter", inspection.Dictionary.Chapters)
	}
}

func TestMapVPKInspectionPersistenceAndGlobalScriptContents(t *testing.T) {
	addonsPath := setupMapVPKInspectionTestPaths(t)
	mapName := "scripts.vpk"
	vpkPath := filepath.Join(addonsPath, mapName)

	utf8Content := []byte("// UTF-8 secret\nMsg(\"地图脚本\")\n")
	gbkContent, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("// GBK\nMsg(\"中文脚本\")\n"))
	if err != nil {
		t.Fatalf("encode GBK: %v", err)
	}
	largeContent := bytes.Repeat([]byte("x"), maxGlobalScriptContentBytes+37)
	writeMapInspectionTestVPK(t, vpkPath, map[string][]byte{
		"scripts/vscripts/mapspawn_addon.nut":      utf8Content,
		"scripts/vscripts/scriptedmode_addon.nut":  gbkContent,
		"scripts/vscripts/director_base_addon.nut": largeContent,
	})
	if err := os.WriteFile(consts.MapListFilePath, []byte(mapName+"\n"), 0644); err != nil {
		t.Fatalf("write maplist: %v", err)
	}

	if err := InspectAndStoreMapVPK(mapName, vpkPath); err != nil {
		t.Fatalf("InspectAndStoreMapVPK() error = %v", err)
	}

	stored, err := os.ReadFile(consts.MapVPKInspectionsPath)
	if err != nil {
		t.Fatalf("read persisted inspection: %v", err)
	}
	if bytes.Contains(stored, []byte("UTF-8 secret")) || len(stored) == 0 {
		t.Fatalf("persisted inspection unexpectedly contains script content: %s", stored)
	}

	scripts, err := GetMapGlobalScriptContents(mapName)
	if err != nil {
		t.Fatalf("GetMapGlobalScriptContents() error = %v", err)
	}
	if len(scripts) != 3 {
		t.Fatalf("script count = %d, want 3", len(scripts))
	}
	byPath := make(map[string]GlobalScriptContent, len(scripts))
	for _, script := range scripts {
		byPath[script.Path] = script
	}

	utf8Script := byPath["scripts/vscripts/mapspawn_addon.nut"]
	if utf8Script.Encoding != "utf-8" || utf8Script.Content != string(utf8Content) || utf8Script.Truncated {
		t.Fatalf("UTF-8 script = %+v", utf8Script)
	}
	gbkScript := byPath["scripts/vscripts/scriptedmode_addon.nut"]
	if gbkScript.Encoding != "gbk" || !strings.Contains(gbkScript.Content, "中文脚本") || gbkScript.Truncated {
		t.Fatalf("GBK script = %+v", gbkScript)
	}
	largeScript := byPath["scripts/vscripts/director_base_addon.nut"]
	if !largeScript.Truncated || largeScript.Size != int64(len(largeContent)) || len(largeScript.Content) != maxGlobalScriptContentBytes {
		t.Fatalf("large script metadata = size:%d truncated:%v content:%d", largeScript.Size, largeScript.Truncated, len(largeScript.Content))
	}

	resetMapVPKInspectionStoreForTest()
	info, err := os.Stat(vpkPath)
	if err != nil {
		t.Fatalf("stat map: %v", err)
	}
	reloaded := GetMapVPKInspection(mapName, info)
	if reloaded.GlobalScripts.Status != GlobalScriptsStatusDetected || len(reloaded.GlobalScripts.Files) != 3 {
		t.Fatalf("reloaded inspection = %+v", reloaded)
	}

	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(vpkPath, future, future); err != nil {
		t.Fatalf("change map mtime: %v", err)
	}
	if _, err := GetMapGlobalScriptContents(mapName); !errors.Is(err, ErrMapInspectionStale) {
		t.Fatalf("stale read error = %v, want ErrMapInspectionStale", err)
	}
}

func TestMapVPKInspectionPersistenceAndScriptOverrideContents(t *testing.T) {
	addonsPath := setupMapVPKInspectionTestPaths(t)
	mapName := "overrides.vpk"
	vpkPath := filepath.Join(addonsPath, mapName)

	utf8Content := []byte("WeaponData\n{\n\t// 地图武器覆盖\n}\n")
	gbkContent, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("GameModes\n{\n\t// 中文模式配置\n}\n"))
	if err != nil {
		t.Fatalf("encode GBK: %v", err)
	}
	largeContent := bytes.Repeat([]byte("x"), maxGlobalScriptContentBytes+19)
	writeMapInspectionTestVPK(t, vpkPath, map[string][]byte{
		"scripts/weapon_rifle.txt": utf8Content,
		"SCRIPTS/GAMEMODES.TXT":    gbkContent,
		"scripts/weapon_smg.txt":   largeContent,
		"scripts/readme.txt":       []byte("ignored"),
	})
	if err := os.WriteFile(consts.MapListFilePath, []byte(mapName+"\n"), 0644); err != nil {
		t.Fatalf("write maplist: %v", err)
	}

	if err := InspectAndStoreMapVPK(mapName, vpkPath); err != nil {
		t.Fatalf("InspectAndStoreMapVPK() error = %v", err)
	}

	stored, err := os.ReadFile(consts.MapVPKInspectionsPath)
	if err != nil {
		t.Fatalf("read persisted inspection: %v", err)
	}
	if bytes.Contains(stored, []byte("WeaponData")) {
		t.Fatalf("persisted inspection unexpectedly contains override content: %s", stored)
	}

	scripts, err := GetMapScriptOverrideContents(mapName)
	if err != nil {
		t.Fatalf("GetMapScriptOverrideContents() error = %v", err)
	}
	if len(scripts) != 3 {
		t.Fatalf("script count = %d, want 3", len(scripts))
	}
	byPath := make(map[string]ScriptOverrideContent, len(scripts))
	for _, script := range scripts {
		byPath[script.Path] = script
	}

	utf8Script := byPath["scripts/weapon_rifle.txt"]
	if utf8Script.Encoding != "utf-8" || utf8Script.Content != string(utf8Content) || utf8Script.Truncated {
		t.Fatalf("UTF-8 override = %+v", utf8Script)
	}
	gbkScript := byPath["scripts/gamemodes.txt"]
	if gbkScript.Encoding != "gbk" || !strings.Contains(gbkScript.Content, "中文模式配置") || gbkScript.Truncated {
		t.Fatalf("GBK override = %+v", gbkScript)
	}
	largeScript := byPath["scripts/weapon_smg.txt"]
	if !largeScript.Truncated || largeScript.Size != int64(len(largeContent)) || len(largeScript.Content) != maxGlobalScriptContentBytes {
		t.Fatalf("large override metadata = size:%d truncated:%v content:%d", largeScript.Size, largeScript.Truncated, len(largeScript.Content))
	}

	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(vpkPath, future, future); err != nil {
		t.Fatalf("change map mtime: %v", err)
	}
	if _, err := GetMapScriptOverrideContents(mapName); !errors.Is(err, ErrMapInspectionStale) {
		t.Fatalf("stale read error = %v, want ErrMapInspectionStale", err)
	}
}

func TestGetMapGlobalScriptContentsIsolatesMissingItemAndRejectsNonWhitelistPath(t *testing.T) {
	addonsPath := setupMapVPKInspectionTestPaths(t)
	mapName := "isolated.vpk"
	vpkPath := filepath.Join(addonsPath, mapName)
	writeMapInspectionTestVPK(t, vpkPath, map[string][]byte{
		"scripts/vscripts/mapspawn_addon.nut": []byte("valid script"),
	})
	if err := os.WriteFile(consts.MapListFilePath, []byte(mapName+"\n"), 0644); err != nil {
		t.Fatalf("write maplist: %v", err)
	}
	if err := InspectAndStoreMapVPK(mapName, vpkPath); err != nil {
		t.Fatalf("InspectAndStoreMapVPK() error = %v", err)
	}

	mapVPKInspectionMu.Lock()
	record := mapVPKInspectionRecords[mapName]
	record.Inspection.GlobalScripts.Files = append(
		record.Inspection.GlobalScripts.Files,
		"scripts/vscripts/director_base_addon.nut",
		"scripts/vscripts/not_whitelisted.nut",
	)
	mapVPKInspectionRecords[mapName] = record
	mapVPKInspectionMu.Unlock()

	scripts, err := GetMapGlobalScriptContents(mapName)
	if err != nil {
		t.Fatalf("GetMapGlobalScriptContents() error = %v", err)
	}
	if len(scripts) != 2 {
		t.Fatalf("script count = %d, want valid and missing whitelist entries only: %+v", len(scripts), scripts)
	}
	if scripts[0].Error != "" {
		t.Fatalf("valid script error = %q", scripts[0].Error)
	}
	if scripts[1].Path != "scripts/vscripts/director_base_addon.nut" || scripts[1].Error == "" {
		t.Fatalf("missing script result = %+v", scripts[1])
	}
	for _, script := range scripts {
		if script.Path == "scripts/vscripts/not_whitelisted.nut" {
			t.Fatalf("non-whitelisted path was returned: %+v", script)
		}
	}
}

func TestGetMapScriptOverrideContentsIsolatesMissingItemAndRejectsUnmatchedPath(t *testing.T) {
	addonsPath := setupMapVPKInspectionTestPaths(t)
	mapName := "override-isolated.vpk"
	vpkPath := filepath.Join(addonsPath, mapName)
	writeMapInspectionTestVPK(t, vpkPath, map[string][]byte{
		"scripts/weapon_rifle.txt": []byte("valid override"),
	})
	if err := os.WriteFile(consts.MapListFilePath, []byte(mapName+"\n"), 0644); err != nil {
		t.Fatalf("write maplist: %v", err)
	}
	if err := InspectAndStoreMapVPK(mapName, vpkPath); err != nil {
		t.Fatalf("InspectAndStoreMapVPK() error = %v", err)
	}

	mapVPKInspectionMu.Lock()
	record := mapVPKInspectionRecords[mapName]
	record.Inspection.ScriptOverrides.Files = append(
		record.Inspection.ScriptOverrides.Files,
		"scripts/weapon_smg.txt",
		"scripts/readme.txt",
	)
	mapVPKInspectionRecords[mapName] = record
	mapVPKInspectionMu.Unlock()

	scripts, err := GetMapScriptOverrideContents(mapName)
	if err != nil {
		t.Fatalf("GetMapScriptOverrideContents() error = %v", err)
	}
	if len(scripts) != 2 {
		t.Fatalf("script count = %d, want valid and missing matched entries only: %+v", len(scripts), scripts)
	}
	if scripts[0].Error != "" {
		t.Fatalf("valid override error = %q", scripts[0].Error)
	}
	if scripts[1].Path != "scripts/weapon_smg.txt" || scripts[1].Error == "" {
		t.Fatalf("missing override result = %+v", scripts[1])
	}
	for _, script := range scripts {
		if script.Path == "scripts/readme.txt" {
			t.Fatalf("unmatched path was returned: %+v", script)
		}
	}
}

func TestMapSummaryEnrichesMissingDictionaryChapters(t *testing.T) {
	addonsPath := setupMapVPKInspectionTestPaths(t)
	mapName := "campaign.vpk"
	vpkPath := filepath.Join(addonsPath, mapName)
	writeMapInspectionTestVPK(t, vpkPath, map[string][]byte{
		"missions/campaign.txt": []byte(`"mission"
{
	"DisplayTitle" "港口战役"
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "c5m1_harbor"
				"DisplayName" "第一章"
			}
		}
	}
}`),
		"maps/C5M1_Harbor.bsp": makeMapInspectionTestBSP(t, false),
		"maps/Unmatched.bsp":   makeMapInspectionTestBSP(t, false),
	})
	if err := os.WriteFile(consts.MapListFilePath, []byte(mapName+"\n"), 0644); err != nil {
		t.Fatalf("write maplist: %v", err)
	}
	if err := InspectAndStoreMapVPK(mapName, vpkPath); err != nil {
		t.Fatalf("InspectAndStoreMapVPK() error = %v", err)
	}

	summary := GetMapSummaries([]string{mapName})[mapName]
	if summary.Error != "" {
		t.Fatalf("summary error = %q", summary.Error)
	}
	if summary.Inspection.Dictionary.Status != DictionaryStatusMissing {
		t.Fatalf("dictionary status = %q", summary.Inspection.Dictionary.Status)
	}

	chapters := make(map[string]DictionaryChapterInspection)
	for _, chapter := range summary.Inspection.Dictionary.Chapters {
		chapters[chapter.ChapterCode] = chapter
	}
	matched := chapters["C5M1_Harbor"]
	if matched.CampaignTitle != "港口战役" || matched.ChapterTitle != "第一章" {
		t.Fatalf("matched chapter labels = %+v", matched)
	}
	unmatched := chapters["Unmatched"]
	if unmatched.CampaignTitle != "" || unmatched.ChapterTitle != "" || unmatched.BSPPath == "" {
		t.Fatalf("unmatched chapter fallback = %+v", unmatched)
	}
}

func TestMapVPKInspectionRecordLifecycle(t *testing.T) {
	addonsPath := setupMapVPKInspectionTestPaths(t)
	legacyPath := filepath.Join(addonsPath, "legacy.vpk")
	writeMapInspectionTestVPK(t, legacyPath, map[string][]byte{
		"missions/legacy.txt": []byte("legacy"),
	})
	legacyInfo, err := os.Stat(legacyPath)
	if err != nil {
		t.Fatalf("stat legacy map: %v", err)
	}
	legacyInspection := GetMapVPKInspection("legacy.vpk", legacyInfo)
	if legacyInspection.Dictionary.Status != DictionaryStatusNotChecked ||
		legacyInspection.GlobalScripts.Status != GlobalScriptsStatusNotChecked ||
		legacyInspection.ScriptOverrides.Status != ScriptOverridesStatusNotChecked {
		t.Fatalf("legacy inspection = %+v, want not_checked", legacyInspection)
	}

	oldName := "old.vpk"
	newName := "renamed.vpk"
	oldPath := filepath.Join(addonsPath, oldName)
	newPath := filepath.Join(addonsPath, newName)
	writeMapInspectionTestVPK(t, oldPath, map[string][]byte{
		"missions/map.txt": []byte("map"),
	})
	if err := InspectAndStoreMapVPK(oldName, oldPath); err != nil {
		t.Fatalf("store old inspection: %v", err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("rename map file: %v", err)
	}
	if err := RenameMapVPKInspection(oldName, newName); err != nil {
		t.Fatalf("rename inspection: %v", err)
	}
	newInfo, err := os.Stat(newPath)
	if err != nil {
		t.Fatalf("stat renamed map: %v", err)
	}
	renamedInspection := GetMapVPKInspection(newName, newInfo)
	if renamedInspection.Dictionary.Status != DictionaryStatusNotApplicable || renamedInspection.ScriptOverrides.Status != ScriptOverridesStatusClean {
		t.Fatalf("renamed inspection = %+v", renamedInspection)
	}
	if err := DeleteMapVPKInspection(newName); err != nil {
		t.Fatalf("delete inspection: %v", err)
	}
	if deleted := GetMapVPKInspection(newName, newInfo); deleted.Dictionary.Status != DictionaryStatusNotChecked {
		t.Fatalf("deleted inspection = %+v, want not_checked", deleted)
	}

	for _, name := range []string{"keep.vpk", "drop.vpk"} {
		path := filepath.Join(addonsPath, name)
		writeMapInspectionTestVPK(t, path, map[string][]byte{
			"missions/map.txt": []byte(name),
		})
		if err := InspectAndStoreMapVPK(name, path); err != nil {
			t.Fatalf("store %s inspection: %v", name, err)
		}
	}
	if err := RetainMapVPKInspections([]string{"keep.vpk"}); err != nil {
		t.Fatalf("retain inspections: %v", err)
	}
	keepInfo, err := os.Stat(filepath.Join(addonsPath, "keep.vpk"))
	if err != nil {
		t.Fatalf("stat kept map: %v", err)
	}
	dropInfo, err := os.Stat(filepath.Join(addonsPath, "drop.vpk"))
	if err != nil {
		t.Fatalf("stat dropped map: %v", err)
	}
	if kept := GetMapVPKInspection("keep.vpk", keepInfo); kept.Dictionary.Status != DictionaryStatusNotApplicable {
		t.Fatalf("kept inspection = %+v", kept)
	}
	if dropped := GetMapVPKInspection("drop.vpk", dropInfo); dropped.Dictionary.Status != DictionaryStatusNotChecked {
		t.Fatalf("dropped inspection = %+v, want not_checked", dropped)
	}
}

func TestDecodeGlobalScriptContentUsesSafeUnknownFallback(t *testing.T) {
	content, encoding := decodeGlobalScriptContent([]byte{0x80, 'a'}, false)
	if encoding != "unknown" || !strings.Contains(content, "�") {
		t.Fatalf("decode result = %q (%s), want safe unknown replacement", content, encoding)
	}
}

func setupMapVPKInspectionTestPaths(t *testing.T) string {
	t.Helper()

	oldAddonsBasePath := consts.AddonsBasePath
	oldMapListFilePath := consts.MapListFilePath
	oldMapVPKInspectionsPath := consts.MapVPKInspectionsPath
	root := t.TempDir()
	addonsPath := filepath.Join(root, "addons")
	if err := os.MkdirAll(addonsPath, 0755); err != nil {
		t.Fatalf("create addons path: %v", err)
	}
	consts.AddonsBasePath = addonsPath
	consts.MapListFilePath = filepath.Join(addonsPath, "maplist.txt")
	consts.MapVPKInspectionsPath = filepath.Join(root, "data", "map_vpk_inspections.json")
	resetMapVPKInspectionStoreForTest()
	resetMapSummaryCacheForTest()

	t.Cleanup(func() {
		consts.AddonsBasePath = oldAddonsBasePath
		consts.MapListFilePath = oldMapListFilePath
		consts.MapVPKInspectionsPath = oldMapVPKInspectionsPath
		resetMapVPKInspectionStoreForTest()
		resetMapSummaryCacheForTest()
	})
	return addonsPath
}

func makeMapInspectionTestBSP(t *testing.T, withDictionary bool) []byte {
	return makeMapInspectionTestBSPWithLayout(t, withDictionary, false)
}

func makeMapInspectionTestBSPWithLayout(t *testing.T, withDictionary, versionFirst bool) []byte {
	t.Helper()

	var pak bytes.Buffer
	zipWriter := zip.NewWriter(&pak)
	entryName := "materials/readme.txt"
	if withDictionary {
		entryName = "stringtable_dictionary.dct"
	}
	entry, err := zipWriter.Create(entryName)
	if err != nil {
		t.Fatalf("create BSP pak entry: %v", err)
	}
	if _, err := entry.Write([]byte("test")); err != nil {
		t.Fatalf("write BSP pak entry: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close BSP pak: %v", err)
	}

	header := make([]byte, bspHeaderSize)
	copy(header[:4], "VBSP")
	binary.LittleEndian.PutUint32(header[4:8], 21)
	lumpOffset := 8 + bspPakfileLump*bspLumpSize
	if versionFirst {
		binary.LittleEndian.PutUint32(header[lumpOffset+4:lumpOffset+8], uint32(len(header)))
		binary.LittleEndian.PutUint32(header[lumpOffset+8:lumpOffset+12], uint32(pak.Len()))
	} else {
		binary.LittleEndian.PutUint32(header[lumpOffset:lumpOffset+4], uint32(len(header)))
		binary.LittleEndian.PutUint32(header[lumpOffset+4:lumpOffset+8], uint32(pak.Len()))
	}
	return append(header, pak.Bytes()...)
}

func writeMapInspectionTestVPK(t *testing.T, path string, contents map[string][]byte) {
	t.Helper()

	type entry struct {
		dir     string
		base    string
		ext     string
		content []byte
		offset  uint32
	}
	entries := make([]entry, 0, len(contents))
	for name, content := range contents {
		dir, base, ext := splitMapDetailVPKName(t, name)
		entries = append(entries, entry{dir: dir, base: base, ext: ext, content: content})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ext != entries[j].ext {
			return entries[i].ext < entries[j].ext
		}
		if entries[i].dir != entries[j].dir {
			return entries[i].dir < entries[j].dir
		}
		return entries[i].base < entries[j].base
	})

	files := make([]vpk.File, 0, len(entries))
	var offset uint32
	for index := range entries {
		current := &entries[index]
		current.offset = offset
		offset += uint32(len(current.content))
		files = append(files, vpk.File{
			Dir:  current.dir,
			Base: current.base,
			Ext:  current.ext,
			DirEntry: vpk.DirEntry{
				CRC:           crc32.ChecksumIEEE(current.content),
				MetadataBytes: 0,
				DataLocation: []vpk.DataChunk{{
					ArchiveIndex: testVPKSelfArchive,
					EntryOffset:  current.offset,
					EntryLength:  uint32(len(current.content)),
				}},
			},
		})
	}

	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test VPK: %v", err)
	}
	defer out.Close()
	archive := &vpk.Archive{
		Header: vpk.Header{Magic: vpk.Magic, Version: 1},
		Files:  files,
	}
	if err := vpk.WriteDirectory(out, archive); err != nil {
		t.Fatalf("write VPK directory: %v", err)
	}
	for _, current := range entries {
		if _, err := out.Write(current.content); err != nil {
			t.Fatalf("write VPK content: %v", err)
		}
	}
}
