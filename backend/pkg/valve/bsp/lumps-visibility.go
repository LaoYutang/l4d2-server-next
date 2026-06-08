package bsp

import (
	"io"
)

type PotentialVisibilityInfo struct {
	Clusters []VisClusterOffsets
	Bits     []byte
}

type VisClusterOffsets struct {
	Visible int32
	Audible int32
}

func (f *File) loadVisibilityLump(l lumpData, r io.ReaderAt) error {
	if l.FileLen == 0 {
		return nil
	}

	if err := l.expectVersion("LUMP_VISIBILITY", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	n, err := readCountedRepeatedStruct(r, &f.PV.Clusters, int64(l.FileOfs), int64(l.FileLen))
	if err != nil {
		return err
	}

	f.PV.Bits = make([]byte, int64(l.FileLen)-n)
	_, err = r.ReadAt(f.PV.Bits, int64(l.FileOfs)+n)

	return err
}

type Node struct {
	PlaneNum  int32
	Children  [2]int32 // negative numbers are -(leafs+1), not nodes
	Mins      [3]int16 // for frustum culling
	Maxs      [3]int16
	FirstFace uint16
	NumFaces  uint16 // counting both sides
	Area      int16  // If all leaves below this node are in the same area, then this is the area index. If not, this is -1.
	Padding   [2]byte
}

func (f *File) loadNodesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_NODES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.Nodes, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type OcclusionInfo struct {
	Occluders   []Occluder
	Polys       []OccluderPoly
	VertIndices []int32
}

type Occluder struct {
	Flags     int32
	FirstPoly int32 // index into Polys
	PolyCount int32
	Mins      Vector
	Maxs      Vector
	Area      int32
}

type OccluderPoly struct {
	FirstVertexIndex int32 // index into VertIndices
	VertexCount      int32
	PlaneNum         int32
}

func (f *File) loadOcclusionLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_OCCLUSION", 2, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	offset, length := int64(l.FileOfs), int64(l.FileLen)

	n, err := readCountedRepeatedStruct(r, &f.Occlusion.Occluders, offset, length)
	if err != nil {
		return err
	}

	offset += n
	length -= n

	n, err = readCountedRepeatedStruct(r, &f.Occlusion.Polys, offset, length)
	if err != nil {
		return err
	}

	_, err = readCountedRepeatedStruct(r, &f.Occlusion.VertIndices, offset, length)

	return err
}

type Leaf struct {
	Contents        Contents
	Cluster         int16
	AreaFlags       uint16   // top 9 bits area bottom 7 bits flags
	Mins            [3]int16 // for frustum culling
	Maxs            [3]int16
	FirstLeafFace   uint16
	NumLeafFaces    uint16
	FirstLeafBrush  uint16
	NumLeafBrushes  uint16
	LeafWaterDataID int16 // -1 for not in water
	Padding         [2]byte
}

func (f *File) loadLeafsLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_LEAFS", 1, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.Leafs, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadLeafFacesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_LEAFFACES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.LeafFaces, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadLeafBrushesLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_LEAFBRUSHES", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.LeafBrushes, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type Area struct {
	NumAreaPortals  int32
	FirstAreaPortal int32
}

func (f *File) loadAreasLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_AREAS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.Areas, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type AreaPortal struct {
	PortalKey           uint16 // Entities have a key called portalnumber (and in vbsp a variable called areaportalnum) which is used to bind them to the area portals by comparing with this value.
	OtherArea           uint16 // The area this portal looks into.
	FirstClipPortalVert uint16 // Portal geometry.
	NumClipPortalVerts  uint16
	PlaneNum            int32
}

func (f *File) loadAreaPortalsLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_AREAPORTALS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.AreaPortals, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type LeafWaterData struct {
	SurfaceZ     float32
	MinZ         float32
	SurfaceTexID int16
	Padding      [2]byte
}

func (f *File) loadLeafWaterDataLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_LEAFWATERDATA", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.LeafWaterData, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadClipPortalVertsLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_CLIPPORTALVERTS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.ClipPortalVerts, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadLeafMinDistToWaterLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_LEAFMINDISTTOWATER", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.LeafMinDistToWater, int64(l.FileOfs), int64(l.FileLen))

	return err
}
