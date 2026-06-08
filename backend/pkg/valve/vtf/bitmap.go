package vtf

import (
	"image"
	"image/color"
)

func makeSingleImage(width, height, depth int, format ImageFormat, pix []byte) *Bitmap {
	if pix == nil {
		pix = make([]byte, memoryRequiredForImage(width, height, depth, 1, format))
	}

	stride := int(imageFormatInfos[format].bytes)
	if stride == 0 {
		stride = 1
	}

	return &Bitmap{
		Width:  width,
		Height: height,
		Depth:  depth,
		Stride: width * stride,
		Format: format,
		Pix:    pix,
	}
}

func makeImageSet(width, height, depth, mipmaps, frames int, format ImageFormat, pix []byte) [][]*Bitmap {
	if pix == nil {
		pix = make([]byte, memoryRequiredForImage(width, height, depth, mipmaps, format)*frames)
	}

	if mipmaps == 0 {
		mipmaps = numMipMapLevels(width, height, depth)
	}

	stride := int(imageFormatInfos[format].bytes)
	if stride == 0 {
		stride = 1
	}

	prealloc1 := make([]Bitmap, mipmaps*frames)
	prealloc2 := make([]*Bitmap, mipmaps*frames)
	bitmaps := make([][]*Bitmap, frames)

	for i := range prealloc2 {
		prealloc2[i] = &prealloc1[i]
	}

	for i := range bitmaps {
		bitmaps[i] = prealloc2[:mipmaps]
		prealloc2 = prealloc2[mipmaps:]
	}

	splitBitmapPixels(bitmaps, 0, width, height, depth, mipmaps-1, format, pix)

	return bitmaps
}

func splitBitmapPixels(bitmaps [][]*Bitmap, mip, width, height, depth, maxMip int, format ImageFormat, pix []byte) []byte {
	if mip < maxMip {
		width2 := width >> 1
		height2 := height >> 1
		depth2 := depth >> 1

		if width2 < 1 {
			width2 = 1
		}
		if height2 < 1 {
			height2 = 1
		}
		if depth2 < 1 {
			depth2 = 1
		}

		pix = splitBitmapPixels(bitmaps, mip+1, width2, height2, depth2, maxMip, format, pix)
	}

	size := memoryRequiredForImage(width, height, depth, 1, format)
	stride := width
	switch format {
	case ImageFormatDXT1, ImageFormatDXT1OneBitAlpha, ImageFormatATI1N:
		width2 := width >> 2
		if width2 < 1 {
			width2 = 1
		}
		stride = width2 * 8
	case ImageFormatDXT3, ImageFormatDXT5, ImageFormatATI2N:
		width2 := width >> 2
		if width2 < 1 {
			width2 = 1
		}
		stride = width2 * 16
	default:
		stride *= int(imageFormatInfos[format].bytes)
	}

	for i := range bitmaps {
		*bitmaps[i][mip] = Bitmap{
			Width:  width,
			Height: height,
			Depth:  depth,
			Stride: stride,
			Format: format,
			Pix:    pix[:size],
		}

		pix = pix[size:]
	}

	return pix
}

type Bitmap struct {
	Width  int
	Height int
	Depth  int
	Stride int
	Format ImageFormat
	Pix    []byte
}

func (b *Bitmap) ColorModel() color.Model {
	return imageFormatInfos[b.Format].model
}

func (b *Bitmap) Bounds() image.Rectangle {
	return image.Rect(0, 0, b.Width, b.Height)
}

func (b *Bitmap) At(x, y int) color.Color {
	switch b.Format {
	case ImageFormatDXT1, ImageFormatDXT1OneBitAlpha, ImageFormatATI1N:
		x2, y2 := x>>2, y>>2
		x3, y3 := x&3, y&3
		off := 8*x2 + b.Stride*y2
		return decodeCompressedColor(b.Format, b.Pix[off:], x3, y3)
	case ImageFormatDXT3, ImageFormatDXT5, ImageFormatATI2N:
		x2, y2 := x>>2, y>>2
		x3, y3 := x&3, y&3
		off := 16*x2 + b.Stride*y2
		return decodeCompressedColor(b.Format, b.Pix[off:], x3, y3)
	default:
		off := int(imageFormatInfos[b.Format].bytes)*x + b.Stride*y
		return decodeColor(b.Format, b.Pix[off:])
	}
}

func (b *Bitmap) RGBA64At(x, y int) color.RGBA64 {
	switch b.Format {
	case ImageFormatDXT1, ImageFormatDXT1OneBitAlpha, ImageFormatATI1N:
		x2, y2 := x>>2, y>>2
		x3, y3 := x&3, y&3
		off := 8*x2 + b.Stride*y2
		return decodeCompressedRGBA64(b.Format, b.Pix[off:], x3, y3)
	case ImageFormatDXT3, ImageFormatDXT5, ImageFormatATI2N:
		x2, y2 := x>>2, y>>2
		x3, y3 := x&3, y&3
		off := 16*x2 + b.Stride*y2
		return decodeCompressedRGBA64(b.Format, b.Pix[off:], x3, y3)
	default:
		off := int(imageFormatInfos[b.Format].bytes)*x + b.Stride*y
		return decodeRGBA64(b.Format, b.Pix[off:])
	}
}
