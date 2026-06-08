package bsp

import (
	"io"
	"strings"
)

type TexData struct {
	Reflectivity          Vector
	NameStringTableID     int32
	Width, Height         int32
	ViewWidth, ViewHeight int32
}

func (f *File) loadTexDataLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_TEXDATA", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.TexData, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type TexInfo struct {
	TextureVecsTexelsPerWorldUnits  [2][4]float32 // [s/t][xyz offset]
	LightmapVecsLuxelsPerWorldUnits [2][4]float32 // [s/t][xyz offset] - length is in units of texels/area
	Flags                           int32         // miptex flags + overrides
	TexData                         int32         // Pointer to texture name, size, etc.
}

func (f *File) loadTexInfoLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_TEXINFO", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.TexInfo, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type Cubemap struct {
	Origin  [3]int32
	Size    uint8 // 0 - default; otherwise, 1<<(size-1)
	Padding [3]byte
}

func (f *File) loadCubemapsLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_CUBEMAPS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.Cubemaps, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadTexDataStringDataLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_TEXDATA_STRING_DATA", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	b := make([]byte, l.FileLen)
	_, err := r.ReadAt(b, int64(l.FileOfs))

	f.TexNames = []string{string(b)}

	return err
}

func (f *File) loadTexDataStringTableLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_TEXDATA_STRING_TABLE", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	var table []int32
	_, err := readSimpleRepeatedStruct(r, &table, int64(l.FileOfs), int64(l.FileLen))
	if err != nil {
		return err
	}

	buf := f.TexNames[0]
	f.TexNames = make([]string, len(table))

	for i, start := range table {
		end := start + int32(strings.IndexByte(buf[start:], 0))
		f.TexNames[i] = buf[start:end]
	}

	return nil
}

func (f *File) loadFaceMacroTextureInfoLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_FACE_MACRO_TEXTURE_INFO", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.FaceTextureID, int64(l.FileOfs), int64(l.FileLen))

	return err
}
