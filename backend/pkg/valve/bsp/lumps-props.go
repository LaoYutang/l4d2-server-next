package bsp

import (
	"bytes"
	"io"
)

func (f *File) loadPropCollisionLump(l lumpData, r io.ReaderAt) error {
	return l.expectEmpty("LUMP_PROPCOLLISION", 0, [4]byte{0, 0, 0, 0})
}

func (f *File) loadPropHullsLump(l lumpData, r io.ReaderAt) error {
	return l.expectEmpty("LUMP_PROPHULLS", 0, [4]byte{0, 0, 0, 0})
}

func (f *File) loadPropHullVertsLump(l lumpData, r io.ReaderAt) error {
	return l.expectEmpty("LUMP_PROPHULLVERTS", 0, [4]byte{0, 0, 0, 0})
}

func (f *File) loadPropTrisLump(l lumpData, r io.ReaderAt) error {
	return l.expectEmpty("LUMP_PROPTRIS", 0, [4]byte{0, 0, 0, 0})
}

func (f *File) loadPropBlobLump(l lumpData, r io.ReaderAt) error {
	return l.expectEmpty("LUMP_PROP_BLOB", 0, [4]byte{0, 0, 0, 0})
}

type DetailPropInfo struct {
	Names   []string
	Sprites []DetailSprite
	Props   []DetailProp
}

type DetailSprite struct {
	UL    Vector2D // Coordinate of upper left
	LR    Vector2D // Coordinate of lower right
	TexUL Vector2D // Texcoords of upper left
	TexLR Vector2D // Texcoords of lower left
}

type DetailPropOrientation uint8

const (
	OrientNormal                DetailPropOrientation = 0
	OrientScreenAligned         DetailPropOrientation = 1
	OrientScreenAlignedVertical DetailPropOrientation = 2
)

type DetailPropType uint8

const (
	DetailPropTypeModel      DetailPropType = 0
	DetailPropTypeSprite     DetailPropType = 1
	DetailPropTypeShapeCross DetailPropType = 2
	DetailPropTypeShapeTri   DetailPropType = 3
)

type DetailProp struct {
	Origin          Vector
	Angles          QAngle
	DetailModel     uint16 // either index into Names or Sprites
	Leaf            uint16
	Lighting        ColorRGBExp32
	LightStyles     uint32
	LightStyleCount uint8
	SwayAmount      uint8 // how much do the details sway
	ShapeAngle      uint8 // angle param for shaped sprites
	ShapeSize       uint8 // size param for shaped sprites
	Orientation     DetailPropOrientation
	Padding2        [3]byte
	Type            DetailPropType
	Padding3        [3]byte
	Scale           float32 // For sprites only currently
}

func (f *File) loadDetailPropsGameLump(l gameLumpData, r io.ReaderAt) error {
	if err := l.expectVersion("GAMELUMP_DETAIL_PROPS", 4, 0); err != nil {
		return err
	}

	offset, length := int64(l.FileOfs), int64(l.FileLen)
	var names [][128]byte
	n, err := readCountedRepeatedStruct(r, &names, offset, length)
	if err != nil {
		return err
	}

	f.DetailProps.Names = make([]string, len(names))
	for i := range names {
		end := bytes.IndexByte(names[i][:], 0)
		f.DetailProps.Names[i] = string(names[i][:end])
	}

	offset += n
	length -= n

	n, err = readCountedRepeatedStruct(r, &f.DetailProps.Sprites, offset, length)
	if err != nil {
		return err
	}

	offset += n
	length -= n

	_, err = readCountedRepeatedStruct(r, &f.DetailProps.Props, offset, length)

	return err
}

type StaticPropInfo struct {
	Names   []string
	LeafIDs []uint16
	Props   []StaticProp
}

type StaticPropFlags uint8

const (
	StaticPropFades             StaticPropFlags = 0x1
	StaticPropUseLightingOrigin StaticPropFlags = 0x2
	StaticPropNoDraw            StaticPropFlags = 0x4
	SttaicPropIgnoreNormals     StaticPropFlags = 0x8
	StaticPropNoShadow          StaticPropFlags = 0x10
	StaticPropNoVertexLighting  StaticPropFlags = 0x40
	StaticPropNoSelfShadowing   StaticPropFlags = 0x80
)

type StaticProp struct {
	Origin            Vector
	Angles            QAngle
	PropType          uint16
	FirstLeaf         uint16
	LeafCount         uint16
	Solid             Solid
	Flags             StaticPropFlags
	Skin              int32
	FadeMinDist       float32
	FadeMaxDist       float32
	LightingOrigin    Vector
	ForcedFadeScale   float32
	MinCPULevel       uint8
	MaxCPULevel       uint8
	MinGPULevel       uint8
	MaxGPULevel       uint8
	DiffuseModulation Color32 // per instance color and alpha modulation
	DisableX360       bool
	Padding           [3]byte
}

type Color32 struct {
	R, G, B, A uint8
}

func (f *File) loadStaticPropsGameLump(l gameLumpData, r io.ReaderAt) error {
	if err := l.expectVersion("GAMELUMP_STATIC_PROPS", 9, 0); err != nil {
		return err
	}

	offset, length := int64(l.FileOfs), int64(l.FileLen)
	var names [][128]byte
	n, err := readCountedRepeatedStruct(r, &names, offset, length)
	if err != nil {
		return err
	}

	f.StaticProps.Names = make([]string, len(names))
	for i := range names {
		end := bytes.IndexByte(names[i][:], 0)
		f.StaticProps.Names[i] = string(names[i][:end])
	}

	offset += n
	length -= n

	n, err = readCountedRepeatedStruct(r, &f.StaticProps.LeafIDs, offset, length)
	if err != nil {
		return err
	}

	offset += n
	length -= n

	_, err = readCountedRepeatedStruct(r, &f.StaticProps.Props, offset, length)

	return err
}
