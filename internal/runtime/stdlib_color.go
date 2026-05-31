package runtime

import (
	"fmt"

	stdcolor "github.com/never-labs/gscript/internal/stdlib/data/color"
)

// --------------------------------------------------------------------------
// Color metatable (shared across all color instances)
// --------------------------------------------------------------------------

var colorMeta *Table

func newColorMeta() *Table {
	mt := NewTable()

	getRGBA := func(v Value) (float64, float64, float64, float64) {
		tbl := v.Table()
		return toFloat(tbl.RawGet(StringValue("r"))),
			toFloat(tbl.RawGet(StringValue("g"))),
			toFloat(tbl.RawGet(StringValue("b"))),
			toFloat(tbl.RawGet(StringValue("a")))
	}

	// __add: component-wise add (clamped to 1)
	mt.RawSet(StringValue("__add"), FunctionValue(&GoFunction{
		Name: "color.__add",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("color.__add requires 2 arguments")
			}
			r1, g1, b1, a1 := getRGBA(args[0])
			r2, g2, b2, a2 := getRGBA(args[1])
			return []Value{makeColorRGBA(stdcolor.Add(
				stdcolor.RGBA{R: r1, G: g1, B: b1, A: a1},
				stdcolor.RGBA{R: r2, G: g2, B: b2, A: a2},
			))}, nil
		},
	}))

	// __mul: scale by float or component-wise multiply by color
	mt.RawSet(StringValue("__mul"), FunctionValue(&GoFunction{
		Name: "color.__mul",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("color.__mul requires 2 arguments")
			}
			a, b := args[0], args[1]
			if a.IsTable() && (b.IsNumber() || b.IsInt()) {
				r, g, bl, al := getRGBA(a)
				s := toFloat(b)
				return []Value{makeColorRGBA(stdcolor.Scale(stdcolor.RGBA{R: r, G: g, B: bl, A: al}, s))}, nil
			}
			if (a.IsNumber() || a.IsInt()) && b.IsTable() {
				s := toFloat(a)
				r, g, bl, al := getRGBA(b)
				return []Value{makeColorRGBA(stdcolor.Scale(stdcolor.RGBA{R: r, G: g, B: bl, A: al}, s))}, nil
			}
			// Component-wise multiply of two colors
			if a.IsTable() && b.IsTable() {
				r1, g1, b1, a1 := getRGBA(a)
				r2, g2, b2, a2 := getRGBA(b)
				return []Value{makeColorRGBA(stdcolor.Mul(
					stdcolor.RGBA{R: r1, G: g1, B: b1, A: a1},
					stdcolor.RGBA{R: r2, G: g2, B: b2, A: a2},
				))}, nil
			}
			return nil, fmt.Errorf("color.__mul: unsupported operand types")
		},
	}))

	// __eq: compare r,g,b,a
	mt.RawSet(StringValue("__eq"), FunctionValue(&GoFunction{
		Name: "color.__eq",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("color.__eq requires 2 arguments")
			}
			r1, g1, b1, a1 := getRGBA(args[0])
			r2, g2, b2, a2 := getRGBA(args[1])
			return []Value{BoolValue(r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2)}, nil
		},
	}))

	return mt
}

// --------------------------------------------------------------------------
// Internal helper to create color table values
// --------------------------------------------------------------------------

func makeColorValue(r, g, b, a float64) Value {
	return makeColorRGBA(stdcolor.RGBA{R: r, G: g, B: b, A: a})
}

func makeColorRGBA(c stdcolor.RGBA) Value {
	t := NewTable()
	t.RawSet(StringValue("r"), FloatValue(c.R))
	t.RawSet(StringValue("g"), FloatValue(c.G))
	t.RawSet(StringValue("b"), FloatValue(c.B))
	t.RawSet(StringValue("a"), FloatValue(c.A))
	t.RawSet(StringValue("_type"), StringValue("color"))
	t.SetMetatable(colorMeta)
	return TableValue(t)
}

func isColorValue(v Value) bool {
	if !v.IsTable() {
		return false
	}
	ty := v.Table().RawGet(StringValue("_type"))
	return ty.IsString() && ty.Str() == "color"
}

// --------------------------------------------------------------------------
// buildColorLib creates the "color" standard library table.
// --------------------------------------------------------------------------

func buildColorLib() *Table {
	colorMeta = newColorMeta()

	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "color." + name,
			Fn:   fn,
		}))
	}

	getRGBA := func(v Value) (float64, float64, float64, float64) {
		tbl := v.Table()
		return toFloat(tbl.RawGet(StringValue("r"))),
			toFloat(tbl.RawGet(StringValue("g"))),
			toFloat(tbl.RawGet(StringValue("b"))),
			toFloat(tbl.RawGet(StringValue("a")))
	}

	// ----------------------------------------------------------------
	// Constructors
	// ----------------------------------------------------------------

	// color.new(r, g, b [, a])  -- r,g,b,a in [0, 1]
	set("new", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'color.new': expected at least 3 arguments")
		}
		r := toFloat(args[0])
		g := toFloat(args[1])
		b := toFloat(args[2])
		a := 1.0
		if len(args) >= 4 {
			a = toFloat(args[3])
		}
		return []Value{makeColorValue(r, g, b, a)}, nil
	})

	// color.rgb(r, g, b) -- r,g,b in [0, 255]
	set("rgb", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'color.rgb': expected 3 arguments")
		}
		return []Value{makeColorRGBA(stdcolor.FromRGB(toFloat(args[0]), toFloat(args[1]), toFloat(args[2])))}, nil
	})

	// color.rgba(r, g, b, a) -- r,g,b,a in [0, 255]
	set("rgba", func(args []Value) ([]Value, error) {
		if len(args) < 4 {
			return nil, fmt.Errorf("bad argument to 'color.rgba': expected 4 arguments")
		}
		return []Value{makeColorRGBA(stdcolor.FromRGBA(toFloat(args[0]), toFloat(args[1]), toFloat(args[2]), toFloat(args[3])))}, nil
	})

	// color.fromHex(hexStr) -- parses "#RGB", "#RRGGBB", "#RRGGBBAA"
	set("fromHex", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return []Value{NilValue(), StringValue("bad argument to 'color.fromHex': expected string")}, nil
		}
		c, err := stdcolor.FromHex(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{makeColorRGBA(c)}, nil
	})

	// color.toHex(c) -> "#RRGGBB"
	set("toHex", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument to 'color.toHex'")
		}
		r, g, b, _ := getRGBA(args[0])
		return []Value{StringValue(stdcolor.ToHex(stdcolor.RGBA{R: r, G: g, B: b, A: 1}))}, nil
	})

	// ----------------------------------------------------------------
	// HSV conversions
	// ----------------------------------------------------------------

	// color.fromHSV(h, s, v) -- h in [0,360], s,v in [0,1]
	set("fromHSV", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'color.fromHSV'")
		}
		h := toFloat(args[0])
		s := toFloat(args[1])
		v := toFloat(args[2])
		return []Value{makeColorRGBA(stdcolor.FromHSV(h, s, v))}, nil
	})

	// color.toHSV(c) -> h, s, v
	set("toHSV", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument to 'color.toHSV'")
		}
		r, g, b, _ := getRGBA(args[0])
		h, s, v := stdcolor.ToHSV(stdcolor.RGBA{R: r, G: g, B: b, A: 1})
		return []Value{FloatValue(h), FloatValue(s), FloatValue(v)}, nil
	})

	// ----------------------------------------------------------------
	// HSL conversions
	// ----------------------------------------------------------------

	// color.fromHSL(h, s, l) -- h in [0,360], s,l in [0,1]
	set("fromHSL", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'color.fromHSL'")
		}
		h := toFloat(args[0])
		s := toFloat(args[1])
		l := toFloat(args[2])
		return []Value{makeColorRGBA(stdcolor.FromHSL(h, s, l))}, nil
	})

	// color.toHSL(c) -> h, s, l
	set("toHSL", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument to 'color.toHSL'")
		}
		r, g, b, _ := getRGBA(args[0])
		h, s, l := stdcolor.ToHSL(stdcolor.RGBA{R: r, G: g, B: b, A: 1})
		return []Value{FloatValue(h), FloatValue(s), FloatValue(l)}, nil
	})

	// ----------------------------------------------------------------
	// Interpolation / manipulation
	// ----------------------------------------------------------------

	// color.lerp(c1, c2, t) -- linear interpolation in RGB space
	set("lerp", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'color.lerp'")
		}
		r1, g1, b1, a1 := getRGBA(args[0])
		r2, g2, b2, a2 := getRGBA(args[1])
		t := toFloat(args[2])
		return []Value{makeColorRGBA(stdcolor.Lerp(
			stdcolor.RGBA{R: r1, G: g1, B: b1, A: a1},
			stdcolor.RGBA{R: r2, G: g2, B: b2, A: a2},
			t,
		))}, nil
	})

	// color.mix -- alias for lerp
	set("mix", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'color.mix'")
		}
		r1, g1, b1, a1 := getRGBA(args[0])
		r2, g2, b2, a2 := getRGBA(args[1])
		t := toFloat(args[2])
		return []Value{makeColorRGBA(stdcolor.Lerp(
			stdcolor.RGBA{R: r1, G: g1, B: b1, A: a1},
			stdcolor.RGBA{R: r2, G: g2, B: b2, A: a2},
			t,
		))}, nil
	})

	// color.darken(c, amount) -- reduce brightness by amount (0-1)
	set("darken", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'color.darken'")
		}
		r, g, b, a := getRGBA(args[0])
		amount := toFloat(args[1])
		return []Value{makeColorRGBA(stdcolor.Darken(stdcolor.RGBA{R: r, G: g, B: b, A: a}, amount))}, nil
	})

	// color.lighten(c, amount) -- increase brightness
	set("lighten", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'color.lighten'")
		}
		r, g, b, a := getRGBA(args[0])
		amount := toFloat(args[1])
		return []Value{makeColorRGBA(stdcolor.Lighten(stdcolor.RGBA{R: r, G: g, B: b, A: a}, amount))}, nil
	})

	// color.alpha(c, a) -- same color with new alpha
	set("alpha", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'color.alpha'")
		}
		r, g, b, _ := getRGBA(args[0])
		newA := toFloat(args[1])
		return []Value{makeColorRGBA(stdcolor.WithAlpha(stdcolor.RGBA{R: r, G: g, B: b, A: 1}, newA))}, nil
	})

	// color.withAlpha -- alias for alpha
	set("withAlpha", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'color.withAlpha'")
		}
		r, g, b, _ := getRGBA(args[0])
		newA := toFloat(args[1])
		return []Value{makeColorRGBA(stdcolor.WithAlpha(stdcolor.RGBA{R: r, G: g, B: b, A: 1}, newA))}, nil
	})

	// color.invert(c)
	set("invert", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument to 'color.invert'")
		}
		r, g, b, a := getRGBA(args[0])
		return []Value{makeColorRGBA(stdcolor.Invert(stdcolor.RGBA{R: r, G: g, B: b, A: a}))}, nil
	})

	// color.grayscale(c) -- using luminance weights
	set("grayscale", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument to 'color.grayscale'")
		}
		r, g, b, a := getRGBA(args[0])
		return []Value{makeColorRGBA(stdcolor.Grayscale(stdcolor.RGBA{R: r, G: g, B: b, A: a}))}, nil
	})

	// color.isColor(v)
	set("isColor", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return []Value{BoolValue(false)}, nil
		}
		return []Value{BoolValue(isColorValue(args[0]))}, nil
	})

	// ----------------------------------------------------------------
	// Named color constants
	// ----------------------------------------------------------------
	t.RawSet(StringValue("RED"), makeColorValue(1, 0, 0, 1))
	t.RawSet(StringValue("GREEN"), makeColorValue(0, 1, 0, 1))
	t.RawSet(StringValue("BLUE"), makeColorValue(0, 0, 1, 1))
	t.RawSet(StringValue("WHITE"), makeColorValue(1, 1, 1, 1))
	t.RawSet(StringValue("BLACK"), makeColorValue(0, 0, 0, 1))
	t.RawSet(StringValue("YELLOW"), makeColorValue(1, 1, 0, 1))
	t.RawSet(StringValue("CYAN"), makeColorValue(0, 1, 1, 1))
	t.RawSet(StringValue("MAGENTA"), makeColorValue(1, 0, 1, 1))
	t.RawSet(StringValue("ORANGE"), makeColorValue(1, 0.5, 0, 1))
	t.RawSet(StringValue("PURPLE"), makeColorValue(0.5, 0, 0.5, 1))
	t.RawSet(StringValue("TRANSPARENT"), makeColorValue(0, 0, 0, 0))

	return t
}
