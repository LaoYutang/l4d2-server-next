package logic

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestSourceModRuntimeInfo(t *testing.T) {
	const filename = "Nested/123.smx"
	running := sourceModRuntimeInfo(filename, "  Status: running")
	absent := sourceModRuntimeAbsent(filename)
	tests := []struct {
		name   string
		output string
		want   pluginRuntimeState
	}{
		{"running", running, pluginRuntimeRunning},
		{"absent", absent, pluginRuntimeAbsent},
		{"empty error is nonrunning", sourceModRuntimeInfo(filename, "  Error: "), pluginRuntimeNonrunning},
		{"error", sourceModRuntimeInfo(filename, "  Error: Missing native"), pluginRuntimeNonrunning},
		{"load error", sourceModRuntimeInfo(filename, "  Load error: Unable to open file"), pluginRuntimeNonrunning},
		{"engine error after running", running + "\nError: maps/missing.bsp could not be found", pluginRuntimeRunning},
		{"title contains error", sourceModRuntimeInfo(filename, "  Title: Error: Maps", "  Status: running"), pluginRuntimeRunning},
		{"other blocks", sourceModRuntimeInfo("Other/123.smx", "  Error: ignored") + "\n" + running + "\n" + sourceModRuntimeInfo("Other.smx", "  Error: ignored"), pluginRuntimeRunning},
		{"CRLF", strings.ReplaceAll(running, "\n", "\r\n"), pluginRuntimeRunning},
		{"basename does not match", sourceModRuntimeInfo("123.smx", "  Status: running"), pluginRuntimeUnknown},
		{"case does not match", sourceModRuntimeInfo("nested/123.smx", "  Status: running"), pluginRuntimeUnknown},
		{"suffix does not match", sourceModRuntimeInfo("Nested/123.smx.smx", "  Status: running"), pluginRuntimeUnknown},
		{"empty", "", pluginRuntimeUnknown},
		{"noise only", "Error: maps/missing.bsp", pluginRuntimeUnknown},
		{"missing status", sourceModRuntimeInfo(filename, "  Title: Plugin"), pluginRuntimeUnknown},
		{"unknown status", sourceModRuntimeInfo(filename, "  Status: paused"), pluginRuntimeUnknown},
		{"duplicate status", running + "\n  Status: running", pluginRuntimeUnknown},
		{"duplicate error", sourceModRuntimeInfo(filename, "  Error:", "  Error:"), pluginRuntimeUnknown},
		{"conflicting status", running + "\n  Error: Failed", pluginRuntimeUnknown},
		{"two target blocks", running + "\n" + running, pluginRuntimeUnknown},
		{"absent and present", absent + "\n" + running, pluginRuntimeUnknown},
		{"repeated absent", absent + "\n" + absent, pluginRuntimeUnknown},
		{"unindented fields", "Filename: " + filename + "\nStatus: running", pluginRuntimeUnknown},
		{"malformed next header ends target block", sourceModRuntimeInfo(filename) + "\n  Filename:Other.smx\n  Status: running", pluginRuntimeUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseSourceModPluginInfo(test.output, filename)
			if got != test.want || errors.Is(err, ErrPluginResultUnknown) != (test.want == pluginRuntimeUnknown) {
				t.Fatalf("state = %v, error = %v; want %v", got, err, test.want)
			}
		})
	}
}

func TestSourceModRuntimeLoadReply(t *testing.T) {
	const filename = "Nested/123.smx"
	success := sourceModRuntimeLoaded(filename)
	failed := sourceModRuntimeFailed(filename)
	tests := []struct {
		name   string
		output string
		want   sourceModLoadReply
	}{
		{"loaded", success, sourceModLoadReplyLoaded},
		{"already", sourceModRuntimeAlreadyLoaded(filename), sourceModLoadReplyAlreadyLoaded},
		{"failure", failed, sourceModLoadReplyFailed},
		{"map noise", "Error: maps/missing.bsp failed\n" + success, sourceModLoadReplyLoaded},
		{"other plugin failure", sourceModRuntimeFailed("Other.smx") + "\n" + success, sourceModLoadReplyLoaded},
		{"other plugin only", sourceModRuntimeLoaded("Other.smx"), sourceModLoadReplyUnknown},
		{"basename", sourceModRuntimeLoaded("123.smx"), sourceModLoadReplyUnknown},
		{"case", sourceModRuntimeLoaded("nested/123.smx"), sourceModLoadReplyUnknown},
		{"missing period", strings.TrimSuffix(success, "."), sourceModLoadReplyUnknown},
		{"empty", "", sourceModLoadReplyUnknown},
		{"duplicate success", success + "\n" + success, sourceModLoadReplyUnknown},
		{"conflicting replies", success + "\n" + failed, sourceModLoadReplyUnknown},
		{"success and already", success + "\n" + sourceModRuntimeAlreadyLoaded(filename), sourceModLoadReplyUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := parseSourceModPluginLoadReply(test.output, filename)
			if got != test.want {
				t.Fatalf("reply = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSourceModRuntimeLoad(t *testing.T) {
	const filename = "Nested/123.smx"
	running := sourceModRuntimeInfo(filename, "  Status: running")
	nonrunning := sourceModRuntimeInfo(filename, "  Error:")
	absent := sourceModRuntimeAbsent(filename)
	timeout := errors.New("RCON timeout")
	tests := []struct {
		name       string
		before     pluginRuntimeState
		reply      string
		commandErr error
		queries    []pluginRuntimeTestReply
		owned      bool
		wantErr    bool
		unknown    bool
	}{
		{name: "success ignores map errors", before: pluginRuntimeAbsent, reply: "Error: maps/a.bsp failed\n" + sourceModRuntimeLoaded(filename), queries: []pluginRuntimeTestReply{{output: running}}, owned: true},
		{name: "running original is not owned", before: pluginRuntimeRunning, reply: sourceModRuntimeLoaded(filename), queries: []pluginRuntimeTestReply{{output: running}}},
		{name: "replaced failed original is not owned", before: pluginRuntimeNonrunning, reply: sourceModRuntimeLoaded(filename), queries: []pluginRuntimeTestReply{{output: running}}},
		{name: "already running original", before: pluginRuntimeRunning, reply: sourceModRuntimeAlreadyLoaded(filename), queries: []pluginRuntimeTestReply{{output: running}}},
		{name: "already after absent is a race", before: pluginRuntimeAbsent, reply: sourceModRuntimeAlreadyLoaded(filename), queries: repeatPluginRuntimeReply(running), wantErr: true, unknown: true},
		{name: "missing receipt cannot establish ownership", before: pluginRuntimeAbsent, queries: repeatPluginRuntimeReply(running), wantErr: true, unknown: true},
		{name: "other receipt cannot establish ownership", before: pluginRuntimeAbsent, reply: sourceModRuntimeLoaded("Other.smx"), queries: repeatPluginRuntimeReply(running), wantErr: true, unknown: true},
		{name: "conflicting receipt cannot establish ownership", before: pluginRuntimeAbsent, reply: sourceModRuntimeLoaded(filename) + "\n" + sourceModRuntimeFailed(filename), queries: repeatPluginRuntimeReply(running), wantErr: true, unknown: true},
		{name: "duplicate receipt cannot establish ownership", before: pluginRuntimeAbsent, reply: sourceModRuntimeLoaded(filename) + "\n" + sourceModRuntimeLoaded(filename), queries: repeatPluginRuntimeReply(running), wantErr: true, unknown: true},
		{name: "failed and absent", before: pluginRuntimeAbsent, reply: sourceModRuntimeFailed(filename), queries: []pluginRuntimeTestReply{{output: absent}}, wantErr: true},
		{name: "failed and nonrunning", before: pluginRuntimeAbsent, reply: sourceModRuntimeFailed(filename), queries: []pluginRuntimeTestReply{{output: nonrunning}}, wantErr: true},
		{name: "failure but running conflicts", before: pluginRuntimeAbsent, reply: sourceModRuntimeFailed(filename), queries: repeatPluginRuntimeReply(running), wantErr: true, unknown: true},
		{name: "current new plugin is owned even when nonrunning", before: pluginRuntimeAbsent, reply: sourceModRuntimeLoaded(filename), queries: []pluginRuntimeTestReply{{output: nonrunning}}, owned: true, wantErr: true},
		{name: "current new plugin is owned when absent conflicts", before: pluginRuntimeAbsent, reply: sourceModRuntimeLoaded(filename), queries: repeatPluginRuntimeReply(absent), owned: true, wantErr: true, unknown: true},
		{name: "transport error with running state", before: pluginRuntimeAbsent, commandErr: timeout, queries: repeatPluginRuntimeReply(running), wantErr: true, unknown: true},
		{name: "partial receipt with transport error is not trusted", before: pluginRuntimeAbsent, reply: sourceModRuntimeLoaded(filename), commandErr: timeout, queries: repeatPluginRuntimeReply(running), wantErr: true, unknown: true},
		{name: "query can recover without repeating load", before: pluginRuntimeAbsent, reply: sourceModRuntimeLoaded(filename), queries: []pluginRuntimeTestReply{{err: timeout}, {output: running}}, owned: true},
		{name: "unknown query retains current ownership", before: pluginRuntimeAbsent, reply: sourceModRuntimeLoaded(filename), queries: []pluginRuntimeTestReply{{err: timeout}, {err: timeout}, {err: timeout}}, owned: true, wantErr: true, unknown: true},
		{name: "already but nonrunning is a confirmed failure", before: pluginRuntimeNonrunning, reply: sourceModRuntimeAlreadyLoaded(filename), queries: []pluginRuntimeTestReply{{output: nonrunning}}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			replies := []pluginRuntimeTestReply{{command: `sm plugins load "Nested/123.smx"`, output: test.reply, err: test.commandErr}}
			for _, query := range test.queries {
				query.command = `sm plugins info "Nested/123.smx"`
				replies = append(replies, query)
			}
			usePluginRuntimeReplies(t, replies)
			owned, err := loadSourceModPlugin(filename, test.before)
			if owned != test.owned || (err != nil) != test.wantErr || errors.Is(err, ErrPluginResultUnknown) != test.unknown {
				t.Fatalf("owned = %v, error = %v; want owned %v, error %v, unknown %v", owned, err, test.owned, test.wantErr, test.unknown)
			}
		})
	}
}

func TestSourceModRuntimeRejectsUnknownPreflightAndInvalidNames(t *testing.T) {
	for _, test := range []struct {
		filename string
		before   pluginRuntimeState
	}{
		{"good.smx", pluginRuntimeUnknown},
		{"123", pluginRuntimeAbsent},
		{"../good.smx", pluginRuntimeAbsent},
		{"/good.smx", pluginRuntimeAbsent},
		{"a/./good.smx", pluginRuntimeAbsent},
		{"a\\good.smx", pluginRuntimeAbsent},
		{"a\nchangelevel.smx", pluginRuntimeAbsent},
	} {
		t.Run(fmt.Sprintf("%q/%d", test.filename, test.before), func(t *testing.T) {
			usePluginRuntimeReplies(t, nil)
			owned, err := loadSourceModPlugin(test.filename, test.before)
			if owned || !errors.Is(err, ErrPluginResultUnknown) {
				t.Fatalf("owned = %v, error = %v", owned, err)
			}
		})
	}
}

func TestSourceModRuntimeQuery(t *testing.T) {
	t.Run("case suffix and legitimate dots are preserved", func(t *testing.T) {
		const filename = "Nested/Foo..Bar.SMX"
		usePluginRuntimeReplies(t, []pluginRuntimeTestReply{{command: `sm plugins info "Nested/Foo..Bar.SMX"`, output: sourceModRuntimeInfo(filename, "  Status: running")}})
		state, err := querySourceModPluginState(filename)
		if state != pluginRuntimeRunning || err != nil {
			t.Fatalf("state = %v, error = %v", state, err)
		}
	})
	t.Run("numeric filename keeps suffix", func(t *testing.T) {
		usePluginRuntimeReplies(t, []pluginRuntimeTestReply{{command: `sm plugins info "123.smx"`, output: sourceModRuntimeAbsent("123.smx")}})
		state, err := querySourceModPluginState("123.smx")
		if state != pluginRuntimeAbsent || err != nil {
			t.Fatalf("state = %v, error = %v", state, err)
		}
	})
	t.Run("partial info with transport error is unknown", func(t *testing.T) {
		usePluginRuntimeReplies(t, []pluginRuntimeTestReply{{command: `sm plugins info "123.smx"`, output: sourceModRuntimeAbsent("123.smx"), err: errors.New("timeout")}})
		state, err := querySourceModPluginState("123.smx")
		if state != pluginRuntimeUnknown || !errors.Is(err, ErrPluginResultUnknown) {
			t.Fatalf("state = %v, error = %v", state, err)
		}
	})
}

func TestSourceModRuntimeRollback(t *testing.T) {
	timeout := errors.New("timeout")
	tests := []struct {
		name      string
		filenames []string
		replies   []pluginRuntimeTestReply
		unknown   bool
	}{
		{name: "reverse order and deduplicate", filenames: []string{"First.smx", "Nested/123.smx", "First.smx"}, replies: []pluginRuntimeTestReply{
			{command: `sm plugins info "First.smx"`, output: sourceModRuntimeInfo("First.smx", "  Status: running")},
			{command: `sm plugins unload "First.smx"`},
			{command: `sm plugins info "First.smx"`, output: sourceModRuntimeAbsent("First.smx")},
			{command: `sm plugins info "Nested/123.smx"`, output: sourceModRuntimeInfo("Nested/123.smx", "  Status: running")},
			{command: `sm plugins unload "Nested/123.smx"`},
			{command: `sm plugins info "Nested/123.smx"`, output: sourceModRuntimeAbsent("Nested/123.smx")},
		}},
		{name: "unload timeout but absent", filenames: []string{"123.smx"}, replies: []pluginRuntimeTestReply{
			{command: `sm plugins info "123.smx"`, output: sourceModRuntimeInfo("123.smx", "  Status: running")},
			{command: `sm plugins unload "123.smx"`, err: timeout},
			{command: `sm plugins info "123.smx"`, output: sourceModRuntimeAbsent("123.smx")},
		}},
		{name: "next frame without repeated unload", filenames: []string{"123.smx"}, replies: []pluginRuntimeTestReply{
			{command: `sm plugins info "123.smx"`, output: sourceModRuntimeInfo("123.smx", "  Status: running")},
			{command: `sm plugins unload "123.smx"`, output: "[SM] Plugin Display Name will be unloaded on the next frame."},
			{command: `sm plugins info "123.smx"`, output: sourceModRuntimeInfo("123.smx", "  Status: running")},
			{command: `sm plugins info "123.smx"`, err: timeout},
			{command: `sm plugins info "123.smx"`, output: sourceModRuntimeAbsent("123.smx")},
		}},
		{name: "still present does not trust success receipt", filenames: []string{"123.smx"}, unknown: true, replies: []pluginRuntimeTestReply{
			{command: `sm plugins info "123.smx"`, output: sourceModRuntimeInfo("123.smx", "  Status: running")},
			{command: `sm plugins unload "123.smx"`, output: "[SM] Plugin Display Name unloaded successfully."},
			{command: `sm plugins info "123.smx"`, output: sourceModRuntimeInfo("123.smx", "  Status: running")},
			{command: `sm plugins info "123.smx"`, output: sourceModRuntimeInfo("123.smx", "  Status: running")},
			{command: `sm plugins info "123.smx"`, output: sourceModRuntimeInfo("123.smx", "  Status: running")},
		}},
		{name: "stop earlier targets when later plugin remains running", filenames: []string{"First.smx", "Last.smx"}, unknown: true, replies: []pluginRuntimeTestReply{
			{command: `sm plugins info "Last.smx"`, output: sourceModRuntimeInfo("Last.smx", "  Status: running")},
			{command: `sm plugins unload "Last.smx"`},
			{command: `sm plugins info "Last.smx"`, output: sourceModRuntimeInfo("Last.smx", "  Status: running")},
			{command: `sm plugins info "Last.smx"`, output: sourceModRuntimeInfo("Last.smx", "  Status: running")},
			{command: `sm plugins info "Last.smx"`, output: sourceModRuntimeInfo("Last.smx", "  Status: running")},
		}},
		{name: "stop earlier targets when later plugin state is unknown", filenames: []string{"First.smx", "Last.smx"}, unknown: true, replies: []pluginRuntimeTestReply{
			{command: `sm plugins info "Last.smx"`, err: timeout},
		}},
		{name: "already absent needs no unload", filenames: []string{"123.smx"}, replies: []pluginRuntimeTestReply{
			{command: `sm plugins info "123.smx"`, output: sourceModRuntimeAbsent("123.smx")},
		}},
		{name: "unknown pre-unload state cannot mutate", filenames: []string{"123.smx"}, unknown: true, replies: []pluginRuntimeTestReply{
			{command: `sm plugins info "123.smx"`, err: timeout},
		}},
		{name: "invalid target cannot mutate", filenames: []string{"123"}, unknown: true},
		{name: "empty list"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usePluginRuntimeReplies(t, test.replies)
			err := rollbackNewSourceModPlugins(test.filenames)
			if (err != nil) != test.unknown || errors.Is(err, ErrPluginResultUnknown) != test.unknown {
				t.Fatalf("error = %v, want unknown %v", err, test.unknown)
			}
		})
	}
}

type pluginRuntimeTestReply struct {
	command string
	output  string
	err     error
}

func usePluginRuntimeReplies(t *testing.T, replies []pluginRuntimeTestReply) {
	t.Helper()
	oldExecutor, oldWait := executePluginRconCommand, waitForPluginRuntimeQuery
	waitForPluginRuntimeQuery = func() {}
	var commands []string
	executePluginRconCommand = func(command string) (string, error) {
		commands = append(commands, command)
		index := len(commands) - 1
		if index >= len(replies) {
			t.Fatalf("unexpected command: %s", command)
		}
		reply := replies[index]
		if command != reply.command {
			t.Fatalf("command %d = %q, want %q", index, command, reply.command)
		}
		return reply.output, reply.err
	}
	t.Cleanup(func() {
		executePluginRconCommand, waitForPluginRuntimeQuery = oldExecutor, oldWait
		var want []string
		for _, reply := range replies {
			want = append(want, reply.command)
		}
		if !reflect.DeepEqual(commands, want) {
			t.Errorf("commands = %q, want %q", commands, want)
		}
	})
}

func repeatPluginRuntimeReply(output string) []pluginRuntimeTestReply {
	return []pluginRuntimeTestReply{{output: output}, {output: output}, {output: output}}
}

func sourceModRuntimeInfo(filename string, fields ...string) string {
	return "  Filename: " + filename + "\n" + strings.Join(fields, "\n")
}

func sourceModRuntimeAbsent(filename string) string {
	return "[SM] Plugin " + filename + " is not loaded."
}

func sourceModRuntimeLoaded(filename string) string {
	return "[SM] Loaded plugin " + filename + " successfully."
}

func sourceModRuntimeAlreadyLoaded(filename string) string {
	return "[SM] Plugin " + filename + " is already loaded."
}

func sourceModRuntimeFailed(filename string) string {
	return "[SM] Plugin " + filename + " failed to load: Missing native."
}
