package tests_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

// runLeia executes a Leia source string and captures its print output.
func runLeia(t *testing.T, src string) string {
	t.Helper()
	var buf bytes.Buffer

	vm := leia.New(leia.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		fmt.Fprintln(&buf, strings.Join(parts, "\t"))
	}))
	if err := vm.Exec(src); err != nil {
		t.Fatalf("runtime error: %v", err)
	}
	return buf.String()
}

// runLeiaFile reads a .leia file and executes it.
func runLeiaFile(t *testing.T, filename string) string {
	t.Helper()
	src, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read %s: %v", filename, err)
	}
	return runLeia(t, string(src))
}

func TestBasicArithmetic(t *testing.T) {
	out := runLeiaFile(t, filepath.Join("smoke", "01_basic.leia"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	expected := []string{"3", "7", "20", "2.5", "1", "256"}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expected), len(lines), out)
	}
	for i, exp := range expected {
		got := strings.TrimSpace(lines[i])
		if got != exp {
			t.Errorf("line %d: expected %q, got %q", i+1, exp, got)
		}
	}
}

func TestStrings(t *testing.T) {
	out := runLeiaFile(t, filepath.Join("smoke", "02_strings.leia"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	expected := []string{"hello world", "5", "HELLO", "ell", "ababab"}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expected), len(lines), out)
	}
	for i, exp := range expected {
		got := strings.TrimSpace(lines[i])
		if got != exp {
			t.Errorf("line %d: expected %q, got %q", i+1, exp, got)
		}
	}
}

func TestControlFlow(t *testing.T) {
	out := runLeiaFile(t, filepath.Join("smoke", "03_control.leia"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	expected := []string{"medium", "55", "128"}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expected), len(lines), out)
	}
	for i, exp := range expected {
		got := strings.TrimSpace(lines[i])
		if got != exp {
			t.Errorf("line %d: expected %q, got %q", i+1, exp, got)
		}
	}
}

func TestFunctions(t *testing.T) {
	out := runLeiaFile(t, filepath.Join("smoke", "04_functions.leia"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// add(3,4) = 7
	// divmod(17,5): q = 3.4, r = 2
	// fib(10) = 55
	expected := []string{"7", "3.4", "2", "55"}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expected), len(lines), out)
	}
	for i, exp := range expected {
		got := strings.TrimSpace(lines[i])
		if got != exp {
			t.Errorf("line %d: expected %q, got %q", i+1, exp, got)
		}
	}
}

func TestTables(t *testing.T) {
	out := runLeiaFile(t, filepath.Join("smoke", "05_tables.leia"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	expected := []string{
		"5",     // #arr
		"10",    // arr[1]
		"30",    // arr[3]
		"6",     // #arr after insert
		"20",    // arr[1] after remove
		"alice", // person.name
		"30",    // person["age"]
		"5",     // matrix[2][2]
	}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expected), len(lines), out)
	}
	for i, exp := range expected {
		got := strings.TrimSpace(lines[i])
		if got != exp {
			t.Errorf("line %d: expected %q, got %q", i+1, exp, got)
		}
	}
}

func TestClosures(t *testing.T) {
	out := runLeiaFile(t, filepath.Join("smoke", "06_closures.leia"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	expected := []string{"1", "2", "11", "3", "12"}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expected), len(lines), out)
	}
	for i, exp := range expected {
		got := strings.TrimSpace(lines[i])
		if got != exp {
			t.Errorf("line %d: expected %q, got %q", i+1, exp, got)
		}
	}
}

func TestMetatable(t *testing.T) {
	out := runLeiaFile(t, filepath.Join("smoke", "07_metatable.leia"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	expected := []string{
		"Rex says woof",
		"Whiskers says meow",
		"4", // v3.x
		"6", // v3.y
	}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expected), len(lines), out)
	}
	for i, exp := range expected {
		got := strings.TrimSpace(lines[i])
		if got != exp {
			t.Errorf("line %d: expected %q, got %q", i+1, exp, got)
		}
	}
}

func TestCoroutine(t *testing.T) {
	out := runLeiaFile(t, filepath.Join("smoke", "08_coroutine.leia"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	expected := []string{"1", "4", "9", "16", "25"}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expected), len(lines), out)
	}
	for i, exp := range expected {
		got := strings.TrimSpace(lines[i])
		if got != exp {
			t.Errorf("line %d: expected %q, got %q", i+1, exp, got)
		}
	}
}

func TestError(t *testing.T) {
	out := runLeiaFile(t, filepath.Join("smoke", "09_error.leia"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	expected := []string{
		"false",                // ok (pcall caught error)
		"something went wrong", // err message
		"true",                 // ok2 (pcall success)
		"42",                   // val
		"false",                // ok3 (error object)
		"404",                  // e.code
		"not found",            // e.msg
		"false",                // ok4 (assert failed)
		"math is broken",       // e2
	}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expected), len(lines), out)
	}
	for i, exp := range expected {
		got := strings.TrimSpace(lines[i])
		if got != exp {
			t.Errorf("line %d: expected %q, got %q", i+1, exp, got)
		}
	}
}

func TestStringOps(t *testing.T) {
	out := runLeiaFile(t, filepath.Join("smoke", "10_string_ops.leia"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	expected := []string{
		"13",               // string.len
		"hello, world!",    // string.lower
		"HELLO, WORLD!",    // string.upper
		"Hello",            // string.sub(s, 1, 5)
		"8\t12",            // string.find returns two values
		"HeLLo, WorLd!\t3", // string.gsub returns string and count
		"1 + 2 = 3",        // string.format
	}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expected), len(lines), out)
	}
	for i, exp := range expected {
		got := strings.TrimSpace(lines[i])
		if got != exp {
			t.Errorf("line %d: expected %q, got %q", i+1, exp, got)
		}
	}
}

func TestIterator(t *testing.T) {
	out := runLeiaFile(t, filepath.Join("smoke", "11_iterator.leia"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// ipairs output: 1 a, 2 b, 3 c, 4 d
	// pairs sorted output: x 1, y 2, z 3
	expected := []string{
		"1\ta", "2\tb", "3\tc", "4\td",
		"x\t1", "y\t2", "z\t3",
	}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expected), len(lines), out)
	}
	for i, exp := range expected {
		got := strings.TrimSpace(lines[i])
		if got != exp {
			t.Errorf("line %d: expected %q, got %q", i+1, exp, got)
		}
	}
}

func TestAdvanced(t *testing.T) {
	out := runLeiaFile(t, filepath.Join("smoke", "12_advanced.leia"))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	expected := []string{"832040"}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expected), len(lines), out)
	}
	for i, exp := range expected {
		got := strings.TrimSpace(lines[i])
		if got != exp {
			t.Errorf("line %d: expected %q, got %q", i+1, exp, got)
		}
	}
}

// TestExamples runs example programs to make sure they don't error.
func TestExamples(t *testing.T) {
	examples := []string{
		filepath.Join("..", "examples", "hello", "fib.leia"),
		filepath.Join("..", "examples", "hello", "counter.leia"),
		filepath.Join("..", "examples", "hello", "class.leia"),
		filepath.Join("..", "examples", "hello", "coroutines.leia"),
	}
	for _, ex := range examples {
		t.Run(filepath.Base(ex), func(t *testing.T) {
			src, err := os.ReadFile(ex)
			if err != nil {
				t.Fatalf("failed to read %s: %v", ex, err)
			}
			vm := leia.New()
			if err := vm.Exec(string(src)); err != nil {
				t.Fatalf("runtime error: %v", err)
			}
		})
	}
}
