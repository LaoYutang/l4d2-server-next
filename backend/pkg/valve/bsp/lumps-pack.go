package bsp

import (
	"archive/zip"
	"bytes"
	"io"
)

func (f *File) loadPakFileLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_PAKFILE", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	b := make([]byte, l.FileLen)
	_, err := r.ReadAt(b, int64(l.FileOfs))
	if err != nil {
		return err
	}

	f.PakFile, err = zip.NewReader(bytes.NewReader(b), int64(l.FileLen))

	return err
}

func (f *File) loadXZipPakFileLump(l lumpData, r io.ReaderAt) error {
	return l.expectEmpty("LUMP_XZIPPAKFILE", 0, [4]byte{0, 0, 0, 0})
}
