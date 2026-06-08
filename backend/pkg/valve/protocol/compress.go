package protocol

import (
	"encoding/binary"
	"fmt"
)

func decompress(b []byte) ([]byte, error) {
	if len(b) < 8 {
		return nil, fmt.Errorf("compressed data (%d bytes) too short", len(b))
	}

	magic := binary.LittleEndian.Uint32(b)
	size := binary.LittleEndian.Uint32(b[4:])

	dst := make([]byte, size)

	switch magic {
	case 0x53535a4c: // LZSS
		return dst, lzssDecompress(dst, b[8:])
	default:
		return nil, fmt.Errorf("unknown compression type %08x", magic)
	}
}

func lzssDecompress(dst, src []byte) error {
	var cmdByte, getCmdByte uint8
	i, j := 0, 0

	for {
		if getCmdByte == 0 {
			cmdByte = src[i]
			i++
		}
		getCmdByte = (getCmdByte + 1) & 7

		if cmdByte&1 != 0 {
			position := int(src[i]) << 4
			i++
			position |= int(src[i]) >> 4
			count := int(src[i]&0xf) + 1
			i++
			if count == 1 {
				break
			}

			for k := 0; k < count; k++ {
				dst[j] = dst[j-position-1]
				j++
			}
		} else {
			dst[j] = src[i]
			i, j = i+1, j+1
		}
		cmdByte >>= 1
	}

	if j != len(dst) {
		return fmt.Errorf("unexpected length %d != %d", j, len(dst))
	}

	return nil
}
