//go:build darwin && arm64

package tests_test

import "testing"

// ============================================================================
// GETGLOBAL native compilation tests
// ============================================================================

func TestInv_GetGlobal_Native(t *testing.T) {
	// Global variable read in loop should work correctly
	assertVMEqualsJIT(t, `
        data := {10, 20, 30, 40, 50}
        s := 0
        for i := 1; i <= 5; i++ {
            s = s + data[i]
        }
        result := s`, "result")
}

func TestInv_GetGlobal_TableFieldRead(t *testing.T) {
	// Global table + field read in loop
	assertVMEqualsJIT(t, `
        obj := {x: 1.0, y: 2.0}
        s := 0.0
        for i := 1; i <= 50; i++ {
            s = s + obj.x + obj.y
        }
        result := s`, "result")
}

func TestInv_GetGlobal_TableFieldWrite(t *testing.T) {
	// Global table + field write in loop
	assertVMEqualsJIT(t, `
        obj := {x: 0.0}
        for i := 1; i <= 50; i++ {
            obj.x = obj.x + 0.1
        }
        result := obj.x`, "result")
}

func TestInv_GetGlobal_NestedTable(t *testing.T) {
	// Global table of tables + index + field access in loop
	assertVMEqualsJIT(t, `
        data := { {x: 1.0}, {x: 2.0} }
        s := 0.0
        for i := 1; i <= 50; i++ {
            s = s + data[1].x + data[2].x
        }
        result := s`, "result")
}

func TestInv_GetGlobal_NBody_Mini(t *testing.T) {
	// nbody-like pattern: global table + field access in loop
	assertVMEqualsJIT(t, `
        bodies := {
            {x: 0.0, vx: 0.1, mass: 10.0},
            {x: 1.0, vx: -0.1, mass: 1.0},
        }
        for step := 1; step <= 50; step++ {
            b1 := bodies[1]
            b2 := bodies[2]
            dx := b1.x - b2.x
            f := b2.mass / (dx * dx + 0.01)
            b1.vx = b1.vx - dx * f * 0.01
            b2.vx = b2.vx + dx * f * 0.01
            b1.x = b1.x + b1.vx * 0.01
            b2.x = b2.x + b2.vx * 0.01
        }
        result := bodies[1].x`, "result")
}
