package logic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"l4d2-manager-next/consts"
)

func TestGetMapSummariesParsesAndCachesByFileState(t *testing.T) {
	addonsPath := setupMapSummaryTestPaths(t)
	resetMapSummaryCacheForTest()

	vpkPath := filepath.Join(addonsPath, "detail.vpk")
	writeMapDetailTestVPK(t, vpkPath, map[string]string{
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
		}
	}
}`,
	})
	if err := os.WriteFile(consts.MapListFilePath, []byte("detail.vpk\n"), 0644); err != nil {
		t.Fatalf("write maplist: %v", err)
	}

	oldParser := parseMapSummaryVPK
	parseCalls := 0
	parseMapSummaryVPK = func(path string) ([]*Campaign, error) {
		parseCalls++
		return oldParser(path)
	}
	t.Cleanup(func() {
		parseMapSummaryVPK = oldParser
		resetMapSummaryCacheForTest()
	})

	first := GetMapSummaries([]string{"detail.vpk"})["detail.vpk"]
	if first.Error != "" {
		t.Fatalf("first summary error = %q", first.Error)
	}
	if first.Title != "Detail Campaign" {
		t.Fatalf("first title = %q, want Detail Campaign", first.Title)
	}
	if first.ChapterCount != 1 {
		t.Fatalf("first chapter count = %d, want 1", first.ChapterCount)
	}
	if parseCalls != 1 {
		t.Fatalf("parseCalls after first request = %d, want 1", parseCalls)
	}

	second := GetMapSummaries([]string{"detail.vpk"})["detail.vpk"]
	if second.Title != "Detail Campaign" {
		t.Fatalf("second title = %q, want cached Detail Campaign", second.Title)
	}
	if parseCalls != 1 {
		t.Fatalf("parseCalls after cached request = %d, want 1", parseCalls)
	}

	writeMapDetailTestVPK(t, vpkPath, map[string]string{
		"missions/detail.txt": `"mission"
{
	"DisplayTitle" "Updated Detail Campaign"
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "updated_m1"
				"DisplayName" "Updated One"
			}
			"2"
			{
				"Map" "updated_m2"
				"DisplayName" "Updated Two"
			}
		}
	}
}`,
	})
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(vpkPath, future, future); err != nil {
		t.Fatalf("update vpk mtime: %v", err)
	}

	updated := GetMapSummaries([]string{"detail.vpk"})["detail.vpk"]
	if updated.Error != "" {
		t.Fatalf("updated summary error = %q", updated.Error)
	}
	if updated.Title != "Updated Detail Campaign" {
		t.Fatalf("updated title = %q, want Updated Detail Campaign", updated.Title)
	}
	if updated.ChapterCount != 2 {
		t.Fatalf("updated chapter count = %d, want 2", updated.ChapterCount)
	}
	if parseCalls != 2 {
		t.Fatalf("parseCalls after changed file = %d, want 2", parseCalls)
	}
}

func TestGetMapSummariesReturnsPerItemErrors(t *testing.T) {
	addonsPath := setupMapSummaryTestPaths(t)
	resetMapSummaryCacheForTest()

	if err := os.WriteFile(filepath.Join(addonsPath, "bad.vpk"), []byte("not a valid vpk"), 0644); err != nil {
		t.Fatalf("write bad vpk: %v", err)
	}
	if err := os.WriteFile(consts.MapListFilePath, []byte("bad.vpk\nmissing.vpk\nnotvpk.txt\n"), 0644); err != nil {
		t.Fatalf("write maplist: %v", err)
	}
	t.Cleanup(resetMapSummaryCacheForTest)

	result := GetMapSummaries([]string{
		"../escape.vpk",
		"notvpk.txt",
		"ghost.vpk",
		"missing.vpk",
		"bad.vpk",
	})

	for _, name := range []string{"../escape.vpk", "notvpk.txt", "ghost.vpk", "missing.vpk", "bad.vpk"} {
		summary, ok := result[name]
		if !ok {
			t.Fatalf("missing summary for %s", name)
		}
		if strings.TrimSpace(summary.Error) == "" {
			t.Fatalf("summary for %s has empty error: %+v", name, summary)
		}
	}
}

func setupMapSummaryTestPaths(t *testing.T) string {
	t.Helper()

	oldAddonsPath := consts.AddonsBasePath
	oldMapListPath := consts.MapListFilePath

	addonsPath := t.TempDir()
	consts.AddonsBasePath = addonsPath
	consts.MapListFilePath = filepath.Join(addonsPath, "maplist.txt")
	t.Cleanup(func() {
		consts.AddonsBasePath = oldAddonsPath
		consts.MapListFilePath = oldMapListPath
	})

	return addonsPath
}
