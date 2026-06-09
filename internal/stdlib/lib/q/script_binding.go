package q

import (
	"fmt"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qScriptBindingKind uint8

const (
	qScriptBindingInvalid qScriptBindingKind = iota
	qScriptBindingLiteral
	qScriptBindingName
	qScriptBindingVector
	qScriptBindingUnary
	qScriptBindingBinary
	qScriptBindingIndex
)

type qScriptBindingPlan struct {
	kind    qScriptBindingKind
	op      string
	name    string
	literal any
	items   []qScriptBindingPlan
	left    *qScriptBindingPlan
	right   *qScriptBindingPlan
}

func buildQScriptBindingPlan(expr Expr) qScriptBindingPlan {
	switch x := expr.(type) {
	case nil:
		return qScriptBindingPlan{}
	case Number:
		value, _, err := parseNumberOrBool(x.Text)
		if err != nil {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: value}
	case String:
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: x.Value}
	case Symbol:
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: data.Symbol(x.Name)}
	case Bool:
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: x.Value}
	case Null:
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: data.NullValue}
	case Temporal:
		value, err := parseQTemporal(x.Kind, x.Text)
		if err != nil {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: value}
	case TypedNull:
		return qScriptBindingPlan{kind: qScriptBindingLiteral, literal: data.NullForKind(data.Kind(x.Kind))}
	case Ident:
		return qScriptBindingPlan{kind: qScriptBindingName, name: x.Name}
	case Vector:
		items := make([]qScriptBindingPlan, len(x.Items))
		for i, item := range x.Items {
			items[i] = buildQScriptBindingPlan(item)
			if items[i].kind == qScriptBindingInvalid {
				return qScriptBindingPlan{}
			}
		}
		return qScriptBindingPlan{kind: qScriptBindingVector, items: items}
	case Call:
		arg := buildQScriptBindingPlan(x.Arg)
		if arg.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingUnary, op: x.Func, left: &arg}
	case Binary:
		left := buildQScriptBindingPlan(x.Left)
		right := buildQScriptBindingPlan(x.Right)
		if left.kind == qScriptBindingInvalid || right.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingBinary, op: x.Op, left: &left, right: &right}
	case IndexExpr:
		left := buildQScriptBindingPlan(x.Expr)
		right := buildQScriptBindingPlan(x.Index)
		if left.kind == qScriptBindingInvalid || right.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingIndex, left: &left, right: &right}
	default:
		return qScriptBindingPlan{}
	}
}

func buildQScriptBindingPlanForRHS(src string, expr Expr) qScriptBindingPlan {
	if expr == nil {
		parsed, ok, err := parseValueExpr(src)
		if err != nil || !ok {
			return buildQScriptPrefixBindingPlan(src)
		}
		expr = parsed
	}
	plan := buildQScriptBindingPlan(expr)
	if plan.kind != qScriptBindingInvalid {
		return plan
	}
	return buildQScriptPrefixBindingPlan(src)
}

func buildQScriptPrefixBindingPlan(src string) qScriptBindingPlan {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "where ") && wordBoundary(src, 0, len("where")) {
		arg := strings.TrimSpace(src[len("where "):])
		expr, ok, err := parseValueExpr(arg)
		if err != nil || !ok {
			return qScriptBindingPlan{}
		}
		argPlan := buildQScriptBindingPlan(expr)
		if argPlan.kind == qScriptBindingInvalid {
			return qScriptBindingPlan{}
		}
		return qScriptBindingPlan{kind: qScriptBindingUnary, op: "where", left: &argPlan}
	}
	return qScriptBindingPlan{}
}

func (s *EvalState) evalQScriptBindingPlan(plan *qScriptBindingPlan) (any, bool, error) {
	if plan == nil {
		return nil, false, nil
	}
	switch plan.kind {
	case qScriptBindingInvalid:
		return nil, false, nil
	case qScriptBindingLiteral:
		return plan.literal, true, nil
	case qScriptBindingName:
		value, ok := s.lookupName(plan.name)
		if !ok {
			return nil, false, nil
		}
		return value, true, nil
	case qScriptBindingVector:
		values := make([]any, len(plan.items))
		for i := range plan.items {
			value, handled, err := s.evalQScriptBindingPlan(&plan.items[i])
			if err != nil || !handled {
				return nil, handled, err
			}
			values[i] = value
		}
		out, err := evalValueVector(values)
		return out, true, err
	case qScriptBindingUnary:
		return s.evalQScriptUnaryBinding(plan)
	case qScriptBindingBinary:
		return s.evalQScriptBinaryBinding(plan)
	case qScriptBindingIndex:
		collection, handled, err := s.evalQScriptBindingPlan(plan.left)
		if err != nil || !handled {
			return nil, handled, err
		}
		index, handled, err := s.evalQScriptBindingPlan(plan.right)
		if err != nil || !handled {
			return nil, handled, err
		}
		if isCallable(collection) {
			out, err := s.applyCallable(collection, []any{index})
			return out, true, err
		}
		out, err := indexValue(collection, index)
		return out, true, err
	default:
		return nil, false, nil
	}
}

func (s *EvalState) evalQScriptUnaryBinding(plan *qScriptBindingPlan) (any, bool, error) {
	arg, handled, err := s.evalQScriptBindingPlan(plan.left)
	if err != nil || !handled {
		return nil, handled, err
	}
	switch plan.op {
	case "til":
		n, ok := integerValue(arg)
		if !ok {
			return nil, true, fmt.Errorf("til expects an integer")
		}
		if n < 0 {
			return nil, true, fmt.Errorf("til expects a non-negative integer")
		}
		if int64(int(n)) != n {
			return nil, true, fmt.Errorf("til count is too large")
		}
		return data.NewI64Range(0, 1, int(n)), true, nil
	case "where":
		if mask, ok := arg.(data.Array); ok && mask.Kind() == data.KindBool {
			out, handled, err := data.TryTypedWhereMaskI64(mask)
			recordRuntimeKernelProbe("ArrayWhere", "mask-to-index/i64", handled, err)
			if err != nil || handled {
				return out, true, err
			}
		}
		out, err := where(arg)
		return out, true, err
	}
	fn, ok := lookupUnaryVerb(plan.op)
	if !ok {
		return nil, false, nil
	}
	out, err := fn(arg)
	return out, true, err
}

func (s *EvalState) evalQScriptBinaryBinding(plan *qScriptBindingPlan) (any, bool, error) {
	left, handled, err := s.evalQScriptBindingPlan(plan.left)
	if err != nil || !handled {
		return nil, handled, err
	}
	right, handled, err := s.evalQScriptBindingPlan(plan.right)
	if err != nil || !handled {
		return nil, handled, err
	}
	if plan.op == "and" || plan.op == "or" {
		if out, handled, err := data.TryTypedBoolLogical(plan.op, left, right); err != nil || handled {
			recordRuntimeKernelProbe("ArrayBoolLogical", "logical/"+plan.op, handled, err)
			if err != nil {
				return nil, true, err
			}
			return out, true, nil
		}
		out, err := evalValueBinary(plan.op, left, right)
		return out, true, err
	}
	if dataOp, ok := qDataCompareOpString(plan.op); ok {
		la, _ := left.(data.Array)
		ra, _ := right.(data.Array)
		if la != nil || ra != nil {
			out, handled, err := qTryTypedCompareMask(dataOp, left, right, la, ra)
			recordRuntimeKernelProbe("ArrayDyadicCompare", qRuntimeKernelCompositeVectorDyadicShape(plan.op, left, right, la, ra), handled, err)
			if err != nil {
				return nil, true, err
			}
			if handled {
				return out, true, nil
			}
		}
	}
	if len(plan.op) == 1 {
		op := plan.op[0]
		if dataOp, ok := qDataArithmeticOp(op); ok {
			la, _ := left.(data.Array)
			ra, _ := right.(data.Array)
			if la != nil || ra != nil {
				typedLeft, typedRight, canUse, err := qVectorDyadicTypedOperands(left, right, la, ra)
				if err != nil {
					return nil, true, err
				}
				if canUse && qVectorDyadicCanUseTypedArithmetic(typedLeft, typedRight) {
					out, handled, err := qTryTypedArithmeticDyadic(dataOp, typedLeft, typedRight)
					recordRuntimeKernelProbe("ArrayDyadicArithmetic", qRuntimeKernelVectorDyadicShape(op, left, right, la, ra), handled, err)
					if err != nil {
						return nil, true, err
					}
					if handled {
						return out, true, nil
					}
				}
			}
		}
	}
	out, err := evalValueBinary(plan.op, left, right)
	return out, true, err
}
