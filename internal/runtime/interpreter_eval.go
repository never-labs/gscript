package runtime

// Expression evaluation for the tree-walking interpreter: evalExpr family,
// arithmetic/bitwise/comparison/concat operators, unary/binary expressions,
// call and method dispatch (callFunction), closures, and table literals.
// Moved verbatim from interpreter.go (pure code movement).

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/gscript/gscript/internal/ast"
)

// ====================================================================
// Expression evaluation
// ====================================================================

// evalExpr evaluates an expression and returns a slice of Values.
// Most expressions return a single-element slice; CallExpr may return multiple.
func (interp *Interpreter) evalExpr(expr ast.Expr, env *Environment) ([]Value, error) {
	vals, err := interp.evalExprRaw(expr, env)
	if err != nil {
		return nil, interp.wrapRuntimeError(err, expr.GetPos())
	}
	return vals, nil
}

func (interp *Interpreter) evalExprRaw(expr ast.Expr, env *Environment) ([]Value, error) {
	switch e := expr.(type) {
	case *ast.NumberLit:
		v, err := parseNumber(e.Value)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil

	case *ast.StringLit:
		return []Value{StringValue(e.Value)}, nil

	case *ast.BoolLit:
		return []Value{BoolValue(e.Value)}, nil

	case *ast.NilLit:
		return []Value{NilValue()}, nil

	case *ast.VarArgExpr:
		v, ok := env.Get("...")
		if !ok {
			return nil, nil
		}
		// varargs are stored as a table in "..."
		if v.IsTable() {
			tbl := v.Table()
			n := tbl.Length()
			result := make([]Value, n)
			for i := 1; i <= n; i++ {
				result[i-1] = tbl.RawGet(IntValue(int64(i)))
			}
			return result, nil
		}
		return []Value{v}, nil

	case *ast.IdentExpr:
		v, ok := env.Get(e.Name)
		if !ok {
			return nil, fmt.Errorf("undefined variable: %s", e.Name)
		}
		return []Value{v}, nil

	case *ast.BinaryExpr:
		v, err := interp.evalBinary(e, env)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil

	case *ast.UnaryExpr:
		v, err := interp.evalUnary(e, env)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil

	case *ast.ParenExpr:
		vals, err := interp.evalExpr(e.Inner, env)
		if err != nil {
			return nil, err
		}
		if len(vals) == 0 {
			return []Value{NilValue()}, nil
		}
		return []Value{vals[0]}, nil

	case *ast.IndexExpr:
		tbl, err := interp.evalExprSingle(e.Table, env)
		if err != nil {
			return nil, err
		}
		key, err := interp.evalExprSingle(e.Index, env)
		if err != nil {
			return nil, err
		}
		val, err := interp.tableGet(tbl, key)
		if err != nil {
			return nil, err
		}
		return []Value{val}, nil

	case *ast.FieldExpr:
		tbl, err := interp.evalExprSingle(e.Table, env)
		if err != nil {
			return nil, err
		}
		val, err := interp.tableGet(tbl, StringValue(e.Field))
		if err != nil {
			return nil, err
		}
		return []Value{val}, nil

	case *ast.CallExpr:
		return interp.evalCall(e, env)

	case *ast.MethodCallExpr:
		return interp.evalMethodCall(e, env)

	case *ast.FuncLitExpr:
		v := interp.makeClosure(e.Params, e.Body, "", env)
		return []Value{v}, nil

	case *ast.TableLitExpr:
		v, err := interp.evalTableLit(e, env)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil

	case *ast.DenseLitExpr:
		v, err := interp.evalDenseLit(e, env)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil

	case *ast.MakeChanExpr:
		cap := 0
		if e.Size != nil {
			sizeVal, err := interp.evalExprSingle(e.Size, env)
			if err != nil {
				return nil, err
			}
			cap, err = ChannelCapacityFromValue(sizeVal, "make(chan)")
			if err != nil {
				return nil, err
			}
		}
		ch := NewChannel(cap)
		return []Value{ChannelValue(ch)}, nil

	case *ast.RecvExpr:
		chVal, err := interp.evalExprSingle(e.Channel, env)
		if err != nil {
			return nil, err
		}
		if !chVal.IsChannel() {
			return nil, fmt.Errorf("receive from non-channel value")
		}
		val, _ := chVal.Channel().Recv()
		return []Value{val}, nil

	default:
		return nil, fmt.Errorf("unknown expression type: %T", expr)
	}
}

// evalExprSingle evaluates an expression and returns a single Value.
// For VarArgExpr, returns the varargs table itself (not the first expanded element),
// so that #... and ...[i] work correctly.
func (interp *Interpreter) evalExprSingle(expr ast.Expr, env *Environment) (Value, error) {
	// Special case: VarArgExpr in single-value context returns the table.
	if _, ok := expr.(*ast.VarArgExpr); ok {
		v, ok := env.Get("...")
		if !ok {
			return NilValue(), nil
		}
		return v, nil
	}
	vals, err := interp.evalExpr(expr, env)
	if err != nil {
		return NilValue(), err
	}
	if len(vals) == 0 {
		return NilValue(), nil
	}
	return vals[0], nil
}

// evalExprList evaluates a list of expressions, expanding the last one for
// multiple return values.
func (interp *Interpreter) evalExprList(exprs []ast.Expr, env *Environment) ([]Value, error) {
	if len(exprs) == 0 {
		return nil, nil
	}
	var result []Value
	for i, expr := range exprs {
		if spreadExpr, ok := explicitSpreadExpr(expr); ok {
			vals, err := interp.evalExpr(spreadExpr, env)
			if err != nil {
				return nil, err
			}
			result = append(result, vals...)
			continue
		}
		vals, err := interp.evalExpr(expr, env)
		if err != nil {
			return nil, err
		}
		if i == len(exprs)-1 {
			// Last expression: expand all return values
			result = append(result, vals...)
		} else {
			// Not last: take only first value
			if len(vals) > 0 {
				result = append(result, vals[0])
			} else {
				result = append(result, NilValue())
			}
		}
	}
	return result, nil
}

// explicitSpreadExpr recognizes GScript's explicit multi-value expansion forms.
// spread(expr) expands expr's values, while table.spread is an ordinary
// multi-return call that opts in to expansion at any list position.
func explicitSpreadExpr(expr ast.Expr) (ast.Expr, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	if ident, ok := call.Func.(*ast.IdentExpr); ok && ident.Name == "spread" && len(call.Args) == 1 {
		return call.Args[0], true
	}
	if field, ok := call.Func.(*ast.FieldExpr); ok {
		if ident, ok := field.Table.(*ast.IdentExpr); ok && ident.Name == "table" &&
			field.Field == "spread" {
			return call, true
		}
	}
	return nil, false
}

// ------------------------------------------------------------------
// Binary expressions
// ------------------------------------------------------------------
func (interp *Interpreter) evalBinary(e *ast.BinaryExpr, env *Environment) (Value, error) {
	// Short-circuit operators
	if e.Op == "&&" {
		left, err := interp.evalExprSingle(e.Left, env)
		if err != nil {
			return NilValue(), err
		}
		if !left.Truthy() {
			return left, nil
		}
		return interp.evalExprSingle(e.Right, env)
	}
	if e.Op == "||" {
		left, err := interp.evalExprSingle(e.Left, env)
		if err != nil {
			return NilValue(), err
		}
		if left.Truthy() {
			return left, nil
		}
		return interp.evalExprSingle(e.Right, env)
	}

	left, err := interp.evalExprSingle(e.Left, env)
	if err != nil {
		return NilValue(), err
	}
	right, err := interp.evalExprSingle(e.Right, env)
	if err != nil {
		return NilValue(), err
	}

	if left.IsDenseArray() || right.IsDenseArray() {
		if op, ok := denseArrayBinaryOp(e.Op); ok {
			return DenseArrayElementwise(op, left, right)
		}
	}

	switch e.Op {
	case "+", "-", "*", "/", "%", "**":
		return interp.arith(e.Op, left, right)
	case "&", "|", "^", "&^", "<<", ">>":
		return interp.bitwise(e.Op, left, right)
	case "..":
		return interp.concat(left, right)
	case "==":
		eq, err := interp.valEqual(left, right)
		if err != nil {
			return NilValue(), err
		}
		return BoolValue(eq), nil
	case "!=":
		eq, err := interp.valEqual(left, right)
		if err != nil {
			return NilValue(), err
		}
		return BoolValue(!eq), nil
	case "<":
		res, err := interp.valLessThan(left, right)
		if err != nil {
			return NilValue(), err
		}
		return BoolValue(res), nil
	case "<=":
		res, err := interp.valLessEqual(left, right)
		if err != nil {
			return NilValue(), err
		}
		return BoolValue(res), nil
	case ">":
		// a > b is equivalent to b < a
		res, err := interp.valLessThan(right, left)
		if err != nil {
			return NilValue(), err
		}
		return BoolValue(res), nil
	case ">=":
		// a >= b is equivalent to b <= a
		res, err := interp.valLessEqual(right, left)
		if err != nil {
			return NilValue(), err
		}
		return BoolValue(res), nil
	default:
		return NilValue(), fmt.Errorf("unknown binary operator: %s", e.Op)
	}
}

func denseArrayBinaryOp(op string) (DenseArrayBinaryOp, bool) {
	switch op {
	case "+":
		return DenseArrayAdd, true
	case "-":
		return DenseArraySub, true
	case "*":
		return DenseArrayMul, true
	case "/":
		return DenseArrayDiv, true
	case "==":
		return DenseArrayEQ, true
	case "!=":
		return DenseArrayNE, true
	case "<":
		return DenseArrayLT, true
	case "<=":
		return DenseArrayLE, true
	case ">":
		return DenseArrayGT, true
	case ">=":
		return DenseArrayGE, true
	default:
		return 0, false
	}
}

func (interp *Interpreter) bitwise(op string, left, right Value) (Value, error) {
	l, lok := left.ToNumber()
	r, rok := right.ToNumber()
	if !lok || !rok {
		return NilValue(), fmt.Errorf("attempt to perform bitwise operation on a %s value", map[bool]string{true: right.TypeName(), false: left.TypeName()}[lok])
	}
	a, b := toInt(l), toInt(r)
	switch op {
	case "&":
		return IntValue(a & b), nil
	case "|":
		return IntValue(a | b), nil
	case "^":
		return IntValue(a ^ b), nil
	case "&^":
		return IntValue(a &^ b), nil
	case "<<":
		if b < 0 {
			return NilValue(), fmt.Errorf("negative shift count")
		}
		if b >= 64 {
			return IntValue(0), nil
		}
		return IntValue(int64(uint64(a) << uint(b))), nil
	case ">>":
		if b < 0 {
			return NilValue(), fmt.Errorf("negative shift count")
		}
		if b >= 64 {
			if a < 0 {
				return IntValue(-1), nil
			}
			return IntValue(0), nil
		}
		return IntValue(a >> uint(b)), nil
	default:
		return NilValue(), fmt.Errorf("unknown bitwise operator: %s", op)
	}
}

// arith performs arithmetic on two values, with metamethod fallback.
func (interp *Interpreter) arith(op string, left, right Value) (Value, error) {
	// Try to coerce strings to numbers
	l, lok := left.ToNumber()
	r, rok := right.ToNumber()
	if !lok || !rok {
		// Try metamethod before giving up
		mmName := opToMetamethod(op)
		if mmName != "" {
			if mm, ok := interp.getMetamethod(left, mmName); ok {
				results, err := interp.callFunction(mm, []Value{left, right})
				if err != nil {
					return NilValue(), err
				}
				if len(results) > 0 {
					return results[0], nil
				}
				return NilValue(), nil
			}
			if mm, ok := interp.getMetamethod(right, mmName); ok {
				results, err := interp.callFunction(mm, []Value{left, right})
				if err != nil {
					return NilValue(), err
				}
				if len(results) > 0 {
					return results[0], nil
				}
				return NilValue(), nil
			}
		}
		return NilValue(), fmt.Errorf("attempt to perform arithmetic on a %s value", map[bool]string{true: right.TypeName(), false: left.TypeName()}[lok])
	}
	left, right = l, r

	// If both are ints and the op keeps integer domain, use int arithmetic
	if left.IsInt() && right.IsInt() {
		a, b := left.Int(), right.Int()
		switch op {
		case "+":
			return IntValue(a + b), nil
		case "-":
			return IntValue(a - b), nil
		case "*":
			return IntValue(a * b), nil
		case "/":
			// Integer division produces float (like Lua 5.3 / operator)
			if b == 0 {
				return NilValue(), fmt.Errorf("attempt to divide by zero")
			}
			// If evenly divisible, keep int
			if a%b == 0 {
				return IntValue(a / b), nil
			}
			return FloatValue(float64(a) / float64(b)), nil
		case "%":
			if b == 0 {
				return NilValue(), fmt.Errorf("attempt to perform modulo by zero")
			}
			r := a % b
			if r != 0 && (r^b) < 0 {
				r += b
			}
			return IntValue(r), nil
		case "**":
			if b >= 0 && b < 64 {
				return IntValue(intPow(a, b)), nil
			}
			return FloatValue(math.Pow(float64(a), float64(b))), nil
		}
	}

	// Float arithmetic
	a, b := left.Number(), right.Number()
	switch op {
	case "+":
		return FloatValue(a + b), nil
	case "-":
		return FloatValue(a - b), nil
	case "*":
		return FloatValue(a * b), nil
	case "/":
		if b == 0 {
			return NilValue(), fmt.Errorf("attempt to divide by zero")
		}
		return FloatValue(a / b), nil
	case "%":
		if b == 0 {
			return NilValue(), fmt.Errorf("attempt to perform modulo by zero")
		}
		r := math.Mod(a, b)
		if r != 0 && (r < 0) != (b < 0) {
			r += b
		}
		return FloatValue(r), nil
	case "**":
		return FloatValue(math.Pow(a, b)), nil
	default:
		return NilValue(), fmt.Errorf("unknown arithmetic operator: %s", op)
	}
}

// intPow computes a^b for non-negative b using exponentiation by squaring.
func intPow(a, b int64) int64 {
	result := int64(1)
	for b > 0 {
		if b&1 == 1 {
			result *= a
		}
		a *= a
		b >>= 1
	}
	return result
}

// concat performs string concatenation, with __concat metamethod fallback.
func (interp *Interpreter) concat(left, right Value) (Value, error) {
	ls := valueToStr(left)
	rs := valueToStr(right)
	canL := canConcatType(left)
	canR := canConcatType(right)
	if canL && canR {
		return StringValue(ls + rs), nil
	}
	// Try __concat metamethod
	if mm, ok := interp.getMetamethod(left, "__concat"); ok {
		results, err := interp.callFunction(mm, []Value{left, right})
		if err != nil {
			return NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
		return NilValue(), nil
	}
	if mm, ok := interp.getMetamethod(right, "__concat"); ok {
		results, err := interp.callFunction(mm, []Value{left, right})
		if err != nil {
			return NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
		return NilValue(), nil
	}
	if !canL {
		return NilValue(), fmt.Errorf("attempt to concatenate a %s value", left.TypeName())
	}
	return NilValue(), fmt.Errorf("attempt to concatenate a %s value", right.TypeName())
}

// valEqual compares two values for equality, with __eq metamethod support.
func (interp *Interpreter) valEqual(a, b Value) (bool, error) {
	// For primitive types, use raw equality
	if a.IsTable() && b.IsTable() {
		if a.Table() == b.Table() {
			return true, nil
		}
		// Try __eq from a's metatable, then b's
		if mm, ok := interp.getMetamethod(a, "__eq"); ok {
			results, err := interp.callFunction(mm, []Value{a, b})
			if err != nil {
				return false, err
			}
			if len(results) > 0 {
				return results[0].Truthy(), nil
			}
			return false, nil
		}
		if mm, ok := interp.getMetamethod(b, "__eq"); ok {
			results, err := interp.callFunction(mm, []Value{a, b})
			if err != nil {
				return false, err
			}
			if len(results) > 0 {
				return results[0].Truthy(), nil
			}
			return false, nil
		}
		return false, nil
	}
	return a.Equal(b), nil
}

// valLessThan compares two values with < operator, with __lt metamethod support.
func (interp *Interpreter) valLessThan(a, b Value) (bool, error) {
	// Try normal comparison first
	ok, valid := a.LessThan(b)
	if valid {
		return ok, nil
	}
	// Try __lt metamethod
	if mm, found := interp.getMetamethod(a, "__lt"); found {
		results, err := interp.callFunction(mm, []Value{a, b})
		if err != nil {
			return false, err
		}
		if len(results) > 0 {
			return results[0].Truthy(), nil
		}
		return false, nil
	}
	if mm, found := interp.getMetamethod(b, "__lt"); found {
		results, err := interp.callFunction(mm, []Value{a, b})
		if err != nil {
			return false, err
		}
		if len(results) > 0 {
			return results[0].Truthy(), nil
		}
		return false, nil
	}
	return false, fmt.Errorf("attempt to compare %s with %s", a.TypeName(), b.TypeName())
}

// valLessEqual compares two values with <= operator, with __le metamethod support.
func (interp *Interpreter) valLessEqual(a, b Value) (bool, error) {
	// Try normal comparison first
	less, valid := a.LessThan(b)
	if valid {
		return less || a.Equal(b), nil
	}
	// Try __le metamethod
	if mm, found := interp.getMetamethod(a, "__le"); found {
		results, err := interp.callFunction(mm, []Value{a, b})
		if err != nil {
			return false, err
		}
		if len(results) > 0 {
			return results[0].Truthy(), nil
		}
		return false, nil
	}
	if mm, found := interp.getMetamethod(b, "__le"); found {
		results, err := interp.callFunction(mm, []Value{a, b})
		if err != nil {
			return false, err
		}
		if len(results) > 0 {
			return results[0].Truthy(), nil
		}
		return false, nil
	}
	return false, fmt.Errorf("attempt to compare %s with %s", a.TypeName(), b.TypeName())
}

func canConcatType(v Value) bool {
	return v.IsString() || v.IsNumber()
}

func valueToStr(v Value) string {
	switch v.Type() {
	case TypeString:
		return v.Str()
	case TypeInt:
		return strconv.FormatInt(v.Int(), 10)
	case TypeFloat:
		return strconv.FormatFloat(v.Float(), 'g', -1, 64)
	default:
		return ""
	}
}

// ------------------------------------------------------------------
// Unary expressions
// ------------------------------------------------------------------
func (interp *Interpreter) evalUnary(e *ast.UnaryExpr, env *Environment) (Value, error) {
	operand, err := interp.evalExprSingle(e.Operand, env)
	if err != nil {
		return NilValue(), err
	}
	switch e.Op {
	case "-":
		n, ok := operand.ToNumber()
		if !ok {
			// Try __unm metamethod
			if mm, found := interp.getMetamethod(operand, "__unm"); found {
				results, err := interp.callFunction(mm, []Value{operand})
				if err != nil {
					return NilValue(), err
				}
				if len(results) > 0 {
					return results[0], nil
				}
				return NilValue(), nil
			}
			return NilValue(), fmt.Errorf("attempt to perform arithmetic on a %s value", operand.TypeName())
		}
		if n.IsInt() {
			return IntValue(-n.Int()), nil
		}
		return FloatValue(-n.Float()), nil
	case "!":
		return BoolValue(!operand.Truthy()), nil
	case "#":
		// Check __len metamethod first for tables
		if operand.IsTable() {
			if mm, ok := interp.getMetamethod(operand, "__len"); ok {
				results, err := interp.callFunction(mm, []Value{operand})
				if err != nil {
					return NilValue(), err
				}
				if len(results) > 0 {
					return results[0], nil
				}
				return NilValue(), nil
			}
		}
		switch operand.Type() {
		case TypeString:
			return IntValue(int64(StringLen(operand))), nil
		case TypeTable:
			return IntValue(int64(operand.Table().Length())), nil
		default:
			return NilValue(), fmt.Errorf("attempt to get length of a %s value", operand.TypeName())
		}
	case "^":
		n, ok := operand.ToNumber()
		if !ok {
			return NilValue(), fmt.Errorf("attempt to perform bitwise operation on a %s value", operand.TypeName())
		}
		return IntValue(^toInt(n)), nil
	default:
		return NilValue(), fmt.Errorf("unknown unary operator: %s", e.Op)
	}
}

// ------------------------------------------------------------------
// Call expressions
// ------------------------------------------------------------------
func (interp *Interpreter) evalCall(e *ast.CallExpr, env *Environment) ([]Value, error) {
	fn, err := interp.evalExprSingle(e.Func, env)
	if err != nil {
		return nil, err
	}

	// Build args with last-arg expansion
	args, err := interp.evalExprList(e.Args, env)
	if err != nil {
		return nil, err
	}

	return interp.callFunction(fn, args)
}

func (interp *Interpreter) evalMethodCall(e *ast.MethodCallExpr, env *Environment) ([]Value, error) {
	obj, err := interp.evalExprSingle(e.Object, env)
	if err != nil {
		return nil, err
	}

	method, err := interp.methodValue(obj, e.Method)
	if err != nil {
		return nil, err
	}

	if !method.IsFunction() {
		return nil, fmt.Errorf("attempt to call a %s value (method '%s' not found)", method.TypeName(), e.Method)
	}

	args, err := interp.evalExprList(e.Args, env)
	if err != nil {
		return nil, err
	}
	// Prepend self as first argument
	args = append([]Value{obj}, args...)

	return interp.callFunction(method, args)
}

func (interp *Interpreter) methodValue(obj Value, methodName string) (Value, error) {
	if obj.IsTable() {
		return interp.tableGet(obj, StringValue(methodName))
	}
	if obj.IsString() && interp.stringMeta != nil {
		idx := interp.stringMeta.RawGet(StringValue("__index"))
		if idx.IsTable() {
			return idx.Table().RawGet(StringValue(methodName)), nil
		}
	}
	return NilValue(), fmt.Errorf("attempt to call method on a %s value", obj.TypeName())
}

// callFunction invokes a function value with the given arguments.
// If fn is a table with a __call metamethod, invokes that instead.
func (interp *Interpreter) callFunction(fn Value, args []Value) ([]Value, error) {
	if !fn.IsFunction() {
		// Try __call metamethod for tables
		if fn.IsTable() {
			if mm, ok := interp.getMetamethod(fn, "__call"); ok {
				// Prepend the table itself as first arg (Lua convention)
				newArgs := make([]Value, 0, len(args)+1)
				newArgs = append(newArgs, fn)
				newArgs = append(newArgs, args...)
				return interp.callFunction(mm, newArgs)
			}
		}
		return nil, fmt.Errorf("attempt to call a %s value", fn.TypeName())
	}

	if gf := fn.GoFunction(); gf != nil {
		interp.pushDebugFrame(gf.Name, "native")
		defer interp.popDebugFrame()
		if err := interp.emitDebugHook("call", "native", gf.Name, NilValue()); err != nil {
			return nil, err
		}
		results, err := gf.Fn(args)
		if err != nil {
			_ = interp.emitDebugHook("error", "native", gf.Name, StringValue(err.Error()))
			return nil, err
		}
		if err := interp.emitDebugHook("return", "native", gf.Name, NilValue()); err != nil {
			return nil, err
		}
		return results, nil
	}

	cl := fn.Closure()
	if cl == nil {
		return nil, fmt.Errorf("attempt to call a nil function")
	}

	// Create a new environment for the function body.
	// Parent is the globals so that built-in functions are accessible.
	// Captured upvalues are injected directly -- they provide lexical scoping.
	callEnv := NewEnvironment(interp.globals)

	// Inject captured upvalues: these share the same *Upvalue pointer as the
	// enclosing scope, so mutations are visible to all closures that share them.
	for name, uv := range cl.Upvalues {
		callEnv.DefineUpvalue(name, uv)
	}

	proto := cl.Proto
	name := proto.Name
	if name == "" {
		name = "<anonymous>"
	}
	interp.pushDebugFrameWithSource(name, "script", proto.SourceName, proto.Line, proto.Column)
	defer interp.popDebugFrame()
	interp.pushDeferFrame()
	oldSourceName := interp.currentSourceName
	if proto.SourceName != "" {
		interp.currentSourceName = proto.SourceName
	}
	defer func() {
		interp.currentSourceName = oldSourceName
	}()
	if err := interp.emitDebugHook("call", "script", name, NilValue()); err != nil {
		_ = interp.runAndPopDeferFrame()
		return nil, err
	}

	// Bind parameters (as new local variables -- these shadow any captured upvalues)
	nParams := len(proto.Params)
	if proto.HasVarArg {
		nParams-- // last param is the vararg collector name
	}

	for i := 0; i < nParams; i++ {
		v := NilValue()
		if i < len(args) {
			v = args[i]
		}
		callEnv.Define(proto.Params[i], v)
	}

	if proto.HasVarArg {
		// Collect remaining args into a table stored as "..."
		varargs := NewTable()
		start := nParams
		for i := start; i < len(args); i++ {
			varargs.RawSet(IntValue(int64(i-start+1)), args[i])
		}
		callEnv.Define("...", TableValue(varargs))
	}

	retVals, isRet, _, _, err := interp.execBlockInEnv(proto.Body, callEnv)
	deferErr := interp.runAndPopDeferFrame()
	if err != nil {
		var jump *gotoSignal
		if errors.As(err, &jump) {
			err = fmt.Errorf("goto %q target not found", jump.name)
		}
		_ = interp.emitDebugHook("error", "script", name, StringValue(err.Error()))
		return nil, err
	}
	if deferErr != nil {
		_ = interp.emitDebugHook("error", "script", name, StringValue(deferErr.Error()))
		return nil, deferErr
	}
	if err := interp.emitDebugHook("return", "script", name, NilValue()); err != nil {
		return nil, err
	}
	if isRet {
		return retVals, nil
	}
	return nil, nil
}

// CallFunction calls a GScript function value with the given args.
// This is a public method for embedding use.
func (interp *Interpreter) CallFunction(fn Value, args []Value) ([]Value, error) {
	return interp.callFunction(fn, args)
}

// ------------------------------------------------------------------
// Function literal
// ------------------------------------------------------------------
func (interp *Interpreter) makeClosure(params []ast.FuncParam, body *ast.BlockStmt, name string, env *Environment) Value {
	proto := &FuncProto{
		Name:       name,
		Body:       body,
		SourceName: interp.currentSourceName,
		Line:       body.P.Line,
		Column:     body.P.Column,
	}
	paramNames := make([]string, 0, len(params))
	for _, p := range params {
		paramNames = append(paramNames, p.Name)
		proto.Params = append(proto.Params, p.Name)
		if p.IsVarArg {
			proto.HasVarArg = true
		}
	}

	// Capture free variables from the enclosing environment
	freeVarNames := FreeVars(body, paramNames)
	upvalues := make(map[string]*Upvalue)
	for _, fv := range freeVarNames {
		if uv, ok := env.GetUpvalue(fv); ok {
			upvalues[fv] = uv
		}
		// If not found in any scope, it's a global or builtin -- don't capture.
		// It will be resolved via interp.globals at call time.
	}

	closure := &Closure{
		Proto:    proto,
		Upvalues: upvalues,
		Env:      env,
	}
	return FunctionValue(closure)
}

// ------------------------------------------------------------------
// Table literal
// ------------------------------------------------------------------
func (interp *Interpreter) evalTableLit(e *ast.TableLitExpr, env *Environment) (Value, error) {
	tbl := NewTable()
	arrayIdx := int64(1) // 1-indexed auto-incrementing key for positional fields

	for i, field := range e.Fields {
		if field.Key == nil {
			// Array-style (positional)
			if spreadExpr, ok := explicitSpreadExpr(field.Value); ok {
				vals, err := interp.evalExpr(spreadExpr, env)
				if err != nil {
					return NilValue(), err
				}
				for _, v := range vals {
					tbl.RawSet(IntValue(arrayIdx), v)
					arrayIdx++
				}
				continue
			}
			var val Value
			var err error
			if i == len(e.Fields)-1 {
				// Last field: expand multiple returns
				vals, err2 := interp.evalExpr(field.Value, env)
				if err2 != nil {
					return NilValue(), err2
				}
				for _, v := range vals {
					tbl.RawSet(IntValue(arrayIdx), v)
					arrayIdx++
				}
				continue
			}
			val, err = interp.evalExprSingle(field.Value, env)
			if err != nil {
				return NilValue(), err
			}
			tbl.RawSet(IntValue(arrayIdx), val)
			arrayIdx++
		} else {
			// Keyed field
			key, err := interp.evalExprSingle(field.Key, env)
			if err != nil {
				return NilValue(), err
			}
			val, err := interp.evalExprSingle(field.Value, env)
			if err != nil {
				return NilValue(), err
			}
			tbl.RawSet(key, val)
		}
	}

	return TableValue(tbl), nil
}

func (interp *Interpreter) evalDenseLit(e *ast.DenseLitExpr, env *Environment) (Value, error) {
	dtype, err := denseDTypeFromLiteral(e.DType)
	if err != nil {
		return NilValue(), err
	}
	if e.Len > 0 && len(e.Values) != e.Len {
		return NilValue(), fmt.Errorf("dense literal length mismatch: declared %d, got %d", e.Len, len(e.Values))
	}
	arr, err := NewDenseArrayOfLen(dtype, len(e.Values))
	if err != nil {
		return NilValue(), err
	}
	for i, expr := range e.Values {
		v, err := interp.evalExprSingle(expr, env)
		if err != nil {
			return NilValue(), err
		}
		if err := arr.Set(i, v); err != nil {
			return NilValue(), err
		}
	}
	return DenseArrayValue(arr), nil
}

func denseDTypeFromLiteral(dtype string) (DenseArrayDType, error) {
	switch dtype {
	case "f64", "f32":
		return DenseArrayF64, nil
	case "i64", "i32":
		return DenseArrayI64, nil
	case "bool":
		return DenseArrayBool, nil
	default:
		return 0, fmt.Errorf("unsupported dense literal dtype %q", dtype)
	}
}
