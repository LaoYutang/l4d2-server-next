package bits

import (
	"fmt"
	"io"
	"math"
	"unicode/utf16"
)

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

type Writer struct {
	w      io.Writer
	buffer [1]uint8
	bits   uint8
}

func (w *Writer) WriteBits(x uint64, bits int) error {
	if bits > 64 {
		panic("bits: cannot write more than 64 bits with this function")
	}

	// make sure there aren't any bits above what we're writing set
	x &= 1<<bits - 1

	// first try to get to a byte boundary
	if w.bits != 0 {
		w.buffer[0] |= uint8(x) << w.bits

		if int(w.bits)+bits < 8 {
			w.bits += uint8(bits)

			// and we're done
			return nil
		}

		bits -= (8 - int(w.bits))
		x >>= 8 - w.bits
		w.bits = 0

		_, err := w.w.Write(w.buffer[:])
		w.buffer[0] = 0
		if err != nil {
			return err
		}
	}

	// chop off bytes
	for bits >= 8 {
		w.buffer[0] = uint8(x)
		x >>= 8
		bits -= 8

		_, err := w.w.Write(w.buffer[:])
		if err != nil {
			return err
		}
	}

	// shove the rest in the buffer
	w.buffer[0] = uint8(x)
	w.bits = uint8(bits)

	return nil
}

func (w *Writer) WriteString(s string) error {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return fmt.Errorf("bits: cannot write string that contains U+0000: %q", s)
		}
	}

	_, err := w.Write([]byte(s + "\x00"))

	return err
}

func (w *Writer) WriteWString(s []uint16) error {
	for _, c := range s {
		if c == 0 {
			return fmt.Errorf("bits: cannot write string that contains U+0000: %q", string(utf16.Decode(s)))
		}
	}

	for _, c := range s {
		err := w.WriteUint16(c)
		if err != nil {
			return err
		}
	}

	return w.WriteUint16(0)
}

func (w *Writer) Write(b []byte) (int, error) {
	for i, c := range b {
		err := w.WriteUint8(c)
		if err != nil {
			return i, err
		}
	}

	return len(b), nil
}

func (w *Writer) WriteSlice(b []byte, bits int) error {
	if len(b)*8 < bits {
		return fmt.Errorf("bits: a %d byte slice cannot hold %d bits", len(b), bits)
	}

	i := 0
	for bits >= 8 {
		err := w.WriteUint8(b[i])
		if err != nil {
			return err
		}

		i++
		bits -= 8
	}

	if bits > 0 {
		return w.WriteBits(uint64(b[i]), bits)
	}

	return nil
}

func (w *Writer) WriteBitCoord(coord float32, intBits, fracBits int) error {
	isNegative := coord <= -1.0/float32(uint64(1)<<fracBits)

	intPart, fracPart := math.Modf(math.Abs(float64(coord)))

	intPartInt := uint64(intPart)
	fracPartInt := uint64(fracPart * float64(uint64(1)<<fracBits))

	err := w.WriteBool(intPartInt != 0)
	if err != nil {
		return err
	}

	err = w.WriteBool(fracPartInt != 0)
	if err != nil {
		return err
	}

	if intPartInt != 0 || fracPartInt != 0 {
		err = w.WriteBool(isNegative)
		if err != nil {
			return err
		}

		if intPartInt != 0 {
			err = w.WriteBits(intPartInt-1, intBits)
			if err != nil {
				return err
			}
		}

		if fracPartInt != 0 {
			err = w.WriteBits(fracPartInt, fracBits)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *Writer) WriteBitVec3Coord(coord [3]float32, intBits, fracBits int) error {
	min := 1.0 / float32(uint64(1)<<fracBits)
	for i := range coord {
		isZero := coord[i] > -min && coord[i] < min
		if isZero {
			coord[i] = 0
		}

		err := w.WriteBool(isZero)
		if err != nil {
			return err
		}
	}

	for i := range coord {
		if coord[i] != 0 {
			err := w.WriteBitCoord(coord[i], intBits, fracBits)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *Writer) WriteBitNormal(normal float32, bits int) error {
	normalBits := int64(normal * float32(uint64(1)<<bits-1))
	isNegative := normalBits < 0
	if isNegative {
		normalBits = -normalBits
	}

	err := w.WriteBool(isNegative)
	if err != nil {
		return err
	}

	err = w.WriteBits(uint64(normalBits), bits)
	if err != nil {
		return err
	}

	return nil
}

func (w *Writer) WriteBitVec3Normal(normal [3]float32, bits int) error {
	min := 1.0 / float32(uint64(1)<<bits-1)
	xNonZero := normal[0] <= -min || normal[0] >= min
	yNonZero := normal[1] <= -min || normal[1] >= min
	zNegative := normal[2] <= -min

	err := w.WriteBool(xNonZero)
	if err != nil {
		return err
	}

	err = w.WriteBool(yNonZero)
	if err != nil {
		return err
	}

	if xNonZero {
		err = w.WriteBitNormal(normal[0], bits)
		if err != nil {
			return err
		}
	}

	if yNonZero {
		err = w.WriteBitNormal(normal[1], bits)
		if err != nil {
			return err
		}
	}

	return w.WriteBool(zNegative)
}

func (w *Writer) WriteBitAngle(angle float32, bits int) error {
	// note: combined pitch/yaw/roll uses WriteBitVec3Coord in Source Engine

	_, angleMod := math.Modf(float64(angle / 360))

	return w.WriteBits(uint64(angleMod*float64(uint64(1)<<bits)), bits)
}

func (w *Writer) WriteUBitVar(n uint32) error {
	if n < 16 {
		return w.WriteBits(uint64(n), 6)
	}

	if n < 256 {
		return w.WriteBits(uint64((n&15)|16|(n&(128|64|32|16))<<2), 10)
	}

	if n < 4096 {
		return w.WriteBits(uint64((n&15)|32|((n&(2048|1024|512|256|128|64|32|16))<<2)), 14)
	}

	err := w.WriteBits(uint64(n&15)|48, 6)
	if err != nil {
		return err
	}

	return w.WriteBits(uint64(n>>4), 32-4)
}
