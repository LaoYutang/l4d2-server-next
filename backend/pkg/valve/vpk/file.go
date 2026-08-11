package vpk

import (
	"bytes"
	"crypto/md5"
	"errors"
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

// OpenReaderAt returns a random-access reader for a file without calculating
// its checksum. The Opener owns the underlying file handles and must remain
// open while the returned reader is in use.
func (f *File) OpenReaderAt(o *Opener) io.ReaderAt {
	return &fileReaderAt{file: f, opener: o}
}

type fileReaderAt struct {
	file   *File
	opener *Opener
}

func (r *fileReaderAt) ReadAt(p []byte, off int64) (int, error) {
	return readFileAt(r.file, r.opener, p, off)
}

type fileReader struct {
	io.Reader
	crc    hash.Hash32
	file   *File
	opener *Opener
}

func (r *fileReader) ReadAt(p []byte, off int64) (n int, err error) {
	return readFileAt(r.file, r.opener, p, off)
}

func readFileAt(file *File, opener *Opener, p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, fmt.Errorf("vpk: negative file offset %d", off)
	}
	if len(p) == 0 {
		return 0, nil
	}
	if off >= int64(file.Size()) {
		return 0, io.EOF
	}

	metadataSize := int64(len(file.Metadata))
	if off < metadataSize {
		n1 := copy(p, file.Metadata[off:])
		n += n1
		p = p[n1:]
		off += int64(n1)
		if len(p) == 0 {
			return n, nil
		}
	}

	dataOffset := off - metadataSize
	for _, chunk := range file.DataLocation {
		chunkLength := int64(chunk.EntryLength)
		if dataOffset >= chunkLength {
			dataOffset -= chunkLength
			continue
		}

		archiveOffset := int64(chunk.EntryOffset) + dataOffset
		if chunk.ArchiveIndex == selfArchive {
			archiveOffset += int64(file.fileOffset)
		}

		readLength := int(chunkLength - dataOffset)
		if readLength > len(p) {
			readLength = len(p)
		}

		archive, openErr := opener.Archive(int(chunk.ArchiveIndex))
		if openErr != nil {
			return n, openErr
		}

		n1, readErr := archive.ReadAt(p[:readLength], archiveOffset)
		n += n1
		p = p[n1:]
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return n, readErr
		}
		if n1 != readLength {
			return n, io.EOF
		}
		if len(p) == 0 {
			return n, nil
		}
		dataOffset = 0
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
