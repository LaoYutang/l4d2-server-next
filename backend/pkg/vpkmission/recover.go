package vpkmission

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"l4d2-manager-next/pkg/valve/vdf"
)

type missionParseResult struct {
	Campaign  *Campaign
	Recovered bool
	Issues    []missionRecoveryIssue
}

type missionRecoveryIssue struct {
	Line   int
	Column int
	Kind   string
	Detail string
}

type strictMissionStats struct {
	hasMission      bool
	hasModes        bool
	expectedMapKeys int
	allMapKeys      int
}

type missionTokenKind uint8

const (
	missionTokenScalar missionTokenKind = iota
	missionTokenOpenBrace
	missionTokenCloseBrace
)

type missionToken struct {
	Kind   missionTokenKind
	Text   string
	Line   int
	Column int
	Indent int
	Quoted bool
}

type missionField struct {
	Name          string
	Value         string
	Next          int
	Line          int
	Column        int
	WrappedBraces int
	UnquotedWords int
}

type semanticRole uint8

const (
	semanticUnknown semanticRole = iota
	semanticMission
	semanticModes
	semanticMode
	semanticChapter
	semanticTransparent
)

type recoveryFrame struct {
	role         semanticRole
	key          string
	line         int
	indent       int
	physical     bool
	chapterCode  string
	chapterTitle string
}

type modeHint struct {
	key    string
	line   int
	indent int
	valid  bool
}

func parseMissionData(data []byte) (missionParseResult, error) {
	tokens, lexicalIssues := lexMission(data)

	var root vdf.KeyValues
	readBytes, strictErr := root.ReadFrom(bytes.NewReader(data))
	if strictErr == nil {
		campaign, stats := campaignFromStrictKeyValues(&root)
		if strictMissionIsValid(stats, tokens) {
			return missionParseResult{Campaign: campaign}, nil
		}
	}

	issues := make([]missionRecoveryIssue, 0, len(lexicalIssues)+8)
	if strictErr != nil {
		line, column := sourcePosition(data, readBytes)
		issues = append(issues, missionRecoveryIssue{
			Line:   line,
			Column: column,
			Kind:   "strict_parse_failed",
			Detail: strictErr.Error(),
		})
	} else {
		issues = append(issues, missionRecoveryIssue{
			Kind:   "strict_semantic_validation_failed",
			Detail: "strict VDF tree omitted or misplaced mission Map fields",
		})
	}
	issues = append(issues, lexicalIssues...)

	campaign, recoveryIssues := recoverMissionTokens(tokens)
	issues = append(issues, recoveryIssues...)
	if campaign == nil || len(campaign.Chapters) == 0 {
		if strictErr != nil {
			return missionParseResult{}, fmt.Errorf(
				"strict parse failed (%v) and semantic recovery found no chapters",
				strictErr,
			)
		}
		return missionParseResult{}, fmt.Errorf("mission semantic validation failed and recovery found no chapters")
	}

	return missionParseResult{
		Campaign:  campaign,
		Recovered: true,
		Issues:    issues,
	}, nil
}

func campaignFromStrictKeyValues(root *vdf.KeyValues) (*Campaign, strictMissionStats) {
	campaign := &Campaign{Chapters: make([]*Chapter, 0, 8)}
	stats := strictMissionStats{}

	mission := findTopLevelKey(root, "mission")
	if mission == nil {
		return campaign, stats
	}
	stats.hasMission = true
	campaign.Title = valueForKey(mission, "displaytitle")

	for child := mission.FirstTrueSubKey(); child != nil; child = child.NextTrueSubKey() {
		if !strings.EqualFold(child.Key, "modes") {
			continue
		}
		stats.hasModes = true

		for modeNode := child.FirstTrueSubKey(); modeNode != nil; modeNode = modeNode.NextTrueSubKey() {
			mode := strings.ToLower(strings.TrimSpace(modeNode.Key))
			if mode == "" {
				continue
			}
			for _, chapter := range chaptersFromMode(modeNode, mode) {
				stats.expectedMapKeys++
				mergeChapter(campaign, chapter)
			}
		}
	}

	stats.allMapKeys = countMapValues(mission)

	return campaign, stats
}

func countMapValues(node *vdf.KeyValues) int {
	if node == nil {
		return 0
	}

	count := 0
	for child := node.FirstSubKey(); child != nil; child = child.NextSubKey() {
		if child.HasValue && strings.EqualFold(child.Key, "map") && strings.TrimSpace(child.Value) != "" {
			count++
		}
	}
	for child := node.FirstTrueSubKey(); child != nil; child = child.NextTrueSubKey() {
		count += countMapValues(child)
	}
	return count
}

func strictMissionIsValid(stats strictMissionStats, tokens []missionToken) bool {
	if !stats.hasMission || !stats.hasModes || stats.expectedMapKeys == 0 {
		return false
	}
	if stats.allMapKeys != stats.expectedMapKeys {
		return false
	}
	return countLooseMapFields(tokens) == stats.expectedMapKeys
}

func findTopLevelKey(root *vdf.KeyValues, key string) *vdf.KeyValues {
	for node := root; node != nil; node = node.NextSubKey() {
		if strings.EqualFold(node.Key, key) {
			return node
		}
	}
	return nil
}

func countLooseMapFields(tokens []missionToken) int {
	count := 0
	for i := 0; i < len(tokens); {
		field, ok := readMissionField(tokens, i)
		if !ok {
			i++
			continue
		}
		if field.Name == "map" && field.Value != "" {
			count++
		}
		if field.Next <= i {
			i++
		} else {
			i = field.Next
		}
	}
	return count
}

func lexMission(data []byte) ([]missionToken, []missionRecoveryIssue) {
	input := []rune(string(data))
	tokens := make([]missionToken, 0, len(input)/6)
	issues := make([]missionRecoveryIssue, 0, 4)

	line := 1
	column := 1
	indent := 0
	atLineStart := true

	for i := 0; i < len(input); {
		r := input[i]

		if r == '\r' {
			i++
			if i < len(input) && input[i] == '\n' {
				i++
			}
			line++
			column = 1
			indent = 0
			atLineStart = true
			continue
		}
		if r == '\n' {
			i++
			line++
			column = 1
			indent = 0
			atLineStart = true
			continue
		}
		if unicode.IsSpace(r) {
			if atLineStart {
				if r == '\t' {
					indent += 4
				} else {
					indent++
				}
			}
			i++
			column++
			continue
		}

		atLineStart = false
		if r == '/' && i+1 < len(input) && input[i+1] == '/' {
			for i < len(input) && input[i] != '\r' && input[i] != '\n' {
				i++
				column++
			}
			continue
		}

		switch r {
		case '{':
			tokens = append(tokens, missionToken{
				Kind:   missionTokenOpenBrace,
				Text:   "{",
				Line:   line,
				Column: column,
				Indent: indent,
			})
			i++
			column++
			continue
		case '}':
			tokens = append(tokens, missionToken{
				Kind:   missionTokenCloseBrace,
				Text:   "}",
				Line:   line,
				Column: column,
				Indent: indent,
			})
			i++
			column++
			continue
		case '"':
			startLine, startColumn, startIndent := line, column, indent
			i++
			column++

			var value strings.Builder
			closed := false
			for i < len(input) {
				r = input[i]
				if r == '\r' || r == '\n' {
					break
				}
				if r == '\\' && i+1 < len(input) && input[i+1] != '\r' && input[i+1] != '\n' {
					value.WriteRune(r)
					value.WriteRune(input[i+1])
					i += 2
					column += 2
					continue
				}
				if r == '"' {
					closed = true
					i++
					column++
					break
				}
				value.WriteRune(r)
				i++
				column++
			}

			tokens = append(tokens, missionToken{
				Kind:   missionTokenScalar,
				Text:   vdf.Unescape.Replace(value.String()),
				Line:   startLine,
				Column: startColumn,
				Indent: startIndent,
				Quoted: true,
			})
			if !closed {
				issues = append(issues, missionRecoveryIssue{
					Line:   startLine,
					Column: startColumn,
					Kind:   "missing_quote",
					Detail: "closed an unterminated quoted token at the end of its line",
				})
			}
			continue
		}

		start := i
		startColumn := column
		for i < len(input) {
			r = input[i]
			if r == '\r' || r == '\n' || unicode.IsSpace(r) || r == '{' || r == '}' || r == '"' {
				break
			}
			if r == '/' && i+1 < len(input) && input[i+1] == '/' {
				break
			}
			i++
			column++
		}
		if start == i {
			i++
			column++
			continue
		}
		end := i
		if i < len(input) && input[i] == '"' {
			i++
			column++
			issues = append(issues, missionRecoveryIssue{
				Line:   line,
				Column: column - 1,
				Kind:   "missing_quote",
				Detail: "treated a quote after an unquoted token as its missing opening quote pair",
			})
		}
		tokens = append(tokens, missionToken{
			Kind:   missionTokenScalar,
			Text:   string(input[start:end]),
			Line:   line,
			Column: startColumn,
			Indent: indent,
		})
	}

	return tokens, issues
}

func readMissionField(tokens []missionToken, index int) (missionField, bool) {
	if index < 0 || index >= len(tokens) || tokens[index].Kind != missionTokenScalar {
		return missionField{}, false
	}

	name, embeddedValue, ok := splitMissionFieldToken(tokens[index].Text)
	if !ok {
		return missionField{}, false
	}

	field := missionField{
		Name:   name,
		Value:  strings.TrimSpace(embeddedValue),
		Next:   index + 1,
		Line:   tokens[index].Line,
		Column: tokens[index].Column,
	}
	if field.Value != "" {
		return field, true
	}

	valueIndex := index + 1
	for valueIndex < len(tokens) && tokens[valueIndex].Kind == missionTokenOpenBrace {
		field.WrappedBraces++
		valueIndex++
	}
	if valueIndex >= len(tokens) || tokens[valueIndex].Kind != missionTokenScalar {
		field.Next = valueIndex
		return field, true
	}

	if _, _, isNextField := splitMissionFieldToken(tokens[valueIndex].Text); isNextField && !tokens[valueIndex].Quoted {
		field.Next = valueIndex
		return field, true
	}

	first := tokens[valueIndex]

	if name == "map" {
		field.Value = cleanRecoveredValue(first.Text)
		field.UnquotedWords = 1
		field.Next = valueIndex + 1
	} else if first.Quoted {
		field.Value = cleanRecoveredValue(first.Text)
		field.UnquotedWords = 1
		field.Next = valueIndex + 1
	} else {
		parts := make([]string, 0, 4)
		valueLine := first.Line
		cursor := valueIndex
		for cursor < len(tokens) {
			token := tokens[cursor]
			if token.Line != valueLine || token.Kind != missionTokenScalar {
				break
			}
			if cursor > valueIndex {
				if _, _, isNextField := splitMissionFieldToken(token.Text); isNextField {
					break
				}
			}
			value := cleanRecoveredValue(token.Text)
			if value != "" {
				parts = append(parts, value)
				field.UnquotedWords++
			}
			cursor++
		}
		field.Value = strings.Join(parts, " ")
		field.Next = cursor
	}

	if field.WrappedBraces > 0 {
		remaining := field.WrappedBraces
		cursor := field.Next
		for cursor < len(tokens) && remaining > 0 && tokens[cursor].Kind == missionTokenCloseBrace {
			remaining--
			cursor++
		}
		field.Next = cursor
	}

	return field, true
}

func splitMissionFieldToken(value string) (string, string, bool) {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	for _, key := range []string{"displaytitle", "displayname", "map"} {
		if lower == key {
			return key, "", true
		}
		if len(lower) > len(key) && lower[:len(key)] == key && unicode.IsSpace(rune(lower[len(key)])) {
			return key, strings.TrimSpace(trimmed[len(key):]), true
		}
	}
	return "", "", false
}

func cleanRecoveredValue(value string) string {
	return strings.Trim(strings.TrimSpace(value), "\"")
}

func recoverMissionTokens(tokens []missionToken) (*Campaign, []missionRecoveryIssue) {
	campaign := &Campaign{Chapters: make([]*Chapter, 0, 8)}
	issues := make([]missionRecoveryIssue, 0, 8)
	stack := make([]recoveryFrame, 0, 8)

	var lastMode modeHint
	var lastMapCode string
	var looseChapterTitle string
	seenMission := false
	seenModes := false

	for i := 0; i < len(tokens); {
		token := tokens[i]

		if field, ok := readMissionField(tokens, i); ok {
			if !seenMission && !seenModes {
				if field.Next <= i {
					i++
				} else {
					i = field.Next
				}
				continue
			}
			if field.WrappedBraces > 0 {
				issues = append(issues, missionRecoveryIssue{
					Line:   field.Line,
					Column: field.Column,
					Kind:   "extra_open_brace",
					Detail: fmt.Sprintf("treated %d brace pair(s) around %s as transparent", field.WrappedBraces, field.Name),
				})
			}
			if field.UnquotedWords > 1 {
				issues = append(issues, missionRecoveryIssue{
					Line:   field.Line,
					Column: field.Column,
					Kind:   "missing_quote",
					Detail: fmt.Sprintf("joined %d unquoted words for %s", field.UnquotedWords, field.Name),
				})
			}

			switch field.Name {
			case "displaytitle":
				if campaign.Title == "" {
					campaign.Title = field.Value
				}
			case "displayname":
				if field.Value != "" {
					if chapterIndex := nearestFrame(stack, semanticChapter); chapterIndex >= 0 {
						stack[chapterIndex].chapterTitle = field.Value
						if stack[chapterIndex].chapterCode != "" {
							setRecoveredChapterTitle(campaign, stack[chapterIndex].chapterCode, field.Value)
						}
					} else if lastMapCode != "" {
						setRecoveredChapterTitle(campaign, lastMapCode, field.Value)
					} else {
						looseChapterTitle = field.Value
					}
				}
			case "map":
				if field.Value != "" {
					mode := recoveredMode(stack, lastMode, token)
					title := looseChapterTitle
					if chapterIndex := nearestFrame(stack, semanticChapter); chapterIndex >= 0 {
						if stack[chapterIndex].chapterTitle != "" {
							title = stack[chapterIndex].chapterTitle
						}
						stack[chapterIndex].chapterCode = field.Value
					}

					modes := []string(nil)
					if mode != "" {
						modes = []string{mode}
					}
					mergeChapter(campaign, &Chapter{
						Code:  field.Value,
						Title: title,
						Modes: modes,
					})
					lastMapCode = field.Value
					looseChapterTitle = ""
				}
			}

			if field.Next <= i {
				i++
			} else {
				i = field.Next
			}
			continue
		}

		switch token.Kind {
		case missionTokenOpenBrace:
			stack = append(stack, recoveryFrame{
				role:     semanticTransparent,
				line:     token.Line,
				indent:   token.Indent,
				physical: true,
			})
			issues = append(issues, missionRecoveryIssue{
				Line:   token.Line,
				Column: token.Column,
				Kind:   "extra_open_brace",
				Detail: "treated an unexpected opening brace as a transparent block",
			})
			i++
			continue
		case missionTokenCloseBrace:
			if len(stack) == 0 {
				issues = append(issues, missionRecoveryIssue{
					Line:   token.Line,
					Column: token.Column,
					Kind:   "extra_close_brace",
					Detail: "ignored an unmatched closing brace",
				})
			} else {
				stack = stack[:len(stack)-1]
			}
			i++
			continue
		}

		if token.Kind != missionTokenScalar {
			i++
			continue
		}

		key := cleanRecoveredValue(token.Text)
		lowerKey := strings.ToLower(key)
		nextIsOpen := i+1 < len(tokens) && tokens[i+1].Kind == missionTokenOpenBrace
		role := classifySemanticHeader(tokens, i, stack, seenModes, lastMode)
		if role == semanticUnknown {
			i++
			continue
		}

		switch role {
		case semanticMission:
			if seenMission && len(campaign.Chapters) > 0 {
				i = len(tokens)
				continue
			}
			stack = closeFramesForRole(stack, semanticMission, token, &issues)
			seenMission = true
			seenModes = false
			lastMode = modeHint{}
			lastMapCode = ""
			looseChapterTitle = ""
		case semanticModes:
			if !seenMission {
				seenMission = true
				issues = append(issues, missionRecoveryIssue{
					Line:   token.Line,
					Column: token.Column,
					Kind:   "missing_mission",
					Detail: "inferred a mission root from the modes block",
				})
			}
			stack = closeFramesForRole(stack, semanticModes, token, &issues)
			seenModes = true
			lastMapCode = ""
			looseChapterTitle = ""
		case semanticMode:
			stack = closeFramesForRole(stack, semanticMode, token, &issues)
			if !seenModes {
				seenModes = true
				issues = append(issues, missionRecoveryIssue{
					Line:   token.Line,
					Column: token.Column,
					Kind:   "missing_modes",
					Detail: "inferred a modes block from a mode-shaped section",
				})
			}
			lastMode = modeHint{
				key:    strings.ToLower(key),
				line:   token.Line,
				indent: token.Indent,
				valid:  key != "",
			}
			lastMapCode = ""
			looseChapterTitle = ""
		case semanticChapter:
			stack = closeFramesForRole(stack, semanticChapter, token, &issues)
			if nearestFrame(stack, semanticMode) < 0 && lastMode.valid {
				stack = append(stack, recoveryFrame{
					role:   semanticMode,
					key:    lastMode.key,
					line:   lastMode.line,
					indent: lastMode.indent,
				})
				issues = append(issues, missionRecoveryIssue{
					Line:   token.Line,
					Column: token.Column,
					Kind:   "extra_close_brace",
					Detail: fmt.Sprintf("restored mode %q after premature closing braces", lastMode.key),
				})
			}
			lastMapCode = ""
			looseChapterTitle = ""
		}

		frame := recoveryFrame{
			role:     role,
			key:      lowerKey,
			line:     token.Line,
			indent:   token.Indent,
			physical: nextIsOpen,
		}
		stack = append(stack, frame)
		if !nextIsOpen {
			issues = append(issues, missionRecoveryIssue{
				Line:   token.Line,
				Column: token.Column,
				Kind:   "missing_open_brace",
				Detail: fmt.Sprintf("inferred an opening brace for %q", key),
			})
			i++
		} else {
			i += 2
		}
	}

	for _, frame := range stack {
		if !frame.physical || frame.role == semanticTransparent {
			continue
		}
		issues = append(issues, missionRecoveryIssue{
			Line:   frame.line,
			Column: 1,
			Kind:   "missing_close_brace",
			Detail: fmt.Sprintf("closed %q at the end of the mission file", frame.key),
		})
	}

	if !seenMission && !seenModes && len(campaign.Chapters) == 0 {
		return nil, issues
	}
	return campaign, issues
}

func classifySemanticHeader(
	tokens []missionToken,
	index int,
	stack []recoveryFrame,
	seenModes bool,
	lastMode modeHint,
) semanticRole {
	token := tokens[index]
	key := strings.ToLower(cleanRecoveredValue(token.Text))
	if key == "" {
		return semanticUnknown
	}
	if key == "mission" {
		return semanticMission
	}
	if key == "modes" {
		return semanticModes
	}

	nextIsOpen := index+1 < len(tokens) && tokens[index+1].Kind == missionTokenOpenBrace
	currentRole := nearestSemanticRole(stack)
	if isNumericKey(key) {
		if currentRole == semanticMode || currentRole == semanticChapter || seenModes || lastMode.valid {
			return semanticChapter
		}
		return semanticUnknown
	}

	switch currentRole {
	case semanticModes:
		if nextIsOpen || looksLikeMissingModeOpen(tokens, index) {
			return semanticMode
		}
	case semanticMode:
		modeIndex := nearestFrame(stack, semanticMode)
		if modeIndex >= 0 &&
			token.Indent <= stack[modeIndex].indent &&
			nextIsOpen &&
			blockLooksLikeMode(tokens, index+1) {
			return semanticMode
		}
		if nextIsOpen && blockContainsMapField(tokens, index+1) {
			return semanticChapter
		}
	case semanticChapter:
		modeIndex := nearestFrame(stack, semanticMode)
		if modeIndex >= 0 && token.Indent <= stack[modeIndex].indent {
			return semanticMode
		}
		if nextIsOpen && blockLooksLikeMode(tokens, index+1) {
			return semanticMode
		}
		if nextIsOpen && blockContainsMapField(tokens, index+1) {
			return semanticChapter
		}
	default:
		if seenModes && nextIsOpen && blockLooksLikeMode(tokens, index+1) {
			return semanticMode
		}
	}

	return semanticUnknown
}

func looksLikeMissingModeOpen(tokens []missionToken, index int) bool {
	for i := index + 1; i < len(tokens) && i <= index+4; i++ {
		if tokens[i].Kind != missionTokenScalar {
			continue
		}
		key := strings.ToLower(cleanRecoveredValue(tokens[i].Text))
		if isNumericKey(key) || key == "map" {
			return true
		}
		if _, _, ok := splitMissionFieldToken(tokens[i].Text); ok {
			return false
		}
	}
	return false
}

func blockContainsMapField(tokens []missionToken, openIndex int) bool {
	depth := 0
	for i := openIndex; i < len(tokens) && i <= openIndex+96; i++ {
		switch tokens[i].Kind {
		case missionTokenOpenBrace:
			depth++
		case missionTokenCloseBrace:
			depth--
			if depth <= 0 {
				return false
			}
		case missionTokenScalar:
			if depth == 1 {
				name, _, ok := splitMissionFieldToken(tokens[i].Text)
				if ok && name == "map" {
					return true
				}
			}
		}
	}
	return false
}

func blockLooksLikeMode(tokens []missionToken, openIndex int) bool {
	depth := 0
	for i := openIndex; i < len(tokens) && i <= openIndex+128; i++ {
		switch tokens[i].Kind {
		case missionTokenOpenBrace:
			depth++
		case missionTokenCloseBrace:
			depth--
			if depth <= 0 {
				return false
			}
		case missionTokenScalar:
			if depth == 1 && isNumericKey(strings.ToLower(cleanRecoveredValue(tokens[i].Text))) {
				return true
			}
		}
	}
	return false
}

func closeFramesForRole(
	stack []recoveryFrame,
	role semanticRole,
	token missionToken,
	issues *[]missionRecoveryIssue,
) []recoveryFrame {
	targetRole := semanticUnknown
	switch role {
	case semanticModes:
		targetRole = semanticMission
	case semanticMode:
		targetRole = semanticModes
	case semanticChapter:
		targetRole = semanticMode
	case semanticMission:
		targetRole = semanticUnknown
	}

	if targetRole == semanticUnknown {
		for len(stack) > 0 {
			stack = inferCloseTopFrame(stack, token, issues)
		}
		return stack
	}

	targetIndex := nearestFrame(stack, targetRole)
	if targetIndex < 0 {
		return stack
	}
	for len(stack)-1 > targetIndex {
		stack = inferCloseTopFrame(stack, token, issues)
	}
	return stack
}

func inferCloseTopFrame(
	stack []recoveryFrame,
	token missionToken,
	issues *[]missionRecoveryIssue,
) []recoveryFrame {
	if len(stack) == 0 {
		return stack
	}
	frame := stack[len(stack)-1]
	stack = stack[:len(stack)-1]
	if frame.physical && frame.role != semanticTransparent {
		*issues = append(*issues, missionRecoveryIssue{
			Line:   token.Line,
			Column: token.Column,
			Kind:   "missing_close_brace",
			Detail: fmt.Sprintf("closed %q before the next semantic section", frame.key),
		})
	}
	return stack
}

func nearestSemanticRole(stack []recoveryFrame) semanticRole {
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i].role {
		case semanticTransparent, semanticUnknown:
			continue
		default:
			return stack[i].role
		}
	}
	return semanticUnknown
}

func nearestFrame(stack []recoveryFrame, role semanticRole) int {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].role == role {
			return i
		}
	}
	return -1
}

func recoveredMode(stack []recoveryFrame, lastMode modeHint, token missionToken) string {
	if index := nearestFrame(stack, semanticMode); index >= 0 {
		return stack[index].key
	}
	if lastMode.valid && (token.Indent > lastMode.indent || token.Line == lastMode.line) {
		return lastMode.key
	}
	return ""
}

func setRecoveredChapterTitle(campaign *Campaign, code, title string) {
	if campaign == nil || code == "" || title == "" {
		return
	}
	for _, chapter := range campaign.Chapters {
		if chapter.Code == code && chapter.Title == "" {
			chapter.Title = title
			return
		}
	}
}

func isNumericKey(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func sourcePosition(data []byte, offset int64) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}

	line := 1
	column := 1
	for _, r := range string(data[:offset]) {
		if r == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return line, column
}

func formatRecoveryIssues(issues []missionRecoveryIssue) string {
	if len(issues) == 0 {
		return "semantic recovery used"
	}

	const maxIssues = 20
	limit := len(issues)
	if limit > maxIssues {
		limit = maxIssues
	}

	parts := make([]string, 0, limit+1)
	for _, issue := range issues[:limit] {
		location := ""
		if issue.Line > 0 {
			location = fmt.Sprintf("line %d", issue.Line)
			if issue.Column > 0 {
				location += fmt.Sprintf(":%d", issue.Column)
			}
			location += " "
		}
		parts = append(parts, fmt.Sprintf("%s%s: %s", location, issue.Kind, issue.Detail))
	}
	if len(issues) > limit {
		parts = append(parts, fmt.Sprintf("and %d more issue(s)", len(issues)-limit))
	}
	return strings.Join(parts, "; ")
}
