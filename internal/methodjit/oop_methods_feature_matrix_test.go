//go:build darwin && arm64

package methodjit

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"github.com/never-labs/leia/internal/vm"
)

const methodJITOOPFeatureMatrixSrc = `
func Class(parent) {
  cls := {}
  cls.__index = cls
  if parent != nil {
    setmetatable(cls, {__index: parent})
  }
  cls.new = func(...) {
    instance := {}
    setmetatable(instance, cls)
    if cls.init != nil {
      cls.init(instance, ...)
    }
    return instance
  }
  return cls
}

Animal := Class(nil)
Animal.init = func(self, name) {
  self.name = name
}
Animal.describe = func(self) {
  return #self.name
}

Dog := Class(Animal)
Dog.init = func(self, name, start) {
  Animal.init(self, name)
  self.tricks = {}
  self.score = start
}
Dog.learn = func(self, trick, points) {
  self.tricks[#self.tricks + 1] = trick
  self.score = self.score + points
  return self.score
}
Dog.total = func(self) {
  return self.score + #self.tricks + self:describe()
}

func make_counter(start) {
  value := start
  return {
    add: func(self, n) {
      value = value + n
      self.last = value
      return value
    },
    peek: func(self) {
      return value + self.last
    },
  }
}

func oop_method_matrix(n) {
  dog := Dog.new("Rex", 10)
  counter := make_counter(3)
  total := 0
  for i := 1; i <= n; i++ {
    total = total + dog:learn("t", i)
    total = total + dog:total()
    total = total + counter:add(i)
  }
  return total + counter:peek() + dog.score + #dog.tricks + dog:describe()
}
`

func TestMethodJITOOPTier1MethodSemanticsMatchVM(t *testing.T) {
	want := runMethodJITOOPFunction(t, compileTop(t, methodJITOOPFeatureMatrixSrc), "oop_method_matrix",
		[]runtime.Value{runtime.IntValue(8)}, nil, 1)

	tm := NewTieringManager()
	got := runMethodJITOOPFunction(t, compileTop(t, methodJITOOPFeatureMatrixSrc), "oop_method_matrix",
		[]runtime.Value{runtime.IntValue(8)}, tm, 20)

	assertMethodJITOOPResultsEqual(t, "OOP methods Tier1-vs-VM", got, want)
	assertMethodJITOOPResultsEqual(t, "OOP methods expected", got, []runtime.Value{runtime.IntValue(739)})
	if tm.Tier1Count() == 0 {
		t.Fatalf("OOP method matrix did not compile any Tier1 code; Tier2Entered=%v Tier2Failed=%v", tm.Tier2Entered(), tm.Tier2Failed())
	}
}

func TestMethodJITOOPTier2CompileOrFallbackCorrectness(t *testing.T) {
	const fnName = "oop_method_matrix"
	args := []runtime.Value{runtime.IntValue(10)}
	want := runMethodJITOOPFunction(t, compileTop(t, methodJITOOPFeatureMatrixSrc), fnName, args, nil, 1)

	top := compileTop(t, methodJITOOPFeatureMatrixSrc)
	proto := findProtoByName(top, fnName)
	if proto == nil {
		t.Fatalf("%s proto not found", fnName)
	}

	tm := NewTieringManager()
	tier2Err := tm.CompileTier2(proto)
	if tier2Err != nil {
		errText := strings.ToLower(tier2Err.Error())
		if !strings.Contains(errText, "unsupported") &&
			!strings.Contains(errText, "call") &&
			!strings.Contains(errText, "self") &&
			!strings.Contains(errText, "staying at tier 1") {
			t.Fatalf("CompileTier2(%s) error = %q, want explicit OOP method unsupported fallback", fnName, tier2Err)
		}

		got := runMethodJITOOPFunction(t, top, fnName, args, tm, 3)
		assertMethodJITOOPResultsEqual(t, "OOP methods rejected Tier2 fallback", got, want)
		if proto.EnteredTier2 != 0 {
			t.Fatalf("%s EnteredTier2=%d after rejected Tier2 compile, want 0", fnName, proto.EnteredTier2)
		}
		return
	}

	proto.CallCount = tmDefaultTier2Threshold + 1
	got := runMethodJITOOPFunction(t, top, fnName, args, tm, 3)
	assertMethodJITOOPResultsEqual(t, "OOP methods accepted Tier2 correctness", got, want)
	if proto.EnteredTier2 == 0 {
		t.Fatalf("%s compiled for Tier2 but never entered", fnName)
	}
}

func runMethodJITOOPFunction(t *testing.T, top *vm.FuncProto, fnName string, args []runtime.Value, tm *TieringManager, calls int) []runtime.Value {
	t.Helper()
	globals := vmtest.NewInterpreterGlobals()
	v := vm.New(globals)
	defer v.Close()
	if tm != nil {
		v.SetMethodJIT(tm)
	}
	if _, err := v.Execute(top); err != nil {
		t.Fatalf("execute top: %v", err)
	}
	fn := v.GetGlobal(fnName)
	if fn.IsNil() {
		t.Fatalf("function %q not found in globals", fnName)
	}

	var got []runtime.Value
	var err error
	for i := 0; i < calls; i++ {
		got, err = v.CallValue(fn, args)
		if err != nil {
			t.Fatalf("CallValue(%s) #%d: %v", fnName, i+1, err)
		}
	}
	return got
}

func assertMethodJITOOPResultsEqual(t *testing.T, label string, got, want []runtime.Value) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s returned %d values, VM returned %d: got=%v want=%v", label, len(got), len(want), got, want)
	}
	for i := range got {
		assertValuesEqual(t, label, got[i], want[i])
	}
}
