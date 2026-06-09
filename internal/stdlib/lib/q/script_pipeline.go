package q

import (
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qScriptPipelineKind string

const (
	qScriptPipelineWhereReduceSum      qScriptPipelineKind = "where-reduce/sum"
	qScriptPipelineWhereIndexReduceSum qScriptPipelineKind = "where-index-reduce/sum"
	qScriptPipelineGatherReduceSum     qScriptPipelineKind = "gather-reduce/sum"
	qScriptPipelineUnsupported         qScriptPipelineKind = ""
)

type qScriptPipelineDescriptor struct {
	kind              qScriptPipelineKind
	shapeText         string
	assignments       []qScriptPipelineAssignment
	terminal          string
	valueExpr         string
	valueBinding      string
	valuePlan         qScriptBindingPlan
	indexExpr         string
	indexBinding      string
	indexPlan         qScriptBindingPlan
	maskExpr          string
	maskBinding       string
	maskPlan          qScriptBindingPlan
	terminalUsesWhere bool
	terminalPlan      qPipelinePlan
	moduloMaskPlan    *qPipelinePlan
}

type qScriptPipelineAssignment struct {
	name      string
	rhs       string
	valueExpr Expr
	binding   qScriptBindingPlan
}

func (d qScriptPipelineDescriptor) shape() string {
	if d.shapeText != "" {
		return d.shapeText
	}
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
		assignments = append(assignments, qScriptPipelineAssignment{name: stmt.assign, rhs: stmt.rhs, valueExpr: stmt.valueExpr, binding: stmt.bindingPlan})
		bindings[stmt.assign] = stmt.rhs
	}
	descriptor, ok := describeQScriptPipelineTerminal(terminal.src, bindings)
	if !ok {
		return nil, false
	}
	descriptor.assignments = assignments
	descriptor.terminal = terminal.src
	descriptor.terminalPlan = buildQPipelinePlan(terminal.src)
	descriptor.valuePlan = buildQScriptBindingPlanForRHS(descriptor.valueExpr, nil)
	descriptor.indexPlan = buildQScriptBindingPlanForRHS(descriptor.indexExpr, nil)
	descriptor.maskPlan = buildQScriptBindingPlanForRHS(descriptor.maskExpr, nil)
	descriptor.moduloMaskPlan = qScriptPipelineModuloMaskPlan(descriptor.maskExpr)
	descriptor.shapeText = descriptor.shape()
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
		recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "fallback", RuntimeFallbackPlannerUnhandled)
		return nil, false, nil
	}
	s.rememberQPipelinePlan(descriptor.terminal, terminal)
	for _, assignment := range descriptor.assignments {
		if qScriptPipelineCanDeferAssignment(descriptor, assignment) {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", "runtime_error")
			return nil, true, err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				recordRuntimeKernelExecution("QScriptPipelinePlan", shape, "error", "runtime_error")
				return nil, true, err
			}
		}
		s.env[s.resolveAssignmentName(assignment.name)] = value
	}
	out, handled, err := s.evalQScriptTerminalPipeline(descriptor, terminal)
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

func (s *EvalState) evalQScriptTerminalPipeline(descriptor *qScriptPipelineDescriptor, terminal qPipelinePlan) (any, bool, error) {
	switch terminal.kind {
	case qPipelineSumWhereIndex:
		value, handled, err := s.evalQScriptBindingPlan(&descriptor.valuePlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		array, ok := value.(data.Array)
		if !ok {
			return nil, false, nil
		}
		if descriptor.moduloMaskPlan != nil {
			if out, handled, err := s.evalQPipelineModuloCompareValueSum(*descriptor.moduloMaskPlan, array); err != nil || handled {
				return out, handled, err
			}
		}
		mask, handled, err := s.evalQScriptBindingPlan(&descriptor.maskPlan)
		if err != nil {
			return nil, true, err
		}
		if handled {
			if maskArray, ok := mask.(data.Array); ok {
				out, handled, err := qPipelineWhereReduceSumWithPlanStats(terminal, array, maskArray)
				if err != nil || handled {
					return out, handled, err
				}
			}
		}
		index, handled, err := s.evalQScriptBindingPlan(&descriptor.indexPlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			if err := s.evalQScriptPipelineDeferredAssignment(descriptor, descriptor.indexExpr); err != nil {
				return nil, true, err
			}
			index, handled, err = s.evalQScriptBindingPlan(&descriptor.indexPlan)
			if err != nil {
				return nil, true, err
			}
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		indexes, ok := index.(data.Array)
		if !ok {
			return nil, false, nil
		}
		return qPipelineGatherReduceSumWithPlanStats(terminal, array, indexes)
	case qPipelineSumGatherIndexes:
		value, handled, err := s.evalQScriptBindingPlan(&descriptor.valuePlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		array, ok := value.(data.Array)
		if !ok {
			return nil, false, nil
		}
		if descriptor.kind == qScriptPipelineWhereIndexReduceSum {
			if descriptor.moduloMaskPlan != nil {
				if out, handled, err := s.evalQPipelineModuloCompareValueSum(*descriptor.moduloMaskPlan, array); err != nil || handled {
					return out, handled, err
				}
			}
		}
		index, handled, err := s.evalQScriptBindingPlan(&descriptor.indexPlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			if err := s.evalQScriptPipelineDeferredAssignment(descriptor, descriptor.indexExpr); err != nil {
				return nil, true, err
			}
			index, handled, err = s.evalQScriptBindingPlan(&descriptor.indexPlan)
			if err != nil {
				return nil, true, err
			}
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		indexes, ok := index.(data.Array)
		if !ok {
			return nil, false, nil
		}
		return qPipelineGatherReduceSumWithPlanStats(terminal, array, indexes)
	case qPipelineSumWhereMask:
		value, handled, err := s.evalQScriptBindingPlan(&descriptor.valuePlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		array, ok := value.(data.Array)
		if !ok {
			return nil, false, nil
		}
		mask, handled, err := s.evalQScriptBindingPlan(&descriptor.maskPlan)
		if err != nil {
			return nil, true, err
		}
		if !handled {
			return s.evalQPipelinePlan(terminal)
		}
		maskArray, ok := mask.(data.Array)
		if !ok {
			return nil, false, nil
		}
		return qPipelineWhereReduceSumWithPlanStats(terminal, array, maskArray)
	default:
		return s.evalQPipelinePlan(terminal)
	}
}

func qScriptPipelineCanDeferAssignment(descriptor *qScriptPipelineDescriptor, assignment qScriptPipelineAssignment) bool {
	if descriptor == nil || descriptor.kind != qScriptPipelineWhereIndexReduceSum {
		return false
	}
	if strings.TrimSpace(descriptor.indexExpr) != assignment.name {
		return false
	}
	return descriptor.moduloMaskPlan != nil
}

func qScriptPipelineModuloMaskPlan(maskExpr string) *qPipelinePlan {
	if maskExpr == "" {
		return nil
	}
	plan, ok := qPipelineModuloComparePlanFromMask(maskExpr)
	if !ok {
		return nil
	}
	return &plan
}

func (s *EvalState) evalQScriptPipelineDeferredAssignment(descriptor *qScriptPipelineDescriptor, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if _, ok := s.lookupName(name); ok {
		return nil
	}
	for _, assignment := range descriptor.assignments {
		if assignment.name != name {
			continue
		}
		value, handled, err := s.evalQScriptBindingPlan(&assignment.binding)
		if err != nil {
			return err
		}
		if !handled {
			value, err = s.evalCachedOrString(assignment.rhs, assignment.valueExpr, &assignment.binding, nil)
			if err != nil {
				return err
			}
		}
		s.env[s.resolveAssignmentName(assignment.name)] = value
		return nil
	}
	return nil
}
