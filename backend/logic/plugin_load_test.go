package logic

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type loadBatchFixture struct {
	states   map[string]pluginRuntimeState
	failLoad map[string]bool
	commands *[]string
	override func(action, filename string) (string, error, bool)
}

func newLoadBatchFixture(t *testing.T, initial map[string]pluginRuntimeState) *loadBatchFixture {
	t.Helper()
	f := &loadBatchFixture{states: map[string]pluginRuntimeState{}, failLoad: map[string]bool{}}
	for filename, state := range initial {
		f.states[filename] = state
	}
	f.commands = mockPluginRconExecutor(t, func(cmd string) (string, error) {
		parts := strings.SplitN(strings.TrimPrefix(cmd, "sm plugins "), " ", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("unexpected command %q", cmd)
		}
		filename, err := strconv.Unquote(parts[1])
		if err != nil {
			return "", err
		}
		if f.override != nil {
			if output, err, handled := f.override(parts[0], filename); handled {
				return output, err
			}
		}
		switch parts[0] {
		case "info":
			switch f.states[filename] {
			case pluginRuntimeRunning:
				return fmt.Sprintf("  Filename: %s\n  Status: running\n", filename), nil
			case pluginRuntimeNonrunning:
				return fmt.Sprintf("  Filename: %s\n  Error: initialization failed\n", filename), nil
			default:
				return fmt.Sprintf("[SM] Plugin %s is not loaded.\n", filename), nil
			}
		case "load":
			if f.failLoad[filename] {
				return fmt.Sprintf("[SM] Plugin %s failed to load: test failure.", filename), nil
			}
			if f.states[filename] == pluginRuntimeRunning {
				return fmt.Sprintf("[SM] Plugin %s is already loaded.", filename), nil
			}
			f.states[filename] = pluginRuntimeRunning
			return fmt.Sprintf("[SM] Loaded plugin %s successfully.", filename), nil
		case "unload":
			f.states[filename] = pluginRuntimeAbsent
			return "[SM] Plugin Display Name unloaded successfully.", nil
		}
		return "", fmt.Errorf("unexpected action %q", parts[0])
	})
	return f
}

func prepareLoadBatch(t *testing.T, filenames ...string) string {
	t.Helper()
	storePath, gamePath := setupPluginTestPaths(t)
	writeTestFile(t, filepath.Join(storePath, ConfigFileName), "enabled_plugins: []\n")
	for _, filename := range filenames {
		writeTestFile(t, filepath.Join(storePath, "Batch", "left4dead2", "addons", "sourcemod", "plugins", filepath.FromSlash(filename)), "smx")
	}
	return gamePath
}

func assertBatchDeployment(t *testing.T, gamePath, filename string, enabled bool) {
	t.Helper()
	got, err := isPluginEnabled("Batch")
	if err != nil || got != enabled {
		t.Fatalf("enabled = %v, err = %v, want %v", got, err, enabled)
	}
	_, err = os.Stat(filepath.Join(gamePath, "addons", "sourcemod", "plugins", filepath.FromSlash(filename)))
	if enabled && err != nil || !enabled && !os.IsNotExist(err) {
		t.Fatalf("deployment %s stat = %v, enabled = %v", filename, err, enabled)
	}
}

func TestEnableAndLoadIgnoresMapErrorAndPreservesFullFilenames(t *testing.T) {
	gamePath := prepareLoadBatch(t, "123.smx", "a/shared.smx", "b/shared.smx", "Case/Upper.SMX")
	f := newLoadBatchFixture(t, nil)
	f.override = func(action, filename string) (string, error, bool) {
		if action == "load" {
			f.states[filename] = pluginRuntimeRunning
			return fmt.Sprintf("Error loading map unrelated.bsp\n[SM] Loaded plugin %s successfully.\nCould not load map resource", filename), nil, true
		}
		return "", nil, false
	}
	if err := EnableAndLoadPlugin("Batch"); err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{"123.smx", "a/shared.smx", "b/shared.smx", "Case/Upper.SMX"} {
		assertBatchDeployment(t, gamePath, filename, true)
		if !strings.Contains(strings.Join(*f.commands, "\n"), `sm plugins load "`+filename+`"`) {
			t.Fatalf("full filename %q missing from %v", filename, *f.commands)
		}
	}
}

func TestEnableAndLoadPreservesOriginalInstancesOnBatchFailure(t *testing.T) {
	for _, initial := range []pluginRuntimeState{pluginRuntimeRunning, pluginRuntimeNonrunning} {
		t.Run(fmt.Sprint(initial), func(t *testing.T) {
			gamePath := prepareLoadBatch(t, "a.smx", "b.smx")
			f := newLoadBatchFixture(t, map[string]pluginRuntimeState{"a.smx": initial})
			f.failLoad["b.smx"] = true
			err := EnableAndLoadPlugin("Batch")
			if err == nil || errors.Is(err, ErrPluginResultUnknown) {
				t.Fatalf("expected known b.smx failure, got %v", err)
			}
			assertBatchDeployment(t, gamePath, "a.smx", true)
			if f.states["a.smx"] != pluginRuntimeRunning || strings.Contains(strings.Join(*f.commands, "\n"), "sm plugins unload") {
				t.Fatalf("original instance was rolled back: %v", *f.commands)
			}
		})
	}
}

func TestEnableAndLoadUnknownRetainsDeploymentAndPriorNewInstance(t *testing.T) {
	for _, mode := range []string{"empty", "timeout", "contradictory", "already-loaded-race"} {
		t.Run(mode, func(t *testing.T) {
			gamePath := prepareLoadBatch(t, "a.smx", "b.smx")
			f := newLoadBatchFixture(t, nil)
			f.override = func(action, filename string) (string, error, bool) {
				if action != "load" || filename != "b.smx" {
					return "", nil, false
				}
				switch mode {
				case "timeout":
					return "", errors.New("timeout after command write"), true
				case "contradictory":
					f.states[filename] = pluginRuntimeRunning
					return "[SM] Loaded plugin b.smx successfully.\n[SM] Plugin b.smx failed to load: failure.", nil, true
				case "already-loaded-race":
					f.states[filename] = pluginRuntimeRunning
					return "[SM] Plugin b.smx is already loaded.", nil, true
				default:
					return "", nil, true
				}
			}
			if err := EnableAndLoadPlugin("Batch"); !errors.Is(err, ErrPluginResultUnknown) {
				t.Fatalf("want unknown, got %v", err)
			}
			assertBatchDeployment(t, gamePath, "a.smx", true)
			if f.states["a.smx"] != pluginRuntimeRunning || strings.Contains(strings.Join(*f.commands, "\n"), "sm plugins unload") {
				t.Fatalf("unknown result triggered destructive rollback: %v", *f.commands)
			}
		})
	}
}

func TestEnableAndLoadIncludesCurrentNewErrorInstanceInRollback(t *testing.T) {
	gamePath := prepareLoadBatch(t, "current.smx")
	f := newLoadBatchFixture(t, nil)
	f.override = func(action, filename string) (string, error, bool) {
		if action == "load" {
			f.states[filename] = pluginRuntimeNonrunning
			return "[SM] Loaded plugin current.smx successfully.", nil, true
		}
		return "", nil, false
	}
	if err := EnableAndLoadPlugin("Batch"); err == nil || errors.Is(err, ErrPluginResultUnknown) {
		t.Fatalf("want known current instance failure, got %v", err)
	}
	assertBatchDeployment(t, gamePath, "current.smx", false)
	if f.states["current.smx"] != pluginRuntimeAbsent || !strings.Contains(strings.Join(*f.commands, "\n"), `unload "current.smx"`) {
		t.Fatalf("current new instance not cleaned up: %v", *f.commands)
	}
}

func TestEnableAndLoadWaitsForRollbackAndRetainsOnUnconfirmedUnload(t *testing.T) {
	for _, delayed := range []bool{false, true} {
		t.Run(fmt.Sprint(delayed), func(t *testing.T) {
			gamePath := prepareLoadBatch(t, "a.smx", "b.smx")
			f := newLoadBatchFixture(t, nil)
			f.failLoad["b.smx"] = true
			unloads, afterUnloadQueries := 0, 0
			f.override = func(action, filename string) (string, error, bool) {
				if filename != "a.smx" {
					return "", nil, false
				}
				if action == "unload" {
					unloads++
					return "[SM] Plugin Display Name will be unloaded on the next frame.", nil, true
				}
				if action == "info" && unloads > 0 {
					afterUnloadQueries++
					if delayed && afterUnloadQueries >= 2 {
						f.states[filename] = pluginRuntimeAbsent
					}
				}
				return "", nil, false
			}
			err := EnableAndLoadPlugin("Batch")
			if err == nil || errors.Is(err, ErrPluginResultUnknown) == delayed || unloads != 1 {
				t.Fatalf("err=%v, unloads=%d, delayed=%v", err, unloads, delayed)
			}
			assertBatchDeployment(t, gamePath, "a.smx", !delayed)
		})
	}
}

func TestEnableAndLoadUnconfirmedPreflightDoesNotDeployOrLoad(t *testing.T) {
	gamePath := prepareLoadBatch(t, "a.smx")
	f := newLoadBatchFixture(t, nil)
	f.override = func(action, filename string) (string, error, bool) {
		return "Error loading unrelated map", nil, true
	}
	if err := EnableAndLoadPlugin("Batch"); !errors.Is(err, ErrPluginResultUnknown) {
		t.Fatalf("want unknown, got %v", err)
	}
	assertBatchDeployment(t, gamePath, "a.smx", false)
	if len(*f.commands) != 1 || !strings.Contains((*f.commands)[0], "sm plugins info") {
		t.Fatalf("unexpected mutations: %v", *f.commands)
	}
}

func TestEnableAndLoadRejectsAlreadyEnabledWithoutRCON(t *testing.T) {
	prepareLoadBatch(t, "a.smx")
	if err := EnablePlugin("Batch"); err != nil {
		t.Fatal(err)
	}
	f := newLoadBatchFixture(t, nil)
	if err := EnableAndLoadPlugin("Batch"); err == nil || !strings.Contains(err.Error(), "already enabled") {
		t.Fatalf("want existing enabled rejection, got %v", err)
	}
	if len(*f.commands) != 0 {
		t.Fatalf("already-enabled request sent RCON: %v", *f.commands)
	}
}

func TestEnableAndLoadRetainsDeploymentForUnownedFailedObject(t *testing.T) {
	gamePath := prepareLoadBatch(t, "a.smx")
	f := newLoadBatchFixture(t, nil)
	f.override = func(action, filename string) (string, error, bool) {
		if action == "load" {
			f.states[filename] = pluginRuntimeNonrunning
			return "[SM] Plugin a.smx failed to load: test failure.", nil, true
		}
		return "", nil, false
	}
	if err := EnableAndLoadPlugin("Batch"); !errors.Is(err, ErrPluginResultUnknown) {
		t.Fatalf("want unconfirmed cleanup, got %v", err)
	}
	assertBatchDeployment(t, gamePath, "a.smx", true)
	if strings.Contains(strings.Join(*f.commands, "\n"), "sm plugins unload") {
		t.Fatalf("unowned instance was unloaded: %v", *f.commands)
	}
}

func TestPluginRuntimeReservationRejectsConcurrentMutations(t *testing.T) {
	prepareLoadBatch(t, "a.smx")
	if err := EnablePlugin("Batch"); err != nil {
		t.Fatal(err)
	}
	f := newLoadBatchFixture(t, nil)
	started, resume, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	first := true
	f.override = func(action, filename string) (string, error, bool) {
		if first {
			first = false
			close(started)
			<-resume
		}
		return "", nil, false
	}
	go func() { done <- LoadPlugin("Batch") }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("load did not reach preflight")
	}
	mutations := []func(string) error{EnablePlugin, DisablePlugin, LoadPlugin, EnableAndLoadPlugin, ReloadPlugin, UnloadPlugin, DisableAndUnloadPlugin, DeletePlugin}
	for _, mutate := range mutations {
		if err := mutate("Batch"); !errors.Is(err, ErrPluginOperationBusy) {
			close(resume)
			<-done
			t.Fatalf("concurrent mutation was not rejected: %v", err)
		}
	}
	close(resume)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if pluginMutationBusy.Load() {
		t.Fatal("reservation leaked")
	}
}
