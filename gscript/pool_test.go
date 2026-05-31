package gscript_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

func TestPool(t *testing.T) {
	pool := gs.NewPool(5, func() *gs.VM {
		return gs.New()
	})

	vm := pool.Get()
	if vm == nil {
		t.Fatal("expected non-nil VM")
	}
	pool.Put(vm)
	if pool.Size() != 1 {
		t.Fatalf("expected pool size 1, got %d", pool.Size())
	}

	// Get should reuse
	vm2 := pool.Get()
	if vm2 == nil {
		t.Fatal("expected non-nil VM")
	}
	if pool.Size() != 0 {
		t.Fatalf("expected pool size 0, got %d", pool.Size())
	}
}

func TestPool_concurrent(t *testing.T) {
	pool := gs.NewPool(10, func() *gs.VM {
		vm := gs.New()
		vm.RegisterFunc("square", func(x int64) int64 { return x * x })
		return vm
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			err := pool.Do(func(vm *gs.VM) error {
				results, err := vm.Call("square", int64(n))
				if err != nil {
					return err
				}
				expected := int64(n) * int64(n)
				if results[0] != expected {
					return fmt.Errorf("expected %d^2=%d, got %v", n, expected, results[0])
				}
				return nil
			})
			if err != nil {
				t.Errorf("goroutine %d: %v", n, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestPool_Do(t *testing.T) {
	pool := gs.NewPool(2, func() *gs.VM {
		vm := gs.New()
		vm.RegisterFunc("inc", func(x int64) int64 { return x + 1 })
		return vm
	})

	err := pool.Do(func(vm *gs.VM) error {
		results, err := vm.Call("inc", 41)
		if err != nil {
			return err
		}
		if results[0] != int64(42) {
			return fmt.Errorf("expected 42, got %v", results[0])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// VM should be returned to pool
	if pool.Size() != 1 {
		t.Fatalf("expected pool size 1 after Do, got %d", pool.Size())
	}
}

func TestPoolPreservesStateByDefault(t *testing.T) {
	pool := gs.NewPool(1, func() *gs.VM {
		return gs.New()
	})

	vm := pool.Get()
	if err := vm.Set("x", int64(42)); err != nil {
		t.Fatal(err)
	}
	pool.Put(vm)

	reused := pool.Get()
	got, err := reused.Get("x")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("x = %v (%T), want int64(42)", got, got)
	}
}

func TestPoolWithResetClearsGlobalsBeforeReuse(t *testing.T) {
	pool := gs.NewPoolWithReset(1, func() *gs.VM {
		return gs.New()
	}, func(vm *gs.VM) bool {
		vm.Reset()
		return true
	})

	vm := pool.Get()
	if err := vm.Set("x", int64(42)); err != nil {
		t.Fatal(err)
	}
	pool.Put(vm)

	reused := pool.Get()
	got, err := reused.Get("x")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("x = %v (%T), want nil after reset", got, got)
	}
}

func TestPoolWithResetCanDiscardVM(t *testing.T) {
	created := 0
	pool := gs.NewPoolWithReset(1, func() *gs.VM {
		created++
		vm := gs.New()
		if err := vm.Set("id", int64(created)); err != nil {
			t.Fatal(err)
		}
		return vm
	}, func(vm *gs.VM) bool {
		return false
	})

	first := pool.Get()
	pool.Put(first)
	if pool.Size() != 0 {
		t.Fatalf("expected discarded VM to leave pool empty, got size %d", pool.Size())
	}
	second := pool.Get()
	got, err := second.Get("id")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(2) {
		t.Fatalf("id = %v (%T), want int64(2)", got, got)
	}
}

func TestVMResetClearsGlobalsAndModuleCache(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "helper.gs")
	if err := os.WriteFile(modPath, []byte(`return { value: 1 }`), 0644); err != nil {
		t.Fatal(err)
	}

	vm := gs.New(gs.WithRequirePath(dir))
	if err := vm.Exec(`helper := require("helper"); extra := 99`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modPath, []byte(`return { value: 2 }`), 0644); err != nil {
		t.Fatal(err)
	}

	vm.Reset()
	if got, err := vm.Get("extra"); err != nil {
		t.Fatal(err)
	} else if got != nil {
		t.Fatalf("extra = %v (%T), want nil after reset", got, got)
	}
	if err := vm.Exec(`helper := require("helper"); result := helper.value`); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(2) {
		t.Fatalf("result = %v (%T), want int64(2)", got, got)
	}
}
