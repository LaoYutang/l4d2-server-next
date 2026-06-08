package vpk

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
)

// Name returns the full path of the File.
func (f *File) Name() string {
	name := f.Base
	if name == " " {
		name = ""
	}

	if f.Ext != " " {
		name += "." + f.Ext
	}

	if f.Dir != " " {
		name = f.Dir + "/" + name
	}

	return name
}

// Open reads a file. Call the Close method to verify the file's checksum.
// The returned reader also implements io.ReaderAt.
func (f *File) Open(o *Opener) (io.ReadCloser, error) {
	crc := crc32.NewIEEE()

	chunks := make([]io.Reader, len(f.DataLocation))
	for i, c := range f.DataLocation {
		r, err := o.Archive(int(c.ArchiveIndex))
		if err != nil {
			return nil, err
		}

		offset := int64(c.EntryOffset)
		if c.ArchiveIndex == selfArchive {
			offset += int64(f.fileOffset)
		}

		chunks[i] = io.NewSectionReader(r, offset, int64(c.EntryLength))
	}

	r := io.MultiReader(chunks...)
	if len(f.Metadata) != 0 {
		r = io.MultiReader(bytes.NewReader(f.Metadata), r)
	}

	return &fileReader{
		Reader: io.TeeReader(r, crc),
		crc:    crc,
		file:   f,
		opener: o,
	}, nil
}

type fileReader struct {
	io.Reader
	crc    hash.Hash32
	file   *File
	opener *Opener
}

func (r *fileReader) ReadAt(p []byte, off int64) (n int, err error) {
	if off < int64(len(r.file.Metadata)) {
		n = copy(p, r.file.Metadata[off:])
		off -= int64(n)
		p = p[n:]
	}

	if len(p) == 0 {
		return
	}

	for _, c := range r.file.DataLocation {
		if off >= int64(c.EntryLength) {
			off -= int64(c.EntryLength)
			continue
		}

		offset := int64(c.EntryOffset) + off
		if c.ArchiveIndex == selfArchive {
			offset += int64(r.file.fileOffset)
		}

		len1 := int(c.EntryLength) - int(off)
		if len1 > len(p) {
			len1 = len(p)
		}

		f, err := r.opener.Archive(int(c.ArchiveIndex))
		if err != nil {
			return n, err
		}

		n1, err := f.ReadAt(p[:len1], offset)
		n += n1
		p = p[len1:]
		off += int64(len1)
		if err != nil || len(p) == 0 {
			return n, err
		}
	}

	return n, io.EOF
}

func (r *fileReader) Close() error {
	_, err := io.Copy(io.Discard, r.Reader)
	if err != nil {
		return fmt.Errorf("vpk: reading rest of file for checksum: %w", err)
	}

	if sum := r.crc.Sum32(); sum != r.file.CRC {
		return fmt.Errorf("vpk: file checksum mismatch %08x != %08x", sum, r.file.CRC)
	}

	return nil
}

// Bytes reads the entire file into a slice of bytes.
func (f *File) Bytes(o *Opener) ([]byte, error) {
	r, err := f.Open(o)
	if err != nil {
		return nil, err
	}

	b := make([]byte, f.Size())
	n, err := io.ReadFull(r, b)
	if err != nil {
		return b[:n], err
	}

	return b, r.Close()
}

// Size returns the total number of bytes in the file.
func (f *File) Size() int {
	s := len(f.Metadata)
	for _, c := range f.DataLocation {
		s += int(c.EntryLength)
	}

	return s
}

// Verify returns a non-nil error if the ChunkHash does not match the actual data.
func (h *ChunkHash) Verify(r io.ReaderAt) error {
	check := md5.New()

	_, err := io.Copy(check, io.NewSectionReader(r, int64(h.StartingOffset), int64(h.Count)))
	if err != nil {
		return fmt.Errorf("vpk: verifying hash: %w", err)
	}

	sum := check.Sum(nil)
	if !bytes.Equal(sum, h.MD5Checksum[:]) {
		return fmt.Errorf("vpk: chunk checksum mismatch: %x != %x", sum, h.MD5Checksum[:])
	}

	return nil
}
