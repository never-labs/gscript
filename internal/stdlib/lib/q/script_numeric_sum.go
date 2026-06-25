package q

import (
	"fmt"
	"math"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

type qScriptNumericSumPlan struct {
	source    string
	sources   []string
	root      qScriptNumericExprPlan
	count     *qScriptNumericExprPlan
	closed    qScriptNumericSumSummary
	closedOK  bool
	closedErr string
}

type qScriptNumericExprKind uint8

const (
	qScriptNumericExprInvalid qScriptNumericExprKind = iota
	qScriptNumericExprScalar
	qScriptNumericExprTil
	qScriptNumericExprName
	qScriptNumericExprCast
	qScriptNumericExprBinary
)

type qScriptNumericExprPlan struct {
	kind        qScriptNumericExprKind
	value       int64
	name        string
	cast        data.Kind
	op          string
	left        *qScriptNumericExprPlan
	right       *qScriptNumericExprPlan
	nonNegative bool
	hasCast     bool
	isFloat     bool
	fvalue      float64
}

func buildQScriptNumericSumPlan(statements []qScriptStatement) *qScriptNumericSumPlan {
	if len(statements) < 2 {
		return nil
	}
	terminal := strings.TrimSpace(statements[len(statements)-1].src)
	terminalPlan, ok := qScriptNumericSumTerminal(terminal)
	if !ok {
		return nil
	}
	bindings := make(map[string]qScriptNumericExprPlan, len(statements)-1)
	for i := 0; i < len(statements)-1; i++ {
		stmt := statements[i]
		if stmt.assign == "" || stmt.idxAssignName != "" || stmt.rhs == "" {
			return nil
		}
		expr := compileQEvalExpr(stmt.rhs, 0)
		if expr == nil {
			return nil
		}
		plan, ok := buildQScriptNumericExprPlan(expr, bindings)
		if !ok {
			return nil
		}
		bindings[stmt.assign] = plan
	}
	root, ok := bindings[terminalPlan.sumName]
	if !ok {
		return nil
	}
	if _, ok := qScriptNumericExprLength(root, bindings); !ok {
		return nil
	}
	var count *qScriptNumericExprPlan
	if terminalPlan.countName != "" {
		countPlan, ok := bindings[terminalPlan.countName]
		if !ok {
			return nil
		}
		sumLen, sumOK := qScriptNumericExprLength(root, bindings)
		countLen, countOK := qScriptNumericExprLength(countPlan, bindings)
		if !sumOK || !countOK || sumLen != countLen || sumLen < 0 {
			return nil
		}
		count = &countPlan
	}
	if !root.hasCast {
		return nil
	}
	plan := &qScriptNumericSumPlan{source: terminal, sources: qScriptNumericSumStatementSources(statements), root: root, count: count}
	if summary, ok, err := qScriptNumericSummarize(root); ok {
		plan.closed = summary
		plan.closedOK = true
		if err != nil {
			plan.closedErr = err.Error()
		}
	}
	if plan.closedErr == "" && !plan.closedOK && root.isFloat {
		return nil
	}
	return plan
}

type qScriptNumericSumTerminalPlan struct {
	sumName   string
	countName string
}

func qScriptNumericSumTerminal(src string) (qScriptNumericSumTerminalPlan, bool) {
	if name, ok := qScriptNumericSumTargetName(src); ok {
		return qScriptNumericSumTerminalPlan{sumName: name}, true
	}
	expr := compileQEvalExpr(src, 0)
	binary, ok := expr.(Binary)
	if !ok || binary.Op != "+" {
		return qScriptNumericSumTerminalPlan{}, false
	}
	if sumName, ok := qScriptNumericSumExprName(binary.Left); ok {
		if countName, ok := qScriptNumericCountExprName(binary.Right); ok {
			return qScriptNumericSumTerminalPlan{sumName: sumName, countName: countName}, true
		}
	}
	if countName, ok := qScriptNumericCountExprName(binary.Left); ok {
		if sumName, ok := qScriptNumericSumExprName(binary.Right); ok {
			return qScriptNumericSumTerminalPlan{sumName: sumName, countName: countName}, true
		}
	}
	return qScriptNumericSumTerminalPlan{}, false
}

func qScriptNumericSumTargetName(src string) (string, bool) {
	if strings.HasPrefix(src, "+/") {
		name := strings.TrimSpace(src[2:])
		if isQAssignmentName(name) {
			return name, true
		}
		return "", false
	}
	if strings.HasPrefix(src, "sum ") && wordBoundary(src, 0, len("sum")) {
		name := strings.TrimSpace(src[len("sum "):])
		if isQAssignmentName(name) {
			return name, true
		}
	}
	return "", false
}

func qScriptNumericSumExprName(expr Expr) (string, bool) {
	switch x := expr.(type) {
	case SafeCall:
		if x.Func != "sum" {
			return "", false
		}
		if ident, ok := x.Arg.(Ident); ok {
			return ident.Name, true
		}
	case Call:
		if x.Func != "sum" {
			return "", false
		}
		if ident, ok := x.Arg.(Ident); ok {
			return ident.Name, true
		}
	}
	return "", false
}

func qScriptNumericCountExprName(expr Expr) (string, bool) {
	switch x := expr.(type) {
	case SafeCall:
		if x.Func != "count" {
			return "", false
		}
		if ident, ok := x.Arg.(Ident); ok {
			return ident.Name, true
		}
	case Call:
		if x.Func != "count" {
			return "", false
		}
		if ident, ok := x.Arg.(Ident); ok {
			return ident.Name, true
		}
	}
	return "", false
}

func buildQScriptNumericExprPlan(expr Expr, bindings map[string]qScriptNumericExprPlan) (qScriptNumericExprPlan, bool) {
	switch x := expr.(type) {
	case Const:
		return qScriptNumericScalarPlan(x.Value)
	case Number:
		value, _, err := parseNumberOrBool(x.Text)
		if err != nil {
			return qScriptNumericExprPlan{}, false
		}
		return qScriptNumericScalarPlan(value)
	case Ident:
		referenced, ok := bindings[x.Name]
		if !ok {
			return qScriptNumericExprPlan{}, false
		}
		return qScriptNumericExprPlan{kind: qScriptNumericExprName, name: x.Name, left: &referenced, nonNegative: referenced.nonNegative, hasCast: referenced.hasCast}, true
	case Call:
		if x.Func != "til" {
			return qScriptNumericExprPlan{}, false
		}
		count, ok := qScriptNumericLiteralI64(x.Arg)
		if !ok || count < 0 || count > int64(math.MaxInt) {
			return qScriptNumericExprPlan{}, false
		}
		return qScriptNumericExprPlan{kind: qScriptNumericExprTil, value: count, nonNegative: true}, true
	case CastExpr:
		kind, ok := qScriptNumericCastKind(x)
		if !ok || !qScriptNumericIntegerKind(kind) {
			return qScriptNumericExprPlan{}, false
		}
		value, ok := buildQScriptNumericExprPlan(x.Value, bindings)
		if !ok {
			return qScriptNumericExprPlan{}, false
		}
		return qScriptNumericExprPlan{kind: qScriptNumericExprCast, cast: kind, left: &value, nonNegative: value.nonNegative, hasCast: true}, true
	case Binary:
		return buildQScriptNumericBinaryPlan(x.Op, x.Left, x.Right, bindings)
	case DyadicWordExpr:
		if x.Set {
			return qScriptNumericExprPlan{}, false
		}
		return buildQScriptNumericBinaryPlan(x.Word, x.Left, x.Right, bindings)
	default:
		return qScriptNumericExprPlan{}, false
	}
}

func buildQScriptNumericBinaryPlan(op string, leftExpr, rightExpr Expr, bindings map[string]qScriptNumericExprPlan) (qScriptNumericExprPlan, bool) {
	switch op {
	case "+", "-", "*", "%", "div", "mod":
	default:
		return qScriptNumericExprPlan{}, false
	}
	left, ok := buildQScriptNumericExprPlan(leftExpr, bindings)
	if !ok {
		return qScriptNumericExprPlan{}, false
	}
	right, ok := buildQScriptNumericExprPlan(rightExpr, bindings)
	if !ok {
		return qScriptNumericExprPlan{}, false
	}
	if op == "mod" {
		if right.kind != qScriptNumericExprScalar || right.value <= 0 || !left.nonNegative {
			return qScriptNumericExprPlan{}, false
		}
	}
	if op == "div" {
		if right.kind != qScriptNumericExprScalar || right.value <= 0 || !left.nonNegative {
			return qScriptNumericExprPlan{}, false
		}
	}
	plan := qScriptNumericExprPlan{kind: qScriptNumericExprBinary, op: op, left: &left, right: &right}
	switch op {
	case "+":
		plan.nonNegative = left.nonNegative && right.nonNegative
	case "*":
		plan.nonNegative = left.nonNegative && right.nonNegative
	case "mod":
		plan.nonNegative = true
	case "div":
		plan.nonNegative = true
	}
	plan.hasCast = left.hasCast || right.hasCast
	plan.isFloat = left.isFloat || right.isFloat || op == "%"
	return plan, true
}

func qScriptNumericScalarPlan(value any) (qScriptNumericExprPlan, bool) {
	switch v := value.(type) {
	case int:
		return qScriptNumericExprPlan{kind: qScriptNumericExprScalar, value: int64(v), nonNegative: v >= 0}, true
	case int8:
		return qScriptNumericExprPlan{kind: qScriptNumericExprScalar, value: int64(v), nonNegative: v >= 0}, true
	case int16:
		return qScriptNumericExprPlan{kind: qScriptNumericExprScalar, value: int64(v), nonNegative: v >= 0}, true
	case int32:
		return qScriptNumericExprPlan{kind: qScriptNumericExprScalar, value: int64(v), nonNegative: v >= 0}, true
	case int64:
		return qScriptNumericExprPlan{kind: qScriptNumericExprScalar, value: v, nonNegative: v >= 0}, true
	case float32:
		return qScriptNumericExprPlan{kind: qScriptNumericExprScalar, fvalue: float64(v), nonNegative: v >= 0, isFloat: true}, true
	case float64:
		return qScriptNumericExprPlan{kind: qScriptNumericExprScalar, fvalue: v, nonNegative: v >= 0, isFloat: true}, true
	default:
		return qScriptNumericExprPlan{}, false
	}
}

func qScriptNumericLiteralI64(expr Expr) (int64, bool) {
	plan, ok := buildQScriptNumericExprPlan(expr, nil)
	if !ok || plan.kind != qScriptNumericExprScalar {
		return 0, false
	}
	return plan.value, true
}

func qScriptNumericCastKind(x CastExpr) (data.Kind, bool) {
	text := strings.TrimSpace(x.DomainSrc)
	text = strings.TrimPrefix(text, "`")
	if kind, ok := qCastKindFromTypeText(text); ok {
		return kind, true
	}
	if x.BareSym != "" {
		return qCastKindFromSymbol(x.BareSym)
	}
	if x.Domain != nil {
		if c, ok := x.Domain.(Const); ok {
			if sym, ok := c.Value.(data.Symbol); ok {
				return qCastKindFromSymbol(sym)
			}
			if text, ok := c.Value.(string); ok {
				return qCastKindFromTypeText(text)
			}
		}
		if sym, ok := x.Domain.(Symbol); ok {
			return qCastKindFromTypeText(sym.Name)
		}
		if text, ok := x.Domain.(String); ok {
			return qCastKindFromTypeText(text.Value)
		}
	}
	return "", false
}

func qScriptNumericIntegerKind(kind data.Kind) bool {
	switch kind {
	case data.KindI16, data.KindI32, data.KindI64, data.KindF32, data.KindF64:
		return true
	default:
		return false
	}
}

func qScriptNumericExprLength(plan qScriptNumericExprPlan, bindings map[string]qScriptNumericExprPlan) (int, bool) {
	switch plan.kind {
	case qScriptNumericExprScalar:
		return -1, true
	case qScriptNumericExprTil:
		return int(plan.value), true
	case qScriptNumericExprName:
		if plan.left != nil {
			return qScriptNumericExprLength(*plan.left, bindings)
		}
		ref, ok := bindings[plan.name]
		if !ok {
			return 0, false
		}
		return qScriptNumericExprLength(ref, bindings)
	case qScriptNumericExprCast:
		return qScriptNumericExprLength(*plan.left, bindings)
	case qScriptNumericExprBinary:
		leftLen, ok := qScriptNumericExprLength(*plan.left, bindings)
		if !ok {
			return 0, false
		}
		rightLen, ok := qScriptNumericExprLength(*plan.right, bindings)
		if !ok {
			return 0, false
		}
		switch {
		case leftLen < 0:
			return rightLen, true
		case rightLen < 0:
			return leftLen, true
		case leftLen == rightLen:
			return leftLen, true
		default:
			return 0, false
		}
	default:
		return 0, false
	}
}

func (s *EvalState) evalQScriptNumericSumPlan(plan *qScriptNumericSumPlan) (any, bool, error) {
	if plan == nil {
		return nil, false, nil
	}
	if plan.closedOK {
		if plan.closedErr != "" {
			err := fmt.Errorf("%s", plan.closedErr)
			recordRuntimeKernelProbe("QScriptNumericSumPlan", "vector-reduce/int-cast-expr-sum", false, err)
			return nil, true, err
		}
		recordRuntimeKernelProbe("QScriptNumericSumPlan", "vector-reduce/int-cast-expr-sum", true, nil)
		plan.recordDispatch()
		return plan.closed.Result(plan.countValue()), true, nil
	}
	if plan.root.isFloat {
		return nil, false, nil
	}
	bindings := make(map[string]qScriptNumericExprPlan, 8)
	// The immutable root contains inlined name references to plan-time
	// bindings; collect them into a runtime map only for the evaluator's
	// uniform resolver. This avoids materializing every assignment value.
	qScriptNumericCollectBindings(plan.root, bindings)
	length, ok := qScriptNumericExprLength(plan.root, bindings)
	if !ok {
		return nil, false, nil
	}
	if length < 0 {
		value, err := qScriptNumericEvalRow(plan.root, 0, bindings)
		if err != nil {
			return nil, true, err
		}
		recordQEvalDispatch(plan.source, EvalDispatchScriptNumericSum)
		return value, true, nil
	}
	var sum int64
	for row := 0; row < length; row++ {
		value, err := qScriptNumericEvalRow(plan.root, row, bindings)
		if err != nil {
			recordRuntimeKernelProbe("QScriptNumericSumPlan", "vector-reduce/int-cast-expr-sum", false, err)
			return nil, true, err
		}
		sum += value
	}
	recordRuntimeKernelProbe("QScriptNumericSumPlan", "vector-reduce/int-cast-expr-sum", true, nil)
	plan.recordDispatch()
	return sum + plan.countValue(), true, nil
}

func qScriptNumericSumStatementSources(statements []qScriptStatement) []string {
	sources := make([]string, 0, len(statements))
	for _, stmt := range statements {
		if stmt.src != "" {
			sources = append(sources, stmt.src)
		}
	}
	return sources
}

func (plan *qScriptNumericSumPlan) recordDispatch() {
	if len(plan.sources) == 0 {
		recordQEvalDispatch(plan.source, EvalDispatchScriptNumericSum)
		return
	}
	for _, source := range plan.sources {
		recordQEvalDispatch(source, EvalDispatchScriptNumericSum)
	}
}

func (plan *qScriptNumericSumPlan) countValue() int64 {
	if plan == nil || plan.count == nil {
		return 0
	}
	if length, ok := qScriptNumericExprLength(*plan.count, nil); ok && length >= 0 {
		return int64(length)
	}
	return 0
}

func qScriptNumericCollectBindings(plan qScriptNumericExprPlan, out map[string]qScriptNumericExprPlan) {
	switch plan.kind {
	case qScriptNumericExprName:
		if _, ok := out[plan.name]; !ok && plan.left != nil {
			out[plan.name] = *plan.left
			qScriptNumericCollectBindings(*plan.left, out)
		}
	case qScriptNumericExprCast:
		qScriptNumericCollectBindings(*plan.left, out)
	case qScriptNumericExprBinary:
		qScriptNumericCollectBindings(*plan.left, out)
		qScriptNumericCollectBindings(*plan.right, out)
	}
}

func qScriptNumericEvalRow(plan qScriptNumericExprPlan, row int, bindings map[string]qScriptNumericExprPlan) (int64, error) {
	switch plan.kind {
	case qScriptNumericExprScalar:
		return plan.value, nil
	case qScriptNumericExprTil:
		return int64(row), nil
	case qScriptNumericExprName:
		if plan.left != nil {
			return qScriptNumericEvalRow(*plan.left, row, bindings)
		}
		ref, ok := bindings[plan.name]
		if !ok {
			return 0, fmt.Errorf("unbound name %s", plan.name)
		}
		return qScriptNumericEvalRow(ref, row, bindings)
	case qScriptNumericExprCast:
		value, err := qScriptNumericEvalRow(*plan.left, row, bindings)
		if err != nil {
			return 0, err
		}
		return qScriptNumericCastValue(plan.cast, value, row)
	case qScriptNumericExprBinary:
		left, err := qScriptNumericEvalRow(*plan.left, row, bindings)
		if err != nil {
			return 0, err
		}
		right, err := qScriptNumericEvalRow(*plan.right, row, bindings)
		if err != nil {
			return 0, err
		}
		switch plan.op {
		case "+":
			return left + right, nil
		case "-":
			return left - right, nil
		case "*":
			return left * right, nil
		case "mod":
			return left % right, nil
		case "div":
			return left / right, nil
		default:
			return 0, fmt.Errorf("unsupported numeric op %s", plan.op)
		}
	default:
		return 0, fmt.Errorf("invalid numeric expression")
	}
}

func qScriptNumericCastValue(kind data.Kind, value int64, row int) (int64, error) {
	switch kind {
	case data.KindI16:
		if value < -32768 || value > 32767 {
			return 0, fmt.Errorf("value %d must be i16 for %s", row+1, kind)
		}
		return int64(int16(value)), nil
	case data.KindI32:
		if value < -2147483648 || value > 2147483647 {
			return 0, fmt.Errorf("value %d must be i32 for %s", row+1, kind)
		}
		return int64(int32(value)), nil
	case data.KindI64:
		return value, nil
	default:
		return 0, fmt.Errorf("unsupported numeric cast %s", kind)
	}
}

type qScriptNumericSumSummary struct {
	length  int
	scalar  bool
	value   int64
	linear  bool
	start   int64
	step    int64
	period  []int64
	min     int64
	max     int64
	isFloat bool
	fvalue  float64
	flinear bool
	fstart  float64
	fstep   float64
	fperiod []float64
	fmin    float64
	fmax    float64
}

const qScriptNumericClosedFormMaxPeriod = 1 << 16

func qScriptNumericSummarize(plan qScriptNumericExprPlan) (qScriptNumericSumSummary, bool, error) {
	switch plan.kind {
	case qScriptNumericExprScalar:
		if plan.isFloat {
			return qScriptNumericSumSummary{length: -1, scalar: true, isFloat: true, fvalue: plan.fvalue, fmin: plan.fvalue, fmax: plan.fvalue}, true, nil
		}
		return qScriptNumericSumSummary{length: -1, scalar: true, value: plan.value, min: plan.value, max: plan.value}, true, nil
	case qScriptNumericExprTil:
		n := int(plan.value)
		if n == 0 {
			return qScriptNumericSumSummary{length: 0, linear: true, min: 0, max: -1}, true, nil
		}
		return qScriptNumericSumSummary{length: n, linear: true, start: 0, step: 1, min: 0, max: int64(n - 1)}, true, nil
	case qScriptNumericExprName:
		if plan.left == nil {
			return qScriptNumericSumSummary{}, false, nil
		}
		return qScriptNumericSummarize(*plan.left)
	case qScriptNumericExprCast:
		left, ok, err := qScriptNumericSummarize(*plan.left)
		if !ok || err != nil {
			return left, ok, err
		}
		if err := qScriptNumericSumSummaryCastCheck(plan.cast, left); err != nil {
			return left, true, err
		}
		return qScriptNumericCastSummary(plan.cast, left)
	case qScriptNumericExprBinary:
		left, ok, err := qScriptNumericSummarize(*plan.left)
		if !ok || err != nil {
			return left, ok, err
		}
		right, ok, err := qScriptNumericSummarize(*plan.right)
		if !ok || err != nil {
			return right, ok, err
		}
		return qScriptNumericSummarizeBinary(plan.op, left, right)
	default:
		return qScriptNumericSumSummary{}, false, nil
	}
}

func qScriptNumericSumSummaryCastCheck(kind data.Kind, summary qScriptNumericSumSummary) error {
	switch kind {
	case data.KindI16:
		if summary.isFloat {
			return qScriptNumericSumSummaryFloatCastCheck(kind, summary)
		}
		if summary.min < -32768 || summary.max > 32767 {
			return fmt.Errorf("value %d must be i16 for %s", qScriptNumericSumSummaryFirstOutOfRangeRow(summary, -32768, 32767), kind)
		}
	case data.KindI32:
		if summary.isFloat {
			return qScriptNumericSumSummaryFloatCastCheck(kind, summary)
		}
		if summary.min < -2147483648 || summary.max > 2147483647 {
			return fmt.Errorf("value %d must be i32 for %s", qScriptNumericSumSummaryFirstOutOfRangeRow(summary, -2147483648, 2147483647), kind)
		}
	case data.KindI64:
		if summary.isFloat {
			return qScriptNumericSumSummaryFloatCastCheck(kind, summary)
		}
		return nil
	case data.KindF32, data.KindF64:
		return nil
	default:
		return fmt.Errorf("unsupported numeric cast %s", kind)
	}
	return nil
}

func qScriptNumericSumSummaryFloatCastCheck(kind data.Kind, summary qScriptNumericSumSummary) error {
	n := summary.length
	if n < 0 {
		n = 1
	}
	for row := 0; row < n; row++ {
		value := math.RoundToEven(summary.FloatAt(row))
		switch kind {
		case data.KindI16:
			if math.IsNaN(value) || math.IsInf(value, 0) || value < -32768 || value > 32767 {
				return fmt.Errorf("value %d must be i16 for %s", row+1, kind)
			}
		case data.KindI32:
			if math.IsNaN(value) || math.IsInf(value, 0) || value < -2147483648 || value > 2147483647 {
				return fmt.Errorf("value %d must be i32 for %s", row+1, kind)
			}
		case data.KindI64:
			if math.IsNaN(value) || math.IsInf(value, 0) || value < -9223372036854775808.0 || value >= 9223372036854775808.0 {
				return fmt.Errorf("value %d must be i64 for %s", row+1, kind)
			}
		}
	}
	return nil
}

func qScriptNumericCastSummary(kind data.Kind, summary qScriptNumericSumSummary) (qScriptNumericSumSummary, bool, error) {
	switch kind {
	case data.KindF32:
		return qScriptNumericCastSummaryF32(summary), true, nil
	case data.KindF64:
		return summary.AsFloat(), true, nil
	case data.KindI16, data.KindI32, data.KindI64:
		if !summary.isFloat {
			return summary, true, nil
		}
		return qScriptNumericCastSummaryInteger(summary)
	default:
		return qScriptNumericSumSummary{}, false, nil
	}
}

func qScriptNumericCastSummaryF32(summary qScriptNumericSumSummary) qScriptNumericSumSummary {
	summary = summary.AsFloat()
	out := qScriptNumericSumSummary{length: summary.length, scalar: summary.scalar, isFloat: true}
	if summary.scalar {
		out.fvalue = float64(float32(summary.fvalue))
		out.fmin, out.fmax = out.fvalue, out.fvalue
		return out
	}
	if summary.flinear {
		out.flinear = true
		out.fstart = float64(float32(summary.fstart))
		out.fstep = float64(float32(summary.fstep))
		out.fmin, out.fmax = qScriptNumericFloatLinearMinMax(out)
		return out
	}
	if len(summary.fperiod) > 0 {
		period := make([]float64, len(summary.fperiod))
		for i, value := range summary.fperiod {
			period[i] = float64(float32(value))
		}
		return qScriptNumericFloatPeriodSummary(summary.length, period)
	}
	return out
}

func qScriptNumericCastSummaryInteger(summary qScriptNumericSumSummary) (qScriptNumericSumSummary, bool, error) {
	if summary.scalar {
		value := int64(math.RoundToEven(summary.fvalue))
		return qScriptNumericSumSummary{length: -1, scalar: true, value: value, min: value, max: value}, true, nil
	}
	if summary.flinear {
		start := math.RoundToEven(summary.fstart)
		step := math.RoundToEven(summary.fstep)
		if start != summary.fstart || step != summary.fstep {
			return qScriptNumericSumSummary{}, false, nil
		}
		out := qScriptNumericSumSummary{length: summary.length, linear: true, start: int64(start), step: int64(step)}
		out.min, out.max = qScriptNumericLinearMinMax(out)
		return out, true, nil
	}
	if len(summary.fperiod) > 0 {
		period := make([]int64, len(summary.fperiod))
		for i, value := range summary.fperiod {
			period[i] = int64(math.RoundToEven(value))
		}
		return qScriptNumericPeriodSummary(summary.length, period), true, nil
	}
	return qScriptNumericSumSummary{}, false, nil
}

func qScriptNumericSumSummaryFirstOutOfRangeRow(summary qScriptNumericSumSummary, min, max int64) int {
	n := summary.length
	if n < 0 {
		n = 1
	}
	for row := 0; row < n; row++ {
		value := summary.At(row)
		if value < min || value > max {
			return row + 1
		}
	}
	return 1
}

func qScriptNumericSummarizeBinary(op string, left, right qScriptNumericSumSummary) (qScriptNumericSumSummary, bool, error) {
	if op == "%" || left.isFloat || right.isFloat {
		return qScriptNumericSummarizeFloatBinary(op, left, right)
	}
	if op == "mod" {
		if !right.scalar || right.value <= 0 {
			return qScriptNumericSumSummary{}, false, nil
		}
		return qScriptNumericSummarizeMod(left, right.value)
	}
	if op == "div" {
		if !right.scalar || right.value <= 0 {
			return qScriptNumericSumSummary{}, false, nil
		}
		return qScriptNumericSummarizeDiv(left, right.value)
	}
	length, ok := qScriptNumericSumSummaryResultLength(left, right)
	if !ok {
		return qScriptNumericSumSummary{}, false, nil
	}
	if left.scalar && right.scalar {
		value := qScriptNumericApplyBinary(op, left.value, right.value)
		return qScriptNumericSumSummary{length: -1, scalar: true, value: value, min: value, max: value}, true, nil
	}
	if op == "+" || op == "-" {
		if left.linear && right.scalar {
			return qScriptNumericLinearScalar(op, left, right.value), true, nil
		}
		if left.scalar && right.linear {
			if op == "+" {
				return qScriptNumericLinearScalar("+", right, left.value), true, nil
			}
			out := qScriptNumericSumSummary{length: right.length, linear: true, start: left.value - right.start, step: -right.step}
			out.min, out.max = qScriptNumericLinearMinMax(out)
			return out, true, nil
		}
	}
	if op == "*" {
		if left.linear && right.scalar {
			return qScriptNumericLinearMultiply(left, right.value), true, nil
		}
		if left.scalar && right.linear {
			return qScriptNumericLinearMultiply(right, left.value), true, nil
		}
	}
	if out, ok := qScriptNumericPeriodicBinary(op, left, right, length); ok {
		return out, true, nil
	}
	return qScriptNumericSumSummary{}, false, nil
}

func qScriptNumericSummarizeFloatBinary(op string, left, right qScriptNumericSumSummary) (qScriptNumericSumSummary, bool, error) {
	length, ok := qScriptNumericSumSummaryResultLength(left, right)
	if !ok {
		return qScriptNumericSumSummary{}, false, nil
	}
	left = left.AsFloat()
	right = right.AsFloat()
	if op == "mod" {
		if !right.scalar || right.fvalue == 0 {
			return qScriptNumericSumSummary{}, false, nil
		}
		return qScriptNumericSummarizeFloatMod(left, right.fvalue)
	}
	if left.scalar && right.scalar {
		value := qScriptNumericApplyFloatBinary(op, left.fvalue, right.fvalue)
		return qScriptNumericSumSummary{length: -1, scalar: true, isFloat: true, fvalue: value, fmin: value, fmax: value}, true, nil
	}
	if op == "+" || op == "-" {
		if left.flinear && right.scalar {
			return qScriptNumericFloatLinearScalar(op, left, right.fvalue), true, nil
		}
		if left.scalar && right.flinear {
			if op == "+" {
				return qScriptNumericFloatLinearScalar("+", right, left.fvalue), true, nil
			}
			out := qScriptNumericSumSummary{length: right.length, isFloat: true, flinear: true, fstart: left.fvalue - right.fstart, fstep: -right.fstep}
			out.fmin, out.fmax = qScriptNumericFloatLinearMinMax(out)
			return out, true, nil
		}
	}
	if op == "*" {
		if left.flinear && right.scalar {
			return qScriptNumericFloatLinearMultiply(left, right.fvalue), true, nil
		}
		if left.scalar && right.flinear {
			return qScriptNumericFloatLinearMultiply(right, left.fvalue), true, nil
		}
	}
	if op == "%" {
		if left.flinear && right.scalar && right.fvalue != 0 {
			return qScriptNumericFloatLinearMultiply(left, 1/right.fvalue), true, nil
		}
		if len(left.fperiod) > 0 && right.scalar && right.fvalue != 0 {
			period := make([]float64, len(left.fperiod))
			for i, value := range left.fperiod {
				period[i] = value / right.fvalue
			}
			return qScriptNumericFloatPeriodSummary(length, period), true, nil
		}
	}
	if out, ok := qScriptNumericFloatPeriodicBinary(op, left, right, length); ok {
		return out, true, nil
	}
	return qScriptNumericSumSummary{}, false, nil
}

func qScriptNumericSummarizeFloatMod(left qScriptNumericSumSummary, modulus float64) (qScriptNumericSumSummary, bool, error) {
	if left.scalar {
		value := math.Mod(left.fvalue, modulus)
		return qScriptNumericSumSummary{length: -1, scalar: true, isFloat: true, fvalue: value, fmin: value, fmax: value}, true, nil
	}
	if left.flinear && left.fstart == 0 && left.fstep > 0 {
		periodLen := int(math.Ceil(modulus / left.fstep))
		if periodLen <= 0 || periodLen > qScriptNumericClosedFormMaxPeriod {
			return qScriptNumericSumSummary{}, false, nil
		}
		period := make([]float64, periodLen)
		for i := range period {
			period[i] = math.Mod(float64(i)*left.fstep, modulus)
		}
		return qScriptNumericFloatPeriodSummary(left.length, period), true, nil
	}
	if len(left.fperiod) > 0 {
		period := make([]float64, len(left.fperiod))
		for i, value := range left.fperiod {
			period[i] = math.Mod(value, modulus)
		}
		return qScriptNumericFloatPeriodSummary(left.length, period), true, nil
	}
	return qScriptNumericSumSummary{}, false, nil
}

func qScriptNumericSummarizeMod(left qScriptNumericSumSummary, modulus int64) (qScriptNumericSumSummary, bool, error) {
	if left.scalar {
		value := left.value % modulus
		return qScriptNumericSumSummary{length: -1, scalar: true, value: value, min: value, max: value}, true, nil
	}
	if left.linear && left.start == 0 && left.step == 1 {
		if modulus > qScriptNumericClosedFormMaxPeriod {
			return qScriptNumericSumSummary{}, false, nil
		}
		period := make([]int64, int(modulus))
		for i := range period {
			period[i] = int64(i)
		}
		return qScriptNumericPeriodSummary(left.length, period), true, nil
	}
	if len(left.period) > 0 {
		period := make([]int64, len(left.period))
		for i, value := range left.period {
			period[i] = value % modulus
		}
		return qScriptNumericPeriodSummary(left.length, period), true, nil
	}
	return qScriptNumericSumSummary{}, false, nil
}

func qScriptNumericSummarizeDiv(left qScriptNumericSumSummary, divisor int64) (qScriptNumericSumSummary, bool, error) {
	if left.scalar {
		value := left.value / divisor
		return qScriptNumericSumSummary{length: -1, scalar: true, value: value, min: value, max: value}, true, nil
	}
	if left.linear && left.start == 0 && left.step == 1 {
		if divisor > qScriptNumericClosedFormMaxPeriod {
			return qScriptNumericSumSummary{}, false, nil
		}
		periodLen := int(divisor)
		period := make([]int64, periodLen)
		for i := range period {
			period[i] = int64(i) / divisor
		}
		return qScriptNumericPeriodSummary(left.length, period), true, nil
	}
	if len(left.period) > 0 {
		period := make([]int64, len(left.period))
		for i, value := range left.period {
			period[i] = value / divisor
		}
		return qScriptNumericPeriodSummary(left.length, period), true, nil
	}
	return qScriptNumericSumSummary{}, false, nil
}

func qScriptNumericLinearScalar(op string, linear qScriptNumericSumSummary, scalar int64) qScriptNumericSumSummary {
	out := linear
	if op == "+" {
		out.start += scalar
	} else {
		out.start -= scalar
	}
	out.min, out.max = qScriptNumericLinearMinMax(out)
	return out
}

func qScriptNumericLinearMultiply(linear qScriptNumericSumSummary, scalar int64) qScriptNumericSumSummary {
	out := linear
	out.start *= scalar
	out.step *= scalar
	out.min, out.max = qScriptNumericLinearMinMax(out)
	return out
}

func qScriptNumericPeriodicBinary(op string, left, right qScriptNumericSumSummary, length int) (qScriptNumericSumSummary, bool) {
	leftPeriod, ok := qScriptNumericSumSummaryPeriod(left)
	if !ok {
		return qScriptNumericSumSummary{}, false
	}
	rightPeriod, ok := qScriptNumericSumSummaryPeriod(right)
	if !ok {
		return qScriptNumericSumSummary{}, false
	}
	periodLen := qScriptNumericLCM(len(leftPeriod), len(rightPeriod))
	if periodLen <= 0 || periodLen > qScriptNumericClosedFormMaxPeriod {
		return qScriptNumericSumSummary{}, false
	}
	period := make([]int64, periodLen)
	for i := range period {
		period[i] = qScriptNumericApplyBinary(op, leftPeriod[i%len(leftPeriod)], rightPeriod[i%len(rightPeriod)])
	}
	return qScriptNumericPeriodSummary(length, period), true
}

func qScriptNumericFloatPeriodicBinary(op string, left, right qScriptNumericSumSummary, length int) (qScriptNumericSumSummary, bool) {
	leftPeriod, ok := qScriptNumericFloatSummaryPeriod(left)
	if !ok {
		return qScriptNumericSumSummary{}, false
	}
	rightPeriod, ok := qScriptNumericFloatSummaryPeriod(right)
	if !ok {
		return qScriptNumericSumSummary{}, false
	}
	periodLen := qScriptNumericLCM(len(leftPeriod), len(rightPeriod))
	if periodLen <= 0 || periodLen > qScriptNumericClosedFormMaxPeriod {
		return qScriptNumericSumSummary{}, false
	}
	period := make([]float64, periodLen)
	for i := range period {
		period[i] = qScriptNumericApplyFloatBinary(op, leftPeriod[i%len(leftPeriod)], rightPeriod[i%len(rightPeriod)])
	}
	return qScriptNumericFloatPeriodSummary(length, period), true
}

func qScriptNumericSumSummaryPeriod(summary qScriptNumericSumSummary) ([]int64, bool) {
	if summary.scalar {
		return []int64{summary.value}, true
	}
	if len(summary.period) > 0 {
		return summary.period, true
	}
	return nil, false
}

func qScriptNumericFloatSummaryPeriod(summary qScriptNumericSumSummary) ([]float64, bool) {
	if !summary.isFloat {
		summary = summary.AsFloat()
	}
	if summary.scalar {
		return []float64{summary.fvalue}, true
	}
	if len(summary.fperiod) > 0 {
		return summary.fperiod, true
	}
	return nil, false
}

func qScriptNumericSumSummaryResultLength(left, right qScriptNumericSumSummary) (int, bool) {
	switch {
	case left.scalar:
		return right.length, true
	case right.scalar:
		return left.length, true
	case left.length == right.length:
		return left.length, true
	default:
		return 0, false
	}
}

func qScriptNumericApplyBinary(op string, left, right int64) int64 {
	switch op {
	case "+":
		return left + right
	case "-":
		return left - right
	case "*":
		return left * right
	case "mod":
		return left % right
	case "div":
		return left / right
	default:
		return 0
	}
}

func qScriptNumericApplyFloatBinary(op string, left, right float64) float64 {
	switch op {
	case "+":
		return left + right
	case "-":
		return left - right
	case "*":
		return left * right
	case "%":
		return left / right
	case "mod":
		return math.Mod(left, right)
	default:
		return 0
	}
}

func qScriptNumericPeriodSummary(length int, period []int64) qScriptNumericSumSummary {
	out := qScriptNumericSumSummary{length: length, period: period}
	if len(period) == 0 {
		return out
	}
	out.min, out.max = period[0], period[0]
	for _, value := range period[1:] {
		if value < out.min {
			out.min = value
		}
		if value > out.max {
			out.max = value
		}
	}
	return out
}

func qScriptNumericLinearMinMax(summary qScriptNumericSumSummary) (int64, int64) {
	if summary.length <= 0 {
		return 0, -1
	}
	first := summary.start
	last := summary.start + int64(summary.length-1)*summary.step
	if first <= last {
		return first, last
	}
	return last, first
}

func qScriptNumericFloatLinearScalar(op string, linear qScriptNumericSumSummary, scalar float64) qScriptNumericSumSummary {
	out := linear.AsFloat()
	if op == "+" {
		out.fstart += scalar
	} else {
		out.fstart -= scalar
	}
	out.fmin, out.fmax = qScriptNumericFloatLinearMinMax(out)
	return out
}

func qScriptNumericFloatLinearMultiply(linear qScriptNumericSumSummary, scalar float64) qScriptNumericSumSummary {
	out := linear.AsFloat()
	out.fstart *= scalar
	out.fstep *= scalar
	out.fmin, out.fmax = qScriptNumericFloatLinearMinMax(out)
	return out
}

func qScriptNumericFloatPeriodSummary(length int, period []float64) qScriptNumericSumSummary {
	out := qScriptNumericSumSummary{length: length, isFloat: true, fperiod: period}
	if len(period) == 0 {
		return out
	}
	out.fmin, out.fmax = period[0], period[0]
	for _, value := range period[1:] {
		if value < out.fmin {
			out.fmin = value
		}
		if value > out.fmax {
			out.fmax = value
		}
	}
	return out
}

func qScriptNumericFloatLinearMinMax(summary qScriptNumericSumSummary) (float64, float64) {
	if summary.length <= 0 {
		return 0, -1
	}
	first := summary.fstart
	last := summary.fstart + float64(summary.length-1)*summary.fstep
	if first <= last {
		return first, last
	}
	return last, first
}

func qScriptNumericLCM(a, b int) int {
	if a == 0 || b == 0 {
		return 0
	}
	return a / qScriptNumericGCD(a, b) * b
}

func qScriptNumericGCD(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

func (s qScriptNumericSumSummary) At(row int) int64 {
	switch {
	case s.scalar:
		return s.value
	case s.linear:
		return s.start + int64(row)*s.step
	case len(s.period) > 0:
		return s.period[row%len(s.period)]
	default:
		return 0
	}
}

func (s qScriptNumericSumSummary) FloatAt(row int) float64 {
	if !s.isFloat {
		return float64(s.At(row))
	}
	switch {
	case s.scalar:
		return s.fvalue
	case s.flinear:
		return s.fstart + float64(row)*s.fstep
	case len(s.fperiod) > 0:
		return s.fperiod[row%len(s.fperiod)]
	default:
		return 0
	}
}

func (s qScriptNumericSumSummary) AsFloat() qScriptNumericSumSummary {
	if s.isFloat {
		return s
	}
	out := qScriptNumericSumSummary{length: s.length, scalar: s.scalar, isFloat: true, fvalue: float64(s.value), fmin: float64(s.min), fmax: float64(s.max)}
	if s.linear {
		out.scalar = false
		out.flinear = true
		out.fstart = float64(s.start)
		out.fstep = float64(s.step)
		out.fmin, out.fmax = qScriptNumericFloatLinearMinMax(out)
	}
	if len(s.period) > 0 {
		out.scalar = false
		out.fperiod = make([]float64, len(s.period))
		for i, value := range s.period {
			out.fperiod[i] = float64(value)
		}
		out.fmin, out.fmax = qScriptNumericFloatPeriodSummary(s.length, out.fperiod).fmin, qScriptNumericFloatPeriodSummary(s.length, out.fperiod).fmax
	}
	return out
}

func (s qScriptNumericSumSummary) Sum() int64 {
	if s.scalar {
		return s.value
	}
	if s.length <= 0 {
		return 0
	}
	if s.linear {
		n := int64(s.length)
		return n * (2*s.start + (n-1)*s.step) / 2
	}
	if len(s.period) > 0 {
		var periodSum int64
		for _, value := range s.period {
			periodSum += value
		}
		cycles := s.length / len(s.period)
		rem := s.length % len(s.period)
		total := int64(cycles) * periodSum
		for i := 0; i < rem; i++ {
			total += s.period[i]
		}
		return total
	}
	return 0
}

func (s qScriptNumericSumSummary) FloatSum() float64 {
	if !s.isFloat {
		return float64(s.Sum())
	}
	if s.scalar {
		return s.fvalue
	}
	if s.length <= 0 {
		return 0
	}
	if s.flinear {
		n := float64(s.length)
		return n * (2*s.fstart + (n-1)*s.fstep) / 2
	}
	if len(s.fperiod) > 0 {
		var periodSum float64
		for _, value := range s.fperiod {
			periodSum += value
		}
		cycles := s.length / len(s.fperiod)
		rem := s.length % len(s.fperiod)
		total := float64(cycles) * periodSum
		for i := 0; i < rem; i++ {
			total += s.fperiod[i]
		}
		return total
	}
	return 0
}

func (s qScriptNumericSumSummary) Result(count int64) any {
	if s.isFloat {
		return s.FloatSum() + float64(count)
	}
	return s.Sum() + count
}
