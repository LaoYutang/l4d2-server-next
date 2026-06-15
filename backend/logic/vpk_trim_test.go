package logic

import (
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"l4d2-manager-next/pkg/valve/vpk"
)

func TestShouldRemoveVPKTrimFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "materials/test/demo.vtf", want: true},
		{path: "materials/demo.vtf", want: true},
		{path: "sound/test/theme.mp3", want: true},
		{path: "sound/test/theme.wav", want: true},
		{path: "sounds/test/theme.mp3", want: true},
		{path: "models/props/test.vvd", want: true},
		{path: "models/props/test.sw.vtx", want: true},
		{path: "maps/source.vmf", want: true},
		{path: "source.vmx", want: true},
		{path: "materials/test/demo.vmt", want: false},
		{path: "maps/c1m1_hotel.bsp", want: false},
		{path: "missions/test.txt", want: false},
		{path: "models/props/test.mdl", want: false},
		{path: "unknown/test.vtf", want: false},
		{path: "root.vtf", want: false},
		{path: "sound/test/theme.ogg", want: false},
		{path: "no_extension", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := shouldRemoveVPKTrimFile(tt.path); got != tt.want {
				t.Fatalf("shouldRemoveVPKTrimFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestTrimRawSingleFileVPKDeletesSafeServerFiles(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.vpk")
	trimmedPath := filepath.Join(dir, "trimmed.vpk")

	writeVPKTrimTestVPK(t, sourcePath, map[string]string{
		"materials/test/demo.vtf": "delete texture",
		"materials/test/demo.vmt": "keep material",
		"sound/test/theme.wav":    "delete sound",
		"models/test/demo.vvd":    "delete model data",
		"maps/test.vmf":           "delete source map",
		"maps/test.bsp":           "keep bsp",
		"missions/test.txt":       "keep mission",
		"unknown/test.vtf":        "keep unknown texture",
	})

	removedCount, _, err := trimRawSingleFileVPK(sourcePath, trimmedPath)
	if err != nil {
		t.Fatalf("trimRawSingleFileVPK() error = %v", err)
	}
	if removedCount != 4 {
		t.Fatalf("removedCount = %d, want 4", removedCount)
	}

	opener := vpk.Single(trimmedPath)
	defer opener.Close()
	archive, err := opener.ReadArchive()
	if err != nil {
		t.Fatalf("read trimmed archive: %v", err)
	}

	names := make(map[string]bool)
	for _, file := range archive.Files {
		names[file.Name()] = true
	}

	for _, name := range []string{
		"materials/test/demo.vtf",
		"sound/test/theme.wav",
		"models/test/demo.vvd",
		"maps/test.vmf",
	} {
		if names[name] {
			t.Fatalf("trimmed archive kept %s", name)
		}
	}

	for _, name := range []string{
		"materials/test/demo.vmt",
		"maps/test.bsp",
		"missions/test.txt",
		"unknown/test.vtf",
	} {
		if !names[name] {
			t.Fatalf("trimmed archive missing %s", name)
		}
	}
}

func writeVPKTrimTestVPK(t *testing.T, path string, contents map[string]string) {
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
		entries = append(entries, entry{
			dir:     dir,
			base:    base,
			ext:     ext,
			content: []byte(content),
		})
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
	for i := range entries {
		entries[i].offset = offset
		offset += uint32(len(entries[i].content))

		files = append(files, vpk.File{
			Dir:  entries[i].dir,
			Base: entries[i].base,
			Ext:  entries[i].ext,
			DirEntry: vpk.DirEntry{
				CRC:           crc32.ChecksumIEEE(entries[i].content),
				MetadataBytes: 0,
				DataLocation: []vpk.DataChunk{{
					ArchiveIndex: rawVPKSelfArchive,
					EntryOffset:  entries[i].offset,
					EntryLength:  uint32(len(entries[i].content)),
				}},
			},
		})
	}

	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test vpk: %v", err)
	}
	defer out.Close()

	archive := &vpk.Archive{
		Header: vpk.Header{
			Magic:   vpk.Magic,
			Version: 1,
		},
		Files: files,
	}
	if err := vpk.WriteDirectory(out, archive); err != nil {
		t.Fatalf("write vpk directory: %v", err)
	}
	for _, entry := range entries {
		if _, err := out.Write(entry.content); err != nil {
			t.Fatalf("write vpk content: %v", err)
		}
	}
}
