package bsp

import (
	"encoding/binary"
	"io"
)

type Vector2D struct {
	X, Y float32
}

type Vector struct {
	Vector2D
	Z float32
}

type Vector4D struct {
	Vector
	W float32
}

type QAngle struct {
	Pitch, Yaw, Roll float32
}

type PlaneType int32

const (
	// 0-2 are axial planes
	PlaneX PlaneType = 0
	PlaneY PlaneType = 1
	PlaneZ PlaneType = 2

	// 3-5 are non-axial planes snapped to the nearest
	PlaneAnyX PlaneType = 3
	PlaneAnyY PlaneType = 4
	PlaneAnyZ PlaneType = 5
)

type Plane struct {
	Normal   Vector
	Distance float32
	Type     PlaneType
}

func (f *File) loadPlanesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_PLANES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.Planes, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type Vertex struct {
	Point Vector
}

func (f *File) loadVertexesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_VERTEXES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.Vertexes, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type FaceID struct {
	HammerID uint16
}

func (f *File) loadFaceIDsLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_FACEIDS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.FaceIDs, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type Edge struct {
	Vertex [2]uint16
}

func (f *File) loadEdgesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_EDGES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.Edges, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type SurfEdge struct {
	// positive refers to Edges[Edge].Vertex[0],
	// negative to Edges[-Edge].Vertex[1].
	Edge int32
}

func (f *File) loadSurfEdgesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_SURFEDGES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.SurfEdges, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type BrushModel struct {
	Mins, Maxs Vector
	Origin     Vector // for sounds or lights
	HeadNode   int32
	FirstFace  int32 // submodels just draw faces without walking the bsp tree
	NumFaces   int32
}

func (f *File) loadBrushModelsLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_MODELS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.BrushModels, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type Brush struct {
	FirstSide int32
	NumSides  int32
	Contents  Contents
}

func (f *File) loadBrushesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_BRUSHES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.Brushes, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type BrushSide struct {
	PlaneNum uint16 // facing out of the leaf
	TexInfo  int16
	DispInfo int16 // displacement info (BSPVERSION 7)
	Bevel    uint8 // is the side a bevel plane? (BSPVERSION 7)
	Thin     uint8 // is a thin side?
}

func (f *File) loadBrushSidesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_BRUSHSIDES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.BrushSides, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadOriginalFacesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_ORIGINALFACES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.OriginalFaces, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadPhysCollideLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_PHYSCOLLIDE", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	b := make([]byte, l.FileLen)
	_, err := r.ReadAt(b, int64(l.FileOfs))
	if err != nil {
		return err
	}

	for {
		mi := binary.LittleEndian.Uint32(b)
		ds := binary.LittleEndian.Uint32(b[4:])
		ks := binary.LittleEndian.Uint32(b[8:])
		sc := binary.LittleEndian.Uint32(b[12:])
		b = b[16:]
		if mi == ^uint32(0) && ds == ^uint32(0) && ks == 0 && sc == 0 {
			break
		}

		// TODO

		b = b[ds+ks:]
	}

	return l.expectTODO("LUMP_PHYSCOLLIDE", r)
}

func (f *File) loadVertNormalsLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_VERTNORMALS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.VertNormals, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadVertNormalIndicesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_VERTNORMALINDICES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.VertNormalIndices, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type PrimitiveType uint8

const (
	PrimTriList  PrimitiveType = 0
	PrimTriStrip PrimitiveType = 1
)

type Primitive struct {
	Type       PrimitiveType
	Padding    [1]byte
	FirstIndex uint16
	IndexCount uint16
	FirstVert  uint16
	VertCount  uint16
}

func (f *File) loadPrimitivesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_PRIMITIVES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.Primitives, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type PrimVert struct {
	Position Vector
}

func (f *File) loadPrimVertsLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_PRIMVERTS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.PrimVerts, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadPrimIndicesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_PRIMINDICES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.PrimIndices, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type Overlay struct {
	ID                      int32
	TexInfo                 int16
	FaceCountAndRenderOrder uint16 // top 2 bits are render order
	Faces                   [64]int32
	U                       Vector2D
	V                       Vector2D
	UVPoints                [4]Vector
	Origin                  Vector
	BasisNormal             Vector
}

func (f *File) loadOverlaysLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_OVERLAYS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.Overlays, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type WaterOverlay struct {
	ID                      int32
	TexInfo                 int16
	FaceCountAndRenderOrder uint16 // top 2 bits are render order
	Faces                   [256]int32
	U                       Vector2D
	V                       Vector2D
	UVPoints                [4]Vector
	Origin                  Vector
	BasisNormal             Vector
}

func (f *File) loadWaterOverlaysLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_WATEROVERLAYS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.WaterOverlays, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type OverlayFade struct {
	FadeDistMinSq float32
	FadeDistMaxSq float32
}

func (f *File) loadOverlayFadesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_OVERLAY_FADES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.OverlayFades, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type OverlaySystemLevel struct {
	MinCPULevel uint8
	MaxCPULevel uint8
	MinGPULevel uint8
	MaxGPULevel uint8
}

func (f *File) loadOverlaySystemLevelsLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_OVERLAY_SYSTEM_LEVELS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.OverlaySystemLevels, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadPhysLevelLump(l lumpData, r io.ReaderAt) error {
	if l.expectEmpty("LUMP_PHYSLEVEL", 0, [4]byte{0, 0, 0, 0}) == nil {
		return nil
	}

	return l.expectEmpty("LUMP_PHYSLEVEL", 16, [4]byte{0, 0, 0, 0})
}
