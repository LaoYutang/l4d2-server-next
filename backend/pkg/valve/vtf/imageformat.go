package vtf

import (
	"image/color"
	"strconv"
)

type ImageFormat int32

const (
	ImageFormatRGBA8888           ImageFormat = 0
	ImageFormatABGR8888           ImageFormat = 1
	ImageFormatRGB888             ImageFormat = 2
	ImageFormatBGR888             ImageFormat = 3
	ImageFormatRGB565             ImageFormat = 4
	ImageFormatI8                 ImageFormat = 5
	ImageFormatIA88               ImageFormat = 6
	ImageFormatP8                 ImageFormat = 7
	ImageFormatA8                 ImageFormat = 8
	ImageFormatRGB888BlueScreen   ImageFormat = 9
	ImageFormatBGR888BlueScreen   ImageFormat = 10
	ImageFormatARGB8888           ImageFormat = 11
	ImageFormatBGRA8888           ImageFormat = 12
	ImageFormatDXT1               ImageFormat = 13
	ImageFormatDXT3               ImageFormat = 14
	ImageFormatDXT5               ImageFormat = 15
	ImageFormatBGRX8888           ImageFormat = 16
	ImageFormatBGR565             ImageFormat = 17
	ImageFormatBGRX5551           ImageFormat = 18
	ImageFormatBGRA4444           ImageFormat = 19
	ImageFormatDXT1OneBitAlpha    ImageFormat = 20
	ImageFormatBGRA5551           ImageFormat = 21
	ImageFormatUV88               ImageFormat = 22
	ImageFormatUVWQ8888           ImageFormat = 23
	ImageFormatRGBA16161616F      ImageFormat = 24
	ImageFormatRGBA16161616       ImageFormat = 25
	ImageFormatUVLX8888           ImageFormat = 26
	ImageFormatR32F               ImageFormat = 27
	ImageFormatRGB323232F         ImageFormat = 28
	ImageFormatRGBA32323232F      ImageFormat = 29
	ImageFormatRG1616F            ImageFormat = 30
	ImageFormatRG3232F            ImageFormat = 31
	ImageFormatRGBX8888           ImageFormat = 32
	ImageFormatNull               ImageFormat = 33
	ImageFormatATI2N              ImageFormat = 34
	ImageFormatATI1N              ImageFormat = 35
	ImageFormatRGBA1010102        ImageFormat = 36
	ImageFormatBGRA1010102        ImageFormat = 37
	ImageFormatR16F               ImageFormat = 38
	ImageFormatD16                ImageFormat = 39
	ImageFormatD15S1              ImageFormat = 40
	ImageFormatD32                ImageFormat = 41
	ImageFormatD24S8              ImageFormat = 42
	ImageFormatLinearD24S8        ImageFormat = 43
	ImageFormatD24X8              ImageFormat = 44
	ImageFormatD24X4S4            ImageFormat = 45
	ImageFormatD24FS8             ImageFormat = 46
	ImageFormatD16Shadow          ImageFormat = 47
	ImageFormatD24X8Shadow        ImageFormat = 48
	ImageFormatLinearBGRX8888     ImageFormat = 49
	ImageFormatLinearRGBA8888     ImageFormat = 50
	ImageFormatLinearABGR8888     ImageFormat = 51
	ImageFormatLinearARGB8888     ImageFormat = 52
	ImageFormatLinearBGRA8888     ImageFormat = 53
	ImageFormatLinearRGB888       ImageFormat = 54
	ImageFormatLinearBGR888       ImageFormat = 55
	ImageFormatLinearBGRX5551     ImageFormat = 56
	ImageFormatLinearI8           ImageFormat = 57
	ImageFormatLinearRGBA16161616 ImageFormat = 58
	ImageFormatLEBGRX8888         ImageFormat = 59
	ImageFormatLEBGRA8888         ImageFormat = 60
)

func (i ImageFormat) String() string {
	if i >= 0 && int(i) < len(imageFormatInfos) {
		return imageFormatInfos[i].name
	}

	return "ImageFormat(" + strconv.Itoa(int(i)) + ")"
}

type imageFormatFlags uint8

const (
	iffIsCompressed  imageFormatFlags = 1 << 0
	iffIsFloat       imageFormatFlags = 1 << 1
	iffIsDepthFormat imageFormatFlags = 1 << 2
)

type imageFormatInfo struct {
	name        string
	bytes       uint8
	redBits     uint8
	greenBits   uint8
	blueBits    uint8
	alphaBits   uint8
	depthBits   uint8
	stencilBits uint8
	flags       imageFormatFlags
	model       color.Model
}

var imageFormatInfos = [...]imageFormatInfo{
	ImageFormatRGBA8888:           {"RGBA8888", 4, 8, 8, 8, 8, 0, 0, 0, color.NRGBAModel},
	ImageFormatABGR8888:           {"ABGR8888", 4, 8, 8, 8, 8, 0, 0, 0, color.NRGBAModel},
	ImageFormatRGB888:             {"RGB888", 3, 8, 8, 8, 0, 0, 0, 0, color.RGBAModel},
	ImageFormatBGR888:             {"BGR888", 3, 8, 8, 8, 0, 0, 0, 0, color.RGBAModel},
	ImageFormatRGB565:             {"RGB565", 2, 5, 6, 5, 0, 0, 0, 0, color.RGBAModel},
	ImageFormatI8:                 {"I8", 1, 0, 0, 0, 0, 0, 0, 0, color.GrayModel},
	ImageFormatIA88:               {"IA88", 2, 0, 0, 0, 8, 0, 0, 0, color.NRGBAModel},
	ImageFormatP8:                 {"P8", 1, 0, 0, 0, 0, 0, 0, 0, color.NRGBAModel},
	ImageFormatA8:                 {"A8", 1, 0, 0, 0, 8, 0, 0, 0, color.RGBAModel},
	ImageFormatRGB888BlueScreen:   {"RGB888_BLUESCREEN", 3, 8, 8, 8, 0, 0, 0, 0, color.RGBAModel},
	ImageFormatBGR888BlueScreen:   {"BGR888_BLUESCREEN", 3, 8, 8, 8, 0, 0, 0, 0, color.RGBAModel},
	ImageFormatARGB8888:           {"ARGB8888", 4, 8, 8, 8, 8, 0, 0, 0, color.NRGBAModel},
	ImageFormatBGRA8888:           {"BGRA8888", 4, 8, 8, 8, 8, 0, 0, 0, color.NRGBAModel},
	ImageFormatDXT1:               {"DXT1", 0, 0, 0, 0, 0, 0, 0, iffIsCompressed, color.RGBAModel},
	ImageFormatDXT3:               {"DXT3", 0, 0, 0, 0, 8, 0, 0, iffIsCompressed, color.NRGBAModel},
	ImageFormatDXT5:               {"DXT5", 0, 0, 0, 0, 8, 0, 0, iffIsCompressed, color.NRGBAModel},
	ImageFormatBGRX8888:           {"BGRX8888", 4, 8, 8, 8, 0, 0, 0, 0, color.RGBAModel},
	ImageFormatBGR565:             {"BGR565", 2, 5, 6, 5, 0, 0, 0, 0, color.RGBAModel},
	ImageFormatBGRX5551:           {"BGRX5551", 2, 5, 5, 5, 0, 0, 0, 0, color.RGBAModel},
	ImageFormatBGRA4444:           {"BGRA4444", 2, 4, 4, 4, 4, 0, 0, 0, color.NRGBAModel},
	ImageFormatDXT1OneBitAlpha:    {"DXT1_ONEBITALPHA", 0, 0, 0, 0, 0, 0, 0, iffIsCompressed, color.RGBAModel},
	ImageFormatBGRA5551:           {"BGRA5551", 2, 5, 5, 5, 1, 0, 0, 0, color.NRGBAModel},
	ImageFormatUV88:               {"UV88", 2, 8, 8, 0, 0, 0, 0, 0, color.RGBAModel},
	ImageFormatUVWQ8888:           {"UVWQ8888", 4, 8, 8, 8, 8, 0, 0, 0, color.NRGBAModel},
	ImageFormatRGBA16161616F:      {"RGBA16161616F", 8, 16, 16, 16, 16, 0, 0, iffIsFloat, FloatColorModel},
	ImageFormatRGBA16161616:       {"RGBA16161616", 8, 16, 16, 16, 16, 0, 0, 0, color.NRGBA64Model},
	ImageFormatUVLX8888:           {"UVLX8888", 4, 8, 8, 8, 8, 0, 0, 0, color.RGBAModel},
	ImageFormatR32F:               {"R32F", 4, 32, 0, 0, 0, 0, 0, iffIsFloat, FloatColorModel},
	ImageFormatRGB323232F:         {"RGB323232F", 12, 32, 32, 32, 0, 0, 0, iffIsFloat, FloatColorModel},
	ImageFormatRGBA32323232F:      {"RGBA32323232F", 16, 32, 32, 32, 32, 0, 0, iffIsFloat, FloatColorModel},
	ImageFormatRG1616F:            {"RG1616F", 4, 16, 16, 0, 0, 0, 0, iffIsFloat, FloatColorModel},
	ImageFormatRG3232F:            {"RG3232F", 8, 32, 32, 0, 0, 0, 0, iffIsFloat, FloatColorModel},
	ImageFormatRGBX8888:           {"RGBX8888", 4, 8, 8, 8, 0, 0, 0, 0, color.RGBAModel},
	ImageFormatNull:               {"NV_NULL", 4, 8, 8, 8, 8, 0, 0, 0, color.RGBAModel},
	ImageFormatATI1N:              {"ATI1N", 0, 0, 0, 0, 0, 0, 0, iffIsCompressed, color.GrayModel},
	ImageFormatATI2N:              {"ATI2N", 0, 0, 0, 0, 0, 0, 0, iffIsCompressed, color.RGBAModel},
	ImageFormatRGBA1010102:        {"RGBA1010102", 4, 10, 10, 10, 2, 0, 0, 0, color.NRGBA64Model},
	ImageFormatBGRA1010102:        {"BGRA1010102", 4, 10, 10, 10, 2, 0, 0, 0, color.NRGBA64Model},
	ImageFormatR16F:               {"R16F", 2, 16, 0, 0, 0, 0, 0, iffIsFloat, FloatColorModel},
	ImageFormatD16:                {"D16", 2, 0, 0, 0, 0, 16, 0, iffIsDepthFormat, nil},
	ImageFormatD15S1:              {"D15S1", 2, 0, 0, 0, 0, 15, 1, iffIsDepthFormat, nil},
	ImageFormatD32:                {"D32", 4, 0, 0, 0, 0, 32, 0, iffIsDepthFormat, nil},
	ImageFormatD24S8:              {"D24S8", 4, 0, 0, 0, 0, 24, 8, iffIsDepthFormat, nil},
	ImageFormatLinearD24S8:        {"LINEAR_D24S8", 4, 0, 0, 0, 0, 24, 8, iffIsDepthFormat, nil},
	ImageFormatD24X8:              {"D24X8", 4, 0, 0, 0, 0, 24, 0, iffIsDepthFormat, nil},
	ImageFormatD24X4S4:            {"D24X4S4", 4, 0, 0, 0, 0, 24, 4, iffIsDepthFormat, nil},
	ImageFormatD24FS8:             {"D24FS8", 4, 0, 0, 0, 0, 24, 8, iffIsDepthFormat, nil},
	ImageFormatD16Shadow:          {"D16_SHADOW", 2, 0, 0, 0, 0, 16, 0, iffIsDepthFormat, nil},
	ImageFormatD24X8Shadow:        {"D24X8_SHADOW", 4, 0, 0, 0, 0, 24, 0, iffIsDepthFormat, nil},
	ImageFormatLinearBGRX8888:     {"LINEAR_BGRX8888", 4, 8, 8, 8, 8, 0, 0, 0, color.RGBAModel},
	ImageFormatLinearRGBA8888:     {"LINEAR_RGBA8888", 4, 8, 8, 8, 8, 0, 0, 0, color.NRGBAModel},
	ImageFormatLinearABGR8888:     {"LINEAR_ABGR8888", 4, 8, 8, 8, 8, 0, 0, 0, color.NRGBAModel},
	ImageFormatLinearARGB8888:     {"LINEAR_ARGB8888", 4, 8, 8, 8, 8, 0, 0, 0, color.NRGBAModel},
	ImageFormatLinearBGRA8888:     {"LINEAR_BGRA8888", 4, 8, 8, 8, 8, 0, 0, 0, color.NRGBAModel},
	ImageFormatLinearRGB888:       {"LINEAR_RGB888", 3, 8, 8, 8, 0, 0, 0, 0, color.RGBAModel},
	ImageFormatLinearBGR888:       {"LINEAR_BGR888", 3, 8, 8, 8, 0, 0, 0, 0, color.RGBAModel},
	ImageFormatLinearBGRX5551:     {"LINEAR_BGRX5551", 2, 5, 5, 5, 0, 0, 0, 0, color.RGBAModel},
	ImageFormatLinearI8:           {"LINEAR_I8", 1, 0, 0, 0, 0, 0, 0, 0, color.GrayModel},
	ImageFormatLinearRGBA16161616: {"LINEAR_RGBA16161616", 8, 16, 16, 16, 16, 0, 0, 0, color.NRGBA64Model},
	ImageFormatLEBGRX8888:         {"LE_BGRX8888", 4, 8, 8, 8, 8, 0, 0, 0, color.RGBAModel},
	ImageFormatLEBGRA8888:         {"LE_BGRA8888", 4, 8, 8, 8, 8, 0, 0, 0, color.NRGBAModel},
}

func memoryRequiredForImage(width, height, depth, mipmaps int, format ImageFormat) int {
	if depth < 1 {
		depth = 1
	}

	if mipmaps == 1 {
		f := imageFormatInfos[format]
		if f.flags&iffIsCompressed == 0 {
			return width * height * depth * int(f.bytes)
		}

		if width < 4 && width > 0 {
			width = 4
		}

		if height < 4 && height > 0 {
			height = 4
		}

		if depth < 4 && depth > 1 {
			depth = 4
		}

		width >>= 2
		height >>= 2

		blocks := width * height * depth
		switch format {
		case ImageFormatDXT1, ImageFormatDXT1OneBitAlpha, ImageFormatATI1N:
			return blocks * 8
		case ImageFormatDXT3, ImageFormatDXT5, ImageFormatATI2N:
			return blocks * 16
		default:
			panic("unexpected compressed format")
		}
	}

	size := 0
	for {
		size += memoryRequiredForImage(width, height, depth, 1, format)
		if width == 1 && height == 1 && depth == 1 {
			break
		}

		width >>= 1
		height >>= 1
		depth >>= 1

		if width < 1 {
			width = 1
		}
		if height < 1 {
			height = 1
		}
		if depth < 1 {
			depth = 1
		}

		if mipmaps != 0 {
			mipmaps--
			if mipmaps == 0 {
				break
			}
		}
	}

	return size
}

func numMipMapLevels(width, height, depth int) int {
	if depth <= 0 {
		depth = 1
	}

	if width < 1 || height < 1 || depth < 1 {
		return 0
	}

	count := 1
	for width != 1 || height != 1 || depth != 1 {
		width >>= 1
		height >>= 1
		depth >>= 1

		if width < 1 {
			width = 1
		}
		if height < 1 {
			height = 1
		}
		if depth < 1 {
			depth = 1
		}

		count++
	}

	return count
}
