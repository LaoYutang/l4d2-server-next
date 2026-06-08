package vtf

import "image/color"

type FloatColor struct {
	R, G, B, A float32
}

func (c FloatColor) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R * c.A * 0xffff)
	g = uint32(c.G * c.A * 0xffff)
	b = uint32(c.B * c.A * 0xffff)
	a = uint32(c.A * 0xffff)

	return
}

var FloatColorModel = color.ModelFunc(floatColorModel)

func floatColorModel(c color.Color) color.Color {
	// don't lose precision for the identity conversion
	if fc, ok := c.(FloatColor); ok {
		return fc
	}

	r, g, b, a := c.RGBA()

	invA := float32(1)
	if a > 0 {
		invA = 0xffff / float32(a)
	}

	return FloatColor{
		R: float32(r) / 0xffff * invA,
		G: float32(g) / 0xffff * invA,
		B: float32(b) / 0xffff * invA,
		A: float32(a) / 0xffff,
	}
}
