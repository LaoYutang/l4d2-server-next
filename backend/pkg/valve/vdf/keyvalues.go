// Package vdf implements a parser and serializer for Valve Data Format.
package vdf

import (
	"bytes"
	"fmt"
)

type KeyValues struct {
	Key      string
	Value    string
	Cond     string
	HasValue bool

	sub  *KeyValues
	peer *KeyValues
}

var _ fmt.Stringer = (*KeyValues)(nil)

func (kv *KeyValues) String() string {
	solo := *kv
	solo.peer = nil

	var buf bytes.Buffer

	_, err := solo.WriteTo(&buf)
	if err != nil {
		return "<" + err.Error() + ">"
	}

	return buf.String()[:buf.Len()-1]
}

func (kv *KeyValues) AddSubKey(child *KeyValues) {
	c := &kv.sub
	for *c != nil {
		c = &(*c).peer
	}

	*c = child
}
