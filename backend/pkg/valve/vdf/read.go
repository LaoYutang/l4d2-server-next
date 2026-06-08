package vdf

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

var _ io.ReaderFrom = (*KeyValues)(nil)

func (kv *KeyValues) ReadFrom(r io.Reader) (int64, error) {
	var n int64

	err := kv.read(bufio.NewReader(r), &n, true)

	return n, err
}

func (kv *KeyValues) read(r *bufio.Reader, total *int64, outermost bool) error {
	var target KeyValues

	var (
		s   string
		tok Token
		err error
	)

	next := func() bool {
		if err != nil {
			return false
		}

		s, tok, err = ReadToken(r)

		*total += int64(len(s))

		return err == nil
	}

	if outermost {
		for tok == TokenSpace || tok == TokenComment {
			if !next() {
				return err
			}
		}

		switch tok {
		case TokenQuoted:
			s = s[1 : len(s)-1]
			s = Unescape.Replace(s)
			fallthrough
		case TokenString:
			target.Key = s
		default:
			return fmt.Errorf("vdf: %v is invalid here", tok)
		}

		tok = TokenSpace
	} else {
		target.Key = kv.Key
	}

	for tok == TokenSpace || tok == TokenComment {
		if !next() {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}

			return err
		}
	}

	switch tok {
	case TokenOpenBrace:
		sub := &target.sub

		for {
			tok = TokenSpace

			for tok == TokenSpace || tok == TokenComment {
				if !next() {
					if errors.Is(err, io.EOF) {
						err = io.ErrUnexpectedEOF
					}

					return err
				}
			}

			if tok == TokenCloseBrace {
				break
			}

			switch tok {
			case TokenQuoted:
				s = s[1 : len(s)-1]
				s = Unescape.Replace(s)
				fallthrough
			case TokenString:
				*sub = &KeyValues{
					Key: s,
				}
			default:
				return fmt.Errorf("vdf: %v is invalid here", tok)
			}

			err = (*sub).read(r, total, false)
			if err != nil {
				return err
			}

			sub = &(*sub).peer
		}
	case TokenQuoted:
		s = s[1 : len(s)-1]
		s = Unescape.Replace(s)
		fallthrough
	case TokenString:
		target.Value, target.HasValue = s, true
	default:
		return fmt.Errorf("vdf: %v is invalid here", tok)
	}

	*kv = target

	if !outermost {
		return nil
	}

	peer := &kv.peer
	for {
		tok = TokenSpace
		for tok == TokenSpace || tok == TokenComment {
			if !next() {
				if errors.Is(err, io.EOF) {
					return nil
				}

				return err
			}
		}

		switch tok {
		case TokenQuoted:
			s = s[1 : len(s)-1]
			s = Unescape.Replace(s)
			fallthrough
		case TokenString:
			*peer = &KeyValues{
				Key: s,
			}
		default:
			return fmt.Errorf("vdf: %v is invalid here", tok)
		}

		err = (*peer).read(r, total, false)
		if err != nil {
			return err
		}

		peer = &(*peer).peer
	}
}
