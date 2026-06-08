package bits

import (
	"encoding/binary"
	"math"
)

// I think these are the same because we're little endian?
// Source Engine uses them as if they're different.

func (r *Reader) ReadFloat32() (float32, error) {
	var b [4]byte

	_, err := r.Read(b[:])

	n := binary.LittleEndian.Uint32(b[:])

	return math.Float32frombits(n), err
}

func (r *Reader) ReadBitFloat32() (float32, error) {
	n, err := r.ReadUint32()
	return math.Float32frombits(n), err
}

func (r *Reader) ReadFloat64() (float64, error) {
	var b [8]byte

	_, err := r.Read(b[:])

	n := binary.LittleEndian.Uint64(b[:])

	return math.Float64frombits(n), err
}

func (r *Reader) ReadBitFloat64() (float64, error) {
	n, err := r.ReadUint64()
	return math.Float64frombits(n), err
}

func (w *Writer) WriteFloat32(f float32) error {
	n := math.Float32bits(f)

	var b [4]byte

	binary.LittleEndian.PutUint32(b[:], n)

	_, err := w.Write(b[:])

	return err
}

func (w *Writer) WriteBitFloat32(f float32) error {
	return w.WriteUint32(math.Float32bits(f))
}

func (w *Writer) WriteFloat64(f float64) error {
	n := math.Float64bits(f)

	var b [8]byte

	binary.LittleEndian.PutUint64(b[:], n)

	_, err := w.Write(b[:])

	return err
}

func (w *Writer) WriteBitFloat64(f float64) error {
	return w.WriteUint64(math.Float64bits(f))
}
