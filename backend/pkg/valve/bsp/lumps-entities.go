package bsp

import (
	"bytes"
	"fmt"
	"io"
)

type Entity struct {
	Pairs []EPair
}

type EPair struct {
	Key   string
	Value string
}

func parseMapToken(data []byte) (token string, rest []byte) {
	if len(data) == 0 {
		return "", nil
	}

skipspace:
	// skip spaces
	for data[0] <= ' ' {
		if data[0] == 0 || len(data) == 1 {
			return "", nil
		}

		data = data[1:]
	}

	// skip C++ style comments
	if data[0] == '/' && data[1] == '/' {
		for data[0] != 0 && data[0] != '\n' {
			data = data[1:]
		}
		goto skipspace
	}

	// handle quoted strings specially
	if data[0] == '"' {
		data = data[1:]

		i := bytes.IndexByte(data, '"')
		if i == -1 {
			i = len(data)
		}

		return string(data[:i]), data[i+1:]
	}

	// parse single characters
	if data[0] == '{' || data[0] == '}' || data[0] == '(' || data[0] == ')' || data[0] == '\'' {
		return string(data[:1]), data[1:]
	}

	// parse a regular word
	i := bytes.IndexAny(data, "\x00\x01\x02\x03\x04\x05\x06\x07\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f {}()'")
	if i == -1 {
		i = len(data)
	}

	return string(data[:i]), data[i:]
}

func (f *File) loadEntitiesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_ENTITIES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	data := make([]byte, l.FileLen)
	_, err := r.ReadAt(data, int64(l.FileOfs))
	if err != nil {
		return err
	}

	token, data := parseMapToken(data)

	for len(data) != 0 {
		if token != "{" {
			return fmt.Errorf("expected \"{\", got %q", token)
		}

		var entity Entity

		token, data = parseMapToken(data)

		for token != "}" {
			key := token
			token, data = parseMapToken(data)
			value := token

			entity.Pairs = append(entity.Pairs, EPair{
				Key:   key,
				Value: value,
			})
		}

		f.Entities = append(f.Entities, entity)

		token, data = parseMapToken(data)
	}

	return nil
}
