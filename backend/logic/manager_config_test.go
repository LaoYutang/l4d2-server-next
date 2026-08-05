package logic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"l4d2-manager-next/consts"
)

func setupManagerConfigTest(t *testing.T) {
	t.Helper()

	oldManagerDataPath := consts.ManagerDataPath
	oldManagerConfigPath := consts.ManagerConfigPath

	consts.ManagerDataPath = filepath.Join(t.TempDir(), "data")
	consts.ManagerConfigPath = filepath.Join(consts.ManagerDataPath, "manager_config.json")
	LoadManagerConfig()

	t.Cleanup(func() {
		consts.ManagerDataPath = oldManagerDataPath
		consts.ManagerConfigPath = oldManagerConfigPath
		LoadManagerConfig()
	})
}

func TestVPKTrimDefaultsEnabledWhenConfigMissingOrLegacy(t *testing.T) {
	setupManagerConfigTest(t)

	if !IsVPKTrimEnabled() {
		t.Fatal("VPK trim disabled with missing config, want enabled by default")
	}

	if err := consts.EnsureManagerDataPath(); err != nil {
		t.Fatalf("create manager data path: %v", err)
	}
	legacyConfig := `{"enable_self_service":true,"enable_player_stats":false}`
	if err := os.WriteFile(consts.ManagerConfigPath, []byte(legacyConfig), 0644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	LoadManagerConfig()
	if !IsVPKTrimEnabled() {
		t.Fatal("VPK trim disabled with legacy config, want enabled by default")
	}
}

func TestMapHotReloadCommandDefaultsWhenConfigMissingOrLegacy(t *testing.T) {
	setupManagerConfigTest(t)

	if got := GetMapHotReloadCommand(); got != DefaultMapHotReloadCommand {
		t.Fatalf("default command = %q, want %q", got, DefaultMapHotReloadCommand)
	}

	if err := consts.EnsureManagerDataPath(); err != nil {
		t.Fatalf("create manager data path: %v", err)
	}
	legacyConfig := `{"enable_self_service":true,"enable_player_stats":false}`
	if err := os.WriteFile(consts.ManagerConfigPath, []byte(legacyConfig), 0644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	LoadManagerConfig()
	if got := GetMapHotReloadCommand(); got != DefaultMapHotReloadCommand {
		t.Fatalf("legacy config command = %q, want %q", got, DefaultMapHotReloadCommand)
	}
}

func TestSetMapHotReloadCommandSavesAndLoads(t *testing.T) {
	setupManagerConfigTest(t)

	const command = `sm_update_vpk; say "maps reloaded"`
	got, err := SetMapHotReloadCommand("  " + command + "  ")
	if err != nil {
		t.Fatalf("SetMapHotReloadCommand() error = %v", err)
	}
	if got != command {
		t.Fatalf("normalized command = %q, want %q", got, command)
	}

	LoadManagerConfig()
	if got := GetMapHotReloadCommand(); got != command {
		t.Fatalf("loaded command = %q, want %q", got, command)
	}
	if IsMapHotReloadCommandDefault() {
		t.Fatalf("IsMapHotReloadCommandDefault() = true, want false for custom command")
	}
}

func TestSetMapHotReloadCommandRestoresDefaultWhenEmpty(t *testing.T) {
	setupManagerConfigTest(t)

	got, err := SetMapHotReloadCommand("   ")
	if err != nil {
		t.Fatalf("SetMapHotReloadCommand(empty) error = %v", err)
	}
	if got != DefaultMapHotReloadCommand {
		t.Fatalf("empty command = %q, want default %q", got, DefaultMapHotReloadCommand)
	}
	if !IsMapHotReloadCommandDefault() {
		t.Fatalf("IsMapHotReloadCommandDefault() = false, want true after empty restore")
	}
}

func TestNormalizeMapHotReloadCommandRejectsInvalidCommands(t *testing.T) {
	cases := []string{
		"mission_reload\nstatus",
		"mission_reload\rstatus",
		"mission_reload\x00status",
		strings.Repeat("a", 513),
	}

	for _, tc := range cases {
		if got, err := NormalizeMapHotReloadCommand(tc); err == nil {
			t.Fatalf("NormalizeMapHotReloadCommand(%q) = %q, want error", tc, got)
		}
	}
}

func TestSteamCDNIPDefaultsEmptyWhenConfigMissingOrLegacy(t *testing.T) {
	setupManagerConfigTest(t)

	if got := GetSteamCDNIP(); got != "" {
		t.Fatalf("default Steam CDN IP = %q, want empty", got)
	}

	if err := consts.EnsureManagerDataPath(); err != nil {
		t.Fatalf("create manager data path: %v", err)
	}
	legacyConfig := `{"enable_self_service":true,"enable_player_stats":false}`
	if err := os.WriteFile(consts.ManagerConfigPath, []byte(legacyConfig), 0644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	LoadManagerConfig()
	if got := GetSteamCDNIP(); got != "" {
		t.Fatalf("legacy config Steam CDN IP = %q, want empty", got)
	}
}

func TestSetSteamCDNIPSavesNormalizesAndClears(t *testing.T) {
	setupManagerConfigTest(t)

	got, err := SetSteamCDNIP(" 192.0.2.10 ")
	if err != nil {
		t.Fatalf("SetSteamCDNIP(IPv4) error = %v", err)
	}
	if got != "192.0.2.10" {
		t.Fatalf("normalized IPv4 = %q, want 192.0.2.10", got)
	}

	LoadManagerConfig()
	if got := GetSteamCDNIP(); got != "192.0.2.10" {
		t.Fatalf("loaded IPv4 = %q, want 192.0.2.10", got)
	}

	got, err = SetSteamCDNIP("2001:0db8:0:0:0:0:0:1")
	if err != nil {
		t.Fatalf("SetSteamCDNIP(IPv6) error = %v", err)
	}
	if got != "2001:db8::1" {
		t.Fatalf("normalized IPv6 = %q, want 2001:db8::1", got)
	}

	got, err = SetSteamCDNIP("   ")
	if err != nil {
		t.Fatalf("SetSteamCDNIP(empty) error = %v", err)
	}
	if got != "" || GetSteamCDNIP() != "" {
		t.Fatalf("cleared Steam CDN IP = %q / %q, want empty", got, GetSteamCDNIP())
	}
}

func TestNormalizeSteamCDNIPRejectsNonIPValues(t *testing.T) {
	for _, value := range []string{
		"cdn.steamusercontent.com",
		"192.0.2.999",
		"fe80::1%eth0",
	} {
		if got, err := NormalizeSteamCDNIP(value); err == nil {
			t.Fatalf("NormalizeSteamCDNIP(%q) = %q, want error", value, got)
		}
	}
}

func TestLoadManagerConfigIgnoresInvalidSteamCDNIP(t *testing.T) {
	setupManagerConfigTest(t)

	if err := consts.EnsureManagerDataPath(); err != nil {
		t.Fatalf("create manager data path: %v", err)
	}
	config := `{"steam_cdn_ip":"not-an-ip"}`
	if err := os.WriteFile(consts.ManagerConfigPath, []byte(config), 0644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	LoadManagerConfig()
	if got := GetSteamCDNIP(); got != "" {
		t.Fatalf("invalid persisted Steam CDN IP = %q, want empty", got)
	}
}
