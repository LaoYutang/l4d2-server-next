package bsp

import (
	"io"
)

type LightingInfo struct {
	Faces               []Face
	Lighting            []ColorRGBExp32
	WorldLights         []WorldLight
	LeafAmbientIndex    []LeafAmbientIndex
	LeafAmbientLighting []LeafAmbientLighting
	DetailPropLighting  []DetailPropLighting
}

type Face struct {
	PlaneNum                    uint16
	Side                        uint8 // faces opposite to the node's plane direction
	OnNode                      uint8 // 1 of on node, 0 if in leaf
	FirstEdge                   int32 // we must support > 64k edges
	NumEdges                    int16
	TexInfo                     int16
	DispInfo                    int16
	SurfaceFogVolumeID          int16
	Styles                      [4]uint8
	LightOfs                    int32 // start of [numstyles*surfsize] samples
	Area                        float32
	LightmapTextureMinsInLuxels [2]int32
	LightmapTextureSizeInLuxels [2]int32
	OrigFace                    int32  // reference the original face this face was derived from
	NumPrims                    uint16 // Top bit, if set, disables shadows on this surface
	FirstPrimID                 uint16
	SmoothingGroups             uint32
}

func (f *File) loadFacesLumpLDR(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_FACES", 1, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.LDR.Faces, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadFacesLumpHDR(l lumpData, r io.ReaderAt) error {
	if l.FileLen == 0 {
		return nil
	}

	if err := l.expectVersion("LUMP_FACES_HDR", 1, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.HDR.Faces, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type ColorRGBExp32 struct {
	R, G, B  uint8
	Exponent int8
}

func (f *File) loadLightingLumpLDR(l lumpData, r io.ReaderAt) error {
	if l.FileLen == 0 {
		return nil
	}

	if err := l.expectVersion("LUMP_LIGHTING", 1, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.LDR.Lighting, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadLightingLumpHDR(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_LIGHTING_HDR", 1, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.HDR.Lighting, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type WorldLight struct {
	Origin           Vector
	Intensity        Vector
	Normal           Vector // for surfaces and spotlights
	ShadowCastOffset Vector // gets added to the light origin when this light is used as a shadow caster (only if DWL_FLAGS_CASTENTITYSHADOWS flag is set)
	Cluster          int32
	// 0 = emit_surface, 90 degree spotlight
	// 1 = emit_point, simple point light source
	// 2 = emit_spotlight, spotlight with penumbra
	// 3 = emit_skylight, directional light with no falloff (surface must trace to SKY texture)
	// 4 = emit_quakelight, linear falloff, non-lambertian
	// 5 = emit_skyambient, spherical light source with no falloff (surface must trace to SKY texture)
	Type     int32
	Style    int32
	StopDot  float32 // start of penumbra for emit_spotlight
	StopDot2 float32 // end of penumbra for emit_spotlight
	Exponent float32
	Radius   float32 // cutoff distance
	// falloff for emit_spotlight + emit_point:
	// 1 / (ConstantAttn + LinearAttn * dist + QuadraticAttn * dist^2)
	ConstantAttn  float32
	LinearAttn    float32
	QuadraticAttn float32
	Flags         int32 // Uses a combination of the DWL_FLAGS_ defines.
	TexInfo       int32
	Owner         int32 // entity that this light is relative to
}

func (f *File) loadWorldLightsLumpLDR(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_WORLDLIGHTS", 1, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.LDR.WorldLights, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadWorldLightsLumpHDR(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_WORLDLIGHTS_HDR", 1, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.HDR.WorldLights, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type LeafAmbientIndex struct {
	AmbientSampleCount uint16
	FirstAmbientSample uint16
}

func (f *File) loadLeafAmbientIndexLumpLDR(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_LEAF_AMBIENT_INDEX", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.LDR.LeafAmbientIndex, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadLeafAmbientIndexLumpHDR(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_LEAF_AMBIENT_INDEX_HDR", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.HDR.LeafAmbientIndex, int64(l.FileOfs), int64(l.FileLen))

	return err
}

// each leaf contains N samples of the ambient lighting
// each sample contains a cube of ambient light projected on to each axis
// and a sampling position encoded as a 0.8 fraction (mins=0,maxs=255) of the leaf's bounding box
type LeafAmbientLighting struct {
	Cube     CompressedLightCube
	X, Y, Z  uint8 // fixed point fraction of leaf bounds
	Reserved byte
}

type CompressedLightCube struct {
	Color [6]ColorRGBExp32
}

func (f *File) loadLeafAmbientLightingLumpLDR(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_LEAF_AMBIENT_LIGHTING", 1, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.LDR.LeafAmbientLighting, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadLeafAmbientLightingLumpHDR(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_LEAF_AMBIENT_LIGHTING_HDR", 1, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.HDR.LeafAmbientLighting, int64(l.FileOfs), int64(l.FileLen))

	return err
}

type DetailPropLighting struct {
	Lighting ColorRGBExp32
	Style    uint8
}

func (f *File) loadDetailPropLightingGameLumpLDR(l gameLumpData, r io.ReaderAt) error {
	if err := l.expectVersion("GAMELUMP_DETAIL_PROP_LIGHTING", 0, 0); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.LDR.DetailPropLighting, int64(l.FileOfs), int64(l.FileLen))

	return err
}

func (f *File) loadDetailPropLightingGameLumpHDR(l gameLumpData, r io.ReaderAt) error {
	if err := l.expectVersion("GAMELUMP_DETAIL_PROP_LIGHTING_HDR", 0, 0); err != nil {
		return err
	}

	_, err := readSimpleRepeatedStruct(r, &f.HDR.DetailPropLighting, int64(l.FileOfs), int64(l.FileLen))

	return err
}
