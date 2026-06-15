package controller

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"

	"github.com/gin-gonic/gin"
)

func TestFinalizeVpkFileFallsBackToOriginalWhenTrimUnsupported(t *testing.T) {
	oldAddonsBasePath := consts.AddonsBasePath
	oldMapListFilePath := consts.MapListFilePath
	oldManagerDataPath := consts.ManagerDataPath
	oldManagerConfigPath := consts.ManagerConfigPath

	root := t.TempDir()
	addonsPath := filepath.Join(root, "addons")
	if err := os.MkdirAll(addonsPath, 0755); err != nil {
		t.Fatalf("create addons dir: %v", err)
	}

	consts.AddonsBasePath = addonsPath
	consts.MapListFilePath = filepath.Join(addonsPath, "maplist.txt")
	consts.ManagerDataPath = filepath.Join(root, "data")
	consts.ManagerConfigPath = filepath.Join(consts.ManagerDataPath, "manager_config.json")

	if err := logic.SetVPKTrimEnable(true); err != nil {
		t.Fatalf("enable vpk trim: %v", err)
	}
	t.Cleanup(func() {
		_ = logic.SetVPKTrimEnable(false)
		consts.AddonsBasePath = oldAddonsBasePath
		consts.MapListFilePath = oldMapListFilePath
		consts.ManagerDataPath = oldManagerDataPath
		consts.ManagerConfigPath = oldManagerConfigPath
	})

	sourcePath := filepath.Join(root, "incoming.vpk")
	sourceContent := []byte("not a supported vpk layout")
	if err := os.WriteFile(sourcePath, sourceContent, 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := finalizeVpkFile(sourcePath, "unsupported.vpk"); err != nil {
		t.Fatalf("finalizeVpkFile() error = %v", err)
	}

	destPath := filepath.Join(addonsPath, "unsupported.vpk")
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(destContent) != string(sourceContent) {
		t.Fatalf("dest content = %q, want original content", destContent)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("source file still exists or stat failed: %v", err)
	}

	mapList, err := os.ReadFile(consts.MapListFilePath)
	if err != nil {
		t.Fatalf("read maplist: %v", err)
	}
	if !strings.Contains(string(mapList), "unsupported.vpk\n") {
		t.Fatalf("maplist = %q, want unsupported.vpk entry", mapList)
	}
}

func TestTrimMapKeepsOriginalWhenTrimUnsupported(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldAddonsBasePath := consts.AddonsBasePath
	oldMapListFilePath := consts.MapListFilePath

	root := t.TempDir()
	addonsPath := filepath.Join(root, "addons")
	if err := os.MkdirAll(addonsPath, 0755); err != nil {
		t.Fatalf("create addons dir: %v", err)
	}

	consts.AddonsBasePath = addonsPath
	consts.MapListFilePath = filepath.Join(addonsPath, "maplist.txt")
	t.Cleanup(func() {
		consts.AddonsBasePath = oldAddonsBasePath
		consts.MapListFilePath = oldMapListFilePath
	})

	sourceContent := []byte("not a supported vpk layout")
	sourcePath := filepath.Join(addonsPath, "unsupported.vpk")
	if err := os.WriteFile(sourcePath, sourceContent, 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(consts.MapListFilePath, []byte("unsupported.vpk\n"), 0644); err != nil {
		t.Fatalf("write maplist: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/maps/trim", strings.NewReader("map=unsupported.vpk"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	TrimMap(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %q", w.Code, http.StatusOK, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"trimmed":false`) {
		t.Fatalf("body = %q, want trimmed false", w.Body.String())
	}

	gotContent, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(gotContent) != string(sourceContent) {
		t.Fatalf("source content = %q, want original content", gotContent)
	}
}
