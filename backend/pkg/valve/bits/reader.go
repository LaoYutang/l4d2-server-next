package bits

import (
	"bufio"
	"fmt"
	"io"
	"math"
)

func NewReader(r io.Reader) *Reader {
	if br, ok := r.(io.ByteReader); ok {
		return &Reader{r: br}
	}

	return &Reader{r: bufio.NewReader(r)}
}

type Reader struct {
	r      io.ByteReader
	buffer uint8
	bits   uint8
}

func (r *Reader) ReadBits(bits int) (uint64, error) {
	if bits > 64 {
		panic("bits: cannot read more than 64 bits with this function")
	}

	var x uint64
	off := 0
	for {
		remaining := bits - off
		if int(r.bits) >= remaining {
			mask := uint8(1)<<remaining - 1
			x |= uint64(r.buffer&mask) << off
			r.buffer >>= remaining
			r.bits -= uint8(remaining)
			return x, nil
		}

		x |= uint64(r.buffer) << off
		off += int(r.bits)

		var err error
		r.buffer, err = r.r.ReadByte()
		if err != nil {
			r.bits = 0
			return x, err
		}

		r.bits = 8
	}
}

func (r *Reader) ReadString() (string, error) {
	var b []byte

	for {
		c, err := r.ReadUint8()
		if err != nil || c == 0 {
			return string(b), err
		}

		b = append(b, c)
	}
}

func (r *Reader) ReadWString() ([]uint16, error) {
	var b []uint16

	for {
		c, err := r.ReadUint16()
		if err != nil || c == 0 {
			return b, err
		}

		b = append(b, c)
	}
}

func (r *Reader) Read(b []byte) (int, error) {
	for i := range b {
		c, err := r.ReadUint8()
		if err != nil {
			return i, err
		}

		b[i] = c
	}

	return len(b), nil
}

func (r *Reader) ReadSlice(b []byte, bits int) error {
	if len(b)*8 < bits {
		return fmt.Errorf("bits: a %d byte slice cannot hold %d bits", len(b), bits)
	}

	for i := 0; bits > 0; i++ {
		now := bits
		if now > 8 {
			now = 8
		}

		x, err := r.ReadBits(now)
		if err != nil {
			return err
		}

		b[i] = byte(x)
		bits -= now
	}

	return nil
}

func (r *Reader) ReadBitCoord(intBits, fracBits int) (float32, error) {
	intFrac, err := r.ReadBits(2)
	if err != nil || intFrac == 0 {
		return 0, err
	}

	isNegative, err := r.ReadBool()
	if err != nil {
		return 0, err
	}

	var integerPart, fractionPart uint64

	if intFrac&1 != 0 {
		integerPart, err = r.ReadBits(intBits)
		if err != nil {
			return 0, err
		}

		integerPart++
	}
	if intFrac&2 != 0 {
		fractionPart, err = r.ReadBits(fracBits)
		if err != nil {
			return 0, err
		}
	}

	x := float32(integerPart) + float32(fractionPart)/float32(uint64(1)<<fracBits)
	if isNegative {
		return -x, nil
	}

	return x, nil
}

func (r *Reader) ReadBitVec3Coord(intBits, fracBits int) ([3]float32, error) {
	nonZero, err := r.ReadBits(3)
	if err != nil || nonZero == 0 {
		return [3]float32{}, err
	}

	var coord [3]float32
	for i := range coord {
		if nonZero&1 != 0 {
			coord[i], err = r.ReadBitCoord(intBits, fracBits)
			if err != nil {
				return coord, err
			}
		}

		nonZero >>= 1
	}

	return coord, nil
}

func (r *Reader) ReadBitNormal(bits int) (float32, error) {
	isNegative, err := r.ReadBool()
	if err != nil {
		return 0, err
	}

	x, err := r.ReadBits(bits)
	if err != nil {
		return 0, err
	}

	normal := float32(x) / float32(uint64(1)<<bits-1)
	if isNegative {
		return -normal, nil
	}

	return normal, nil
}

func (r *Reader) ReadBitVec3Normal(bits int) ([3]float32, error) {
	nonZero, err := r.ReadBits(2)
	if err != nil {
		return [3]float32{0, 0, 1}, err
	}

	var normal [3]float32
	if nonZero&1 != 0 {
		normal[0], err = r.ReadBitNormal(bits)
		if err != nil {
			return [3]float32{0, 0, 1}, err
		}
	}

	if nonZero&2 != 0 {
		normal[1], err = r.ReadBitNormal(bits)
		if err != nil {
			return [3]float32{0, 0, 1}, err
		}
	}

	negativeZ, err := r.ReadBool()
	if err != nil {
		return [3]float32{0, 0, 1}, err
	}

	zSquared := 1 - normal[0]*normal[0] + normal[1]*normal[1]
	if zSquared > 0 {
		normal[2] = float32(math.Sqrt(float64(zSquared)))
	}

	if negativeZ {
		normal[2] = -normal[2]
	}

	return normal, nil
}

func (r *Reader) ReadBitAngle(bits int) (float32, error) {
	// note: combined pitch/yaw/roll uses ReadBitVec3Coord in Source Engine

	x, err := r.ReadBits(bits)
	if err != nil {
		return 0, err
	}

	return float32(x) * 360 / float32(uint64(1)<<bits), nil
}

func (r *Reader) ReadUBitVar() (uint32, error) {
	x, err := r.ReadBits(6)
	if err != nil {
		return uint32(x), err
	}

	extra := x >> 4
	x &= 15

	var y uint64
	switch extra {
	case 1:
		y, err = r.ReadBits(4)
	case 2:
		y, err = r.ReadBits(8)
	case 3:
		y, err = r.ReadBits(32 - 4)
	}

	return uint32(x | y<<4), err
}
