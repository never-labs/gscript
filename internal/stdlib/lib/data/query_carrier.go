package data

import "fmt"

// QueryKernelPlan is the schema-aligned, source-free description that qSQL, q
// runtime helpers and JIT backends can share before selecting an execution
// backend. Shape and PipelineShape are intentionally literal-free; Fingerprint
// and CacheKey include the plan details that affect compiled reuse.
type QueryKernelPlan struct {
	Shape         string
	PipelineShape string
	Pipeline      QueryKernelPipelineDescriptor
	Fingerprint   string
	CacheKey      string
	Carrier       QueryKernelCarrierPlan
}

// QueryKernelCarrierPlan describes how a QueryKernelPlan connects to typed
// carrier primitives for row selection, projection and grouped aggregation.
type QueryKernelCarrierPlan struct {
	Filter     QueryKernelCarrierStage
	Projection QueryKernelCarrierStage
	Group      QueryKernelCarrierStage
}

// QueryKernelCarrierStage is a stable per-stage shape/key/cache handle for
// typed carrier backends. Handled is false when a stage is absent or must stay
// on the generic row path.
type QueryKernelCarrierStage struct {
	Family  string
	Ops     string
	Key     string
	Handled bool
}

// DescribeQueryKernelPlan validates and describes the shared data-layer query
// kernel contract for a frame and plan. Unsupported query-kernel shapes return
// ok=false; validation errors for supported shapes are returned as errors.
func DescribeQueryKernelPlan(namespace string, frame Frame, plan QueryPlan) (QueryKernelPlan, bool, error) {
	ok, _ := QueryKernelSupportReason(plan)
	if !ok {
		return QueryKernelPlan{}, false, nil
	}
	compiled := cloneQueryKernelPlan(plan)
	compiled.Source = Frame{}
	if err := validateQueryKernelFrame(frame, compiled); err != nil {
		return QueryKernelPlan{}, true, err
	}
	pipeline := QueryKernelPlanPipelineDescriptor(compiled)
	carrier, err := describeQueryKernelCarrierPlan(namespace, frame, compiled, pipeline)
	if err != nil {
		return QueryKernelPlan{}, true, err
	}
	return QueryKernelPlan{
		Shape:         QueryKernelPlanShape(compiled),
		PipelineShape: pipeline.String(),
		Pipeline:      pipeline,
		Fingerprint:   QueryKernelPlanFingerprint(compiled),
		CacheKey:      QueryKernelCacheKey(namespace, frame, compiled),
		Carrier:       carrier,
	}, true, nil
}

// QueryKernelCarrierCacheKey returns a schema-stable per-stage carrier cache
// key derived from the same query-kernel fingerprint as QueryKernelCacheKey.
func QueryKernelCarrierCacheKey(namespace string, frame Frame, plan QueryPlan, stage string) string {
	return querySchemaStableCacheKey(namespace, "carrier:"+stage, frame, QueryKernelPlanFingerprint(plan))
}

func describeQueryKernelCarrierPlan(namespace string, frame Frame, plan QueryPlan, pipeline QueryKernelPipelineDescriptor) (QueryKernelCarrierPlan, error) {
	filter, err := describeQueryKernelFilterCarrier(namespace, frame, plan, pipeline)
	if err != nil {
		return QueryKernelCarrierPlan{}, err
	}
	projection := describeQueryKernelProjectionCarrier(namespace, frame, plan, pipeline, filter.Handled)
	group, err := describeQueryKernelGroupCarrier(namespace, frame, plan, pipeline, filter.Handled)
	if err != nil {
		return QueryKernelCarrierPlan{}, err
	}
	return QueryKernelCarrierPlan{Filter: filter, Projection: projection, Group: group}, nil
}

func describeQueryKernelFilterCarrier(namespace string, frame Frame, plan QueryPlan, pipeline QueryKernelPipelineDescriptor) (QueryKernelCarrierStage, error) {
	stage := QueryKernelCarrierStage{
		Family:  pipeline.FilterFamily,
		Ops:     "i64_index",
		Key:     QueryKernelCarrierCacheKey(namespace, frame, plan, "filter"),
		Handled: false,
	}
	if plan.Where == nil {
		stage.Family = "index"
		stage.Ops = "i64_range_all"
		stage.Handled = true
		return stage, nil
	}
	if pipeline.FilterFamily == "" {
		return stage, nil
	}
	handled, err := typedFilterIndexArraySupported(frame, plan.Where)
	if err != nil || !handled {
		return stage, err
	}
	stage.Handled = true
	return stage, nil
}

func describeQueryKernelProjectionCarrier(namespace string, frame Frame, plan QueryPlan, pipeline QueryKernelPipelineDescriptor, filterHandled bool) QueryKernelCarrierStage {
	stage := QueryKernelCarrierStage{
		Family: pipeline.ProjectionFamily,
		Ops:    pipeline.ProjectionOps,
		Key:    QueryKernelCarrierCacheKey(namespace, frame, plan, "project"),
	}
	if stage.Family == "" {
		stage.Family = "column_load"
		stage.Ops = "column_load:all"
	}
	stage.Handled = filterHandled && queryKernelProjectionUsesTypedCarrier(plan)
	return stage
}

func queryKernelProjectionUsesTypedCarrier(plan QueryPlan) bool {
	if len(plan.Select) == 0 {
		return true
	}
	if selectItemsNeedGroupedRows(plan.Select) {
		return false
	}
	for _, item := range plan.Select {
		if !queryKernelProjectionExprUsesTypedCarrier(item.Expr) {
			return false
		}
	}
	return true
}

func queryKernelProjectionExprUsesTypedCarrier(expr Expr) bool {
	switch e := expr.(type) {
	case ColumnRef:
		return true
	case Binary:
		_, leftOK := e.Left.(ColumnRef)
		if !leftOK {
			_, leftOK = e.Left.(Literal)
		}
		_, rightOK := e.Right.(ColumnRef)
		if !rightOK {
			_, rightOK = e.Right.(Literal)
		}
		return leftOK && rightOK
	default:
		return false
	}
}

func describeQueryKernelGroupCarrier(namespace string, frame Frame, plan QueryPlan, pipeline QueryKernelPipelineDescriptor, filterHandled bool) (QueryKernelCarrierStage, error) {
	stage := QueryKernelCarrierStage{
		Family:  pipeline.GroupFamily,
		Ops:     pipeline.GroupOps,
		Key:     QueryKernelCarrierCacheKey(namespace, frame, plan, "group"),
		Handled: false,
	}
	if len(plan.By)+len(plan.ByExprs) == 0 || len(plan.Aggregates) == 0 {
		return stage, nil
	}
	byInputs, err := bindGroupInputs(frame, groupByItems(plan))
	if err != nil {
		return stage, err
	}
	aggs, err := bindAggregateInputs(frame, plan.Aggregates)
	if err != nil {
		return stage, err
	}
	if !filterHandled || len(byInputs) != 1 || !typedGroupedAggregateInputsSupported(aggs) {
		return stage, nil
	}
	index, ok, err := queryKernelGroupIndexForCarrier(byInputs[0])
	if err != nil || !ok {
		return stage, err
	}
	if !isTypedGroupedAggregateKeyKind(byInputs[0].keyKind()) {
		return stage, nil
	}
	if len(index.Rows) == 0 && frame.Len() > 0 {
		return stage, fmt.Errorf("group carrier index is empty for non-empty frame")
	}
	stage.Handled = true
	return stage, nil
}

func queryKernelGroupIndexForCarrier(input groupInput) (ArrayIndex, bool, error) {
	if input.column == nil {
		return ArrayIndex{}, false, nil
	}
	if index, ok := arrayIndexForBorrowed(input.column, ArrayAttributeUnique); ok {
		return index, true, nil
	}
	if index, ok := arrayIndexForBorrowed(input.column, ArrayAttributeGrouped); ok {
		return index, true, nil
	}
	index, err := BuildArrayIndex(input.column, ArrayAttributeGrouped)
	return index, err == nil, err
}

// TryExecuteQueryKernelTypedCarrier executes the typed filter/projection
// carrier path when the shared QueryKernelPlan says the shape is fully handled.
func TryExecuteQueryKernelTypedCarrier(frame Frame, plan QueryPlan, carrier QueryKernelCarrierPlan) (Frame, bool, error) {
	if plan.Distinct || plan.PreProjectOrder || plan.LimitN >= 0 || len(plan.OrderBy) > 0 ||
		len(plan.By) > 0 || len(plan.ByExprs) > 0 || len(plan.Aggregates) > 0 {
		return Frame{}, false, nil
	}
	if !carrier.Filter.Handled || !carrier.Projection.Handled {
		return Frame{}, false, nil
	}
	indexes, ok, err := typedFilterIndexArray(frame, plan.Where)
	if err != nil || !ok {
		return Frame{}, ok, err
	}
	out, err := execProjectByI64IndexArray(frame, indexes, plan.Select)
	return out, true, err
}

// TryExecuteQueryKernelGroupedProjectionCarrier executes grouped projection
// shapes whose grouping is only semantic and whose projection can be evaluated
// through the typed row-index carrier.
func TryExecuteQueryKernelGroupedProjectionCarrier(frame Frame, plan QueryPlan, carrier QueryKernelCarrierPlan) (Frame, bool, error) {
	if plan.Distinct || plan.PreProjectOrder || plan.LimitN >= 0 || len(plan.OrderBy) > 0 ||
		len(plan.Aggregates) > 0 || len(plan.Select) == 0 {
		return Frame{}, false, nil
	}
	if len(plan.By) == 0 && len(plan.ByExprs) == 0 {
		return Frame{}, false, nil
	}
	if !carrier.Filter.Handled || !carrier.Projection.Handled {
		return Frame{}, false, nil
	}
	indexes, ok, err := typedFilterIndexArray(frame, plan.Where)
	if err != nil || !ok {
		return Frame{}, ok, err
	}
	out, err := execProjectByI64IndexArray(frame, indexes, plan.Select)
	return out, true, err
}
