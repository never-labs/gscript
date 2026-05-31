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
