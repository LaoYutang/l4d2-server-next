package vccd

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"sort"
	"strings"
	"unicode/utf16"
)

const Magic = 0x44434356

type CaptionDictionary struct {
	Header Header
	Lookup Entries
	Data   []uint16
}

type Header struct {
	Magic         int32
	Version       int32
	NumBlocks     int32
	BlockSize     int32
	DirectorySize int32
	DataOffset    int32
}

type Entry struct {
	Hash     uint32
	BlockNum int32
	Offset   uint16
	Length   uint16
}

type Entries []Entry

func (e Entries) Len() int {
	return len(e)
}
func (e Entries) Less(i, j int) bool {
	return e[i].Hash < e[j].Hash
}
func (e Entries) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}
func (e *Entries) Push(x interface{}) {
	*e = append(*e, x.(Entry))
}
func (e *Entries) Pop() interface{} {
	x := (*e)[len(*e)-1]
	*e = (*e)[:len(*e)-1]
	return x
}
func (e Entries) Find(key string) int {
	return e.FindHash(Hash(key))
}
func (e Entries) FindHash(hash uint32) int {
	x := sort.Search(len(e), func(i int) bool {
		return e[i].Hash >= hash
	})

	if x < len(e) && e[x].Hash == hash {
		return x
	}

	return -1
}

func Hash(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(strings.ToLower(key)))
}

type countReader struct {
	r io.Reader
	n int64
}

func (r *countReader) Read(b []byte) (int, error) {
	n, err := r.r.Read(b)

	r.n += int64(n)

	return n, err
}

func (d *CaptionDictionary) ReadFrom(r io.Reader) (int64, error) {
	cr := countReader{r, 0}
	r = &cr

	err := binary.Read(r, binary.LittleEndian, &d.Header)
	if err != nil {
		return cr.n, fmt.Errorf("vccd: reading header: %w", err)
	}

	if d.Header.Magic != Magic {
		var invalid [4]byte

		binary.LittleEndian.PutUint32(invalid[:], uint32(d.Header.Magic))

		return cr.n, fmt.Errorf("vccd: invalid magic: %q", invalid[:])
	}

	if d.Header.Version < 1 {
		return cr.n, fmt.Errorf("vccd: invalid version: %d", d.Header.Version)
	}

	if d.Header.Version > 1 {
		return cr.n, fmt.Errorf("vccd: unsupported version: %d", d.Header.Version)
	}

	if d.Header.DirectorySize < 0 || d.Header.DirectorySize > 64*1024 {
		return cr.n, fmt.Errorf("vccd: invalid directory size: %d", d.Header.DirectorySize)
	}

	if d.Header.BlockSize != 8192 {
		return cr.n, fmt.Errorf("vccd: invalid block size: %d", d.Header.BlockSize)
	}

	if d.Header.NumBlocks < 0 {
		return cr.n, fmt.Errorf("vccd: invalid number of blocks: %d", d.Header.NumBlocks)
	}

	skip := int(d.Header.DataOffset) - (binary.Size(Header{}) + int(d.Header.DirectorySize)*binary.Size(Entry{}))
	if skip < 0 || skip > int(d.Header.BlockSize) {
		return cr.n, fmt.Errorf("vccd: invalid data offset: %d", d.Header.DataOffset)
	}

	d.Lookup = make(Entries, d.Header.DirectorySize)
	d.Data = make([]uint16, d.Header.NumBlocks*d.Header.BlockSize/2)

	err = binary.Read(r, binary.LittleEndian, &d.Lookup)
	if err != nil {
		return cr.n, fmt.Errorf("vccd: reading directory: %w", err)
	}

	sort.Sort(d.Lookup)

	for i := range d.Lookup {
		if d.Lookup[i].BlockNum < 0 {
			return cr.n, fmt.Errorf("vccd: invalid block number: %d", d.Lookup[i].BlockNum)
		}

		if d.Lookup[i].BlockNum >= d.Header.NumBlocks {
			return cr.n, fmt.Errorf("vccd: out-of-range block number: %d", d.Lookup[i].BlockNum)
		}

		if d.Lookup[i].Offset+d.Lookup[i].Length > uint16(d.Header.BlockSize) {
			return cr.n, fmt.Errorf("vccd: out-of-range offset+length: %d+%d", d.Lookup[i].Offset, d.Lookup[i].Length)
		}

		if d.Lookup[i].Offset&1 != 0 {
			return cr.n, fmt.Errorf("vccd: offset must be even: %d", d.Lookup[i].Offset)
		}

		if d.Lookup[i].Length&1 != 0 {
			return cr.n, fmt.Errorf("vccd: length must be even: %d", d.Lookup[i].Length)
		}

		if d.Lookup[i].Length == 0 {
			return cr.n, fmt.Errorf("vccd: length must be positive: %d", d.Lookup[i].Length)
		}

		if d.Data[(d.Lookup[i].BlockNum*d.Header.BlockSize+int32(d.Lookup[i].Offset+d.Lookup[i].Length-1))/2] != 0 {
			return cr.n, fmt.Errorf("vccd: string not null-terminated")
		}
	}

	_, err = io.CopyN(io.Discard, r, int64(skip))
	if err != nil {
		return cr.n, fmt.Errorf("vccd: seeking to data: %w", err)
	}

	err = binary.Read(r, binary.LittleEndian, &d.Data)
	if err != nil {
		return cr.n, fmt.Errorf("vccd: reading data: %w", err)
	}

	return cr.n, nil
}

func (e Entry) String(d *CaptionDictionary) string {
	start := (e.BlockNum*d.Header.BlockSize + int32(e.Offset)) / 2
	end := start + int32(e.Length-1)/2

	return string(utf16.Decode(d.Data[start:end]))
}
