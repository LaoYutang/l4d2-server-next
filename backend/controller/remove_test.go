package controller

import (
	"l4d2-manager-next/consts"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRemoveRejectsInvalidMapNamesWithoutChangingFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rootDir, addonsPath := setupRemoveTestPaths(t)

	mapPath := filepath.Join(addonsPath, "safe.vpk")
	outsidePath := filepath.Join(rootDir, "outside.vpk")
	if err := os.WriteFile(mapPath, []byte("map"), 0644); err != nil {
		t.Fatalf("write map: %v", err)
	}
	if err := os.WriteFile(outsidePath, []byte("outside"), 0644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	originalMapList := "safe.vpk\n../outside.vpk\n"
	if err := os.WriteFile(consts.MapListFilePath, []byte(originalMapList), 0644); err != nil {
		t.Fatalf("write maplist: %v", err)
	}

	invalidNames := []string{
		"",
		"../outside.vpk",
		`..\outside.vpk`,
		"/tmp/outside.vpk",
		`C:\temp\outside.vpk`,
		"safe.txt",
		"safe.vpk/child",
	}
	for _, mapName := range invalidNames {
		t.Run(mapName, func(t *testing.T) {
			c, w := newFormTestContext("/remove", url.Values{"map": {mapName}})
			Remove(c)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %q, want 400", w.Code, w.Body.String())
			}
			for _, path := range []string{mapPath, outsidePath} {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("file %s changed after invalid request: %v", path, err)
				}
			}
			mapList, err := os.ReadFile(consts.MapListFilePath)
			if err != nil {
				t.Fatalf("read maplist: %v", err)
			}
			if string(mapList) != originalMapList {
				t.Fatalf("maplist = %q, want unchanged %q", string(mapList), originalMapList)
			}
		})
	}
}

func TestRemoveDeletesValidVPKAndMapListRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, mapName := range []string{"campaign.vpk", "campaign.VPK"} {
		t.Run(mapName, func(t *testing.T) {
			_, addonsPath := setupRemoveTestPaths(t)
			mapPath := filepath.Join(addonsPath, mapName)
			if err := os.WriteFile(mapPath, []byte("map"), 0644); err != nil {
				t.Fatalf("write map: %v", err)
			}
			if err := os.WriteFile(consts.MapListFilePath, []byte(mapName+"\nother.vpk\n"), 0644); err != nil {
				t.Fatalf("write maplist: %v", err)
			}

			c, w := newFormTestContext("/remove", url.Values{"map": {mapName}})
			Remove(c)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %q, want 200", w.Code, w.Body.String())
			}
			if _, err := os.Stat(mapPath); !os.IsNotExist(err) {
				t.Fatalf("map still exists: %v", err)
			}
			mapList, err := os.ReadFile(consts.MapListFilePath)
			if err != nil {
				t.Fatalf("read maplist: %v", err)
			}
			if strings.Contains(string(mapList), mapName) {
				t.Fatalf("maplist still contains %q: %q", mapName, string(mapList))
			}
			if !strings.Contains(string(mapList), "other.vpk") {
				t.Fatalf("maplist lost unrelated record: %q", string(mapList))
			}
		})
	}
}

func TestRemoveCleansMapListWhenValidVPKIsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupRemoveTestPaths(t)

	if err := os.WriteFile(consts.MapListFilePath, []byte("missing.vpk\nother.vpk\n"), 0644); err != nil {
		t.Fatalf("write maplist: %v", err)
	}

	c, w := newFormTestContext("/remove", url.Values{"map": {"missing.vpk"}})
	Remove(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q, want 200", w.Code, w.Body.String())
	}
	mapList, err := os.ReadFile(consts.MapListFilePath)
	if err != nil {
		t.Fatalf("read maplist: %v", err)
	}
	if strings.Contains(string(mapList), "missing.vpk") {
		t.Fatalf("maplist still contains missing map: %q", string(mapList))
	}
	if !strings.Contains(string(mapList), "other.vpk") {
		t.Fatalf("maplist lost unrelated record: %q", string(mapList))
	}
}

func setupRemoveTestPaths(t *testing.T) (string, string) {
	t.Helper()

	oldAddonsBasePath := consts.AddonsBasePath
	oldMapListFilePath := consts.MapListFilePath
	rootDir := t.TempDir()
	addonsPath := filepath.Join(rootDir, "addons")
	if err := os.MkdirAll(addonsPath, 0755); err != nil {
		t.Fatalf("create addons path: %v", err)
	}
	consts.AddonsBasePath = addonsPath
	consts.MapListFilePath = filepath.Join(addonsPath, "maplist.txt")
	if err := os.WriteFile(consts.MapListFilePath, nil, 0644); err != nil {
		t.Fatalf("create maplist: %v", err)
	}
	t.Cleanup(func() {
		consts.AddonsBasePath = oldAddonsBasePath
		consts.MapListFilePath = oldMapListFilePath
	})
	return rootDir, addonsPath
}
