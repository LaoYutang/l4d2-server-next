package logic

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
)

// ErrPluginResultUnknown means that neither the command reply nor the follow-up
// query established a safe result. Callers must retain deployments in this case.
var ErrPluginResultUnknown = errors.New("plugin command result is unknown")

type pluginRuntimeState uint8

const (
	pluginRuntimeUnknown pluginRuntimeState = iota
	pluginRuntimeAbsent
	pluginRuntimeRunning
	pluginRuntimeNonrunning
)

type sourceModLoadReply uint8

const (
	sourceModLoadReplyUnknown sourceModLoadReply = iota
	sourceModLoadReplyLoaded
	sourceModLoadReplyAlreadyLoaded
	sourceModLoadReplyFailed
)

const pluginRuntimeQueryAttempts = 3

var waitForPluginRuntimeQuery = func() { time.Sleep(50 * time.Millisecond) }

// SourceMod treats a bare numeric argument as a plugin index for info/unload.
// Keep the complete relative filename, including its case, directories and .smx.
func validateSourceModPluginFilename(filename string) error {
	if filename == "" || !strings.EqualFold(path.Ext(filename), ".smx") ||
		strings.ContainsAny(filename, "\x00\r\n\\:") ||
		strings.HasPrefix(filename, "/") || strings.HasPrefix(filename, "../") || path.Clean(filename) != filename {
		return fmt.Errorf("%w: invalid plugin filename %q", ErrPluginResultUnknown, filename)
	}
	return nil
}

// parseSourceModPluginLoadReply only accepts a single, exact reply for this
// filename. Server output and replies for other plugins do not classify it.
func parseSourceModPluginLoadReply(output, filename string) (sourceModLoadReply, string) {
	loaded := "[SM] Loaded plugin " + filename + " successfully."
	already := "[SM] Plugin " + filename + " is already loaded."
	failed := "[SM] Plugin " + filename + " failed to load: "
	reply := sourceModLoadReplyUnknown
	var message string
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		var next sourceModLoadReply
		switch {
		case line == loaded:
			next = sourceModLoadReplyLoaded
		case line == already:
			next = sourceModLoadReplyAlreadyLoaded
		case strings.HasPrefix(line, failed) && strings.HasSuffix(line, "."):
			next = sourceModLoadReplyFailed
		default:
			continue
		}
		if reply != sourceModLoadReplyUnknown {
			return sourceModLoadReplyUnknown, "conflicting or repeated plugin load replies"
		}
		reply, message = next, line
	}
	return reply, message
}

// SourceMod 1.11/1.12 info prints two-space-indented fields. Requiring that
// prefix prevents unindented engine "Error:" lines from becoming plugin errors.
// A target block must have exactly one status field; even an empty Error field
// means nonrunning. Unknown, repeated or contradictory evidence stays unknown.
func parseSourceModPluginInfo(output, filename string) (pluginRuntimeState, error) {
	missing := "[SM] Plugin " + filename + " is not loaded."
	absentCount, targetCount, statusCount := 0, 0, 0
	inTarget := false
	state := pluginRuntimeUnknown
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSuffix(raw, "\r")
		if strings.TrimSpace(line) == missing {
			absentCount++
		}
		if strings.HasPrefix(line, "  Filename:") {
			inTarget = strings.TrimPrefix(line, "  Filename:") == " "+filename
			if inTarget {
				targetCount++
			}
			continue
		}
		if !inTarget {
			continue
		}
		switch {
		case strings.HasPrefix(line, "  Status:"):
			statusCount++
			if strings.TrimSpace(strings.TrimPrefix(line, "  Status:")) == "running" {
				state = pluginRuntimeRunning
			} else {
				state = pluginRuntimeUnknown
			}
		case strings.HasPrefix(line, "  Error:"), strings.HasPrefix(line, "  Load error:"):
			statusCount++
			state = pluginRuntimeNonrunning
		}
	}
	if absentCount == 1 && targetCount == 0 {
		return pluginRuntimeAbsent, nil
	}
	if absentCount == 0 && targetCount == 1 && statusCount == 1 && state != pluginRuntimeUnknown {
		return state, nil
	}
	return pluginRuntimeUnknown, fmt.Errorf("%w: incomplete or conflicting info for %s", ErrPluginResultUnknown, filename)
}

func querySourceModPluginState(filename string) (pluginRuntimeState, error) {
	if err := validateSourceModPluginFilename(filename); err != nil {
		return pluginRuntimeUnknown, err
	}
	output, err := executePluginRconCommand("sm plugins info " + quoteSourceModArg(filename))
	if err != nil {
		return pluginRuntimeUnknown, fmt.Errorf("%w: query %s: %v", ErrPluginResultUnknown, filename, err)
	}
	return parseSourceModPluginInfo(output, filename)
}

// loadSourceModPlugin never repeats a mutation. Ownership requires both an
// absent preflight and an unambiguous success reply, independently of postflight.
func loadSourceModPlugin(filename string, before pluginRuntimeState) (owned bool, err error) {
	if err := validateSourceModPluginFilename(filename); err != nil {
		return false, err
	}
	if before != pluginRuntimeAbsent && before != pluginRuntimeRunning && before != pluginRuntimeNonrunning {
		return false, fmt.Errorf("%w: no preflight state for %s", ErrPluginResultUnknown, filename)
	}
	output, commandErr := executePluginRconCommand("sm plugins load " + quoteSourceModArg(filename))
	reply, message := sourceModLoadReplyUnknown, ""
	if commandErr == nil {
		reply, message = parseSourceModPluginLoadReply(output, filename)
	}
	owned = before == pluginRuntimeAbsent && reply == sourceModLoadReplyLoaded
	var queryErr error
	for attempt := 0; attempt < pluginRuntimeQueryAttempts; attempt++ {
		if attempt > 0 {
			waitForPluginRuntimeQuery()
		}
		var after pluginRuntimeState
		after, queryErr = querySourceModPluginState(filename)
		if queryErr != nil {
			continue
		}
		switch reply {
		case sourceModLoadReplyLoaded:
			if after == pluginRuntimeRunning {
				return owned, nil
			}
			if after == pluginRuntimeNonrunning {
				return owned, fmt.Errorf("plugin %s is not running after load", filename)
			}
		case sourceModLoadReplyAlreadyLoaded:
			// An absent preflight followed by AlreadyLoaded indicates a race;
			// its deployment must not be treated as ours, even if it runs now.
			if before != pluginRuntimeAbsent && after == pluginRuntimeRunning {
				return false, nil
			}
			if before != pluginRuntimeAbsent && after == pluginRuntimeNonrunning {
				return false, fmt.Errorf("plugin %s is already loaded but not running", filename)
			}
		case sourceModLoadReplyFailed:
			if after == pluginRuntimeAbsent || after == pluginRuntimeNonrunning {
				return false, fmt.Errorf("%s", message)
			}
		}
	}
	detail := "未收到唯一的目标加载回执，或目标状态与回执不一致"
	if commandErr != nil {
		detail = "加载响应失败: " + commandErr.Error()
	}
	if queryErr != nil {
		detail += "; 状态查询失败: " + queryErr.Error()
	}
	return owned, fmt.Errorf("%w: %s: %s", ErrPluginResultUnknown, filename, detail)
}

// Only call this with filenames whose absence before loading was established.
// Unload receipts may contain a display name, so absence is confirmed with info.
func rollbackNewSourceModPlugins(filenames []string) error {
	var errs []error
	seen := make(map[string]bool, len(filenames))
	for i := len(filenames) - 1; i >= 0; i-- {
		filename := filenames[i]
		if seen[filename] {
			continue
		}
		seen[filename] = true
		if err := unloadNewSourceModPlugin(filename); err != nil {
			errs = append(errs, err)
			// A later plugin may still depend on earlier ones. Preserve them
			// until this unload is confirmed instead of breaking its dependencies.
			if errors.Is(err, ErrPluginResultUnknown) {
				return errors.Join(errs...)
			}
		}
	}
	return errors.Join(errs...)
}

func unloadNewSourceModPlugin(filename string) error {
	if err := validateSourceModPluginFilename(filename); err != nil {
		return err
	}
	before, err := querySourceModPluginState(filename)
	if err != nil {
		return err
	}
	if before == pluginRuntimeAbsent {
		return nil
	}
	_, commandErr := executePluginRconCommand("sm plugins unload " + quoteSourceModArg(filename))
	var queryErr error
	for attempt := 0; attempt < pluginRuntimeQueryAttempts; attempt++ {
		if attempt > 0 {
			waitForPluginRuntimeQuery()
		}
		var state pluginRuntimeState
		state, queryErr = querySourceModPluginState(filename)
		if queryErr == nil && state == pluginRuntimeAbsent {
			return nil
		}
	}
	detail := "卸载后仍能查询到目标插件"
	if commandErr != nil {
		detail = "卸载响应失败: " + commandErr.Error()
	}
	if queryErr != nil {
		detail += "; 状态查询失败: " + queryErr.Error()
	}
	return fmt.Errorf("%w: %s: %s", ErrPluginResultUnknown, filename, detail)
}
