package logic

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/pkg/valve/vpk"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"gopkg.in/yaml.v3"
)

func putPlugin(t *testing.T, store, name string, files map[string]string) {
	t.Helper()
	for p, content := range files {
		writeTestFile(t, filepath.Join(store, name, "left4dead2", filepath.FromSlash(p)), content)
	}
}

func mustPluginState(t *testing.T) *pluginState {
	t.Helper()
	s, err := readPluginState()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustReadPluginFile(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPluginTypeClassification(t *testing.T) {
	store, _ := setupPluginTestPaths(t)
	for _, tc := range []struct {
		name, want string
		files      map[string]string
	}{
		{"nut-config-assets", "nut", map[string]string{"scripts/vscripts/a.nut": "script", "cfg/a.cfg": "x", "ems/a/settings.json": "{}", "materials/a.vmt": "asset", "README.txt": "readme"}},
		{"source-mod", "sm", map[string]string{"addons/sourcemod/plugins/a.smx": "smx", "cfg/a.cfg": "cfg", "addons/sourcemod/scripting/a.sp": "source"}},
		{"windows-platform", "sm", map[string]string{"addons/metamod/bin/server.dll": "dll", "addons/sourcemod/plugins/a.smx": "smx", "addons/sourcemod/scripting/spcomp.exe": "compiler", "addons/sourcemod/scripting/spcomp64.exe": "compiler64", "addons/sourcemod/scripting/compile.exe": "wrapper"}},
		{"linux-platform", "sm", map[string]string{"addons/metamod/bin/server.so": "so", "addons/sourcemod/plugins/a.smx": "smx", "addons/sourcemod/scripting/spcomp": "compiler", "addons/sourcemod/scripting/spcomp64": "compiler64", "addons/sourcemod/scripting/compile.sh": "wrapper"}},
		{"compiler-only", "other", map[string]string{"addons/sourcemod/scripting/spcomp.exe": "compiler", "addons/sourcemod/scripting/a.sp": "source"}},
		{"unknown-executable", "other", map[string]string{"addons/sourcemod/plugins/a.smx": "smx", "addons/sourcemod/scripting/unknown.exe": "other"}},
		{"compiler-outside-sourcemod", "other", map[string]string{"addons/sourcemod/plugins/a.smx": "smx", "bin/spcomp.exe": "other"}},
		{"sourcemod-with-vpk", "other", map[string]string{"addons/sourcemod/plugins/a.smx": "smx", "addons/a.vpk": "vpk"}},
		{"metamod", "sm", map[string]string{"addons/metamod/bin/server.so": "so"}},
		{"mixed", "mix", map[string]string{"scripts/vscripts/a.nut": "nut", "addons/sourcemod/plugins/a.smx": "smx"}},
		{"already-packed", "other", map[string]string{"scripts/vscripts/a.nut": "nut", "addons/a.vpk": "vpk"}},
		{"source-only", "other", map[string]string{"addons/sourcemod/scripting/a.sp": "source", "cfg/a.cfg": "cfg"}},
		{"misplaced-nut", "other", map[string]string{"a.nut": "nut"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			putPlugin(t, store, tc.name, tc.files)
			meta, _, err := scanPlugin(tc.name)
			if err != nil {
				t.Fatal(err)
			}
			if meta.Type != tc.want {
				t.Fatalf("type=%s, want %s", meta.Type, tc.want)
			}
		})
	}
}

func TestPluginMetadataMigrationPreservesLegacyDeployment(t *testing.T) {
	store, game := setupPluginTestPaths(t)
	legacy := "# keep this comment\nenabled_plugins:\n  - name: Legacy\n    files: [scripts/vscripts/a.nut, cfg/a.cfg]\n    custom: keep\nplugin_sources:\n  Legacy: upload\nfuture_setting: {nested: preserved}\n"
	writeTestFile(t, getConfigPath(), legacy)
	putPlugin(t, store, "Legacy", map[string]string{"scripts/vscripts/a.nut": "source", "cfg/a.cfg": "default"})
	writeTestFile(t, filepath.Join(game, "scripts/vscripts/a.nut"), "running")
	writeTestFile(t, filepath.Join(game, "cfg/a.cfg"), "edited")
	if err := InitPluginMetadata(); err != nil {
		t.Fatal(err)
	}
	s := mustPluginState(t)
	id := s.metadata("Legacy").ID
	if s.Version != 2 || s.metadata("Legacy").Type != "nut" || s.Enabled[0].DeploymentMode != "loose" || s.Enabled[0].Extra["custom"] != "keep" {
		t.Fatalf("migration lost state: %+v", s)
	}
	if string(mustReadPluginFile(t, getConfigPath()+".v1.bak")) != legacy {
		t.Fatal("legacy backup changed")
	}
	before := mustReadPluginFile(t, getConfigPath())
	if !bytes.Contains(before, []byte("future_setting")) || !bytes.Contains(before, []byte("keep this comment")) {
		t.Fatal("unknown settings lost")
	}
	// Existing valid metadata is not rescanned at every startup.
	writeTestFile(t, filepath.Join(store, "Legacy/left4dead2/addons/sourcemod/plugins/a.smx"), "smx")
	if err := InitPluginMetadata(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, mustReadPluginFile(t, getConfigPath())) {
		t.Fatal("migration is not idempotent")
	}
	if string(mustReadPluginFile(t, filepath.Join(game, "scripts/vscripts/a.nut"))) != "running" {
		t.Fatal("startup modified deployment")
	}
	if err := os.Remove(filepath.Join(store, "Legacy/left4dead2/addons/sourcemod/plugins/a.smx")); err != nil {
		t.Fatal(err)
	}
	if err := DisablePlugin("Legacy"); err != nil {
		t.Fatal(err)
	}
	if string(mustReadPluginFile(t, filepath.Join(game, "cfg/a.cfg"))) != "edited" {
		t.Fatal("legacy config deleted")
	}
	if err := EnablePlugin("Legacy"); err != nil {
		t.Fatal(err)
	}
	s = mustPluginState(t)
	if s.Enabled[0].DeploymentMode != "vpk" || s.metadata("Legacy").ID != id {
		t.Fatal("reenable did not preserve identity and switch to VPK")
	}
}

func TestPluginMetadataRejectsBrokenConfigWithoutOverwrite(t *testing.T) {
	store, _ := setupPluginTestPaths(t)
	putPlugin(t, store, "P", map[string]string{"scripts/vscripts/a.nut": "nut"})
	for _, broken := range []string{"enabled_plugins: [", "schema_version: 999\n", "plugin_sources: {}\nplugin_sources: {}\n", "enabled_plugins: wrong\n"} {
		writeTestFile(t, getConfigPath(), broken)
		if err := InitPluginMetadata(); err == nil {
			t.Fatalf("accepted %q", broken)
		}
		if err := EnablePlugin("P"); err == nil {
			t.Fatal("enabled with broken state")
		}
		if string(mustReadPluginFile(t, getConfigPath())) != broken {
			t.Fatal("broken config overwritten")
		}
	}
}

func TestPluginTypeRuleUpgradeCorrectsCachedWindowsPlatform(t *testing.T) {
	store, game := setupPluginTestPaths(t)
	putPlugin(t, store, "Windows", map[string]string{
		"addons/metamod/bin/server.dll":         "dll",
		"addons/sourcemod/plugins/a.smx":        "smx",
		"addons/sourcemod/scripting/spcomp.exe": "compiler",
		"cfg/sourcemod/sourcemod.cfg":           "default",
	})
	if err := EnablePlugin("Windows"); err != nil {
		t.Fatal(err)
	}
	state := mustPluginState(t)
	m := state.metadata("Windows")
	id := m.ID
	m.Type, m.TypeDetectionVersion = "other", 0 // Persisted by the original detector.
	m.Extra = map[string]any{"custom": "keep"}
	enabled := state.Enabled
	if err := state.save(); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(game, "cfg/sourcemod/sourcemod.cfg"), "edited")
	if err := InitPluginMetadata(); err != nil {
		t.Fatal(err)
	}
	state = mustPluginState(t)
	m = state.metadata("Windows")
	if m.Type != "sm" || m.TypeDetectionVersion != pluginTypeDetectionVersion || m.ID != id || m.Extra["custom"] != "keep" {
		t.Fatalf("cached classification not upgraded correctly: %+v", m)
	}
	if !reflect.DeepEqual(state.Enabled, enabled) {
		t.Fatal("type upgrade changed deployment records")
	}
	if string(mustReadPluginFile(t, filepath.Join(game, "cfg/sourcemod/sourcemod.cfg"))) != "edited" {
		t.Fatal("type upgrade modified game config")
	}
	before := mustReadPluginFile(t, getConfigPath())
	if err := InitPluginMetadata(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, mustReadPluginFile(t, getConfigPath())) {
		t.Fatal("type upgrade repeated unnecessarily")
	}
}

func TestPluginUploadAndDownloadPersistTypes(t *testing.T) {
	store, _ := setupPluginTestPaths(t)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for p, data := range map[string]string{"left4dead2/scripts/vscripts/a.nut": "nut", "left4dead2/cfg/a.cfg": "cfg"} {
		f, err := w.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := UploadPlugin(bytes.NewReader(buf.Bytes()), int64(buf.Len()), "Upload.zip"); err != nil {
		t.Fatal(err)
	}
	temp := filepath.Join(store, DownloadTempDir, "download")
	putPlugin(t, filepath.Join(store, DownloadTempDir), "download", map[string]string{"addons/sourcemod/plugins/a.smx": "smx"})
	if err := commitDownloadedPlugin(context.Background(), temp, "Download"); err != nil {
		t.Fatal(err)
	}
	s := mustPluginState(t)
	if s.metadata("Upload").Type != "nut" || s.metadata("Download").Type != "sm" || s.Sources["Upload"] != "upload" || s.Sources["Download"] != "store" {
		t.Fatal("import metadata missing")
	}
	// An invalid archive cannot leave behind a partially extracted plugin.
	var bad bytes.Buffer
	z := zip.NewWriter(&bad)
	f, _ := z.Create("left4dead2/../../escape.cfg")
	_, _ = f.Write([]byte("x"))
	_ = z.Close()
	if err := UploadPlugin(bytes.NewReader(bad.Bytes()), int64(bad.Len()), "Bad.zip"); err == nil {
		t.Fatal("accepted traversal archive")
	}
	if _, err := os.Stat(filepath.Join(store, "Bad")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed upload retained")
	}
}

func TestNUTVPKContentAndExternalConfigs(t *testing.T) {
	store, game := setupPluginTestPaths(t)
	files := map[string]string{
		"scripts/vscripts/mapspawn_addon.nut": "IncludeScript(\"demo/main\")",
		"scripts/vscripts/demo/main.nut":      "FileToString(\"demo/settings.cfg\")",
		"materials/demo.vmt":                  "material", "asset_without_extension": "data", ".asset": "hidden asset",
		"cfg/demo.cfg": "default", "ems/demo/settings.cfg": "settings", "ems/demo/state.json": "{}", "scripts/vscripts/demo/custom.cfg": "custom",
	}
	putPlugin(t, store, "Demo", files)
	writeTestFile(t, filepath.Join(game, "cfg/demo.cfg"), "already edited")
	if err := EnablePlugin("Demo"); err != nil {
		t.Fatal(err)
	}
	s := mustPluginState(t)
	record := s.Enabled[0]
	if record.DeploymentMode != "vpk" || filepath.ToSlash(filepath.Dir(record.VPKFile)) != "addons" {
		t.Fatalf("invalid deployment: %+v", record)
	}
	opener := vpk.Single(filepath.Join(game, filepath.FromSlash(record.VPKFile)))
	archive, err := opener.ReadArchive()
	if err != nil {
		t.Fatal(err)
	}
	if archive.Version != 1 {
		t.Fatal("not a v1 VPK")
	}
	contents := map[string]string{}
	for _, f := range archive.Files {
		b, err := f.Bytes(opener)
		if err != nil {
			t.Fatal(err)
		}
		contents[f.Name()] = string(b)
	}
	opener.Close()
	for p, data := range files {
		if isLooseNUTFile(p) {
			if _, ok := contents[p]; ok {
				t.Fatalf("config packed: %s", p)
			}
			if p == "cfg/demo.cfg" {
				data = "already edited"
			}
			if string(mustReadPluginFile(t, filepath.Join(game, filepath.FromSlash(p)))) != data {
				t.Fatalf("external path/content changed: %s", p)
			}
		} else if contents[p] != data {
			t.Fatalf("VPK payload differs: %s", p)
		}
	}
	if contents["addoninfo.txt"] == "" {
		t.Fatal("missing addoninfo")
	}
	if _, err := os.Stat(filepath.Join(game, "scripts/vscripts/mapspawn_addon.nut")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("script deployed loose")
	}
	if err := DisablePlugin("Demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(game, record.VPKFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("VPK not removed")
	}
	if string(mustReadPluginFile(t, filepath.Join(game, "ems/demo/settings.cfg"))) != "settings" {
		t.Fatal("EMS config deleted")
	}
	if string(mustReadPluginFile(t, filepath.Join(store, "Demo/left4dead2/cfg/demo.cfg"))) != "default" {
		t.Fatal("source defaults changed")
	}
}

func TestNUTConflictsAndGlobalEntries(t *testing.T) {
	store, game := setupPluginTestPaths(t)
	for name, files := range map[string]map[string]string{
		"A":     {"scripts/vscripts/mapspawn_addon.nut": "a", "scripts/vscripts/common.nut": "shared"},
		"B":     {"scripts/vscripts/mapspawn_addon.nut": "b", "scripts/vscripts/common.nut": "shared"},
		"C":     {"scripts/vscripts/common.nut": "different"},
		"Mixed": {"scripts/vscripts/common.nut": "different", "addons/sourcemod/plugins/m.smx": "smx"},
	} {
		putPlugin(t, store, name, files)
	}
	if err := EnablePlugin("A"); err != nil {
		t.Fatal(err)
	}
	if err := EnablePlugin("B"); err != nil {
		t.Fatalf("distinct addon global entries should coexist: %v", err)
	}
	before := mustReadPluginFile(t, getConfigPath())
	for _, name := range []string{"C", "Mixed"} {
		if err := EnablePlugin(name); err == nil || !strings.Contains(err.Error(), "冲突") {
			t.Fatalf("conflict accepted: %s %v", name, err)
		}
		if !bytes.Equal(before, mustReadPluginFile(t, getConfigPath())) {
			t.Fatal("failed enable changed state")
		}
	}
	if err := DisablePlugin("A"); err != nil {
		t.Fatal(err)
	}
	remaining := mustPluginState(t).Enabled[0]
	if _, err := os.Stat(filepath.Join(game, remaining.VPKFile)); err != nil {
		t.Fatal("disabling A removed B")
	}
	putPlugin(t, store, "LooseConflict", map[string]string{"scripts/vscripts/loose.nut": "new"})
	writeTestFile(t, filepath.Join(game, "scripts/vscripts/loose.nut"), "unmanaged")
	if err := EnablePlugin("LooseConflict"); err == nil {
		t.Fatal("overwrote loose script")
	}
	if string(mustReadPluginFile(t, filepath.Join(game, "scripts/vscripts/loose.nut"))) != "unmanaged" {
		t.Fatal("loose file changed")
	}
}

func TestNUTEnableRollbackAndReclassification(t *testing.T) {
	store, game := setupPluginTestPaths(t)
	putPlugin(t, store, "P", map[string]string{"scripts/vscripts/p.nut": "nut", "cfg/a.cfg": "new", "ems/p/settings.cfg": "new"})
	if err := InitPluginMetadata(); err != nil {
		t.Fatal(err)
	}
	before := mustReadPluginFile(t, getConfigPath())
	// A directory where a config file should be forces failure after the CFG copy.
	if err := os.MkdirAll(filepath.Join(game, "ems/p/settings.cfg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := EnablePlugin("P"); err == nil {
		t.Fatal("enable unexpectedly succeeded")
	}
	if _, err := os.Stat(filepath.Join(game, "cfg/a.cfg")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("copied config not rolled back")
	}
	if !bytes.Equal(before, mustReadPluginFile(t, getConfigPath())) {
		t.Fatal("failed deployment persisted")
	}
	entries, err := os.ReadDir(game)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".manager-plugin-") {
			t.Fatal("rollback staging retained")
		}
	}
	if err := os.Remove(filepath.Join(game, "ems/p/settings.cfg")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(store, "P/left4dead2/addons/sourcemod/plugins/p.smx"), "smx")
	if err := EnablePlugin("P"); err != nil {
		t.Fatal(err)
	}
	s := mustPluginState(t)
	if s.metadata("P").Type != "mix" || s.Enabled[0].DeploymentMode != "loose" {
		t.Fatal("enable did not reclassify modified package")
	}
}

func TestNUTTextConfigEncodingRevisionAndOwnership(t *testing.T) {
	store, game := setupPluginTestPaths(t)
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("中文配置\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	putPlugin(t, store, "P", map[string]string{"scripts/vscripts/p.nut": "nut", "cfg/p.cfg": "\xef\xbb\xbfvalue=1\r\n", "ems/p/settings.cfg": string(gbk)})
	if err := InitPluginMetadata(); err != nil {
		t.Fatal(err)
	}
	if _, err := ListPluginTextConfigs("P"); !errors.Is(err, ErrPluginConfigUnavailable) {
		t.Fatal("disabled plugin editable")
	}
	if err := EnablePlugin("P"); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(game, "ems/p/generated.json"), "{\"generated\":true}")
	writeTestFile(t, filepath.Join(game, "ems/other/secret.cfg"), "secret")
	list, err := ListPluginTextConfigs("P")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("owned generated configs not found: %+v", list)
	}
	for _, p := range []string{"../secret.cfg", "ems/other/secret.cfg", "cfg/server.cfg", "scripts/vscripts/p.nut", "ems/p/../other/secret.cfg", "ems/p/settings.cfg:stream"} {
		if _, err := ReadPluginTextConfig("P", p); !errors.Is(err, ErrPluginConfigPath) {
			t.Fatalf("read allowed: %s %v", p, err)
		}
		if _, err := UpdatePluginTextConfig("P", p, "attack", "rev"); !errors.Is(err, ErrPluginConfigPath) {
			t.Fatalf("write allowed: %s %v", p, err)
		}
	}
	cfg, err := ReadPluginTextConfig("P", "cfg/p.cfg")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.BOM || cfg.Newline != "crlf" || cfg.Encoding != "utf-8" {
		t.Fatalf("wrong text metadata: %+v", cfg)
	}
	updated, err := UpdatePluginTextConfig("P", cfg.Path, "value=2\n", cfg.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(mustReadPluginFile(t, filepath.Join(game, cfg.Path))); got != "\xef\xbb\xbfvalue=2\r\n" {
		t.Fatalf("format changed: %q", got)
	}
	if _, err := UpdatePluginTextConfig("P", cfg.Path, "stale", cfg.Revision); !errors.Is(err, ErrPluginConfigConflict) {
		t.Fatal("stale revision accepted")
	}
	if updated.Revision == cfg.Revision {
		t.Fatal("revision unchanged")
	}
	cfg, err = ReadPluginTextConfig("P", "ems/p/settings.cfg")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Encoding != "gbk" || cfg.Content != "中文配置\r\n" {
		t.Fatal("GBK not decoded")
	}
	if _, err := UpdatePluginTextConfig("P", cfg.Path, "修改配置\n", cfg.Revision); err != nil {
		t.Fatal(err)
	}
	raw := mustReadPluginFile(t, filepath.Join(game, cfg.Path))
	want, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("修改配置\r\n"))
	if !bytes.Equal(raw, want) {
		t.Fatal("GBK not preserved")
	}
	cfg, _ = ReadPluginTextConfig("P", cfg.Path)
	if _, err := UpdatePluginTextConfig("P", cfg.Path, "😀", cfg.Revision); err == nil {
		t.Fatal("unsupported GBK text accepted")
	}
	if !bytes.Equal(raw, mustReadPluginFile(t, filepath.Join(game, cfg.Path))) {
		t.Fatal("failed edit changed content")
	}
	for filename, content := range map[string]string{"binary.cfg": "\x00\x01", "large.cfg": strings.Repeat("a", MaxPluginTextConfigBytes+1), "invalid.cfg": "\xff"} {
		writeTestFile(t, filepath.Join(game, "ems/p", filename), content)
		if _, err := ReadPluginTextConfig("P", "ems/p/"+filename); !errors.Is(err, ErrPluginConfigNotEditable) {
			t.Fatalf("invalid text accepted: %s %v", filename, err)
		}
	}
}

func TestNUTTextConfigRejectsSymlink(t *testing.T) {
	store, game := setupPluginTestPaths(t)
	putPlugin(t, store, "P", map[string]string{"scripts/vscripts/p.nut": "nut", "ems/p/settings.cfg": "cfg"})
	if err := EnablePlugin("P"); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(game, "ems/other/private.cfg"), "private")
	if err := os.Symlink(filepath.Join(game, "ems/other"), filepath.Join(game, "ems/p/alias")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := ReadPluginTextConfig("P", "ems/p/alias/private.cfg"); err == nil {
		t.Fatal("symlink read allowed")
	}
	if _, err := ListPluginTextConfigs("P"); err == nil {
		t.Fatal("symlink enumeration allowed")
	}
}

func TestNUTBackupRoundTripAndInvalidRestore(t *testing.T) {
	store, game := setupPluginTestPaths(t)
	// All backup helpers must also use the temporary game path.
	oldAddons := consts.AddonsBasePath
	consts.AddonsBasePath = filepath.Join(game, "addons")
	t.Cleanup(func() { consts.AddonsBasePath = oldAddons })
	putPlugin(t, store, "P", map[string]string{"scripts/vscripts/p.nut": "nut", "ems/p/settings.cfg": "default"})
	if err := EnablePlugin("P"); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(game, "ems/p/settings.cfg"), "\xef\xbb\xbfedited\r\n")
	if err := CreateBackup("nut-backup"); err != nil {
		t.Fatal(err)
	}
	backup, err := GetBackupDetail("nut-backup")
	if err != nil {
		t.Fatal(err)
	}
	if len(backup.Plugins) != 1 || len(backup.Plugins[0].TextConfigs) != 1 || len(backup.Plugins[0].Configs) != 0 {
		t.Fatalf("text config missing: %+v", backup)
	}
	writeTestFile(t, filepath.Join(game, "ems/p/settings.cfg"), "newer")
	if _, err := RestoreBackup("nut-backup"); err != nil {
		t.Fatal(err)
	}
	if got := string(mustReadPluginFile(t, filepath.Join(game, "ems/p/settings.cfg"))); got != "\xef\xbb\xbfedited\r\n" {
		t.Fatalf("restored %q", got)
	}
	bad := *backup
	bad.Name = "bad"
	bad.Plugins = []BackupPlugin{{Name: "P", TextConfigs: []PluginTextBackup{{Path: "../escape.cfg", Content: "x", Encoding: "utf-8"}}}}
	if err := saveBackupConfig(&BackupConfig{Backups: []BackupEntry{bad}}); err != nil {
		t.Fatal(err)
	}
	before := mustReadPluginFile(t, getConfigPath())
	if _, err := RestoreBackup("bad"); err == nil {
		t.Fatal("invalid restore accepted")
	}
	if !bytes.Equal(before, mustReadPluginFile(t, getConfigPath())) {
		t.Fatal("invalid backup changed enabled state")
	}
	var legacy BackupPlugin
	if err := yaml.Unmarshal([]byte("name: old\nconfigs: []\n"), &legacy); err != nil || legacy.TextConfigs != nil {
		t.Fatal("legacy backup incompatible")
	}
}

func TestNUTVPKDetectsCorruption(t *testing.T) {
	store, game := setupPluginTestPaths(t)
	putPlugin(t, store, "P", map[string]string{"scripts/vscripts/p.nut": "script data"})
	if err := EnablePlugin("P"); err != nil {
		t.Fatal(err)
	}
	filename := mustPluginState(t).Enabled[0].VPKFile
	root, err := os.OpenRoot(game)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	digests, err := readPluginVPKDigests(root, filename)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(digests["scripts/vscripts/p.nut"], sha256.Sum256([]byte("script data"))) {
		t.Fatal("wrong VPK digest")
	}
	raw := mustReadPluginFile(t, filepath.Join(game, filename))
	raw[len(raw)-1] ^= 0xff
	if err := os.WriteFile(filepath.Join(game, filename), raw, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPluginVPKDigests(root, filename); err == nil {
		t.Fatal("corrupt VPK accepted")
	}
}

func TestNUTSharedEditedConfigCanBeReenabled(t *testing.T) {
	store, game := setupPluginTestPaths(t)
	for _, name := range []string{"A", "B", "Different"} {
		config := "default"
		if name == "Different" {
			config = "different default"
		}
		putPlugin(t, store, name, map[string]string{"scripts/vscripts/" + name + ".nut": name, "cfg/shared.cfg": config})
	}
	if err := EnablePlugin("A"); err != nil {
		t.Fatal(err)
	}
	if err := EnablePlugin("B"); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(game, "cfg/shared.cfg"), "edited")
	if err := DisablePlugin("B"); err != nil {
		t.Fatal(err)
	}
	if err := EnablePlugin("B"); err != nil {
		t.Fatalf("shared config edit should survive reenable: %v", err)
	}
	if string(mustReadPluginFile(t, filepath.Join(game, "cfg/shared.cfg"))) != "edited" {
		t.Fatal("shared config overwritten")
	}
	files, err := ListPluginTextConfigs("A")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || !reflect.DeepEqual(files[0].SharedBy, []string{"B"}) {
		t.Fatalf("shared ownership missing: %+v", files)
	}
	if err := EnablePlugin("Different"); err == nil {
		t.Fatal("different default config accepted")
	}
}
