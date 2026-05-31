package gscript_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

type Vec2 struct {
	X, Y float64
}

func (v Vec2) Length() float64     { return math.Sqrt(v.X*v.X + v.Y*v.Y) }
func (v Vec2) Add(other Vec2) Vec2 { return Vec2{v.X + other.X, v.Y + other.Y} }
func (v *Vec2) Scale(f float64)    { v.X *= f; v.Y *= f }
func (v Vec2) String() string      { return fmt.Sprintf("Vec2(%g, %g)", v.X, v.Y) }

func TestBindStruct_new(t *testing.T) {
	vm := gs.New()
	if err := vm.BindStruct("Vec2", Vec2{}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`v := Vec2.new(3, 4)`)
	if err != nil {
		t.Fatal(err)
	}
	// v should be a table wrapping Vec2{3, 4}
	val, err := vm.Get("v")
	if err != nil {
		t.Fatal(err)
	}
	if val == nil {
		t.Fatal("expected non-nil value for v")
	}
}

func TestBindStruct_fieldAccess(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	if err := vm.BindStruct("Vec2", Vec2{}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		v := Vec2.new(3, 4)
		print(v.X)
		print(v.Y)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 2 {
		t.Fatalf("expected 2 outputs, got %d: %v", len(output), output)
	}
	if output[0] != "3.0" && output[0] != "3" {
		t.Fatalf("expected X=3, got %q", output[0])
	}
	if output[1] != "4.0" && output[1] != "4" {
		t.Fatalf("expected Y=4, got %q", output[1])
	}
}

func TestBindStruct_fieldSet(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	if err := vm.BindStruct("Vec2", Vec2{}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		v := Vec2.new(3, 4)
		v.X = 10
		print(v.X)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 1 {
		t.Fatalf("expected 1 output, got %d: %v", len(output), output)
	}
	if output[0] != "10.0" && output[0] != "10" {
		t.Fatalf("expected X=10, got %q", output[0])
	}
}

func TestBindStruct_methodCall(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	if err := vm.BindStruct("Vec2", Vec2{}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		v := Vec2.new(3, 4)
		print(v.Length())
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 1 {
		t.Fatalf("expected 1 output, got %d: %v", len(output), output)
	}
	if output[0] != "5.0" && output[0] != "5" {
		t.Fatalf("expected Length()=5, got %q", output[0])
	}
}

func TestBindStruct_returnStruct(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	if err := vm.BindStruct("Vec2", Vec2{}); err != nil {
		t.Fatal(err)
	}
	err := vm.Exec(`
		a := Vec2.new(1, 2)
		b := Vec2.new(3, 4)
		c := a.Add(b)
		print(c.X)
		print(c.Y)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 2 {
		t.Fatalf("expected 2 outputs, got %d: %v", len(output), output)
	}
	if output[0] != "4.0" && output[0] != "4" {
		t.Fatalf("expected c.X=4, got %q", output[0])
	}
	if output[1] != "6.0" && output[1] != "6" {
		t.Fatalf("expected c.Y=6, got %q", output[1])
	}
}

func TestBindStructWithConstructor(t *testing.T) {
	vm := gs.New()

	type Player struct {
		Name  string
		HP    int
		Level int
	}

	if err := vm.BindStructWithConstructor("Player", Player{}, func(name string) Player {
		return Player{Name: name, HP: 100, Level: 1}
	}); err != nil {
		t.Fatal(err)
	}

	var output []string
	vm = gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))
	if err := vm.BindStructWithConstructor("Player", Player{}, func(name string) Player {
		return Player{Name: name, HP: 100, Level: 1}
	}); err != nil {
		t.Fatal(err)
	}

	err := vm.Exec(`
		p := Player.new("Alice")
		print(p.Name)
		print(p.HP)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 2 {
		t.Fatalf("expected 2 outputs, got %d: %v", len(output), output)
	}
	if output[0] != "Alice" {
		t.Fatalf("expected 'Alice', got %q", output[0])
	}
	if output[1] != "100" {
		t.Fatalf("expected '100', got %q", output[1])
	}
}
