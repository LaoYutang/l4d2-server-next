package logic

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNormalizeServerCustomConfig(t *testing.T) {
	lines := []string{
		"// 第一行注释",
		"// 第二行注释",
		`sm_cvar welcome "http://example.com/a//b" // 行尾注释`,
		`sm_cvar endpoint http://example.com/path`,
		`sm_cvar quoted "value // literal"`,
		"",
	}

	normalized, err := NormalizeServerCustomConfig(lines)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"// 第一行注释",
		"// 第二行注释",
		"// 行尾注释",
		`sm_cvar welcome "http://example.com/a//b"`,
		`sm_cvar endpoint http://example.com/path`,
		`sm_cvar quoted "value // literal"`,
	}
	if strings.Join(normalized, "\n") != strings.Join(want, "\n") {
		t.Fatalf("normalized config:\n%s\nwant:\n%s", strings.Join(normalized, "\n"), strings.Join(want, "\n"))
	}
}

func TestNormalizeServerCustomConfigRejectsTrailingComments(t *testing.T) {
	_, err := NormalizeServerCustomConfig([]string{
		"sm_cvar value 1",
		"// 没有对应指令",
		"// 第二行",
	})
	if !errors.Is(err, ErrUnboundServerConfigComment) {
		t.Fatalf("error = %v; want ErrUnboundServerConfigComment", err)
	}
	if !strings.Contains(err.Error(), "第 2 行") || !strings.Contains(err.Error(), "2 行") {
		t.Fatalf("error lacks location: %v", err)
	}
}

func TestExtractRedactedServerFixedConfig(t *testing.T) {
	content := strings.Join([]string{
		"// tick config",
		"sm_cvar sv_minrate 100000",
		`rcon_password "very-secret" // 管理密码`,
		`SV_PASSWORD join-secret`,
		`tv_password "watch-secret"`,
		`SM_CVAR plugin_password "plugin-secret"`,
		GameBanConfigMarker,
		"exec banned_user.cfg",
		"exec banned_ip.cfg",
		`sv_tags "hidden"`,
		`sm_cvar sv_allow_lobby_connect_only "1"`,
		`sv_steamgroup "123"`,
		ServerCustomConfigMarker,
		"sm_cvar custom_value 1",
	}, "\n")

	fixed := ExtractRedactedServerFixedConfig(content)
	for _, secret := range []string{"very-secret", "join-secret", "watch-secret", "plugin-secret"} {
		if strings.Contains(fixed, secret) {
			t.Fatalf("fixed config leaked %q:\n%s", secret, fixed)
		}
	}
	for _, expected := range []string{
		"sm_cvar sv_minrate 100000",
		`rcon_password "********" // 管理密码`,
		`SV_PASSWORD "********"`,
		`tv_password "********"`,
		`SM_CVAR plugin_password "********"`,
		GameBanConfigMarker,
		"exec banned_user.cfg",
	} {
		if !strings.Contains(fixed, expected) {
			t.Fatalf("fixed config missing %q:\n%s", expected, fixed)
		}
	}
	for _, excluded := range []string{"sv_tags", "sv_allow_lobby_connect_only", "sv_steamgroup", "custom_value"} {
		if strings.Contains(fixed, excluded) {
			t.Fatalf("fixed config contains managed/custom value %q:\n%s", excluded, fixed)
		}
	}
}

func TestUpdateServerConfigFilePreservesFixedContentAndMode(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "server.cfg.60tick")
	original := strings.Join([]string{
		"// 60 tick",
		"sm_cvar sv_minrate 60000",
		`sv_tags "coop,hidden"`,
		ServerCustomConfigMarker,
		"sm_cvar old 1",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	err := UpdateServerConfigFile(configPath, ServerConfigSettings{
		Hidden:           false,
		LobbyConnectOnly: true,
		SteamGroup:       "123",
		CustomConfig: []string{
			"// 说明",
			"sm_cvar changed 1 // 行尾",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	updatedBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := string(updatedBytes)
	for _, expected := range []string{
		"// 60 tick",
		"sm_cvar sv_minrate 60000",
		`sv_tags "coop"`,
		`sm_cvar sv_allow_lobby_connect_only "1"`,
		`sv_steamgroup "123"`,
		"// 说明\n// 行尾\nsm_cvar changed 1",
	} {
		if !strings.Contains(updated, expected) {
			t.Fatalf("updated config missing %q:\n%s", expected, updated)
		}
	}
	if strings.Contains(updated, "sm_cvar old 1") {
		t.Fatalf("old custom config was preserved:\n%s", updated)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("mode = %o; want 600", info.Mode().Perm())
		}
	}
}

func TestApplyServerConfigToFileAcceptsLegacyAndCommentedBackups(t *testing.T) {
	tests := []struct {
		name   string
		custom []string
		want   []string
	}{
		{
			name:   "legacy command-only backup",
			custom: []string{"sm_cvar legacy_value 1"},
			want:   []string{"sm_cvar legacy_value 1"},
		},
		{
			name: "commented backup",
			custom: []string{
				"// 第一行",
				`sm_cvar endpoint "https://example.com/a//b" // 行尾`,
			},
			want: []string{
				"// 第一行",
				"// 行尾",
				`sm_cvar endpoint "https://example.com/a//b"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "server.cfg")
			original := "// active tick\nsm_cvar sv_minrate 100000\n"
			if err := os.WriteFile(configPath, []byte(original), 0644); err != nil {
				t.Fatal(err)
			}

			err := applyServerConfigToFile(configPath, &BackupServerConfig{CustomConfig: tt.custom})
			if err != nil {
				t.Fatal(err)
			}
			updated, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(updated), "sm_cvar sv_minrate 100000") {
				t.Fatalf("fixed tick config was not preserved:\n%s", updated)
			}
			got := ExtractServerCustomConfig(string(updated))
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Fatalf("custom config = %#v; want %#v", got, tt.want)
			}
		})
	}
}
