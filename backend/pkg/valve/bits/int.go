package bits

import (
	"encoding/binary"
	"fmt"
	"math"
)

func (r *Reader) ReadBool() (bool, error) {
	n, err := r.ReadBits(1)
	return n != 0, err
}
func (r *Reader) ReadByte() (byte, error) {
	// implements io.ByteReader
	return r.ReadUint8()
}
func (r *Reader) ReadUint8() (uint8, error) {
	n, err := r.ReadBits(8)
	return uint8(n), err
}
func (r *Reader) ReadInt8() (int8, error) {
	n, err := r.ReadSigned(8)
	return int8(n), err
}
func (r *Reader) ReadUint16() (uint16, error) {
	n, err := r.ReadBits(16)
	return uint16(n), err
}
func (r *Reader) ReadInt16() (int16, error) {
	n, err := r.ReadSigned(16)
	return int16(n), err
}
func (r *Reader) ReadUint32() (uint32, error) {
	n, err := r.ReadBits(32)
	return uint32(n), err
}
func (r *Reader) ReadInt32() (int32, error) {
	n, err := r.ReadSigned(32)
	return int32(n), err
}
func (r *Reader) ReadVarInt32() (uint32, error) {
	n, err := binary.ReadUvarint(r)
	if err == nil && n > math.MaxUint32 {
		err = fmt.Errorf("%d overflows uint32", n)
	}
	return uint32(n), err
}
func (r *Reader) ReadSignedVarInt32() (int32, error) {
	n, err := binary.ReadVarint(r)
	if err == nil && (n < math.MinInt32 || n > math.MaxInt32) {
		err = fmt.Errorf("%d overflows int32", n)
	}
	return int32(n), err
}
func (r *Reader) ReadUint64() (uint64, error) {
	n, err := r.ReadBits(64)
	return uint64(n), err
}
func (r *Reader) ReadInt64() (int64, error) {
	n, err := r.ReadSigned(64)
	return int64(n), err
}
func (r *Reader) ReadVarInt64() (uint64, error) {
	return binary.ReadUvarint(r)
}
func (r *Reader) ReadSignedVarInt64() (int64, error) {
	return binary.ReadVarint(r)
}
func (r *Reader) ReadSigned(bits int) (int64, error) {
	n, err := r.ReadBits(bits)

	s := uint64(1) << (bits - 1)
	if n >= s {
		// sign-extend by subtracting the sign bit twice
		n -= s
		n -= s
	}

	return int64(n), err
}

func (w *Writer) WriteBool(b bool) error {
	if b {
		return w.WriteBits(1, 1)
	}

	return w.WriteBits(0, 1)
}
func (w *Writer) WriteUint8(n uint8) error {
	return w.WriteBits(uint64(n), 8)
}
func (w *Writer) WriteInt8(n int8) error {
	return w.WriteSigned(int64(n), 8)
}
func (w *Writer) WriteUint16(n uint16) error {
	return w.WriteBits(uint64(n), 16)
}
func (w *Writer) WriteInt16(n int16) error {
	return w.WriteSigned(int64(n), 16)
}
func (w *Writer) WriteUint32(n uint32) error {
	return w.WriteBits(uint64(n), 32)
}
func (w *Writer) WriteInt32(n int32) error {
	return w.WriteSigned(int64(n), 32)
}
func (w *Writer) WriteVarInt32(n uint32) error {
	var buf [binary.MaxVarintLen32]byte
	i := binary.PutUvarint(buf[:], uint64(n))
	_, err := w.Write(buf[:i])
	return err
}
func (w *Writer) WriteUint64(n uint64) error {
	return w.WriteBits(n, 64)
}
func (w *Writer) WriteInt64(n int64) error {
	return w.WriteSigned(int64(n), 64)
}
func (w *Writer) WriteVarInt64(n uint64) error {
	var buf [binary.MaxVarintLen64]byte
	i := binary.PutUvarint(buf[:], n)
	_, err := w.Write(buf[:i])
	return err
}
func (w *Writer) WriteSigned(n int64, bits int) error {
	return w.WriteBits(uint64(n), bits)
}
