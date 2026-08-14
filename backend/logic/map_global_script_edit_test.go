package logic

import (
	"bytes"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"l4d2-manager-next/consts"
	"l4d2-manager-next/pkg/valve/vpk"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestUpdateMapGlobalScriptRepackagesAndPreservesOtherEntries(t *testing.T) {
	addonsPath := setupMapVPKInspectionTestPaths(t)
	mapName := "editable.vpk"
	vpkPath := filepath.Join(addonsPath, mapName)
	originalOther := []byte("preserve this mission data")
	originalGBK, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("// 原始 GBK\nMsg(\"旧内容\")\n"))
	if err != nil {
		t.Fatalf("encode original GBK: %v", err)
	}
	writeMapInspectionTestVPK(t, vpkPath, map[string][]byte{
		"missions/editable.txt":                   originalOther,
		"scripts/vscripts/mapspawn_addon.nut":     []byte("// old utf-8\n"),
		"scripts/vscripts/scriptedmode_addon.nut": originalGBK,
	})
	if err := os.WriteFile(consts.MapListFilePath, []byte(mapName+"\n"), 0644); err != nil {
		t.Fatalf("write maplist: %v", err)
	}
	if err := InspectAndStoreMapVPK(mapName, vpkPath); err != nil {
		t.Fatalf("inspect map: %v", err)
	}

	_, revision, err := GetMapGlobalScriptContentsWithRevision(mapName)
	if err != nil {
		t.Fatalf("get scripts: %v", err)
	}
	newUTF8 := "// edited UTF-8\nMsg(\"新内容\")\n"
	updated, err := UpdateMapGlobalScript(
		mapName,
		"scripts/vscripts/mapspawn_addon.nut",
		newUTF8,
		"utf-8",
		revision,
	)
	if err != nil {
		t.Fatalf("UpdateMapGlobalScript() error = %v", err)
	}
	if updated.Revision == revision {
		t.Fatal("map revision did not change after repack")
	}
	if updated.Script.Content != newUTF8 || updated.Script.Encoding != "utf-8" {
		t.Fatalf("updated script = %+v", updated.Script)
	}

	contents := readMapScriptEditTestVPK(t, vpkPath)
	if !bytes.Equal(contents["scripts/vscripts/mapspawn_addon.nut"], []byte(newUTF8)) {
		t.Fatalf("UTF-8 script = %q, want %q", contents["scripts/vscripts/mapspawn_addon.nut"], newUTF8)
	}
	if !bytes.Equal(contents["scripts/vscripts/scriptedmode_addon.nut"], originalGBK) {
		t.Fatal("editing one script changed the other global script")
	}
	if !bytes.Equal(contents["missions/editable.txt"], originalOther) {
		t.Fatal("editing a script changed another VPK entry")
	}

	newGBKText := "// 编辑后的 GBK\nMsg(\"中文内容\")\n"
	updatedGBK, err := UpdateMapGlobalScript(
		mapName,
		"scripts/vscripts/scriptedmode_addon.nut",
		newGBKText,
		"gbk",
		updated.Revision,
	)
	if err != nil {
		t.Fatalf("update GBK script: %v", err)
	}
	wantGBK, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(newGBKText))
	if err != nil {
		t.Fatalf("encode wanted GBK: %v", err)
	}
	contents = readMapScriptEditTestVPK(t, vpkPath)
	if !bytes.Equal(contents["scripts/vscripts/scriptedmode_addon.nut"], wantGBK) {
		t.Fatal("GBK script encoding was not preserved")
	}

	info, err := os.Stat(vpkPath)
	if err != nil {
		t.Fatalf("stat edited map: %v", err)
	}
	inspection := GetMapVPKInspection(mapName, info)
	if inspection.GlobalScripts.Status != GlobalScriptsStatusDetected || len(inspection.GlobalScripts.Files) != 2 {
		t.Fatalf("inspection after edit = %+v", inspection.GlobalScripts)
	}
	if updatedGBK.Revision == updated.Revision {
		t.Fatal("second edit did not advance the map revision")
	}
}

func TestUpdateMapGlobalScriptRejectsStaleAndOversizedEdits(t *testing.T) {
	addonsPath := setupMapVPKInspectionTestPaths(t)
	mapName := "conflict.vpk"
	vpkPath := filepath.Join(addonsPath, mapName)
	writeMapInspectionTestVPK(t, vpkPath, map[string][]byte{
		"scripts/vscripts/director_base_addon.nut": []byte("original"),
	})
	if err := os.WriteFile(consts.MapListFilePath, []byte(mapName+"\n"), 0644); err != nil {
		t.Fatalf("write maplist: %v", err)
	}
	if err := InspectAndStoreMapVPK(mapName, vpkPath); err != nil {
		t.Fatalf("inspect map: %v", err)
	}
	_, revision, err := GetMapGlobalScriptContentsWithRevision(mapName)
	if err != nil {
		t.Fatalf("get scripts: %v", err)
	}

	first, err := UpdateMapGlobalScript(
		mapName,
		"scripts/vscripts/director_base_addon.nut",
		"first update is longer",
		"utf-8",
		revision,
	)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	if _, err := UpdateMapGlobalScript(
		mapName,
		"scripts/vscripts/director_base_addon.nut",
		"stale overwrite",
		"utf-8",
		revision,
	); !errors.Is(err, ErrMapRevisionConflict) {
		t.Fatalf("stale update error = %v, want ErrMapRevisionConflict", err)
	}

	contents := readMapScriptEditTestVPK(t, vpkPath)
	if got := string(contents["scripts/vscripts/director_base_addon.nut"]); got != "first update is longer" {
		t.Fatalf("content after stale update = %q", got)
	}
	if _, err := UpdateMapGlobalScript(
		mapName,
		"scripts/vscripts/director_base_addon.nut",
		strings.Repeat("x", maxGlobalScriptContentBytes+1),
		"utf-8",
		first.Revision,
	); !errors.Is(err, ErrGlobalScriptContentTooLarge) {
		t.Fatalf("oversized update error = %v, want ErrGlobalScriptContentTooLarge", err)
	}
}

func TestRepackSingleFileVPKEntryPreservesPreloadAndMultipleChunks(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.vpk")
	destinationPath := filepath.Join(dir, "destination.vpk")
	preservedContent := []byte("PREdata")
	targetContent := []byte("old")

	out, err := os.Create(sourcePath)
	if err != nil {
		t.Fatalf("create source VPK: %v", err)
	}
	archive := &vpk.Archive{
		Header: vpk.Header{Magic: vpk.Magic, Version: 1},
		Files: []vpk.File{
			{
				Dir:  "scripts/vscripts",
				Base: "mapspawn_addon",
				Ext:  "nut",
				DirEntry: vpk.DirEntry{
					CRC:           crc32.ChecksumIEEE(targetContent),
					MetadataBytes: 0,
					DataLocation: []vpk.DataChunk{{
						ArchiveIndex: rawVPKSelfArchive,
						EntryOffset:  4,
						EntryLength:  uint32(len(targetContent)),
					}},
				},
			},
			{
				Dir:      "scripts/vscripts",
				Base:     "ordinary",
				Ext:      "nut",
				Metadata: []byte("PRE"),
				DirEntry: vpk.DirEntry{
					CRC:           crc32.ChecksumIEEE(preservedContent),
					MetadataBytes: 3,
					DataLocation: []vpk.DataChunk{
						{ArchiveIndex: rawVPKSelfArchive, EntryOffset: 0, EntryLength: 2},
						{ArchiveIndex: rawVPKSelfArchive, EntryOffset: 2, EntryLength: 2},
					},
				},
			},
		},
	}
	if err := vpk.WriteDirectory(out, archive); err != nil {
		_ = out.Close()
		t.Fatalf("write source VPK directory: %v", err)
	}
	if _, err := out.Write([]byte("dataold")); err != nil {
		_ = out.Close()
		t.Fatalf("write source VPK data: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close source VPK: %v", err)
	}

	replacement := []byte("new target content")
	if err := repackSingleFileVPKEntry(
		sourcePath,
		destinationPath,
		"scripts/vscripts/mapspawn_addon.nut",
		replacement,
	); err != nil {
		t.Fatalf("repack VPK: %v", err)
	}
	contents := readMapScriptEditTestVPK(t, destinationPath)
	if !bytes.Equal(contents["scripts/vscripts/mapspawn_addon.nut"], replacement) {
		t.Fatal("target entry was not replaced")
	}
	if !bytes.Equal(contents["scripts/vscripts/ordinary.nut"], preservedContent) {
		t.Fatalf("preload/multi-chunk entry = %q, want %q", contents["scripts/vscripts/ordinary.nut"], preservedContent)
	}
}

func readMapScriptEditTestVPK(t *testing.T, path string) map[string][]byte {
	t.Helper()
	opener := vpk.Single(path)
	defer opener.Close()
	archive, err := opener.ReadArchive()
	if err != nil {
		t.Fatalf("read edited VPK: %v", err)
	}

	contents := make(map[string][]byte, len(archive.Files))
	for index := range archive.Files {
		archiveFile := &archive.Files[index]
		content, err := archiveFile.Bytes(opener)
		if err != nil {
			t.Fatalf("read VPK entry %s: %v", archiveFile.Name(), err)
		}
		contents[normalizeVPKEntryPath(archiveFile.Name())] = content
	}
	return contents
}
