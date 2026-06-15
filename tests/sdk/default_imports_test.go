package leia_test

import (
	"testing"

	leia "github.com/never-labs/leia"
)

func TestDefaultImportsExposeNumericPrelude(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "vm", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(tc.opts...)
			if err := vm.Exec(`
				root := sqrt(9)
				wave := sin(0) + cos(0) + asin(0) + acos(1)
				v := [1, 2, 3]
				energy := dot(v, v)
				score := mean([1, 2, 3]) + avg([2, 4, 6])
				q := linalg.matrix([[1, 0], [0, 1]])
				unit_trace := trace(q)
				projected := matvec(q, col(5, 7))
				projected_first := at(projected, 1)
				chain_trace := trace(matmul(row(1, 0), col(9, 4)))
				shifted := axpy([1, 1], 2, [10, 20])
				nested := [[1, 2], [3, 4]]
				nested_value := nested[2][1]
				shadow := func() {
					sqrt := func(x) { return 99 }
					return sqrt(4)
				}()
				result := root == 3 && wave == 1 && energy == 14 && score == 6 && unit_trace == 2 &&
					projected_first == 5 && chain_trace == 9 && at(shifted, 2) == 41 &&
					nested_value == 3 && shadow == 99
			`); err != nil {
				t.Fatal(err)
			}
			got, err := vm.Get("result")
			if err != nil {
				t.Fatal(err)
			}
			if got != true {
				t.Fatalf("result = %v, want true", got)
			}
		})
	}
}

func TestDefaultImportsFollowWithLibs(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "vm", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(append(tc.opts, leia.WithLibs(leia.LibMath))...)
			assertSDKGlobalPresence(t, vm, "sqrt", true)
			assertSDKGlobalPresence(t, vm, "asin", true)
			assertSDKGlobalPresence(t, vm, "eye", false)
			assertSDKGlobalPresence(t, vm, "matmul", false)
			assertSDKGlobalPresence(t, vm, "mean", false)

			vm = leia.New(append(tc.opts, leia.WithLibs(leia.LibLinalg|leia.LibStats))...)
			assertSDKGlobalPresence(t, vm, "sqrt", false)
			assertSDKGlobalPresence(t, vm, "eye", true)
			assertSDKGlobalPresence(t, vm, "matmul", true)
			assertSDKGlobalPresence(t, vm, "mean", true)
		})
	}
}

func assertSDKGlobalPresence(t *testing.T, vm *leia.VM, name string, want bool) {
	t.Helper()
	got, err := vm.Get(name)
	if err != nil {
		t.Fatal(err)
	}
	if (got != nil) != want {
		t.Fatalf("%s presence = %v (%#v), want %v", name, got != nil, got, want)
	}
}
