package kvpacker

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

func (d Data) MarshalBinary() ([]byte, error) {
	return d.appendBinary(make([]byte, 0, 1024))
}

func (d Data) appendBinary(b []byte) ([]byte, error) {
	var err error

	for i := range d {
		b, err = d[i].appendBinary(b)
		if err != nil {
			return nil, err
		}
	}

	b = append(b, byte(numTypes))

	return b, nil
}

func (v *Var) appendBinary(b []byte) ([]byte, error) {
	if strings.IndexByte(v.Name, 0) != -1 {
		return nil, fmt.Errorf("kvpacker: string contains null byte: %q", v.Name)
	}

	b = append(b, byte(v.Type))
	b = append(b, v.Name...)
	b = append(b, 0)

	switch v.Type {
	case None:
		var err error
		b, err = v.None.appendBinary(b)
		if err != nil {
			return nil, err
		}
	case String:
		if strings.IndexByte(v.String, 0) != -1 {
			return nil, fmt.Errorf("kvpacker: string contains null byte: %q", v.String)
		}

		b = append(b, v.String...)
		b = append(b, 0)
	case Int:
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], uint32(v.Int))
		b = append(b, buf[:]...)
	case Float:
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], math.Float32bits(v.Float))
		b = append(b, buf[:]...)
	case Ptr:
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], v.Ptr)
		b = append(b, buf[:]...)
	case WString:
		if len(v.WString) > math.MaxUint16 {
			return nil, fmt.Errorf("kvpacker: wide string too long: %d characters", len(v.WString))
		}
		var buf [2]byte
		binary.LittleEndian.PutUint16(buf[:], uint16(len(v.WString)))
		b = append(b, buf[:]...)

		for i := range v.WString {
			binary.LittleEndian.PutUint16(buf[:], v.WString[i])
			b = append(b, buf[:]...)
		}
	case Color:
		b = append(b, v.Color.R, v.Color.G, v.Color.B, v.Color.A)
	case Uint64:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], v.Uint64)
		b = append(b, buf[:]...)
	case CompiledIntByte:
		if v.Int < math.MinInt8 || v.Int > math.MaxInt8 {
			return nil, fmt.Errorf("kvpacker: int byte out of range: %d", v.Int)
		}
		b = append(b, byte(int8(v.Int)))
	case CompiledInt0:
		if v.Int != 0 {
			return nil, fmt.Errorf("kvpacker: int 0 out of range: %d", v.Int)
		}
	case CompiledInt1:
		if v.Int != 1 {
			return nil, fmt.Errorf("kvpacker: int 1 out of range: %d", v.Int)
		}
	default:
		return nil, fmt.Errorf("kvpacker: invalid type %v", v.Type)
	}

	return b, nil
}
