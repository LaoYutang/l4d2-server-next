//go:generate go run golang.org/x/tools/cmd/stringer -type Token -trimprefix Token

package vdf

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"unicode"
)

// Token is an enumeration of the types of syntax token in a VDF file.
type Token uint8

const (
	TokenSpace Token = iota
	TokenComment
	TokenString
	TokenQuoted
	TokenCond
	TokenOpenBrace
	TokenCloseBrace
)

// ReadToken consumes and returns the next token in the reader.
//
// The returned string is verbatim; no processing is done other than
// determining its length and token type.
//
// Input to this function is expected to be UTF-8 encoded.
func ReadToken(r *bufio.Reader) (string, Token, error) {
	ch, _, err := r.ReadRune()
	if err != nil {
		return "", TokenSpace, err
	}

	buf := make([]rune, 1, 4096)
	buf[0] = ch

	if unicode.IsSpace(ch) {
		for {
			ch, _, err = r.ReadRune()
			if errors.Is(err, io.EOF) {
				return string(buf), TokenSpace, nil
			}

			if err != nil {
				return string(buf), TokenSpace, err
			}

			if !unicode.IsSpace(ch) {
				err = r.UnreadRune()

				return string(buf), TokenSpace, err
			}

			buf = append(buf, ch)
		}
	}

	switch ch {
	case '/':
		ch, _, err = r.ReadRune()
		if err == nil && ch == '/' {
			s, err := r.ReadString('\n')

			return "//" + s, TokenComment, err
		}

		err = r.UnreadRune()
		if err != nil {
			return "/", TokenString, err
		}

		ch = '/'
	case '"':
		for {
			ch, _, err = r.ReadRune()
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}

			if err != nil {
				return string(buf), TokenQuoted, err
			}

			buf = append(buf, ch)

			if ch == '\\' {
				ch, _, err = r.ReadRune()
				if errors.Is(err, io.EOF) {
					err = io.ErrUnexpectedEOF
				}

				if err != nil {
					return string(buf), TokenQuoted, err
				}

				buf = append(buf, ch)

				continue
			}

			if ch == '"' {
				return string(buf), TokenQuoted, nil
			}
		}
	case '{':
		return "{", TokenOpenBrace, nil
	case '}':
		return "}", TokenCloseBrace, nil
	}

	condStart, tokenType := ch == '[', TokenString

	for {
		ch, _, err = r.ReadRune()
		if errors.Is(err, io.EOF) {
			err = nil

			break
		}

		if err != nil {
			break
		}

		if unicode.IsSpace(ch) {
			err = r.UnreadRune()

			break
		}

		if ch == '"' || ch == '{' || ch == '}' {
			err = r.UnreadRune()

			break
		}

		buf = append(buf, ch)

		if ch == '[' {
			condStart = true
		}

		if ch == ']' && condStart {
			tokenType = TokenCond
		}
	}

	return string(buf), tokenType, err
}

var (
	Escape = strings.NewReplacer(
		"\\", "\\\\",
		"\n", "\\n",
		"\t", "\\t",
		"\v", "\\v",
		"\b", "\\b",
		"\r", "\\r",
		"\f", "\\f",
		"\a", "\\a",
		"\"", "\\\"",
	)

	Unescape = strings.NewReplacer(
		"\\n", "\n",
		"\\t", "\t",
		"\\v", "\v",
		"\\b", "\b",
		"\\r", "\r",
		"\\f", "\f",
		"\\a", "\a",
		"\\\"", "\"",
		"\\'", "'",
		"\\?", "?",
		"\\\\", "\\",
	)
)
