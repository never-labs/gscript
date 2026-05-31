package vec

import "testing"

func TestVec2Operations(t *testing.T) {
	a := Vec2{X: 3, Y: 4}
	b := Vec2{X: 1, Y: 2}

	if got := Add2(a, b); got != (Vec2{X: 4, Y: 6}) {
		t.Fatalf("Add2 = %+v", got)
	}
	if got := Sub2(a, b); got != (Vec2{X: 2, Y: 2}) {
		t.Fatalf("Sub2 = %+v", got)
	}
	if got := Scale2(a, 2); got != (Vec2{X: 6, Y: 8}) {
		t.Fatalf("Scale2 = %+v", got)
	}
	if got := Dot2(a, b); got != 11 {
		t.Fatalf("Dot2 = %v", got)
	}
	if got := Length2(a); got != 5 {
		t.Fatalf("Length2 = %v", got)
	}
	if got := Normalize2(Vec2{}); got != (Vec2{}) {
		t.Fatalf("Normalize2 zero = %+v", got)
	}
	if got := Perp2(b); got != (Vec2{X: -2, Y: 1}) {
		t.Fatalf("Perp2 = %+v", got)
	}
	if got := Clamp2(Vec2{X: -1, Y: 9}, Vec2{}, Vec2{X: 5, Y: 6}); got != (Vec2{X: 0, Y: 6}) {
		t.Fatalf("Clamp2 = %+v", got)
	}
}

func TestVec3Operations(t *testing.T) {
	a := Vec3{X: 1, Y: 2, Z: 3}
	b := Vec3{X: 4, Y: 5, Z: 6}

	if got := Add3(a, b); got != (Vec3{X: 5, Y: 7, Z: 9}) {
		t.Fatalf("Add3 = %+v", got)
	}
	if got := Sub3(b, a); got != (Vec3{X: 3, Y: 3, Z: 3}) {
		t.Fatalf("Sub3 = %+v", got)
	}
	if got := Scale3(a, 3); got != (Vec3{X: 3, Y: 6, Z: 9}) {
		t.Fatalf("Scale3 = %+v", got)
	}
	if got := Dot3(a, b); got != 32 {
		t.Fatalf("Dot3 = %v", got)
	}
	if got := Cross3(Vec3{X: 1}, Vec3{Y: 1}); got != (Vec3{Z: 1}) {
		t.Fatalf("Cross3 = %+v", got)
	}
	if got := Normalize3(Vec3{}); got != (Vec3{}) {
		t.Fatalf("Normalize3 zero = %+v", got)
	}
	if got := DistSq3(a, b); got != 27 {
		t.Fatalf("DistSq3 = %v", got)
	}
}
