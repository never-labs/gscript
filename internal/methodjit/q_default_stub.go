//go:build !leia_q

package methodjit

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

const (
	qQueryLoweringFallbackUnsupportedSpec         = "unsupported_spec"
	qQueryLoweringFallbackMultiUse                = "multi_use"
	qQueryLoweringFallbackMissingProto            = "missing_proto"
	qQueryLoweringFallbackBadProjectOrResultConst = "bad_project_or_result_const"
	qQueryLoweringFallbackBadSourceColumnConst    = "bad_source_column_const"
	qQueryLoweringFallbackMissingCompareRHS       = "missing_compare_rhs"
	qQueryLoweringFallbackTooManyDynamicArgs      = "too_many_dynamic_args"
	qQueryLoweringFallbackBadMaskSpecConst        = "bad_mask_spec_const"
	qQueryLoweringFallbackMissingPredicate        = "missing_predicate"
	qQueryLoweringFallbackBadOrderConst           = "bad_order_const"
	qQueryLoweringFallbackMissingRowValue         = "missing_row_value"
	qQueryLoweringFallbackOpaqueRowConst          = "opaque_row_const"
	qQueryLoweringFallbackMaskCombineUnsupported  = "mask_combine_unsupported"
	qQueryLoweringFallbackGroupAggregateCall      = "group_aggregate_call"
	qQueryLoweringFallbackJoinCall                = "join_call"
	qEvalHotPlanFallbackDynamicSource             = "dynamic_source"
	qEvalHotPlanFallbackEmptySource               = "empty_source"
	qEvalHotPlanFallbackUnsupportedShape          = "unsupported_shape"
	qEvalHotPlanFallbackHeuristicOnly             = "heuristic_only"
)

const qEvalPipelineTypedRuntimeBackend = "q_pipeline_typed_runtime"

type QQueryHotPath struct {
	SourceColumn *Instr
	Compare      *Instr
	Mask         *Instr
	MaskCombine  *Instr
	Filter       *Instr
	RowGather    *Instr
	RowSlice     *Instr
	RowOrder     *Instr
	Project      *Instr
	ResultColumn *Instr
}

type QVectorWhereHotPath struct {
	SourceColumn *Instr
	Compare      *Instr
	Mask         *Instr
	MaskCombine  *Instr
	TrueColumn   *Instr
	FalseColumn  *Instr
	Where        *Instr
}

type QVectorReduceHotPath struct {
	SourceColumn *Instr
	Gather       *Instr
	Where        *Instr
	Reduce       *Instr
}

type QVectorRuntimeKernel struct {
	Instr     *Instr
	Kernel    string
	ShapeName string
	Detail    string
}

type QFrameRuntimeKernel struct {
	Instr     *Instr
	Kernel    string
	ShapeName string
	Detail    string
}

func (k QVectorRuntimeKernel) Shape() string {
	if k.ShapeName == "" {
		return "unknown"
	}
	return k.ShapeName
}

func (k QFrameRuntimeKernel) Shape() string {
	if k.ShapeName == "" {
		return "unknown"
	}
	return k.ShapeName
}

type QKernelDescriptor struct {
	Source        string
	Kind          string
	Kernel        string
	Shape         string
	PipelineShape string
	Route         string
	Outcome       string
	ReasonFamily  string
	ReasonCode    string
}

type QKernelDescriptorJSONRow struct {
	Source        string `json:"source"`
	Kind          string `json:"kind"`
	Kernel        string `json:"kernel"`
	Shape         string `json:"shape"`
	PipelineShape string `json:"pipeline_shape,omitempty"`
	Route         string `json:"route"`
	Outcome       string `json:"outcome"`
	ReasonFamily  string `json:"reason_family,omitempty"`
	ReasonCode    string `json:"reason_code,omitempty"`
}

type QKernelExecutionStat struct {
	Source        string
	Kernel        string
	Shape         string
	PipelineShape string
	Route         string
	Outcome       string
	ReasonCode    string
	Count         uint64
}

type QKernelDescriptorCacheStat struct {
	Source        string
	Kernel        string
	Shape         string
	PipelineShape string
	Route         string
	SchemaHash    string
	Entries       uint64
	Hits          uint64
	Misses        uint64
	Evictions     uint64
}

type QKernelExecutionStatJSONRow struct {
	Source        string `json:"source"`
	Kernel        string `json:"kernel"`
	Shape         string `json:"shape"`
	PipelineShape string `json:"pipeline_shape,omitempty"`
	Route         string `json:"route"`
	Outcome       string `json:"outcome"`
	ReasonCode    string `json:"reason_code,omitempty"`
	Count         uint64 `json:"count"`
}

type QKernelExecutionRouteSummary struct {
	Source  string
	Kernel  string
	Route   string
	Outcome string
	Count   uint64
}

type QKernelExecutionRouteSummaryJSONRow struct {
	Source  string `json:"source"`
	Kernel  string `json:"kernel"`
	Route   string `json:"route"`
	Outcome string `json:"outcome"`
	Count   uint64 `json:"count"`
}

type QKernelShapeSummary struct {
	Source       string
	Kind         string
	Shape        string
	Outcome      string
	ReasonFamily string
	ReasonCode   string
	Count        int
}

type QEvalPipelineDescriptor struct {
	Kernel                 string
	Shape                  string
	PipelineShape          string
	ShapeFamily            string
	ShapeReducer           string
	ShapeSelector          string
	ShapeTransform         string
	Source                 string
	Backend                string
	Detail                 string
	Kind                   string
	Terminal               string
	AssignmentText         string
	ValueExpr              string
	ValueBinding           string
	IndexExpr              string
	IndexBinding           string
	MaskExpr               string
	MaskBinding            string
	RowValueExpr           string
	RowIndexExpr           string
	ColIndexExpr           string
	CallableExpr           string
	DyadicOp               string
	ScalarExpr             string
	ScalarLeft             bool
	IncludeCount           bool
	SequenceValueExpr      string
	SequenceTransformChain string
	SequenceTransformNames string
	LeftExpr               string
	RightExpr              string
	CompareOp              string
	ComparePrefix          string
	ModExpr                string
	ModulusExpr            string
	ModTargetExpr          string
	ReductionInput         string
}

type QEvalPipelinePlanner interface {
	DescribeQEvalPipeline(source string) (QEvalPipelineDescriptor, bool)
}

type QEvalPipelineBackend interface {
	BackendName() string
	LookupQEvalPipelinePlan(ref QEvalPipelinePlanRef) (QEvalPipelinePlan, bool)
}

type QEvalPipelineExecutor interface {
	ExecuteQEvalPipelinePlan(ref QEvalPipelinePlanRef) (any, bool, error)
}

type QEvalPipelineValueExecutor interface {
	ExecuteQEvalPipelinePlanValue(ref QEvalPipelinePlanRef) (runtime.Value, bool, error)
}

type QEvalPipelinePlan interface {
	Ref() QEvalPipelinePlanRef
}

type QEvalPipelinePlanRef struct {
	ID                     int
	Kernel                 string
	Shape                  string
	PipelineShape          string
	ShapeFamily            string
	ShapeReducer           string
	ShapeSelector          string
	ShapeTransform         string
	Source                 string
	Backend                string
	Kind                   string
	Terminal               string
	AssignmentText         string
	ValueExpr              string
	ValueBinding           string
	IndexExpr              string
	IndexBinding           string
	MaskExpr               string
	MaskBinding            string
	RowValueExpr           string
	RowIndexExpr           string
	ColIndexExpr           string
	CallableExpr           string
	DyadicOp               string
	ScalarExpr             string
	ScalarLeft             bool
	IncludeCount           bool
	SequenceValueExpr      string
	SequenceTransformChain string
	SequenceTransformNames string
	LeftExpr               string
	RightExpr              string
	CompareOp              string
	ComparePrefix          string
	ModExpr                string
	ModulusExpr            string
	ModTargetExpr          string
	ReductionInput         string
}

func (r QEvalPipelinePlanRef) Valid() bool {
	return r.ID >= 0 && r.Kernel != "" && r.Shape != "" && r.Backend != ""
}

type qEvalPipelinePlanExecutionCounters struct {
	nativeSuccess atomic.Uint64
	nativeError   atomic.Uint64
	opSuccess     atomic.Uint64
	opError       atomic.Uint64
	directSuccess atomic.Uint64
	directError   atomic.Uint64
}

type qEvalPipelineExecutionRoute string

const (
	qEvalPipelineExecutionRouteOpExit      qEvalPipelineExecutionRoute = "typed_runtime_op_exit"
	qEvalPipelineExecutionRouteNativeExit  qEvalPipelineExecutionRoute = "typed_runtime_native_exit"
	qEvalPipelineExecutionRouteDirectEntry qEvalPipelineExecutionRoute = "typed_runtime_direct_entry"
)

const (
	qEvalPipelineReasonPlanUnhandled  = "plan_unhandled"
	qEvalPipelineReasonExecutionError = "plan_execution_error"
)

type qRuntimeEvalPipelinePlanner struct{}
type qRuntimeEvalPipelineBackend struct{}
type qEvalPipelinePlanHelper struct{}

type qEvalSessionEvalExecutionCounters struct {
	success        atomic.Uint64
	errors         atomic.Uint64
	plannedSuccess atomic.Uint64
	plannedErrors  atomic.Uint64
}

type qEvalSessionEvalSite struct {
	resumeOff        atomic.Int32
	resumeOffNumeric atomic.Int32
	stats            qEvalSessionEvalExecutionCounters
	kernel           string
	shape            string
	pipelineShape    string
}

func DetectQQueryHotPaths(*Function) []QQueryHotPath                      { return nil }
func DetectQVectorWhereHotPaths(*Function) []QVectorWhereHotPath          { return nil }
func DetectQVectorReduceHotPaths(*Function) []QVectorReduceHotPath        { return nil }
func DetectQVectorRuntimeKernels(*Function) []QVectorRuntimeKernel        { return nil }
func DetectQFrameRuntimeKernels(*Function) []QFrameRuntimeKernel          { return nil }
func CountQQueryHotPathShapes([]QQueryHotPath) map[string]int             { return map[string]int{} }
func CountQVectorWhereHotPathShapes([]QVectorWhereHotPath) map[string]int { return map[string]int{} }
func CountQVectorReduceHotPathShapes([]QVectorReduceHotPath) map[string]int {
	return map[string]int{}
}
func CountQVectorRuntimeKernelShapes([]QVectorRuntimeKernel) map[string]int { return map[string]int{} }
func CountQFrameRuntimeKernelShapes([]QFrameRuntimeKernel) map[string]int   { return map[string]int{} }

func CountQQueryLoweringFallbackReasons(remarks []OptimizationRemark) map[string]int {
	return countQFallbackReasons(remarks, "QQueryNativeLowering")
}

func CountQVectorLoweringFallbackReasons(remarks []OptimizationRemark) map[string]int {
	return countQFallbackReasons(remarks, "QVectorNativeLowering")
}

func countQFallbackReasons(remarks []OptimizationRemark, pass string) map[string]int {
	out := map[string]int{}
	for _, remark := range remarks {
		if remark.Pass != pass || remark.Kind != "missed" {
			continue
		}
		reason := remark.Reason
		if remark.Fields != nil && remark.Fields["reason_code"] != "" {
			reason = remark.Fields["reason_code"]
		}
		if reason != "" {
			out[reason]++
		}
	}
	return out
}

func BuildQKernelDescriptors(vectors []QVectorRuntimeKernel, frames []QFrameRuntimeKernel, typed []QFrameSelectColumnSpec, remarks []OptimizationRemark) []QKernelDescriptor {
	out := make([]QKernelDescriptor, 0, len(vectors)+len(frames)+len(typed)+len(remarks))
	for _, kernel := range vectors {
		out = append(out, QKernelDescriptor{Source: qVectorRuntimeExecutionSource, Kind: "runtime", Kernel: kernel.Kernel, Shape: kernel.ShapeName, Route: "typed_runtime_op_exit", Outcome: "success"})
	}
	for _, kernel := range frames {
		out = append(out, QKernelDescriptor{Source: qFrameRuntimeExecutionSource, Kind: "runtime", Kernel: kernel.Kernel, Shape: kernel.ShapeName, Route: "typed_runtime_op_exit", Outcome: "success"})
	}
	for _, spec := range typed {
		out = append(out, QKernelDescriptor{Source: qFrameRuntimeExecutionSource, Kind: "runtime", Kernel: "QFrameSelectColumn", Shape: qFrameSelectColumnSpecShape(spec), Route: "typed_runtime_op_exit", Outcome: "success"})
	}
	for _, remark := range remarks {
		if remark.Kind != "missed" || remark.Fields == nil {
			continue
		}
		kernel := remark.Fields["kernel"]
		shape := remark.Fields["shape"]
		if kernel == "" || shape == "" {
			continue
		}
		out = append(out, QKernelDescriptor{
			Source:        remark.Fields["source"],
			Kind:          "fallback",
			Kernel:        kernel,
			Shape:         shape,
			PipelineShape: remark.Fields["pipeline_shape"],
			Route:         remark.Fields["route"],
			Outcome:       "fallback",
			ReasonFamily:  remark.Fields["reason_family"],
			ReasonCode:    remark.Fields["reason_code"],
		})
	}
	return out
}

func BuildQKernelShapeSummaryFromDescriptors(rows []QKernelDescriptor) []QKernelShapeSummary {
	counts := map[QKernelShapeSummary]int{}
	for _, row := range rows {
		key := QKernelShapeSummary{Source: row.Source, Kind: row.Kind, Shape: row.Shape, Outcome: row.Outcome, ReasonFamily: row.ReasonFamily, ReasonCode: row.ReasonCode}
		counts[key]++
	}
	return sortedQKernelShapeSummary(counts)
}

func BuildQKernelShapeSummaryFromDescriptorsAndExecutionStats(rows []QKernelDescriptor, stats []QKernelExecutionStat) []QKernelShapeSummary {
	counts := map[QKernelShapeSummary]int{}
	for _, row := range BuildQKernelShapeSummaryFromDescriptors(rows) {
		row.Count = 0
		counts[row]++
	}
	for _, stat := range stats {
		key := QKernelShapeSummary{Source: stat.Source, Kind: "runtime", Shape: stat.Shape, Outcome: stat.Outcome, ReasonCode: stat.ReasonCode}
		counts[key] += int(stat.Count)
	}
	return sortedQKernelShapeSummary(counts)
}

func sortedQKernelShapeSummary(counts map[QKernelShapeSummary]int) []QKernelShapeSummary {
	out := make([]QKernelShapeSummary, 0, len(counts))
	for row, count := range counts {
		row.Count = count
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].Shape != out[j].Shape {
			return out[i].Shape < out[j].Shape
		}
		if out[i].Outcome != out[j].Outcome {
			return out[i].Outcome < out[j].Outcome
		}
		return out[i].ReasonCode < out[j].ReasonCode
	})
	return out
}

func BuildQKernelExecutionRouteSummary(stats []QKernelExecutionStat) []QKernelExecutionRouteSummary {
	counts := map[QKernelExecutionRouteSummary]uint64{}
	for _, stat := range stats {
		key := QKernelExecutionRouteSummary{Source: stat.Source, Kernel: stat.Kernel, Route: stat.Route, Outcome: stat.Outcome}
		counts[key] += stat.Count
	}
	out := make([]QKernelExecutionRouteSummary, 0, len(counts))
	for row, count := range counts {
		row.Count = count
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].Kernel != out[j].Kernel {
			return out[i].Kernel < out[j].Kernel
		}
		if out[i].Route != out[j].Route {
			return out[i].Route < out[j].Route
		}
		return out[i].Outcome < out[j].Outcome
	})
	return out
}

func QKernelDescriptorJSONRows(rows []QKernelDescriptor) []QKernelDescriptorJSONRow {
	out := make([]QKernelDescriptorJSONRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, QKernelDescriptorJSONRow{
			Source: row.Source, Kind: row.Kind, Kernel: row.Kernel, Shape: row.Shape,
			PipelineShape: row.PipelineShape, Route: row.Route, Outcome: row.Outcome,
			ReasonFamily: row.ReasonFamily, ReasonCode: row.ReasonCode,
		})
	}
	return out
}

func QKernelExecutionStatJSONRows(rows []QKernelExecutionStat) []QKernelExecutionStatJSONRow {
	out := make([]QKernelExecutionStatJSONRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, QKernelExecutionStatJSONRow{
			Source: row.Source, Kernel: row.Kernel, Shape: row.Shape, PipelineShape: row.PipelineShape,
			Route: row.Route, Outcome: row.Outcome, ReasonCode: row.ReasonCode, Count: row.Count,
		})
	}
	return out
}

func QKernelExecutionRouteSummaryJSONRows(rows []QKernelExecutionRouteSummary) []QKernelExecutionRouteSummaryJSONRow {
	out := make([]QKernelExecutionRouteSummaryJSONRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, QKernelExecutionRouteSummaryJSONRow{Source: row.Source, Kernel: row.Kernel, Route: row.Route, Outcome: row.Outcome, Count: row.Count})
	}
	return out
}

func formatQQueryHotPaths([]QQueryHotPath) string { return "0 q query hot path(s)\n" }
func formatQVectorWhereHotPaths([]QVectorWhereHotPath) string {
	return "0 q vector conditional hot path(s)\n"
}
func formatQVectorReduceHotPaths([]QVectorReduceHotPath) string {
	return "0 q vector reduce hot path(s)\n"
}
func formatQFrameRuntimeKernelReport([]QFrameRuntimeKernel) string {
	return "0 q frame runtime kernel(s)\n"
}
func formatQFrameSelectColumnSpecs([]QFrameSelectColumnSpec) string {
	return "0 typed runtime kernel(s)\n"
}
func formatQTypedVectorRuntimeKernelReport([]QVectorRuntimeKernel) string {
	return "0 q vector runtime kernel(s)\n"
}
func formatQQueryLoweringFallbackReasons(map[string]int) string {
	return formatQReasonCounts("q query fallback", nil)
}
func formatQVectorLoweringFallbackReasons(map[string]int) string {
	return formatQReasonCounts("q vector fallback", nil)
}
func formatQKernelDescriptors(rows []QKernelDescriptor) string {
	return fmt.Sprintf("%d q kernel descriptor(s)\n", len(rows))
}
func formatQKernelExecutionStats(rows []QKernelExecutionStat) string {
	return fmt.Sprintf("%d q kernel execution stat(s)\n", len(rows))
}
func formatQKernelDescriptorCacheStats(rows []QKernelDescriptorCacheStat) string {
	return fmt.Sprintf("%d q kernel descriptor cache stat(s)\n", len(rows))
}
func formatQKernelExecutionRouteSummary(rows []QKernelExecutionRouteSummary) string {
	return fmt.Sprintf("%d q kernel execution route summary row(s)\n", len(rows))
}
func formatQKernelShapeSummary(rows []QKernelShapeSummary) string {
	return fmt.Sprintf("%d q kernel shape summary row(s)\n", len(rows))
}

func formatQReasonCounts(label string, counts map[string]int) string {
	if len(counts) == 0 {
		return "0 " + label + " reason(s)\n"
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	fmt.Fprintf(&b, "%d %s reason(s)\n", len(keys), label)
	for _, key := range keys {
		fmt.Fprintf(&b, "  %s: %d\n", key, counts[key])
	}
	return b.String()
}

func QQueryHotPathRemarkPass(fn *Function) (*Function, error)      { return fn, nil }
func QEvalHotPlanRemarkPass(fn *Function) (*Function, error)       { return fn, nil }
func QEvalPipelineLoweringPass(fn *Function) (*Function, error)    { return fn, nil }
func QEvalSessionEvalLoweringPass(fn *Function) (*Function, error) { return fn, nil }
func QQueryNativeLoweringPass(fn *Function) (*Function, error)     { return fn, nil }

func CountQFrameSelectColumnSpecShapes(specs []QFrameSelectColumnSpec) map[string]int {
	if len(specs) == 0 {
		return map[string]int{}
	}
	counts := make(map[string]int, len(specs))
	for _, spec := range specs {
		counts[qFrameSelectColumnSpecShape(spec)]++
	}
	return counts
}

func qFrameSelectColumnSpecShape(spec QFrameSelectColumnSpec) string {
	if spec.Shape == "" {
		return "unknown"
	}
	return spec.Shape
}

func protoLoopCallsAreLowerableQSessionEval(*vm.FuncProto) bool      { return false }
func protoLoopCallsAreLowerableQEvalPipelinePlan(*vm.FuncProto) bool { return false }

func (qRuntimeEvalPipelinePlanner) DescribeQEvalPipeline(string) (QEvalPipelineDescriptor, bool) {
	return QEvalPipelineDescriptor{}, false
}

func newQRuntimeEvalPipelineBackend([]QEvalPipelinePlanRef) qRuntimeEvalPipelineBackend {
	return qRuntimeEvalPipelineBackend{}
}

func (qRuntimeEvalPipelineBackend) hasPlans() bool { return false }
func (qRuntimeEvalPipelineBackend) BackendName() string {
	return "methodjit_q_eval_pipeline_disabled"
}
func (qRuntimeEvalPipelineBackend) LookupQEvalPipelinePlan(QEvalPipelinePlanRef) (QEvalPipelinePlan, bool) {
	return nil, false
}
func (qRuntimeEvalPipelineBackend) ExecuteQEvalPipelinePlan(QEvalPipelinePlanRef) (any, bool, error) {
	return nil, false, nil
}
func (qRuntimeEvalPipelineBackend) ExecuteQEvalPipelinePlanValue(QEvalPipelinePlanRef) (runtime.Value, bool, error) {
	return runtime.NilValue(), false, nil
}

func newQEvalPipelinePlanHelpers([]QEvalPipelinePlanRef, qRuntimeEvalPipelineBackend) []qEvalPipelinePlanHelper {
	return nil
}

func (h *qEvalPipelinePlanHelper) validForID(int) bool { return false }
func (h *qEvalPipelinePlanHelper) execute() (runtime.Value, bool, error) {
	return runtime.NilValue(), false, nil
}

func newQEvalPipelinePlanExecutionCounters(refs []QEvalPipelinePlanRef) []qEvalPipelinePlanExecutionCounters {
	if len(refs) == 0 {
		return nil
	}
	return make([]qEvalPipelinePlanExecutionCounters, len(refs))
}

func qEvalPipelinePlanRefByID(refs []QEvalPipelinePlanRef, id int) (QEvalPipelinePlanRef, bool) {
	if id < 0 || id >= len(refs) {
		return QEvalPipelinePlanRef{}, false
	}
	ref := refs[id]
	return ref, ref.Valid()
}

func qEvalPipelinePlanRefKernel(ref QEvalPipelinePlanRef) string        { return ref.Kernel }
func qEvalPipelinePlanRefShape(ref QEvalPipelinePlanRef) string         { return ref.Shape }
func qEvalPipelinePlanRefPipelineShape(ref QEvalPipelinePlanRef) string { return ref.PipelineShape }
func qEvalPipelineBackendNameFromRef(ref QEvalPipelinePlanRef) string   { return ref.Backend }
func qEvalPipelinePlanRefSource(ref QEvalPipelinePlanRef) string        { return ref.Source }

func formatQEvalPipelinePlanRefs(refs []QEvalPipelinePlanRef) string {
	return fmt.Sprintf("%d q.eval pipeline plan(s)\n", len(refs))
}

func qEvalPipelinePlanExecutionShape(refs []QEvalPipelinePlanRef, id int) string {
	if ref, ok := qEvalPipelinePlanRefByID(refs, id); ok && ref.Shape != "" {
		return ref.Shape
	}
	return "q-eval/pipeline-plan"
}

func qEvalPipelineResumeOffsetTable(*Function, map[int]int) []int { return nil }
func qEvalPipelineTerminalReturnTable(*Function) []bool           { return nil }
func qEvalPipelineDirectReturnPlanID(*Function) int               { return -1 }
func (cf *CompiledFunction) qEvalPipelineResumeOffset(int, bool) (int, bool) {
	return 0, false
}
func (cf *CompiledFunction) qEvalPipelineTerminalReturn(int) bool { return false }
func (cf *CompiledFunction) qEvalPipelinePlanHelper(int) *qEvalPipelinePlanHelper {
	return nil
}
func (cf *CompiledFunction) ExecuteQEvalPipelinePlanValue(int) (runtime.Value, bool, error) {
	return runtime.NilValue(), false, nil
}
func (cf *CompiledFunction) tryExecuteQEvalPipelineDirectReturnValue() (runtime.Value, bool, error) {
	return runtime.NilValue(), false, nil
}
func (cf *CompiledFunction) tryExecuteQEvalPipelineDirectReturn([]runtime.Value) ([]runtime.Value, bool, error) {
	return nil, false, nil
}
func (cf *CompiledFunction) executeQEvalPipelinePlanSlot(int, int, []runtime.Value, qEvalPipelineExecutionRoute) error {
	return fmt.Errorf("q eval pipeline disabled in default build")
}
func (cf *CompiledFunction) executeQEvalPipelinePlanExit(*ExecContext, []runtime.Value, int, qEvalPipelineExecutionRoute) error {
	return fmt.Errorf("q eval pipeline disabled in default build")
}

func qEvalPipelineRuntimeValue(v any) (runtime.Value, error) {
	if value, ok := v.(runtime.Value); ok {
		return value, nil
	}
	return runtime.NilValue(), fmt.Errorf("q eval pipeline disabled in default build")
}

func qEvalSessionEvalSiteTable(*Function) []*qEvalSessionEvalSite { return nil }
func (cf *CompiledFunction) qEvalSessionEvalSite(instrID int) *qEvalSessionEvalSite {
	if cf == nil || instrID < 0 || instrID >= len(cf.QEvalSessionEvalSites) {
		return nil
	}
	return cf.QEvalSessionEvalSites[instrID]
}
func (cf *CompiledFunction) executeQEvalSessionEval(int, int, runtime.Value) (runtime.Value, error) {
	return runtime.NilValue(), fmt.Errorf("q session eval disabled in default build")
}
func executeQEvalSessionEvalValue([]runtime.Value, int, runtime.Value) (runtime.Value, error) {
	return runtime.NilValue(), fmt.Errorf("q session eval disabled in default build")
}
func (cf *CompiledFunction) protoConstants() []runtime.Value {
	if cf == nil || cf.Proto == nil {
		return nil
	}
	return cf.Proto.Constants
}
func (cf *CompiledFunction) recordQEvalSessionEvalExecution(error)        {}
func (cf *CompiledFunction) recordQEvalSessionEvalPlannedExecution(error) {}
func (cf *CompiledFunction) appendQEvalSessionEvalExecutionStats(map[qKernelExecutionKey]uint64) {
}
