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
	QueryShape    string
	Pipeline      QSQLRuntimePipelineDescriptor
	Unsupported   QSQLRuntimeUnsupported
	Supported     bool
	Reason        string
}

func (d QSQLRuntimeDescriptor) Valid() bool {
	return d.Shape != "" && d.PipelineShape != ""
}

// QSQLRuntimeUnsupported is the stable fallback reason carried by runtime
// descriptors and stats.
type QSQLRuntimeUnsupported struct {
	Reason     string
	ReasonCode string
}

// QSQLRuntimePipelineDescriptor keeps qSQL source-side stages separate from the
// data query plan shape. Backend implementations can consume the structured
// fields without parsing the compact PipelineShape string.
type QSQLRuntimePipelineDescriptor struct {
	Query data.QueryKernelPipelineDescriptor
	Join  QSQLRuntimeJoinDescriptor
}

func (d QSQLRuntimePipelineDescriptor) String() string {
	stages := make([]string, 0, 2)
	if d.Join.Count > 0 {
		stages = append(stages, d.Join.PipelineStage())
	}
	if query := d.Query.String(); query != "" {
		stages = append(stages, query)
	}
	return strings.Join(stages, "|")
}

// QSQLRuntimeJoinDescriptor is the schema-stable source-side join shape.
type QSQLRuntimeJoinDescriptor struct {
	Kinds    []string
	KeyCount int
	Count    int
}

func (d QSQLRuntimeJoinDescriptor) ShapePart() qSQLRuntimeShapePart {
	if d.Count == 0 {
		return qSQLRuntimeShapePart{}
	}
	return qSQLRuntimeShapePart{Name: "join", Value: d.compactValue()}
}

func (d QSQLRuntimeJoinDescriptor) PipelineStage() string {
	if d.Count == 0 {
		return ""
	}
	return "join=" + d.compactValue()
}

func (d QSQLRuntimeJoinDescriptor) Unsupported() QSQLRuntimeUnsupported {
	if d.Count == 0 {
		return QSQLRuntimeUnsupported{}
	}
	reason := "join plan requires qSQL source handoff before query kernel execution"
	return QSQLRuntimeUnsupported{Reason: reason, ReasonCode: RuntimeFallbackReasonCode(reason)}
}

func (d QSQLRuntimeJoinDescriptor) compactValue() string {
	return strings.Join(d.Kinds, "+") + "/keys-" + strconv.Itoa(d.KeyCount) + "/count-" + strconv.Itoa(d.Count)
}

type qSQLRuntimeShapePart struct {
	Name  string
	Value string
}

func (p qSQLRuntimeShapePart) String() string {
	if p.Name == "" || p.Value == "" {
		return ""
	}
	return p.Name + "=" + p.Value
}

func qSQLRuntimeShape(base string, parts ...qSQLRuntimeShapePart) string {
	values := make([]string, 0, 1+len(parts))
	values = append(values, base)
	for _, part := range parts {
		if value := part.String(); value != "" {
			values = append(values, value)
		}
	}
	return strings.Join(values, "|")
}

type qSQLRuntimeBackend interface {
	Route() string
	Compile(frame data.Frame, plan data.QueryPlan, descriptor QSQLRuntimeDescriptor) (qSQLRuntimeExecutable, bool, error)
}

type qSQLRuntimeExecutable interface {
	Exec(plan data.QueryPlan, frame data.Frame) (data.Frame, error)
}

type qSQLDataQueryBackend struct{}

func (qSQLDataQueryBackend) Route() string {
	return qSQLRuntimeRouteTypedBackend
}

func (qSQLDataQueryBackend) Compile(frame data.Frame, plan data.QueryPlan, descriptor QSQLRuntimeDescriptor) (qSQLRuntimeExecutable, bool, error) {
	kernel, ok, err := data.CompileQueryKernel(frame, plan)
	if err != nil || !ok {
		return nil, ok, err
	}
	return qSQLDataQueryExecutable{kernel: kernel}, true, nil
}

type qSQLDataQueryExecutable struct {
	kernel *data.QueryKernel
}

func (e qSQLDataQueryExecutable) Exec(plan data.QueryPlan, frame data.Frame) (data.Frame, error) {
	return data.ExecQueryKernelOrPlan(e.kernel, plan, frame)
}

var qSQLDefaultRuntimeBackend qSQLRuntimeBackend = qSQLDataQueryBackend{}

// RuntimeDescriptor returns the backend descriptor for a lowered qSQL read
// query. Mutation plans are excluded because they carry write semantics rather
// than a pure columnar query pipeline.
func (l *Lowered) RuntimeDescriptor() QSQLRuntimeDescriptor {
	if l == nil || l.Mutation != nil || (l.Op != SelectQuery && l.Op != ExecQuery) {
		return QSQLRuntimeDescriptor{}
	}
	queryShape := data.QueryKernelPlanShape(l.Plan)
	join := qSQLRuntimeJoinDescriptor(l.Joins, l.Join)
	pipeline := QSQLRuntimePipelineDescriptor{
		Query: data.QueryKernelPlanPipelineDescriptor(l.Plan),
		Join:  join,
	}
	supported, reason := data.QueryKernelSupportReason(l.Plan)
	var unsupported QSQLRuntimeUnsupported
	if !supported {
		unsupported = QSQLRuntimeUnsupported{Reason: reason, ReasonCode: RuntimeFallbackReasonCode(reason)}
	}
	if joinUnsupported := join.Unsupported(); joinUnsupported.Reason != "" {
		supported = false
		unsupported = joinUnsupported
	}
	descriptorReason := reason
	if !supported {
		descriptorReason = unsupported.Reason
	}
	return QSQLRuntimeDescriptor{
		Op:            l.Op,
		Shape:         "qsql/" + string(l.Op) + "/" + qSQLRuntimeShape(queryShape, join.ShapePart()),
		PipelineShape: pipeline.String(),
		QueryShape:    queryShape,
		Pipeline:      pipeline,
		Unsupported:   unsupported,
		Supported:     supported,
		Reason:        descriptorReason,
	}
}

// ExecRuntime executes a lowered qSQL read plan through the typed query kernel
// backend when supported, preserving QueryPlan.Exec as the semantic fallback.
func (l *Lowered) ExecRuntime(frame data.Frame) (data.Frame, error) {
	return l.execRuntime(frame, qSQLDefaultRuntimeBackend)
}

func (l *Lowered) execRuntime(frame data.Frame, backend qSQLRuntimeBackend) (data.Frame, error) {
	if l == nil {
		return data.Frame{}, fmt.Errorf("nil qSQL runtime plan")
	}
	if backend == nil {
		backend = qSQLDefaultRuntimeBackend
	}
	plan := l.Plan
	plan.Source = frame
	descriptor := l.RuntimeDescriptor()
	if !descriptor.Valid() {
		return plan.Exec()
	}
	recordRuntimeExecutionWithPipelineShape(qSQLRuntimeSource, qSQLPlanKernel, descriptor.Shape, descriptor.PipelineShape, backend.Route(), "attempt", "attempt")
	if !descriptor.Supported {
		reason := qSQLRuntimeUnsupportedReason(descriptor, plan)
		recordRuntimeExecutionWithPipelineShape(qSQLRuntimeSource, qSQLPlanKernel, descriptor.Shape, descriptor.PipelineShape, qSQLRuntimeRoutePlanFallback, "fallback", reason)
		return plan.Exec()
	}
	executable, ok, err := backend.Compile(frame, plan, descriptor)
	if err != nil {
		recordRuntimeExecutionWithPipelineShape(qSQLRuntimeSource, qSQLPlanKernel, descriptor.Shape, descriptor.PipelineShape, backend.Route(), "error", err.Error())
		return data.Frame{}, err
	}
	if !ok {
		reason := qSQLRuntimeBackendFallbackReason(descriptor, plan)
		recordRuntimeExecutionWithPipelineShape(qSQLRuntimeSource, qSQLPlanKernel, descriptor.Shape, descriptor.PipelineShape, qSQLRuntimeRoutePlanFallback, "fallback", reason)
		return plan.Exec()
	}
	out, err := executable.Exec(plan, frame)
	if err != nil {
		recordRuntimeExecutionWithPipelineShape(qSQLRuntimeSource, qSQLPlanKernel, descriptor.Shape, descriptor.PipelineShape, backend.Route(), "error", err.Error())
		return data.Frame{}, err
	}
	recordRuntimeExecutionWithPipelineShape(qSQLRuntimeSource, qSQLPlanKernel, descriptor.Shape, descriptor.PipelineShape, backend.Route(), "hit", "typed_kernel")
	return out, nil
}

func qSQLRuntimeUnsupportedReason(descriptor QSQLRuntimeDescriptor, plan data.QueryPlan) string {
	if descriptor.Unsupported.Reason != "" {
		return descriptor.Unsupported.Reason
	}
	if descriptor.Reason != "" {
		return descriptor.Reason
	}
	_, reason := data.QueryKernelSupportReason(plan)
	return reason
}

func qSQLRuntimeBackendFallbackReason(descriptor QSQLRuntimeDescriptor, plan data.QueryPlan) string {
	if descriptor.Unsupported.Reason != "" {
		return descriptor.Unsupported.Reason
	}
	if supported, reason := data.QueryKernelSupportReason(plan); !supported {
		return reason
	}
	return "runtime backend did not handle descriptor shape"
}

func qSQLRuntimeJoinDescriptor(joins []*JoinPlan, first *JoinPlan) QSQLRuntimeJoinDescriptor {
	return QSQLRuntimeJoinDescriptor{
		Kinds:    qSQLRuntimeJoinKinds(joins, first),
		KeyCount: qSQLRuntimeJoinKeyCount(joins, first),
		Count:    qSQLRuntimeJoinCount(joins, first),
	}
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
