package bsp

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
)

type File struct {
	Entities []Entity
	PakFile  *zip.Reader
	Flags    MapFlags

	// world geometry
	Planes              []Plane
	Vertexes            []Vertex
	FaceIDs             []FaceID
	Edges               []Edge
	SurfEdges           []SurfEdge
	BrushModels         []BrushModel
	Brushes             []Brush
	BrushSides          []BrushSide
	OriginalFaces       []Face
	VertNormals         []Vector
	VertNormalIndices   []uint16
	Primitives          []Primitive
	PrimVerts           []PrimVert
	PrimIndices         []uint16
	Overlays            []Overlay
	WaterOverlays       []WaterOverlay
	OverlayFades        []OverlayFade
	OverlaySystemLevels []OverlaySystemLevel

	// displacements
	DispInfo                    []DispInfo
	DispVerts                   []DispVert
	DispLightmapSamplePositions []byte
	DispTris                    []DispTri
	DispMultiBlend              []DispMultiBlend
	PhysDisp                    [][]byte

	// texture data
	TexData       []TexData
	TexInfo       []TexInfo
	Cubemaps      []Cubemap
	TexNames      []string
	FaceTextureID []uint16

	// visibility data
	PV                 PotentialVisibilityInfo
	Occlusion          OcclusionInfo
	Nodes              []Node
	Leafs              []Leaf
	LeafFaces          []uint16
	LeafBrushes        []uint16
	Areas              []Area
	AreaPortals        []AreaPortal
	LeafWaterData      []LeafWaterData
	ClipPortalVerts    []Vector
	LeafMinDistToWater []uint16

	// lighting data
	LDR LightingInfo
	HDR LightingInfo

	// props
	StaticProps StaticPropInfo
	DetailProps DetailPropInfo
}

type headerData struct {
	Ident    [4]byte
	Version  int32
	Lumps    [64]lumpData
	Revision int32
}

type lumpData struct {
	FileOfs int32
	FileLen int32
	Version int32
	FourCC  [4]byte
}

type gameLumpData struct {
	ID      [4]byte
	Flags   uint16
	Version uint16
	FileOfs int32
	FileLen int32
}

func readStruct(r io.ReaderAt, v interface{}, offset, length int64) (int64, error) {
	n := int64(binary.Size(v))
	if n > length {
		return 0, fmt.Errorf("%T (%d bytes) exceeds %d available bytes", v, n, length)
	}

	sr := io.NewSectionReader(r, offset, n)
	return n, binary.Read(sr, binary.LittleEndian, v)
}

func readSimpleRepeatedStruct(r io.ReaderAt, v interface{}, offset, length int64) (int64, error) {
	val := reflect.Indirect(reflect.ValueOf(v))
	if val.Kind() != reflect.Slice {
		panic("invalid type " + val.Type().String() + " for simple repeated struct")
	}

	elem := val.Type().Elem()
	elemSize := int64(binary.Size(reflect.Zero(elem).Interface()))

	if length%elemSize != 0 {
		return 0, fmt.Errorf("%v array cannot be %d bytes - must be a multiple of %d", elem, length, elemSize)
	}

	n := int(length / elemSize)
	val.Set(reflect.MakeSlice(val.Type(), n, n))

	sr := io.NewSectionReader(r, offset, length)
	return length, binary.Read(sr, binary.LittleEndian, v)
}

func readCountedRepeatedStruct(r io.ReaderAt, v interface{}, offset, length int64) (int64, error) {
	val := reflect.Indirect(reflect.ValueOf(v))
	if val.Kind() != reflect.Slice {
		panic("invalid type " + val.Type().String() + " for simple repeated struct")
	}

	elem := val.Type().Elem()
	elemSize := int64(binary.Size(reflect.Zero(elem).Interface()))

	var count int32

	n, err := readStruct(r, &count, offset, length)
	if err != nil {
		return n, err
	}

	offset += n
	length -= n
	totalSize := int64(elemSize) * int64(count)
	if totalSize > length {
		return 0, fmt.Errorf("%v array cannot be %d bytes - only %d bytes remain", elem, totalSize, length)
	}

	val.Set(reflect.MakeSlice(val.Type(), int(count), int(count)))

	sr := io.NewSectionReader(r, offset, length)
	return n + totalSize, binary.Read(sr, binary.LittleEndian, v)
}

func (f *File) ReadFrom(r io.ReaderAt) error {
	var file File

	var header headerData

	_, err := readStruct(r, &header, 0, 1<<31)
	if err != nil {
		return fmt.Errorf("reading BSP header: %w", err)
	}

	if header.Ident != [4]byte{'V', 'B', 'S', 'P'} {
		return fmt.Errorf("invalid BSP magic number: %q", header.Ident[:])
	}

	if header.Version != 21 {
		return fmt.Errorf("unhandled BSP version: %d", header.Version)
	}

	if err := file.loadLumps(&header, r); err != nil {
		return err
	}

	*f = file

	return nil
}
