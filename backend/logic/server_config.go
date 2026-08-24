package logic

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

var ErrUnboundServerConfigComment = errors.New("自定义配置存在未关联指令的末尾注释")

type ServerConfigSettings struct {
	Hidden           bool
	LobbyConnectOnly bool
	SteamGroup       string
	CustomConfig     []string
}

type ServerCustomConfigEntry struct {
	Comments []string
	Command  string
}

// NormalizeServerCustomConfig associates full-line comments with the next
// command and moves supported inline comments above that command.
func NormalizeServerCustomConfig(lines []string) ([]string, error) {
	entries, trailingComments, trailingStartLine := parseServerCustomConfig(lines)
	if len(trailingComments) > 0 {
		return nil, fmt.Errorf(
			"%w（从第 %d 行开始，共 %d 行）",
			ErrUnboundServerConfigComment,
			trailingStartLine,
			len(trailingComments),
		)
	}
	return serializeServerCustomConfig(entries), nil
}

// ExtractServerCustomConfig returns the non-empty, non-managed lines in the
// custom block without rewriting the file. This keeps legacy inline comments
// available to the text editor until the next save normalizes them.
func ExtractServerCustomConfig(content string) []string {
	lines := splitServerConfigLines(content)
	customLines := make([]string, 0)
	pendingComments := make([]string, 0)
	inCustomBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == ServerCustomConfigMarker {
			inCustomBlock = true
			continue
		}
		if !inCustomBlock || trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "//") {
			pendingComments = append(pendingComments, line)
			continue
		}
		if IsManagedServerConfigLine(line) {
			pendingComments = pendingComments[:0]
			continue
		}
		customLines = append(customLines, pendingComments...)
		pendingComments = pendingComments[:0]
		customLines = append(customLines, line)
	}
	customLines = append(customLines, pendingComments...)
	return customLines
}

// ExtractRedactedServerFixedConfig returns the current server.cfg content that
// is preserved above the manager custom marker. Managed fields are omitted and
// password directive values are always redacted before leaving the backend.
func ExtractRedactedServerFixedConfig(content string) string {
	lines := splitServerConfigLines(content)
	fixedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == ServerCustomConfigMarker {
			break
		}
		if IsManagedServerConfigLine(line) {
			continue
		}
		fixedLines = append(fixedLines, redactServerConfigPassword(line))
	}
	fixedLines = trimTrailingBlankLines(fixedLines)
	return strings.Join(fixedLines, "\n")
}

func IsManagedServerConfigLine(line string) bool {
	command, _, _ := splitInlineServerConfigComment(line)
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	switch {
	case strings.EqualFold(fields[0], "sv_tags"):
		return true
	case strings.EqualFold(fields[0], "sv_steamgroup"):
		return true
	case len(fields) >= 2 && strings.EqualFold(fields[0], "sm_cvar") && strings.EqualFold(fields[1], "sv_allow_lobby_connect_only"):
		return true
	default:
		return false
	}
}

// UpdateServerConfigFile preserves the file-specific fixed section and writes
// the manager-owned fields and normalized custom configuration atomically.
func UpdateServerConfigFile(configPath string, settings ServerConfigSettings) error {
	normalizedCustomConfig, err := NormalizeServerCustomConfig(settings.CustomConfig)
	if err != nil {
		return err
	}

	contentBytes, err := os.ReadFile(configPath)
	mode := os.FileMode(0644)
	var lines []string
	switch {
	case err == nil:
		lines = splitServerConfigLines(string(contentBytes))
		info, statErr := os.Stat(configPath)
		if statErr != nil {
			return fmt.Errorf("读取 %s 权限失败: %w", configPath, statErr)
		}
		mode = info.Mode().Perm()
	case os.IsNotExist(err):
		lines = []string{}
	default:
		return fmt.Errorf("读取 %s 失败: %w", configPath, err)
	}

	originalTags := extractServerTags(lines)
	newLines := make([]string, 0, len(lines)+len(normalizedCustomConfig)+5)
	for _, line := range lines {
		if strings.TrimSpace(line) == ServerCustomConfigMarker {
			break
		}
		if IsManagedServerConfigLine(line) {
			continue
		}
		newLines = append(newLines, line)
	}
	newLines = trimTrailingBlankLines(newLines)
	if len(newLines) > 0 {
		newLines = append(newLines, "")
	}
	newLines = append(newLines, ServerCustomConfigMarker)

	if settings.Hidden {
		originalTags = append(originalTags, "hidden")
	}
	if uniqueTags := uniqueServerTags(originalTags); len(uniqueTags) > 0 {
		newLines = append(newLines, fmt.Sprintf("sv_tags \"%s\"", strings.Join(uniqueTags, ",")))
	}

	lobbyValue := "0"
	if settings.LobbyConnectOnly {
		lobbyValue = "1"
	}
	newLines = append(newLines, fmt.Sprintf("sm_cvar sv_allow_lobby_connect_only \"%s\"", lobbyValue))
	if settings.SteamGroup != "" {
		newLines = append(newLines, fmt.Sprintf("sv_steamgroup \"%s\"", settings.SteamGroup))
	}
	newLines = append(newLines, normalizedCustomConfig...)

	return atomicWriteFile(configPath, []byte(strings.Join(newLines, "\n")+"\n"), mode)
}

func parseServerCustomConfig(lines []string) ([]ServerCustomConfigEntry, []string, int) {
	entries := make([]ServerCustomConfigEntry, 0)
	pendingComments := make([]string, 0)
	pendingStartLine := 0

	for index, rawLine := range lines {
		line := strings.TrimSuffix(rawLine, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "//") {
			if len(pendingComments) == 0 {
				pendingStartLine = index + 1
			}
			pendingComments = append(pendingComments, strings.TrimSpace(strings.TrimPrefix(trimmed, "//")))
			continue
		}

		command, inlineComment, hasInlineComment := splitInlineServerConfigComment(line)
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		comments := append([]string(nil), pendingComments...)
		if hasInlineComment {
			comments = append(comments, inlineComment)
		}
		entries = append(entries, ServerCustomConfigEntry{Comments: comments, Command: command})
		pendingComments = pendingComments[:0]
		pendingStartLine = 0
	}

	return entries, append([]string(nil), pendingComments...), pendingStartLine
}

func serializeServerCustomConfig(entries []ServerCustomConfigEntry) []string {
	lines := make([]string, 0)
	for _, entry := range entries {
		for _, comment := range entry.Comments {
			comment = strings.TrimSpace(comment)
			if comment == "" {
				lines = append(lines, "//")
			} else {
				lines = append(lines, "// "+comment)
			}
		}
		if command := strings.TrimSpace(entry.Command); command != "" {
			lines = append(lines, command)
		}
	}
	return lines
}

func splitInlineServerConfigComment(line string) (string, string, bool) {
	inQuotes := false
	for index := 0; index+1 < len(line); index++ {
		if line[index] == '"' && !isEscapedServerConfigByte(line, index) {
			inQuotes = !inQuotes
			continue
		}
		if inQuotes || line[index] != '/' || line[index+1] != '/' {
			continue
		}
		if isServerConfigURLSlash(line, index) {
			continue
		}
		return strings.TrimSpace(line[:index]), strings.TrimSpace(line[index+2:]), true
	}
	return line, "", false
}

func isServerConfigURLSlash(line string, slashIndex int) bool {
	tokenStart := slashIndex
	for tokenStart > 0 && line[tokenStart-1] != ' ' && line[tokenStart-1] != '\t' {
		tokenStart--
	}
	tokenPrefix := line[tokenStart:slashIndex]
	if strings.Contains(tokenPrefix, `"`) {
		return false
	}
	colonIndex := strings.LastIndex(tokenPrefix, "://")
	if colonIndex < 0 {
		if !strings.HasSuffix(tokenPrefix, ":") {
			return false
		}
		colonIndex = len(tokenPrefix) - 1
	}

	schemeStart := colonIndex
	for schemeStart > 0 && isServerConfigURLSchemeByte(tokenPrefix[schemeStart-1]) {
		schemeStart--
	}
	scheme := tokenPrefix[schemeStart:colonIndex]
	if len(scheme) == 0 || !isASCIIAlpha(scheme[0]) {
		return false
	}
	for index := 1; index < len(scheme); index++ {
		if !isServerConfigURLSchemeByte(scheme[index]) {
			return false
		}
	}
	return true
}

func isServerConfigURLSchemeByte(value byte) bool {
	return isASCIIAlpha(value) || value >= '0' && value <= '9' || value == '+' || value == '-' || value == '.'
}

func isASCIIAlpha(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func isEscapedServerConfigByte(value string, index int) bool {
	backslashes := 0
	for index--; index >= 0 && value[index] == '\\'; index-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func redactServerConfigPassword(line string) string {
	command, inlineComment, hasInlineComment := splitInlineServerConfigComment(line)
	trimmedCommand := strings.TrimSpace(command)
	fields := strings.Fields(trimmedCommand)
	if len(fields) == 0 {
		return line
	}
	directiveIndex := 0
	if strings.EqualFold(fields[0], "sm_cvar") {
		if len(fields) < 2 {
			return line
		}
		directiveIndex = 1
	}
	directive := strings.ToLower(fields[directiveIndex])
	if directive != "password" && !strings.HasSuffix(directive, "_password") {
		return line
	}

	leadingWhitespace := command[:len(command)-len(strings.TrimLeft(command, " \t"))]
	redactedFields := fields[:directiveIndex+1]
	redacted := leadingWhitespace + strings.Join(redactedFields, " ") + ` "********"`
	if hasInlineComment {
		redacted += " //"
		if inlineComment != "" {
			redacted += " " + inlineComment
		}
	}
	return redacted
}

func extractServerTags(lines []string) []string {
	tags := make([]string, 0)
	for _, line := range lines {
		command, _, _ := splitInlineServerConfigComment(line)
		trimmed := strings.TrimSpace(command)
		fields := strings.Fields(trimmed)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "sv_tags") {
			continue
		}
		args := strings.TrimSpace(trimmed[len(fields[0]):])
		args = strings.Trim(args, `"`)
		for _, part := range strings.Split(args, ",") {
			tag := strings.TrimSpace(part)
			if tag != "" && !strings.EqualFold(tag, "hidden") {
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

func uniqueServerTags(tags []string) []string {
	unique := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if !seen[tag] {
			unique = append(unique, tag)
			seen[tag] = true
		}
	}
	return unique
}

func splitServerConfigLines(content string) []string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	return strings.Split(normalized, "\n")
}
