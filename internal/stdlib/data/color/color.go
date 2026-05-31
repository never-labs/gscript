package color

import (
	"fmt"
	"math"
	"strings"
)

type RGBA struct {
	R float64
	G float64
	B float64
	A float64
}

func Clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func FromRGB(r, g, b float64) RGBA {
	return RGBA{R: r / 255, G: g / 255, B: b / 255, A: 1}
}

func FromRGBA(r, g, b, a float64) RGBA {
	return RGBA{R: r / 255, G: g / 255, B: b / 255, A: a / 255}
}

func FromHex(input string) (RGBA, error) {
	hex := strings.TrimPrefix(input, "#")
	a := uint8(255)

	var r, g, b uint8
	switch len(hex) {
	case 3:
		rb, ok1 := parseHexByte(string(hex[0]) + string(hex[0]))
		gb, ok2 := parseHexByte(string(hex[1]) + string(hex[1]))
		bb, ok3 := parseHexByte(string(hex[2]) + string(hex[2]))
		if !ok1 || !ok2 || !ok3 {
			return RGBA{}, fmt.Errorf("invalid hex color: %s", input)
		}
		r, g, b = rb, gb, bb
	case 6:
		rb, ok1 := parseHexByte(hex[0:2])
		gb, ok2 := parseHexByte(hex[2:4])
		bb, ok3 := parseHexByte(hex[4:6])
		if !ok1 || !ok2 || !ok3 {
			return RGBA{}, fmt.Errorf("invalid hex color: %s", input)
		}
		r, g, b = rb, gb, bb
	case 8:
		rb, ok1 := parseHexByte(hex[0:2])
		gb, ok2 := parseHexByte(hex[2:4])
		bb, ok3 := parseHexByte(hex[4:6])
		ab, ok4 := parseHexByte(hex[6:8])
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return RGBA{}, fmt.Errorf("invalid hex color: %s", input)
		}
		r, g, b, a = rb, gb, bb, ab
	default:
		return RGBA{}, fmt.Errorf("invalid hex color format: %s", input)
	}

	return RGBA{
		R: float64(r) / 255,
		G: float64(g) / 255,
		B: float64(b) / 255,
		A: float64(a) / 255,
	}, nil
}

func ToHex(c RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X",
		uint8(math.Round(c.R*255)),
		uint8(math.Round(c.G*255)),
		uint8(math.Round(c.B*255)),
	)
}

func FromHSV(h, s, v float64) RGBA {
	r, g, b := hsvToRGB(h, s, v)
	return RGBA{R: r, G: g, B: b, A: 1}
}

func ToHSV(c RGBA) (float64, float64, float64) {
	return rgbToHSV(c.R, c.G, c.B)
}

func FromHSL(h, s, l float64) RGBA {
	r, g, b := hslToRGB(h, s, l)
	return RGBA{R: r, G: g, B: b, A: 1}
}

func ToHSL(c RGBA) (float64, float64, float64) {
	return rgbToHSL(c.R, c.G, c.B)
}

func Add(a, b RGBA) RGBA {
	return RGBA{
		R: Clamp01(a.R + b.R),
		G: Clamp01(a.G + b.G),
		B: Clamp01(a.B + b.B),
		A: Clamp01(a.A + b.A),
	}
}

func Scale(c RGBA, s float64) RGBA {
	return RGBA{R: c.R * s, G: c.G * s, B: c.B * s, A: c.A}
}

func Mul(a, b RGBA) RGBA {
	return RGBA{R: a.R * b.R, G: a.G * b.G, B: a.B * b.B, A: a.A * b.A}
}

func Lerp(a, b RGBA, t float64) RGBA {
	return RGBA{
		R: a.R + (b.R-a.R)*t,
		G: a.G + (b.G-a.G)*t,
		B: a.B + (b.B-a.B)*t,
		A: a.A + (b.A-a.A)*t,
	}
}

func Darken(c RGBA, amount float64) RGBA {
	factor := 1 - amount
	return RGBA{R: c.R * factor, G: c.G * factor, B: c.B * factor, A: c.A}
}

func Lighten(c RGBA, amount float64) RGBA {
	return RGBA{
		R: c.R + (1-c.R)*amount,
		G: c.G + (1-c.G)*amount,
		B: c.B + (1-c.B)*amount,
		A: c.A,
	}
}

func WithAlpha(c RGBA, a float64) RGBA {
	c.A = a
	return c
}

func Invert(c RGBA) RGBA {
	return RGBA{R: 1 - c.R, G: 1 - c.G, B: 1 - c.B, A: c.A}
}

func Grayscale(c RGBA) RGBA {
	lum := 0.2126*c.R + 0.7152*c.G + 0.0722*c.B
	return RGBA{R: lum, G: lum, B: lum, A: c.A}
}

func parseHexByte(s string) (uint8, bool) {
	if len(s) != 2 {
		return 0, false
	}
	var val uint8
	for _, c := range s {
		val <<= 4
		switch {
		case c >= '0' && c <= '9':
			val |= uint8(c - '0')
		case c >= 'a' && c <= 'f':
			val |= uint8(c-'a') + 10
		case c >= 'A' && c <= 'F':
			val |= uint8(c-'A') + 10
		default:
			return 0, false
		}
	}
	return val, true
}

func hsvToRGB(h, s, v float64) (float64, float64, float64) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c

	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return r + m, g + m, b + m
}

func rgbToHSV(r, g, b float64) (float64, float64, float64) {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	delta := max - min

	var h float64
	if delta == 0 {
		h = 0
	} else if max == r {
		h = 60 * math.Mod((g-b)/delta, 6)
	} else if max == g {
		h = 60 * ((b-r)/delta + 2)
	} else {
		h = 60 * ((r-g)/delta + 4)
	}
	if h < 0 {
		h += 360
	}

	var s float64
	if max != 0 {
		s = delta / max
	}

	return h, s, max
}

func hslToRGB(h, s, l float64) (float64, float64, float64) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2

	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return r + m, g + m, b + m
}

func rgbToHSL(r, g, b float64) (float64, float64, float64) {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l := (max + min) / 2
	delta := max - min

	if delta == 0 {
		return 0, 0, l
	}

	var s float64
	if l <= 0.5 {
		s = delta / (max + min)
	} else {
		s = delta / (2 - max - min)
	}

	var h float64
	if max == r {
		h = 60 * math.Mod((g-b)/delta, 6)
	} else if max == g {
		h = 60 * ((b-r)/delta + 2)
	} else {
		h = 60 * ((r-g)/delta + 4)
	}
	if h < 0 {
		h += 360
	}

	return h, s, l
}
