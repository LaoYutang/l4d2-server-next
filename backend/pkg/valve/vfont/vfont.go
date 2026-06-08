package vfont

import (
	"bytes"
	"crypto/rand"
)

const Tag = "VFONT1"

func Encode(ttf []byte) []byte {
	var salt [1]byte
	_, err := rand.Read(salt[:])
	if err != nil {
		// assume that crypto/rand.Read will never fail
		panic(err)
	}

	salt[0] &= 0xf
	salt[0] |= 0x10

	buf := make([]byte, len(ttf)+int(salt[0])+len(Tag))
	copy(buf[len(ttf)+int(salt[0]):], Tag)

	buf[len(ttf)+int(salt[0])-1] = salt[0]
	salt[0]--

	iv := buf[len(ttf) : len(ttf)+int(salt[0])]
	_, err = rand.Read(iv)
	if err != nil {
		panic(err)
	}

	x := byte(0xA7)

	for _, b := range iv {
		x ^= b + 0xA7
	}

	for i, b := range ttf {
		x ^= b
		buf[i] = x
		x += 0xA7
	}

	return buf
}

func Decode(buf []byte) []byte {
	if len(buf) <= len(Tag) || !bytes.HasSuffix(buf, []byte(Tag)) {
		return nil
	}

	buf = buf[:len(buf)-len(Tag)]

	salt := buf[len(buf)-1]
	if int(salt) > len(buf) {
		return nil
	}

	iv := buf[len(buf)-int(salt) : len(buf)-1]
	ttf := make([]byte, len(buf)-int(salt))

	x := byte(0xA7)

	for _, b := range iv {
		x ^= b + 0xA7
	}

	for i, b := range buf[:len(ttf)] {
		ttf[i] = x ^ b
		x = b + 0xA7
	}

	return ttf
}
