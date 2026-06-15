package vdf

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ReadTolerant parses Valve KeyValues while accepting common mission-file
// mistakes such as missing trailing close braces. The regular ReadFrom method
// remains strict and unchanged for callers that need exact VDF validation.
func ReadTolerant(r io.Reader) (*KeyValues, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("vdf: reading tolerant input: %w", err)
	}

	var kv KeyValues
	text := string(data)
	if _, err := kv.ReadFrom(strings.NewReader(text)); err == nil {
		return &kv, nil
	} else if !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}

	candidates := []string{text}
	if repaired, changed := closeLineBrokenQuotes(text); changed {
		candidates = append(candidates, repaired)
	}

	for _, candidate := range candidates {
		kv = KeyValues{}
		if _, err := kv.ReadFrom(strings.NewReader(candidate)); err == nil {
			return &kv, nil
		}

		missingBraces := missingCloseBraceCount(candidate)
		if missingBraces <= 0 {
			continue
		}

		repaired := candidate + strings.Repeat("\n}", missingBraces)
		kv = KeyValues{}
		if _, err := kv.ReadFrom(strings.NewReader(repaired)); err == nil {
			return &kv, nil
		}
	}

	return nil, io.ErrUnexpectedEOF
}

func missingCloseBraceCount(text string) int {
	depth := 0
	reader := bufio.NewReader(strings.NewReader(text))
	for {
		_, token, err := ReadToken(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return depth
			}
			return 0
		}
		switch token {
		case TokenOpenBrace:
			depth++
		case TokenCloseBrace:
			depth--
			if depth < 0 {
				return 0
			}
		}
	}
}

func closeLineBrokenQuotes(text string) (string, bool) {
	var out strings.Builder
	changed := false

	for len(text) > 0 {
		lineEnd := strings.IndexByte(text, '\n')
		line := text
		newline := ""
		if lineEnd >= 0 {
			line = text[:lineEnd]
			newline = "\n"
			text = text[lineEnd+1:]
		} else {
			text = ""
		}

		if strings.HasSuffix(line, "\r") {
			line = strings.TrimSuffix(line, "\r")
			newline = "\r" + newline
		}

		out.WriteString(line)
		if hasOddUnescapedQuotes(line) {
			out.WriteByte('"')
			changed = true
		}
		out.WriteString(newline)
	}

	return out.String(), changed
}

func hasOddUnescapedQuotes(line string) bool {
	quotes := 0
	escaped := false
	for _, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			quotes++
		}
	}
	return quotes%2 == 1
}
