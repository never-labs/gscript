package q

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

const (
	qSQLRuntimeSource = "q_sql_runtime"
	qSQLPlanKernel    = "QSQLPlan"

	qSQLRuntimeRouteTypedBackend = "typed_query_backend"
	qSQLRuntimeRoutePlanFallback = "query_plan_fallback"
)

// QSQLRuntimeDescriptor is the stable qSQL handoff contract for the typed data
// runtime/JIT backend. It reuses data.QueryKernelPlanShape vocabulary so qSQL
// and Leia-native query execution converge on the same columnar pipeline model.
type QSQLRuntimeDescriptor struct {
	Op            QueryKind
	Shape         string
	PipelineShape string
	Supported     bool
	Reason        string
}

func (d QSQLRuntimeDescriptor) Valid() bool {
	return d.Shape != "" && d.PipelineShape != ""
}

// RuntimeDescriptor returns the backend descriptor for a lowered qSQL read
// query. Mutation plans are excluded because they carry write semantics rather
// than a pure columnar query pipeline.
func (l *Lowered) RuntimeDescriptor() QSQLRuntimeDescriptor {
	if l == nil || l.Mutation != nil || (l.Op != SelectQuery && l.Op != ExecQuery) {
		return QSQLRuntimeDescriptor{}
	}
	shape := data.QueryKernelPlanShape(l.Plan)
	pipelineShape := data.QueryKernelPlanPipelineShape(l.Plan)
	if joins := qSQLRuntimeJoinPipelineShape(l.Joins, l.Join); joins != "" {
		shape += "|join=" + qSQLRuntimeJoinShape(l.Joins, l.Join)
		pipelineShape = joins + "|" + pipelineShape
	}
	supported, reason := data.QueryKernelSupportReason(l.Plan)
	if qSQLRuntimeJoinCount(l.Joins, l.Join) > 0 {
		supported = false
		reason = "join plan requires qSQL source handoff before query kernel execution"
	}
	return QSQLRuntimeDescriptor{
		Op:            l.Op,
		Shape:         "qsql/" + string(l.Op) + "/" + shape,
		PipelineShape: pipelineShape,
		Supported:     supported,
		Reason:        reason,
	}
}

// ExecRuntime executes a lowered qSQL read plan through the typed query kernel
// backend when supported, preserving QueryPlan.Exec as the semantic fallback.
func (l *Lowered) ExecRuntime(frame data.Frame) (data.Frame, error) {
	if l == nil {
		return data.Frame{}, fmt.Errorf("nil qSQL runtime plan")
	}
	plan := l.Plan
	plan.Source = frame
	descriptor := l.RuntimeDescriptor()
	if !descriptor.Valid() {
		return plan.Exec()
	}
	recordRuntimeExecutionWithPipelineShape(qSQLRuntimeSource, qSQLPlanKernel, descriptor.Shape, descriptor.PipelineShape, qSQLRuntimeRouteTypedBackend, "attempt", "attempt")
	if !descriptor.Supported {
		reason := descriptor.Reason
		if reason == "" {
			_, reason = data.QueryKernelSupportReason(plan)
		}
		recordRuntimeExecutionWithPipelineShape(qSQLRuntimeSource, qSQLPlanKernel, descriptor.Shape, descriptor.PipelineShape, qSQLRuntimeRoutePlanFallback, "fallback", reason)
		return plan.Exec()
	}
	kernel, ok, err := data.CompileQueryKernel(frame, plan)
	if err != nil {
		recordRuntimeExecutionWithPipelineShape(qSQLRuntimeSource, qSQLPlanKernel, descriptor.Shape, descriptor.PipelineShape, qSQLRuntimeRouteTypedBackend, "error", err.Error())
		return data.Frame{}, err
	}
	if !ok {
		reason := descriptor.Reason
		if reason == "" {
			_, reason = data.QueryKernelSupportReason(plan)
		}
		recordRuntimeExecutionWithPipelineShape(qSQLRuntimeSource, qSQLPlanKernel, descriptor.Shape, descriptor.PipelineShape, qSQLRuntimeRoutePlanFallback, "fallback", reason)
		return plan.Exec()
	}
	out, err := data.ExecQueryKernelOrPlan(kernel, plan, frame)
	if err != nil {
		recordRuntimeExecutionWithPipelineShape(qSQLRuntimeSource, qSQLPlanKernel, descriptor.Shape, descriptor.PipelineShape, qSQLRuntimeRouteTypedBackend, "error", err.Error())
		return data.Frame{}, err
	}
	recordRuntimeExecutionWithPipelineShape(qSQLRuntimeSource, qSQLPlanKernel, descriptor.Shape, descriptor.PipelineShape, qSQLRuntimeRouteTypedBackend, "hit", "typed_kernel")
	return out, nil
}

func qSQLRuntimeJoinPipelineShape(joins []*JoinPlan, first *JoinPlan) string {
	count := qSQLRuntimeJoinCount(joins, first)
	if count == 0 {
		return ""
	}
	kinds := qSQLRuntimeJoinKinds(joins, first)
	return "join=" + strings.Join(kinds, "+") + "/keys-" + strconv.Itoa(qSQLRuntimeJoinKeyCount(joins, first)) + "/count-" + strconv.Itoa(count)
}

func qSQLRuntimeJoinShape(joins []*JoinPlan, first *JoinPlan) string {
	if qSQLRuntimeJoinCount(joins, first) == 0 {
		return "none"
	}
	return strings.TrimPrefix(qSQLRuntimeJoinPipelineShape(joins, first), "join=")
}

func qSQLRuntimeJoinCount(joins []*JoinPlan, first *JoinPlan) int {
	if len(joins) > 0 {
		count := 0
		for _, join := range joins {
			if join != nil {
				count++
			}
		}
		return count
	}
	if first != nil {
		return 1
	}
	return 0
}

func qSQLRuntimeJoinKeyCount(joins []*JoinPlan, first *JoinPlan) int {
	total := 0
	for _, join := range qSQLRuntimeJoinList(joins, first) {
		total += len(join.Keys)
	}
	return total
}

func qSQLRuntimeJoinKinds(joins []*JoinPlan, first *JoinPlan) []string {
	list := qSQLRuntimeJoinList(joins, first)
	out := make([]string, 0, len(list))
	for _, join := range list {
		kind := strings.TrimSpace(join.Kind)
		if kind == "" {
			kind = "join"
		}
		out = append(out, kind)
	}
	return out
}

func qSQLRuntimeJoinList(joins []*JoinPlan, first *JoinPlan) []*JoinPlan {
	if len(joins) > 0 {
		out := make([]*JoinPlan, 0, len(joins))
		for _, join := range joins {
			if join != nil {
				out = append(out, join)
			}
		}
		return out
	}
	if first != nil {
		return []*JoinPlan{first}
	}
	return nil
}
