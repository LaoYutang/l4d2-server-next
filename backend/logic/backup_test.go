package logic

import (
	"bytes"
	"l4d2-manager-next/consts"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func setupBackupTestPaths(t *testing.T) string {
	t.Helper()
	_, game := setupPluginTestPaths(t)
	oldAddons := consts.AddonsBasePath
	consts.AddonsBasePath = filepath.Join(game, "addons")
	t.Cleanup(func() { consts.AddonsBasePath = oldAddons })
	return game
}

func backupInfoTestPaths(game string) map[string]string {
	return map[string]string{
		"hostname": filepath.Join(game, "addons", "sourcemod", "configs", "l4d2_hostname.txt"),
		"motd":     filepath.Join(game, "motd.txt"),
		"host":     filepath.Join(game, "host.txt"),
	}
}

func TestBackupEmptyContentRoundTrip(t *testing.T) {
	for _, tt := range []struct {
		name          string
		missing       bool
		removeTargets bool
		info          BackupServerInfo
		admins        string
	}{
		{name: "empty files"},
		{name: "missing files", missing: true},
		{
			name:   "empty announcement",
			info:   BackupServerInfo{Hostname: "原服名\n", Host: "<html>原描述</html>\r\n"},
			admins: "\"STEAM_1:1:123\" \"99:z\" // 原管理员\n",
		},
		{name: "empty name and description", info: BackupServerInfo{Motd: "<html>原公告</html>\n"}},
		{
			name:          "recreate missing targets",
			removeTargets: true,
			info:          BackupServerInfo{Hostname: "原服名", Motd: "原公告", Host: "原描述"},
			admins:        "\"STEAM_1:1:123\" \"99:z\" // 原管理员\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			game := setupBackupTestPaths(t)
			paths := backupInfoTestPaths(game)
			want := map[string]string{"hostname": tt.info.Hostname, "motd": tt.info.Motd, "host": tt.info.Host}
			if !tt.missing {
				for key, path := range paths {
					writeTestFile(t, path, want[key])
				}
				writeTestFile(t, getAdminsFilePath(), tt.admins)
			}
			if err := CreateBackup("snapshot"); err != nil {
				t.Fatal(err)
			}
			exported, err := ExportBackup("snapshot")
			if err != nil {
				t.Fatal(err)
			}
			var document map[string]any
			if err := yaml.Unmarshal(exported, &document); err != nil {
				t.Fatal(err)
			}
			info, ok := document["server_info"].(map[string]any)
			if !ok || len(info) != 3 {
				t.Fatalf("export omitted server information: %s", exported)
			}
			for key, value := range want {
				if info[key] != value {
					t.Fatalf("exported %s = %q, want %q", key, info[key], value)
				}
			}
			if _, ok := document["admins"]; !ok {
				t.Fatalf("export omitted empty admin snapshot: %s", exported)
			}
			if count, err := ImportBackup(exported); err != nil || count != 1 {
				t.Fatalf("import count = %d, error = %v", count, err)
			}
			for key, path := range paths {
				writeTestFile(t, path, "new "+key)
			}
			writeTestFile(t, getAdminsFilePath(), "// preserve header\n\"STEAM_1:1:999\" \"99:z\" // new admin\n")
			if tt.removeTargets {
				for _, path := range paths {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Remove(getAdminsFilePath()); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Dir(getAdminsFilePath())); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := RestoreBackup("snapshot(1)"); err != nil {
				t.Fatal(err)
			}
			for key, path := range paths {
				if got := string(mustReadPluginFile(t, path)); got != want[key] {
					t.Fatalf("restored %s = %q, want %q", key, got, want[key])
				}
			}
			admins, err := ParseAdminsSimple()
			if err != nil {
				t.Fatal(err)
			}
			var wantAdmins []AdminUser
			if tt.admins != "" {
				wantAdmins = []AdminUser{{SteamID: "STEAM_1:1:123", Remark: "原管理员"}}
			}
			if len(admins) != len(wantAdmins) || (len(admins) > 0 && !reflect.DeepEqual(admins, wantAdmins)) {
				t.Fatalf("restored admins = %+v, want %+v", admins, wantAdmins)
			}
			if !tt.removeTargets && !strings.HasPrefix(string(mustReadPluginFile(t, getAdminsFilePath())), "// preserve header\n") {
				t.Fatal("admin file header was lost")
			}
			infos, err := ListBackups()
			if err != nil || len(infos) != 2 || infos[1].AdminCount != len(wantAdmins) {
				t.Fatalf("backup list = %+v, error = %v", infos, err)
			}
		})
	}
}

func TestRestoreBackupLegacyEmptyAndAbsentSections(t *testing.T) {
	for _, tt := range []struct {
		name        string
		sections    string
		want        map[string]string
		clearAdmins bool
	}{
		{name: "sections absent"},
		{
			name:     "omitted empty fields",
			sections: "server_info:\n  hostname: original\n",
			want:     map[string]string{"hostname": "original", "motd": "", "host": ""},
		},
		{
			name:     "empty server information",
			sections: "server_info: {}\n",
			want:     map[string]string{"hostname": "", "motd": "", "host": ""},
		},
		{name: "explicit empty admins", sections: "admins: []\n", clearAdmins: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			game := setupBackupTestPaths(t)
			data := []byte("name: legacy\ncreated_at: 1\nplugins: []\n" + tt.sections)
			if _, err := ImportBackup(data); err != nil {
				t.Fatal(err)
			}
			exported, err := ExportBackup("legacy")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ImportBackup(exported); err != nil {
				t.Fatal(err)
			}
			paths := backupInfoTestPaths(game)
			for key, path := range paths {
				writeTestFile(t, path, "current "+key)
			}
			const currentAdmins = "\"STEAM_1:1:999\" \"99:z\"\n"
			writeTestFile(t, getAdminsFilePath(), currentAdmins)
			if _, err := RestoreBackup("legacy(1)"); err != nil {
				t.Fatal(err)
			}
			for key, path := range paths {
				want := "current " + key
				if tt.want != nil {
					want = tt.want[key]
				}
				if got := string(mustReadPluginFile(t, path)); got != want {
					t.Fatalf("restored %s = %q, want %q", key, got, want)
				}
			}
			admins, err := ParseAdminsSimple()
			if err != nil {
				t.Fatal(err)
			}
			wantCount := 1
			if tt.clearAdmins {
				wantCount = 0
			}
			if len(admins) != wantCount {
				t.Fatalf("admin count = %d, want %d", len(admins), wantCount)
			}
		})
	}
}

func TestBackupFileErrors(t *testing.T) {
	for _, operation := range []string{"capture", "restore"} {
		for _, field := range []string{"hostname", "motd", "host", "admins"} {
			t.Run(operation+"/"+field, func(t *testing.T) {
				game := setupBackupTestPaths(t)
				if err := CreateBackup("snapshot"); err != nil {
					t.Fatal(err)
				}
				before := mustReadPluginFile(t, getBackupPath())
				paths := backupInfoTestPaths(game)
				paths["admins"] = getAdminsFilePath()
				// A directory at a file path causes a deterministic IO error,
				// including when tests run with root permissions.
				writeTestFile(t, filepath.Join(paths[field], "keep.txt"), "keep")
				var err error
				if operation == "capture" {
					err = CreateBackup("unreadable")
				} else {
					_, err = RestoreBackup("snapshot")
				}
				if err == nil {
					t.Fatal("file error was reported as success")
				}
				if !strings.Contains(err.Error(), paths[field]) {
					t.Fatalf("error does not identify the failed file: %v", err)
				}
				if !bytes.Equal(before, mustReadPluginFile(t, getBackupPath())) {
					t.Fatal("failed operation changed the saved backup")
				}
				if got := string(mustReadPluginFile(t, filepath.Join(paths[field], "keep.txt"))); got != "keep" {
					t.Fatalf("failed operation changed existing data: %q", got)
				}
			})
		}
	}
}
