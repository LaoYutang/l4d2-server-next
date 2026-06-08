// Package ice implements the Information Concealment Engine by Matthew Kwan.
//
// This package is derived from Matthew Kwan's public domain source code.
//
// https://www.darkside.com.au/ice/
package ice

import (
	"crypto/cipher"
	"fmt"
	"sync"
)

// Key implements crypto/cipher.Block. It is safe to use concurrently from
// multiple goroutines.
type Key struct {
	size     int32
	rounds   int32
	keysched [][3]uint32
}

var _ cipher.Block = (*Key)(nil)

var (
	// The S-boxes.
	sbox         [4][1024]uint32
	sboxInitOnce sync.Once

	// Modulo values for the S-boxes.
	smod = [4][4]uint32{
		{333, 313, 505, 369},
		{379, 375, 319, 391},
		{361, 445, 451, 397},
		{397, 425, 395, 505},
	}

	// XOR values for the S-boxes.
	sxor = [4][4]uint32{
		{0x83, 0x85, 0x9b, 0xcd},
		{0xcc, 0xa7, 0xad, 0x41},
		{0x4b, 0x2e, 0xd4, 0x33},
		{0xea, 0xcb, 0x2e, 0x04},
	}

	// Expanded permutation values for the P-box.
	pbox = [32]uint32{
		0x00000001, 0x00000080, 0x00000400, 0x00002000,
		0x00080000, 0x00200000, 0x01000000, 0x40000000,
		0x00000008, 0x00000020, 0x00000100, 0x00004000,
		0x00010000, 0x00800000, 0x04000000, 0x20000000,
		0x00000004, 0x00000010, 0x00000200, 0x00008000,
		0x00020000, 0x00400000, 0x08000000, 0x10000000,
		0x00000002, 0x00000040, 0x00000800, 0x00001000,
		0x00040000, 0x00100000, 0x02000000, 0x80000000,
	}

	// The key rotation schedule.
	keyrot = [16]int32{
		0, 1, 2, 3, 2, 1, 3, 0,
		1, 3, 2, 0, 3, 1, 0, 2,
	}
)

// Galois Field multiplication of a by b, modulo m.
// Just like arithmetic multiplication, except that additions and
// subtractions are replaced by XOR.
func gfMult(a, b, m uint32) uint32 {
	var res uint32

	for b != 0 {
		if b&1 != 0 {
			res ^= a
		}

		a <<= 1
		b >>= 1

		if a >= 256 {
			a ^= m
		}
	}

	return res
}

// Galois Field exponentiation.
// Raise the base to the power of 7, modulo m.
func gfExp7(b, m uint32) uint32 {
	if b == 0 {
		return 0
	}

	x := gfMult(b, b, m)
	x = gfMult(b, x, m)
	x = gfMult(x, x, m)
	x = gfMult(b, x, m)

	return x
}

// Carry out the ICE 32-bit P-box permutation.
func perm32(x uint32) uint32 {
	var res uint32

	i := 0

	for x != 0 {
		if x&1 != 0 {
			res |= pbox[i]
		}

		i++

		x >>= 1
	}

	return res
}

// Initialise the ICE S-boxes.
// This only has to be done once.
func sboxInit() {
	for i := uint32(0); i < 1024; i++ {
		col := (i >> 1) & 0xff
		row := (i & 0x1) | ((i & 0x200) >> 8)

		x := gfExp7(col^sxor[0][row], smod[0][row]) << 24
		sbox[0][i] = perm32(x)

		x = gfExp7(col^sxor[1][row], smod[1][row]) << 16
		sbox[1][i] = perm32(x)

		x = gfExp7(col^sxor[2][row], smod[2][row]) << 8
		sbox[2][i] = perm32(x)

		x = gfExp7(col^sxor[3][row], smod[3][row])
		sbox[3][i] = perm32(x)
	}
}

// Must panics if the error is non-nil.
func Must(ik *Key, err error) *Key {
	if err != nil {
		panic(err)
	}

	return ik
}

// New creates a new 64-bit ICE key.
func New(key string) (*Key, error) {
	if len(key) != 8 {
		return nil, fmt.Errorf("ice: invalid key length: %d but length must be 8", len(key))
	}

	sboxInitOnce.Do(sboxInit)

	ik := &Key{
		size:     1,
		rounds:   8,
		keysched: make([][3]uint32, 8),
	}

	var kb [4]uint16
	for i := range kb {
		kb[3-i] = uint16(key[i*2])<<8 | uint16(key[i*2+1])
	}

	ik.schedBuild(kb, 0, keyrot[:])

	return ik, nil
}

// NewMulti creates a new ICE key that is a multiple of 8 bytes.
func NewMulti(key string) (*Key, error) {
	if key == "" {
		return nil, fmt.Errorf("ice: key cannot be blank")
	}

	if len(key)%8 != 0 {
		return nil, fmt.Errorf("ice: invalid key length: %d is not a multiple of 8 bytes", len(key))
	}

	sboxInitOnce.Do(sboxInit)

	n := int32(len(key) / 8)
	ik := &Key{
		size:     n,
		rounds:   n * 16,
		keysched: make([][3]uint32, n*16),
	}

	for i := int32(0); i < n; i++ {
		var kb [4]uint16

		for j := int32(0); j < 4; j++ {
			kb[3-j] = uint16(key[i*8+j*2])<<8 | uint16(key[i*8+j*2+1])
		}

		ik.schedBuild(kb, i*8, keyrot[:])
		ik.schedBuild(kb, ik.rounds-8-i*8, keyrot[8:])
	}

	return ik, nil
}

// The single round ICE f function.
func icef(p uint32, sk [3]uint32) uint32 {
	var (
		tl, tr uint32 // Expanded 40-bit values
		al, ar uint32 // Salted expanded 40-bit values
	)

	// Left half expansion
	tl = ((p >> 16) & 0x3ff) | (((p >> 14) | (p << 18)) & 0xffc00)

	// Right half expansion
	tr = (p & 0x3ff) | ((p << 2) & 0xffc00)

	// Perform the salt permutation
	al = sk[2] & (tl ^ tr)
	ar = al ^ tr
	al ^= tl

	al ^= sk[0] // XOR with the subkey
	ar ^= sk[1]

	// S-box lookup and permutation
	return sbox[0][al>>10] | sbox[1][al&0x3ff] | sbox[2][ar>>10] | sbox[3][ar&0x3ff]
}

// Encrypt a block of 8 bytes of data with the given ICE key.
func (ik *Key) Encrypt(dst, src []byte) {
	l := uint32(src[0])<<24 | uint32(src[1])<<16 | uint32(src[2])<<8 | uint32(src[3])
	r := uint32(src[4])<<24 | uint32(src[5])<<16 | uint32(src[6])<<8 | uint32(src[7])

	for i := int32(0); i < ik.rounds; i += 2 {
		l ^= icef(r, ik.keysched[i])
		r ^= icef(l, ik.keysched[i+1])
	}

	for i := int32(0); i < 4; i++ {
		dst[3-i] = uint8(r)
		dst[7-i] = uint8(l)

		r >>= 8
		l >>= 8
	}
}

// Decrypt a block of 8 bytes of data with the given ICE key.
func (ik *Key) Decrypt(dst, src []byte) {
	l := uint32(src[0])<<24 | uint32(src[1])<<16 | uint32(src[2])<<8 | uint32(src[3])
	r := uint32(src[4])<<24 | uint32(src[5])<<16 | uint32(src[6])<<8 | uint32(src[7])

	for i := ik.rounds - 1; i > 0; i -= 2 {
		l ^= icef(r, ik.keysched[i])
		r ^= icef(l, ik.keysched[i-1])
	}

	for i := int32(0); i < 4; i++ {
		dst[3-i] = uint8(r)
		dst[7-i] = uint8(l)

		r >>= 8
		l >>= 8
	}
}

// Set 8 rounds [n, n+7] of the key schedule of an ICE key.
func (ik *Key) schedBuild(kb [4]uint16, n int32, keyrot []int32) {
	for i := int32(0); i < 8; i++ {
		kr := keyrot[i]
		isk := &ik.keysched[n+i]

		for j := 0; j < 3; j++ {
			isk[j] = 0
		}

		for j := 0; j < 15; j++ {
			curSk := &isk[j%3]

			for k := int32(0); k < 4; k++ {
				curKb := &kb[(kr+k)&3]
				bit := *curKb & 1

				*curSk = (*curSk << 1) | uint32(bit)
				*curKb = (*curKb >> 1) | ((bit ^ 1) << 15)
			}
		}
	}
}

// BlockSize returns the block size, in bytes.
func (ik *Key) BlockSize() int {
	return 8
}
