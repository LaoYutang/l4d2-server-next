package vdf

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ReadTolerant parses Valve KeyValues while accepting common mission-file
// mistakes such as missing or extra trailing close braces. The regular ReadFrom
// method remains strict and unchanged for callers that need exact VDF validation.
func ReadTolerant(r io.Reader) (*KeyValues, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("vdf: reading tolerant input: %w", err)
	}

	var kv KeyValues
	text := string(data)
	_, parseErr := kv.ReadFrom(strings.NewReader(text))
	if parseErr == nil {
		return &kv, nil
	}

	if repaired, changed := removeTrailingExtraCloseBraces(text); changed {
		kv = KeyValues{}
		if _, err := kv.ReadFrom(strings.NewReader(repaired)); err == nil {
			return &kv, nil
		}
	}

	if !errors.Is(parseErr, io.ErrUnexpectedEOF) {
		return nil, parseErr
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

// removeTrailingExtraCloseBraces drops surplus close-brace tokens only when the
// remainder of the file contains no data other than close braces and comments.
func removeTrailingExtraCloseBraces(text string) (string, bool) {
	reader := bufio.NewReader(strings.NewReader(text))
	var repaired strings.Builder
	repaired.Grow(len(text))

	depth := 0
	sawOpenBrace := false
	foundExtraClose := false

	for {
		raw, token, err := ReadToken(reader)
		if raw != "" {
			if foundExtraClose {
				switch token {
				case TokenSpace, TokenComment:
					repaired.WriteString(raw)
				case TokenCloseBrace:
					// Drop trailing extra close braces.
				default:
					return text, false
				}
			} else {
				switch token {
				case TokenOpenBrace:
					sawOpenBrace = true
					depth++
					repaired.WriteString(raw)
				case TokenCloseBrace:
					if depth == 0 {
						foundExtraClose = true
					} else {
						depth--
						repaired.WriteString(raw)
					}
				default:
					repaired.WriteString(raw)
				}
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return text, false
		}
	}

	if !sawOpenBrace || depth != 0 || !foundExtraClose {
		return text, false
	}

	return repaired.String(), true
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
