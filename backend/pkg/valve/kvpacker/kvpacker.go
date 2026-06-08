//go:generate go run golang.org/x/tools/cmd/stringer -type Type

package kvpacker

import (
	"image/color"
)

type Type uint8

const (
	None            Type = iota // contains child key/value pairs
	String                      // utf-8 string (no 00 bytes)
	Int                         // 32-bit signed integer
	Float                       // 32-bit float
	Ptr                         // 32-bit pointer (deprecated)
	WString                     // utf-16 string
	Color                       // RGBA8888 color
	Uint64                      // 64-bit unsigned integer
	CompiledIntByte             // 8-bit signed integer
	CompiledInt0                // integer (can only be 0)
	CompiledInt1                // integer (can only be 1)
	numTypes
)

type Data []Var

type Var struct {
	Type Type
	Name string

	// The field that is set below is based on Type.
	None    Data
	String  string
	Int     int32
	Float   float32
	Ptr     uint32
	WString []uint16
	Color   color.RGBA
	Uint64  uint64
}
