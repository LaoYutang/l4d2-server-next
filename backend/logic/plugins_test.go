package logic

import (
	"l4d2-manager-next/consts"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestGetPluginsHasSMX(t *testing.T) {
	storePath, _ := setupPluginTestPaths(t)

	writeTestFile(t, filepath.Join(storePath, ConfigFileName), "enabled_plugins: []\n")
	writeTestFile(t, filepath.Join(storePath, "NoSMX", "left4dead2", "cfg", "server.cfg"), "cfg")
	writeTestFile(t, filepath.Join(storePath, "HasSMX", "left4dead2", "addons", "sourcemod", "plugins", "has.smx"), "smx")
	writeTestFile(t, filepath.Join(storePath, "OnlyDisabledSMX", "left4dead2", "addons", "sourcemod", "plugins", "disabled", "off.smx"), "smx")

	plugins, err := GetPlugins()
	if err != nil {
		t.Fatalf("GetPlugins() error = %v", err)
	}

	byName := make(map[string]Plugin)
	for _, plugin := range plugins {
		byName[plugin.Name] = plugin
	}

	if byName["NoSMX"].HasSMX {
		t.Fatalf("NoSMX HasSMX = true, want false")
	}
	if !byName["HasSMX"].HasSMX {
		t.Fatalf("HasSMX HasSMX = false, want true")
	}
	if byName["OnlyDisabledSMX"].HasSMX {
		t.Fatalf("OnlyDisabledSMX HasSMX = true, want false")
	}
}

func TestListPluginSMXIDsSortedAndExcludesDisabled(t *testing.T) {
	storePath, _ := setupPluginTestPaths(t)

	base := filepath.Join(storePath, "PluginHot", "left4dead2", "addons", "sourcemod", "plugins")
	writeTestFile(t, filepath.Join(base, "zeta.smx"), "smx")
	writeTestFile(t, filepath.Join(base, "alpha.smx"), "smx")
	writeTestFile(t, filepath.Join(base, "nested", "beta.smx"), "smx")
	writeTestFile(t, filepath.Join(base, "disabled", "ignored.smx"), "smx")

	got, err := listPluginSMXIDs("PluginHot")
	if err != nil {
		t.Fatalf("listPluginSMXIDs() error = %v", err)
	}

	want := []string{"alpha", "nested/beta", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listPluginSMXIDs() = %v, want %v", got, want)
	}
}

func TestEnableAndLoadPluginRollsBackOnLoadFailure(t *testing.T) {
	storePath, gamePath := setupPluginTestPaths(t)

	writeTestFile(t, filepath.Join(storePath, ConfigFileName), "enabled_plugins: []\n")
	writeTestFile(t, filepath.Join(storePath, "PluginHot", "left4dead2", "addons", "sourcemod", "plugins", "hot.smx"), "smx")
	writeTestFile(t, filepath.Join(storePath, "PluginHot", "left4dead2", "cfg", "sourcemod", "hot.cfg"), "cfg")

	commands := mockPluginRconExecutor(t, func(cmd string) (string, error) {
		return "[SM] Failed to load plugin", nil
	})

	err := EnableAndLoadPlugin("PluginHot")
	if err == nil || !strings.Contains(err.Error(), "load smx plugin hot failed") {
		t.Fatalf("EnableAndLoadPlugin() error = %v, want load failure", err)
	}

	if got, want := *commands, []string{`sm plugins load "hot"`}; !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
	if _, err := os.Stat(filepath.Join(gamePath, "addons", "sourcemod", "plugins", "hot.smx")); !os.IsNotExist(err) {
		t.Fatalf("hot.smx still exists after rollback: %v", err)
	}

	plugins, err := GetPlugins()
	if err != nil {
		t.Fatalf("GetPlugins() error = %v", err)
	}
	for _, plugin := range plugins {
		if plugin.Name == "PluginHot" && plugin.Status != "disabled" {
			t.Fatalf("PluginHot status = %s, want disabled", plugin.Status)
		}
	}
}

func TestDisableAndUnloadPluginRollsBackOnUnloadFailure(t *testing.T) {
	storePath, gamePath := setupPluginTestPaths(t)

	writeTestFile(t, filepath.Join(storePath, ConfigFileName), "enabled_plugins: []\n")
	writeTestFile(t, filepath.Join(storePath, "PluginHot", "left4dead2", "addons", "sourcemod", "plugins", "alpha.smx"), "smx")
	writeTestFile(t, filepath.Join(storePath, "PluginHot", "left4dead2", "addons", "sourcemod", "plugins", "nested", "beta.smx"), "smx")

	if err := EnablePlugin("PluginHot"); err != nil {
		t.Fatalf("EnablePlugin() error = %v", err)
	}

	commands := mockPluginRconExecutor(t, func(cmd string) (string, error) {
		if strings.Contains(cmd, `unload "alpha"`) {
			return "[SM] Failed to unload plugin", nil
		}
		return "[SM] OK", nil
	})

	err := DisableAndUnloadPlugin("PluginHot")
	if err == nil || !strings.Contains(err.Error(), "unload smx plugin alpha failed") {
		t.Fatalf("DisableAndUnloadPlugin() error = %v, want unload failure", err)
	}

	wantCommands := []string{
		`sm plugins unload "nested/beta"`,
		`sm plugins unload "alpha"`,
		`sm plugins load "nested/beta"`,
	}
	if !reflect.DeepEqual(*commands, wantCommands) {
		t.Fatalf("commands = %v, want %v", *commands, wantCommands)
	}
	if _, err := os.Stat(filepath.Join(gamePath, "addons", "sourcemod", "plugins", "alpha.smx")); err != nil {
		t.Fatalf("alpha.smx missing after rollback: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gamePath, "addons", "sourcemod", "plugins", "nested", "beta.smx")); err != nil {
		t.Fatalf("nested/beta.smx missing after rollback: %v", err)
	}

	plugins, err := GetPlugins()
	if err != nil {
		t.Fatalf("GetPlugins() error = %v", err)
	}
	for _, plugin := range plugins {
		if plugin.Name == "PluginHot" && plugin.Status != "enabled" {
			t.Fatalf("PluginHot status = %s, want enabled", plugin.Status)
		}
	}
}

func setupPluginTestPaths(t *testing.T) (string, string) {
	t.Helper()

	storePath := t.TempDir()
	gamePath := t.TempDir()
	t.Setenv(PluginStorePathEnv, storePath)

	pluginMutex.Lock()
	oldConfigViper := configViper
	oldFileRefs := fileRefs
	configViper = viper.New()
	configViper.SetConfigType("yaml")
	fileRefs = nil
	pluginMutex.Unlock()

	oldGamePath := consts.GamePath
	consts.GamePath = gamePath

	t.Cleanup(func() {
		pluginMutex.Lock()
		configViper = oldConfigViper
		fileRefs = oldFileRefs
		pluginMutex.Unlock()
		consts.GamePath = oldGamePath
	})

	return storePath, gamePath
}

func mockPluginRconExecutor(t *testing.T, executor func(cmd string) (string, error)) *[]string {
	t.Helper()

	oldExecutor := executePluginRconCommand
	commands := []string{}
	executePluginRconCommand = func(cmd string) (string, error) {
		commands = append(commands, cmd)
		return executor(cmd)
	}
	t.Cleanup(func() {
		executePluginRconCommand = oldExecutor
	})

	return &commands
}
