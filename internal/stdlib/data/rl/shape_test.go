package rl

import "testing"

func TestNamedColors(t *testing.T) {
	colors := NamedColors()
	if len(colors) != 26 {
		t.Fatalf("expected 26 colors, got %d", len(colors))
	}
	if colors[0].Name != "LIGHTGRAY" || colors[0].Color.R != 200 || colors[0].Color.A != 255 {
		t.Fatalf("unexpected first color: %+v", colors[0])
	}
	if colors[23].Name != "BLANK" || colors[23].Color.A != 0 {
		t.Fatalf("unexpected blank color: %+v", colors[23])
	}
	if colors[25].Name != "RAYWHITE" || colors[25].Color.R != 245 {
		t.Fatalf("unexpected last color: %+v", colors[25])
	}
}

func TestDefaultCamera2DShape(t *testing.T) {
	camera := DefaultCamera2DShape()
	if camera.Zoom != 1 {
		t.Fatalf("expected default zoom 1, got %v", camera.Zoom)
	}
	if camera.OffsetX != 0 || camera.TargetY != 0 || camera.Rotation != 0 {
		t.Fatalf("unexpected non-zero camera fields: %+v", camera)
	}
}

func TestKeyConstants(t *testing.T) {
	keys := KeyConstants()
	if len(keys) != 77 {
		t.Fatalf("expected 77 key constants, got %d", len(keys))
	}
	if keys[0] != (IntConstant{Name: "KEY_NULL", Value: 0}) {
		t.Fatalf("unexpected first key constant: %+v", keys[0])
	}
	if keys[18] != (IntConstant{Name: "KEY_A", Value: 65}) {
		t.Fatalf("unexpected KEY_A constant: %+v", keys[18])
	}
	if keys[76] != (IntConstant{Name: "KEY_RIGHT_ALT", Value: 346}) {
		t.Fatalf("unexpected last key constant: %+v", keys[76])
	}
}

func TestMouseButtonConstants(t *testing.T) {
	buttons := MouseButtonConstants()
	if len(buttons) != 3 {
		t.Fatalf("expected 3 mouse button constants, got %d", len(buttons))
	}
	want := []IntConstant{
		{Name: "MOUSE_BUTTON_LEFT", Value: 0},
		{Name: "MOUSE_BUTTON_RIGHT", Value: 1},
		{Name: "MOUSE_BUTTON_MIDDLE", Value: 2},
	}
	for i := range want {
		if buttons[i] != want[i] {
			t.Fatalf("unexpected mouse button constant at %d: got %+v want %+v", i, buttons[i], want[i])
		}
	}
}
