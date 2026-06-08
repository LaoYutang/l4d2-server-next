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
	if _, err := kv.ReadFrom(strings.NewReader(string(data))); err == nil {
		return &kv, nil
	} else if !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}

	missingBraces := missingCloseBraceCount(data)
	if missingBraces <= 0 {
		return nil, io.ErrUnexpectedEOF
	}

	repaired := string(data) + strings.Repeat("\n}", missingBraces)
	kv = KeyValues{}
	if _, err := kv.ReadFrom(strings.NewReader(repaired)); err != nil {
		return nil, err
	}
	return &kv, nil
}

func missingCloseBraceCount(data []byte) int {
	depth := 0
	reader := bufio.NewReader(strings.NewReader(string(data)))
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
