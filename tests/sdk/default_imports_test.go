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
				wave := sin(0) + cos(0)
				v := vector(1, 2, 3)
				energy := dot(v, v)
				scale := mean({1, 2, 3}) + avg({2, 4, 6})
				q := eye(2)
				unit_trace := trace(q)
				shadow := func() {
					sqrt := func(x) { return 99 }
					return sqrt(4)
				}()
				result := root == 3 && wave == 1 && energy == 14 && scale == 6 && unit_trace == 2 && shadow == 99
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
			assertSDKGlobalPresence(t, vm, "eye", false)
			assertSDKGlobalPresence(t, vm, "mean", false)

			vm = leia.New(append(tc.opts, leia.WithLibs(leia.LibLinalg|leia.LibStats))...)
			assertSDKGlobalPresence(t, vm, "sqrt", false)
			assertSDKGlobalPresence(t, vm, "eye", true)
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
