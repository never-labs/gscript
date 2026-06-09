package q

import "strings"

type qScriptPipelineKind string

const (
	qScriptPipelineWhereReduceSum      qScriptPipelineKind = "where-reduce/sum"
	qScriptPipelineWhereIndexReduceSum qScriptPipelineKind = "where-index-reduce/sum"
	qScriptPipelineGatherReduceSum     qScriptPipelineKind = "gather-reduce/sum"
	qScriptPipelineUnsupported         qScriptPipelineKind = ""
)

type qScriptPipelineDescriptor struct {
	kind              qScriptPipelineKind
	assignments       []qScriptPipelineAssignment
	terminal          string
	valueExpr         string
	valueBinding      string
	indexExpr         string
	indexBinding      string
	maskExpr          string
	maskBinding       string
	terminalUsesWhere bool
	terminalPlan      qPipelinePlan
}

type qScriptPipelineAssignment struct {
	name      string
	rhs       string
	valueExpr Expr
	binding   qScriptBindingPlan
}

func (d qScriptPipelineDescriptor) shape() string {
	if d.kind == qScriptPipelineUnsupported {
		return "script-pipeline/unsupported"
	}
	if len(d.assignments) == 0 {
		return "script-pipeline/" + string(d.kind) + "/direct"
	}
	return "script-pipeline/" + string(d.kind) + "/assignments"
}

func buildQScriptPipelineDescriptor(statements []qScriptStatement) (*qScriptPipelineDescriptor, bool) {
	compact := make([]qScriptStatement, 0, len(statements))
	for _, stmt := range statements {
		if strings.TrimSpace(stmt.src) != "" {
			compact = append(compact, stmt)
		}
	}
	if len(compact) < 2 {
		return nil, false
	}
	terminal := compact[len(compact)-1]
	if terminal.assign != "" {
		return nil, false
	}
	assignments := make([]qScriptPipelineAssignment, 0, len(compact)-1)
	bindings := make(map[string]string, len(compact)-1)
	for _, stmt := range compact[:len(compact)-1] {
		if stmt.assign == "" || stmt.rhs == "" {
			return nil, false
		}
		if _, _, ok := parseDeferredScan(stmt.rhs); ok {
			return nil, false
		}
		assignments = append(assignments, qScriptPipelineAssignment{name: stmt.assign, rhs: stmt.rhs, valueExpr: stmt.valueExpr, binding: buildQScriptBindingPlanForRHS(stmt.rhs, stmt.valueExpr)})
		bindings[stmt.assign] = stmt.rhs
	}
	descriptor, ok := describeQScriptPipelineTerminal(terminal.src, bindings)
	if !ok {
		return nil, false
	}
	descriptor.assignments = assignments
	descriptor.terminal = terminal.src
	descriptor.terminalPlan = buildQPipelinePlan(terminal.src)
	return &descriptor, true
}

func describeQScriptPipelineTerminal(src string, bindings map[string]string) (qScriptPipelineDescriptor, bool) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "+/") {
		return qScriptPipelineDescriptor{}, false
	}
	body := strings.TrimSpace(src[len("+/"):])
	if body == "" {
		return qScriptPipelineDescriptor{}, false
	}
	if valueExpr, maskExpr, ok := splitTopLevelWord(body, "where"); ok {
		d := qScriptPipelineDescriptor{
			kind:              qScriptPipelineWhereReduceSum,
			valueExpr:         valueExpr,
			valueBinding:      qScriptPipelineBinding(valueExpr, bindings),
			maskExpr:          maskExpr,
			maskBinding:       qScriptPipelineBinding(maskExpr, bindings),
			terminalUsesWhere: true,
		}
		return d, true
	}
	valueExpr, indexExpr, ok := findPostfixIndex(body)
	if !ok {
		return qScriptPipelineDescriptor{}, false
	}
	d := qScriptPipelineDescriptor{
		kind:         qScriptPipelineGatherReduceSum,
		valueExpr:    valueExpr,
		valueBinding: qScriptPipelineBinding(valueExpr, bindings),
		indexExpr:    indexExpr,
		indexBinding: qScriptPipelineBinding(indexExpr, bindings),
	}
	if maskExpr, ok := qScriptPipelineIndexMaskExpr(indexExpr, bindings); ok {
		d.kind = qScriptPipelineWhereIndexReduceSum
		d.maskExpr = maskExpr
		d.maskBinding = qScriptPipelineBinding(maskExpr, bindings)
	}
	return d, true
}

func qScriptPipelineIndexMaskExpr(indexExpr string, bindings map[string]string) (string, bool) {
	if maskExpr, ok := directWhereMaskExpr(indexExpr); ok {
		return maskExpr, true
	}
	bound, ok := bindings[strings.TrimSpace(indexExpr)]
	if !ok {
		return "", false
	}
	return directWhereMaskExpr(bound)
}

func qScriptPipelineBinding(expr string, bindings map[string]string) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ""
	}
	if rhs, ok := bindings[expr]; ok {
		return rhs
	}
	return ""
}

func (s *EvalState) tryEvalQScriptPipeline(descriptor *qScriptPipelineDescriptor) (any, bool, error) {
	if descriptor == nil || descriptor.kind == qScriptPipelineUnsupported {
		return nil, false, nil
	}
	shape := descriptor.shape()
	recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "attempt", "attempt")
	terminal := descriptor.terminalPlan
	if terminal.kind == qPipelineInvalid {
		recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "fallback", "unsupported_runtime_shape")
		return nil, false, nil
	}
	s.rememberQPipelinePlan(descriptor.terminal, terminal)
	for _, assignment := range descriptor.assignments {
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", "runtime_error")
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr)
			if err != nil {
				recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", "runtime_error")
				return nil, true, err
			}
		}
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	out, handled, err := s.evalQPipelinePlan(terminal)
	switch {
	case err != nil:
		recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", "runtime_error")
	case handled:
		recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "hit", "typed_pipeline")
	default:
		recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "fallback", "unsupported_runtime_shape")
	}
	return out, handled, err
}
