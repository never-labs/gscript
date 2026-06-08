//go:build darwin && arm64

package methodjit

import (
	"encoding/json"
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

func TestVectorMaskBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_mask",
		MaxStack: 2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_MASK, 0, 1, int(runtime.DenseArrayMaskAnd)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var mask *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpVectorMask {
				mask = instr
				break
			}
		}
	}
	if mask == nil {
		t.Fatalf("BuildGraph did not emit OpVectorMask:\n%s", Print(fn))
	}
	if len(mask.Args) != 2 {
		t.Fatalf("OpVectorMask arg count = %d, want 2", len(mask.Args))
	}
	if mask.Type != TypeAny {
		t.Fatalf("OpVectorMask type = %s, want Any", mask.Type)
	}
	if mask.Aux != int64(runtime.DenseArrayMaskAnd) {
		t.Fatalf("OpVectorMask Aux = %d, want %d", mask.Aux, runtime.DenseArrayMaskAnd)
	}
}

func TestVectorWhereBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_where",
		MaxStack: 3,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_WHERE, 0, 1, 2),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var where *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpVectorWhere {
				where = instr
				break
			}
		}
	}
	if where == nil {
		t.Fatalf("BuildGraph did not emit OpVectorWhere:\n%s", Print(fn))
	}
	if len(where.Args) != 3 {
		t.Fatalf("OpVectorWhere arg count = %d, want 3", len(where.Args))
	}
	if where.Type != TypeAny {
		t.Fatalf("OpVectorWhere type = %s, want Any", where.Type)
	}
}

func TestVectorReduceBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_reduce",
		MaxStack: 1,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 0, 0, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var reduce *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpVectorReduce {
				reduce = instr
				break
			}
		}
	}
	if reduce == nil {
		t.Fatalf("BuildGraph did not emit OpVectorReduce:\n%s", Print(fn))
	}
	if len(reduce.Args) != 1 {
		t.Fatalf("OpVectorReduce arg count = %d, want 1", len(reduce.Args))
	}
	if reduce.Type != TypeAny {
		t.Fatalf("OpVectorReduce type = %s, want Any", reduce.Type)
	}
	if reduce.Aux != int64(runtime.DenseArrayReduceSum) {
		t.Fatalf("OpVectorReduce Aux = %d, want %d", reduce.Aux, runtime.DenseArrayReduceSum)
	}
}

func TestVectorWhereReduceBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_where_reduce",
		MaxStack: 3,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_WHERE_REDUCE, 0, 1, int(runtime.DenseArrayReduceMean)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var reduce *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpQVectorWhereReduce {
				reduce = instr
				break
			}
		}
	}
	if reduce == nil {
		t.Fatalf("BuildGraph did not emit OpQVectorWhereReduce:\n%s", Print(fn))
	}
	if len(reduce.Args) != 3 {
		t.Fatalf("OpQVectorWhereReduce arg count = %d, want 3", len(reduce.Args))
	}
	if reduce.Type != TypeAny {
		t.Fatalf("OpQVectorWhereReduce type = %s, want Any", reduce.Type)
	}
	if reduce.Aux != int64(runtime.DenseArrayReduceMean) {
		t.Fatalf("OpQVectorWhereReduce Aux = %d, want %d", reduce.Aux, runtime.DenseArrayReduceMean)
	}
}

func TestVectorWhereReduceBytecodeDiagnosesTypedRuntimeKernel(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "vector_where_reduce_primitive_diag",
		NumParams: 4,
		MaxStack:  4,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 0, 1, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_VECTOR_WHERE_REDUCE, 0, 2, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}
	args := []runtime.Value{
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1, 4, 6})),
		runtime.IntValue(4),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30})),
		runtime.IntValue(7),
	}

	fn := BuildGraph(proto)
	counts := countOps(fn)
	if counts[OpQVectorWhereReduce] != 1 {
		t.Fatalf("BuildGraph OpQVectorWhereReduce count = %d, want 1\n%s", counts[OpQVectorWhereReduce], Print(fn))
	}
	if counts[OpVectorWhere] != 0 || counts[OpVectorReduce] != 0 {
		t.Fatalf("BuildGraph split where/reduce counts = %d/%d, want 0/0 for VM fused primitive\n%s",
			counts[OpVectorWhere], counts[OpVectorReduce], Print(fn))
	}
	kernels := DetectQVectorRuntimeKernels(fn)
	if got := CountQVectorRuntimeKernelShapes(kernels)["compare/vector-where/vector-reduce"]; got != 1 {
		t.Fatalf("BuildGraph QVectorRuntimeKernelShapes = %+v, want compare/vector-where/vector-reduce", CountQVectorRuntimeKernelShapes(kernels))
	}

	report := Diagnose(proto, args)
	if !report.OptimizerMatch || !report.BackendMatch || !report.Match {
		t.Fatalf("Diagnose vector where-reduce primitive mismatch: optimizer=%v %s backend=%v %s match=%v %s\n%s",
			report.OptimizerMatch, report.OptimizerMismatch,
			report.BackendMatch, report.BackendMismatch,
			report.Match, report.Mismatch,
			report.String())
	}
	if report.QVectorRuntimeKernelShapes["compare/vector-where/vector-reduce"] != 1 {
		t.Fatalf("Diagnose QVectorRuntimeKernelShapes = %+v, want compare/vector-where/vector-reduce", report.QVectorRuntimeKernelShapes)
	}
	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_vector_runtime", "runtime_kernel", "QVectorWhereReduce", "compare/vector-where/vector-reduce", "typed_runtime_op_exit", "supported", "")
	assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_vector_runtime", "runtime_kernel", "compare/vector-where/vector-reduce", "supported", "", 1)
	assertQKernelShapeSummaryExecution(t, report.QKernelShapeSummary, "methodjit_q_vector_runtime", "runtime_kernel", "compare/vector-where/vector-reduce", "supported", 1, 1, 0)
	assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_vector_runtime", "QVectorWhereReduce", "compare/vector-where/vector-reduce", "typed_runtime_op_exit", "success", 1)
	assertQKernelExecutionRouteSummary(t, report.QKernelExecutionRoutes, "methodjit_q_vector_runtime", "QVectorWhereReduce", "typed_runtime_op_exit", "success", 1)
	assertQKernelJSONRows(t, report.QKernelDescriptors, report.QKernelExecutionStats, report.QKernelExecutionRoutes, report.QKernelShapeSummary)
	if len(report.InterpResult) != 1 || !report.InterpResult[0].IsInt() || report.InterpResult[0].Int() != 57 {
		t.Fatalf("Diagnose vector where-reduce primitive result = %#v, want int 57", report.InterpResult)
	}
	if !strings.Contains(report.String(), "kernel=QVectorWhereReduce") ||
		!strings.Contains(report.String(), "source=methodjit_q_vector_runtime kernel=QVectorWhereReduce shape=compare/vector-where/vector-reduce route=typed_runtime_op_exit outcome=success count=1") ||
		!strings.Contains(report.String(), "Q kernel execution route summary") ||
		!strings.Contains(report.String(), "source=methodjit_q_vector_runtime kernel=QVectorWhereReduce route=typed_runtime_op_exit outcome=success count=1") {
		t.Fatalf("diagnostic report missing VM fused primitive typed-kernel evidence:\n%s", report.String())
	}
}

func TestVectorScanBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_scan",
		MaxStack: 1,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_SCAN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var scan *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpVectorScan {
				scan = instr
				break
			}
		}
	}
	if scan == nil {
		t.Fatalf("BuildGraph did not emit OpVectorScan:\n%s", Print(fn))
	}
	if len(scan.Args) != 1 {
		t.Fatalf("OpVectorScan arg count = %d, want 1", len(scan.Args))
	}
	if scan.Type != TypeAny {
		t.Fatalf("OpVectorScan type = %s, want Any", scan.Type)
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

func TestFrameProjectColumnBytecodeBuildsMethodJITIR(t *testing.T) {
	spec := runtime.NewTable()
	spec.RawSetString("project", runtime.StringValue("size"))
	spec.RawSetString("column", runtime.StringValue("size"))
	proto := &vm.FuncProto{
		Name:      "frame_project_column",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			runtime.TableValue(spec),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_PROJECT_COLUMN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var projectColumn *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameProjectColumn {
				projectColumn = instr
				break
			}
		}
	}
	if projectColumn == nil {
		t.Fatalf("BuildGraph did not emit OpFrameProjectColumn:\n%s", Print(fn))
	}
	if len(projectColumn.Args) != 1 {
		t.Fatalf("OpFrameProjectColumn arg count = %d, want 1", len(projectColumn.Args))
	}
	if projectColumn.Type != TypeAny {
		t.Fatalf("OpFrameProjectColumn type = %s, want Any", projectColumn.Type)
	}
	if projectColumn.Aux != 0 {
		t.Fatalf("OpFrameProjectColumn Aux = %d, want const index 0", projectColumn.Aux)
	}

	kernels := DetectQFrameRuntimeKernels(fn)
	if got := CountQFrameRuntimeKernelShapes(kernels)["project/column"]; got != 1 {
		t.Fatalf("QFrameRuntimeKernelShapes = %+v, want project/column count 1", CountQFrameRuntimeKernelShapes(kernels))
	}
	descriptors := BuildQKernelDescriptors(nil, kernels, nil, nil)
	assertQKernelDescriptor(t, descriptors, "methodjit_q_frame_runtime", "runtime_kernel", "FrameProjectColumn", "project/column", "typed_runtime_op_exit", "supported", "")

	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if report.QFrameRuntimeKernelShapes["project/column"] != 1 {
		t.Fatalf("Diagnose QFrameRuntimeKernelShapes = %+v, want project/column count 1\n%s", report.QFrameRuntimeKernelShapes, report.String())
	}
	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_frame_runtime", "runtime_kernel", "FrameProjectColumn", "project/column", "typed_runtime_op_exit", "supported", "")
	assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_frame_runtime", "runtime_kernel", "project/column", "supported", "", 1)
	assertQKernelShapeSummaryExecution(t, report.QKernelShapeSummary, "methodjit_q_frame_runtime", "runtime_kernel", "project/column", "supported", 1, 1, 0)
	assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_frame_runtime", "FrameProjectColumn", "project/column", "typed_runtime_op_exit", "success", 1)
	assertQKernelExecutionRouteSummary(t, report.QKernelExecutionRoutes, "methodjit_q_frame_runtime", "FrameProjectColumn", "typed_runtime_op_exit", "success", 1)
	if !strings.Contains(report.String(), "Q frame runtime kernels") ||
		!strings.Contains(report.String(), "kernel=FrameProjectColumn") ||
		!strings.Contains(report.String(), "source=methodjit_q_frame_runtime kind=runtime_kernel kernel=FrameProjectColumn shape=project/column route=typed_runtime_op_exit outcome=supported") ||
		!strings.Contains(report.String(), "source=methodjit_q_frame_runtime kernel=FrameProjectColumn shape=project/column route=typed_runtime_op_exit outcome=success count=1") {
		t.Fatalf("diagnostic report missing frame project-column runtime kernel descriptor:\n%s", report.String())
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

func TestFrameFilterProjectBytecodeBuildsMethodJITIR(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("price"))
	proto := &vm.FuncProto{
		Name:      "frame_filter_project",
		NumParams: 2,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.TableValue(names),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_FILTER_PROJECT, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var filterProject *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameFilterProject {
				filterProject = instr
				break
			}
		}
	}
	if filterProject == nil {
		t.Fatalf("BuildGraph did not emit OpFrameFilterProject:\n%s", Print(fn))
	}
	if len(filterProject.Args) != 2 {
		t.Fatalf("OpFrameFilterProject arg count = %d, want 2", len(filterProject.Args))
	}
	if filterProject.Type != TypeAny {
		t.Fatalf("OpFrameFilterProject type = %s, want Any", filterProject.Type)
	}
	if filterProject.Aux != 0 {
		t.Fatalf("OpFrameFilterProject Aux = %d, want const index 0", filterProject.Aux)
	}
}

func TestFrameFilterProjectColumnBytecodeBuildsMethodJITIR(t *testing.T) {
	spec := runtime.NewTable()
	spec.RawSetString("project", runtime.StringValue("price"))
	spec.RawSetString("column", runtime.StringValue("price"))
	proto := &vm.FuncProto{
		Name:      "frame_filter_project_column",
		NumParams: 2,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.TableValue(spec),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_FILTER_PROJECT_COLUMN, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var filterProjectColumn *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameFilterProjectColumn {
				filterProjectColumn = instr
				break
			}
		}
	}
	if filterProjectColumn == nil {
		t.Fatalf("BuildGraph did not emit OpFrameFilterProjectColumn:\n%s", Print(fn))
	}
	if len(filterProjectColumn.Args) != 2 {
		t.Fatalf("OpFrameFilterProjectColumn arg count = %d, want 2", len(filterProjectColumn.Args))
	}
	if filterProjectColumn.Type != TypeAny {
		t.Fatalf("OpFrameFilterProjectColumn type = %s, want Any", filterProjectColumn.Type)
	}
	if filterProjectColumn.Aux != 0 {
		t.Fatalf("OpFrameFilterProjectColumn Aux = %d, want const index 0", filterProjectColumn.Aux)
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

func TestFrameOrderGatherBytecodeBuildsMethodJITIR(t *testing.T) {
	order := runtime.NewTable()
	order.RawSetString("column", runtime.StringValue("price"))
	proto := &vm.FuncProto{
		Name:      "frame_order_gather",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			runtime.TableValue(order),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_ORDER_GATHER, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var orderGather *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameOrderGather {
				orderGather = instr
				break
			}
		}
	}
	if orderGather == nil {
		t.Fatalf("BuildGraph did not emit OpFrameOrderGather:\n%s", Print(fn))
	}
	if len(orderGather.Args) != 1 {
		t.Fatalf("OpFrameOrderGather arg count = %d, want 1", len(orderGather.Args))
	}
	if orderGather.Type != TypeAny {
		t.Fatalf("OpFrameOrderGather type = %s, want Any", orderGather.Type)
	}
	if orderGather.Aux != 0 {
		t.Fatalf("OpFrameOrderGather Aux = %d, want const index 0", orderGather.Aux)
	}
}

func TestFrameGroupAggregateBytecodeBuildsMethodJITIR(t *testing.T) {
	spec := qFrameGroupAggregateSpec("size", []runtime.FrameAggregateSpec{
		{Name: "n", Op: "count"},
		{Name: "total", Op: "sum", Column: "price"},
	})
	proto := &vm.FuncProto{
		Name:      "frame_group_aggregate",
		NumParams: 2,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.TableValue(spec),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_GROUP_AGGREGATE, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var groupAgg *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameGroupAggregate {
				groupAgg = instr
				break
			}
		}
	}
	if groupAgg == nil {
		t.Fatalf("BuildGraph did not emit OpFrameGroupAggregate:\n%s", Print(fn))
	}
	if len(groupAgg.Args) != 2 {
		t.Fatalf("OpFrameGroupAggregate arg count = %d, want 2", len(groupAgg.Args))
	}
	if groupAgg.Type != TypeAny {
		t.Fatalf("OpFrameGroupAggregate type = %s, want Any", groupAgg.Type)
	}
	if groupAgg.Aux != 0 {
		t.Fatalf("OpFrameGroupAggregate Aux = %d, want const index 0", groupAgg.Aux)
	}
}

func TestFrameOrderGatherDiagnoseUsesRuntimeOpExit(t *testing.T) {
	order := runtime.NewTable()
	order.RawSetString("column", runtime.StringValue("price"))
	order.RawSetString("desc", runtime.BoolValue(true))
	order.RawSetString("limit", runtime.IntValue(2))
	proto := &vm.FuncProto{
		Name:      "frame_order_gather_diag",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			runtime.TableValue(order),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_ORDER_GATHER, 0, 0, 0),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 1),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if report.NativeError != nil {
		t.Fatalf("FrameOrderGather native error: %v\n%s", report.NativeError, report.String())
	}
	if report.InterpError != nil || report.OptInterpError != nil {
		t.Fatalf("FrameOrderGather interpreter errors: unopt=%v opt=%v\n%s", report.InterpError, report.OptInterpError, report.String())
	}
	if len(report.NativeResult) != 1 || !report.NativeResult[0].IsDenseArray() {
		t.Fatalf("FrameOrderGather native result = %#v, want dense array", report.NativeResult)
	}
	got, ok := report.NativeResult[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 20 || got[1] != 10 {
		t.Fatalf("FrameOrderGather size result = %#v, want [20 10]", got)
	}
	if !strings.Contains(report.IRAfter, "FrameOrderGather") {
		t.Fatalf("FrameOrderGather missing from optimized IR:\n%s", report.IRAfter)
	}
	if got := report.QFrameRuntimeKernelShapes["order/gather"]; got != 1 {
		t.Fatalf("QFrameRuntimeKernelShapes[order/gather] = %d, want 1; shapes=%+v", got, report.QFrameRuntimeKernelShapes)
	}
	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_frame_runtime", "runtime_kernel", "FrameOrderGather", "order/gather", "typed_runtime_op_exit", "supported", "")
	assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_frame_runtime", "runtime_kernel", "order/gather", "supported", "", 1)
	assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_frame_runtime", "FrameOrderGather", "order/gather", "typed_runtime_op_exit", "success", 1)
	assertQKernelExecutionRouteSummary(t, report.QKernelExecutionRoutes, "methodjit_q_frame_runtime", "FrameOrderGather", "typed_runtime_op_exit", "success", 1)
	assertQKernelShapeSummaryExecution(t, report.QKernelShapeSummary, "methodjit_q_frame_runtime", "runtime_kernel", "order/gather", "supported", 1, 1, 0)
}

func TestQFrameRuntimePrimitiveDiagnoseExecutionStats(t *testing.T) {
	projectNames := runtime.NewTable()
	projectNames.RawSetInt(1, runtime.StringValue("size"))
	maskSpec := runtime.NewTable()
	maskSpec.RawSetString("column", runtime.StringValue("price"))
	maskSpec.RawSetString("op", runtime.StringValue(">="))
	maskSpec.RawSetString("value", runtime.FloatValue(100))
	orderSpec := runtime.NewTable()
	orderSpec.RawSetString("column", runtime.StringValue("price"))
	orderSpec.RawSetString("desc", runtime.BoolValue(true))
	orderSpec.RawSetString("limit", runtime.IntValue(2))
	columnSpec := runtime.NewTable()
	columnSpec.RawSetString("project", runtime.StringValue("size"))
	columnSpec.RawSetString("column", runtime.StringValue("size"))
	groupAggSpec := qFrameGroupAggregateSpec("size", []runtime.FrameAggregateSpec{
		{Name: "n", Op: "count"},
		{Name: "total", Op: "sum", Column: "price"},
	})

	frameArg := func(t *testing.T) runtime.Value {
		t.Helper()
		return runtime.TableValue(qHotPathTestFrame(t))
	}
	maskArg := runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{false, true, true}))
	indexArg := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1}))

	tests := []struct {
		name      string
		kernel    string
		shape     string
		constants []runtime.Value
		code      []uint32
		args      func(*testing.T) []runtime.Value
	}{
		{
			name:   "len",
			kernel: "FrameLen",
			shape:  "len",
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_LEN, 0, 0, 0),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t)} },
		},
		{
			name:      "column",
			kernel:    "FrameColumn",
			shape:     "column",
			constants: []runtime.Value{runtime.StringValue("size")},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 0),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t)} },
		},
		{
			name:      "mask",
			kernel:    "FrameMask",
			shape:     "mask",
			constants: []runtime.Value{runtime.TableValue(maskSpec)},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_MASK, 0, 0, 0),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t)} },
		},
		{
			name:      "project",
			kernel:    "FrameProject",
			shape:     "project",
			constants: []runtime.Value{runtime.TableValue(projectNames)},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 0),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t)} },
		},
		{
			name:   "filter",
			kernel: "FrameFilter",
			shape:  "filter",
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t), maskArg} },
		},
		{
			name:      "filter_project",
			kernel:    "FrameFilterProject",
			shape:     "filter/project",
			constants: []runtime.Value{runtime.TableValue(projectNames)},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_FILTER_PROJECT, 1, 0, 0),
				vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t), maskArg} },
		},
		{
			name:   "gather",
			kernel: "FrameGather",
			shape:  "gather",
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_GATHER, 0, 0, 1),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t), indexArg} },
		},
		{
			name:   "slice",
			kernel: "FrameSlice",
			shape:  "slice",
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_SLICE, 0, 0, 1),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t), runtime.IntValue(2)} },
		},
		{
			name:      "order",
			kernel:    "FrameOrder",
			shape:     "order",
			constants: []runtime.Value{runtime.TableValue(orderSpec)},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_ORDER, 0, 0, 0),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t)} },
		},
		{
			name:      "order_gather",
			kernel:    "FrameOrderGather",
			shape:     "order/gather",
			constants: []runtime.Value{runtime.TableValue(orderSpec)},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_ORDER_GATHER, 0, 0, 0),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t)} },
		},
		{
			name:      "project_column",
			kernel:    "FrameProjectColumn",
			shape:     "project/column",
			constants: []runtime.Value{runtime.TableValue(columnSpec)},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_PROJECT_COLUMN, 0, 0, 0),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t)} },
		},
		{
			name:      "filter_project_column",
			kernel:    "FrameFilterProjectColumn",
			shape:     "filter/project/column",
			constants: []runtime.Value{runtime.TableValue(columnSpec)},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_FILTER_PROJECT_COLUMN, 1, 0, 0),
				vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t), maskArg} },
		},
		{
			name:      "group_aggregate",
			kernel:    "FrameGroupAggregate",
			shape:     "group/aggregate",
			constants: []runtime.Value{runtime.TableValue(groupAggSpec)},
			code: []uint32{
				vm.EncodeABC(vm.OP_LOADNIL, 1, 0, 0),
				vm.EncodeABC(vm.OP_FRAME_GROUP_AGGREGATE, 1, 0, 0),
				vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t)} },
		},
		{
			name:      "filter_group_aggregate",
			kernel:    "FrameGroupAggregate",
			shape:     "filter/group/aggregate",
			constants: []runtime.Value{runtime.TableValue(groupAggSpec)},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_GROUP_AGGREGATE, 1, 0, 0),
				vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
			},
			args: func(t *testing.T) []runtime.Value { return []runtime.Value{frameArg(t), maskArg} },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proto := &vm.FuncProto{
				Name:      "q_frame_runtime_primitive_" + tt.name,
				NumParams: len(tt.args(t)),
				MaxStack:  3,
				Constants: tt.constants,
				Code:      tt.code,
			}
			report := Diagnose(proto, tt.args(t))
			if report.NativeError != nil || report.InterpError != nil || report.OptInterpError != nil {
				t.Fatalf("Diagnose %s errors: native=%v interp=%v opt=%v\n%s",
					tt.name, report.NativeError, report.InterpError, report.OptInterpError, report.String())
			}
			if got := report.QFrameRuntimeKernelShapes[tt.shape]; got != 1 {
				t.Fatalf("QFrameRuntimeKernelShapes[%s] = %d, want 1; shapes=%+v", tt.shape, got, report.QFrameRuntimeKernelShapes)
			}
			assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_frame_runtime", "runtime_kernel", tt.kernel, tt.shape, "typed_runtime_op_exit", "supported", "")
			assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_frame_runtime", "runtime_kernel", tt.shape, "supported", "", 1)
			assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_frame_runtime", tt.kernel, tt.shape, "typed_runtime_op_exit", "success", 1)
			assertQKernelExecutionRouteSummary(t, report.QKernelExecutionRoutes, "methodjit_q_frame_runtime", tt.kernel, "typed_runtime_op_exit", "success", 1)
			assertQKernelShapeSummaryExecution(t, report.QKernelShapeSummary, "methodjit_q_frame_runtime", "runtime_kernel", tt.shape, "supported", 1, 1, 0)
		})
	}
}

func TestQTypedRuntimeKernelErrorStatsAreObservable(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_runtime_kernel_error_stats",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			runtime.StringValue("missing"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if report.NativeError == nil {
		t.Fatalf("Diagnose native error = nil, want missing column error\n%s", report.String())
	}
	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_frame_runtime", "runtime_kernel", "FrameColumn", "column", "typed_runtime_op_exit", "supported", "")
	assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_frame_runtime", "FrameColumn", "column", "typed_runtime_op_exit", "error", 1)
	assertQKernelExecutionRouteSummary(t, report.QKernelExecutionRoutes, "methodjit_q_frame_runtime", "FrameColumn", "typed_runtime_op_exit", "error", 1)
	assertQKernelShapeSummaryExecution(t, report.QKernelShapeSummary, "methodjit_q_frame_runtime", "runtime_kernel", "column", "supported", 1, 0, 1)
	if !strings.Contains(report.String(), "source=methodjit_q_frame_runtime kernel=FrameColumn shape=column route=typed_runtime_op_exit outcome=error count=1") ||
		!strings.Contains(report.String(), "source=methodjit_q_frame_runtime kind=runtime_kernel shape=column count=1 outcome=supported executions=1 successes=0 errors=1") {
		t.Fatalf("diagnostic report missing typed runtime error stats:\n%s", report.String())
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

func TestQFramePrimitiveSharedPredicateStillLowersToTypedRuntimeKernel(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("size"))
	proto := &vm.FuncProto{
		Name:      "q_frame_shared_predicate_lowered",
		NumParams: 1,
		MaxStack:  5,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			runtime.TableValue(names),
			runtime.StringValue("size"),
			runtime.IntValue(0),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 2, 0, 1),
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 2, 2, 2),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 2, 2, 3),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 3, 0, 3),
			vm.EncodeABx(vm.OP_LOADK, 4, 4),
			vm.EncodeABC(vm.OP_VECTOR_WHERE, 1, 3, 4),
			vm.EncodeABC(vm.OP_RETURN, 1, 3, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
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
		t.Fatalf("OpQFrameSelectColumn count = %d, want 1 for shared predicate path\n%s", counts[OpQFrameSelectColumn], Print(lowered))
	}
	if counts[OpVectorWhere] != 1 || counts[OpVectorCompare] != 1 {
		t.Fatalf("shared predicate vector ops = compare %d where %d, want 1/1\n%s", counts[OpVectorCompare], counts[OpVectorWhere], Print(lowered))
	}
	for _, op := range []Op{OpFrameFilter, OpFrameProject} {
		if counts[op] != 0 {
			t.Fatalf("%s count = %d, want 0 after shared predicate lowering\n%s", op, counts[op], Print(lowered))
		}
	}
	if fallbacks := CountQQueryLoweringFallbackReasons(fn.Remarks.List()); len(fallbacks) != 0 {
		t.Fatalf("q lowering fallback counts = %+v, want none for shared predicate path", fallbacks)
	}

	result, err := Interpret(lowered, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if err != nil {
		t.Fatalf("Interpret lowered shared predicate q hot path: %v", err)
	}
	if len(result) != 2 || !result[0].IsDenseArray() || !result[1].IsDenseArray() {
		t.Fatalf("shared predicate result = %#v, want two dense arrays", result)
	}
	whereVals, ok := result[0].DenseArray().I64()
	if !ok || len(whereVals) != 3 || whereVals[0] != 0 || whereVals[1] != 10 || whereVals[2] != 20 {
		t.Fatalf("shared predicate vector where values = %#v, want [0 10 20]", whereVals)
	}
	selectVals, ok := result[1].DenseArray().I64()
	if !ok || len(selectVals) != 2 || selectVals[0] != 10 || selectVals[1] != 20 {
		t.Fatalf("shared predicate q select values = %#v, want [10 20]", selectVals)
	}
}

func TestQFramePrimitiveHotPathLoweringReportsFallbackReason(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_frame_pipeline_lowering_fallback",
		NumParams: 3,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			qHotPathNamesValue("size"),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 3, 0, 0),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 3, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 3),
			vm.EncodeABC(vm.OP_FRAME_GATHER, 0, 0, 1),
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 1),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 2),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
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
	if counts[OpQFrameSelectColumn] != 0 {
		t.Fatalf("OpQFrameSelectColumn count = %d, want 0 for unsupported dynamic rhs+row path\n%s", counts[OpQFrameSelectColumn], Print(lowered))
	}
	fallbacks := CountQQueryLoweringFallbackReasons(fn.Remarks.List())
	if fallbacks[qQueryLoweringFallbackTooManyDynamicArgs] != 1 {
		t.Fatalf("q lowering fallback counts = %+v, want %s=1", fallbacks, qQueryLoweringFallbackTooManyDynamicArgs)
	}
	got := formatOptimizationRemarks(fn.Remarks.List())
	if !strings.Contains(got, "QQueryNativeLowering") ||
		!strings.Contains(got, "reason_code=too_many_dynamic_args") ||
		!strings.Contains(got, "shape=compare/filter/gather/project/column") {
		t.Fatalf("q lowering fallback remark = %q, want structured reason and shape", got)
	}
}

func TestQFramePrimitiveProjectionOnlyLowersToTypedRuntimeKernel(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_frame_projection_only_lowered",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			qHotPathNamesValue("size"),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 0),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 1),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	paths := DetectQQueryHotPaths(fn)
	if len(paths) != 1 {
		t.Fatalf("DetectQQueryHotPaths count = %d, want 1\n%s", len(paths), Print(fn))
	}
	if got := paths[0].Shape(); got != "project/column" {
		t.Fatalf("projection-only hot path shape = %q, want project/column", got)
	}
	if got := qQueryHotPathPredicateName(paths[0]); got != "none" {
		t.Fatalf("projection-only predicate = %q, want none", got)
	}

	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	if len(lowered.QFrameSelectColumnSpecs) != 1 {
		t.Fatalf("QFrameSelectColumnSpecs count = %d, want 1", len(lowered.QFrameSelectColumnSpecs))
	}
	spec := lowered.QFrameSelectColumnSpecs[0]
	if spec.Shape != "project/column" || spec.SourceColumnConst != -1 || spec.MaskSpecConst != -1 || len(spec.MaskTerms) != 0 {
		t.Fatalf("projection-only lowered spec = %+v, want no predicate", spec)
	}

	result, err := Interpret(lowered, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if err != nil {
		t.Fatalf("Interpret lowered projection-only q hot path: %v", err)
	}
	got, ok := result[0].DenseArray().I64()
	if !ok || len(got) != 3 || got[0] != 5 || got[1] != 10 || got[2] != 20 {
		t.Fatalf("projection-only result values = %#v, want [5 10 20]", got)
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

func TestQFramePrimitiveDynamicCompareConstRowLowersToTypedRuntimeKernel(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_frame_dynamic_compare_const_row_lowered",
		NumParams: 2,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.IntValue(1),
			qHotPathNamesValue("size"),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 2, 0, 0),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 2, 1, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 2),
			vm.EncodeABx(vm.OP_LOADK, 3, 1),
			vm.EncodeABC(vm.OP_FRAME_SLICE, 0, 0, 3),
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
	if len(lowered.QFrameSelectColumnSpecs) != 1 {
		t.Fatalf("QFrameSelectColumnSpecs count = %d, want 1", len(lowered.QFrameSelectColumnSpecs))
	}
	spec := lowered.QFrameSelectColumnSpecs[0]
	if spec.Shape != "compare/filter/slice/project/column" ||
		spec.DynamicArgRole != QFrameSelectColumnArgCompareRHS ||
		!spec.HasRowValueConst ||
		!spec.RowValueConst.IsInt() ||
		spec.RowValueConst.Int() != 1 {
		t.Fatalf("lowered q spec = %+v, want dynamic compare rhs with const slice row", spec)
	}
	result, err := Interpret(lowered, []runtime.Value{
		runtime.TableValue(qHotPathTestFrame(t)),
		runtime.FloatValue(100),
	})
	if err != nil {
		t.Fatalf("Interpret lowered q hot path: %v", err)
	}
	got, ok := result[0].DenseArray().I64()
	if !ok || len(got) != 1 || got[0] != 10 {
		t.Fatalf("lowered result values = %#v, want [10]", got)
	}
}

func TestQFramePrimitiveMaskRowHotPathsLowerToTypedRuntimeKernel(t *testing.T) {
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
				qHotPathMaskValue("price", ">=", runtime.FloatValue(100)),
				qHotPathNamesValue("size"),
				runtime.StringValue("size"),
			},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_MASK, 2, 0, 0),
				vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 2),
				vm.EncodeABC(vm.OP_FRAME_GATHER, 0, 0, 1),
				vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 1),
				vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 2),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: []runtime.Value{
				runtime.TableValue(qHotPathTestFrame(t)),
				runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{2, 1})),
			},
			shape: "mask/filter/gather/project/column",
			want:  []int64{20, 10},
		},
		{
			name:      "slice",
			numParams: 1,
			constants: []runtime.Value{
				qHotPathMaskValue("price", ">=", runtime.FloatValue(100)),
				runtime.IntValue(1),
				qHotPathNamesValue("size"),
				runtime.StringValue("size"),
			},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_MASK, 1, 0, 0),
				vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
				vm.EncodeABx(vm.OP_LOADK, 2, 1),
				vm.EncodeABC(vm.OP_FRAME_SLICE, 0, 0, 2),
				vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 2),
				vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 3),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args:  []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))},
			shape: "mask/filter/slice/project/column",
			want:  []int64{10},
		},
		{
			name:      "order",
			numParams: 1,
			constants: []runtime.Value{
				qHotPathMaskValue("price", ">=", runtime.FloatValue(100)),
				qHotPathOrderValue("price", true),
				qHotPathNamesValue("size"),
				runtime.StringValue("size"),
			},
			code: []uint32{
				vm.EncodeABC(vm.OP_FRAME_MASK, 1, 0, 0),
				vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
				vm.EncodeABC(vm.OP_FRAME_ORDER, 2, 0, 1),
				vm.EncodeABC(vm.OP_FRAME_GATHER, 0, 0, 2),
				vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 2),
				vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 3),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args:  []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))},
			shape: "mask/filter/order/gather/project/column",
			want:  []int64{20, 10},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proto := &vm.FuncProto{
				Name:      "q_frame_mask_row_" + tc.name + "_lowered",
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

func TestQFramePrimitiveHotPathLowersFrameOrderGatherRows(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("size"))
	order := runtime.NewTable()
	order.RawSetString("column", runtime.StringValue("price"))
	order.RawSetString("desc", runtime.BoolValue(true))
	order.RawSetString("limit", runtime.IntValue(2))
	proto := &vm.FuncProto{
		Name:      "q_frame_order_gather_pipeline",
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
			vm.EncodeABC(vm.OP_FRAME_ORDER_GATHER, 0, 0, 2),
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
	if paths[0].RowGather == nil || paths[0].RowOrder == nil || paths[0].RowGather.ID != paths[0].RowOrder.ID {
		t.Fatalf("DetectQQueryHotPaths RowGather=%v RowOrder=%v, want fused OpFrameOrderGather\n%s", paths[0].RowGather, paths[0].RowOrder, Print(fn))
	}
	if got := paths[0].Shape(); got != "compare/filter/order/gather/project/column" {
		t.Fatalf("fused ordered hot path shape = %q, want compare/filter/order/gather/project/column", got)
	}

	fn.Remarks = &OptimizationRemarks{}
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	if counts := countOps(lowered); counts[OpQFrameSelectColumn] != 1 || counts[OpFrameOrderGather] != 0 {
		t.Fatalf("lowered op counts QFrameSelectColumn=%d FrameOrderGather=%d, want 1/0\n%s", counts[OpQFrameSelectColumn], counts[OpFrameOrderGather], Print(lowered))
	}
	if len(lowered.QFrameSelectColumnSpecs) != 1 {
		t.Fatalf("QFrameSelectColumnSpecs = %d, want 1\n%s", len(lowered.QFrameSelectColumnSpecs), Print(lowered))
	}
	spec := lowered.QFrameSelectColumnSpecs[0]
	if spec.RowMode != QFrameSelectColumnRowsOrderGather || spec.RowOrderConst != 2 || spec.Shape != "compare/filter/order/gather/project/column" {
		t.Fatalf("lowered spec row mode/order/shape = %s/%d/%q, want order-gather/2/shape",
			qFrameSelectColumnRowModeName(spec.RowMode), spec.RowOrderConst, spec.Shape)
	}
	descriptors := BuildQKernelDescriptors(DetectQVectorRuntimeKernels(lowered), DetectQFrameRuntimeKernels(lowered), lowered.QFrameSelectColumnSpecs, fn.Remarks.List())
	assertQKernelDescriptor(t, descriptors, "methodjit_q_frame_runtime", "runtime_kernel", "QFrameSelectColumn", "compare/filter/order/gather/project/column", "typed_runtime_op_exit", "supported", "")
	summary := BuildQKernelShapeSummaryFromDescriptors(descriptors)
	assertQKernelShapeSummary(t, summary, "methodjit_q_frame_runtime", "runtime_kernel", "compare/filter/order/gather/project/column", "supported", "", 1)
}

func TestQFrameSelectColumnOrderGatherDiagnoseReportsNativeExecutionStats(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_frame_order_gather_select_column_execution_stats",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			qHotPathOrderValue("price", true),
			qHotPathNamesValue("size"),
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

	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if report.InterpError != nil || report.OptInterpError != nil || report.NativeError != nil {
		t.Fatalf("Diagnose ordered q frame select errors: unopt=%v opt=%v native=%v\n%s",
			report.InterpError, report.OptInterpError, report.NativeError, report.String())
	}
	if len(report.QTypedRuntimeKernels) != 1 {
		t.Fatalf("Diagnose QTypedRuntimeKernels = %d, want 1\n%s", len(report.QTypedRuntimeKernels), report.String())
	}
	const shape = "compare/filter/order/gather/project/column"
	if report.QTypedRuntimeKernels[0].Shape != shape {
		t.Fatalf("Diagnose QTypedRuntimeKernels[0].Shape = %q, want %s", report.QTypedRuntimeKernels[0].Shape, shape)
	}
	if len(report.NativeResult) != 1 || !report.NativeResult[0].IsDenseArray() {
		t.Fatalf("ordered q frame native result = %#v, want dense array", report.NativeResult)
	}
	got, ok := report.NativeResult[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 20 || got[1] != 10 {
		t.Fatalf("ordered q frame native values = %#v, want [20 10]", got)
	}

	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_frame_runtime", "runtime_kernel", "QFrameSelectColumn", shape, "typed_runtime_op_exit", "supported", "")
	assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_frame_runtime", "QFrameSelectColumn", shape, "typed_runtime_op_exit", "success", 1)
	assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_frame_runtime", "runtime_kernel", shape, "supported", "", 1)
	assertQKernelShapeSummaryExecution(t, report.QKernelShapeSummary, "methodjit_q_frame_runtime", "runtime_kernel", shape, "supported", 1, 1, 0)
	if !strings.Contains(report.String(), "source=methodjit_q_frame_runtime kernel=QFrameSelectColumn shape="+shape+" route=typed_runtime_op_exit outcome=success count=1") ||
		!strings.Contains(report.String(), "executions=1 successes=1 errors=0") {
		t.Fatalf("diagnostic report missing ordered QFrameSelectColumn execution stats:\n%s", report.String())
	}
}

func TestQFramePrimitiveHotPathKeepsCrossFrameOrderGatherAsGather(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("size"))
	order := runtime.NewTable()
	order.RawSetString("column", runtime.StringValue("price"))
	order.RawSetString("desc", runtime.BoolValue(true))
	proto := &vm.FuncProto{
		Name:      "q_cross_frame_order_gather_pipeline",
		NumParams: 2,
		MaxStack:  5,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(0),
			runtime.TableValue(order),
			runtime.TableValue(names),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 2, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 3, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 2, 3, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 2),
			vm.EncodeABC(vm.OP_FRAME_ORDER, 4, 1, 2),
			vm.EncodeABC(vm.OP_FRAME_GATHER, 0, 0, 4),
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
	if paths[0].RowOrder != nil {
		t.Fatalf("DetectQQueryHotPaths RowOrder = %v, want nil for cross-frame order indexes\n%s", paths[0].RowOrder, Print(fn))
	}
	if got := paths[0].Shape(); got != "compare/filter/gather/project/column" {
		t.Fatalf("cross-frame ordered gather shape = %q, want compare/filter/gather/project/column", got)
	}

	fn.Remarks = &OptimizationRemarks{}
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	if len(lowered.QFrameSelectColumnSpecs) != 1 {
		t.Fatalf("QFrameSelectColumnSpecs = %d, want 1\n%s", len(lowered.QFrameSelectColumnSpecs), Print(lowered))
	}
	if got := lowered.QFrameSelectColumnSpecs[0].Shape; got != "compare/filter/gather/project/column" {
		t.Fatalf("lowered cross-frame order-gather shape = %q, want compare/filter/gather/project/column", got)
	}
	descriptors := BuildQKernelDescriptors(DetectQVectorRuntimeKernels(lowered), DetectQFrameRuntimeKernels(lowered), lowered.QFrameSelectColumnSpecs, fn.Remarks.List())
	assertQKernelDescriptor(t, descriptors, "methodjit_q_frame_runtime", "runtime_kernel", "QFrameSelectColumn", "compare/filter/gather/project/column", "typed_runtime_op_exit", "supported", "")
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

func TestQFramePrimitiveHotPathDetectsVectorMaskPredicate(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_frame_vector_mask_pipeline",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			qHotPathMaskValue("size", ">=", runtime.IntValue(10)),
			qHotPathNamesValue("size"),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_MASK, 2, 0, 2),
			vm.EncodeABC(vm.OP_VECTOR_MASK, 1, 2, int(runtime.DenseArrayMaskAnd)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 1),
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
	if got := paths[0].Shape(); got != "mask-combine/filter/project/column" {
		t.Fatalf("q vector mask hot path shape = %q, want mask-combine/filter/project/column", got)
	}
	if paths[0].MaskCombine == nil {
		t.Fatalf("q vector mask hot path MaskCombine = nil\n%s", Print(fn))
	}

	remarks := &OptimizationRemarks{}
	fn.Remarks = remarks
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	if len(lowered.QFrameSelectColumnSpecs) != 1 {
		t.Fatalf("QFrameSelectColumnSpecs = %+v, want one fused lowering for mask combine", lowered.QFrameSelectColumnSpecs)
	}
	spec := lowered.QFrameSelectColumnSpecs[0]
	if spec.Shape != "mask-combine/filter/project/column" || len(spec.MaskTerms) != 3 || spec.MaskRoot != 2 {
		t.Fatalf("lowered mask-combine spec = %+v, want three mask terms with root 2", spec)
	}
	if got := CountQQueryLoweringFallbackReasons(remarks.List()); len(got) != 0 {
		t.Fatalf("fallback reasons = %+v, want none after mask-combine lowering", got)
	}
	result, err := Interpret(lowered, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if err != nil {
		t.Fatalf("Interpret lowered vector mask q hot path: %v", err)
	}
	got, ok := result[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Fatalf("lowered vector mask result values = %#v, want [10 20]", got)
	}
}

func TestQVectorWhereHotPathDiagnosesConditionalProjection(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_vector_where_conditional_projection",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			runtime.StringValue("size"),
			runtime.IntValue(0),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 2, 0, 2),
			vm.EncodeABx(vm.OP_LOADK, 3, 3),
			vm.EncodeABC(vm.OP_VECTOR_WHERE, 1, 2, 3),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	paths := DetectQVectorWhereHotPaths(fn)
	if len(paths) != 1 {
		t.Fatalf("DetectQVectorWhereHotPaths count = %d, want 1\n%s", len(paths), Print(fn))
	}
	if got := paths[0].Shape(); got != "compare/vector-where" {
		t.Fatalf("q vector where shape = %q, want compare/vector-where", got)
	}
	if paths[0].Compare == nil || paths[0].TrueColumn == nil || paths[0].FalseColumn != nil {
		t.Fatalf("q vector where path = %+v, want compare predicate, frame true column, scalar false operand", paths[0])
	}
	if got := formatQVectorWhereHotPaths(paths); !strings.Contains(got, "shape=compare/vector-where") ||
		!strings.Contains(got, "predicate=compare >=") ||
		!strings.Contains(got, "true=frame-column") ||
		!strings.Contains(got, "false=scalar") {
		t.Fatalf("formatted vector where hot paths missing details:\n%s", got)
	}

	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if len(report.QVectorWhereHotPaths) != 1 {
		t.Fatalf("Diagnose QVectorWhereHotPaths = %d, want 1\n%s", len(report.QVectorWhereHotPaths), report.String())
	}
	if report.QVectorWhereHotPathShapes["compare/vector-where"] != 1 {
		t.Fatalf("Diagnose QVectorWhereHotPathShapes = %+v, want compare/vector-where count 1", report.QVectorWhereHotPathShapes)
	}
	if report.QVectorRuntimeKernelShapes["compare/vector-where"] != 1 {
		t.Fatalf("Diagnose QVectorRuntimeKernelShapes = %+v, want compare/vector-where count 1", report.QVectorRuntimeKernelShapes)
	}
	if !strings.Contains(report.String(), "Q vector conditional hot paths") ||
		!strings.Contains(report.String(), "Q typed vector runtime kernels") ||
		!strings.Contains(report.String(), "kernel=VectorWhere") ||
		!strings.Contains(report.String(), "shapes: compare/vector-where=1") {
		t.Fatalf("diagnostic report missing q vector where typed kernel shape:\n%s", report.String())
	}
}

func TestQVectorReduceHotPathDiagnosesAggregateKernel(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_vector_reduce_aggregate",
		NumParams: 1,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 1, 0, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	paths := DetectQVectorReduceHotPaths(fn)
	if len(paths) != 1 {
		t.Fatalf("DetectQVectorReduceHotPaths count = %d, want 1\n%s", len(paths), Print(fn))
	}
	if got := paths[0].Shape(); got != "column/vector-reduce" {
		t.Fatalf("q vector reduce shape = %q, want column/vector-reduce", got)
	}
	if got := qVectorReduceOpName(paths[0]); got != "sum" {
		t.Fatalf("q vector reduce op = %q, want sum", got)
	}
	if got := qVectorReduceInputName(paths[0]); got != "frame-column" {
		t.Fatalf("q vector reduce input = %q, want frame-column", got)
	}

	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if len(report.QVectorReduceHotPaths) != 1 {
		t.Fatalf("Diagnose QVectorReduceHotPaths = %d, want 1\n%s", len(report.QVectorReduceHotPaths), report.String())
	}
	if report.QVectorReduceHotPathShapes["column/vector-reduce"] != 1 {
		t.Fatalf("Diagnose QVectorReduceHotPathShapes = %+v, want column/vector-reduce count 1", report.QVectorReduceHotPathShapes)
	}
	if report.QVectorRuntimeKernelShapes["column/vector-reduce"] != 1 {
		t.Fatalf("Diagnose QVectorRuntimeKernelShapes = %+v, want column/vector-reduce count 1", report.QVectorRuntimeKernelShapes)
	}
	if !strings.Contains(report.String(), "Q typed vector runtime kernels") ||
		!strings.Contains(report.String(), "kernel=VectorReduce") ||
		!strings.Contains(report.String(), "op=sum") ||
		!strings.Contains(report.String(), "shapes: column/vector-reduce=1") {
		t.Fatalf("diagnostic report missing q vector reduce typed kernel shape:\n%s", report.String())
	}
}

func TestQVectorWhereReduceLowersToFusedTypedRuntimeKernel(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_vector_where_reduce",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			runtime.StringValue("size"),
			runtime.IntValue(0),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 2, 0, 2),
			vm.EncodeABx(vm.OP_LOADK, 3, 3),
			vm.EncodeABC(vm.OP_VECTOR_WHERE, 1, 2, 3),
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 1, 1, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	counts := countOps(lowered)
	if counts[OpQVectorWhereReduce] != 1 {
		t.Fatalf("OpQVectorWhereReduce count = %d, want 1\n%s", counts[OpQVectorWhereReduce], Print(lowered))
	}
	if counts[OpVectorWhere] != 0 || counts[OpVectorReduce] != 0 {
		t.Fatalf("vector where/reduce counts = %d/%d, want 0/0 after fusion\n%s", counts[OpVectorWhere], counts[OpVectorReduce], Print(lowered))
	}
	if counts[OpVectorCompare] != 1 {
		t.Fatalf("OpVectorCompare count = %d, want compare mask retained as fused input\n%s", counts[OpVectorCompare], Print(lowered))
	}

	kernels := DetectQVectorRuntimeKernels(lowered)
	if len(kernels) != 2 {
		t.Fatalf("DetectQVectorRuntimeKernels count = %d, want compare + fused kernel\n%s", len(kernels), Print(lowered))
	}
	if got := CountQVectorRuntimeKernelShapes(kernels)["compare/vector-where/vector-reduce"]; got != 1 {
		t.Fatalf("QVectorRuntimeKernelShapes = %+v, want fused compare/vector-where/vector-reduce", CountQVectorRuntimeKernelShapes(kernels))
	}
	if got := formatQTypedVectorRuntimeKernelReport(kernels); !strings.Contains(got, "kernel=QVectorWhereReduce") ||
		!strings.Contains(got, "shape=compare/vector-where/vector-reduce") ||
		!strings.Contains(got, "op=sum") ||
		!strings.Contains(got, "predicate=compare >=") {
		t.Fatalf("formatted vector runtime kernels missing fused where-reduce:\n%s", got)
	}
	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if report.QVectorRuntimeKernelShapes["compare/vector-where/vector-reduce"] != 1 {
		t.Fatalf("Diagnose QVectorRuntimeKernelShapes = %+v, want fused compare/vector-where/vector-reduce", report.QVectorRuntimeKernelShapes)
	}
	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_vector_runtime", "runtime_kernel", "QVectorWhereReduce", "compare/vector-where/vector-reduce", "typed_runtime_op_exit", "supported", "")
	assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_vector_runtime", "runtime_kernel", "compare/vector-where/vector-reduce", "supported", "", 1)
	assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_vector_runtime", "QVectorWhereReduce", "compare/vector-where/vector-reduce", "typed_runtime_op_exit", "success", 1)
	if !strings.Contains(report.String(), "kernel=QVectorWhereReduce") {
		t.Fatalf("diagnostic report missing fused q vector where-reduce kernel:\n%s", report.String())
	}
	if !strings.Contains(report.String(), "Q kernel descriptors") ||
		!strings.Contains(report.String(), "source=methodjit_q_vector_runtime kind=runtime_kernel kernel=QVectorWhereReduce shape=compare/vector-where/vector-reduce route=typed_runtime_op_exit outcome=supported") ||
		!strings.Contains(report.String(), "Q kernel execution stats") ||
		!strings.Contains(report.String(), "source=methodjit_q_vector_runtime kernel=QVectorWhereReduce shape=compare/vector-where/vector-reduce route=typed_runtime_op_exit outcome=success count=1") ||
		!strings.Contains(report.String(), "Q kernel shape summary") ||
		!strings.Contains(report.String(), "source=methodjit_q_vector_runtime kind=runtime_kernel shape=compare/vector-where/vector-reduce count=1 outcome=supported executions=1 successes=1 errors=0") {
		t.Fatalf("diagnostic report missing q kernel descriptor/summary:\n%s", report.String())
	}

	result, err := Interpret(lowered, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if err != nil {
		t.Fatalf("Interpret lowered q vector where-reduce: %v", err)
	}
	if len(result) != 1 || !result[0].IsInt() || result[0].Int() != 30 {
		t.Fatalf("lowered where-reduce result = %#v, want int 30", result)
	}
}

func TestQVectorWhereReduceLowersMaskCombineToFusedTypedRuntimeKernel(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_vector_where_reduce_mask_combine",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			qHotPathMaskValue("size", ">=", runtime.IntValue(10)),
			runtime.StringValue("size"),
			runtime.IntValue(0),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_MASK, 2, 0, 2),
			vm.EncodeABC(vm.OP_VECTOR_MASK, 1, 2, int(runtime.DenseArrayMaskAnd)),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 2, 0, 3),
			vm.EncodeABx(vm.OP_LOADK, 3, 4),
			vm.EncodeABC(vm.OP_VECTOR_WHERE, 1, 2, 3),
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 1, 1, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	counts := countOps(lowered)
	if counts[OpQVectorWhereReduce] != 1 {
		t.Fatalf("OpQVectorWhereReduce count = %d, want 1\n%s", counts[OpQVectorWhereReduce], Print(lowered))
	}
	if counts[OpVectorWhere] != 0 || counts[OpVectorReduce] != 0 {
		t.Fatalf("vector where/reduce counts = %d/%d, want 0/0 after mask-combine fusion\n%s", counts[OpVectorWhere], counts[OpVectorReduce], Print(lowered))
	}
	if counts[OpVectorMask] != 1 || counts[OpVectorCompare] != 1 || counts[OpFrameMask] != 1 {
		t.Fatalf("mask-combine inputs counts compare/mask/frame-mask = %d/%d/%d, want 1/1/1\n%s",
			counts[OpVectorCompare], counts[OpVectorMask], counts[OpFrameMask], Print(lowered))
	}
	if fallbacks := CountQVectorLoweringFallbackReasons(fn.Remarks.List()); len(fallbacks) != 0 {
		t.Fatalf("q vector lowering fallback counts = %+v, want none for mask-combine fusion", fallbacks)
	}

	kernels := DetectQVectorRuntimeKernels(lowered)
	if got := CountQVectorRuntimeKernelShapes(kernels)["mask-combine/vector-where/vector-reduce"]; got != 1 {
		t.Fatalf("QVectorRuntimeKernelShapes = %+v, want fused mask-combine/vector-where/vector-reduce", CountQVectorRuntimeKernelShapes(kernels))
	}
	if got := formatQTypedVectorRuntimeKernelReport(kernels); !strings.Contains(got, "kernel=QVectorWhereReduce") ||
		!strings.Contains(got, "shape=mask-combine/vector-where/vector-reduce") ||
		!strings.Contains(got, "op=sum") ||
		!strings.Contains(got, "predicate=mask-combine and") {
		t.Fatalf("formatted vector runtime kernels missing fused mask-combine where-reduce:\n%s", got)
	}

	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if report.QVectorRuntimeKernelShapes["mask-combine/vector-where/vector-reduce"] != 1 {
		t.Fatalf("Diagnose QVectorRuntimeKernelShapes = %+v, want fused mask-combine/vector-where/vector-reduce", report.QVectorRuntimeKernelShapes)
	}
	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_vector_runtime", "runtime_kernel", "QVectorWhereReduce", "mask-combine/vector-where/vector-reduce", "typed_runtime_op_exit", "supported", "")
	assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_vector_runtime", "runtime_kernel", "mask-combine/vector-where/vector-reduce", "supported", "", 1)
	assertQKernelShapeSummaryExecution(t, report.QKernelShapeSummary, "methodjit_q_vector_runtime", "runtime_kernel", "mask-combine/vector-where/vector-reduce", "supported", 1, 1, 0)
	assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_vector_runtime", "QVectorWhereReduce", "mask-combine/vector-where/vector-reduce", "typed_runtime_op_exit", "success", 1)
	assertQKernelExecutionRouteSummary(t, report.QKernelExecutionRoutes, "methodjit_q_vector_runtime", "QVectorWhereReduce", "typed_runtime_op_exit", "success", 1)
	if len(report.QVectorLoweringFallbacks) != 0 {
		t.Fatalf("Diagnose QVectorLoweringFallbacks = %+v, want none for mask-combine fusion\n%s", report.QVectorLoweringFallbacks, report.String())
	}
	if !report.OptimizerMatch || !report.BackendMatch || !report.Match {
		t.Fatalf("Diagnose q vector mask-combine where-reduce mismatch: optimizer=%v %s backend=%v %s match=%v %s\n%s",
			report.OptimizerMatch, report.OptimizerMismatch,
			report.BackendMatch, report.BackendMismatch,
			report.Match, report.Mismatch,
			report.String())
	}

	result, err := Interpret(lowered, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if err != nil {
		t.Fatalf("Interpret lowered q vector mask-combine where-reduce: %v", err)
	}
	if len(result) != 1 || !result[0].IsInt() || result[0].Int() != 30 {
		t.Fatalf("lowered mask-combine where-reduce result = %#v, want int 30", result)
	}
}

func TestQVectorGatherReduceLowersToFusedTypedRuntimeKernel(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_vector_gather_reduce",
		NumParams: 2,
		MaxStack:  3,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 2, 0, 0),
			vm.EncodeABC(vm.OP_VECTOR_GATHER, 2, 1, 0),
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 2, 2, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 2, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	counts := countOps(lowered)
	if counts[OpQVectorGatherReduce] != 1 {
		t.Fatalf("OpQVectorGatherReduce count = %d, want 1\n%s", counts[OpQVectorGatherReduce], Print(lowered))
	}
	if counts[OpVectorGather] != 0 || counts[OpVectorReduce] != 0 {
		t.Fatalf("vector gather/reduce counts = %d/%d, want 0/0 after fusion\n%s", counts[OpVectorGather], counts[OpVectorReduce], Print(lowered))
	}
	if fallbacks := CountQVectorLoweringFallbackReasons(fn.Remarks.List()); len(fallbacks) != 0 {
		t.Fatalf("q vector lowering fallback counts = %+v, want none for gather-reduce fusion", fallbacks)
	}

	kernels := DetectQVectorRuntimeKernels(lowered)
	if got := CountQVectorRuntimeKernelShapes(kernels)["gather/vector-reduce"]; got != 1 {
		t.Fatalf("QVectorRuntimeKernelShapes = %+v, want fused gather/vector-reduce", CountQVectorRuntimeKernelShapes(kernels))
	}
	if got := formatQTypedVectorRuntimeKernelReport(kernels); !strings.Contains(got, "kernel=QVectorGatherReduce") ||
		!strings.Contains(got, "shape=gather/vector-reduce") ||
		!strings.Contains(got, "op=sum") {
		t.Fatalf("formatted vector runtime kernels missing fused gather-reduce:\n%s", got)
	}

	args := []runtime.Value{
		runtime.TableValue(qHotPathTestFrame(t)),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1})),
	}
	report := Diagnose(proto, args)
	if report.QVectorRuntimeKernelShapes["gather/vector-reduce"] != 1 {
		t.Fatalf("Diagnose QVectorRuntimeKernelShapes = %+v, want fused gather/vector-reduce", report.QVectorRuntimeKernelShapes)
	}
	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_vector_runtime", "runtime_kernel", "QVectorGatherReduce", "gather/vector-reduce", "typed_runtime_op_exit", "supported", "")
	assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_vector_runtime", "runtime_kernel", "gather/vector-reduce", "supported", "", 1)
	assertQKernelShapeSummaryExecution(t, report.QKernelShapeSummary, "methodjit_q_vector_runtime", "runtime_kernel", "gather/vector-reduce", "supported", 1, 1, 0)
	assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_vector_runtime", "QVectorGatherReduce", "gather/vector-reduce", "typed_runtime_op_exit", "success", 1)
	assertQKernelExecutionRouteSummary(t, report.QKernelExecutionRoutes, "methodjit_q_vector_runtime", "QVectorGatherReduce", "typed_runtime_op_exit", "success", 1)

	result, err := Interpret(lowered, args)
	if err != nil {
		t.Fatalf("Interpret lowered q vector gather-reduce: %v", err)
	}
	if len(result) != 1 || !result[0].IsFloat() || result[0].Float() != 200.25 {
		t.Fatalf("lowered gather-reduce result = %#v, want float 200.25", result)
	}
}

func TestQVectorGatherReduceSharedGatherDoesNotLower(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_vector_gather_reduce_shared",
		NumParams: 2,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 2, 0, 0),
			vm.EncodeABC(vm.OP_VECTOR_GATHER, 2, 1, 0),
			vm.EncodeABC(vm.OP_MOVE, 3, 2, 0),
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 2, 2, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_VECTOR_SCAN, 3, 3, 0),
			vm.EncodeABC(vm.OP_RETURN, 2, 3, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	counts := countOps(lowered)
	if counts[OpQVectorGatherReduce] != 0 {
		t.Fatalf("OpQVectorGatherReduce count = %d, want 0 for shared gather\n%s", counts[OpQVectorGatherReduce], Print(lowered))
	}
	if counts[OpVectorGather] != 1 || counts[OpVectorReduce] != 1 || counts[OpVectorScan] != 1 {
		t.Fatalf("shared vector primitive counts gather/reduce/scan = %d/%d/%d, want 1/1/1\n%s",
			counts[OpVectorGather], counts[OpVectorReduce], counts[OpVectorScan], Print(lowered))
	}
	vectorFallbacks := CountQVectorLoweringFallbackReasons(fn.Remarks.List())
	if vectorFallbacks[qVectorGatherReduceFallbackSharedGather] != 1 {
		t.Fatalf("q vector lowering fallback counts = %+v, want shared_gather=1", vectorFallbacks)
	}
	assertQLoweringRemarkFields(t, fn.Remarks.List(), "QVectorNativeLowering", "QVectorGatherReduce", "gather/vector-reduce", qVectorGatherReduceFallbackSharedGather)
	formatted := formatOptimizationRemarks(fn.Remarks.List())
	if !strings.Contains(formatted, "kernel=QVectorGatherReduce") ||
		!strings.Contains(formatted, "reason_code=shared_gather") ||
		!strings.Contains(formatted, "shape=gather/vector-reduce") {
		t.Fatalf("q vector gather-reduce fallback remark missing stable kernel/reason/shape:\n%s", formatted)
	}

	report := Diagnose(proto, []runtime.Value{
		runtime.TableValue(qHotPathTestFrame(t)),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1})),
	})
	if report.QVectorLoweringFallbacks[qVectorGatherReduceFallbackSharedGather] != 1 {
		t.Fatalf("Diagnose QVectorLoweringFallbacks = %+v, want shared_gather=1\n%s", report.QVectorLoweringFallbacks, report.String())
	}
	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_vector_lowering", "fallback", "QVectorGatherReduce", "gather/vector-reduce", "lowering", "fallback", qVectorGatherReduceFallbackSharedGather)
	assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_vector_lowering", "fallback", "gather/vector-reduce", "fallback", qVectorGatherReduceFallbackSharedGather, 1)
}

func TestQVectorWhereReduceSharedWhereDoesNotLower(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_vector_where_reduce_shared",
		NumParams: 1,
		MaxStack:  5,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			runtime.StringValue("size"),
			runtime.IntValue(0),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 2, 0, 2),
			vm.EncodeABx(vm.OP_LOADK, 3, 3),
			vm.EncodeABC(vm.OP_VECTOR_WHERE, 1, 2, 3),
			vm.EncodeABC(vm.OP_MOVE, 4, 1, 0),
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 1, 1, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_VECTOR_SCAN, 4, 4, 0),
			vm.EncodeABC(vm.OP_MOVE, 2, 4, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 3, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	counts := countOps(lowered)
	if counts[OpQVectorWhereReduce] != 0 {
		t.Fatalf("OpQVectorWhereReduce count = %d, want 0 for shared where\n%s", counts[OpQVectorWhereReduce], Print(lowered))
	}
	if counts[OpVectorWhere] != 1 || counts[OpVectorReduce] != 1 || counts[OpVectorScan] != 1 {
		t.Fatalf("shared vector primitive counts where/reduce/scan = %d/%d/%d, want 1/1/1\n%s",
			counts[OpVectorWhere], counts[OpVectorReduce], counts[OpVectorScan], Print(lowered))
	}
	vectorFallbacks := CountQVectorLoweringFallbackReasons(fn.Remarks.List())
	if vectorFallbacks[qVectorWhereReduceFallbackSharedWhere] != 1 {
		t.Fatalf("q vector lowering fallback counts = %+v, want shared_where=1", vectorFallbacks)
	}
	if queryFallbacks := CountQQueryLoweringFallbackReasons(fn.Remarks.List()); len(queryFallbacks) != 0 {
		t.Fatalf("q query fallback counts = %+v, want none for vector lowering fallback", queryFallbacks)
	}
	formatted := formatOptimizationRemarks(fn.Remarks.List())
	if !strings.Contains(formatted, "QVectorNativeLowering") ||
		!strings.Contains(formatted, "reason_code=shared_where") ||
		!strings.Contains(formatted, "shape=compare/vector-where/vector-reduce") {
		t.Fatalf("q vector lowering fallback remark missing stable reason/shape:\n%s", formatted)
	}

	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if report.QVectorLoweringFallbacks[qVectorWhereReduceFallbackSharedWhere] != 1 {
		t.Fatalf("Diagnose QVectorLoweringFallbacks = %+v, want shared_where=1\n%s", report.QVectorLoweringFallbacks, report.String())
	}
	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_vector_lowering", "fallback", "QVectorWhereReduce", "compare/vector-where/vector-reduce", "lowering", "fallback", qVectorWhereReduceFallbackSharedWhere)
	assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_vector_lowering", "fallback", "compare/vector-where/vector-reduce", "fallback", qVectorWhereReduceFallbackSharedWhere, 1)
	if len(report.QQueryFallbacks) != 0 {
		t.Fatalf("Diagnose QQueryFallbacks = %+v, want none for vector fallback", report.QQueryFallbacks)
	}
	if !strings.Contains(report.String(), "Q vector fallback reasons") ||
		!strings.Contains(report.String(), "shared_where=1") {
		t.Fatalf("diagnostic report missing q vector fallback reason:\n%s", report.String())
	}
}

func TestQVectorWhereReduceBadWhereArgCountReportsFallbackReason(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_vector_where_reduce_bad_where_args",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			runtime.FloatValue(100),
			runtime.StringValue("size"),
			runtime.IntValue(0),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 2, 1),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 2, 0, 2),
			vm.EncodeABx(vm.OP_LOADK, 3, 3),
			vm.EncodeABC(vm.OP_VECTOR_WHERE, 1, 2, 3),
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 1, 1, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	var where *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpVectorWhere {
				where = instr
				break
			}
		}
	}
	if where == nil || len(where.Args) != 3 {
		t.Fatalf("test setup missing VectorWhere with 3 args:\n%s", Print(fn))
	}
	where.Args = where.Args[:2]

	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	counts := countOps(lowered)
	if counts[OpQVectorWhereReduce] != 0 {
		t.Fatalf("OpQVectorWhereReduce count = %d, want 0 for malformed where\n%s", counts[OpQVectorWhereReduce], Print(lowered))
	}
	vectorFallbacks := CountQVectorLoweringFallbackReasons(fn.Remarks.List())
	if vectorFallbacks[qVectorWhereReduceFallbackBadWhereArgCount] != 1 {
		t.Fatalf("q vector lowering fallback counts = %+v, want bad_where_arg_count=1", vectorFallbacks)
	}
	formatted := formatOptimizationRemarks(fn.Remarks.List())
	if !strings.Contains(formatted, "reason_code=bad_where_arg_count") ||
		!strings.Contains(formatted, "shape=vector-where/vector-reduce") {
		t.Fatalf("q vector bad-arg fallback remark missing stable reason/shape:\n%s", formatted)
	}
}

func TestQVectorReduceColumnInputReportsUnsupportedLoweringShape(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_vector_reduce_column_input",
		NumParams: 1,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 1, 0, 0),
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 1, 1, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	counts := countOps(lowered)
	if counts[OpQVectorWhereReduce] != 0 {
		t.Fatalf("OpQVectorWhereReduce count = %d, want 0 for column reduce\n%s", counts[OpQVectorWhereReduce], Print(lowered))
	}
	if counts[OpVectorReduce] != 1 {
		t.Fatalf("OpVectorReduce count = %d, want 1 for column reduce\n%s", counts[OpVectorReduce], Print(lowered))
	}
	vectorFallbacks := CountQVectorLoweringFallbackReasons(fn.Remarks.List())
	if vectorFallbacks[qVectorWhereReduceFallbackUnsupportedInput] != 1 {
		t.Fatalf("q vector lowering fallback counts = %+v, want unsupported_input_shape=1", vectorFallbacks)
	}
	formatted := formatOptimizationRemarks(fn.Remarks.List())
	if !strings.Contains(formatted, "reason_code=unsupported_input_shape") ||
		!strings.Contains(formatted, "shape=column/vector-reduce") {
		t.Fatalf("q vector unsupported-input fallback remark missing stable reason/shape:\n%s", formatted)
	}

	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if report.QVectorLoweringFallbacks[qVectorWhereReduceFallbackUnsupportedInput] != 1 {
		t.Fatalf("Diagnose QVectorLoweringFallbacks = %+v, want unsupported_input_shape=1\n%s", report.QVectorLoweringFallbacks, report.String())
	}
	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_vector_lowering", "fallback", "QVectorWhereReduce", "column/vector-reduce", "lowering", "fallback", qVectorWhereReduceFallbackUnsupportedInput)
	assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_vector_lowering", "fallback", "column/vector-reduce", "fallback", qVectorWhereReduceFallbackUnsupportedInput, 1)
	if !strings.Contains(report.String(), "unsupported_input_shape=1") {
		t.Fatalf("diagnostic report missing unsupported vector fallback reason:\n%s", report.String())
	}
}

func TestQGroupAggregateCallReportsStructuredFallback(t *testing.T) {
	const query = "select notional:sum price*size, fills:count i by sym from trades where price>100 order by sym asc"
	proto := &vm.FuncProto{
		Name:      "q_group_aggregate_call",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("q"),
			runtime.StringValue("sql"),
			runtime.StringValue(query),
		},
		Code: []uint32{
			vm.EncodeABx(vm.OP_GETGLOBAL, 1, 0),
			vm.EncodeABC(vm.OP_GETFIELD, 1, 1, 1),
			vm.EncodeABC(vm.OP_MOVE, 2, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 3, 2),
			vm.EncodeABC(vm.OP_CALL, 1, 3, 2),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	if counts := countOps(lowered); counts[OpCall] != 1 {
		t.Fatalf("group aggregate call op count = %d, want opaque OpCall to remain\n%s", counts[OpCall], Print(lowered))
	}
	fallbacks := CountQQueryLoweringFallbackReasons(fn.Remarks.List())
	if fallbacks[qQueryLoweringFallbackGroupAggregateCall] != 1 {
		t.Fatalf("q query fallback counts = %+v, want group_aggregate_call=1", fallbacks)
	}
	assertQLoweringRemarkFields(t, fn.Remarks.List(), "QQueryNativeLowering", "QGroupAggregate", "select/where/group/aggregate/order", qQueryLoweringFallbackGroupAggregateCall)
	descriptors := BuildQKernelDescriptors(nil, nil, nil, fn.Remarks.List())
	assertQKernelDescriptor(t, descriptors, "methodjit_q_query_lowering", "fallback", "QGroupAggregate", "select/where/group/aggregate/order", "lowering", "fallback", qQueryLoweringFallbackGroupAggregateCall)
	summary := BuildQKernelShapeSummaryFromDescriptors(descriptors)
	assertQKernelShapeSummary(t, summary, "methodjit_q_query_lowering", "fallback", "select/where/group/aggregate/order", "fallback", qQueryLoweringFallbackGroupAggregateCall, 1)
	formatted := formatOptimizationRemarks(fn.Remarks.List())
	if !strings.Contains(formatted, "kernel=QGroupAggregate") ||
		!strings.Contains(formatted, "reason_code=group_aggregate_call") ||
		!strings.Contains(formatted, "shape=select/where/group/aggregate/order") {
		t.Fatalf("group aggregate fallback remark missing stable taxonomy:\n%s", formatted)
	}
}

func TestQGroupAggregateCallLowersToFrameGroupAggregateKernel(t *testing.T) {
	const query = "select total:sum price, fills:count i by size from trades"
	proto := &vm.FuncProto{
		Name:      "q_group_aggregate_simple_call",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("q"),
			runtime.StringValue("sql"),
			runtime.StringValue(query),
		},
		Code: []uint32{
			vm.EncodeABx(vm.OP_GETGLOBAL, 1, 0),
			vm.EncodeABC(vm.OP_GETFIELD, 1, 1, 1),
			vm.EncodeABC(vm.OP_MOVE, 2, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 3, 2),
			vm.EncodeABC(vm.OP_CALL, 1, 3, 2),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	counts := countOps(lowered)
	if counts[OpFrameGroupAggregate] != 1 || counts[OpCall] != 0 {
		t.Fatalf("group aggregate lowering counts FrameGroupAggregate=%d OpCall=%d\n%s", counts[OpFrameGroupAggregate], counts[OpCall], Print(lowered))
	}
	if fallbacks := CountQQueryLoweringFallbackReasons(fn.Remarks.List()); fallbacks[qQueryLoweringFallbackGroupAggregateCall] != 0 {
		t.Fatalf("q query fallback counts = %+v, want no group aggregate fallback", fallbacks)
	}
	kernels := DetectQFrameRuntimeKernels(lowered)
	assertQKernelDescriptor(t, BuildQKernelDescriptors(nil, kernels, nil, fn.Remarks.List()),
		"methodjit_q_frame_runtime", "runtime_kernel", "FrameGroupAggregate", "group/aggregate", "typed_runtime_op_exit", "supported", "")

	result, err := Interpret(lowered, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if err != nil {
		t.Fatalf("Interpret lowered q group aggregate: %v", err)
	}
	if len(result) != 1 || !result[0].IsTable() {
		t.Fatalf("lowered result = %#v, want one native frame table", result)
	}
	payload, info, ok := result[0].Table().NativeFramePayload()
	if !ok || info.Rows != 3 || info.Columns != 3 {
		t.Fatalf("lowered result payload = %#v info=%#v ok=%v, want 3x3 native frame", payload, info, ok)
	}
	soa, ok := payload.(*runtime.SoA)
	if !ok {
		t.Fatalf("lowered result payload type = %T, want *runtime.SoA", payload)
	}
	size, ok := soa.Column("size")
	if !ok {
		t.Fatalf("lowered result missing size column")
	}
	total, ok := soa.Column("total")
	if !ok {
		t.Fatalf("lowered result missing total column")
	}
	fills, ok := soa.Column("fills")
	if !ok {
		t.Fatalf("lowered result missing fills column")
	}
	sizeVals, _ := size.I64()
	totalVals, _ := total.F64()
	fillVals, _ := fills.I64()
	if len(sizeVals) != 3 || sizeVals[0] != 5 || sizeVals[1] != 10 || sizeVals[2] != 20 {
		t.Fatalf("lowered size values = %#v, want [5 10 20]", sizeVals)
	}
	if len(totalVals) != 3 || totalVals[0] != 99 || totalVals[1] != 100.5 || totalVals[2] != 101.25 {
		t.Fatalf("lowered total values = %#v, want [99 100.5 101.25]", totalVals)
	}
	if len(fillVals) != 3 || fillVals[0] != 1 || fillVals[1] != 1 || fillVals[2] != 1 {
		t.Fatalf("lowered fills values = %#v, want [1 1 1]", fillVals)
	}
}

func TestQGroupAggregateComputedExpressionStaysOnFallback(t *testing.T) {
	const query = "select notional:sum price*size by size from trades"
	proto := &vm.FuncProto{
		Name:      "q_group_aggregate_computed_call",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("q"),
			runtime.StringValue("sql"),
			runtime.StringValue(query),
		},
		Code: []uint32{
			vm.EncodeABx(vm.OP_GETGLOBAL, 1, 0),
			vm.EncodeABC(vm.OP_GETFIELD, 1, 1, 1),
			vm.EncodeABC(vm.OP_MOVE, 2, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 3, 2),
			vm.EncodeABC(vm.OP_CALL, 1, 3, 2),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	counts := countOps(lowered)
	if counts[OpFrameGroupAggregate] != 0 || counts[OpCall] != 1 {
		t.Fatalf("computed group aggregate lowering counts FrameGroupAggregate=%d OpCall=%d\n%s", counts[OpFrameGroupAggregate], counts[OpCall], Print(lowered))
	}
	if fallbacks := CountQQueryLoweringFallbackReasons(fn.Remarks.List()); fallbacks[qQueryLoweringFallbackGroupAggregateCall] != 1 {
		t.Fatalf("q query fallback counts = %+v, want group_aggregate_call=1", fallbacks)
	}
	assertQLoweringRemarkFields(t, fn.Remarks.List(), "QQueryNativeLowering", "QGroupAggregate", "select/group/aggregate", qQueryLoweringFallbackGroupAggregateCall)
}

func TestQGroupAggregateCallReportsJoinSelectOrderShape(t *testing.T) {
	const query = "select total:sum amount by acct from trades join accounts on acct where amount>0 order by acct asc"
	proto := &vm.FuncProto{
		Name:      "q_join_group_aggregate_call",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("q"),
			runtime.StringValue("sql"),
			runtime.StringValue(query),
		},
		Code: []uint32{
			vm.EncodeABx(vm.OP_GETGLOBAL, 1, 0),
			vm.EncodeABC(vm.OP_GETFIELD, 1, 1, 1),
			vm.EncodeABC(vm.OP_MOVE, 2, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 3, 2),
			vm.EncodeABC(vm.OP_CALL, 1, 3, 2),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	if _, err := QQueryNativeLoweringPass(fn); err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}

	fallbacks := CountQQueryLoweringFallbackReasons(fn.Remarks.List())
	if fallbacks[qQueryLoweringFallbackGroupAggregateCall] != 1 {
		t.Fatalf("q query fallback counts = %+v, want group_aggregate_call=1", fallbacks)
	}
	descriptors := BuildQKernelDescriptors(nil, nil, nil, fn.Remarks.List())
	assertQKernelDescriptor(t, descriptors, "methodjit_q_query_lowering", "fallback", "QGroupAggregate", "select/join/where/group/aggregate/order", "lowering", "fallback", qQueryLoweringFallbackGroupAggregateCall)
	summary := BuildQKernelShapeSummaryFromDescriptors(descriptors)
	assertQKernelShapeSummary(t, summary, "methodjit_q_query_lowering", "fallback", "select/join/where/group/aggregate/order", "fallback", qQueryLoweringFallbackGroupAggregateCall, 1)
	formatted := formatOptimizationRemarks(fn.Remarks.List())
	if !strings.Contains(formatted, "shape=select/join/where/group/aggregate/order") {
		t.Fatalf("join group aggregate fallback remark missing stable shape:\n%s", formatted)
	}
}

func TestQGroupAggregateFallbackDoesNotMatchPlainQSelect(t *testing.T) {
	const query = "select sym,price from trades where price>100"
	proto := &vm.FuncProto{
		Name:      "q_plain_select_call",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("q"),
			runtime.StringValue("select"),
			runtime.StringValue(query),
		},
		Code: []uint32{
			vm.EncodeABx(vm.OP_GETGLOBAL, 1, 0),
			vm.EncodeABC(vm.OP_GETFIELD, 1, 1, 1),
			vm.EncodeABC(vm.OP_MOVE, 2, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 3, 2),
			vm.EncodeABC(vm.OP_CALL, 1, 3, 2),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	if _, err := QQueryNativeLoweringPass(fn); err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	if fallbacks := CountQQueryLoweringFallbackReasons(fn.Remarks.List()); len(fallbacks) != 0 {
		t.Fatalf("plain q.select fallback counts = %+v, want none", fallbacks)
	}
	if descriptors := BuildQKernelDescriptors(nil, nil, nil, fn.Remarks.List()); len(descriptors) != 0 {
		t.Fatalf("plain q.select descriptors = %+v, want none", descriptors)
	}
}

func TestQJoinCallReportsStructuredFallback(t *testing.T) {
	const query = "select id,value,qty from accounts left join fills on id=account_id,venue=exchange where value>0 order by qty desc"
	proto := &vm.FuncProto{
		Name:      "q_join_call",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("q"),
			runtime.StringValue("sql"),
			runtime.StringValue(query),
		},
		Code: []uint32{
			vm.EncodeABx(vm.OP_GETGLOBAL, 1, 0),
			vm.EncodeABC(vm.OP_GETFIELD, 1, 1, 1),
			vm.EncodeABC(vm.OP_MOVE, 2, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 3, 2),
			vm.EncodeABC(vm.OP_CALL, 1, 3, 2),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	lowered, err := QQueryNativeLoweringPass(fn)
	if err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	if counts := countOps(lowered); counts[OpCall] != 1 {
		t.Fatalf("join call op count = %d, want opaque OpCall to remain\n%s", counts[OpCall], Print(lowered))
	}
	fallbacks := CountQQueryLoweringFallbackReasons(fn.Remarks.List())
	if fallbacks[qQueryLoweringFallbackJoinCall] != 1 {
		t.Fatalf("q query fallback counts = %+v, want join_call=1", fallbacks)
	}
	assertQLoweringRemarkFields(t, fn.Remarks.List(), "QQueryNativeLowering", "QJoin", "where/join/left/order", qQueryLoweringFallbackJoinCall)
	descriptors := BuildQKernelDescriptors(nil, nil, nil, fn.Remarks.List())
	assertQKernelDescriptor(t, descriptors, "methodjit_q_query_lowering", "fallback", "QJoin", "where/join/left/order", "lowering", "fallback", qQueryLoweringFallbackJoinCall)
	summary := BuildQKernelShapeSummaryFromDescriptors(descriptors)
	assertQKernelShapeSummary(t, summary, "methodjit_q_query_lowering", "fallback", "where/join/left/order", "fallback", qQueryLoweringFallbackJoinCall, 1)
	formatted := formatOptimizationRemarks(fn.Remarks.List())
	if !strings.Contains(formatted, "kernel=QJoin") ||
		!strings.Contains(formatted, "reason_code=join_call") ||
		!strings.Contains(formatted, "shape=where/join/left/order") {
		t.Fatalf("join fallback remark missing stable taxonomy:\n%s", formatted)
	}
}

func TestQJoinFallbackClassifiesDigitAlias(t *testing.T) {
	const query = "select sym,ts,bid from trades wj1[-5,0] quotes on sym,ts"
	proto := &vm.FuncProto{
		Name:      "q_window1_join_call",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("q"),
			runtime.StringValue("select"),
			runtime.StringValue(query),
		},
		Code: []uint32{
			vm.EncodeABx(vm.OP_GETGLOBAL, 1, 0),
			vm.EncodeABC(vm.OP_GETFIELD, 1, 1, 1),
			vm.EncodeABC(vm.OP_MOVE, 2, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 3, 2),
			vm.EncodeABC(vm.OP_CALL, 1, 3, 2),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	fn.Remarks = &OptimizationRemarks{}
	if _, err := QQueryNativeLoweringPass(fn); err != nil {
		t.Fatalf("QQueryNativeLoweringPass: %v", err)
	}
	if fallbacks := CountQQueryLoweringFallbackReasons(fn.Remarks.List()); fallbacks[qQueryLoweringFallbackJoinCall] != 1 {
		t.Fatalf("q query fallback counts = %+v, want join_call=1", fallbacks)
	}
	descriptors := BuildQKernelDescriptors(nil, nil, nil, fn.Remarks.List())
	assertQKernelDescriptor(t, descriptors, "methodjit_q_query_lowering", "fallback", "QJoin", "join/window1", "lowering", "fallback", qQueryLoweringFallbackJoinCall)
}

func TestQVectorRuntimeKernelsDiagnosePrimitiveShapes(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_vector_runtime_kernel_shapes",
		NumParams: 5,
		MaxStack:  5,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_GATHER, 0, 1, 0),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 0, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_VECTOR_MASK, 0, 3, int(runtime.DenseArrayMaskAnd)),
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 4, 0, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 0, 5, 0),
		},
	}
	args := []runtime.Value{
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30})),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1, 3})),
		runtime.IntValue(15),
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, true})),
		runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{1, 2, 3})),
	}

	fn := BuildGraph(proto)
	kernels := DetectQVectorRuntimeKernels(fn)
	if len(kernels) != 4 {
		t.Fatalf("DetectQVectorRuntimeKernels count = %d, want 4\n%s", len(kernels), Print(fn))
	}
	counts := CountQVectorRuntimeKernelShapes(kernels)
	for _, shape := range []string{"vector-gather", "vector-compare", "vector-mask", "vector/vector-reduce"} {
		if counts[shape] != 1 {
			t.Fatalf("vector runtime kernel shape counts = %+v, want %s count 1", counts, shape)
		}
	}

	report := Diagnose(proto, args)
	if len(report.QVectorRuntimeKernels) != 3 {
		t.Fatalf("Diagnose QVectorRuntimeKernels = %d, want 3 live kernels\n%s", len(report.QVectorRuntimeKernels), report.String())
	}
	for _, shape := range []string{"vector-gather", "vector-compare", "vector-mask"} {
		if report.QVectorRuntimeKernelShapes[shape] != 1 {
			t.Fatalf("Diagnose QVectorRuntimeKernelShapes = %+v, want %s count 1", report.QVectorRuntimeKernelShapes, shape)
		}
	}
	assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_vector_runtime", "VectorGather", "vector-gather", "typed_runtime_op_exit", "success", 1)
	assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_vector_runtime", "VectorCompare", "vector-compare", "typed_runtime_op_exit", "success", 1)
	assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_vector_runtime", "VectorMask", "vector-mask", "typed_runtime_op_exit", "success", 1)
	assertQKernelExecutionRouteSummary(t, report.QKernelExecutionRoutes, "methodjit_q_vector_runtime", "VectorGather", "typed_runtime_op_exit", "success", 1)
	assertQKernelShapeSummaryExecution(t, report.QKernelShapeSummary, "methodjit_q_vector_runtime", "runtime_kernel", "vector-gather", "supported", 1, 1, 0)
	if !strings.Contains(report.String(), "kernel=VectorGather") ||
		!strings.Contains(report.String(), "kernel=VectorCompare op=>=") ||
		!strings.Contains(report.String(), "kernel=VectorMask op=and") ||
		!strings.Contains(report.String(), "source=methodjit_q_vector_runtime kernel=VectorGather shape=vector-gather route=typed_runtime_op_exit outcome=success count=1") {
		t.Fatalf("diagnostic report missing vector runtime kernel details:\n%s", report.String())
	}
}

func TestQVectorRuntimeKernelsDiagnoseVectorScan(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_vector_scan_kernel_shape",
		NumParams: 1,
		MaxStack:  1,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_SCAN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	report := Diagnose(proto, []runtime.Value{
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{2, -1, 4})),
	})
	if len(report.QVectorRuntimeKernels) != 1 {
		t.Fatalf("Diagnose QVectorRuntimeKernels = %d, want one scan kernel\n%s", len(report.QVectorRuntimeKernels), report.String())
	}
	if report.QVectorRuntimeKernelShapes["vector-scan"] != 1 {
		t.Fatalf("Diagnose QVectorRuntimeKernelShapes = %+v, want vector-scan count 1", report.QVectorRuntimeKernelShapes)
	}
	if !strings.Contains(report.String(), "kernel=VectorScan") ||
		!strings.Contains(report.String(), "shapes: vector-scan=1") {
		t.Fatalf("diagnostic report missing vector scan kernel details:\n%s", report.String())
	}
}

func TestQVectorRuntimePrimitiveDiagnoseExecutionStats(t *testing.T) {
	tests := []struct {
		name   string
		kernel string
		shape  string
		code   []uint32
		args   []runtime.Value
	}{
		{
			name:   "gather",
			kernel: "VectorGather",
			shape:  "vector-gather",
			code: []uint32{
				vm.EncodeABC(vm.OP_VECTOR_GATHER, 0, 1, 0),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: []runtime.Value{
				runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30})),
				runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1})),
			},
		},
		{
			name:   "compare",
			kernel: "VectorCompare",
			shape:  "vector-compare",
			code: []uint32{
				vm.EncodeABC(vm.OP_VECTOR_COMPARE, 0, 1, int(runtime.DenseArrayGE)),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: []runtime.Value{
				runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1, 4, 6})),
				runtime.IntValue(4),
			},
		},
		{
			name:   "mask",
			kernel: "VectorMask",
			shape:  "vector-mask",
			code: []uint32{
				vm.EncodeABC(vm.OP_VECTOR_MASK, 0, 1, int(runtime.DenseArrayMaskAnd)),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: []runtime.Value{
				runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true})),
				runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, true, false})),
			},
		},
		{
			name:   "where",
			kernel: "VectorWhere",
			shape:  "vector-where",
			code: []uint32{
				vm.EncodeABC(vm.OP_VECTOR_WHERE, 0, 1, 2),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: []runtime.Value{
				runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true})),
				runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30})),
				runtime.IntValue(7),
			},
		},
		{
			name:   "reduce",
			kernel: "VectorReduce",
			shape:  "vector/vector-reduce",
			code: []uint32{
				vm.EncodeABC(vm.OP_VECTOR_REDUCE, 0, 0, int(runtime.DenseArrayReduceSum)),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: []runtime.Value{
				runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1, 2, 3})),
			},
		},
		{
			name:   "scan",
			kernel: "VectorScan",
			shape:  "vector-scan",
			code: []uint32{
				vm.EncodeABC(vm.OP_VECTOR_SCAN, 0, 0, 0),
				vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
			},
			args: []runtime.Value{
				runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{2, -1, 4})),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proto := &vm.FuncProto{
				Name:      "q_vector_runtime_primitive_" + tt.name,
				NumParams: len(tt.args),
				MaxStack:  3,
				Code:      tt.code,
			}
			report := Diagnose(proto, tt.args)
			if report.NativeError != nil || report.InterpError != nil || report.OptInterpError != nil {
				t.Fatalf("Diagnose %s errors: native=%v interp=%v opt=%v\n%s",
					tt.name, report.NativeError, report.InterpError, report.OptInterpError, report.String())
			}
			assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_vector_runtime", "runtime_kernel", tt.kernel, tt.shape, "typed_runtime_op_exit", "supported", "")
			assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_vector_runtime", tt.kernel, tt.shape, "typed_runtime_op_exit", "success", 1)
			assertQKernelExecutionRouteSummary(t, report.QKernelExecutionRoutes, "methodjit_q_vector_runtime", tt.kernel, "typed_runtime_op_exit", "success", 1)
			assertQKernelShapeSummaryExecution(t, report.QKernelShapeSummary, "methodjit_q_vector_runtime", "runtime_kernel", tt.shape, "supported", 1, 1, 0)
		})
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
	if len(report.QTypedRuntimeKernels) != 1 {
		t.Fatalf("Diagnose QTypedRuntimeKernels = %d, want 1\n%s", len(report.QTypedRuntimeKernels), report.String())
	}
	if report.QTypedRuntimeKernels[0].Shape != "compare/filter/project/column" {
		t.Fatalf("Diagnose QTypedRuntimeKernels[0].Shape = %q, want compare/filter/project/column", report.QTypedRuntimeKernels[0].Shape)
	}
	if report.QTypedRuntimeKernelShapes["compare/filter/project/column"] != 1 {
		t.Fatalf("Diagnose QTypedRuntimeKernelShapes = %+v, want compare/filter/project/column count 1", report.QTypedRuntimeKernelShapes)
	}
	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_frame_runtime", "runtime_kernel", "QFrameSelectColumn", "compare/filter/project/column", "typed_runtime_op_exit", "supported", "")
	assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_frame_runtime", "runtime_kernel", "compare/filter/project/column", "supported", "", 1)
	assertQKernelShapeSummaryExecution(t, report.QKernelShapeSummary, "methodjit_q_frame_runtime", "runtime_kernel", "compare/filter/project/column", "supported", 1, 1, 0)
	assertQKernelExecutionStat(t, report.QKernelExecutionStats, "methodjit_q_frame_runtime", "QFrameSelectColumn", "compare/filter/project/column", "typed_runtime_op_exit", "success", 1)
	assertQKernelExecutionRouteSummary(t, report.QKernelExecutionRoutes, "methodjit_q_frame_runtime", "QFrameSelectColumn", "typed_runtime_op_exit", "success", 1)
	if !strings.Contains(report.String(), "Q query hot paths") {
		t.Fatalf("diagnostic report missing q hot path section:\n%s", report.String())
	}
	if !strings.Contains(report.String(), "Q typed runtime kernels") ||
		!strings.Contains(report.String(), "1 typed runtime kernel(s)") ||
		!strings.Contains(report.String(), "shapes: compare/filter/project/column=1") ||
		!strings.Contains(report.String(), "mask=compare:>=:0") ||
		!strings.Contains(report.String(), "rows=none") {
		t.Fatalf("diagnostic report missing q typed runtime kernel section:\n%s", report.String())
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

func TestDiagnoseReportsQQueryFallbackReasons(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "q_frame_pipeline_fallback_diag",
		NumParams: 3,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
			qHotPathNamesValue("size"),
			runtime.StringValue("size"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 3, 0, 0),
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 3, 2, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_FRAME_FILTER, 0, 0, 3),
			vm.EncodeABC(vm.OP_FRAME_GATHER, 0, 0, 1),
			vm.EncodeABC(vm.OP_FRAME_PROJECT, 0, 0, 1),
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 2),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	report := Diagnose(proto, []runtime.Value{
		runtime.TableValue(qHotPathTestFrame(t)),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{2, 1})),
		runtime.FloatValue(100),
	})
	if report.QQueryFallbacks[qQueryLoweringFallbackTooManyDynamicArgs] != 1 {
		t.Fatalf("Diagnose QQueryFallbacks = %+v, want %s count 1\n%s", report.QQueryFallbacks, qQueryLoweringFallbackTooManyDynamicArgs, report.String())
	}
	if len(report.QTypedRuntimeKernels) != 0 {
		t.Fatalf("Diagnose QTypedRuntimeKernels = %d, want 0 for fallback path\n%s", len(report.QTypedRuntimeKernels), report.String())
	}
	if len(report.QQueryHotPaths) != 1 {
		t.Fatalf("Diagnose QQueryHotPaths = %d, want 1 remaining fallback hot path\n%s", len(report.QQueryHotPaths), report.String())
	}
	if !strings.Contains(report.String(), "Q query fallback reasons") ||
		!strings.Contains(report.String(), "too_many_dynamic_args=1") ||
		!strings.Contains(formatOptimizationRemarks(report.OptimizationRemarks), "reason_code=too_many_dynamic_args") {
		t.Fatalf("diagnostic report missing q fallback reason:\n%s", report.String())
	}
}

func TestDiagnoseReportsQGroupAggregateFallbackDescriptor(t *testing.T) {
	const query = "select notional:sum price*size, fills:count i by sym from trades where price>100 order by sym asc"
	proto := &vm.FuncProto{
		Name:      "q_group_aggregate_fallback_diag",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("q"),
			runtime.StringValue("sql"),
			runtime.StringValue(query),
		},
		Code: []uint32{
			vm.EncodeABx(vm.OP_GETGLOBAL, 1, 0),
			vm.EncodeABC(vm.OP_GETFIELD, 1, 1, 1),
			vm.EncodeABC(vm.OP_MOVE, 2, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 3, 2),
			vm.EncodeABC(vm.OP_CALL, 1, 3, 2),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if report.QQueryFallbacks[qQueryLoweringFallbackGroupAggregateCall] != 1 {
		t.Fatalf("Diagnose QQueryFallbacks = %+v, want group_aggregate_call=1\n%s", report.QQueryFallbacks, report.String())
	}
	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_query_lowering", "fallback", "QGroupAggregate", "select/where/group/aggregate/order", "lowering", "fallback", qQueryLoweringFallbackGroupAggregateCall)
	assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_query_lowering", "fallback", "select/where/group/aggregate/order", "fallback", qQueryLoweringFallbackGroupAggregateCall, 1)
	if len(report.QKernelExecutionStats) != 0 || len(report.QKernelExecutionRoutes) != 0 {
		t.Fatalf("Diagnose group fallback execution stats/routes = %+v/%+v, want none\n%s", report.QKernelExecutionStats, report.QKernelExecutionRoutes, report.String())
	}
	if !strings.Contains(report.String(), "kernel=QGroupAggregate") ||
		!strings.Contains(report.String(), "reason_code=group_aggregate_call") ||
		!strings.Contains(report.String(), "shape=select/where/group/aggregate/order") {
		t.Fatalf("diagnostic report missing group fallback descriptor:\n%s", report.String())
	}
}

func TestDiagnoseReportsQJoinFallbackDescriptor(t *testing.T) {
	const query = "select id,value,qty from accounts left join fills on id=account_id,venue=exchange where value>0 order by qty desc"
	proto := &vm.FuncProto{
		Name:      "q_join_fallback_diag",
		NumParams: 1,
		MaxStack:  4,
		Constants: []runtime.Value{
			runtime.StringValue("q"),
			runtime.StringValue("sql"),
			runtime.StringValue(query),
		},
		Code: []uint32{
			vm.EncodeABx(vm.OP_GETGLOBAL, 1, 0),
			vm.EncodeABC(vm.OP_GETFIELD, 1, 1, 1),
			vm.EncodeABC(vm.OP_MOVE, 2, 0, 0),
			vm.EncodeABx(vm.OP_LOADK, 3, 2),
			vm.EncodeABC(vm.OP_CALL, 1, 3, 2),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	report := Diagnose(proto, []runtime.Value{runtime.TableValue(qHotPathTestFrame(t))})
	if report.QQueryFallbacks[qQueryLoweringFallbackJoinCall] != 1 {
		t.Fatalf("Diagnose QQueryFallbacks = %+v, want join_call=1\n%s", report.QQueryFallbacks, report.String())
	}
	assertQKernelDescriptor(t, report.QKernelDescriptors, "methodjit_q_query_lowering", "fallback", "QJoin", "where/join/left/order", "lowering", "fallback", qQueryLoweringFallbackJoinCall)
	assertQKernelShapeSummary(t, report.QKernelShapeSummary, "methodjit_q_query_lowering", "fallback", "where/join/left/order", "fallback", qQueryLoweringFallbackJoinCall, 1)
	if len(report.QKernelExecutionStats) != 0 || len(report.QKernelExecutionRoutes) != 0 {
		t.Fatalf("Diagnose join fallback execution stats/routes = %+v/%+v, want none\n%s", report.QKernelExecutionStats, report.QKernelExecutionRoutes, report.String())
	}
	if !strings.Contains(report.String(), "kernel=QJoin") ||
		!strings.Contains(report.String(), "reason_code=join_call") ||
		!strings.Contains(report.String(), "shape=where/join/left/order") {
		t.Fatalf("diagnostic report missing join fallback descriptor:\n%s", report.String())
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

func TestTier2GateAllowsFrameFilterProjectThroughOpExit(t *testing.T) {
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("price"))
	proto := &vm.FuncProto{
		Name:      "frame_filter_project",
		NumParams: 2,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.TableValue(names),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_FILTER_PROJECT, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_FILTER_PROJECT should be Tier2-eligible via op-exit, got %q", gate.Reason)
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

func TestTier2GateAllowsFrameOrderGatherThroughOpExit(t *testing.T) {
	order := runtime.NewTable()
	order.RawSetString("column", runtime.StringValue("price"))
	proto := &vm.FuncProto{
		Name:      "frame_order_gather",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			runtime.TableValue(order),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_ORDER_GATHER, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_ORDER_GATHER should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsFrameGroupAggregateThroughOpExit(t *testing.T) {
	spec := qFrameGroupAggregateSpec("size", []runtime.FrameAggregateSpec{
		{Name: "n", Op: "count"},
		{Name: "total", Op: "sum", Column: "price"},
	})
	proto := &vm.FuncProto{
		Name:      "frame_group_aggregate",
		NumParams: 1,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.TableValue(spec),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_LOADNIL, 1, 0, 0),
			vm.EncodeABC(vm.OP_FRAME_GROUP_AGGREGATE, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_GROUP_AGGREGATE should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsFrameProjectColumnThroughOpExit(t *testing.T) {
	spec := runtime.NewTable()
	spec.RawSetString("project", runtime.StringValue("price"))
	spec.RawSetString("column", runtime.StringValue("price"))
	proto := &vm.FuncProto{
		Name:      "frame_project_column",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			runtime.TableValue(spec),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_PROJECT_COLUMN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_PROJECT_COLUMN should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsFrameFilterProjectColumnThroughOpExit(t *testing.T) {
	spec := runtime.NewTable()
	spec.RawSetString("project", runtime.StringValue("price"))
	spec.RawSetString("column", runtime.StringValue("price"))
	proto := &vm.FuncProto{
		Name:      "frame_filter_project_column",
		NumParams: 2,
		MaxStack:  2,
		Constants: []runtime.Value{
			runtime.TableValue(spec),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_FILTER_PROJECT_COLUMN, 1, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 1, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_FILTER_PROJECT_COLUMN should be Tier2-eligible via op-exit, got %q", gate.Reason)
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

func TestTier2GateAllowsVectorMaskThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_mask",
		MaxStack: 2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_MASK, 0, 1, int(runtime.DenseArrayMaskAnd)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_VECTOR_MASK should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsVectorWhereThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_where",
		MaxStack: 3,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_WHERE, 0, 1, 2),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_VECTOR_WHERE should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsVectorReduceThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_reduce",
		MaxStack: 1,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_REDUCE, 0, 0, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_VECTOR_REDUCE should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsVectorWhereReduceThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_where_reduce",
		MaxStack: 3,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_WHERE_REDUCE, 0, 1, int(runtime.DenseArrayReduceSum)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_VECTOR_WHERE_REDUCE should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsVectorScanThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_scan",
		MaxStack: 1,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_SCAN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_VECTOR_SCAN should be Tier2-eligible via op-exit, got %q", gate.Reason)
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

func TestFrameFilterProjectRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{10.5, 20.25, 30.75}),
		"size":  runtime.NewDenseArrayI64([]int64{100, 200, 300}),
		"flag":  runtime.NewDenseArrayBool([]bool{true, false, true}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:    runtime.NativePayloadDataFrame,
		Rows:    soa.Len(),
		Columns: 3,
	})

	result, err := executeFrameFilterProjectValue(
		runtime.TableValue(frame),
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true})),
		[]string{"size", "price"},
	)
	if err != nil {
		t.Fatalf("execute frame filter project: %v", err)
	}
	if !result.IsFrame() {
		t.Fatalf("frame filter project result type = %s, want frame", result.TypeName())
	}
	if _, err := executeFrameColumnValue(result, "flag"); err == nil {
		t.Fatalf("filter project kept unprojected flag column")
	}
	col, err := executeFrameColumnValue(result, "size")
	if err != nil {
		t.Fatalf("filter projected frame column: %v", err)
	}
	got, ok := col.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 100 || got[1] != 300 {
		t.Fatalf("filter project size values = %#v, want [100 300]", got)
	}
}

func TestFrameProjectColumnRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
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

	result, err := executeFrameProjectColumnValue(runtime.TableValue(frame), []string{"size"}, "size")
	if err != nil {
		t.Fatalf("execute frame project column: %v", err)
	}
	got, ok := result.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 100 || got[1] != 200 {
		t.Fatalf("project column values = %#v, want [100 200]", got)
	}
	if _, err := executeFrameProjectColumnValue(runtime.TableValue(frame), []string{"price"}, "size"); err == nil {
		t.Fatalf("execute frame project column accepted unprojected result")
	}
}

func TestFrameFilterProjectColumnRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
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

	result, err := executeFrameFilterProjectColumnValue(
		runtime.TableValue(frame),
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true})),
		[]string{"size"},
		"size",
	)
	if err != nil {
		t.Fatalf("execute frame filter project column: %v", err)
	}
	got, ok := result.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 100 || got[1] != 300 {
		t.Fatalf("filter project column values = %#v, want [100 300]", got)
	}
	if _, err := executeFrameFilterProjectColumnValue(
		runtime.TableValue(frame),
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true})),
		[]string{"price"},
		"size",
	); err == nil {
		t.Fatalf("execute frame filter project column accepted unprojected result")
	}
}

func TestFrameCompareFilterProjectColumnRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{10.5, 20.25, 30.75}),
		"limit": runtime.NewDenseArrayF64([]float64{9, 21, 30}),
		"size":  runtime.NewDenseArrayI64([]int64{100, 200, 300}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:    runtime.NativePayloadDataFrame,
		Rows:    soa.Len(),
		Columns: 3,
	})

	result, err := executeFrameCompareFilterProjectColumnValue(
		runtime.TableValue(frame),
		"price",
		runtime.DenseArrayGE,
		runtime.StringValue("limit"),
		[]string{"size"},
		"size",
	)
	if err != nil {
		t.Fatalf("execute frame compare filter project column: %v", err)
	}
	got, ok := result.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 100 || got[1] != 300 {
		t.Fatalf("compare filter project column values = %#v, want [100 300]", got)
	}
	if _, err := executeFrameCompareFilterProjectColumnValue(
		runtime.TableValue(frame),
		"price",
		runtime.DenseArrayAdd,
		runtime.FloatValue(20),
		[]string{"size"},
		"size",
	); err == nil {
		t.Fatalf("execute frame compare filter project column accepted arithmetic op")
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

func TestFrameOrderGatherRuntimeHelperUsesRuntimePrimitives(t *testing.T) {
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

	result, err := executeFrameOrderGatherValue(runtime.TableValue(frame), runtime.TableValue(order))
	if err != nil {
		t.Fatalf("execute frame order gather: %v", err)
	}
	if !result.IsFrame() {
		t.Fatalf("frame order gather result type = %s, want frame", result.TypeName())
	}
	col, err := executeFrameColumnValue(result, "size")
	if err != nil {
		t.Fatalf("ordered/gathered frame column: %v", err)
	}
	got, ok := col.DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 300 || got[1] != 200 {
		t.Fatalf("ordered/gathered size values = %#v, want [300 200]", got)
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

func TestVectorMaskRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	result, err := executeVectorMaskValue(
		int(runtime.DenseArrayMaskAndNot),
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true, false})),
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{false, true, true, false})),
	)
	if err != nil {
		t.Fatalf("execute vector mask: %v", err)
	}
	if !result.IsDenseArray() {
		t.Fatalf("vector mask result = %#v, want dense array", result)
	}
	got, ok := result.DenseArray().Bool()
	if !ok || len(got) != 4 || !got[0] || got[1] || got[2] || got[3] {
		t.Fatalf("vector mask values = %#v, want [true false false false]", got)
	}
}

func TestVectorWhereRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	result, err := executeVectorWhereValue(
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true})),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30})),
		runtime.IntValue(7),
	)
	if err != nil {
		t.Fatalf("execute vector where: %v", err)
	}
	if !result.IsDenseArray() {
		t.Fatalf("vector where result = %#v, want dense array", result)
	}
	got, ok := result.DenseArray().I64()
	if !ok || len(got) != 3 || got[0] != 10 || got[1] != 7 || got[2] != 30 {
		t.Fatalf("vector where values = %#v, want [10 7 30]", got)
	}
}

func TestVectorReduceRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	result, err := executeVectorReduceValue(
		int(runtime.DenseArrayReduceMax),
		runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{1.5, 6.25, 2.0})),
	)
	if err != nil {
		t.Fatalf("execute vector reduce: %v", err)
	}
	if !result.IsFloat() || result.Float() != 6.25 {
		t.Fatalf("vector reduce result = %#v, want float 6.25", result)
	}
}

func TestQVectorWhereReduceRuntimeHelperUsesRuntimePrimitives(t *testing.T) {
	result, err := executeQVectorWhereReduceValue(
		int(runtime.DenseArrayReduceSum),
		runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true})),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30})),
		runtime.IntValue(7),
	)
	if err != nil {
		t.Fatalf("execute q vector where-reduce: %v", err)
	}
	if !result.IsInt() || result.Int() != 47 {
		t.Fatalf("q vector where-reduce result = %#v, want int 47", result)
	}
}

func TestQVectorGatherReduceRuntimeHelperUsesRuntimePrimitives(t *testing.T) {
	result, err := executeQVectorGatherReduceValue(
		int(runtime.DenseArrayReduceSum),
		runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{10, 20.25, 30})),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1})),
	)
	if err != nil {
		t.Fatalf("execute q vector gather-reduce: %v", err)
	}
	if !result.IsFloat() || result.Float() != 40 {
		t.Fatalf("q vector gather-reduce result = %#v, want float 40", result)
	}
}

func TestVectorScanRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	result, err := executeVectorScanValue(
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{2, -1, 4})),
	)
	if err != nil {
		t.Fatalf("execute vector scan: %v", err)
	}
	if !result.IsDenseArray() {
		t.Fatalf("vector scan result = %#v, want dense array", result)
	}
	got, ok := result.DenseArray().I64()
	if !ok || len(got) != 3 || got[0] != 2 || got[1] != 1 || got[2] != 5 {
		t.Fatalf("vector scan values = %#v, want [2 1 5]", got)
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

func qFrameGroupAggregateSpec(by string, aggs []runtime.FrameAggregateSpec) *runtime.Table {
	spec := runtime.NewTable()
	if by != "" {
		spec.RawSetString("by", runtime.StringValue(by))
	}
	aggRows := runtime.NewAppendArrayTable(len(aggs))
	for i, agg := range aggs {
		row := runtime.NewTable()
		row.RawSetString("name", runtime.StringValue(agg.Name))
		row.RawSetString("op", runtime.StringValue(agg.Op))
		if agg.Column != "" {
			row.RawSetString("column", runtime.StringValue(agg.Column))
		}
		aggRows.RawSetInt(int64(i+1), runtime.TableValue(row))
	}
	spec.RawSetString("aggregates", runtime.TableValue(aggRows))
	return spec
}

func assertQKernelShapeSummary(t *testing.T, rows []QKernelShapeSummary, source, kind, shape, outcome, reasonCode string, count int) {
	t.Helper()
	for _, row := range rows {
		if row.Source == source && row.Kind == kind && row.Shape == shape && row.Outcome == outcome && row.ReasonCode == reasonCode {
			if row.Count != count {
				t.Fatalf("QKernelShapeSummary %s/%s/%s/%s/%s count = %d, want %d; rows=%+v",
					source, kind, shape, outcome, reasonCode, row.Count, count, rows)
			}
			return
		}
	}
	t.Fatalf("QKernelShapeSummary missing source=%s kind=%s shape=%s outcome=%s reason=%s; rows=%+v",
		source, kind, shape, outcome, reasonCode, rows)
}

func assertQKernelShapeSummaryExecution(t *testing.T, rows []QKernelShapeSummary, source, kind, shape, outcome string, executions, successes, errors uint64) {
	t.Helper()
	for _, row := range rows {
		if row.Source == source && row.Kind == kind && row.Shape == shape && row.Outcome == outcome {
			if row.Executions != executions || row.Successes != successes || row.Errors != errors {
				t.Fatalf("QKernelShapeSummary execution %s/%s/%s/%s = %d/%d/%d, want %d/%d/%d; rows=%+v",
					source, kind, shape, outcome,
					row.Executions, row.Successes, row.Errors,
					executions, successes, errors, rows)
			}
			return
		}
	}
	t.Fatalf("QKernelShapeSummary execution missing source=%s kind=%s shape=%s outcome=%s; rows=%+v",
		source, kind, shape, outcome, rows)
}

func assertQKernelDescriptor(t *testing.T, rows []QKernelDescriptor, source, kind, kernel, shape, route, outcome, reasonCode string) {
	t.Helper()
	for _, row := range rows {
		if row.Source == source && row.Kind == kind && row.Kernel == kernel && row.Shape == shape && row.Route == route && row.Outcome == outcome && row.ReasonCode == reasonCode {
			return
		}
	}
	t.Fatalf("QKernelDescriptor missing source=%s kind=%s kernel=%s shape=%s route=%s outcome=%s reason=%s; rows=%+v",
		source, kind, kernel, shape, route, outcome, reasonCode, rows)
}

func assertQLoweringRemarkFields(t *testing.T, rows []OptimizationRemark, pass, kernel, shape, reasonCode string) {
	t.Helper()
	for _, row := range rows {
		if row.Pass != pass || row.Kind != "missed" {
			continue
		}
		if row.Fields["kernel"] == kernel &&
			row.Fields["shape"] == shape &&
			row.Fields["kind"] == "fallback" &&
			row.Fields["route"] == "lowering" &&
			row.Fields["outcome"] == "fallback" &&
			row.Fields["reason_family"] == "lowering" &&
			row.Fields["reason_code"] == reasonCode {
			return
		}
	}
	t.Fatalf("OptimizationRemark fields missing pass=%s kernel=%s shape=%s reason=%s; rows=%+v", pass, kernel, shape, reasonCode, rows)
}

func assertQKernelExecutionStat(t *testing.T, rows []QKernelExecutionStat, source, kernel, shape, route, outcome string, count uint64) {
	t.Helper()
	for _, row := range rows {
		if row.Source == source && row.Kernel == kernel && row.Shape == shape && row.Route == route && row.Outcome == outcome {
			if row.Count != count {
				t.Fatalf("QKernelExecutionStat %s/%s/%s/%s/%s count = %d, want %d; rows=%+v",
					source, kernel, shape, route, outcome, row.Count, count, rows)
			}
			return
		}
	}
	t.Fatalf("QKernelExecutionStat missing source=%s kernel=%s shape=%s route=%s outcome=%s; rows=%+v",
		source, kernel, shape, route, outcome, rows)
}

func assertQKernelExecutionRouteSummary(t *testing.T, rows []QKernelExecutionRouteSummary, source, kernel, route, outcome string, count uint64) {
	t.Helper()
	for _, row := range rows {
		if row.Source == source && row.Kernel == kernel && row.Route == route && row.Outcome == outcome {
			if row.Count != count {
				t.Fatalf("QKernelExecutionRouteSummary %s/%s/%s/%s count = %d, want %d; rows=%+v",
					source, kernel, route, outcome, row.Count, count, rows)
			}
			return
		}
	}
	t.Fatalf("QKernelExecutionRouteSummary missing source=%s kernel=%s route=%s outcome=%s; rows=%+v",
		source, kernel, route, outcome, rows)
}

func assertQKernelJSONRows(t *testing.T, descriptors []QKernelDescriptor, stats []QKernelExecutionStat, routes []QKernelExecutionRouteSummary, summaries []QKernelShapeSummary) {
	t.Helper()
	descriptorJSON, err := json.Marshal(QKernelDescriptorJSONRows(descriptors))
	if err != nil {
		t.Fatalf("marshal QKernelDescriptorJSONRows: %v", err)
	}
	descriptorText := string(descriptorJSON)
	if !strings.Contains(descriptorText, `"source":"methodjit_q_vector_runtime"`) ||
		!strings.Contains(descriptorText, `"route":"typed_runtime_op_exit"`) ||
		!strings.Contains(descriptorText, `"outcome":"supported"`) {
		t.Fatalf("descriptor JSON rows missing stable keys:\n%s", descriptorText)
	}
	if strings.Contains(descriptorText, `"Source"`) ||
		strings.Contains(descriptorText, `"ReasonCode"`) ||
		strings.Contains(descriptorText, `"reason_code"`) {
		t.Fatalf("descriptor JSON rows leaked Go field names:\n%s", descriptorText)
	}

	statJSON, err := json.Marshal(QKernelExecutionStatJSONRows(stats))
	if err != nil {
		t.Fatalf("marshal QKernelExecutionStatJSONRows: %v", err)
	}
	statText := string(statJSON)
	if !strings.Contains(statText, `"shape":"compare/vector-where/vector-reduce"`) ||
		!strings.Contains(statText, `"route":"typed_runtime_op_exit"`) ||
		!strings.Contains(statText, `"outcome":"success"`) ||
		!strings.Contains(statText, `"count":1`) {
		t.Fatalf("execution stat JSON rows missing stable keys:\n%s", statText)
	}
	if strings.Contains(statText, `"Shape"`) ||
		strings.Contains(statText, `"Route"`) ||
		strings.Contains(statText, `"Outcome"`) {
		t.Fatalf("execution stat JSON rows leaked Go field names:\n%s", statText)
	}

	routeJSON, err := json.Marshal(QKernelExecutionRouteSummaryJSONRows(routes))
	if err != nil {
		t.Fatalf("marshal QKernelExecutionRouteSummaryJSONRows: %v", err)
	}
	routeText := string(routeJSON)
	if !strings.Contains(routeText, `"route":"typed_runtime_op_exit"`) ||
		!strings.Contains(routeText, `"outcome":"success"`) ||
		!strings.Contains(routeText, `"count":1`) {
		t.Fatalf("route summary JSON rows missing stable keys:\n%s", routeText)
	}
	if strings.Contains(routeText, `"Route"`) ||
		strings.Contains(routeText, `"Outcome"`) {
		t.Fatalf("route summary JSON rows leaked Go field names:\n%s", routeText)
	}

	summaryJSON, err := json.Marshal(QKernelShapeSummaryJSONRows(summaries))
	if err != nil {
		t.Fatalf("marshal QKernelShapeSummaryJSONRows: %v", err)
	}
	summaryText := string(summaryJSON)
	if !strings.Contains(summaryText, `"kind":"runtime_kernel"`) ||
		!strings.Contains(summaryText, `"shape":"compare/vector-where/vector-reduce"`) ||
		!strings.Contains(summaryText, `"outcome":"supported"`) ||
		!strings.Contains(summaryText, `"executions":1`) ||
		!strings.Contains(summaryText, `"successes":1`) {
		t.Fatalf("shape summary JSON rows missing stable keys:\n%s", summaryText)
	}
	if strings.Contains(summaryText, `"ReasonCode"`) ||
		strings.Contains(summaryText, `"Executions"`) ||
		strings.Contains(summaryText, `"reason_code"`) {
		t.Fatalf("shape summary JSON rows leaked Go field names or non-empty optional fields:\n%s", summaryText)
	}
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

func qHotPathMaskValue(column, op string, value runtime.Value) runtime.Value {
	tbl := runtime.NewTable()
	tbl.RawSetString("column", runtime.StringValue(column))
	tbl.RawSetString("op", runtime.StringValue(op))
	tbl.RawSetString("value", value)
	return runtime.TableValue(tbl)
}
