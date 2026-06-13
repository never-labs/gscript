package methodjit

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/never-labs/leia/internal/runtime"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
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

// QQueryHotPath describes an IR pattern for the q query primitive pipeline:
// column load -> typed compare mask -> frame filter -> optional row reorder or
// prefix slice -> frame projection -> projected column load.
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

// QVectorWhereHotPath describes a q vector conditional projection:
// typed mask -> vector where. The true/false operands may be frame columns or
// scalars; this keeps the shape visible before a later fused vector lowering.
type QVectorWhereHotPath struct {
	SourceColumn *Instr
	Compare      *Instr
	Mask         *Instr
	MaskCombine  *Instr
	TrueColumn   *Instr
	FalseColumn  *Instr
	Where        *Instr
}

// QVectorReduceHotPath describes a q vector aggregation primitive. The input
// may be a frame column, gathered vector, conditional vector, or another dense
// vector expression that is reduced through a typed runtime op-exit.
type QVectorReduceHotPath struct {
	SourceColumn *Instr
	Gather       *Instr
	Where        *Instr
	Reduce       *Instr
}

// QVectorRuntimeKernel records vector primitives that already execute through
// typed runtime helpers/op-exits and should be visible in q diagnostics.
type QVectorRuntimeKernel struct {
	Instr     *Instr
	Kernel    string
	ShapeName string
	Detail    string
}

// QFrameRuntimeKernel records frame/select projection primitives that already
// execute through typed runtime helpers/op-exits outside QFrameSelectColumn.
type QFrameRuntimeKernel struct {
	Instr     *Instr
	Kernel    string
	ShapeName string
	Detail    string
}

// QKernelDescriptor is the normalized MethodJIT q kernel observation. Summary
// rows are derived from descriptors so new runtime routes do not fork taxonomy.
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

// QKernelDescriptorJSONRow is the stable external row shape for q kernel
// descriptors. Use QKernelDescriptorJSONRows instead of relying on Go field
// names when exporting diagnostics.
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

// QKernelExecutionStat records observed q typed-runtime kernel execution. It
// uses the same stable source/kernel/shape/route keys as QKernelDescriptor so
// runtime observations can be joined with lowering-time descriptors.
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

// QKernelDescriptorCacheStat records schema-stable descriptor cache behavior
// for q typed-runtime kernels observed during native execution.
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

// QKernelExecutionStatJSONRow is the stable external row shape for raw q
// typed-runtime execution observations.
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

// QKernelExecutionRouteSummary aggregates q typed-runtime execution outcomes
// without shape cardinality. It complements QKernelShapeSummary for route-level
// diagnostics and perf triage.
type QKernelExecutionRouteSummary struct {
	Source  string
	Kernel  string
	Route   string
	Outcome string
	Count   uint64
}

// QKernelExecutionRouteSummaryJSONRow is the stable external row shape for q
// kernel route summaries.
type QKernelExecutionRouteSummaryJSONRow struct {
	Source  string `json:"source"`
	Kernel  string `json:"kernel"`
	Route   string `json:"route"`
	Outcome string `json:"outcome"`
	Count   uint64 `json:"count"`
}

type qEvalHotPlan struct {
	BackendPlan            *stdq.EvalPipelineBackendPlan
	Kernel                 string
	Shape                  string
	PipelineShape          string
	ShapeFamily            string
	ShapeReducer           string
	ShapeSelector          string
	ShapeTransform         string
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

// QKernelShapeSummary is a source-stable row for joining MethodJIT q kernel
// diagnostics with qSQL/data kernel shape statistics.
type QKernelShapeSummary struct {
	Source       string
	Kind         string
	Shape        string
	Outcome      string
	ReasonFamily string
	ReasonCode   string
	Count        int
	Executions   uint64
	Successes    uint64
	Errors       uint64
	Hits         int
	Misses       int
	Evictions    int
}

// QKernelShapeSummaryJSONRow is the stable external row shape for q kernel
// shape summaries.
type QKernelShapeSummaryJSONRow struct {
	Source       string `json:"source"`
	Kind         string `json:"kind"`
	Shape        string `json:"shape"`
	Outcome      string `json:"outcome"`
	ReasonFamily string `json:"reason_family,omitempty"`
	ReasonCode   string `json:"reason_code,omitempty"`
	Count        int    `json:"count"`
	Executions   uint64 `json:"executions,omitempty"`
	Successes    uint64 `json:"successes,omitempty"`
	Errors       uint64 `json:"errors,omitempty"`
	Hits         int    `json:"hits,omitempty"`
	Misses       int    `json:"misses,omitempty"`
	Evictions    int    `json:"evictions,omitempty"`
}

const (
	qVectorWhereReduceFallbackSharedWhere      = "shared_where"
	qVectorWhereReduceFallbackBadWhereArgCount = "bad_where_arg_count"
	qVectorWhereReduceFallbackUnsupportedInput = "unsupported_input_shape"
	qVectorGatherReduceFallbackSharedGather    = "shared_gather"
)

func (p QQueryHotPath) Shape() string {
	if p.Compare == nil && p.Mask == nil && p.MaskCombine == nil {
		switch {
		case p.RowOrder != nil && p.RowGather != nil:
			return "order/gather/project/column"
		case p.RowGather != nil:
			return "gather/project/column"
		case p.RowSlice != nil:
			return "slice/project/column"
		default:
			return "project/column"
		}
	}
	prefix := "compare/filter"
	if p.Mask != nil {
		prefix = "mask/filter"
	} else if p.MaskCombine != nil {
		prefix = "mask-combine/filter"
	}
	switch {
	case p.RowOrder != nil && p.RowGather != nil:
		return prefix + "/order/gather/project/column"
	case p.RowGather != nil:
		return prefix + "/gather/project/column"
	case p.RowSlice != nil:
		return prefix + "/slice/project/column"
	default:
		return prefix + "/project/column"
	}
}

func (p QVectorWhereHotPath) Shape() string {
	prefix := "compare"
	if p.Mask != nil {
		prefix = "mask"
	} else if p.MaskCombine != nil {
		prefix = "mask-combine"
	}
	return prefix + "/vector-where"
}

func (p QVectorReduceHotPath) Shape() string {
	switch {
	case p.Where != nil:
		return "where/vector-reduce"
	case p.Gather != nil:
		return "gather/vector-reduce"
	case p.SourceColumn != nil:
		return "column/vector-reduce"
	default:
		return "vector/vector-reduce"
	}
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

func CountQQueryHotPathShapes(paths []QQueryHotPath) map[string]int {
	if len(paths) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, path := range paths {
		counts[path.Shape()]++
	}
	return counts
}

func CountQVectorWhereHotPathShapes(paths []QVectorWhereHotPath) map[string]int {
	if len(paths) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, path := range paths {
		counts[path.Shape()]++
	}
	return counts
}

func CountQVectorReduceHotPathShapes(paths []QVectorReduceHotPath) map[string]int {
	if len(paths) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, path := range paths {
		counts[path.Shape()]++
	}
	return counts
}

func CountQVectorRuntimeKernelShapes(kernels []QVectorRuntimeKernel) map[string]int {
	if len(kernels) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, kernel := range kernels {
		counts[kernel.Shape()]++
	}
	return counts
}

func CountQFrameRuntimeKernelShapes(kernels []QFrameRuntimeKernel) map[string]int {
	if len(kernels) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, kernel := range kernels {
		counts[kernel.Shape()]++
	}
	return counts
}

func CountQFrameSelectColumnSpecShapes(specs []QFrameSelectColumnSpec) map[string]int {
	if len(specs) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, spec := range specs {
		shape := spec.Shape
		if shape == "" {
			shape = "unknown"
		}
		counts[shape]++
	}
	return counts
}

func CountQQueryLoweringFallbackReasons(remarks []OptimizationRemark) map[string]int {
	counts := make(map[string]int)
	for _, remark := range remarks {
		if remark.Pass != "QQueryNativeLowering" || remark.Kind != "missed" {
			continue
		}
		reason, ok := qQueryLoweringFallbackReasonFromRemark(remark)
		if !ok {
			continue
		}
		counts[reason]++
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

func qQueryLoweringFallbackReasonFromRemark(remark OptimizationRemark) (string, bool) {
	return qLoweringRemarkFieldFromRemark(remark, "reason_code")
}

func qQueryLoweringFallbackShapeFromRemark(remark OptimizationRemark) string {
	shape, ok := qLoweringRemarkFieldFromRemark(remark, "shape")
	if !ok {
		return "unknown"
	}
	return shape
}

func qLoweringRemarkFieldFromRemark(remark OptimizationRemark, name string) (string, bool) {
	if remark.Fields != nil {
		if value := remark.Fields[name]; value != "" {
			return value, true
		}
	}
	return qLoweringRemarkField(remark.Reason, name)
}

func qLoweringRemarkFieldOrDefault(remark OptimizationRemark, name, fallback string) string {
	if value, ok := qLoweringRemarkFieldFromRemark(remark, name); ok {
		return value
	}
	return fallback
}

func qLoweringRemarkField(reason, name string) (string, bool) {
	prefix := name + "="
	for _, field := range strings.Fields(reason) {
		if strings.HasPrefix(field, prefix) {
			code := strings.TrimRight(strings.TrimPrefix(field, prefix), ",;")
			return code, code != ""
		}
	}
	return "", false
}

func CountQVectorLoweringFallbackReasons(remarks []OptimizationRemark) map[string]int {
	counts := make(map[string]int)
	for _, remark := range remarks {
		if remark.Pass != "QVectorNativeLowering" || remark.Kind != "missed" {
			continue
		}
		reason, ok := qQueryLoweringFallbackReasonFromRemark(remark)
		if !ok {
			continue
		}
		counts[reason]++
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

func BuildQKernelDescriptors(vectorKernels []QVectorRuntimeKernel, framePrimitiveKernels []QFrameRuntimeKernel, frameKernels []QFrameSelectColumnSpec, remarks []OptimizationRemark) []QKernelDescriptor {
	var out []QKernelDescriptor
	for _, kernel := range vectorKernels {
		shape := kernel.Shape()
		if shape == "" {
			shape = "unknown"
		}
		out = append(out, QKernelDescriptor{
			Source:        "methodjit_q_vector_runtime",
			Kind:          "runtime_kernel",
			Kernel:        kernel.Kernel,
			Shape:         shape,
			PipelineShape: shape,
			Route:         "typed_runtime_op_exit",
			Outcome:       "supported",
		})
	}
	for _, kernel := range framePrimitiveKernels {
		shape := kernel.Shape()
		if shape == "" {
			shape = "unknown"
		}
		out = append(out, QKernelDescriptor{
			Source:        "methodjit_q_frame_runtime",
			Kind:          "runtime_kernel",
			Kernel:        kernel.Kernel,
			Shape:         shape,
			PipelineShape: shape,
			Route:         "typed_runtime_op_exit",
			Outcome:       "supported",
		})
	}
	for _, spec := range frameKernels {
		shape := spec.Shape
		if shape == "" {
			shape = "unknown"
		}
		out = append(out, QKernelDescriptor{
			Source:        "methodjit_q_frame_runtime",
			Kind:          "runtime_kernel",
			Kernel:        "QFrameSelectColumn",
			Shape:         shape,
			PipelineShape: shape,
			Route:         "typed_runtime_op_exit",
			Outcome:       "supported",
		})
	}
	for _, remark := range remarks {
		if remark.Kind != "missed" && !(remark.Pass == "QEvalHotPlan" && remark.Kind == "changed") {
			continue
		}
		var source, kernel string
		switch remark.Pass {
		case "QQueryNativeLowering":
			source = "methodjit_q_query_lowering"
			kernel = "QFrameSelectColumn"
		case "QVectorNativeLowering":
			source = "methodjit_q_vector_lowering"
			kernel = "QVectorWhereReduce"
		case "QEvalHotPlan":
			source = "methodjit_q_eval_lowering"
			kernel = "QEvalVectorPlan"
		default:
			continue
		}
		reason := ""
		if remark.Kind == "missed" {
			var ok bool
			reason, ok = qQueryLoweringFallbackReasonFromRemark(remark)
			if !ok {
				continue
			}
		}
		if remarkKernel, ok := qLoweringRemarkFieldFromRemark(remark, "kernel"); ok {
			kernel = remarkKernel
		}
		out = append(out, QKernelDescriptor{
			Source:        source,
			Kind:          qLoweringRemarkFieldOrDefault(remark, "kind", "fallback"),
			Kernel:        kernel,
			Shape:         qQueryLoweringFallbackShapeFromRemark(remark),
			PipelineShape: qLoweringRemarkFieldOrDefault(remark, "pipeline_shape", ""),
			Route:         qLoweringRemarkFieldOrDefault(remark, "route", "lowering"),
			Outcome:       qLoweringRemarkFieldOrDefault(remark, "outcome", "fallback"),
			ReasonFamily:  qLoweringRemarkFieldOrDefault(remark, "reason_family", "lowering"),
			ReasonCode:    reason,
		})
	}
	if len(out) == 0 {
		return nil
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Kernel != out[j].Kernel {
			return out[i].Kernel < out[j].Kernel
		}
		if out[i].Shape != out[j].Shape {
			return out[i].Shape < out[j].Shape
		}
		if out[i].PipelineShape != out[j].PipelineShape {
			return out[i].PipelineShape < out[j].PipelineShape
		}
		if out[i].Route != out[j].Route {
			return out[i].Route < out[j].Route
		}
		if out[i].Outcome != out[j].Outcome {
			return out[i].Outcome < out[j].Outcome
		}
		if out[i].ReasonFamily != out[j].ReasonFamily {
			return out[i].ReasonFamily < out[j].ReasonFamily
		}
		return out[i].ReasonCode < out[j].ReasonCode
	})
	return out
}

func BuildQKernelShapeSummaryFromDescriptors(descriptors []QKernelDescriptor) []QKernelShapeSummary {
	return BuildQKernelShapeSummaryFromDescriptorsAndExecutionStats(descriptors, nil)
}

func BuildQKernelShapeSummaryFromDescriptorsAndExecutionStats(descriptors []QKernelDescriptor, executionStats []QKernelExecutionStat) []QKernelShapeSummary {
	counts := make(map[qKernelShapeSummaryKey]int)
	executions := make(map[qKernelShapeSummaryKey]qKernelExecutionCounters)
	for _, descriptor := range descriptors {
		counts[qKernelShapeSummaryKey{
			source:       descriptor.Source,
			kind:         descriptor.Kind,
			shape:        descriptor.Shape,
			outcome:      descriptor.Outcome,
			reasonFamily: descriptor.ReasonFamily,
			reasonCode:   descriptor.ReasonCode,
		}]++
	}
	for _, stat := range executionStats {
		key := qKernelShapeSummaryKey{
			source:  qKernelExecutionSummarySource(stat.Source),
			kind:    "runtime_kernel",
			shape:   qKernelExecutionSummaryShape(stat.Shape),
			outcome: "supported",
		}
		counter := executions[key]
		counter.executions += stat.Count
		switch stat.Outcome {
		case "success":
			counter.successes += stat.Count
		case "error":
			counter.errors += stat.Count
		}
		executions[key] = counter
		if _, ok := counts[key]; !ok {
			counts[key] = 0
		}
	}
	if len(counts) == 0 && len(executions) == 0 {
		return nil
	}
	keys := make([]qKernelShapeSummaryKey, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].source != keys[j].source {
			return keys[i].source < keys[j].source
		}
		if keys[i].kind != keys[j].kind {
			return keys[i].kind < keys[j].kind
		}
		if keys[i].shape != keys[j].shape {
			return keys[i].shape < keys[j].shape
		}
		if keys[i].outcome != keys[j].outcome {
			return keys[i].outcome < keys[j].outcome
		}
		if keys[i].reasonFamily != keys[j].reasonFamily {
			return keys[i].reasonFamily < keys[j].reasonFamily
		}
		return keys[i].reasonCode < keys[j].reasonCode
	})
	out := make([]QKernelShapeSummary, 0, len(keys))
	for _, key := range keys {
		execution := executions[key]
		out = append(out, QKernelShapeSummary{
			Source:       key.source,
			Kind:         key.kind,
			Shape:        key.shape,
			Outcome:      key.outcome,
			ReasonFamily: key.reasonFamily,
			ReasonCode:   key.reasonCode,
			Count:        counts[key],
			Executions:   execution.executions,
			Successes:    execution.successes,
			Errors:       execution.errors,
		})
	}
	return out
}

func BuildQKernelShapeSummary(vectorKernels []QVectorRuntimeKernel, framePrimitiveKernels []QFrameRuntimeKernel, frameKernels []QFrameSelectColumnSpec, remarks []OptimizationRemark) []QKernelShapeSummary {
	return BuildQKernelShapeSummaryFromDescriptors(BuildQKernelDescriptors(vectorKernels, framePrimitiveKernels, frameKernels, remarks))
}

func QKernelDescriptorJSONRows(rows []QKernelDescriptor) []QKernelDescriptorJSONRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]QKernelDescriptorJSONRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, QKernelDescriptorJSONRow{
			Source:        row.Source,
			Kind:          row.Kind,
			Kernel:        row.Kernel,
			Shape:         row.Shape,
			PipelineShape: row.PipelineShape,
			Route:         row.Route,
			Outcome:       row.Outcome,
			ReasonFamily:  row.ReasonFamily,
			ReasonCode:    row.ReasonCode,
		})
	}
	return out
}

func QKernelExecutionStatJSONRows(rows []QKernelExecutionStat) []QKernelExecutionStatJSONRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]QKernelExecutionStatJSONRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, QKernelExecutionStatJSONRow{
			Source:        row.Source,
			Kernel:        row.Kernel,
			Shape:         row.Shape,
			PipelineShape: row.PipelineShape,
			Route:         row.Route,
			Outcome:       row.Outcome,
			ReasonCode:    row.ReasonCode,
			Count:         row.Count,
		})
	}
	return out
}

func BuildQKernelExecutionRouteSummary(rows []QKernelExecutionStat) []QKernelExecutionRouteSummary {
	counts := make(map[qKernelExecutionRouteSummaryKey]uint64)
	for _, row := range rows {
		counts[qKernelExecutionRouteSummaryKey{
			source:  qKernelExecutionSummarySource(row.Source),
			kernel:  qKernelExecutionSummaryKernel(row.Kernel),
			route:   qKernelExecutionSummaryRoute(row.Route),
			outcome: qKernelExecutionSummaryOutcome(row.Outcome),
		}] += row.Count
	}
	if len(counts) == 0 {
		return nil
	}
	keys := make([]qKernelExecutionRouteSummaryKey, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].source != keys[j].source {
			return keys[i].source < keys[j].source
		}
		if keys[i].kernel != keys[j].kernel {
			return keys[i].kernel < keys[j].kernel
		}
		if keys[i].route != keys[j].route {
			return keys[i].route < keys[j].route
		}
		return keys[i].outcome < keys[j].outcome
	})
	out := make([]QKernelExecutionRouteSummary, 0, len(keys))
	for _, key := range keys {
		out = append(out, QKernelExecutionRouteSummary{
			Source:  key.source,
			Kernel:  key.kernel,
			Route:   key.route,
			Outcome: key.outcome,
			Count:   counts[key],
		})
	}
	return out
}

func QKernelExecutionRouteSummaryJSONRows(rows []QKernelExecutionRouteSummary) []QKernelExecutionRouteSummaryJSONRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]QKernelExecutionRouteSummaryJSONRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, QKernelExecutionRouteSummaryJSONRow{
			Source:  row.Source,
			Kernel:  row.Kernel,
			Route:   row.Route,
			Outcome: row.Outcome,
			Count:   row.Count,
		})
	}
	return out
}

func QKernelShapeSummaryJSONRows(rows []QKernelShapeSummary) []QKernelShapeSummaryJSONRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]QKernelShapeSummaryJSONRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, QKernelShapeSummaryJSONRow{
			Source:       row.Source,
			Kind:         row.Kind,
			Shape:        row.Shape,
			Outcome:      row.Outcome,
			ReasonFamily: row.ReasonFamily,
			ReasonCode:   row.ReasonCode,
			Count:        row.Count,
			Executions:   row.Executions,
			Successes:    row.Successes,
			Errors:       row.Errors,
			Hits:         row.Hits,
			Misses:       row.Misses,
			Evictions:    row.Evictions,
		})
	}
	return out
}

func qKernelExecutionSummarySource(source string) string {
	if source == "" {
		return "unknown"
	}
	return source
}

func qKernelExecutionSummaryKernel(kernel string) string {
	if kernel == "" {
		return "unknown"
	}
	return kernel
}

func qKernelExecutionSummaryShape(shape string) string {
	if shape == "" {
		return "unknown"
	}
	return shape
}

func qKernelExecutionSummaryRoute(route string) string {
	if route == "" {
		return "unknown"
	}
	return route
}

func qKernelExecutionSummaryOutcome(outcome string) string {
	if outcome == "" {
		return "unknown"
	}
	return outcome
}

type qKernelExecutionCounters struct {
	executions uint64
	successes  uint64
	errors     uint64
}

type qKernelShapeSummaryKey struct {
	source       string
	kind         string
	shape        string
	outcome      string
	reasonFamily string
	reasonCode   string
}

type qKernelExecutionRouteSummaryKey struct {
	source  string
	kernel  string
	route   string
	outcome string
}

// DetectQQueryHotPaths returns q query primitive pipelines visible in Method
// JIT IR. It is intentionally a recognizer only: execution still uses the
// existing primitive op-exit/runtime helpers until a later lowering consumes
// this metadata.
func DetectQQueryHotPaths(fn *Function) []QQueryHotPath {
	if fn == nil {
		return nil
	}
	var out []QQueryHotPath
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpFrameColumn || len(instr.Args) != 1 {
				continue
			}
			project := valueDef(instr.Args[0], OpFrameProject)
			if project == nil || len(project.Args) != 1 {
				continue
			}
			filterInput := project.Args[0]
			var rowGather *Instr
			var rowSlice *Instr
			var rowOrder *Instr
			if orderGather := valueDef(filterInput, OpFrameOrderGather); orderGather != nil {
				if len(orderGather.Args) != 1 {
					continue
				}
				rowGather = orderGather
				rowOrder = orderGather
				filterInput = orderGather.Args[0]
			} else if gather := valueDef(filterInput, OpFrameGather); gather != nil {
				if len(gather.Args) != 2 {
					continue
				}
				rowGather = gather
				rowOrder = valueDef(gather.Args[1], OpFrameOrder)
				if rowOrder != nil && !qQueryFrameOrderMatchesGather(gather, rowOrder) {
					rowOrder = nil
				}
				filterInput = gather.Args[0]
			} else if slice := valueDef(filterInput, OpFrameSlice); slice != nil {
				if len(slice.Args) != 2 {
					continue
				}
				rowSlice = slice
				filterInput = slice.Args[0]
			}
			filter := valueDef(filterInput, OpFrameFilter)
			compare, mask, maskCombine := (*Instr)(nil), (*Instr)(nil), (*Instr)(nil)
			var sourceColumn *Instr
			if filter != nil {
				if len(filter.Args) != 2 {
					continue
				}
				compare = valueDef(filter.Args[1], OpVectorCompare)
				mask = valueDef(filter.Args[1], OpFrameMask)
				maskCombine = valueDef(filter.Args[1], OpVectorMask)
				if compare != nil {
					if len(compare.Args) != 2 {
						continue
					}
					sourceColumn = qQueryCompareColumn(compare)
					if sourceColumn == nil || len(sourceColumn.Args) != 1 {
						continue
					}
					if filter.Args[0] == nil || sourceColumn.Args[0] == nil || filter.Args[0].ID != sourceColumn.Args[0].ID {
						continue
					}
				} else if mask != nil {
					if len(mask.Args) != 1 || filter.Args[0] == nil || mask.Args[0] == nil || filter.Args[0].ID != mask.Args[0].ID {
						continue
					}
				} else if maskCombine != nil {
					if !qQueryMaskCombineUsesFrame(filter.Args[0], maskCombine) {
						continue
					}
				} else {
					continue
				}
			}
			out = append(out, QQueryHotPath{
				SourceColumn: sourceColumn,
				Compare:      compare,
				Mask:         mask,
				MaskCombine:  maskCombine,
				Filter:       filter,
				RowGather:    rowGather,
				RowSlice:     rowSlice,
				RowOrder:     rowOrder,
				Project:      project,
				ResultColumn: instr,
			})
		}
	}
	return out
}

func qQueryFrameOrderMatchesGather(gather, order *Instr) bool {
	if gather == nil || order == nil || len(gather.Args) < 1 || len(order.Args) < 1 {
		return false
	}
	return gather.Args[0] != nil && order.Args[0] != nil && gather.Args[0].ID == order.Args[0].ID
}

// DetectQVectorWhereHotPaths returns q vector conditional-select pipelines
// visible in Method JIT IR. It is diagnostic metadata today; native lowering
// still uses OpVectorWhere's typed runtime op-exit.
func DetectQVectorWhereHotPaths(fn *Function) []QVectorWhereHotPath {
	if fn == nil {
		return nil
	}
	var out []QVectorWhereHotPath
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpVectorWhere || len(instr.Args) != 3 {
				continue
			}
			compare, mask, maskCombine := qVectorWherePredicate(instr.Args[0])
			if compare == nil && mask == nil && maskCombine == nil {
				continue
			}
			sourceColumn := (*Instr)(nil)
			if compare != nil {
				sourceColumn = qQueryCompareColumn(compare)
				if sourceColumn == nil {
					continue
				}
			}
			out = append(out, QVectorWhereHotPath{
				SourceColumn: sourceColumn,
				Compare:      compare,
				Mask:         mask,
				MaskCombine:  maskCombine,
				TrueColumn:   valueDef(instr.Args[1], OpFrameColumn),
				FalseColumn:  valueDef(instr.Args[2], OpFrameColumn),
				Where:        instr,
			})
		}
	}
	return out
}

// DetectQVectorReduceHotPaths returns q vector aggregate primitives visible in
// Method JIT IR. These already execute through typed runtime op-exits; the
// diagnostic shape is used to track how much aggregate work still falls back.
func DetectQVectorReduceHotPaths(fn *Function) []QVectorReduceHotPath {
	if fn == nil {
		return nil
	}
	var out []QVectorReduceHotPath
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpVectorReduce || len(instr.Args) != 1 {
				continue
			}
			arg := instr.Args[0]
			out = append(out, QVectorReduceHotPath{
				SourceColumn: valueDef(arg, OpFrameColumn),
				Gather:       valueDef(arg, OpVectorGather),
				Where:        valueDef(arg, OpVectorWhere),
				Reduce:       instr,
			})
		}
	}
	return out
}

// DetectQVectorRuntimeKernels returns vector primitives that are carried as
// typed runtime kernels in Method JIT. This intentionally covers standalone
// vector gather/compare/mask/reduce/scan plus conditional vector projection.
func DetectQVectorRuntimeKernels(fn *Function) []QVectorRuntimeKernel {
	if fn == nil {
		return nil
	}
	var out []QVectorRuntimeKernel
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			switch instr.Op {
			case OpVectorGather:
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "VectorGather", ShapeName: "vector-gather"})
			case OpVectorCompare:
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "VectorCompare", ShapeName: "vector-compare", Detail: "op=" + qDenseArrayCompareOpName(runtime.DenseArrayBinaryOp(instr.Aux))})
			case OpVectorMask:
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "VectorMask", ShapeName: "vector-mask", Detail: "op=" + qDenseArrayMaskOpName(runtime.DenseArrayMaskOp(instr.Aux))})
			case OpVectorWhere:
				shape := "vector-where"
				detail := ""
				if path := qVectorWhereHotPath(instr); path.Where != nil {
					shape = path.Shape()
					detail = "predicate=" + qVectorWherePredicateName(path)
				}
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "VectorWhere", ShapeName: shape, Detail: detail})
			case OpVectorReduce:
				path := qVectorReduceHotPath(instr)
				shape := "vector/vector-reduce"
				detail := "op=" + qDenseArrayReduceOpName(runtime.DenseArrayReduceOp(instr.Aux))
				if path.Reduce != nil {
					shape = path.Shape()
					detail = "op=" + qVectorReduceOpName(path) + " input=" + qVectorReduceInputName(path)
				}
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "VectorReduce", ShapeName: shape, Detail: detail})
			case OpQVectorWhereReduce:
				detail := "op=" + qDenseArrayReduceOpName(runtime.DenseArrayReduceOp(instr.Aux))
				if len(instr.Args) == 3 {
					detail += " predicate=" + qVectorWherePredicateValueDetail(instr.Args[0])
				}
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "QVectorWhereReduce", ShapeName: qVectorWhereReduceShape(instr), Detail: detail})
			case OpQVectorGatherReduce:
				detail := "op=" + qDenseArrayReduceOpName(runtime.DenseArrayReduceOp(instr.Aux))
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "QVectorGatherReduce", ShapeName: "gather/vector-reduce", Detail: detail})
			case OpVectorScan:
				out = append(out, QVectorRuntimeKernel{Instr: instr, Kernel: "VectorScan", ShapeName: "vector-scan"})
			}
		}
	}
	return out
}

// DetectQFrameRuntimeKernels returns frame/select projection primitives that
// are carried as typed runtime op-exits but are not produced by
// QQueryNativeLoweringPass.
func DetectQFrameRuntimeKernels(fn *Function) []QFrameRuntimeKernel {
	if fn == nil {
		return nil
	}
	var out []QFrameRuntimeKernel
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			source, kernel, shape, _, ok := qRuntimePrimitiveExecutionMetadata(instr.Op)
			if !ok || source != "methodjit_q_frame_runtime" {
				continue
			}
			if instr.Op == OpFrameGroupAggregate {
				shape = qFrameGroupAggregateRuntimeShapeFromInstr(instr)
			}
			out = append(out, QFrameRuntimeKernel{Instr: instr, Kernel: kernel, ShapeName: shape})
		}
	}
	return out
}

// QQueryHotPathRemarkPass records visible q query primitive hot paths in the
// structured optimization remark stream. It does not mutate IR; the remark is a
// handoff point for diagnostics and future native lowering policy.
func QQueryHotPathRemarkPass(fn *Function) (*Function, error) {
	paths := DetectQQueryHotPaths(fn)
	if len(paths) == 0 {
		return fn, nil
	}
	first := paths[0].ResultColumn
	blockID, valueID := 0, 0
	if first != nil {
		valueID = first.ID
		if first.Block != nil {
			blockID = first.Block.ID
		}
	}
	functionRemarks(fn).Add(
		"QQueryHotPath",
		"missed",
		blockID,
		valueID,
		OpFrameColumn,
		fmt.Sprintf("recognized %d q query primitive hot path(s), first shape %s, compare %s; native lowering pending",
			len(paths), paths[0].Shape(), qQueryHotPathPredicateName(paths[0])),
	)
	return fn, nil
}

// QQueryNativeLoweringPass folds simple q primitive hot paths into a single
// runtime-kernel op-exit. This is the first executable lowering step after
// recognition; full native codegen can target OpQFrameSelectColumn later.
func QQueryNativeLoweringPass(fn *Function) (*Function, error) {
	if fn == nil {
		return fn, nil
	}
	uses := qQueryValueUseCounts(fn)
	for _, path := range DetectQQueryHotPaths(fn) {
		if !qQueryHotPathSingleUse(path, uses) {
			qQueryLoweringFallbackRemark(fn, path, qQueryLoweringFallbackMultiUse)
			continue
		}
		spec, args, reason, ok := qQueryFrameSelectColumnSpec(fn, path)
		if !ok {
			qQueryLoweringFallbackRemark(fn, path, reason)
			continue
		}
		specIdx := len(fn.QFrameSelectColumnSpecs)
		fn.QFrameSelectColumnSpecs = append(fn.QFrameSelectColumnSpecs, spec)
		result := path.ResultColumn
		result.Op = OpQFrameSelectColumn
		result.Type = TypeAny
		result.Args = args
		result.Aux = int64(specIdx)
		result.Aux2 = 0
		qQueryNopPredicateIfSingleUse(path, uses)
		qQueryNop(path.Filter)
		qQueryNop(path.RowOrder)
		qQueryNop(path.RowGather)
		qQueryNop(path.RowSlice)
		qQueryNop(path.Project)
		if result.Block != nil {
			functionRemarks(fn).Add("QQueryNativeLowering", "changed", result.Block.ID, result.ID, OpQFrameSelectColumn,
				fmt.Sprintf("lowered q query primitive hot path shape %s to typed runtime kernel op-exit", spec.Shape))
		}
	}
	qVectorWhereReduceLoweringPass(fn, uses)
	qGroupAggregateNativeLoweringPass(fn)
	qJoinFallbackRemarkPass(fn)
	qGroupAggregateFallbackRemarkPass(fn)
	return fn, nil
}

// QEvalHotPlanRemarkPass records constant q.eval sources that are structurally
// plan-ready for q vector typed-runtime lowering. It is intentionally read-only:
// the descriptor gives MethodJIT and benchmark tooling a stable handoff point
// before execution is lowered into OpQEvalVectorPlan or existing OpVector* ops.
func QEvalHotPlanRemarkPass(fn *Function) (*Function, error) {
	if fn == nil {
		return fn, nil
	}
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpCall || !qCallIsQEvalEntrypoint(fn, instr) {
				continue
			}
			source, ok := qCallEvalSourceString(fn, instr)
			if !ok {
				qEvalHotPlanFallbackRemark(fn, instr, qEvalHotPlanFallbackDynamicSource, "q-eval/dynamic-source")
				continue
			}
			plan, reason, ok := qClassifyEvalHotPlan(source)
			if !ok {
				qEvalHotPlanFallbackRemark(fn, instr, reason, "q-eval/unsupported")
				continue
			}
			backendPlan := qEvalPipelineBackendPlanFromHotPlan(source, plan)
			if !backendPlan.Valid() || backendPlan.Backend != qEvalPipelineTypedRuntimeBackend {
				qEvalHotPlanFallbackRemark(fn, instr, qEvalHotPlanFallbackHeuristicOnly, plan.Shape)
				continue
			}
			ref := fn.addQEvalPipelinePlan(source, plan)
			if _, ok := qEvalPipelineTypedRuntimeBackendPlanFromRef(ref); !ok {
				qEvalHotPlanFallbackRemark(fn, instr, qEvalHotPlanFallbackHeuristicOnly, plan.Shape)
				continue
			}
			qEvalHotPlanSupportedRemark(fn, instr, plan, ref)
		}
	}
	return fn, nil
}

// QEvalPipelineLoweringPass rewrites constant q.eval hot plans to a backend
// plan op. The op keeps fallback explicit: dynamic or unsupported q.eval calls
// remain as OpCall and continue through the normal call path.
func QEvalPipelineLoweringPass(fn *Function) (*Function, error) {
	if fn == nil {
		return fn, nil
	}
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpCall || !qCallIsQEvalEntrypoint(fn, instr) {
				continue
			}
			source, ok := qCallEvalSourceString(fn, instr)
			if !ok {
				continue
			}
			plan, _, ok := qClassifyEvalHotPlan(source)
			if !ok {
				continue
			}
			backendPlan := qEvalPipelineBackendPlanFromHotPlan(source, plan)
			if !backendPlan.Valid() || backendPlan.Backend != qEvalPipelineTypedRuntimeBackend {
				continue
			}
			ref := fn.addQEvalPipelinePlan(source, plan)
			if !ref.Valid() {
				continue
			}
			typedBackendPlan, ok := qEvalPipelineTypedRuntimeBackendPlanFromRef(ref)
			if !ok || !typedBackendPlan.Valid() {
				continue
			}
			instr.Op = OpQEvalPipelinePlan
			instr.Type = TypeAny
			instr.Args = nil
			instr.Aux = int64(ref.ID)
			instr.Aux2 = 0
			blockID, valueID := qRemarkLocation(instr)
			kernel := qEvalPipelinePlanRefKernel(ref)
			shape := qEvalPipelinePlanRefShape(ref)
			pipelineShape := qEvalPipelinePlanRefPipelineShape(ref)
			backend := qEvalPipelineBackendNameFromRef(ref)
			functionRemarks(fn).AddWithFields("QEvalPipelineLowering", "changed", blockID, valueID, OpQEvalPipelinePlan,
				fmt.Sprintf("lowered constant q.eval source to typed pipeline plan id=%d shape=%s", ref.ID, shape),
				map[string]string{
					"kind":           "runtime_kernel",
					"kernel":         kernel,
					"shape":          shape,
					"pipeline_shape": pipelineShape,
					"route":          "typed_pipeline_op",
					"outcome":        "lowered",
					"backend":        backend,
					"plan_id":        strconv.Itoa(ref.ID),
				})
		}
	}
	return fn, nil
}

func qEvalHotPlanSupportedRemark(fn *Function, call *Instr, plan qEvalHotPlan, ref QEvalPipelinePlanRef) {
	blockID, valueID := qRemarkLocation(call)
	kernel := plan.Kernel
	shape := plan.Shape
	pipelineShape := plan.PipelineShape
	backend := ""
	if ref.Valid() {
		if got := qEvalPipelinePlanRefKernel(ref); got != "" {
			kernel = got
		}
		if got := qEvalPipelinePlanRefShape(ref); got != "" {
			shape = got
		}
		if got := qEvalPipelinePlanRefPipelineShape(ref); got != "" {
			pipelineShape = got
		}
		backend = qEvalPipelineBackendNameFromRef(ref)
	}
	fields := map[string]string{
		"kind":    "runtime_kernel",
		"kernel":  kernel,
		"shape":   shape,
		"route":   "hot_plan",
		"outcome": "supported",
	}
	if plan.Detail != "" {
		fields["detail"] = plan.Detail
	}
	if pipelineShape != "" {
		fields["pipeline_shape"] = pipelineShape
	}
	if ref.Valid() {
		fields["plan_id"] = strconv.Itoa(ref.ID)
		fields["backend"] = backend
	}
	functionRemarks(fn).AddWithFields("QEvalHotPlan", "changed", blockID, valueID, OpCall,
		fmt.Sprintf("kernel=%s shape=%s plan_id=%d; constant q.eval source is bound to typed pipeline plan interface",
			kernel, shape, ref.ID),
		fields)
}

func qEvalHotPlanFallbackRemark(fn *Function, call *Instr, reasonCode, shape string) {
	if reasonCode == "" {
		reasonCode = qEvalHotPlanFallbackUnsupportedShape
	}
	if shape == "" {
		shape = "q-eval/unsupported"
	}
	blockID, valueID := qRemarkLocation(call)
	functionRemarks(fn).AddWithFields("QEvalHotPlan", "missed", blockID, valueID, OpCall,
		fmt.Sprintf("kernel=QEvalVectorPlan reason_code=%s shape=%s; q.eval source remains on interpreter fallback",
			reasonCode, shape),
		map[string]string{
			"kind":          "fallback",
			"kernel":        "QEvalVectorPlan",
			"shape":         shape,
			"reason_family": "lowering",
			"reason_code":   reasonCode,
			"route":         "hot_plan",
			"outcome":       "fallback",
		})
}

func qRemarkLocation(instr *Instr) (blockID, valueID int) {
	if instr == nil {
		return 0, 0
	}
	valueID = instr.ID
	if instr.Block != nil {
		blockID = instr.Block.ID
	}
	return blockID, valueID
}

func qClassifyEvalHotPlan(source string) (qEvalHotPlan, string, bool) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return qEvalHotPlan{}, qEvalHotPlanFallbackEmptySource, false
	}
	if plan, ok := qEvalRuntimePipelinePlan(trimmed); ok {
		return plan, "", true
	}
	expr := qEvalFinalExpression(trimmed)
	normalized := strings.ToLower(strings.Join(strings.Fields(expr), " "))
	switch {
	case normalized == "":
		return qEvalHotPlan{}, qEvalHotPlanFallbackEmptySource, false
	case strings.Contains(normalized, " where ") && qEvalLooksLikeReduce(normalized):
		return qEvalHeuristicHotPlan("QEvalWhereReduce", "where/vector-reduce"), "", true
	case qEvalLooksLikeReduce(normalized):
		return qEvalHeuristicHotPlan("QEvalVectorReduce", "vector-reduce/"+qEvalReduceOpName(normalized)), "", true
	case strings.Contains(normalized, "+\\") || strings.HasPrefix(normalized, "sums "):
		return qEvalHeuristicHotPlan("QEvalVectorScan", "vector-scan/sum"), "", true
	case strings.Contains(normalized, "-':") || strings.HasPrefix(normalized, "deltas "):
		return qEvalHeuristicHotPlan("QEvalVectorScan", "vector-scan/deltas"), "", true
	case strings.Contains(normalized, " rotate "):
		return qEvalHeuristicHotPlan("QEvalVectorTransform", "vector-transform/rotate"), "", true
	case strings.HasPrefix(normalized, "reverse "):
		return qEvalHeuristicHotPlan("QEvalVectorTransform", "vector-transform/reverse"), "", true
	case strings.HasPrefix(normalized, "where ") || strings.Contains(normalized, " where "):
		return qEvalHeuristicHotPlan("QEvalWhere", "mask-to-index"), "", true
	case qEvalLooksLikeVectorDyadic(normalized):
		return qEvalHeuristicHotPlan("QEvalVectorDyadic", "vector-dyadic"), "", true
	default:
		return qEvalHotPlan{}, qEvalHotPlanFallbackUnsupportedShape, false
	}
}

func qEvalHeuristicHotPlan(kernel, shape string) qEvalHotPlan {
	return qEvalHotPlan{
		Kernel:        kernel,
		Shape:         shape,
		Backend:       "methodjit_q_eval_heuristic",
		Detail:        "source=constant",
		PipelineShape: "",
	}
}

func qEvalFinalExpression(source string) string {
	depth := 0
	start := 0
	for i, r := range source {
		switch r {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				start = i + 1
			}
		}
	}
	expr := strings.TrimSpace(source[start:])
	if name, rhs, ok := strings.Cut(expr, ":"); ok && qSimpleIdentifier(strings.TrimSpace(name)) {
		return strings.TrimSpace(rhs)
	}
	return expr
}

func qEvalLooksLikeReduce(expr string) bool {
	return strings.Contains(expr, "+/") ||
		strings.HasPrefix(expr, "sum ") ||
		strings.HasPrefix(expr, "avg ") ||
		strings.HasPrefix(expr, "min ") ||
		strings.HasPrefix(expr, "max ") ||
		strings.HasPrefix(expr, "prd ") ||
		strings.HasPrefix(expr, "count ")
}

func qEvalReduceOpName(expr string) string {
	switch {
	case strings.Contains(expr, "+/") || strings.HasPrefix(expr, "sum "):
		return "sum"
	case strings.HasPrefix(expr, "avg "):
		return "avg"
	case strings.HasPrefix(expr, "min "):
		return "min"
	case strings.HasPrefix(expr, "max "):
		return "max"
	case strings.HasPrefix(expr, "prd "):
		return "product"
	case strings.HasPrefix(expr, "count "):
		return "count"
	default:
		return "unknown"
	}
}

func qEvalLooksLikeVectorDyadic(expr string) bool {
	if strings.ContainsAny(expr, "+-*/") {
		return strings.Contains(expr, "til ") || strings.Contains(expr, " til") || strings.Contains(expr, " 0 ") || strings.Contains(expr, " 1 ")
	}
	for _, op := range []string{">=", "<=", "!=", "<>", "=", ">", "<"} {
		if strings.Contains(expr, op) {
			return true
		}
	}
	return false
}

type qSimpleGroupAggregateQuery struct {
	By         []string
	Aggregates []runtime.FrameAggregateSpec
	Where      *qSimpleFramePredicate
}

type qSimpleFramePredicate struct {
	Column    string
	Op        string
	Value     runtime.Value
	Mode      string
	ValueKind string
}

type qQueryToken struct {
	text  string
	lower string
	start int
	end   int
}

func qGroupAggregateNativeLoweringPass(fn *Function) {
	if fn == nil || fn.Proto == nil {
		return
	}
	uses := qQueryValueUseCounts(fn)
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpCall || !qCallIsQSQLEntrypoint(fn, instr) {
				continue
			}
			query, ok := qCallQueryString(fn, instr)
			if !ok || !qQueryStringHasGroupAggregate(query) {
				continue
			}
			spec, ok := qParseSimpleGroupAggregateQuery(query)
			if !ok {
				continue
			}
			frame := qCallSQLFrameArg(fn, instr)
			if frame == nil {
				continue
			}
			mask := qInsertGroupAggregateMaskArg(fn, block, instr, frame, spec)
			specIdx := len(fn.Proto.Constants)
			fn.Proto.Constants = append(fn.Proto.Constants, qFrameGroupAggregateSpecValue(spec))
			qNopQSQLCallScaffold(instr, uses)
			instr.Op = OpFrameGroupAggregate
			instr.Type = TypeAny
			instr.Args = []*Value{frame, mask}
			instr.Aux = int64(specIdx)
			instr.Aux2 = 0
			functionRemarks(fn).Add("QQueryNativeLowering", "changed", block.ID, instr.ID, OpFrameGroupAggregate,
				"lowered simple q.sql grouped aggregate to FrameGroupAggregate typed runtime kernel op-exit")
		}
	}
}

func qNopQSQLCallScaffold(call *Instr, uses map[int]int) {
	if call == nil {
		return
	}
	var callee, receiver, query *Instr
	if len(call.Args) > 0 && call.Args[0] != nil {
		callee = call.Args[0].Def
		if callee != nil && len(callee.Args) == 1 && callee.Args[0] != nil {
			receiver = callee.Args[0].Def
		}
	}
	if len(call.Args) > 2 && call.Args[2] != nil {
		query = call.Args[2].Def
	}
	qQueryNopIfSingleUse(query, uses)
	qQueryNopIfSingleUse(callee, uses)
	qQueryNopIfSingleUse(receiver, uses)
}

func qParseSimpleGroupAggregateQuery(query string) (qSimpleGroupAggregateQuery, bool) {
	tokens := qLexQueryTokens(query)
	if len(tokens) < 5 || tokens[0].lower != "select" {
		return qSimpleGroupAggregateQuery{}, false
	}
	byIdx, fromIdx := -1, -1
	whereIdx := -1
	for i, tok := range tokens {
		switch tok.lower {
		case "order", "limit", "take", "distinct", "join", "lj", "ij", "aj", "wj", "update", "delete", "exec":
			return qSimpleGroupAggregateQuery{}, false
		case "where":
			if whereIdx >= 0 {
				return qSimpleGroupAggregateQuery{}, false
			}
			whereIdx = i
		case "by":
			if byIdx >= 0 {
				return qSimpleGroupAggregateQuery{}, false
			}
			byIdx = i
		case "from":
			if fromIdx >= 0 {
				return qSimpleGroupAggregateQuery{}, false
			}
			fromIdx = i
		}
	}
	if fromIdx <= 1 {
		return qSimpleGroupAggregateQuery{}, false
	}
	if byIdx >= 0 && (byIdx <= 1 || fromIdx <= byIdx+1) {
		return qSimpleGroupAggregateQuery{}, false
	}
	if byIdx < 0 && fromIdx <= 1 {
		return qSimpleGroupAggregateQuery{}, false
	}
	if whereIdx >= 0 && whereIdx != fromIdx+2 {
		return qSimpleGroupAggregateQuery{}, false
	}
	if whereIdx < 0 && fromIdx+2 != len(tokens) {
		return qSimpleGroupAggregateQuery{}, false
	}
	if whereIdx >= 0 && whereIdx+1 >= len(tokens) {
		return qSimpleGroupAggregateQuery{}, false
	}
	selectEnd := tokens[fromIdx].start
	if byIdx >= 0 {
		selectEnd = tokens[byIdx].start
	}
	selectPart := strings.TrimSpace(query[tokens[0].end:selectEnd])
	var byColumns []string
	if byIdx >= 0 {
		byPart := strings.TrimSpace(query[tokens[byIdx].end:tokens[fromIdx].start])
		parsed, ok := qParseSimpleIdentifierList(byPart)
		if !ok || len(parsed) == 0 {
			return qSimpleGroupAggregateQuery{}, false
		}
		byColumns = parsed
	}
	aggregates, ok := qParseSimpleAggregateList(selectPart)
	if !ok || len(aggregates) == 0 {
		return qSimpleGroupAggregateQuery{}, false
	}
	var where *qSimpleFramePredicate
	if whereIdx >= 0 {
		wherePart := strings.TrimSpace(query[tokens[whereIdx].end:])
		predicate, ok := qParseSimpleFramePredicate(wherePart)
		if !ok {
			return qSimpleGroupAggregateQuery{}, false
		}
		where = &predicate
	}
	return qSimpleGroupAggregateQuery{By: byColumns, Aggregates: aggregates, Where: where}, true
}

func qParseSimpleIdentifierList(text string) ([]string, bool) {
	parts := strings.Split(text, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if !qSimpleIdentifier(name) {
			return nil, false
		}
		out = append(out, name)
	}
	return out, true
}

func qParseSimpleAggregateList(text string) ([]runtime.FrameAggregateSpec, bool) {
	parts := strings.Split(text, ",")
	out := make([]runtime.FrameAggregateSpec, 0, len(parts))
	for _, part := range parts {
		agg, ok := qParseSimpleAggregate(strings.TrimSpace(part))
		if !ok {
			return nil, false
		}
		out = append(out, agg)
	}
	return out, true
}

func qParseSimpleAggregate(text string) (runtime.FrameAggregateSpec, bool) {
	name, expr, ok := strings.Cut(text, ":")
	if !ok {
		return runtime.FrameAggregateSpec{}, false
	}
	name = strings.TrimSpace(name)
	if !qSimpleIdentifier(name) {
		return runtime.FrameAggregateSpec{}, false
	}
	exprText := strings.TrimSpace(expr)
	fields := strings.Fields(exprText)
	if len(fields) < 2 {
		return runtime.FrameAggregateSpec{}, false
	}
	op := strings.ToLower(fields[0])
	operand := strings.TrimSpace(exprText[len(fields[0]):])
	switch op {
	case "count":
		if operand != "i" {
			return runtime.FrameAggregateSpec{}, false
		}
		return runtime.FrameAggregateSpec{Name: name, Op: op}, true
	case "sum", "min", "max", "avg":
		if ident, ok := qParseSimpleAggregateIdentifierExpr(operand); ok {
			return runtime.FrameAggregateSpec{Name: name, Op: op, Column: ident}, true
		}
		if left, binaryOp, right, ok := qParseSimpleAggregateBinaryExpr(operand); ok && (op == "sum" || op == "avg") {
			return runtime.FrameAggregateSpec{Name: name, Op: op, Left: left, Right: right, BinaryOp: binaryOp}, true
		}
		return runtime.FrameAggregateSpec{}, false
	default:
		return runtime.FrameAggregateSpec{}, false
	}
}

func qParseSimpleAggregateIdentifierExpr(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if !qSimpleIdentifier(text) {
		return "", false
	}
	return text, true
}

func qParseSimpleAggregateBinaryExpr(text string) (string, string, string, bool) {
	text = strings.TrimSpace(text)
	for _, op := range []string{"*", "+", "-", "/"} {
		idx := strings.Index(text, op)
		if idx < 0 {
			continue
		}
		if strings.ContainsAny(text[idx+len(op):], "*+-/") {
			return "", "", "", false
		}
		left := strings.TrimSpace(text[:idx])
		right := strings.TrimSpace(text[idx+len(op):])
		if !qSimpleIdentifier(left) || !qSimpleIdentifier(right) {
			return "", "", "", false
		}
		return left, op, right, true
	}
	return "", "", "", false
}

func qSimpleIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		isStart := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
		isRest := isStart || (r >= '0' && r <= '9')
		if i == 0 {
			if !isStart {
				return false
			}
			continue
		}
		if !isRest {
			return false
		}
	}
	return true
}

func qParseSimpleFramePredicate(text string) (qSimpleFramePredicate, bool) {
	text = strings.TrimSpace(text)
	if qSimpleBareBoolPredicateIdentifier(text) {
		return qSimpleFramePredicate{Column: text, Mode: "bool_column"}, true
	}
	for _, op := range []string{">=", "<=", "!=", "<>", "==", "=", ">", "<"} {
		idx := strings.Index(text, op)
		if idx < 0 {
			continue
		}
		if strings.Contains(text[idx+len(op):], op) {
			return qSimpleFramePredicate{}, false
		}
		column := strings.TrimSpace(text[:idx])
		rhsText := strings.TrimSpace(text[idx+len(op):])
		if !qSimpleIdentifier(column) {
			return qSimpleFramePredicate{}, false
		}
		if op == "=" {
			op = "=="
		} else if op == "<>" {
			op = "!="
		}
		value, valueKind, ok := qParseSimpleFramePredicateValue(rhsText)
		if !ok {
			return qSimpleFramePredicate{}, false
		}
		return qSimpleFramePredicate{Column: column, Op: op, Value: value, ValueKind: valueKind}, true
	}
	return qSimpleFramePredicate{}, false
}

func qSimpleBareBoolPredicateIdentifier(text string) bool {
	if !qSimpleIdentifier(text) {
		return false
	}
	switch strings.ToLower(text) {
	case "true", "false", "null", "nil", "and", "or", "not":
		return false
	default:
		return true
	}
}

func qParseSimpleFramePredicateValue(text string) (runtime.Value, string, bool) {
	text = strings.TrimSpace(text)
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		unquoted, err := strconv.Unquote(text)
		if err != nil {
			return runtime.NilValue(), "", false
		}
		return runtime.StringValue(unquoted), "literal", true
	}
	if len(text) >= 2 && text[0] == '`' && qSimpleSymbolLiteral(text[1:]) {
		return runtime.StringValue(text[1:]), "literal", true
	}
	switch strings.ToLower(text) {
	case "true":
		return runtime.BoolValue(true), "", true
	case "false":
		return runtime.BoolValue(false), "", true
	}
	if strings.ContainsAny(text, ".eE") {
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return runtime.NilValue(), "", false
		}
		return runtime.FloatValue(value), "", true
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return runtime.NilValue(), "", false
	}
	return runtime.IntValue(value), "", true
}

func qSimpleSymbolLiteral(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if !isLetter && !isDigit && r != '_' && r != '.' && r != '-' {
			return false
		}
	}
	return true
}

func qLexQueryTokens(query string) []qQueryToken {
	var tokens []qQueryToken
	start := -1
	for i, r := range query {
		isIdentStart := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
		isIdentRest := isIdentStart || (r >= '0' && r <= '9')
		isIdent := isIdentStart || (start >= 0 && isIdentRest)
		if isIdent {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			text := query[start:i]
			tokens = append(tokens, qQueryToken{text: text, lower: strings.ToLower(text), start: start, end: i})
			start = -1
		}
	}
	if start >= 0 {
		text := query[start:]
		tokens = append(tokens, qQueryToken{text: text, lower: strings.ToLower(text), start: start, end: len(query)})
	}
	return tokens
}

func qInsertGroupAggregateMaskArg(fn *Function, block *Block, before *Instr, frame *Value, spec qSimpleGroupAggregateQuery) *Value {
	if spec.Where == nil {
		return qInsertConstNilBefore(fn, block, before)
	}
	maskSpecIdx := len(fn.Proto.Constants)
	fn.Proto.Constants = append(fn.Proto.Constants, qFrameMaskSpecValue(*spec.Where))
	maskInstr := &Instr{ID: fn.newValueID(), Op: OpFrameMask, Type: TypeAny, Args: []*Value{frame}, Aux: int64(maskSpecIdx), Block: block}
	for i, instr := range block.Instrs {
		if instr == before {
			block.Instrs = append(block.Instrs[:i], append([]*Instr{maskInstr}, block.Instrs[i:]...)...)
			return maskInstr.Value()
		}
	}
	block.Instrs = append(block.Instrs, maskInstr)
	return maskInstr.Value()
}

func qFrameMaskSpecValue(predicate qSimpleFramePredicate) runtime.Value {
	tbl := runtime.NewTable()
	tbl.RawSetString("column", runtime.StringValue(predicate.Column))
	if predicate.Mode != "" {
		tbl.RawSetString("mode", runtime.StringValue(predicate.Mode))
	}
	if predicate.Op != "" {
		tbl.RawSetString("op", runtime.StringValue(predicate.Op))
	}
	if !predicate.Value.IsNil() {
		tbl.RawSetString("value", predicate.Value)
	}
	if predicate.ValueKind != "" {
		tbl.RawSetString("value_kind", runtime.StringValue(predicate.ValueKind))
	}
	return runtime.TableValue(tbl)
}

func qFrameGroupAggregateSpecValue(spec qSimpleGroupAggregateQuery) runtime.Value {
	tbl := runtime.NewTable()
	if len(spec.By) == 1 {
		tbl.RawSetString("by", runtime.StringValue(spec.By[0]))
	} else {
		by := runtime.NewAppendArrayTable(len(spec.By))
		for i, name := range spec.By {
			by.RawSetInt(int64(i+1), runtime.StringValue(name))
		}
		tbl.RawSetString("by", runtime.TableValue(by))
	}
	aggs := runtime.NewAppendArrayTable(len(spec.Aggregates))
	for i, agg := range spec.Aggregates {
		row := runtime.NewTable()
		row.RawSetString("name", runtime.StringValue(agg.Name))
		row.RawSetString("op", runtime.StringValue(agg.Op))
		if agg.Column != "" {
			row.RawSetString("column", runtime.StringValue(agg.Column))
		}
		if agg.Left != "" {
			row.RawSetString("left", runtime.StringValue(agg.Left))
		}
		if agg.Right != "" {
			row.RawSetString("right", runtime.StringValue(agg.Right))
		}
		if agg.BinaryOp != "" {
			row.RawSetString("binary_op", runtime.StringValue(agg.BinaryOp))
		}
		aggs.RawSetInt(int64(i+1), runtime.TableValue(row))
	}
	tbl.RawSetString("aggregates", runtime.TableValue(aggs))
	return runtime.TableValue(tbl)
}

func qInsertConstNilBefore(fn *Function, block *Block, before *Instr) *Value {
	nilInstr := &Instr{ID: fn.newValueID(), Op: OpConstNil, Type: TypeNil, Block: block}
	for i, instr := range block.Instrs {
		if instr == before {
			block.Instrs = append(block.Instrs[:i], append([]*Instr{nilInstr}, block.Instrs[i:]...)...)
			return nilInstr.Value()
		}
	}
	block.Instrs = append(block.Instrs, nilInstr)
	return nilInstr.Value()
}

func qVectorWhereReduceLoweringPass(fn *Function, uses map[int]int) {
	if fn == nil {
		return
	}
	for _, path := range DetectQVectorReduceHotPaths(fn) {
		if path.Reduce == nil {
			continue
		}
		if path.Gather != nil {
			if len(path.Gather.Args) != 2 {
				qVectorGatherReduceLoweringFallbackRemark(fn, path, qVectorWhereReduceFallbackUnsupportedInput)
				continue
			}
			if uses[path.Gather.ID] != 1 {
				qVectorGatherReduceLoweringFallbackRemark(fn, path, qVectorGatherReduceFallbackSharedGather)
				continue
			}
			reduce := path.Reduce
			gather := path.Gather
			reduce.Op = OpQVectorGatherReduce
			reduce.Type = TypeAny
			reduce.Args = append([]*Value(nil), gather.Args...)
			reduce.Aux2 = 0
			qQueryNop(gather)
			if reduce.Block != nil {
				functionRemarks(fn).Add("QQueryNativeLowering", "changed", reduce.Block.ID, reduce.ID, OpQVectorGatherReduce,
					"lowered q vector hot path shape gather/vector-reduce to fused typed runtime kernel op-exit")
			}
			continue
		}
		if path.Where == nil {
			qVectorWhereReduceLoweringFallbackRemark(fn, path, qVectorWhereReduceFallbackUnsupportedInput)
			continue
		}
		if len(path.Where.Args) != 3 {
			qVectorWhereReduceLoweringFallbackRemark(fn, path, qVectorWhereReduceFallbackBadWhereArgCount)
			continue
		}
		if uses[path.Where.ID] != 1 {
			qVectorWhereReduceLoweringFallbackRemark(fn, path, qVectorWhereReduceFallbackSharedWhere)
			continue
		}
		reduce := path.Reduce
		where := path.Where
		reduce.Op = OpQVectorWhereReduce
		reduce.Type = TypeAny
		reduce.Args = append([]*Value(nil), where.Args...)
		reduce.Aux2 = 0
		qQueryNop(where)
		if reduce.Block != nil {
			functionRemarks(fn).Add("QQueryNativeLowering", "changed", reduce.Block.ID, reduce.ID, OpQVectorWhereReduce,
				fmt.Sprintf("lowered q vector hot path shape %s to fused typed runtime kernel op-exit", qVectorWhereReduceShape(reduce)))
		}
	}
}

func qVectorGatherReduceLoweringFallbackRemark(fn *Function, path QVectorReduceHotPath, reason string) {
	if reason == "" {
		reason = "unsupported_shape"
	}
	blockID, valueID := 0, 0
	op := OpVectorReduce
	if path.Reduce != nil {
		valueID = path.Reduce.ID
		op = path.Reduce.Op
		if path.Reduce.Block != nil {
			blockID = path.Reduce.Block.ID
		}
	}
	shape := path.Shape()
	functionRemarks(fn).AddWithFields("QVectorNativeLowering", "missed", blockID, valueID, op,
		fmt.Sprintf("kernel=QVectorGatherReduce reason_code=%s shape=%s; q vector gather-reduce remains on primitive runtime fallback",
			reason, shape),
		qLoweringFallbackRemarkFields("QVectorGatherReduce", reason, shape))
}

func qVectorWhereReduceLoweringFallbackRemark(fn *Function, path QVectorReduceHotPath, reason string) {
	if reason == "" {
		reason = "unsupported_shape"
	}
	blockID, valueID := 0, 0
	op := OpVectorReduce
	if path.Reduce != nil {
		valueID = path.Reduce.ID
		op = path.Reduce.Op
		if path.Reduce.Block != nil {
			blockID = path.Reduce.Block.ID
		}
	}
	shape := qVectorReduceLoweringFallbackShape(path)
	functionRemarks(fn).AddWithFields("QVectorNativeLowering", "missed", blockID, valueID, op,
		fmt.Sprintf("reason_code=%s shape=%s; q vector where-reduce remains on primitive runtime fallback",
			reason, shape),
		qLoweringFallbackRemarkFields("QVectorWhereReduce", reason, shape))
}

func qVectorReduceLoweringFallbackShape(path QVectorReduceHotPath) string {
	if path.Where == nil {
		return path.Shape()
	}
	if len(path.Where.Args) != 3 {
		return "vector-where/vector-reduce"
	}
	return qVectorWhereReduceShape(path.Where)
}

func qQueryLoweringFallbackRemark(fn *Function, path QQueryHotPath, reason string) {
	if reason == "" {
		reason = qQueryLoweringFallbackUnsupportedSpec
	}
	blockID, valueID := 0, 0
	if path.ResultColumn != nil {
		valueID = path.ResultColumn.ID
		if path.ResultColumn.Block != nil {
			blockID = path.ResultColumn.Block.ID
		}
	}
	shape := path.Shape()
	functionRemarks(fn).AddWithFields("QQueryNativeLowering", "missed", blockID, valueID, OpFrameColumn,
		fmt.Sprintf("reason_code=%s shape=%s compare=%s; q query hot path remains on primitive fallback",
			reason, shape, qQueryHotPathPredicateName(path)),
		qLoweringFallbackRemarkFields("QFrameSelectColumn", reason, shape))
}

func qGroupAggregateFallbackRemarkPass(fn *Function) {
	if fn == nil {
		return
	}
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpCall || !qCallIsQQueryEntrypoint(fn, instr) {
				continue
			}
			query, ok := qCallQueryString(fn, instr)
			if !ok || !qQueryStringHasGroupAggregate(query) {
				continue
			}
			qGroupAggregateFallbackRemark(fn, instr, qGroupAggregateQueryShape(query))
		}
	}
}

func qGroupAggregateFallbackRemark(fn *Function, call *Instr, shape string) {
	if shape == "" {
		shape = "group/aggregate"
	}
	blockID, valueID := 0, 0
	if call != nil {
		valueID = call.ID
		if call.Block != nil {
			blockID = call.Block.ID
		}
	}
	functionRemarks(fn).AddWithFields("QQueryNativeLowering", "missed", blockID, valueID, OpCall,
		fmt.Sprintf("kernel=QGroupAggregate reason_code=%s shape=%s; grouped q aggregate remains on opaque call fallback",
			qQueryLoweringFallbackGroupAggregateCall, shape),
		qLoweringFallbackRemarkFields("QGroupAggregate", qQueryLoweringFallbackGroupAggregateCall, shape))
}

func qJoinFallbackRemarkPass(fn *Function) {
	if fn == nil {
		return
	}
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpCall || !qCallIsQQueryEntrypoint(fn, instr) {
				continue
			}
			query, ok := qCallQueryString(fn, instr)
			if !ok || !qQueryStringHasJoin(query) {
				continue
			}
			qJoinFallbackRemark(fn, instr, qJoinQueryShape(query))
		}
	}
}

func qJoinFallbackRemark(fn *Function, call *Instr, shape string) {
	if shape == "" {
		shape = "join/inner"
	}
	blockID, valueID := 0, 0
	if call != nil {
		valueID = call.ID
		if call.Block != nil {
			blockID = call.Block.ID
		}
	}
	functionRemarks(fn).AddWithFields("QQueryNativeLowering", "missed", blockID, valueID, OpCall,
		fmt.Sprintf("kernel=QJoin reason_code=%s shape=%s; joined q query remains on opaque call fallback",
			qQueryLoweringFallbackJoinCall, shape),
		qLoweringFallbackRemarkFields("QJoin", qQueryLoweringFallbackJoinCall, shape))
}

func qLoweringFallbackRemarkFields(kernel, reasonCode, shape string) map[string]string {
	return map[string]string{
		"kind":          "fallback",
		"kernel":        kernel,
		"shape":         shape,
		"reason_family": "lowering",
		"reason_code":   reasonCode,
		"route":         "lowering",
		"outcome":       "fallback",
	}
}

func qCallIsQQueryEntrypoint(fn *Function, call *Instr) bool {
	if fn == nil || call == nil || len(call.Args) == 0 || call.Args[0] == nil {
		return false
	}
	callee := call.Args[0].Def
	if callee == nil || callee.Op != OpGetField || len(callee.Args) != 1 {
		return false
	}
	field, ok := qConstStringAt(fn, int(callee.Aux))
	if !ok || (field != "sql" && field != "select") {
		return false
	}
	receiver := callee.Args[0].Def
	if receiver == nil || receiver.Op != OpGetGlobal {
		return false
	}
	global, ok := qConstStringAt(fn, int(receiver.Aux))
	return ok && global == "q"
}

func qCallIsQSQLEntrypoint(fn *Function, call *Instr) bool {
	if !qCallIsQQueryEntrypoint(fn, call) || call == nil || len(call.Args) == 0 || call.Args[0] == nil {
		return false
	}
	callee := call.Args[0].Def
	if callee == nil {
		return false
	}
	field, ok := qConstStringAt(fn, int(callee.Aux))
	return ok && field == "sql"
}

func qCallIsQEvalEntrypoint(fn *Function, call *Instr) bool {
	if fn == nil || call == nil || len(call.Args) == 0 || call.Args[0] == nil {
		return false
	}
	callee := call.Args[0].Def
	if callee == nil || callee.Op != OpGetField || len(callee.Args) != 1 {
		return false
	}
	field, ok := qConstStringAt(fn, int(callee.Aux))
	if !ok || field != "eval" {
		return false
	}
	receiver := callee.Args[0].Def
	if receiver == nil || receiver.Op != OpGetGlobal {
		return false
	}
	global, ok := qConstStringAt(fn, int(receiver.Aux))
	return ok && global == "q"
}

func qCallEvalSourceString(fn *Function, call *Instr) (string, bool) {
	if fn == nil || call == nil || len(call.Args) != 2 {
		return "", false
	}
	arg := call.Args[1]
	if arg == nil || arg.Def == nil || arg.Def.Op != OpConstString {
		return "", false
	}
	return qConstStringAt(fn, int(arg.Def.Aux))
}

func qCallSQLFrameArg(fn *Function, call *Instr) *Value {
	if fn == nil || call == nil || len(call.Args) != 3 {
		return nil
	}
	queryArg := call.Args[2]
	if queryArg == nil || queryArg.Def == nil || queryArg.Def.Op != OpConstString {
		return nil
	}
	query, ok := qConstStringAt(fn, int(queryArg.Def.Aux))
	if !ok || !qQueryStringLooksLikeQuery(query) {
		return nil
	}
	frameArg := call.Args[1]
	if frameArg == nil || frameArg.Def == nil && frameArg.ID < 0 {
		return nil
	}
	return frameArg
}

func qCallQueryString(fn *Function, call *Instr) (string, bool) {
	if fn == nil || call == nil {
		return "", false
	}
	for _, arg := range call.Args[1:] {
		if arg == nil || arg.Def == nil || arg.Def.Op != OpConstString {
			continue
		}
		if query, ok := qConstStringAt(fn, int(arg.Def.Aux)); ok && qQueryStringLooksLikeQuery(query) {
			return query, true
		}
	}
	return "", false
}

func qConstStringAt(fn *Function, idx int) (string, bool) {
	if fn == nil || fn.Proto == nil || idx < 0 || idx >= len(fn.Proto.Constants) {
		return "", false
	}
	value := fn.Proto.Constants[idx]
	if !value.IsString() {
		return "", false
	}
	return value.Str(), true
}

func qQueryStringLooksLikeQuery(query string) bool {
	tokens := qQueryStringTokens(query)
	if len(tokens) == 0 {
		return false
	}
	switch tokens[0] {
	case "select", "exec", "update", "delete":
		return true
	default:
		return false
	}
}

func qQueryStringHasGroupAggregate(query string) bool {
	tokens := qQueryStringTokens(query)
	if len(tokens) == 0 || tokens[0] != "select" {
		return false
	}
	hasAggregate := false
	for _, token := range tokens[1:] {
		if token == "by" {
			continue
		}
		if qQueryAggregateToken(token) {
			hasAggregate = true
		}
	}
	return hasAggregate
}

func qQueryStringHasJoin(query string) bool {
	kind, ok := qQueryJoinKind(qQueryStringTokens(query))
	return ok && kind != ""
}

func qGroupAggregateQueryShape(query string) string {
	tokens := qQueryStringTokens(query)
	hasJoin, hasWhere, hasOrder := false, false, false
	statement := ""
	for i, token := range tokens {
		if i == 0 {
			switch token {
			case "select", "exec", "update", "delete":
				statement = token
			}
		}
		switch token {
		case "join", "lj", "ij", "aj", "wj", "left", "right", "inner", "outer", "cross":
			hasJoin = true
		case "where":
			hasWhere = true
		case "order":
			hasOrder = true
		}
	}
	parts := make([]string, 0, 5)
	if statement != "" {
		parts = append(parts, statement)
	}
	if hasJoin {
		parts = append(parts, "join")
	}
	if hasWhere {
		parts = append(parts, "where")
	}
	parts = append(parts, "group", "aggregate")
	shape := strings.Join(parts, "/")
	if hasOrder {
		shape += "/order"
	}
	return shape
}

func qJoinQueryShape(query string) string {
	tokens := qQueryStringTokens(query)
	kind, ok := qQueryJoinKind(tokens)
	if !ok || kind == "" {
		kind = "inner"
	}
	hasWhere, hasOrder := false, false
	for _, token := range tokens {
		switch token {
		case "where":
			hasWhere = true
		case "order":
			hasOrder = true
		}
	}
	shape := "join/" + kind
	if hasWhere {
		shape = "where/" + shape
	}
	if hasOrder {
		shape += "/order"
	}
	return shape
}

func qQueryJoinKind(tokens []string) (string, bool) {
	for i, token := range tokens {
		switch token {
		case "ij":
			return "inner", true
		case "lj":
			return "left", true
		case "uj":
			return "union", true
		case "pj":
			return "plus", true
		case "aj":
			return "asof", true
		case "aj0":
			return "asof0", true
		case "ajf":
			return "asof_fill", true
		case "ajf0":
			return "asof_fill0", true
		case "wj":
			return "window", true
		case "wj1":
			return "window1", true
		case "join":
			if i > 0 {
				switch tokens[i-1] {
				case "inner":
					return "inner", true
				case "left":
					return "left", true
				case "asof":
					return "asof", true
				}
			}
			return "inner", true
		}
	}
	return "", false
}

func qQueryAggregateToken(token string) bool {
	switch token {
	case "count", "sum", "avg", "min", "max", "var", "dev", "med", "wavg":
		return true
	default:
		return false
	}
}

func qQueryStringTokens(query string) []string {
	var tokens []string
	lower := strings.ToLower(query)
	start := -1
	for i, r := range lower {
		isIdentStart := (r >= 'a' && r <= 'z') || r == '_'
		isIdentRest := isIdentStart || (r >= '0' && r <= '9')
		isIdent := isIdentStart || (start >= 0 && isIdentRest)
		if isIdent {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			tokens = append(tokens, lower[start:i])
			start = -1
		}
	}
	if start >= 0 {
		tokens = append(tokens, lower[start:])
	}
	return tokens
}

func qQueryValueUseCounts(fn *Function) map[int]int {
	uses := make(map[int]int)
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			for _, arg := range instr.Args {
				if arg != nil {
					uses[arg.ID]++
				}
			}
		}
	}
	return uses
}

func qQueryHotPathSingleUse(path QQueryHotPath, uses map[int]int) bool {
	for _, instr := range []*Instr{path.RowOrder, path.RowGather, path.RowSlice, path.Project} {
		if instr != nil && uses[instr.ID] != 1 {
			return false
		}
	}
	filterUses := 1
	if path.RowOrder != nil && path.RowGather != nil && path.RowOrder.ID != path.RowGather.ID {
		filterUses = 2
	}
	if path.Filter != nil && uses[path.Filter.ID] != filterUses {
		return false
	}
	return true
}

func qQueryFrameSelectColumnSpec(fn *Function, path QQueryHotPath) (QFrameSelectColumnSpec, []*Value, string, bool) {
	if fn == nil || fn.Proto == nil || path.Project == nil || path.ResultColumn == nil {
		return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackMissingProto, false
	}
	if path.ResultColumn.Aux < 0 || path.ResultColumn.Aux >= int64(len(fn.Proto.Constants)) ||
		path.Project.Aux < 0 || path.Project.Aux >= int64(len(fn.Proto.Constants)) {
		return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackBadProjectOrResultConst, false
	}
	spec := QFrameSelectColumnSpec{
		Shape:             path.Shape(),
		SourceColumnConst: -1,
		MaskSpecConst:     -1,
		MaskRoot:          -1,
		RowMode:           QFrameSelectColumnRowsNone,
		RowOrderConst:     -1,
		DynamicArgRole:    QFrameSelectColumnArgNone,
		ProjectConst:      int(path.Project.Aux),
		ResultColumnConst: int(path.ResultColumn.Aux),
	}
	frameArg := qQueryHotPathFrameArg(path)
	if frameArg == nil {
		return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackMissingProto, false
	}
	args := []*Value{frameArg}
	if path.Compare != nil {
		if path.SourceColumn == nil || path.SourceColumn.Aux < 0 || path.SourceColumn.Aux >= int64(len(fn.Proto.Constants)) {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackBadSourceColumnConst, false
		}
		rhs := qQueryCompareRHS(path.Compare, path.SourceColumn)
		if rhs == nil {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackMissingCompareRHS, false
		}
		spec.SourceColumnConst = int(path.SourceColumn.Aux)
		spec.CompareOp = runtime.DenseArrayBinaryOp(path.Compare.Aux)
		if rhsConst, ok := qQueryConstRuntimeValue(rhs); ok {
			spec.CompareRHSConst = rhsConst
			spec.HasCompareRHSConst = true
		} else if spec.DynamicArgRole == QFrameSelectColumnArgNone {
			spec.DynamicArgRole = QFrameSelectColumnArgCompareRHS
			args = append(args, rhs)
		} else {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackTooManyDynamicArgs, false
		}
	} else if path.Mask != nil {
		if path.Mask.Aux < 0 || path.Mask.Aux >= int64(len(fn.Proto.Constants)) {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackBadMaskSpecConst, false
		}
		spec.MaskSpecConst = int(path.Mask.Aux)
	} else if path.MaskCombine != nil {
		root, reason, ok := qQueryFrameMaskTermSpec(fn, &spec, path.MaskCombine.Value(), &args)
		if !ok {
			return QFrameSelectColumnSpec{}, nil, reason, false
		}
		spec.MaskRoot = root
	}
	switch {
	case path.RowOrder != nil && path.RowGather != nil:
		if path.RowOrder.Aux < 0 || path.RowOrder.Aux >= int64(len(fn.Proto.Constants)) {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackBadOrderConst, false
		}
		spec.RowMode = QFrameSelectColumnRowsOrderGather
		spec.RowOrderConst = int(path.RowOrder.Aux)
	case path.RowGather != nil:
		spec.RowMode = QFrameSelectColumnRowsGather
		if len(path.RowGather.Args) != 2 || path.RowGather.Args[1] == nil {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackMissingRowValue, false
		}
		if qQueryOpaqueConst(path.RowGather.Args[1]) {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackOpaqueRowConst, false
		}
		if rowConst, ok := qQueryConstRuntimeValue(path.RowGather.Args[1]); ok {
			spec.RowValueConst = rowConst
			spec.HasRowValueConst = true
		} else if spec.DynamicArgRole == QFrameSelectColumnArgNone {
			spec.DynamicArgRole = QFrameSelectColumnArgRowValue
			args = append(args, path.RowGather.Args[1])
		} else {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackTooManyDynamicArgs, false
		}
	case path.RowSlice != nil:
		spec.RowMode = QFrameSelectColumnRowsSlice
		if len(path.RowSlice.Args) != 2 || path.RowSlice.Args[1] == nil {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackMissingRowValue, false
		}
		if qQueryOpaqueConst(path.RowSlice.Args[1]) {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackOpaqueRowConst, false
		}
		if rowConst, ok := qQueryConstRuntimeValue(path.RowSlice.Args[1]); ok {
			spec.RowValueConst = rowConst
			spec.HasRowValueConst = true
		} else if spec.DynamicArgRole == QFrameSelectColumnArgNone {
			spec.DynamicArgRole = QFrameSelectColumnArgRowValue
			args = append(args, path.RowSlice.Args[1])
		} else {
			return QFrameSelectColumnSpec{}, nil, qQueryLoweringFallbackTooManyDynamicArgs, false
		}
	}
	return spec, args, "", true
}

func qQueryHotPathFrameArg(path QQueryHotPath) *Value {
	switch {
	case path.Filter != nil && len(path.Filter.Args) >= 1:
		return path.Filter.Args[0]
	case path.RowGather != nil && len(path.RowGather.Args) >= 1:
		return path.RowGather.Args[0]
	case path.RowSlice != nil && len(path.RowSlice.Args) >= 1:
		return path.RowSlice.Args[0]
	case path.Project != nil && len(path.Project.Args) >= 1:
		return path.Project.Args[0]
	default:
		return nil
	}
}

func qQueryOpaqueConst(value *Value) bool {
	return value != nil && value.Def != nil && value.Def.Op == OpConstNil && value.Def.Type == TypeAny
}

func qQueryFrameMaskTermSpec(fn *Function, spec *QFrameSelectColumnSpec, value *Value, args *[]*Value) (int, string, bool) {
	if fn == nil || fn.Proto == nil || spec == nil || value == nil || args == nil {
		return -1, qQueryLoweringFallbackMissingPredicate, false
	}
	if mask := valueDef(value, OpFrameMask); mask != nil {
		if mask.Aux < 0 || mask.Aux >= int64(len(fn.Proto.Constants)) {
			return -1, qQueryLoweringFallbackBadMaskSpecConst, false
		}
		return qFrameMaskAppendTerm(spec, QFrameMaskTermSpec{
			Kind:              QFrameMaskTermFrameMask,
			MaskSpecConst:     int(mask.Aux),
			SourceColumnConst: -1,
			LeftTerm:          -1,
			RightTerm:         -1,
		}), "", true
	}
	if compare := valueDef(value, OpVectorCompare); compare != nil {
		sourceColumn := qQueryCompareColumn(compare)
		if sourceColumn == nil || sourceColumn.Aux < 0 || sourceColumn.Aux >= int64(len(fn.Proto.Constants)) {
			return -1, qQueryLoweringFallbackBadSourceColumnConst, false
		}
		rhs := qQueryCompareRHS(compare, sourceColumn)
		if rhs == nil {
			return -1, qQueryLoweringFallbackMissingCompareRHS, false
		}
		term := QFrameMaskTermSpec{
			Kind:              QFrameMaskTermCompare,
			SourceColumnConst: int(sourceColumn.Aux),
			MaskSpecConst:     -1,
			CompareOp:         runtime.DenseArrayBinaryOp(compare.Aux),
			LeftTerm:          -1,
			RightTerm:         -1,
		}
		if rhsConst, ok := qQueryConstRuntimeValue(rhs); ok {
			term.CompareRHSConst = rhsConst
			term.HasCompareRHSConst = true
		} else if spec.DynamicArgRole == QFrameSelectColumnArgNone {
			term.DynamicCompareRHS = true
			spec.DynamicArgRole = QFrameSelectColumnArgCompareRHS
			*args = append(*args, rhs)
		} else {
			return -1, qQueryLoweringFallbackTooManyDynamicArgs, false
		}
		return qFrameMaskAppendTerm(spec, term), "", true
	}
	if combine := valueDef(value, OpVectorMask); combine != nil {
		if len(combine.Args) != 2 {
			return -1, qQueryLoweringFallbackMissingPredicate, false
		}
		left, reason, ok := qQueryFrameMaskTermSpec(fn, spec, combine.Args[0], args)
		if !ok {
			return -1, reason, false
		}
		right, reason, ok := qQueryFrameMaskTermSpec(fn, spec, combine.Args[1], args)
		if !ok {
			return -1, reason, false
		}
		return qFrameMaskAppendTerm(spec, QFrameMaskTermSpec{
			Kind:              QFrameMaskTermCombine,
			SourceColumnConst: -1,
			MaskSpecConst:     -1,
			CombineOp:         runtime.DenseArrayMaskOp(combine.Aux),
			LeftTerm:          left,
			RightTerm:         right,
		}), "", true
	}
	return -1, qQueryLoweringFallbackMissingPredicate, false
}

func qVectorWherePredicate(value *Value) (*Instr, *Instr, *Instr) {
	if compare := valueDef(value, OpVectorCompare); compare != nil {
		return compare, nil, nil
	}
	if mask := valueDef(value, OpFrameMask); mask != nil {
		return nil, mask, nil
	}
	if combine := valueDef(value, OpVectorMask); combine != nil {
		return nil, nil, combine
	}
	return nil, nil, nil
}

func qVectorWhereHotPath(instr *Instr) QVectorWhereHotPath {
	if instr == nil || instr.Op != OpVectorWhere || len(instr.Args) != 3 {
		return QVectorWhereHotPath{}
	}
	compare, mask, maskCombine := qVectorWherePredicate(instr.Args[0])
	if compare == nil && mask == nil && maskCombine == nil {
		return QVectorWhereHotPath{}
	}
	sourceColumn := (*Instr)(nil)
	if compare != nil {
		sourceColumn = qQueryCompareColumn(compare)
		if sourceColumn == nil {
			return QVectorWhereHotPath{}
		}
	}
	return QVectorWhereHotPath{
		SourceColumn: sourceColumn,
		Compare:      compare,
		Mask:         mask,
		MaskCombine:  maskCombine,
		TrueColumn:   valueDef(instr.Args[1], OpFrameColumn),
		FalseColumn:  valueDef(instr.Args[2], OpFrameColumn),
		Where:        instr,
	}
}

func qVectorWhereReduceShape(instr *Instr) string {
	if instr == nil || len(instr.Args) != 3 {
		return "vector-where/vector-reduce"
	}
	return qVectorWherePredicateValueName(instr.Args[0]) + "/vector-where/vector-reduce"
}

func qVectorWherePredicateValueName(value *Value) string {
	compare, mask, maskCombine := qVectorWherePredicate(value)
	switch {
	case compare != nil:
		return "compare"
	case mask != nil:
		return "mask"
	case maskCombine != nil:
		return "mask-combine"
	default:
		return "vector"
	}
}

func qVectorWherePredicateValueDetail(value *Value) string {
	compare, mask, maskCombine := qVectorWherePredicate(value)
	switch {
	case compare != nil:
		return "compare " + qDenseArrayCompareOpName(runtime.DenseArrayBinaryOp(compare.Aux))
	case mask != nil:
		return "mask"
	case maskCombine != nil:
		return "mask-combine " + qDenseArrayMaskOpName(runtime.DenseArrayMaskOp(maskCombine.Aux))
	default:
		return "vector"
	}
}

func qVectorReduceHotPath(instr *Instr) QVectorReduceHotPath {
	if instr == nil || instr.Op != OpVectorReduce || len(instr.Args) != 1 {
		return QVectorReduceHotPath{}
	}
	arg := instr.Args[0]
	return QVectorReduceHotPath{
		SourceColumn: valueDef(arg, OpFrameColumn),
		Gather:       valueDef(arg, OpVectorGather),
		Where:        valueDef(arg, OpVectorWhere),
		Reduce:       instr,
	}
}

func qFrameMaskAppendTerm(spec *QFrameSelectColumnSpec, term QFrameMaskTermSpec) int {
	spec.MaskTerms = append(spec.MaskTerms, term)
	return len(spec.MaskTerms) - 1
}

func qQueryMaskCombineUsesFrame(frame *Value, mask *Instr) bool {
	if frame == nil || mask == nil || mask.Op != OpVectorMask || len(mask.Args) != 2 {
		return false
	}
	for _, arg := range mask.Args {
		if !qQueryMaskValueUsesFrame(frame, arg) {
			return false
		}
	}
	return true
}

func qQueryMaskValueUsesFrame(frame *Value, value *Value) bool {
	instr := valueDef(value, OpFrameMask)
	if instr != nil {
		return len(instr.Args) == 1 && instr.Args[0] != nil && instr.Args[0].ID == frame.ID
	}
	instr = valueDef(value, OpVectorCompare)
	if instr != nil {
		sourceColumn := qQueryCompareColumn(instr)
		return sourceColumn != nil &&
			len(sourceColumn.Args) == 1 &&
			sourceColumn.Args[0] != nil &&
			sourceColumn.Args[0].ID == frame.ID
	}
	instr = valueDef(value, OpVectorMask)
	if instr != nil {
		return qQueryMaskCombineUsesFrame(frame, instr)
	}
	return false
}

func qQueryConstRuntimeValue(value *Value) (runtime.Value, bool) {
	if value == nil || value.Def == nil {
		return runtime.NilValue(), false
	}
	switch value.Def.Op {
	case OpConstInt:
		return runtime.IntValue(value.Def.Aux), true
	case OpConstFloat:
		return runtime.FloatValue(math.Float64frombits(uint64(value.Def.Aux))), true
	case OpConstBool:
		return runtime.BoolValue(value.Def.Aux != 0), true
	default:
		return runtime.NilValue(), false
	}
}

func qQueryCompareRHS(compare, sourceColumn *Instr) *Value {
	if compare == nil || sourceColumn == nil {
		return nil
	}
	for _, arg := range compare.Args {
		if arg == nil || arg.ID == sourceColumn.ID {
			continue
		}
		return arg
	}
	return nil
}

func qQueryNop(instr *Instr) {
	if instr == nil {
		return
	}
	instr.Op = OpNop
	instr.Type = TypeUnknown
	instr.Args = nil
	instr.Aux = 0
	instr.Aux2 = 0
}

func qQueryNopPredicateIfSingleUse(path QQueryHotPath, uses map[int]int) {
	switch {
	case path.Compare != nil:
		qQueryNopCompareIfSingleUse(path.Compare, uses)
	case path.Mask != nil:
		qQueryNopIfSingleUse(path.Mask, uses)
	case path.MaskCombine != nil:
		qQueryNopMaskTreeIfSingleUse(path.MaskCombine, uses)
	}
}

func qQueryNopMaskTreeIfSingleUse(instr *Instr, uses map[int]int) {
	if instr == nil || uses[instr.ID] != 1 {
		return
	}
	args := append([]*Value(nil), instr.Args...)
	for _, arg := range args {
		if child := valueDef(arg, OpVectorMask); child != nil {
			qQueryNopMaskTreeIfSingleUse(child, uses)
			continue
		}
		if child := valueDef(arg, OpVectorCompare); child != nil {
			qQueryNopCompareIfSingleUse(child, uses)
			continue
		}
		if child := valueDef(arg, OpFrameMask); child != nil {
			qQueryNopIfSingleUse(child, uses)
		}
	}
	qQueryNop(instr)
}

func qQueryNopCompareIfSingleUse(compare *Instr, uses map[int]int) {
	if compare == nil || uses[compare.ID] != 1 {
		return
	}
	qQueryNopIfSingleUse(qQueryCompareColumn(compare), uses)
	qQueryNop(compare)
}

func qQueryNopIfSingleUse(instr *Instr, uses map[int]int) {
	if instr != nil && uses[instr.ID] == 1 {
		qQueryNop(instr)
	}
}

func formatQQueryHotPaths(paths []QQueryHotPath) string {
	if len(paths) == 0 {
		return "0 primitive pipeline(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d primitive pipeline(s)\n", len(paths))
	if counts := CountQQueryHotPathShapes(paths); len(counts) > 0 {
		fmt.Fprintf(&b, "  shapes: %s\n", formatQQueryHotPathShapeCounts(counts))
	}
	for i, path := range paths {
		fmt.Fprintf(&b, "  [%d] shape=%s compare=%s", i, path.Shape(), qQueryHotPathPredicateName(path))
		if path.Mask != nil {
			fmt.Fprintf(&b, " mask_aux=%d", path.Mask.Aux)
		}
		if path.RowOrder != nil {
			fmt.Fprintf(&b, " order_aux=%d", path.RowOrder.Aux)
		}
		if path.RowSlice != nil {
			fmt.Fprintf(&b, " slice=true")
		}
		if path.RowGather != nil {
			fmt.Fprintf(&b, " gather=true")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatQVectorWhereHotPaths(paths []QVectorWhereHotPath) string {
	if len(paths) == 0 {
		return "0 vector conditional pipeline(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d vector conditional pipeline(s)\n", len(paths))
	if counts := CountQVectorWhereHotPathShapes(paths); len(counts) > 0 {
		fmt.Fprintf(&b, "  shapes: %s\n", formatQQueryHotPathShapeCounts(counts))
	}
	for i, path := range paths {
		fmt.Fprintf(&b, "  [%d] shape=%s predicate=%s true=%s false=%s\n",
			i,
			path.Shape(),
			qVectorWherePredicateName(path),
			qVectorWhereOperandName(path.TrueColumn),
			qVectorWhereOperandName(path.FalseColumn),
		)
	}
	return b.String()
}

func formatQQueryHotPathShapeCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}
	shapes := make([]string, 0, len(counts))
	for shape := range counts {
		shapes = append(shapes, shape)
	}
	sort.Strings(shapes)
	parts := make([]string, 0, len(shapes))
	for _, shape := range shapes {
		parts = append(parts, fmt.Sprintf("%s=%d", shape, counts[shape]))
	}
	return strings.Join(parts, ", ")
}

func formatQQueryLoweringFallbackReasons(counts map[string]int) string {
	if len(counts) == 0 {
		return "0 fallback reason(s)\n"
	}
	return fmt.Sprintf("%d fallback reason(s): %s\n", len(counts), formatQQueryHotPathShapeCounts(counts))
}

func formatQVectorLoweringFallbackReasons(counts map[string]int) string {
	if len(counts) == 0 {
		return "0 fallback reason(s)\n"
	}
	return fmt.Sprintf("%d fallback reason(s): %s\n", len(counts), formatQQueryHotPathShapeCounts(counts))
}

func formatQKernelDescriptors(rows []QKernelDescriptor) string {
	if len(rows) == 0 {
		return "0 descriptor row(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d descriptor row(s)\n", len(rows))
	for i, row := range rows {
		fmt.Fprintf(&b, "  [%d] source=%s kind=%s kernel=%s shape=%s route=%s",
			i, row.Source, row.Kind, row.Kernel, row.Shape, row.Route)
		if row.Outcome != "" {
			fmt.Fprintf(&b, " outcome=%s", row.Outcome)
		}
		if row.PipelineShape != "" {
			fmt.Fprintf(&b, " pipeline_shape=%s", row.PipelineShape)
		}
		if row.ReasonFamily != "" {
			fmt.Fprintf(&b, " reason_family=%s", row.ReasonFamily)
		}
		if row.ReasonCode != "" {
			fmt.Fprintf(&b, " reason_code=%s", row.ReasonCode)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatQKernelExecutionStats(rows []QKernelExecutionStat) string {
	if len(rows) == 0 {
		return "0 execution row(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d execution row(s)\n", len(rows))
	for i, row := range rows {
		fmt.Fprintf(&b, "  [%d] source=%s kernel=%s shape=%s route=%s outcome=%s count=%d\n",
			i, row.Source, row.Kernel, row.Shape, row.Route, row.Outcome, row.Count)
	}
	return b.String()
}

func formatQKernelDescriptorCacheStats(rows []QKernelDescriptorCacheStat) string {
	if len(rows) == 0 {
		return "0 descriptor cache row(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d descriptor cache row(s)\n", len(rows))
	for i, row := range rows {
		fmt.Fprintf(&b, "  [%d] source=%s kernel=%s shape=%s pipeline_shape=%s route=%s schema_hash=%s entries=%d hits=%d misses=%d evictions=%d\n",
			i, row.Source, row.Kernel, row.Shape, row.PipelineShape, row.Route, row.SchemaHash, row.Entries, row.Hits, row.Misses, row.Evictions)
	}
	return b.String()
}

func formatQKernelExecutionRouteSummary(rows []QKernelExecutionRouteSummary) string {
	if len(rows) == 0 {
		return "0 route summary row(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d route summary row(s)\n", len(rows))
	for i, row := range rows {
		fmt.Fprintf(&b, "  [%d] source=%s kernel=%s route=%s outcome=%s count=%d\n",
			i, row.Source, row.Kernel, row.Route, row.Outcome, row.Count)
	}
	return b.String()
}

func formatQKernelShapeSummary(rows []QKernelShapeSummary) string {
	if len(rows) == 0 {
		return "0 summary row(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d summary row(s)\n", len(rows))
	for i, row := range rows {
		fmt.Fprintf(&b, "  [%d] source=%s kind=%s shape=%s count=%d",
			i, row.Source, row.Kind, row.Shape, row.Count)
		if row.Outcome != "" {
			fmt.Fprintf(&b, " outcome=%s", row.Outcome)
		}
		if row.ReasonFamily != "" {
			fmt.Fprintf(&b, " reason_family=%s", row.ReasonFamily)
		}
		if row.ReasonCode != "" {
			fmt.Fprintf(&b, " reason_code=%s", row.ReasonCode)
		}
		if row.Executions != 0 || row.Successes != 0 || row.Errors != 0 {
			fmt.Fprintf(&b, " executions=%d successes=%d errors=%d", row.Executions, row.Successes, row.Errors)
		}
		if row.Hits != 0 || row.Misses != 0 || row.Evictions != 0 {
			fmt.Fprintf(&b, " hits=%d misses=%d evictions=%d", row.Hits, row.Misses, row.Evictions)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatQFrameSelectColumnSpecs(specs []QFrameSelectColumnSpec) string {
	if len(specs) == 0 {
		return "0 typed runtime kernel(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d typed runtime kernel(s)\n", len(specs))
	if counts := CountQFrameSelectColumnSpecShapes(specs); len(counts) > 0 {
		fmt.Fprintf(&b, "  shapes: %s\n", formatQQueryHotPathShapeCounts(counts))
	}
	for i, spec := range specs {
		fmt.Fprintf(&b, "  [%d] shape=%s mask=%s rows=%s dynamic_arg=%s project_const=%d result_const=%d",
			i,
			qFrameSelectColumnSpecShape(spec),
			qFrameSelectColumnSpecMaskKind(spec),
			qFrameSelectColumnRowModeName(spec.RowMode),
			qFrameSelectColumnDynamicArgRoleName(spec.DynamicArgRole),
			spec.ProjectConst,
			spec.ResultColumnConst,
		)
		if spec.RowOrderConst >= 0 {
			fmt.Fprintf(&b, " order_const=%d", spec.RowOrderConst)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatQFrameRuntimeKernelReport(kernels []QFrameRuntimeKernel) string {
	if len(kernels) == 0 {
		return "0 frame runtime kernel(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d frame runtime kernel(s)\n", len(kernels))
	if counts := CountQFrameRuntimeKernelShapes(kernels); len(counts) > 0 {
		fmt.Fprintf(&b, "  shapes: %s\n", formatQQueryHotPathShapeCounts(counts))
	}
	for i, kernel := range kernels {
		fmt.Fprintf(&b, "  [%d] shape=%s kernel=%s", i, kernel.Shape(), kernel.Kernel)
		if kernel.Detail != "" {
			fmt.Fprintf(&b, " %s", kernel.Detail)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatQTypedVectorRuntimeKernelReport(kernels []QVectorRuntimeKernel) string {
	if len(kernels) == 0 {
		return "0 typed vector runtime kernel(s)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d typed vector runtime kernel(s)\n", len(kernels))
	if counts := CountQVectorRuntimeKernelShapes(kernels); len(counts) > 0 {
		fmt.Fprintf(&b, "  shapes: %s\n", formatQQueryHotPathShapeCounts(counts))
	}
	for i, kernel := range kernels {
		fmt.Fprintf(&b, "  [%d] shape=%s kernel=%s", i, kernel.Shape(), kernel.Kernel)
		if kernel.Detail != "" {
			fmt.Fprintf(&b, " %s", kernel.Detail)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func qFrameSelectColumnSpecShape(spec QFrameSelectColumnSpec) string {
	if spec.Shape == "" {
		return "unknown"
	}
	return spec.Shape
}

func qFrameSelectColumnSpecMaskKind(spec QFrameSelectColumnSpec) string {
	if len(spec.MaskTerms) > 0 {
		return fmt.Sprintf("mask-terms:%d root=%d", len(spec.MaskTerms), spec.MaskRoot)
	}
	if spec.MaskSpecConst >= 0 {
		return fmt.Sprintf("frame-mask:%d", spec.MaskSpecConst)
	}
	if spec.SourceColumnConst >= 0 {
		return fmt.Sprintf("compare:%s:%d", qDenseArrayCompareOpName(spec.CompareOp), spec.SourceColumnConst)
	}
	return "none"
}

func qVectorWherePredicateName(path QVectorWhereHotPath) string {
	switch {
	case path.Compare != nil:
		return "compare " + qDenseArrayCompareOpName(runtime.DenseArrayBinaryOp(path.Compare.Aux))
	case path.Mask != nil:
		return "frame-mask"
	case path.MaskCombine != nil:
		return "mask-combine"
	default:
		return "unknown"
	}
}

func qVectorWhereOperandName(column *Instr) string {
	if column == nil {
		return "scalar"
	}
	return "frame-column"
}

func qVectorReduceOpName(path QVectorReduceHotPath) string {
	if path.Reduce == nil {
		return "unknown"
	}
	switch runtime.DenseArrayReduceOp(path.Reduce.Aux) {
	case runtime.DenseArrayReduceSum:
		return "sum"
	case runtime.DenseArrayReduceMin:
		return "min"
	case runtime.DenseArrayReduceMax:
		return "max"
	case runtime.DenseArrayReduceMean:
		return "mean"
	default:
		return fmt.Sprintf("op(%d)", path.Reduce.Aux)
	}
}

func qVectorReduceInputName(path QVectorReduceHotPath) string {
	switch {
	case path.Where != nil:
		return "vector-where"
	case path.Gather != nil:
		return "vector-gather"
	case path.SourceColumn != nil:
		return "frame-column"
	default:
		return "vector"
	}
}

func qDenseArrayCompareOpName(op runtime.DenseArrayBinaryOp) string {
	switch op {
	case runtime.DenseArrayEQ:
		return "=="
	case runtime.DenseArrayNE:
		return "!="
	case runtime.DenseArrayLT:
		return "<"
	case runtime.DenseArrayLE:
		return "<="
	case runtime.DenseArrayGT:
		return ">"
	case runtime.DenseArrayGE:
		return ">="
	default:
		return fmt.Sprintf("op(%d)", op)
	}
}

func qDenseArrayMaskOpName(op runtime.DenseArrayMaskOp) string {
	switch op {
	case runtime.DenseArrayMaskAnd:
		return "and"
	case runtime.DenseArrayMaskOr:
		return "or"
	case runtime.DenseArrayMaskXor:
		return "xor"
	case runtime.DenseArrayMaskAndNot:
		return "andnot"
	default:
		return fmt.Sprintf("op(%d)", op)
	}
}

func qDenseArrayReduceOpName(op runtime.DenseArrayReduceOp) string {
	switch op {
	case runtime.DenseArrayReduceSum:
		return "sum"
	case runtime.DenseArrayReduceMin:
		return "min"
	case runtime.DenseArrayReduceMax:
		return "max"
	case runtime.DenseArrayReduceMean:
		return "mean"
	default:
		return fmt.Sprintf("op(%d)", op)
	}
}

func qFrameSelectColumnRowModeName(mode QFrameSelectColumnRowMode) string {
	switch mode {
	case QFrameSelectColumnRowsNone:
		return "none"
	case QFrameSelectColumnRowsGather:
		return "gather"
	case QFrameSelectColumnRowsSlice:
		return "slice"
	case QFrameSelectColumnRowsOrderGather:
		return "order/gather"
	default:
		return fmt.Sprintf("unknown(%d)", mode)
	}
}

func qFrameSelectColumnDynamicArgRoleName(role QFrameSelectColumnDynamicArgRole) string {
	switch role {
	case QFrameSelectColumnArgNone:
		return "none"
	case QFrameSelectColumnArgCompareRHS:
		return "compare_rhs"
	case QFrameSelectColumnArgRowValue:
		return "row_value"
	default:
		return fmt.Sprintf("unknown(%d)", role)
	}
}

func qQueryHotPathCompareOpName(compare *Instr) string {
	if compare == nil {
		return "frame-mask"
	}
	return qDenseArrayCompareOpName(runtime.DenseArrayBinaryOp(compare.Aux))
}

func qQueryHotPathPredicateName(path QQueryHotPath) string {
	switch {
	case path.Compare != nil:
		return qQueryHotPathCompareOpName(path.Compare)
	case path.MaskCombine != nil:
		return "mask-combine"
	case path.Filter == nil:
		return "none"
	default:
		return "frame-mask"
	}
}

func qQueryCompareColumn(compare *Instr) *Instr {
	for _, arg := range compare.Args {
		if col := valueDef(arg, OpFrameColumn); col != nil {
			return col
		}
	}
	return nil
}

func valueDef(value *Value, op Op) *Instr {
	if value == nil || value.Def == nil || value.Def.Op != op {
		return nil
	}
	return value.Def
}
