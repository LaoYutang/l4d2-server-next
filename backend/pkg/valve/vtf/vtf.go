package vtf

import (
	"encoding/binary"
	"fmt"
	"io"
)

type VTF struct {
	Header       headerData
	resources    []resourceEntryInfo
	resourceData [][]byte
	LowResImage  *Bitmap
	Image        [][]*Bitmap
}

type headerData struct {
	basicHeader
	headerV73
}
type basicHeader struct {
	FileTypeString [4]byte
	Version        [2]int32
	HeaderSize     int32
}

type headerV71 struct {
	Width             uint16
	Height            uint16
	Flags             TextureFlags
	NumFrames         uint16
	StartFrame        uint16
	Pad1              [4]byte
	Reflectivity      [3]float32 // B/G/R
	Pad2              [4]byte
	BumpScale         float32
	ImageFormat       ImageFormat
	NumMipLevels      uint8
	LowResImageFormat ImageFormat
	LowResImageWidth  uint8
	LowResImageHeight uint8
}

type headerV72 struct {
	headerV71
	Depth uint16
}

type headerV73 struct {
	headerV72
	Pad4         [3]byte
	NumResources uint32
	Pad5         [8]byte
}

type TextureFlags uint32

const (
	TextureFlagsPointSample       TextureFlags = 0x00000001
	TextureFlagsTrilinear         TextureFlags = 0x00000002
	TextureFlagsClampS            TextureFlags = 0x00000004
	TextureFlagsClampT            TextureFlags = 0x00000008
	TextureFlagsAnisotropic       TextureFlags = 0x00000010
	TextureFlagsHintDXT5          TextureFlags = 0x00000020
	TextureFlagsPWLCorrected      TextureFlags = 0x00000040
	TextureFlagsNormal            TextureFlags = 0x00000080
	TextureFlagsNoMip             TextureFlags = 0x00000100
	TextureFlagsNoLod             TextureFlags = 0x00000200
	TextureFlagsAllMips           TextureFlags = 0x00000400
	TextureFlagsProcedural        TextureFlags = 0x00000800
	TextureFlagsOneBitAlpha       TextureFlags = 0x00001000
	TextureFlagsEightBitAlpha     TextureFlags = 0x00002000
	TextureFlagsEnvMap            TextureFlags = 0x00004000
	TextureFlagsRenderTarget      TextureFlags = 0x00008000
	TextureFlagsDepthRenderTarget TextureFlags = 0x00010000
	TextureFlagsNoDebugOverride   TextureFlags = 0x00020000
	TextureFlagsSingleCopy        TextureFlags = 0x00040000
	TextureFlagsPreSRGB           TextureFlags = 0x00080000 // SRGB correction has already been applied to this texture
	TextureFlagsNoDepthBuffer     TextureFlags = 0x00800000
	TextureFlagsClampU            TextureFlags = 0x02000000
	TextureFlagsVertexTexture     TextureFlags = 0x04000000 // Usable as a vertex texture
	TextureFlagsSSBump            TextureFlags = 0x08000000
	TextureFlagsBorder            TextureFlags = 0x20000000 // Clamp to border color on all texture coordinates
)

type resourceEntryInfo struct {
	Type [4]byte
	Data uint32
}

func Read(r io.ReaderAt) (*VTF, error) {
	var v VTF
	basicHeaderSize := int64(binary.Size(&v.Header.basicHeader))

	sr := io.NewSectionReader(r, 0, basicHeaderSize)
	err := binary.Read(sr, binary.LittleEndian, &v.Header.basicHeader)
	if err != nil {
		return nil, err
	}

	if v.Header.FileTypeString != [4]byte{'V', 'T', 'F', 0} {
		return nil, fmt.Errorf("not a VTF file")
	}

	if v.Header.Version[0] != 7 {
		return nil, fmt.Errorf("unsupported VTF version %d.%d", v.Header.Version[0], v.Header.Version[1])
	}

	sr = io.NewSectionReader(r, basicHeaderSize, int64(v.Header.HeaderSize)-basicHeaderSize)

	switch v.Header.Version[1] {
	case 0, 1:
		err = binary.Read(sr, binary.LittleEndian, &v.Header.headerV71)
	case 2:
		err = binary.Read(sr, binary.LittleEndian, &v.Header.headerV72)
	case 3, 4, 5:
		err = binary.Read(sr, binary.LittleEndian, &v.Header.headerV73)
	default:
		err = fmt.Errorf("unsupported VTF version %d.%d", v.Header.Version[0], v.Header.Version[1])
	}

	if err != nil {
		return nil, err
	}

	// version fixups
	switch v.Header.Version[1] {
	case 0, 1:
		v.Header.Depth = 1
		fallthrough
	case 2:
		v.Header.NumResources = 0
		fallthrough
	case 3:
		v.Header.Flags &^= 0xD1780400
		fallthrough
	case 4, 5:
		break
	}

	if v.Header.Flags&TextureFlagsEnvMap != 0 && v.Header.Width != v.Header.Height {
		return nil, fmt.Errorf("non-square cubemap")
	}

	if v.Header.Flags&TextureFlagsEnvMap != 0 && v.Header.Depth != 1 {
		return nil, fmt.Errorf("volumetric cubemap")
	}

	if v.Header.Width <= 0 || v.Header.Height <= 0 || v.Header.Depth <= 0 {
		return nil, fmt.Errorf("invalid size")
	}

	facesAndFrames := int(v.Header.NumFrames)
	if v.Header.Flags&TextureFlagsEnvMap != 0 {
		facesAndFrames *= 6
	}

	v.resources = make([]resourceEntryInfo, v.Header.NumResources)
	err = binary.Read(sr, binary.LittleEndian, &v.resources)
	if err != nil {
		return nil, err
	}

	if v.Header.Version[1] <= 2 {
		// older version
		size := memoryRequiredForImage(int(v.Header.LowResImageWidth), int(v.Header.LowResImageHeight), 1, 1, v.Header.LowResImageFormat)
		if size != 0 {
			v.resources = append(v.resources, resourceEntryInfo{
				Type: [4]byte{1, 0, 0, 0},
				Data: uint32(v.Header.HeaderSize),
			})
		}

		v.resources = append(v.resources, resourceEntryInfo{
			Type: [4]byte{48, 0, 0, 0},
			Data: uint32(v.Header.HeaderSize) + uint32(size),
		})
	}

	// load low res image
	for _, resource := range v.resources {
		if resource.Type == [4]byte{1, 0, 0, 0} {
			b := make([]byte, memoryRequiredForImage(int(v.Header.LowResImageWidth), int(v.Header.LowResImageHeight), 1, 1, v.Header.LowResImageFormat))

			_, err = r.ReadAt(b, int64(resource.Data))
			if err != nil {
				return nil, err
			}

			v.LowResImage = makeSingleImage(int(v.Header.LowResImageWidth), int(v.Header.LowResImageHeight), 1, v.Header.LowResImageFormat, b)

			break
		}
	}

	// load resources
	v.resourceData = make([][]byte, len(v.resources))

	for i, resource := range v.resources {
		if resource.Type[3]&2 != 0 {
			// RSRCF_HAS_NO_DATA_CHUNK
			continue
		}

		if resource.Type == [4]byte{1, 0, 0, 0} || resource.Type == [4]byte{48, 0, 0, 0} {
			// legacy resource
			continue
		}

		var count int32
		sr := io.NewSectionReader(r, int64(resource.Data), 4)
		err = binary.Read(sr, binary.LittleEndian, &count)
		if err != nil {
			return nil, err
		}

		v.resourceData[i] = make([]byte, count)
		sr = io.NewSectionReader(r, int64(resource.Data)+4, int64(count))
		err = binary.Read(sr, binary.LittleEndian, &v.resourceData[i])
		if err != nil {
			return nil, err
		}
	}

	// load image
	for _, resource := range v.resources {
		if resource.Type == [4]byte{48, 0, 0, 0} {
			b := make([]byte, memoryRequiredForImage(int(v.Header.Width), int(v.Header.Height), int(v.Header.Depth), int(v.Header.NumMipLevels), v.Header.ImageFormat)*facesAndFrames)

			_, err = r.ReadAt(b, int64(resource.Data))
			if err != nil {
				return nil, err
			}

			v.Image = makeImageSet(int(v.Header.Width), int(v.Header.Height), int(v.Header.Depth), int(v.Header.NumMipLevels), facesAndFrames, v.Header.ImageFormat, b)

			break
		}
	}

	return &v, nil
}
