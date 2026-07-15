package vpkmission

import (
	"hash/crc32"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"l4d2-manager-next/pkg/valve/vpk"
)

const vpkSelfArchive = 0x7fff

func TestParseMissionDiscoversGenericModesAndMergesChapters(t *testing.T) {
	campaign, err := ParseMission(strings.NewReader(`"mission"
{
	"DisplayTitle" "Merged Campaign"
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "c1m1_alpha"
				"DisplayName" "Alpha"
			}
			"2"
			{
				"Map" "c1m2_beta"
				"DisplayName" "Beta"
			}
		}
		"mutation1"
		{
			"1"
			{
				"Map" "c1m1_alpha"
				"DisplayName" "Alpha Mutation"
			}
		}
		"community6"
		{
			"1"
			{
				"Map" "c1m2_beta"
				"DisplayName" "Beta Community"
			}
		}
		"customdash"
		{
			"1"
			{
				"Map" "c1m1_alpha"
				"DisplayName" "Alpha Custom"
			}
		}
	}
}`))
	if err != nil {
		t.Fatalf("ParseMission() error = %v", err)
	}

	if campaign.Title != "Merged Campaign" {
		t.Fatalf("Title = %q, want %q", campaign.Title, "Merged Campaign")
	}
	if len(campaign.Chapters) != 2 {
		t.Fatalf("len(Chapters) = %d, want 2: %+v", len(campaign.Chapters), campaign.Chapters)
	}

	assertChapter(t, campaign.Chapters[0], "c1m1_alpha", "Alpha", []string{"coop", "mutation1", "customdash"})
	assertChapter(t, campaign.Chapters[1], "c1m2_beta", "Beta", []string{"coop", "community6"})
}

func TestParseMissionToleratesMissingFinalCloseBrace(t *testing.T) {
	campaign, err := ParseMission(strings.NewReader(`"mission"
{
	"DisplayTitle" "Malformed Campaign"
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "bad_m1"
				"DisplayName" "Bad One"
			}
		}
}`))
	if err != nil {
		t.Fatalf("ParseMission() error = %v", err)
	}

	if campaign.Title != "Malformed Campaign" {
		t.Fatalf("Title = %q, want %q", campaign.Title, "Malformed Campaign")
	}
	if len(campaign.Chapters) != 1 {
		t.Fatalf("len(Chapters) = %d, want 1: %+v", len(campaign.Chapters), campaign.Chapters)
	}
	assertChapter(t, campaign.Chapters[0], "bad_m1", "Bad One", []string{"coop"})
}

func TestParseMissionToleratesExtraFinalCloseBraces(t *testing.T) {
	campaign, err := ParseMission(strings.NewReader(`"mission"
{
	"DisplayTitle" "Extra Braces Campaign"
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "extra_m1"
				"DisplayName" "Extra One"
			}
		}
	}
}
}
}`))
	if err != nil {
		t.Fatalf("ParseMission() error = %v", err)
	}

	if campaign.Title != "Extra Braces Campaign" {
		t.Fatalf("Title = %q, want %q", campaign.Title, "Extra Braces Campaign")
	}
	if len(campaign.Chapters) != 1 {
		t.Fatalf("len(Chapters) = %d, want 1: %+v", len(campaign.Chapters), campaign.Chapters)
	}
	assertChapter(t, campaign.Chapters[0], "extra_m1", "Extra One", []string{"coop"})
}

func TestParseMissionToleratesUnquotedMissionKeysAndValues(t *testing.T) {
	campaign, err := ParseMission(strings.NewReader(`mission
{
	DisplayTitle "Unquoted Campaign"
	modes
	{
		coop
		{
			1
			{
				Map unquoted_m1
				DisplayName "Unquoted One" // inline comment
			}
		}
		custommode
		{
			1
			{
				Map unquoted_m1
				DisplayName UnquotedCustom
			}
		}
	}
}`))
	if err != nil {
		t.Fatalf("ParseMission() error = %v", err)
	}

	if campaign.Title != "Unquoted Campaign" {
		t.Fatalf("Title = %q, want %q", campaign.Title, "Unquoted Campaign")
	}
	if len(campaign.Chapters) != 1 {
		t.Fatalf("len(Chapters) = %d, want 1: %+v", len(campaign.Chapters), campaign.Chapters)
	}
	assertChapter(t, campaign.Chapters[0], "unquoted_m1", "Unquoted One", []string{"coop", "custommode"})
}

func TestParseVPKMergesSameCampaignAndReturnsSeparateCampaigns(t *testing.T) {
	vpkPath := filepath.Join(t.TempDir(), "multi.vpk")
	writeTestVPK(t, vpkPath, map[string]string{
		"missions/shared_coop.txt": `"mission"
{
	"DisplayTitle" "Shared Campaign"
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "shared_m1"
				"DisplayName" "Shared One"
			}
			"2"
			{
				"Map" "shared_m2"
				"DisplayName" "Shared Two"
			}
		}
	}
}`,
		"missions/shared_mutation.txt": `"mission"
{
	"DisplayTitle" "Shared Campaign"
	"modes"
	{
		"mutation1"
		{
			"1"
			{
				"Map" "shared_m1"
				"DisplayName" "Shared One Mutation"
			}
		}
	}
}`,
		"missions/other.txt": `"mission"
{
	"DisplayTitle" "Other Campaign"
	"modes"
	{
		"community6"
		{
			"1"
			{
				"Map" "other_m1"
				"DisplayName" "Other One"
			}
		}
	}
}`,
	})

	campaigns, err := ParseVPK(vpkPath)
	if err != nil {
		t.Fatalf("ParseVPK() error = %v", err)
	}
	if len(campaigns) != 2 {
		t.Fatalf("len(campaigns) = %d, want 2: %+v", len(campaigns), campaigns)
	}

	shared := findCampaign(campaigns, "Shared Campaign")
	if shared == nil {
		t.Fatalf("Shared Campaign not found: %+v", campaigns)
	}
	if shared.VpkName != "multi.vpk" {
		t.Fatalf("shared.VpkName = %q, want multi.vpk", shared.VpkName)
	}
	if len(shared.Chapters) != 2 {
		t.Fatalf("len(shared.Chapters) = %d, want 2: %+v", len(shared.Chapters), shared.Chapters)
	}
	assertChapter(t, shared.Chapters[0], "shared_m1", "Shared One", []string{"coop", "mutation1"})
	assertChapter(t, shared.Chapters[1], "shared_m2", "Shared Two", []string{"coop"})

	other := findCampaign(campaigns, "Other Campaign")
	if other == nil {
		t.Fatalf("Other Campaign not found: %+v", campaigns)
	}
	if other.VpkName != "multi.vpk" {
		t.Fatalf("other.VpkName = %q, want multi.vpk", other.VpkName)
	}
	if len(other.Chapters) != 1 {
		t.Fatalf("len(other.Chapters) = %d, want 1: %+v", len(other.Chapters), other.Chapters)
	}
	assertChapter(t, other.Chapters[0], "other_m1", "Other One", []string{"community6"})
}

func TestParseVPKWithoutMissionFilesReturnsError(t *testing.T) {
	vpkPath := filepath.Join(t.TempDir(), "empty.vpk")
	writeTestVPK(t, vpkPath, map[string]string{
		"materials/readme.txt": "not a mission",
	})

	_, err := ParseVPK(vpkPath)
	if err == nil {
		t.Fatal("ParseVPK() error = nil, want missing mission error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "mission") {
		t.Fatalf("ParseVPK() error = %q, want mention mission", err.Error())
	}
}

func assertChapter(t *testing.T, chapter *Chapter, wantCode string, wantTitle string, wantModes []string) {
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

func findCampaign(campaigns []*Campaign, title string) *Campaign {
	for _, campaign := range campaigns {
		if campaign.Title == title {
			return campaign
		}
	}
	return nil
}

func writeTestVPK(t *testing.T, path string, contents map[string]string) {
	t.Helper()

	entries := make([]testVPKEntry, 0, len(contents))
	for name, content := range contents {
		dir, base, ext := splitVPKName(t, name)
		entries = append(entries, testVPKEntry{
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
					ArchiveIndex: vpkSelfArchive,
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

type testVPKEntry struct {
	dir     string
	base    string
	ext     string
	content []byte
	offset  uint32
}

func splitVPKName(t *testing.T, name string) (string, string, string) {
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
