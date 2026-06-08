package bsp

import (
	"fmt"
	"io"
)

func (l lumpData) expectVersion(name string, version int32, fourCC [4]byte) error {
	if l.Version != version {
		return fmt.Errorf("unhandled %s version %d", name, l.Version)
	}

	if l.FourCC != fourCC {
		return fmt.Errorf("unexpected %s fourCC %q", name, l.FourCC)
	}

	return nil
}

func (l lumpData) expectEmpty(name string, version int32, fourCC [4]byte) error {
	if err := l.expectVersion(name, version, fourCC); err != nil {
		return err
	}

	if l.FileLen != 0 {
		return fmt.Errorf("unexpected %s with data", name)
	}

	return nil
}

func (l lumpData) expectTODO(name string, r io.ReaderAt) error {
	return nil
}

func (l gameLumpData) expectVersion(name string, version, flags uint16) error {
	if l.Version != version {
		return fmt.Errorf("unhandled %s version %d", name, l.Version)
	}

	if l.Flags != flags {
		return fmt.Errorf("unexpected %s flags %04x", name, l.Flags)
	}

	return nil
}

var lumpLoaders = [64]func(*File, lumpData, io.ReaderAt) error{
	0:  (*File).loadEntitiesLump,
	1:  (*File).loadPlanesLump,
	2:  (*File).loadTexDataLump,
	3:  (*File).loadVertexesLump,
	4:  (*File).loadVisibilityLump,
	5:  (*File).loadNodesLump,
	6:  (*File).loadTexInfoLump,
	7:  (*File).loadFacesLumpLDR,
	8:  (*File).loadLightingLumpLDR,
	9:  (*File).loadOcclusionLump,
	10: (*File).loadLeafsLump,
	11: (*File).loadFaceIDsLump,
	12: (*File).loadEdgesLump,
	13: (*File).loadSurfEdgesLump,
	14: (*File).loadBrushModelsLump,
	15: (*File).loadWorldLightsLumpLDR,
	16: (*File).loadLeafFacesLump,
	17: (*File).loadLeafBrushesLump,
	18: (*File).loadBrushesLump,
	19: (*File).loadBrushSidesLump,
	20: (*File).loadAreasLump,
	21: (*File).loadAreaPortalsLump,
	22: (*File).loadPropCollisionLump,
	23: (*File).loadPropHullsLump,
	24: (*File).loadPropHullVertsLump,
	25: (*File).loadPropTrisLump,
	26: (*File).loadDispInfoLump,
	27: (*File).loadOriginalFacesLump,
	28: (*File).loadPhysDispLump,
	29: (*File).loadPhysCollideLump,
	30: (*File).loadVertNormalsLump,
	31: (*File).loadVertNormalIndicesLump,
	32: (*File).loadDispLightmapAlphasLump,
	33: (*File).loadDispVertsLump,
	34: (*File).loadDispLightmapSamplePositionsLump,
	35: (*File).loadGameLumps,
	36: (*File).loadLeafWaterDataLump,
	37: (*File).loadPrimitivesLump,
	38: (*File).loadPrimVertsLump,
	39: (*File).loadPrimIndicesLump,
	40: (*File).loadPakFileLump,
	41: (*File).loadClipPortalVertsLump,
	42: (*File).loadCubemapsLump,
	43: (*File).loadTexDataStringDataLump,
	44: (*File).loadTexDataStringTableLump,
	45: (*File).loadOverlaysLump,
	46: (*File).loadLeafMinDistToWaterLump,
	47: (*File).loadFaceMacroTextureInfoLump,
	48: (*File).loadDispTrisLump,
	49: (*File).loadPropBlobLump,
	50: (*File).loadWaterOverlaysLump,
	51: (*File).loadLeafAmbientIndexLumpHDR,
	52: (*File).loadLeafAmbientIndexLumpLDR,
	53: (*File).loadLightingLumpHDR,
	54: (*File).loadWorldLightsLumpHDR,
	55: (*File).loadLeafAmbientLightingLumpHDR,
	56: (*File).loadLeafAmbientLightingLumpLDR,
	57: (*File).loadXZipPakFileLump,
	58: (*File).loadFacesLumpHDR,
	59: (*File).loadMapFlagsLump,
	60: (*File).loadOverlayFadesLump,
	61: (*File).loadOverlaySystemLevelsLump,
	62: (*File).loadPhysLevelLump,
	63: (*File).loadDispMultiBlendLump,
}

var gameLumpLoaders = map[string]func(*File, gameLumpData, io.ReaderAt) error{
	"dprp": (*File).loadDetailPropsGameLump,
	"dplt": (*File).loadDetailPropLightingGameLumpLDR,
	"dplh": (*File).loadDetailPropLightingGameLumpHDR,
	"sprp": (*File).loadStaticPropsGameLump,
}

func (f *File) loadLumps(header *headerData, r io.ReaderAt) error {
	for i, fn := range lumpLoaders {
		if err := fn(f, header.Lumps[i], r); err != nil {
			return fmt.Errorf("in lump %d: %w", i, err)
		}
	}

	return nil
}

func (f *File) loadGameLumps(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_GAME_LUMP", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	var gameLumps []gameLumpData
	_, err := readCountedRepeatedStruct(r, &gameLumps, int64(l.FileOfs), int64(l.FileLen))
	if err != nil {
		return err
	}

	for _, gl := range gameLumps {
		fourCC := string([]byte{gl.ID[3], gl.ID[2], gl.ID[1], gl.ID[0]})
		fn, ok := gameLumpLoaders[fourCC]
		if !ok {
			return fmt.Errorf("no handler for game lump %q", fourCC)
		}

		if err := fn(f, gl, r); err != nil {
			return fmt.Errorf("in game lump %q: %w", fourCC, err)
		}
	}

	return nil
}

type MapFlags struct {
	Level LevelFlags
}

type LevelFlags uint32

const (
	LevelFlagsBakedStaticPropLightingNonHDR LevelFlags = 0x00000001 // was processed by vrad with -staticproplighting, no hdr data
	LevelFlagsBakedStaticPropLightingHDR    LevelFlags = 0x00000002 // was processed by vrad with -staticproplighting, in hdr
)

func (f *File) loadMapFlagsLump(l lumpData, r io.ReaderAt) error {
	if err := l.expectVersion("LUMP_MAP_FLAGS", 0, [4]byte{0, 0, 0, 0}); err != nil {
		return err
	}

	offset, length := int64(l.FileOfs), int64(l.FileLen)
	if length == 0 {
		return nil
	}

	n, err := readStruct(r, &f.Flags.Level, offset, length)
	if err != nil {
		return err
	}

	offset += n
	length -= n

	if length == 0 {
		return nil
	}

	return nil
}
