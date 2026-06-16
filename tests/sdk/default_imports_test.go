package leia_test

import (
	"strings"
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
				appended := append([], 4, 5, 6)
				energy := dot(v, v)
				score := mean([1, 2, 3]) + avg([2, 4, 6])
				q := mat([[1, 0], [0, 1]])
				unit_trace := trace(q)
				projected := matvec(q, col(5, 7))
				projected_first := at(projected, 1)
				chain_trace := trace(matmul(row(1, 0), col(9, 4)))
				shifted := axpy([1, 1], 2, [10, 20])
				ones_total := sum(ones(4))
				stats := describe([2, 4, 6])
				spread_ok := near(rms([3, 4]), sqrt(12.5), 0.000000001) && near(rmse([1, 2], [1, 4]), sqrt(2), 0.000000001)
				steps := cumsum([1, 2, 3])
				gaps := diff([2, 5, 9])
				noisy := randn(3, 0, 1)
				noisy_summary := describe(noisy)
				pick := sample([7, 8, 9], 2)
				nested := [[1, 2], [3, 4]]
				nested_value := nested[2][1]
				shadow := func() {
					sqrt := func(x) { return 99 }
					return sqrt(4)
				}()
				result := root == 3 && wave == 1 && energy == 14 && score == 6 && unit_trace == 2 &&
					projected_first == 5 && chain_trace == 9 && at(shifted, 2) == 41 &&
					ones_total == 4 && stats.mean == 4 && spread_ok && steps[3] == 6 && gaps[2] == 4 &&
					noisy_summary.count == 3 && #pick == 2 && nested_value == 3 && appended[3] == 6 && shadow == 99
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
			assertSDKGlobalPresence(t, vm, "append", true)
			assertSDKGlobalPresence(t, vm, "sqrt", true)
			assertSDKGlobalPresence(t, vm, "asin", true)
			assertSDKGlobalPresence(t, vm, "eye", false)
			assertSDKGlobalPresence(t, vm, "matmul", false)
			assertSDKGlobalPresence(t, vm, "mean", false)
			assertSDKExecUndefined(t, vm, "x := mat([[1, 0], [0, 1]])", "mat")

			vm = leia.New(append(tc.opts, leia.WithLibs(leia.LibLinalg|leia.LibStats))...)
			assertSDKGlobalPresence(t, vm, "append", true)
			assertSDKGlobalPresence(t, vm, "sqrt", false)
			assertSDKGlobalPresence(t, vm, "eye", true)
			assertSDKGlobalPresence(t, vm, "mat", true)
			assertSDKGlobalPresence(t, vm, "ones", true)
			assertSDKGlobalPresence(t, vm, "matmul", true)
			assertSDKGlobalPresence(t, vm, "mean", true)
			assertSDKGlobalPresence(t, vm, "describe", true)
			assertSDKGlobalPresence(t, vm, "randn", false)
			assertSDKExecUndefined(t, vm, "x := sqrt(9)", "sqrt")
			assertSDKExecUndefined(t, vm, "x := randn(3, 0, 1)", "randn")

			vm = leia.New(append(tc.opts, leia.WithLibs(leia.LibRand))...)
			assertSDKGlobalPresence(t, vm, "append", true)
			assertSDKGlobalPresence(t, vm, "randn", true)
			assertSDKGlobalPresence(t, vm, "sample", true)
			assertSDKGlobalPresence(t, vm, "sqrt", false)
			assertSDKGlobalPresence(t, vm, "eye", false)
			assertSDKExecUndefined(t, vm, "x := sqrt(9)", "sqrt")
			assertSDKExecUndefined(t, vm, "x := eye(2)", "eye")

			vm = leia.New(append(tc.opts, leia.WithLibs(leia.LibTable))...)
			assertSDKGlobalPresence(t, vm, "append", true)
			assertSDKGlobalPresence(t, vm, "sqrt", false)

			vm = leia.New(append(tc.opts, leia.WithLibs(leia.LibLLM))...)
			assertSDKGlobalPresence(t, vm, "append", true)
			assertSDKGlobalPresence(t, vm, "table", false)
		})
	}
}

func assertSDKExecUndefined(t *testing.T, vm *leia.VM, source, name string) {
	t.Helper()
	err := vm.Exec(source)
	if err == nil {
		t.Fatalf("%s unexpectedly executed without error", source)
	}
	if !strings.Contains(err.Error(), "undefined variable: "+name) && !strings.Contains(err.Error(), "attempt to call a nil value") {
		t.Fatalf("%s error = %v, want unavailable %s", source, err, name)
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
