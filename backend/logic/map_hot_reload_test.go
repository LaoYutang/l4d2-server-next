package logic

import "testing"

func TestExecuteMapHotReloadCommandUsesSavedCommand(t *testing.T) {
	setupManagerConfigTest(t)

	const command = "sm_update_vpk"
	if _, err := SetMapHotReloadCommand(command); err != nil {
		t.Fatalf("SetMapHotReloadCommand() error = %v", err)
	}

	oldExecutor := executeMapHotReloadRconCommand
	var gotCommand string
	executeMapHotReloadRconCommand = func(cmd string) (string, error) {
		gotCommand = cmd
		return "ok", nil
	}
	t.Cleanup(func() {
		executeMapHotReloadRconCommand = oldExecutor
	})

	if _, err := ExecuteMapHotReloadCommand(); err != nil {
		t.Fatalf("ExecuteMapHotReloadCommand() error = %v", err)
	}
	if gotCommand != command {
		t.Fatalf("RCON command = %q, want %q", gotCommand, command)
	}
}
