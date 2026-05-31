package rl

import "image/color"

// Camera2DShape is the scalar camera shape accepted by the rl stdlib.
// Runtime code owns script value conversion; this package owns field defaults
// and names that are independent from the VM.
type Camera2DShape struct {
	OffsetX  float64
	OffsetY  float64
	TargetX  float64
	TargetY  float64
	Rotation float64
	Zoom     float64
}

// DefaultCamera2DShape returns the camera used when the script value is not a
// table. This preserves the existing rl.beginMode2D fallback.
func DefaultCamera2DShape() Camera2DShape {
	return Camera2DShape{Zoom: 1}
}

type NamedColor struct {
	Name  string
	Color color.RGBA
}

func Color(r, g, b, a uint8) color.RGBA {
	return color.RGBA{R: r, G: g, B: b, A: a}
}

// NamedColors returns the raylib color constants exposed by the rl stdlib in
// registration order.
func NamedColors() []NamedColor {
	return []NamedColor{
		{"LIGHTGRAY", Color(200, 200, 200, 255)},
		{"GRAY", Color(130, 130, 130, 255)},
		{"DARKGRAY", Color(80, 80, 80, 255)},
		{"YELLOW", Color(253, 249, 0, 255)},
		{"GOLD", Color(255, 203, 0, 255)},
		{"ORANGE", Color(255, 161, 0, 255)},
		{"PINK", Color(255, 109, 194, 255)},
		{"RED", Color(230, 41, 55, 255)},
		{"MAROON", Color(190, 33, 55, 255)},
		{"GREEN", Color(0, 228, 48, 255)},
		{"LIME", Color(0, 158, 47, 255)},
		{"DARKGREEN", Color(0, 117, 44, 255)},
		{"SKYBLUE", Color(102, 191, 255, 255)},
		{"BLUE", Color(0, 121, 241, 255)},
		{"DARKBLUE", Color(0, 82, 172, 255)},
		{"PURPLE", Color(200, 122, 255, 255)},
		{"VIOLET", Color(135, 60, 190, 255)},
		{"DARKPURPLE", Color(112, 31, 126, 255)},
		{"BEIGE", Color(211, 176, 131, 255)},
		{"BROWN", Color(127, 106, 79, 255)},
		{"DARKBROWN", Color(76, 63, 47, 255)},
		{"WHITE", Color(255, 255, 255, 255)},
		{"BLACK", Color(0, 0, 0, 255)},
		{"BLANK", Color(0, 0, 0, 0)},
		{"MAGENTA", Color(255, 0, 255, 255)},
		{"RAYWHITE", Color(245, 245, 245, 255)},
	}
}
