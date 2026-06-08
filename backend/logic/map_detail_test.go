package logic

import (
	"hash/crc32"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"l4d2-manager-next/consts"
	"l4d2-manager-next/pkg/valve/vpk"
)

const testVPKSelfArchive = 0x7fff

func TestGetMapMissionDetailParsesSingleVPK(t *testing.T) {
	oldAddonsPath := consts.AddonsBasePath
	t.Cleanup(func() {
		consts.AddonsBasePath = oldAddonsPath
	})

	addonsPath := t.TempDir()
	consts.AddonsBasePath = addonsPath
	writeMapDetailTestVPK(t, filepath.Join(addonsPath, "detail.vpk"), map[string]string{
		"missions/detail.txt": `"mission"
{
	"DisplayTitle" "Detail Campaign"
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "detail_m1"
				"DisplayName" "Detail One"
			}
			"2"
			{
				"Map" "detail_m2"
				"DisplayName" "Detail Two"
			}
		}
		"mutation1"
		{
			"1"
			{
				"Map" "detail_m1"
				"DisplayName" "Detail One Mutation"
			}
		}
	}
}`,
	})

	campaigns, err := GetMapMissionDetail("detail.vpk")
	if err != nil {
		t.Fatalf("GetMapMissionDetail() error = %v", err)
	}
	if len(campaigns) != 1 {
		t.Fatalf("len(campaigns) = %d, want 1: %+v", len(campaigns), campaigns)
	}

	campaign := campaigns[0]
	if campaign.Title != "Detail Campaign" {
		t.Fatalf("campaign.Title = %q, want %q", campaign.Title, "Detail Campaign")
	}
	if campaign.VpkName != "detail.vpk" {
		t.Fatalf("campaign.VpkName = %q, want detail.vpk", campaign.VpkName)
	}
	if len(campaign.Chapters) != 2 {
		t.Fatalf("len(campaign.Chapters) = %d, want 2: %+v", len(campaign.Chapters), campaign.Chapters)
	}
	assertMapDetailChapter(t, campaign.Chapters[0], "detail_m1", "Detail One", []string{"coop", "mutation1"})
	assertMapDetailChapter(t, campaign.Chapters[1], "detail_m2", "Detail Two", []string{"coop"})
}

func TestGetMapMissionDetailRejectsPathTraversal(t *testing.T) {
	_, err := GetMapMissionDetail("../detail.vpk")
	if err == nil {
		t.Fatal("GetMapMissionDetail() error = nil, want invalid filename error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "invalid") {
		t.Fatalf("GetMapMissionDetail() error = %q, want invalid filename error", err.Error())
	}
}

func assertMapDetailChapter(t *testing.T, chapter *Chapter, wantCode string, wantTitle string, wantModes []string) {
	t.Helper()

	if chapter.Code != wantCode {
		t.Fatalf("chapter.Code = %q, want %q", chapter.Code, wantCode)
	}
	if chapter.Title != wantTitle {
		t.Fatalf("chapter.Title = %q, want %q", chapter.Title, wantTitle)
	}
	if !reflect.DeepEqual(chapter.Modes, wantModes) {
		t.Fatalf("chapter.Modes = %v, want %v", chapter.Modes, wantModes)
	}
}

func writeMapDetailTestVPK(t *testing.T, path string, contents map[string]string) {
	t.Helper()

	entries := make([]mapDetailTestVPKEntry, 0, len(contents))
	for name, content := range contents {
		dir, base, ext := splitMapDetailVPKName(t, name)
		entries = append(entries, mapDetailTestVPKEntry{
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
		entry := &entries[i]
		entry.offset = offset
		offset += uint32(len(entry.content))

		files = append(files, vpk.File{
			Dir:  entry.dir,
			Base: entry.base,
			Ext:  entry.ext,
			DirEntry: vpk.DirEntry{
				CRC:           crc32.ChecksumIEEE(entry.content),
				MetadataBytes: 0,
				DataLocation: []vpk.DataChunk{{
					ArchiveIndex: testVPKSelfArchive,
					EntryOffset:  entry.offset,
					EntryLength:  uint32(len(entry.content)),
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

type mapDetailTestVPKEntry struct {
	dir     string
	base    string
	ext     string
	content []byte
	offset  uint32
}

func splitMapDetailVPKName(t *testing.T, name string) (string, string, string) {
	t.Helper()

	slash := strings.LastIndex(name, "/")
	dir := " "
	fileName := name
	if slash >= 0 {
		dir = name[:slash]
		fileName = name[slash+1:]
	}

	dot := strings.LastIndex(fileName, ".")
	if dot <= 0 || dot == len(fileName)-1 {
		t.Fatalf("invalid test vpk file name %q", name)
	}

	return dir, fileName[:dot], fileName[dot+1:]
}
