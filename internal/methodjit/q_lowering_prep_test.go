//go:build darwin && arm64

package methodjit

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

func TestVectorGatherBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_gather",
		MaxStack: 2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_GATHER, 0, 1, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var gather *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpVectorGather {
				gather = instr
				break
			}
		}
	}
	if gather == nil {
		t.Fatalf("BuildGraph did not emit OpVectorGather:\n%s", Print(fn))
	}
	if len(gather.Args) != 2 {
		t.Fatalf("OpVectorGather arg count = %d, want 2", len(gather.Args))
	}
	if gather.Type != TypeAny {
		t.Fatalf("OpVectorGather type = %s, want Any", gather.Type)
	}
}

func TestVectorCompareBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_compare",
		MaxStack: 2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 0, 1, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var compare *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpVectorCompare {
				compare = instr
				break
			}
		}
	}
	if compare == nil {
		t.Fatalf("BuildGraph did not emit OpVectorCompare:\n%s", Print(fn))
	}
	if len(compare.Args) != 2 {
		t.Fatalf("OpVectorCompare arg count = %d, want 2", len(compare.Args))
	}
	if compare.Type != TypeAny {
		t.Fatalf("OpVectorCompare type = %s, want Any", compare.Type)
	}
	if compare.Aux != int64(runtime.DenseArrayGE) {
		t.Fatalf("OpVectorCompare Aux = %d, want %d", compare.Aux, runtime.DenseArrayGE)
	}
}

func TestFrameColumnBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "frame_column",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var column *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameColumn {
				column = instr
				break
			}
		}
	}
	if column == nil {
		t.Fatalf("BuildGraph did not emit OpFrameColumn:\n%s", Print(fn))
	}
	if len(column.Args) != 1 {
		t.Fatalf("OpFrameColumn arg count = %d, want 1", len(column.Args))
	}
	if column.Type != TypeAny {
		t.Fatalf("OpFrameColumn type = %s, want Any", column.Type)
	}
	if column.Aux != 0 {
		t.Fatalf("OpFrameColumn Aux = %d, want const index 0", column.Aux)
	}
}

func TestFrameMaskBytecodeBuildsMethodJITIR(t *testing.T) {
	spec := runtime.NewTable()
	spec.RawSetString("column", runtime.StringValue("price"))
	spec.RawSetString("op", runtime.StringValue(">="))
	spec.RawSetString("value", runtime.FloatValue(100))
	proto := &vm.FuncProto{
		Name:      "frame_mask",
		NumParams: 1,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.TableValue(spec),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_MASK, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var mask *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameMask {
				mask = instr
				break
			}
		}
	}
	if mask == nil {
		t.Fatalf("BuildGraph did not emit OpFrameMask:\n%s", Print(fn))
	}
	if len(mask.Args) != 1 {
		t.Fatalf("OpFrameMask arg count = %d, want 1", len(mask.Args))
	}
	if mask.Type != TypeAny {
		t.Fatalf("OpFrameMask type = %s, want Any", mask.Type)
	}
	if mask.Aux != 0 {
		t.Fatalf("OpFrameMask Aux = %d, want const index 0", mask.Aux)
	}
}

func TestFrameProjectBytecodeBuildsMethodJITIR(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("price"))
	proto := &vm.FuncProto{
		Name:      "frame_project",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			runtime.TableValue(names),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var project *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameProject {
				project = instr
				break
			}
		}
	}
	if project == nil {
		t.Fatalf("BuildGraph did not emit OpFrameProject:\n%s", Print(fn))
	}
	if len(project.Args) != 1 {
		t.Fatalf("OpFrameProject arg count = %d, want 1", len(project.Args))
	}
	if project.Type != TypeAny {
		t.Fatalf("OpFrameProject type = %s, want Any", project.Type)
	}
	if project.Aux != 0 {
		t.Fatalf("OpFrameProject Aux = %d, want const index 0", project.Aux)
	}
}

func TestFrameFilterBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "frame_filter",
		NumParams: 2,
		MaxStack:  2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var filter *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameFilter {
				filter = instr
				break
			}
		}
	}
	if filter == nil {
		t.Fatalf("BuildGraph did not emit OpFrameFilter:\n%s", Print(fn))
	}
	if len(filter.Args) != 2 {
		t.Fatalf("OpFrameFilter arg count = %d, want 2", len(filter.Args))
	}
	if filter.Type != TypeAny {
		t.Fatalf("OpFrameFilter type = %s, want Any", filter.Type)
	}
}

func TestFrameGatherBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "frame_gather",
		NumParams: 2,
		MaxStack:  2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_GATHER, 0, 0, 1),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var gather *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameGather {
				gather = instr
				break
			}
		}
	}
	if gather == nil {
		t.Fatalf("BuildGraph did not emit OpFrameGather:\n%s", Print(fn))
	}
	if len(gather.Args) != 2 {
		t.Fatalf("OpFrameGather arg count = %d, want 2", len(gather.Args))
	}
	if gather.Type != TypeAny {
		t.Fatalf("OpFrameGather type = %s, want Any", gather.Type)
	}
}

func TestFrameSliceBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "frame_slice",
		NumParams: 2,
		MaxStack:  2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_SLICE, 0, 0, 1),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var slice *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameSlice {
				slice = instr
				break
			}
		}
	}
	if slice == nil {
		t.Fatalf("BuildGraph did not emit OpFrameSlice:\n%s", Print(fn))
	}
	if len(slice.Args) != 2 {
		t.Fatalf("OpFrameSlice arg count = %d, want 2", len(slice.Args))
	}
	if slice.Type != TypeAny {
		t.Fatalf("OpFrameSlice type = %s, want Any", slice.Type)
	}
}

func TestFrameOrderBytecodeBuildsMethodJITIR(t *testing.T) {
	order := runtime.NewTable()
	order.RawSetString("column", runtime.StringValue("price"))
	proto := &vm.FuncProto{
		Name:      "frame_order",
		NumParams: 1,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.TableValue(order),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_ORDER, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var orderInstr *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameOrder {
				orderInstr = instr
				break
			}
		}
	}
	if orderInstr == nil {
		t.Fatalf("BuildGraph did not emit OpFrameOrder:\n%s", Print(fn))
	}
	if len(orderInstr.Args) != 1 {
		t.Fatalf("OpFrameOrder arg count = %d, want 1", len(orderInstr.Args))
	}
	if orderInstr.Type != TypeAny {
		t.Fatalf("OpFrameOrder type = %s, want Any", orderInstr.Type)
	}
	if orderInstr.Aux != 0 {
		t.Fatalf("OpFrameOrder Aux = %d, want const index 0", orderInstr.Aux)
	}
}

func TestQFramePrimitivePipelineBuildsMethodJITIR(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("size"))
	proto := &vm.FuncProto{
		Name:      "q_frame_pipeline",
		NumParams: 1,
		MaxStack:  3,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			runtime.TableValue(names),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 2),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 3),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	counts := map[Op]int{}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			counts[instr.Op]++
		}
	}
	for _, op := range []Op{OpFrameFilter, OpFrameProject, OpVectorCompare} {
		if counts[op] != 1 {
			t.Fatalf("%s count = %d, want 1\n%s", op, counts[op], Print(fn))
		}
	}
	if counts[OpFrameColumn] != 2 {
		t.Fatalf("OpFrameColumn count = %d, want 2\n%s", counts[OpFrameColumn], Print(fn))
	}
	paths := DetectQQueryHotPaths(fn)
	if len(paths) != 1 {
		t.Fatalf("DetectQQueryHotPaths count = %d, want 1\n%s", len(paths), Print(fn))
	}
	if paths[0].Compare.Aux != int64(runtime.DenseArrayGE) {
		t.Fatalf("q hot path compare Aux = %d, want %d", paths[0].Compare.Aux, runtime.DenseArrayGE)
	}
}

func TestQFramePrimitiveHotPathLowersToTypedRuntimeKernel(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("size"))
	proto := &vm.FuncProto{
		Name:      "q_frame_pipeline_lowered",
		NumParams: 1,
		MaxStack:  3,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			runtime.TableValue(names),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 2),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 3),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	counts := map[Op]int{}
	for _, block := range lowered.Blocks {
		for _, instr := range block.Instrs {
			counts[instr.Op]++
		}
	}
	if counts[OpQFrameSelectColumn] != 1 {
		t.Fatalf("OpQFrameSelectColumn count = %d, want 1\n%s", counts[OpQFrameSelectColumn], Print(lowered))
	}
	for _, op := range []Op{OpFrameColumn, OpVectorCompare, OpFrameFilter, OpFrameProject} {
		if counts[op] != 0 {
			t.Fatalf("%s count = %d, want 0 after lowering\n%s", op, counts[op], Print(lowered))
		}
	}
	if len(lowered.QFrameSelectColumnSpecs) != 1 {
		t.Fatalf("QFrameSelectColumnSpecs count = %d, want 1", len(lowered.QFrameSelectColumnSpecs))
	}
	if got := lowered.QFrameSelectColumnSpecs[0].Shape; got != "compare/filter/project/column" {
		t.Fatalf("lowered q spec shape = %q, want compare/filter/project/column", got)
	}

	result, err := Interpret(lowered, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if err != nil {
		t.Fatalf("Interpret lowered q hot path: %v", err)
	}
	if len(result) != 1 || !result[0].IsDenseArray() {
		t.Fatalf("lowered result = %#v, want one dense array", result)
	}
	got, ok := result[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Fatalf("lowered result values = %#v, want [10 20]", got)
	}
}

func TestQFramePrimitiveRowHotPathsLowerToTypedRuntimeKernel(t *testing.T) {
	for _, tc := range []struct {
		name      string
		numParams int
		constants []runtime.Value
		code      []uint32
		args      []runtime.Value
		shape     string
		want      []int64
	}{
		{
			name:      "gather",
			numParams: 2,
			constants: []runtime.Value{
				runtime.StringValue("price"),
				runtime.FloatValue(100),
				qHotPathNamesValue("size"),
				runtime.StringValue("size"),
			},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_COLUMN, 2, 0, 0),
				vm.EncodeABx(vm.OP_LOADK, 3, 1),
				vm.EncodeABC(vm.OP_VECTOR_COMPARE, 2, 3, int(runtime.DenseArrayGE)),
				vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 2),
				vm.EncodeABC(vm.OP_FRAME_GATHER, 0, 0, 1),
				vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 2),
				vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 3),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: []runtime.Value{
				runtime.TableValue(qHotPathTestFrame(t)),
				runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{2, 1})),
			},
			shape: "compare/filter/gather/project/column",
			want:  []int64{20, 10},
		},
		{
			name:      "slice",
			numParams: 1,
			constants: []runtime.Value{
				runtime.StringValue("price"),
				runtime.FloatValue(100),
				runtime.IntValue(1),
				qHotPathNamesValue("size"),
				runtime.StringValue("size"),
			},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
				vm.EncodeABx(vm.OP_LOADK, 2, 1),
				vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
				vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
				vm.EncodeABx(vm.OP_LOADK, 3, 2),
				vm.EncodeABC(vm.OP_FRAME_SLICE, 0, 0, 3),
				vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 3),
				vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 4),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args:  []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))},
			shape: "compare/filter/slice/project/column",
			want:  []int64{10},
		},
		{
			name:      "order",
			numParams: 1,
			constants: []runtime.Value{
				runtime.StringValue("price"),
				runtime.FloatValue(100),
				qHotPathOrderValue("price", true),
				qHotPathNamesValue("size"),
				runtime.StringValue("size"),
			},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
				vm.EncodeABx(vm.OP_LOADK, 2, 1),
				vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
				vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
				vm.EncodeABC(vm.OP_FRAME_ORDER, 3, 0, 2),
				vm.EncodeABC(vm.OP_FRAME_GATHER, 0, 0, 3),
				vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 3),
				vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 4),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args:  []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))},
			shape: "compare/filter/order/gather/project/column",
			want:  []int64{20, 10},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proto := &vm.FuncProto{
				Name:      "q_frame_row_" + tc.name + "_lowered",
				NumParams: tc.numParams,
				MaxStack:  4,
				Constants: tc.constants,
				Code:      tc.code,
			}
			fn := BuildGraph(proto)
			lowered, err := QQueryNativeLoweringPass(fn)
			if err != nil {
				t.Fatalf("QQueryNativeLoweringPass: %v", err)
			}
			counts := map[Op]int{}
			for _, block := range lowered.Blocks {
				for _, instr := range block.Instrs {
					counts[instr.Op]++
				}
			}
			if counts[OpQFrameSelectColumn] != 1 {
				t.Fatalf("OpQFrameSelectColumn count = %d, want 1\n%s", counts[OpQFrameSelectColumn], Print(lowered))
			}
			if len(lowered.QFrameSelectColumnSpecs) != 1 || lowered.QFrameSelectColumnSpecs[0].Shape != tc.shape {
				t.Fatalf("lowered q specs = %+v, want shape %s", lowered.QFrameSelectColumnSpecs, tc.shape)
			}

			result, err := Interpret(lowered, tc.args)
			if err != nil {
				t.Fatalf("Interpret lowered q hot path: %v", err)
			}
			if len(result) != 1 || !result[0].IsDenseArray() {
				t.Fatalf("lowered result = %#v, want one dense array", result)
			}
			got, ok := result[0].DenseArray().I64()
			if !ok || len(got) != len(tc.want) {
				t.Fatalf("lowered result values = %#v, want %#v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("lowered result values = %#v, want %#v", got, tc.want)
				}
			}
		})
	}
}

func TestQFramePrimitiveHotPathDetectsFrameMask(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("size"))
	mask := runtime.NewTable()
	mask.RawSetString("column", runtime.StringValue("price"))
	mask.RawSetString("op", runtime.StringValue(">="))
	mask.RawSetString("value", runtime.FloatValue(100))
	proto := &vm.FuncProto{
		Name:      "q_frame_mask_pipeline",
		NumParams: 1,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.TableValue(mask),
			runtime.TableValue(names),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_MASK, 1, 0, 0),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 1),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 2),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	paths := DetectQQueryHotPaths(fn)
	if len(paths) != 1 {
		t.Fatalf("DetectQQueryHotPaths count = %d, want 1\n%s", len(paths), Print(fn))
	}
	if paths[0].Mask == nil || paths[0].Compare != nil {
		t.Fatalf("DetectQQueryHotPaths Mask=%v Compare=%v, want mask-only path\n%s", paths[0].Mask, paths[0].Compare, Print(fn))
	}
	if paths[0].Shape() != "mask/filter/project/column" {
		t.Fatalf("frame mask hot path shape = %q, want mask/filter/project/column", paths[0].Shape())
	}
	if got := formatQQueryHotPaths(paths); !strings.Contains(got, "compare=frame-mask") || !strings.Contains(got, "mask_aux=0") {
		t.Fatalf("frame mask hot path format = %q, want frame-mask and mask aux", got)
	}
	if got := formatQQueryHotPaths(paths); !strings.Contains(got, "shape=mask/filter/project/column") {
		t.Fatalf("frame mask hot path format = %q, want mask shape", got)
	}
}

func TestQFramePrimitiveHotPathDetectsGatheredRows(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("size"))
	proto := &vm.FuncProto{
		Name:      "q_frame_gathered_pipeline",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{2, 1})),
			runtime.TableValue(names),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
			vm.EncodeABx(vm.OP_LOADK, 3, 2),
			vm.EncodeABC(vm.OP_FRAME_GATHER, 0, 0, 3),
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 3),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 4),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	paths := DetectQQueryHotPaths(fn)
	if len(paths) != 1 {
		t.Fatalf("DetectQQueryHotPaths count = %d, want 1\n%s", len(paths), Print(fn))
	}
	if paths[0].RowGather == nil {
		t.Fatalf("DetectQQueryHotPaths RowGather = nil, want OpFrameGather\n%s", Print(fn))
	}
	if paths[0].Compare.Aux != int64(runtime.DenseArrayGE) {
		t.Fatalf("q gathered hot path compare Aux = %d, want %d", paths[0].Compare.Aux, runtime.DenseArrayGE)
	}
}

func TestQFramePrimitiveHotPathDetectsOrderedRows(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("size"))
	order := runtime.NewTable()
	order.RawSetString("column", runtime.StringValue("price"))
	order.RawSetString("desc", runtime.BoolValue(true))
	proto := &vm.FuncProto{
		Name:      "q_frame_ordered_pipeline",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			runtime.TableValue(order),
			runtime.TableValue(names),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
			vm.EncodeABC(vm.OP_FRAME_ORDER, 3, 0, 2),
			vm.EncodeABC(vm.OP_FRAME_GATHER, 0, 0, 3),
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 3),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 4),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	paths := DetectQQueryHotPaths(fn)
	if len(paths) != 1 {
		t.Fatalf("DetectQQueryHotPaths count = %d, want 1\n%s", len(paths), Print(fn))
	}
	if paths[0].RowGather == nil || paths[0].RowOrder == nil {
		t.Fatalf("DetectQQueryHotPaths RowGather=%v RowOrder=%v, want both\n%s", paths[0].RowGather, paths[0].RowOrder, Print(fn))
	}
	if paths[0].Shape() != "compare/filter/order/gather/project/column" {
		t.Fatalf("ordered hot path shape = %q, want compare/filter/order/gather/project/column", paths[0].Shape())
	}
	if got := formatQQueryHotPaths(paths); !strings.Contains(got, "shape=compare/filter/order/gather/project/column") || !strings.Contains(got, "order_aux=2") {
		t.Fatalf("ordered hot path format = %q, want shape and order aux", got)
	}
	if counts := CountQQueryHotPathShapes(paths); counts["compare/filter/order/gather/project/column"] != 1 {
		t.Fatalf("ordered hot path shape counts = %+v, want ordered count 1", counts)
	}
}

func TestQFramePrimitiveHotPathDetectsSlicedRows(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("size"))
	proto := &vm.FuncProto{
		Name:      "q_frame_sliced_pipeline",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			runtime.IntValue(2),
			runtime.TableValue(names),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
			vm.EncodeABx(vm.OP_LOADK, 3, 2),
			vm.EncodeABC(vm.OP_FRAME_SLICE, 0, 0, 3),
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 3),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 4),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	paths := DetectQQueryHotPaths(fn)
	if len(paths) != 1 {
		t.Fatalf("DetectQQueryHotPaths count = %d, want 1\n%s", len(paths), Print(fn))
	}
	if paths[0].RowSlice == nil {
		t.Fatalf("DetectQQueryHotPaths RowSlice = nil, want OpFrameSlice\n%s", Print(fn))
	}
	if paths[0].Compare.Aux != int64(runtime.DenseArrayGE) {
		t.Fatalf("q sliced hot path compare Aux = %d, want %d", paths[0].Compare.Aux, runtime.DenseArrayGE)
	}
}

func TestQFramePrimitiveHotPathRejectsUnrelatedFrameColumn(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_frame_non_pipeline",
		NumParams: 1,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 0, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 0, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 2),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	if paths := DetectQQueryHotPaths(fn); len(paths) != 0 {
		t.Fatalf("DetectQQueryHotPaths count = %d, want 0\n%s", len(paths), Print(fn))
	}
}

func TestDiagnoseReportsQQueryHotPath(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("size"))
	proto := &vm.FuncProto{
		Name:      "q_frame_pipeline_diag",
		NumParams: 1,
		MaxStack:  3,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			runtime.TableValue(names),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 2),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 3),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if len(report.QQueryHotPaths) != 0 {
		t.Fatalf("Diagnose QQueryHotPaths = %d, want 0 after native lowering\n%s", len(report.QQueryHotPaths), report.String())
	}
	if len(report.QQueryHotPathShapes) != 0 {
		t.Fatalf("Diagnose QQueryHotPathShapes = %+v, want empty after native lowering", report.QQueryHotPathShapes)
	}
	if !strings.Contains(report.String(), "Q query hot paths") {
		t.Fatalf("diagnostic report missing q hot path section:\n%s", report.String())
	}
	if !strings.Contains(report.String(), "QFrameSelectColumn") {
		t.Fatalf("diagnostic report missing lowered q kernel op:\n%s", report.String())
	}
	if !strings.Contains(formatOptimizationRemarks(report.OptimizationRemarks), "QQueryHotPath") ||
		!strings.Contains(formatOptimizationRemarks(report.OptimizationRemarks), "first shape compare/filter/project/column") ||
		!strings.Contains(formatOptimizationRemarks(report.OptimizationRemarks), "compare >=") ||
		!strings.Contains(formatOptimizationRemarks(report.OptimizationRemarks), "native lowering pending") ||
		!strings.Contains(formatOptimizationRemarks(report.OptimizationRemarks), "QQueryNativeLowering") ||
		!strings.Contains(formatOptimizationRemarks(report.OptimizationRemarks), "typed runtime kernel op-exit") {
		t.Fatalf("diagnostic remarks missing q hot path lowering handoff:\n%s", formatOptimizationRemarks(report.OptimizationRemarks))
	}
}

func TestFrameLenBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "frame_len",
		NumParams: 1,
		MaxStack:  1,
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_LEN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var frameLen *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameLen {
				frameLen = instr
				break
			}
		}
	}
	if frameLen == nil {
		t.Fatalf("BuildGraph did not emit OpFrameLen:\n%s", Print(fn))
	}
	if len(frameLen.Args) != 1 {
		t.Fatalf("OpFrameLen arg count = %d, want 1", len(frameLen.Args))
	}
	if frameLen.Type != TypeInt {
		t.Fatalf("OpFrameLen type = %s, want Int", frameLen.Type)
	}
}

func TestTier2GateAllowsFrameLenThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "frame_len",
		NumParams: 1,
		MaxStack:  1,
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_LEN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_LEN should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsFrameColumnThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "frame_column",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_COLUMN should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsFrameMaskThroughOpExit(t *testing.T) {
	spec := runtime.NewTable()
	spec.RawSetString("column", runtime.StringValue("price"))
	spec.RawSetString("op", runtime.StringValue(">="))
	spec.RawSetString("value", runtime.FloatValue(100))
	proto := &vm.FuncProto{
		Name:      "frame_mask",
		NumParams: 1,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.TableValue(spec),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_MASK, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_MASK should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsFrameProjectThroughOpExit(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("price"))
	proto := &vm.FuncProto{
		Name:      "frame_project",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			runtime.TableValue(names),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_PROJECT should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsFrameFilterThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "frame_filter",
		NumParams: 2,
		MaxStack:  2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_FILTER should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsFrameGatherThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "frame_gather",
		NumParams: 2,
		MaxStack:  2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_GATHER, 0, 0, 1),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_GATHER should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsFrameSliceThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "frame_slice",
		NumParams: 2,
		MaxStack:  2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_SLICE, 0, 0, 1),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_SLICE should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsFrameOrderThroughOpExit(t *testing.T) {
	order := runtime.NewTable()
	order.RawSetString("column", runtime.StringValue("price"))
	proto := &vm.FuncProto{
		Name:      "frame_order",
		NumParams: 1,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.TableValue(order),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_ORDER, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_ORDER should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsVectorGatherThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_gather",
		MaxStack: 2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_GATHER, 0, 1, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_VECTOR_GATHER should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsVectorCompareThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_compare",
		MaxStack: 2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 0, 1, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_VECTOR_COMPARE should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestFrameLenRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(struct{}{}, runtime.NativePayloadInfo{
		Kind:    runtime.NativePayloadDataFrame,
		Rows:    42,
		Columns: 3,
	})

	result, err := executeFrameLenValue(runtime.TableValue(frame))
	if err != nil {
		t.Fatalf("execute frame len: %v", err)
	}
	if !result.IsInt() || result.Int() != 42 {
		t.Fatalf("frame len result = %#v, want int 42", result)
	}
}

func TestFrameColumnRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{10.5, 20.25}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:    runtime.NativePayloadDataFrame,
		Rows:    soa.Len(),
		Columns: 1,
	})

	result, err := executeFrameColumnValue(runtime.TableValue(frame), "price")
	if err != nil {
		t.Fatalf("execute frame column: %v", err)
	}
	if !result.IsDenseArray() {
		t.Fatalf("frame column result = %#v, want dense array", result)
	}
	got, ok := result.DenseArray().F64()
	if !ok || len(got) != 2 || got[0] != 10.5 || got[1] != 20.25 {
		t.Fatalf("frame column values = %#v, want [10.5 20.25]", got)
	}
}

func TestFrameMaskRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{10.5, 20.25, 30.75}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:    runtime.NativePayloadDataFrame,
		Rows:    soa.Len(),
		Columns: 1,
	})
	spec := runtime.NewTable()
	spec.RawSetString("column", runtime.StringValue("price"))
	spec.RawSetString("op", runtime.StringValue(">="))
	spec.RawSetString("value", runtime.FloatValue(20))

	result, err := executeFrameMaskValue(runtime.TableValue(frame), runtime.TableValue(spec))
	if err != nil {
		t.Fatalf("execute frame mask: %v", err)
	}
	if !result.IsDenseArray() {
		t.Fatalf("frame mask result = %#v, want dense array", result)
	}
	got, ok := result.DenseArray().Bool()
	if !ok || len(got) != 3 || got[0] || !got[1] || !got[2] {
		t.Fatalf("frame mask values = %#v, want [false true true]", got)
	}
}

func TestFrameProjectRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{10.5, 20.25}),
		"size":  runtime.NewDenseArrayI64([]int64{100, 200}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:    runtime.NativePayloadDataFrame,
		Rows:    soa.Len(),
		Columns: 2,
	})

	result, err := executeFrameProjectValue(runtime.TableValue(frame), []string{"size"})
	if err != nil {
		t.Fatalf("execute frame project: %v", err)
	}
	if !result.IsFrame() {
		t.Fatalf("frame project result type = %s, want frame", result.TypeName())
	}
	col, err := executeFrameColumnValue(result, "size")
	if err != nil {
		t.Fatalf("projected frame column: %v", err)
	}
	got, ok := col.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 100 || got[1] != 200 {
		t.Fatalf("projected size values = %#v, want [100 200]", got)
	}
}

func TestFrameFilterRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{10.5, 20.25, 30.75}),
		"size":  runtime.NewDenseArrayI64([]int64{100, 200, 300}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:    runtime.NativePayloadDataFrame,
		Rows:    soa.Len(),
		Columns: 2,
	})

	result, err := executeFrameFilterValue(
		runtime.TableValue(frame),
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{false, true, true})),
	)
	if err != nil {
		t.Fatalf("execute frame filter: %v", err)
	}
	if !result.IsFrame() {
		t.Fatalf("frame filter result type = %s, want frame", result.TypeName())
	}
	col, err := executeFrameColumnValue(result, "size")
	if err != nil {
		t.Fatalf("filtered frame column: %v", err)
	}
	got, ok := col.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 200 || got[1] != 300 {
		t.Fatalf("filtered size values = %#v, want [200 300]", got)
	}
}

func TestFrameGatherRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{10.5, 20.25, 30.75}),
		"size":  runtime.NewDenseArrayI64([]int64{100, 200, 300}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:    runtime.NativePayloadDataFrame,
		Rows:    soa.Len(),
		Columns: 2,
	})

	result, err := executeFrameGatherValue(
		runtime.TableValue(frame),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1})),
	)
	if err != nil {
		t.Fatalf("execute frame gather: %v", err)
	}
	if !result.IsFrame() {
		t.Fatalf("frame gather result type = %s, want frame", result.TypeName())
	}
	col, err := executeFrameColumnValue(result, "size")
	if err != nil {
		t.Fatalf("gathered frame column: %v", err)
	}
	got, ok := col.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 300 || got[1] != 100 {
		t.Fatalf("gathered size values = %#v, want [300 100]", got)
	}
}

func TestFrameSliceRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{10.5, 20.25, 30.75}),
		"size":  runtime.NewDenseArrayI64([]int64{100, 200, 300}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:    runtime.NativePayloadDataFrame,
		Rows:    soa.Len(),
		Columns: 2,
	})

	result, err := executeFrameSliceValue(runtime.TableValue(frame), runtime.IntValue(2))
	if err != nil {
		t.Fatalf("execute frame slice: %v", err)
	}
	if !result.IsFrame() {
		t.Fatalf("frame slice result type = %s, want frame", result.TypeName())
	}
	col, err := executeFrameColumnValue(result, "size")
	if err != nil {
		t.Fatalf("sliced frame column: %v", err)
	}
	got, ok := col.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 100 || got[1] != 200 {
		t.Fatalf("sliced size values = %#v, want [100 200]", got)
	}
}

func TestFrameOrderRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{10.5, 20.25, 30.75}),
		"size":  runtime.NewDenseArrayI64([]int64{100, 200, 300}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:    runtime.NativePayloadDataFrame,
		Rows:    soa.Len(),
		Columns: 2,
	})
	order := runtime.NewTable()
	order.RawSetString("column", runtime.StringValue("price"))
	order.RawSetString("desc", runtime.BoolValue(true))
	order.RawSetString("limit", runtime.IntValue(2))

	result, err := executeFrameOrderValue(runtime.TableValue(frame), runtime.TableValue(order))
	if err != nil {
		t.Fatalf("execute frame order: %v", err)
	}
	if !result.IsDenseArray() {
		t.Fatalf("frame order result = %#v, want dense array", result)
	}
	got, ok := result.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 3 || got[1] != 2 {
		t.Fatalf("frame order indexes = %#v, want [3 2]", got)
	}
}

func TestVectorGatherRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	result, err := executeVectorGatherValue(
		runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{10, 20, 30})),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1})),
	)
	if err != nil {
		t.Fatalf("execute vector gather: %v", err)
	}
	if !result.IsDenseArray() {
		t.Fatalf("vector gather result = %#v, want dense array", result)
	}
	got, ok := result.DenseArray().F64()
	if !ok || len(got) != 2 || got[0] != 30 || got[1] != 10 {
		t.Fatalf("vector gather values = %#v, want [30 10]", got)
	}
}

func TestVectorCompareRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	result, err := executeVectorCompareValue(
		int(runtime.DenseArrayGE),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1, 4, 6})),
		runtime.IntValue(4),
	)
	if err != nil {
		t.Fatalf("execute vector compare: %v", err)
	}
	if !result.IsDenseArray() {
		t.Fatalf("vector compare result = %#v, want dense array", result)
	}
	got, ok := result.DenseArray().Bool()
	if !ok || len(got) != 3 || got[0] || !got[1] || !got[2] {
		t.Fatalf("vector compare values = %#v, want [false true true]", got)
	}
}

func TestTier2ProfileConsumesQSQLNativeIdentityFeedback(t *testing.T) {
	proto := &vm.FuncProto{Name: "qsql_caller", Code: make([]uint32, 1)}
	proto.EnsureFeedback()
	qsql := runtime.FunctionValue(&runtime.GoFunction{
		Name:       "q.sql",
		NativeKind: runtime.NativeKindStdQSQL,
		NativeData: runtime.StdQSQLIdentityPtr(),
	})
	proto.CallSiteFeedback[0].ObserveCall(qsql, nil, 2, 1)

	profile := BuildTier2SpecializationProfile(proto)
	for _, guard := range profile.Guards {
		if guard.Kind != SpecGuardCallNative {
			continue
		}
		if guard.PC != 0 {
			t.Fatalf("q.sql native guard pc = %d, want 0", guard.PC)
		}
		if guard.CalleeNativeKind != runtime.NativeKindStdQSQL {
			t.Fatalf("q.sql native kind = %d, want %d", guard.CalleeNativeKind, runtime.NativeKindStdQSQL)
		}
		if guard.CalleeNativeData != uintptr(runtime.StdQSQLIdentityPtr()) {
			t.Fatalf("q.sql native data = %#x, want %#x", guard.CalleeNativeData, uintptr(runtime.StdQSQLIdentityPtr()))
		}
		if guard.NArgs != 2 || guard.ResultArity != 1 {
			t.Fatalf("q.sql native call shape = nArgs %d resultArity %d, want 2/1", guard.NArgs, guard.ResultArity)
		}
		return
	}
	t.Fatalf("q.sql native feedback did not produce call_native guard: %+v", profile.Guards)
}

func qHotPathTestFrame(t *testing.T) *runtime.Table {
	t.Helper()
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{99, 100.5, 101.25}),
		"size":  runtime.NewDenseArrayI64([]int64{5, 10, 20}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "q-hot-path-test",
	})
	return frame
}

func qHotPathNamesValue(names ...string) runtime.Value {
	tbl := runtime.NewTable()
	for i, name := range names {
		tbl.RawSetInt(int64(i+1), runtime.StringValue(name))
	}
	return runtime.TableValue(tbl)
}

func qHotPathOrderValue(column string, desc bool) runtime.Value {
	tbl := runtime.NewTable()
	tbl.RawSetString("column", runtime.StringValue(column))
	tbl.RawSetString("desc", runtime.BoolValue(desc))
	return runtime.TableValue(tbl)
}
