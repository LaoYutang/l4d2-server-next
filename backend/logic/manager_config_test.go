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
