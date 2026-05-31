package runtime

import (
	"fmt"

	stdvec "github.com/never-labs/gscript/internal/stdlib/vec"
)

// --------------------------------------------------------------------------
// Vec2 metatable (shared across all vec2 instances)
// --------------------------------------------------------------------------

func newVec2Meta() *Table {
	mt := NewTable()

	// Helper to extract x, y from a vec2 table
	getXY := func(v Value) (float64, float64) {
		tbl := v.Table()
		return toFloat(tbl.RawGet(StringValue("x"))), toFloat(tbl.RawGet(StringValue("y")))
	}
	getVec2 := func(v Value) stdvec.Vec2 {
		x, y := getXY(v)
		return stdvec.Vec2{X: x, Y: y}
	}

	// __add: vec2 + vec2
	mt.RawSet(StringValue("__add"), FunctionValue(&GoFunction{
		Name: "vec2.__add",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("vec2.__add requires 2 arguments")
			}
			return []Value{makeVec2ValueFromStd(stdvec.Add2(getVec2(args[0]), getVec2(args[1])))}, nil
		},
	}))

	// __sub: vec2 - vec2
	mt.RawSet(StringValue("__sub"), FunctionValue(&GoFunction{
		Name: "vec2.__sub",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("vec2.__sub requires 2 arguments")
			}
			return []Value{makeVec2ValueFromStd(stdvec.Sub2(getVec2(args[0]), getVec2(args[1])))}, nil
		},
	}))

	// __mul: vec2 * scalar or scalar * vec2
	mt.RawSet(StringValue("__mul"), FunctionValue(&GoFunction{
		Name: "vec2.__mul",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("vec2.__mul requires 2 arguments")
			}
			a, b := args[0], args[1]
			if a.IsTable() && (b.IsNumber() || b.IsInt()) {
				ax, ay := getXY(a)
				s := toFloat(b)
				return []Value{makeVec2ValueFromStd(stdvec.Scale2(stdvec.Vec2{X: ax, Y: ay}, s))}, nil
			}
			if (a.IsNumber() || a.IsInt()) && b.IsTable() {
				s := toFloat(a)
				bx, by := getXY(b)
				return []Value{makeVec2ValueFromStd(stdvec.Scale2(stdvec.Vec2{X: bx, Y: by}, s))}, nil
			}
			return nil, fmt.Errorf("vec2.__mul: unsupported operand types")
		},
	}))

	// __div: vec2 / scalar
	mt.RawSet(StringValue("__div"), FunctionValue(&GoFunction{
		Name: "vec2.__div",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("vec2.__div requires 2 arguments")
			}
			s := toFloat(args[1])
			if s == 0 {
				return nil, fmt.Errorf("vec2.__div: division by zero")
			}
			return []Value{makeVec2ValueFromStd(stdvec.Div2(getVec2(args[0]), s))}, nil
		},
	}))

	// __unm: -vec2
	mt.RawSet(StringValue("__unm"), FunctionValue(&GoFunction{
		Name: "vec2.__unm",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("vec2.__unm requires 1 argument")
			}
			return []Value{makeVec2ValueFromStd(stdvec.Neg2(getVec2(args[0])))}, nil
		},
	}))

	// __eq: vec2 == vec2
	mt.RawSet(StringValue("__eq"), FunctionValue(&GoFunction{
		Name: "vec2.__eq",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("vec2.__eq requires 2 arguments")
			}
			return []Value{BoolValue(getVec2(args[0]) == getVec2(args[1]))}, nil
		},
	}))

	return mt
}

// --------------------------------------------------------------------------
// Vec3 metatable (shared across all vec3 instances)
// --------------------------------------------------------------------------

func newVec3Meta() *Table {
	mt := NewTable()

	getXYZ := func(v Value) (float64, float64, float64) {
		tbl := v.Table()
		return toFloat(tbl.RawGet(StringValue("x"))),
			toFloat(tbl.RawGet(StringValue("y"))),
			toFloat(tbl.RawGet(StringValue("z")))
	}
	getVec3 := func(v Value) stdvec.Vec3 {
		x, y, z := getXYZ(v)
		return stdvec.Vec3{X: x, Y: y, Z: z}
	}

	// __add
	mt.RawSet(StringValue("__add"), FunctionValue(&GoFunction{
		Name: "vec3.__add",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("vec3.__add requires 2 arguments")
			}
			return []Value{makeVec3ValueFromStd(stdvec.Add3(getVec3(args[0]), getVec3(args[1])))}, nil
		},
	}))

	// __sub
	mt.RawSet(StringValue("__sub"), FunctionValue(&GoFunction{
		Name: "vec3.__sub",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("vec3.__sub requires 2 arguments")
			}
			return []Value{makeVec3ValueFromStd(stdvec.Sub3(getVec3(args[0]), getVec3(args[1])))}, nil
		},
	}))

	// __mul
	mt.RawSet(StringValue("__mul"), FunctionValue(&GoFunction{
		Name: "vec3.__mul",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("vec3.__mul requires 2 arguments")
			}
			a, b := args[0], args[1]
			if a.IsTable() && (b.IsNumber() || b.IsInt()) {
				ax, ay, az := getXYZ(a)
				s := toFloat(b)
				return []Value{makeVec3ValueFromStd(stdvec.Scale3(stdvec.Vec3{X: ax, Y: ay, Z: az}, s))}, nil
			}
			if (a.IsNumber() || a.IsInt()) && b.IsTable() {
				s := toFloat(a)
				bx, by, bz := getXYZ(b)
				return []Value{makeVec3ValueFromStd(stdvec.Scale3(stdvec.Vec3{X: bx, Y: by, Z: bz}, s))}, nil
			}
			return nil, fmt.Errorf("vec3.__mul: unsupported operand types")
		},
	}))

	// __div
	mt.RawSet(StringValue("__div"), FunctionValue(&GoFunction{
		Name: "vec3.__div",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("vec3.__div requires 2 arguments")
			}
			s := toFloat(args[1])
			if s == 0 {
				return nil, fmt.Errorf("vec3.__div: division by zero")
			}
			return []Value{makeVec3ValueFromStd(stdvec.Div3(getVec3(args[0]), s))}, nil
		},
	}))

	// __unm
	mt.RawSet(StringValue("__unm"), FunctionValue(&GoFunction{
		Name: "vec3.__unm",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("vec3.__unm requires 1 argument")
			}
			return []Value{makeVec3ValueFromStd(stdvec.Neg3(getVec3(args[0])))}, nil
		},
	}))

	// __eq
	mt.RawSet(StringValue("__eq"), FunctionValue(&GoFunction{
		Name: "vec3.__eq",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("vec3.__eq requires 2 arguments")
			}
			return []Value{BoolValue(getVec3(args[0]) == getVec3(args[1]))}, nil
		},
	}))

	return mt
}

// --------------------------------------------------------------------------
// Internal helpers to create vec2/vec3 table values
// --------------------------------------------------------------------------

// Shared metatables (created once per buildVecLib call)
var (
	vec2Meta *Table
	vec3Meta *Table
)

func makeVec2Value(x, y float64) Value {
	t := NewTable()
	t.RawSet(StringValue("x"), FloatValue(x))
	t.RawSet(StringValue("y"), FloatValue(y))
	t.RawSet(StringValue("_type"), StringValue("vec2"))
	t.SetMetatable(vec2Meta)
	return TableValue(t)
}

func makeVec2ValueFromStd(v stdvec.Vec2) Value {
	return makeVec2Value(v.X, v.Y)
}

func makeVec3Value(x, y, z float64) Value {
	t := NewTable()
	t.RawSet(StringValue("x"), FloatValue(x))
	t.RawSet(StringValue("y"), FloatValue(y))
	t.RawSet(StringValue("z"), FloatValue(z))
	t.RawSet(StringValue("_type"), StringValue("vec3"))
	t.SetMetatable(vec3Meta)
	return TableValue(t)
}

func makeVec3ValueFromStd(v stdvec.Vec3) Value {
	return makeVec3Value(v.X, v.Y, v.Z)
}

// isVec2 checks if a value is a vec2 table (has _type == "vec2").
func isVec2(v Value) bool {
	if !v.IsTable() {
		return false
	}
	ty := v.Table().RawGet(StringValue("_type"))
	return ty.IsString() && ty.Str() == "vec2"
}

// isVec3 checks if a value is a vec3 table (has _type == "vec3").
func isVec3(v Value) bool {
	if !v.IsTable() {
		return false
	}
	ty := v.Table().RawGet(StringValue("_type"))
	return ty.IsString() && ty.Str() == "vec3"
}

// --------------------------------------------------------------------------
// buildVecLib creates the "vec" standard library table.
// --------------------------------------------------------------------------

func buildVecLib() *Table {
	// Create shared metatables
	vec2Meta = newVec2Meta()
	vec3Meta = newVec3Meta()

	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "vec." + name,
			Fn:   fn,
		}))
	}

	// ----------------------------------------------------------------
	// Vec2 constructors
	// ----------------------------------------------------------------

	// vec.vec2(x, y)
	set("vec2", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'vec.vec2': expected 2 arguments")
		}
		return []Value{makeVec2Value(toFloat(args[0]), toFloat(args[1]))}, nil
	})

	// vec.zero2()
	set("zero2", func(args []Value) ([]Value, error) {
		return []Value{makeVec2Value(0, 0)}, nil
	})

	// vec.one2()
	set("one2", func(args []Value) ([]Value, error) {
		return []Value{makeVec2Value(1, 1)}, nil
	})

	// vec.up()
	set("up", func(args []Value) ([]Value, error) {
		return []Value{makeVec2Value(0, 1)}, nil
	})

	// vec.right()
	set("right", func(args []Value) ([]Value, error) {
		return []Value{makeVec2Value(1, 0)}, nil
	})

	// ----------------------------------------------------------------
	// Vec2 utilities
	// ----------------------------------------------------------------

	getXY := func(v Value) (float64, float64) {
		tbl := v.Table()
		return toFloat(tbl.RawGet(StringValue("x"))), toFloat(tbl.RawGet(StringValue("y")))
	}
	getVec2 := func(v Value) stdvec.Vec2 {
		x, y := getXY(v)
		return stdvec.Vec2{X: x, Y: y}
	}

	// vec.dot2(v1, v2)
	set("dot2", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'vec.dot2'")
		}
		return []Value{FloatValue(stdvec.Dot2(getVec2(args[0]), getVec2(args[1])))}, nil
	})

	// vec.length2(v)
	set("length2", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument to 'vec.length2'")
		}
		return []Value{FloatValue(stdvec.Length2(getVec2(args[0])))}, nil
	})

	// vec.lengthSq2(v)
	set("lengthSq2", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument to 'vec.lengthSq2'")
		}
		return []Value{FloatValue(stdvec.LengthSq2(getVec2(args[0])))}, nil
	})

	// vec.normalize2(v)
	set("normalize2", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument to 'vec.normalize2'")
		}
		return []Value{makeVec2ValueFromStd(stdvec.Normalize2(getVec2(args[0])))}, nil
	})

	// vec.angle2(v)
	set("angle2", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument to 'vec.angle2'")
		}
		return []Value{FloatValue(stdvec.Angle2(getVec2(args[0])))}, nil
	})

	// vec.rotate2(v, angle)
	set("rotate2", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'vec.rotate2'")
		}
		angle := toFloat(args[1])
		return []Value{makeVec2ValueFromStd(stdvec.Rotate2(getVec2(args[0]), angle))}, nil
	})

	// vec.lerp2(v1, v2, t)
	set("lerp2", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'vec.lerp2'")
		}
		t := toFloat(args[2])
		return []Value{makeVec2ValueFromStd(stdvec.Lerp2(getVec2(args[0]), getVec2(args[1]), t))}, nil
	})

	// vec.dist2(v1, v2)
	set("dist2", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'vec.dist2'")
		}
		return []Value{FloatValue(stdvec.Dist2(getVec2(args[0]), getVec2(args[1])))}, nil
	})

	// vec.distSq2(v1, v2)
	set("distSq2", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'vec.distSq2'")
		}
		return []Value{FloatValue(stdvec.DistSq2(getVec2(args[0]), getVec2(args[1])))}, nil
	})

	// vec.reflect2(v, normal)
	set("reflect2", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'vec.reflect2'")
		}
		return []Value{makeVec2ValueFromStd(stdvec.Reflect2(getVec2(args[0]), getVec2(args[1])))}, nil
	})

	// vec.perp2(v) -> (-y, x)
	set("perp2", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument to 'vec.perp2'")
		}
		return []Value{makeVec2ValueFromStd(stdvec.Perp2(getVec2(args[0])))}, nil
	})

	// vec.clamp2(v, min, max)
	// min and max can be floats (clamp each component uniformly) or vec2 tables
	set("clamp2", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'vec.clamp2'")
		}
		v := getVec2(args[0])

		var minX, minY, maxX, maxY float64
		if args[1].IsTable() {
			minX, minY = getXY(args[1])
		} else {
			minX = toFloat(args[1])
			minY = minX
		}
		if args[2].IsTable() {
			maxX, maxY = getXY(args[2])
		} else {
			maxX = toFloat(args[2])
			maxY = maxX
		}

		return []Value{makeVec2ValueFromStd(stdvec.Clamp2(v, stdvec.Vec2{X: minX, Y: minY}, stdvec.Vec2{X: maxX, Y: maxY}))}, nil
	})

	// vec.isVec2(v)
	set("isVec2", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return []Value{BoolValue(false)}, nil
		}
		return []Value{BoolValue(isVec2(args[0]))}, nil
	})

	// ----------------------------------------------------------------
	// Vec3 constructors
	// ----------------------------------------------------------------

	getXYZ := func(v Value) (float64, float64, float64) {
		tbl := v.Table()
		return toFloat(tbl.RawGet(StringValue("x"))),
			toFloat(tbl.RawGet(StringValue("y"))),
			toFloat(tbl.RawGet(StringValue("z")))
	}
	getVec3 := func(v Value) stdvec.Vec3 {
		x, y, z := getXYZ(v)
		return stdvec.Vec3{X: x, Y: y, Z: z}
	}

	// vec.vec3(x, y, z)
	set("vec3", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'vec.vec3': expected 3 arguments")
		}
		return []Value{makeVec3Value(toFloat(args[0]), toFloat(args[1]), toFloat(args[2]))}, nil
	})

	// vec.zero3()
	set("zero3", func(args []Value) ([]Value, error) {
		return []Value{makeVec3Value(0, 0, 0)}, nil
	})

	// vec.one3()
	set("one3", func(args []Value) ([]Value, error) {
		return []Value{makeVec3Value(1, 1, 1)}, nil
	})

	// ----------------------------------------------------------------
	// Vec3 utilities
	// ----------------------------------------------------------------

	// vec.dot3(v1, v2)
	set("dot3", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'vec.dot3'")
		}
		return []Value{FloatValue(stdvec.Dot3(getVec3(args[0]), getVec3(args[1])))}, nil
	})

	// vec.cross3(v1, v2)
	set("cross3", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'vec.cross3'")
		}
		return []Value{makeVec3ValueFromStd(stdvec.Cross3(getVec3(args[0]), getVec3(args[1])))}, nil
	})

	// vec.length3(v)
	set("length3", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument to 'vec.length3'")
		}
		return []Value{FloatValue(stdvec.Length3(getVec3(args[0])))}, nil
	})

	// vec.normalize3(v)
	set("normalize3", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument to 'vec.normalize3'")
		}
		return []Value{makeVec3ValueFromStd(stdvec.Normalize3(getVec3(args[0])))}, nil
	})

	// vec.lerp3(v1, v2, t)
	set("lerp3", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'vec.lerp3'")
		}
		t := toFloat(args[2])
		return []Value{makeVec3ValueFromStd(stdvec.Lerp3(getVec3(args[0]), getVec3(args[1]), t))}, nil
	})

	// vec.dist3(v1, v2)
	set("dist3", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'vec.dist3'")
		}
		return []Value{FloatValue(stdvec.Dist3(getVec3(args[0]), getVec3(args[1])))}, nil
	})

	// vec.isVec3(v)
	set("isVec3", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return []Value{BoolValue(false)}, nil
		}
		return []Value{BoolValue(isVec3(args[0]))}, nil
	})

	return t
}
