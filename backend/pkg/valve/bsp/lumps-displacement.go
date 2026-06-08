package bsp

import (
	"io"
)

type DispInfo struct {
	StartPosition               Vector   // start position used for orientation -- (added BSPVERSION 6)
	DispVertStart               int32    // Index into DispVerts.
	DispTriStart                int32    // Index into DispTris.
	Power                       int32    // power - indicates size of map (2^power + 1)
	MinTess                     int32    // minimum tesselation allowed
	SmoothingAngle              float32  // lighting smoothing angle
	Contents                    Contents // surface contents
	MapFace                     uint16   // Which map face this displacement comes from.
	Padding                     [2]byte
	LightmapAlphaStart          int32                  // Index into ddisplightmapalpha. The count is m_pParent->lightmapTextureSizeInLuxels[0]*m_pParent->lightmapTextureSizeInLuxels[1].
	LightmapSamplePositionStart int32                  // Index into DispLightmapSamplePositions.
	EdgeNeighbors               [4]DispNeighbor        // Indexed by NeighborEdge enum.
	CornerNeighbors             [4]DispCornerNeighbors // Indexed by NeighborCorner enum.
	AllowedVerts                [10]uint32             // This is built based on the layout and sizes of our neighbors and tells us which vertices are allowed to be active.
}

type NeighborEdge uint8

const (
	NeighborEdgeLeft   NeighborEdge = 0
	NeighborEdgeTop    NeighborEdge = 1
	NeighborEdgeRight  NeighborEdge = 2
	NeighborEdgeBottom NeighborEdge = 3
)

type NeighborCorner uint8

const (
	NeighborCornerLowerLeft  NeighborCorner = 0
	NeighborCornerUpperLeft  NeighborCorner = 1
	NeighborCornerUpperRight NeighborCorner = 2
	NeighborCornerLowerRight NeighborCorner = 3
)

type DispNeighbor struct {
	SubNeighbors [2]DispSubNeighbor
}

type NeighborSpan uint8

const (
	CornerToCorner   NeighborSpan = 0
	CornerToMidpoint NeighborSpan = 1
	MidpointToCorner NeighborSpan = 2
)

type NeighborOrientation uint8

const (
	OrientationCCW0   NeighborOrientation = 0
	OrientationCCW90  NeighborOrientation = 1
	OrientationCCW180 NeighborOrientation = 2
	OrientationCCW270 NeighborOrientation = 3
)

type DispSubNeighbor struct {
	Neighbor            uint16              // This indexes into DispInfo. 0xFFFF if there is no neighbor here.
	NeighborOrientation NeighborOrientation // (CCW) rotation of the neighbor wrt this displacement.
	Span                NeighborSpan        // Where the neighbor fits onto this side of our displacement.
	NeighborSpan        NeighborSpan        // Where we fit onto our neighbor.
	Padding             [1]byte
}

type DispCornerNeighbors struct {
	Neighbors    [4]uint16 // indices of neighbors.
	NumNeighbors uint8
	Padding      [1]byte
}

func (f *File) loadDispInfoLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_DISPINFO", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.DispInfo, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadPhysDispLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_PHYSDISP", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	offset := int64(l.FileOfs)
	length := int64(l.FileLen)

	var dispCount uint16
	n, err := readStruct(r, &dispCount, offset, length)
	if err != nil {
		return err
	}

	offset += n
	length -= n

	dispSizes := make([]uint16, dispCount)
	n, err = readStruct(r, &dispSizes, offset, length)
	if err != nil {
		return err
	}

	offset += n
	length -= n

	f.PhysDisp = make([][]byte, dispCount)

	for i, size := range dispSizes {
		if size == ^uint16(0) {
			continue
		}

		f.PhysDisp[i] = make([]byte, size)
		_, err = r.ReadAt(f.PhysDisp[i], offset)
		if err != nil {
			return err
		}

		offset += int64(size)
		length -= int64(size)
	}

	return nil
}

func (f *File) loadDispLightmapAlphasLump(l lumpData, r io.ReaderAt) error {
	return l.expectEmpty("LUMP_DISP_LIGHTMAP_ALPHAS", 0, [4]byte{0, 0, 0, 0})
}

type DispVert struct {
	Vector Vector  // Vector field defining displacement volume.
	Dist   float32 // Displacement distances.
	Alpha  float32 // "per vertex" alpha values.
}

func (f *File) loadDispVertsLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_DISP_VERTS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.DispVerts, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadDispLightmapSamplePositionsLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_DISP_LIGHTMAP_SAMPLE_POSITIONS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	// For each displacement
	//     For each lightmap sample
	//         byte for index
	//         if 255, then index = next byte + 255
	//         3 bytes for barycentric coordinates
	f.DispLightmapSamplePositions = make([]byte, l.FileLen)
	_, err := r.ReadAt(f.DispLightmapSamplePositions, int64(l.FileOfs))

	return err
}

type DispTriFlags uint16

const (
	DispTriFlagSurface   DispTriFlags = 0x1
	DispTriFlagWalkable  DispTriFlags = 0x2
	DispTriFlagBuildable DispTriFlags = 0x4
	DispTriFlagSurfProp1 DispTriFlags = 0x8
	DispTriFlagSurfProp2 DispTriFlags = 0x10
	DispTriFlagSurfProp3 DispTriFlags = 0x20
	DispTriFlagSurfProp4 DispTriFlags = 0x40
)

type DispTri struct {
	Flags DispTriFlags
}

func (f *File) loadDispTrisLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_DISP_TRIS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.DispTris, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type DispMultiBlend struct {
	MultiBlend       Vector4D
	AlphaBlend       Vector4D
	MultiBlendColors [4]Vector
}

func (f *File) loadDispMultiBlendLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_DISP_MULTIBLEND", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.DispMultiBlend, int64(l.FileOfs), int64(l.FileLen))

	return err
}
