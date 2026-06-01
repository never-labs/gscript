package vec

import "math"

type Vec2 struct {
	X float64
	Y float64
}

type Vec3 struct {
	X float64
	Y float64
	Z float64
}

func Add2(a, b Vec2) Vec2 {
	return Vec2{X: a.X + b.X, Y: a.Y + b.Y}
}

func Sub2(a, b Vec2) Vec2 {
	return Vec2{X: a.X - b.X, Y: a.Y - b.Y}
}

func Scale2(v Vec2, scalar float64) Vec2 {
	return Vec2{X: v.X * scalar, Y: v.Y * scalar}
}

func Div2(v Vec2, scalar float64) Vec2 {
	return Vec2{X: v.X / scalar, Y: v.Y / scalar}
}

func Neg2(v Vec2) Vec2 {
	return Vec2{X: -v.X, Y: -v.Y}
}

func Dot2(a, b Vec2) float64 {
	return a.X*b.X + a.Y*b.Y
}

func LengthSq2(v Vec2) float64 {
	return Dot2(v, v)
}

func Length2(v Vec2) float64 {
	return math.Sqrt(LengthSq2(v))
}

func Normalize2(v Vec2) Vec2 {
	length := Length2(v)
	if length == 0 {
		return Vec2{}
	}
	return Div2(v, length)
}

func Angle2(v Vec2) float64 {
	return math.Atan2(v.Y, v.X)
}

func Rotate2(v Vec2, angle float64) Vec2 {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return Vec2{X: v.X*cos - v.Y*sin, Y: v.X*sin + v.Y*cos}
}

func Lerp2(a, b Vec2, t float64) Vec2 {
	return Vec2{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}
}

func DistSq2(a, b Vec2) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return dx*dx + dy*dy
}

func Dist2(a, b Vec2) float64 {
	return math.Sqrt(DistSq2(a, b))
}

func Reflect2(v, normal Vec2) Vec2 {
	dot := Dot2(v, normal)
	return Vec2{X: v.X - 2*dot*normal.X, Y: v.Y - 2*dot*normal.Y}
}

func Perp2(v Vec2) Vec2 {
	return Vec2{X: -v.Y, Y: v.X}
}

func Clamp2(v, min, max Vec2) Vec2 {
	return Vec2{
		X: math.Max(min.X, math.Min(max.X, v.X)),
		Y: math.Max(min.Y, math.Min(max.Y, v.Y)),
	}
}

func Add3(a, b Vec3) Vec3 {
	return Vec3{X: a.X + b.X, Y: a.Y + b.Y, Z: a.Z + b.Z}
}

func Sub3(a, b Vec3) Vec3 {
	return Vec3{X: a.X - b.X, Y: a.Y - b.Y, Z: a.Z - b.Z}
}

func Scale3(v Vec3, scalar float64) Vec3 {
	return Vec3{X: v.X * scalar, Y: v.Y * scalar, Z: v.Z * scalar}
}

func Div3(v Vec3, scalar float64) Vec3 {
	return Vec3{X: v.X / scalar, Y: v.Y / scalar, Z: v.Z / scalar}
}

func Neg3(v Vec3) Vec3 {
	return Vec3{X: -v.X, Y: -v.Y, Z: -v.Z}
}

func Dot3(a, b Vec3) float64 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}

func Cross3(a, b Vec3) Vec3 {
	return Vec3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

func LengthSq3(v Vec3) float64 {
	return Dot3(v, v)
}

func Length3(v Vec3) float64 {
	return math.Sqrt(LengthSq3(v))
}

func Normalize3(v Vec3) Vec3 {
	length := Length3(v)
	if length == 0 {
		return Vec3{}
	}
	return Div3(v, length)
}

func Lerp3(a, b Vec3, t float64) Vec3 {
	return Vec3{
		X: a.X + (b.X-a.X)*t,
		Y: a.Y + (b.Y-a.Y)*t,
		Z: a.Z + (b.Z-a.Z)*t,
	}
}

func DistSq3(a, b Vec3) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return dx*dx + dy*dy + dz*dz
}

func Dist3(a, b Vec3) float64 {
	return math.Sqrt(DistSq3(a, b))
}
