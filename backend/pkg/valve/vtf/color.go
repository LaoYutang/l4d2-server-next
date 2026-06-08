package vtf

import (
	"encoding/binary"
	"image/color"
	"math"

	"github.com/x448/float16"
)

func decodeColor(format ImageFormat, b []byte) color.Color {
	switch format {
	case ImageFormatRGBA8888, ImageFormatLinearRGBA8888:
		return color.NRGBA{
			R: b[0],
			G: b[1],
			B: b[2],
			A: b[3],
		}
	case ImageFormatABGR8888, ImageFormatLinearABGR8888:
		return color.NRGBA{
			R: b[3],
			G: b[2],
			B: b[1],
			A: b[0],
		}
	case ImageFormatRGB888, ImageFormatLinearRGB888:
		return color.RGBA{
			R: b[0],
			G: b[1],
			B: b[2],
			A: 255,
		}
	case ImageFormatBGR888, ImageFormatLinearBGR888:
		return color.RGBA{
			R: b[2],
			G: b[1],
			B: b[0],
			A: 255,
		}
	case ImageFormatRGB565:
		return color.RGBA(rgbaFrom565(binary.LittleEndian.Uint16(b)))
	case ImageFormatI8, ImageFormatLinearI8:
		return color.Gray{
			Y: b[0],
		}
	case ImageFormatIA88:
		return color.NRGBA{
			R: b[0],
			G: b[0],
			B: b[0],
			A: b[1],
		}
	case ImageFormatP8:
		panic("no support for P8 format")
	case ImageFormatA8:
		return color.RGBA{
			R: b[0],
			G: b[0],
			B: b[0],
			A: b[0],
		}
	case ImageFormatRGB888BlueScreen:
		if b[0] == 0 && b[1] == 0 && b[2] == 255 {
			return color.RGBA{}
		}

		return color.RGBA{
			R: b[0],
			G: b[1],
			B: b[2],
			A: 255,
		}
	case ImageFormatBGR888BlueScreen:
		if b[0] == 255 && b[1] == 0 && b[2] == 0 {
			return color.RGBA{}
		}

		return color.RGBA{
			R: b[2],
			G: b[1],
			B: b[0],
			A: 255,
		}
	case ImageFormatARGB8888, ImageFormatLinearARGB8888:
		return color.NRGBA{
			R: b[1],
			G: b[2],
			B: b[3],
			A: b[0],
		}
	case ImageFormatBGRA8888, ImageFormatLinearBGRA8888, ImageFormatLEBGRA8888:
		return color.NRGBA{
			R: b[2],
			G: b[1],
			B: b[0],
			A: b[3],
		}
	case ImageFormatBGRX8888, ImageFormatLinearBGRX8888, ImageFormatLEBGRX8888:
		return color.RGBA{
			R: b[2],
			G: b[1],
			B: b[0],
			A: 255,
		}
	case ImageFormatBGR565:
		c := rgbaFrom565(binary.LittleEndian.Uint16(b))
		return color.RGBA{
			R: c.B,
			G: c.G,
			B: c.R,
			A: c.A,
		}
	case ImageFormatBGRX5551, ImageFormatLinearBGRX5551:
		c := rgbaFrom5551(binary.LittleEndian.Uint16(b))
		return color.RGBA{
			R: c.B,
			G: c.G,
			B: c.R,
			A: 255,
		}
	case ImageFormatBGRA4444:
		c := rgbaFrom4444(binary.LittleEndian.Uint16(b))
		return color.NRGBA{
			R: c.B,
			G: c.G,
			B: c.R,
			A: c.A,
		}
	case ImageFormatBGRA5551:
		c := rgbaFrom5551(binary.LittleEndian.Uint16(b))
		return color.NRGBA{
			R: c.B,
			G: c.G,
			B: c.R,
			A: c.A,
		}
	case ImageFormatUV88:
		return color.RGBA{
			R: b[0],
			G: b[1],
			B: 0,
			A: 255,
		}
	case ImageFormatUVWQ8888:
		return color.NRGBA{
			R: b[0],
			G: b[1],
			B: b[2],
			A: b[3],
		}
	case ImageFormatRGBA16161616F:
		return FloatColor{
			R: float16.Frombits(binary.LittleEndian.Uint16(b)).Float32(),
			G: float16.Frombits(binary.LittleEndian.Uint16(b[2:])).Float32(),
			B: float16.Frombits(binary.LittleEndian.Uint16(b[4:])).Float32(),
			A: float16.Frombits(binary.LittleEndian.Uint16(b[6:])).Float32(),
		}
	case ImageFormatRGBA16161616, ImageFormatLinearRGBA16161616:
		return color.NRGBA64{
			R: binary.LittleEndian.Uint16(b),
			G: binary.LittleEndian.Uint16(b[2:]),
			B: binary.LittleEndian.Uint16(b[4:]),
			A: binary.LittleEndian.Uint16(b[6:]),
		}
	case ImageFormatUVLX8888:
		return color.NRGBA{
			R: b[0],
			G: b[1],
			B: b[2],
			A: b[3],
		}
	case ImageFormatR32F:
		return FloatColor{
			R: math.Float32frombits(binary.LittleEndian.Uint32(b)),
			G: 0.0,
			B: 0.0,
			A: 1.0,
		}
	case ImageFormatRGB323232F:
		return FloatColor{
			R: math.Float32frombits(binary.LittleEndian.Uint32(b)),
			G: math.Float32frombits(binary.LittleEndian.Uint32(b[4:])),
			B: math.Float32frombits(binary.LittleEndian.Uint32(b[8:])),
			A: 1.0,
		}
	case ImageFormatRGBA32323232F:
		return FloatColor{
			R: math.Float32frombits(binary.LittleEndian.Uint32(b)),
			G: math.Float32frombits(binary.LittleEndian.Uint32(b[4:])),
			B: math.Float32frombits(binary.LittleEndian.Uint32(b[8:])),
			A: math.Float32frombits(binary.LittleEndian.Uint32(b[12:])),
		}
	case ImageFormatRG1616F:
		return FloatColor{
			R: float16.Frombits(binary.LittleEndian.Uint16(b)).Float32(),
			G: float16.Frombits(binary.LittleEndian.Uint16(b[2:])).Float32(),
			B: 0.0,
			A: 1.0,
		}
	case ImageFormatRG3232F:
		return FloatColor{
			R: math.Float32frombits(binary.LittleEndian.Uint32(b)),
			G: math.Float32frombits(binary.LittleEndian.Uint32(b[4:])),
			B: 0.0,
			A: 1.0,
		}
	case ImageFormatRGBX8888:
		return color.RGBA{
			R: b[0],
			G: b[1],
			B: b[2],
			A: 255,
		}
	case ImageFormatNull:
		return color.RGBA{}
	case ImageFormatRGBA1010102:
		return rgbaFrom1010102(binary.LittleEndian.Uint32(b))
	case ImageFormatBGRA1010102:
		c := rgbaFrom1010102(binary.LittleEndian.Uint32(b))
		return color.NRGBA64{
			R: c.B,
			G: c.G,
			B: c.R,
			A: c.A,
		}
	case ImageFormatR16F:
		return FloatColor{
			R: float16.Frombits(binary.LittleEndian.Uint16(b)).Float32(),
			G: 0.0,
			B: 0.0,
			A: 1.0,
		}
	case ImageFormatD16, ImageFormatD15S1, ImageFormatD32,
		ImageFormatD24S8, ImageFormatLinearD24S8, ImageFormatD24X8,
		ImageFormatD24X4S4, ImageFormatD24FS8, ImageFormatD16Shadow,
		ImageFormatD24X8Shadow:
		panic("depth texture formats are not supported")
	}

	panic("unhandled image format " + format.String())
}

func decodeRGBA64(format ImageFormat, b []byte) color.RGBA64 {
	// TODO: rewrite to not allocate
	return color.RGBA64Model.Convert(decodeColor(format, b)).(color.RGBA64)
}

func decodeS3TC(b []byte, x, y int, alpha bool) color.RGBA {
	c0 := binary.LittleEndian.Uint16(b)
	c1 := binary.LittleEndian.Uint16(b[2:])
	c0c := color.RGBA(rgbaFrom565(c0))
	c1c := color.RGBA(rgbaFrom565(c1))
	switch (b[7-y] >> ((3 - x) << 1)) & 3 {
	case 0:
		return c0c
	case 1:
		return c1c
	case 2:
		if c0 <= c1 {
			return mixRGBA(c0c, 1, c1c, 1)
		}

		return mixRGBA(c0c, 2, c1c, 1)
	case 3:
		if c0 <= c1 {
			if alpha {
				return color.RGBA{0, 0, 0, 0}
			}

			return color.RGBA{0, 0, 0, 255}
		}

		return mixRGBA(c0c, 1, c1c, 2)
	default:
		panic("unreachable")
	}
}

func decode3Dc(b []byte, x, y int) uint8 {
	bits := binary.LittleEndian.Uint64(b) >> 16
	mode := bits >> (3 * (y<<2 | x)) & 7
	if mode == 0 {
		return b[0]
	}

	if mode == 1 {
		return b[1]
	}

	if b[0] <= b[1] {
		if mode == 6 {
			return 0
		}

		if mode == 7 {
			return 255
		}

		return uint8((uint16(6-mode)*uint16(b[0]) + uint16(mode-1)*uint16(b[1])) / 5)
	}

	return uint8((uint16(8-mode)*uint16(b[0]) + uint16(mode-1)*uint16(b[1])) / 7)
}

func decodeCompressedColor(format ImageFormat, b []byte, x, y int) color.Color {
	switch format {
	case ImageFormatDXT1:
		return decodeS3TC(b, x, y, false)
	case ImageFormatDXT1OneBitAlpha:
		return decodeS3TC(b, x, y, true)
	case ImageFormatDXT3:
		c := color.NRGBA(decodeS3TC(b[8:], x, y, false))
		alphaBits := b[y<<1|x>>1]
		if x&1 == 0 {
			alphaBits >>= 4
		} else {
			alphaBits &= 15
		}
		c.A = alphaBits | alphaBits<<4
		return c
	case ImageFormatDXT5:
		c := color.NRGBA(decodeS3TC(b[8:], x, y, false))
		c.A = decode3Dc(b, x, y)

		return c
	case ImageFormatATI1N:
		return color.Gray{Y: decode3Dc(b, x, y)}
	case ImageFormatATI2N:
		return color.RGBA{
			R: decode3Dc(b, x, y),
			G: decode3Dc(b[8:], x, y),
			A: 255,
		}
	default:
		panic("unexpected compressed color format " + format.String())
	}
}

func decodeCompressedRGBA64(format ImageFormat, b []byte, x, y int) color.RGBA64 {
	// TODO: rewrite to not allocate
	return color.RGBA64Model.Convert(decodeCompressedColor(format, b, x, y)).(color.RGBA64)
}

func rgbaFrom565(bits uint16) color.NRGBA {
	r := bits >> 11
	g := bits >> 5 & 63
	b := bits & 31

	return color.NRGBA{
		R: uint8(r<<3 | r>>2),
		G: uint8(g<<2 | g>>4),
		B: uint8(b<<3 | b>>2),
		A: 255,
	}
}

func rgbaFrom5551(bits uint16) color.NRGBA {
	r := bits >> 11
	g := bits >> 6 & 31
	b := bits >> 1 & 31
	a := bits & 1

	return color.NRGBA{
		R: uint8(r<<3 | r>>2),
		G: uint8(g<<3 | g>>2),
		B: uint8(b<<3 | b>>2),
		A: uint8(a) * 255,
	}
}

func rgbaFrom4444(bits uint16) color.NRGBA {
	r := bits >> 12
	g := bits >> 8 & 15
	b := bits >> 4 & 15
	a := bits & 15

	return color.NRGBA{
		R: uint8(r<<4 | r),
		G: uint8(g<<4 | g),
		B: uint8(b<<4 | b),
		A: uint8(a<<4 | a),
	}
}

func rgbaFrom1010102(bits uint32) color.NRGBA64 {
	r := bits >> 22
	g := bits >> 12 & 1023
	b := bits >> 2 & 1023
	a := bits & 3
	a |= a << 2
	a |= a << 4
	a |= a << 8

	return color.NRGBA64{
		R: uint16(r<<6 | r>>4),
		G: uint16(g<<6 | g>>4),
		B: uint16(b<<6 | b>>4),
		A: uint16(a),
	}
}

func mixRGBA(c0 color.RGBA, m0 uint16, c1 color.RGBA, m1 uint16) color.RGBA {
	total := m0 + m1
	return color.RGBA{
		R: uint8((uint16(c0.R)*m0 + uint16(c1.R)*m1) / total),
		G: uint8((uint16(c0.G)*m0 + uint16(c1.G)*m1) / total),
		B: uint8((uint16(c0.B)*m0 + uint16(c1.B)*m1) / total),
		A: uint8((uint16(c0.A)*m0 + uint16(c1.A)*m1) / total),
	}
}
