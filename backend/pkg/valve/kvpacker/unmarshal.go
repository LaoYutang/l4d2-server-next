package kvpacker

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

func (d *Data) UnmarshalBinary(b []byte) error {
	b, err := d.readBinary(b)
	if err != nil {
		return err
	}

	if len(b) == 0 {
		return nil
	}

	return fmt.Errorf("kvpacker: %d extra bytes after end of data", len(b))
}

func (d *Data) readBinary(b []byte) ([]byte, error) {
	var err error

	*d = nil

	for {
		if len(b) == 0 {
			return b, io.ErrUnexpectedEOF
		}

		if b[0] == byte(numTypes) {
			return b[1:], nil
		}

		var v Var
		b, err = v.readBinary(b)
		if err != nil {
			return b, err
		}

		*d = append(*d, v)
	}
}

func (v *Var) readBinary(b []byte) ([]byte, error) {
	if len(b) < 2 {
		return nil, io.ErrUnexpectedEOF
	}

	v.Type = Type(b[0])

	b = b[1:]
	i := bytes.IndexByte(b, 0)
	if i == -1 {
		return nil, fmt.Errorf("kvpacker: unterminated string")
	}

	v.Name = string(b[:i])
	b = b[i+1:]

	switch v.Type {
	case None:
		return v.None.readBinary(b)
	case String:
		i = bytes.IndexByte(b, 0)
		if i == -1 {
			return nil, fmt.Errorf("kvpacker: unterminated string")
		}

		v.String = string(b[:i])
		b = b[i+1:]
	case Int:
		if len(b) < 4 {
			return nil, io.ErrUnexpectedEOF
		}

		v.Int = int32(binary.LittleEndian.Uint32(b))
		b = b[4:]
	case Float:
		if len(b) < 4 {
			return nil, io.ErrUnexpectedEOF
		}

		v.Float = math.Float32frombits(binary.LittleEndian.Uint32(b))
		b = b[4:]
	case Ptr:
		if len(b) < 4 {
			return nil, io.ErrUnexpectedEOF
		}

		v.Ptr = binary.LittleEndian.Uint32(b)
		b = b[4:]
	case WString:
		if len(b) < 2 {
			return nil, io.ErrUnexpectedEOF
		}

		length := binary.LittleEndian.Uint16(b)
		if len(b) < 2+2*int(length) {
			return nil, io.ErrUnexpectedEOF
		}

		b = b[2:]
		v.WString = make([]uint16, length)

		for i := range v.WString {
			v.WString[i] = binary.LittleEndian.Uint16(b)
			b = b[2:]
		}
	case Color:
		if len(b) < 4 {
			return nil, io.ErrUnexpectedEOF
		}

		v.Color.R = b[0]
		v.Color.G = b[1]
		v.Color.B = b[2]
		v.Color.A = b[3]
		b = b[4:]
	case Uint64:
		if len(b) < 8 {
			return nil, io.ErrUnexpectedEOF
		}

		v.Uint64 = binary.LittleEndian.Uint64(b)
		b = b[8:]
	case CompiledIntByte:
		if len(b) < 1 {
			return nil, io.ErrUnexpectedEOF
		}

		v.Int = int32(int8(b[0]))
		b = b[1:]
	case CompiledInt0:
		v.Int = 0
	case CompiledInt1:
		v.Int = 1
	default:
		return nil, fmt.Errorf("kvpacker: invalid type %v", v.Type)
	}

	return b, nil
}
