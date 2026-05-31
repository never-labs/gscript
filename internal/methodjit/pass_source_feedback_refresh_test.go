package methodjit

import (
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
	"github.com/never-labs/gscript/internal/vm"
)

func TestSourceFeedbackRefresh_RestoresInlinedGetTableKindAndType(t *testing.T) {
	source := &vm.FuncProto{
		Code:             make([]uint32, 3),
		Feedback:         make([]vm.TypeFeedback, 3),
		TableKeyFeedback: vm.NewTableKeyFeedbackVector(3),
	}
	source.Feedback[1].Kind = vm.FBKindInt
	source.Feedback[1].Result = vm.FBInt

	fn := &Function{}
	b := &Block{ID: 0}
	fn.Entry = b
	fn.Blocks = []*Block{b}
	tbl := &Instr{ID: fn.newValueID(), Op: OpLoadSlot, Type: TypeTable, Aux: 0, Block: b}
	key := &Instr{ID: fn.newValueID(), Op: OpLoadSlot, Type: TypeInt, Aux: 1, Block: b}
	get := &Instr{ID: fn.newValueID(), Op: OpGetTable, Type: TypeAny, Args: []*Value{tbl.Value(), key.Value()}, Block: b}
	get.setSourceFromPC(source, 1)
	b.Instrs = []*Instr{tbl, key, get}

	if _, err := SourceFeedbackRefreshPass(fn); err != nil {
		t.Fatalf("SourceFeedbackRefreshPass: %v", err)
	}
	if get.Aux2 != int64(vm.FBKindInt) {
		t.Fatalf("GetTable Aux2=%d want FBKindInt", get.Aux2)
	}
	if get.Type != TypeInt {
		t.Fatalf("GetTable Type=%s want int", get.Type)
	}
}

func TestSourceFeedbackRefresh_RestoresInlinedSetTableKind(t *testing.T) {
	source := &vm.FuncProto{
		Code:     make([]uint32, 3),
		Feedback: make([]vm.TypeFeedback, 3),
	}
	source.Feedback[1].Kind = vm.FBKindInt

	fn := &Function{}
	b := &Block{ID: 0}
	fn.Entry = b
	fn.Blocks = []*Block{b}
	tbl := &Instr{ID: fn.newValueID(), Op: OpLoadSlot, Type: TypeTable, Aux: 0, Block: b}
	key := &Instr{ID: fn.newValueID(), Op: OpLoadSlot, Type: TypeInt, Aux: 1, Block: b}
	val := &Instr{ID: fn.newValueID(), Op: OpLoadSlot, Type: TypeInt, Aux: 2, Block: b}
	set := &Instr{ID: fn.newValueID(), Op: OpSetTable, Args: []*Value{tbl.Value(), key.Value(), val.Value()}, Block: b}
	set.setSourceFromPC(source, 1)
	b.Instrs = []*Instr{tbl, key, val, set}

	if _, err := SourceFeedbackRefreshPass(fn); err != nil {
		t.Fatalf("SourceFeedbackRefreshPass: %v", err)
	}
	if set.Aux2 != int64(vm.FBKindInt) {
		t.Fatalf("SetTable Aux2=%d want FBKindInt", set.Aux2)
	}
}

func TestSourceFeedbackRefresh_RestoresInlinedGetFieldShapeAndType(t *testing.T) {
	source := &vm.FuncProto{
		Code:                make([]uint32, 3),
		FieldAccessFeedback: vm.NewFieldAccessFeedbackVector(3),
	}
	source.FieldAccessFeedback[1].ObserveFieldCache(runtime.FieldCacheEntry{
		ShapeID:  42,
		FieldIdx: 3,
	}, runtime.IntValue(7), vm.TableAccessKindGet)

	fn := &Function{}
	b := &Block{ID: 0}
	fn.Entry = b
	fn.Blocks = []*Block{b}
	tbl := &Instr{ID: fn.newValueID(), Op: OpLoadSlot, Type: TypeTable, Aux: 0, Block: b}
	get := &Instr{ID: fn.newValueID(), Op: OpGetField, Type: TypeAny, Args: []*Value{tbl.Value()}, Aux: 2, Block: b}
	get.setSourceFromPC(source, 1)
	b.Instrs = []*Instr{tbl, get}

	if _, err := SourceFeedbackRefreshPass(fn); err != nil {
		t.Fatalf("SourceFeedbackRefreshPass: %v", err)
	}
	wantAux2 := int64(42)<<32 | int64(3)
	if get.Aux2 != wantAux2 {
		t.Fatalf("GetField Aux2=%d want %d", get.Aux2, wantAux2)
	}
	if get.Type != TypeInt {
		t.Fatalf("GetField Type=%s want int", get.Type)
	}
}

func TestSourceFeedbackRefresh_RestoresRuntimeFieldPolyShapeCases(t *testing.T) {
	source := &vm.FuncProto{
		Code:                make([]uint32, 3),
		Constants:           []runtime.Value{runtime.StringValue("kind")},
		FieldAccessFeedback: vm.NewFieldAccessFeedbackVector(3),
		FieldPolyCache:      make([]runtime.FieldPolyCacheEntry, 3*runtime.FieldPolyCacheWays),
	}
	source.FieldAccessFeedback[1].ObserveFieldCache(runtime.FieldCacheEntry{
		ShapeID:  42,
		FieldIdx: 0,
	}, runtime.IntValue(7), vm.TableAccessKindGet)
	slot := runtime.FieldPolyCacheSlot(source.FieldPolyCache, 1)
	slot[0] = runtime.FieldPolyCacheEntry{ShapeID: 42, FieldIdx: 0}
	slot[1] = runtime.FieldPolyCacheEntry{ShapeID: 77, FieldIdx: 0}

	fn := &Function{Proto: source}
	b := &Block{ID: 0}
	fn.Entry = b
	fn.Blocks = []*Block{b}
	tbl := &Instr{ID: fn.newValueID(), Op: OpLoadSlot, Type: TypeTable, Aux: 0, Block: b}
	get := &Instr{ID: fn.newValueID(), Op: OpGetField, Type: TypeAny, Args: []*Value{tbl.Value()}, Aux: 0, Aux2: 123, Block: b}
	get.setSourceFromPC(source, 1)
	b.Instrs = []*Instr{tbl, get}

	if _, err := SourceFeedbackRefreshPass(fn); err != nil {
		t.Fatalf("SourceFeedbackRefreshPass: %v", err)
	}
	cases, ok := fn.Analysis.TableShapeFacts().FieldPolyShapeCases(get.ID)
	if !ok || len(cases) != 2 {
		t.Fatalf("expected two runtime field poly cases, got %#v ok=%v", cases, ok)
	}
	if get.Aux2 != 0 {
		t.Fatalf("GetField Aux2=%d want cleared poly shape facts", get.Aux2)
	}
	if get.Type != TypeInt {
		t.Fatalf("GetField Type=%s want int", get.Type)
	}
}

func TestSourceFeedbackRefresh_RestoresInlinedSetFieldShape(t *testing.T) {
	source := &vm.FuncProto{
		Code:                make([]uint32, 3),
		FieldAccessFeedback: vm.NewFieldAccessFeedbackVector(3),
	}
	source.FieldAccessFeedback[1].ObserveFieldCache(runtime.FieldCacheEntry{
		ShapeID:  99,
		FieldIdx: 4,
	}, runtime.StringValue("x"), vm.TableAccessKindSet)

	fn := &Function{}
	b := &Block{ID: 0}
	fn.Entry = b
	fn.Blocks = []*Block{b}
	tbl := &Instr{ID: fn.newValueID(), Op: OpLoadSlot, Type: TypeTable, Aux: 0, Block: b}
	val := &Instr{ID: fn.newValueID(), Op: OpLoadSlot, Type: TypeString, Aux: 1, Block: b}
	set := &Instr{ID: fn.newValueID(), Op: OpSetField, Args: []*Value{tbl.Value(), val.Value()}, Aux: 2, Block: b}
	set.setSourceFromPC(source, 1)
	b.Instrs = []*Instr{tbl, val, set}

	if _, err := SourceFeedbackRefreshPass(fn); err != nil {
		t.Fatalf("SourceFeedbackRefreshPass: %v", err)
	}
	wantAux2 := int64(99)<<32 | int64(4)
	if set.Aux2 != wantAux2 {
		t.Fatalf("SetField Aux2=%d want %d", set.Aux2, wantAux2)
	}
}

func TestEnsureSourceProtoBackfillsInlinedCalleeSource(t *testing.T) {
	source := &vm.FuncProto{Name: "callee", Code: make([]uint32, 2)}
	src := &Instr{ID: 1, Op: OpGetTable, HasSource: true, SourcePC: 1, SourceLine: 12}
	dst := &Instr{ID: 2, Op: OpGetTable}

	dst.copySourceFrom(src)
	dst.ensureSourceProto(source)

	if !dst.HasSource || dst.SourcePC != 1 || dst.SourceLine != 12 {
		t.Fatalf("source metadata not copied: %+v", dst)
	}
	if dst.SourceProto != source {
		t.Fatalf("SourceProto=%p want %p", dst.SourceProto, source)
	}
}
