package vm

// Value operations: arithmetic, comparison, indexing, concat and metamethod helpers, split verbatim from vm.go.

import (
	"fmt"
	"github.com/never-labs/leia/internal/runtime"
	"math"
)

func registerRangeMayRead(reg, start, b int) bool {
	if b == 0 {
		return reg >= start
	}
	return reg >= start && reg < start+b-1
}

func callRegisterRangeMayRead(reg, a, b int) bool {
	if b == 0 {
		return reg >= a
	}
	return reg >= a && reg < a+b
}

func callRegisterRangeMayWrite(reg, a, c int) bool {
	if c == 0 {
		return reg >= a
	}
	return reg >= a && reg < a+c-1
}

func (vm *VM) writeCallResults(dst, c int, results []runtime.Value) {
	if c == 0 {
		for i, r := range results {
			vm.regs[dst+i] = r
		}
		vm.top = dst + len(results)
		return
	}
	if c == 1 {
		return
	}
	if c == 2 {
		if len(results) > 0 {
			vm.regs[dst] = results[0]
		} else {
			vm.regs[dst] = runtime.NilValue()
		}
		return
	}
	nr := c - 1
	for i := 0; i < nr; i++ {
		if i < len(results) {
			vm.regs[dst+i] = results[i]
		} else {
			vm.regs[dst+i] = runtime.NilValue()
		}
	}
}

// callValue dispatches a function call (supports Closure, GoFunction, and __call metamethod).

func (vm *VM) callValue(fnVal runtime.Value, args []runtime.Value) ([]runtime.Value, error) {
	if fnVal.IsFunction() {
		if cl, ok := closureFromValue(fnVal); ok {
			if callSiteRuntimeSpecializationArity(len(args)) {
				if handled, results, err := vm.tryRunNonRecursiveTableValueRuntimeSpecialization(cl, args); handled {
					return results, err
				}
				if handled, err := vm.tryRunNoResultRuntimeSpecialization(cl, args); handled {
					return nil, err
				}
			}
			newBase := vm.top
			if vm.frameCount > 0 {
				curFrame := &vm.frames[vm.frameCount-1]
				minBase := curFrame.base + curFrame.closure.Proto.MaxStack
				if newBase < minBase {
					newBase = minBase
				}
			}
			return vm.call(cl, args, newBase, -1)
		}
		if gf := fnVal.GoFunction(); gf != nil {
			return vm.callGoFunction(gf, args)
		}
		if c := fnVal.Closure(); c != nil {
			return nil, fmt.Errorf("cannot call tree-walker closure from VM")
		}
	}
	if fnVal.IsTable() {
		mt := fnVal.Table().GetMetatable()
		if mt != nil {
			callMM := mt.RawGetString("__call")
			if !callMM.IsNil() {
				var local [8]runtime.Value
				var newArgs []runtime.Value
				if len(args)+1 <= len(local) {
					newArgs = local[:len(args)+1]
				} else {
					newArgs = make([]runtime.Value, len(args)+1)
				}
				newArgs[0] = fnVal
				copy(newArgs[1:], args)
				return vm.callValue(callMM, newArgs)
			}
		}
	}
	return nil, fmt.Errorf("attempt to call a %s value", fnVal.TypeName())
}

// tableGet performs table access with __index metamethod support.

func (vm *VM) tableGet(t runtime.Value, key runtime.Value) (runtime.Value, error) {
	return vm.tableGetDepth(t, key, 0)
}

func (vm *VM) tableGetDepth(t runtime.Value, key runtime.Value, depth int) (runtime.Value, error) {
	if depth > maxMetaDepth {
		return runtime.NilValue(), fmt.Errorf("__index chain too deep")
	}

	if t.IsString() {
		if vm.stringMeta != nil {
			v := vm.stringMeta.RawGet(key)
			if !v.IsNil() {
				return v, nil
			}
			idx := vm.stringMeta.RawGetString("__index")
			if idx.IsTable() {
				return vm.tableGetDepth(runtime.TableValue(idx.Table()), key, depth+1)
			}
			if idx.IsFunction() {
				if result, ok, err := vm.fastIndexStringDispatch(idx, t, key); ok || err != nil {
					return result, err
				}
				args := [2]runtime.Value{t, key}
				results, err := vm.callValue(idx, args[:])
				if err != nil {
					return runtime.NilValue(), err
				}
				if len(results) > 0 {
					return results[0], nil
				}
				return runtime.NilValue(), nil
			}
		}
		return runtime.NilValue(), nil
	}

	if t.IsSoA() {
		if v, ok, err := t.SoA().GetIndex(key); ok || err != nil {
			return v, err
		}
		return runtime.NilValue(), fmt.Errorf("attempt to index a %s value", t.TypeName())
	}

	if key.IsString() {
		if v, ok := t.FixedRecordRawGetString(key.Str()); ok {
			return v, nil
		}
	}

	if !t.IsTable() {
		if t.IsNil() && vm.frameCount > 0 {
			frame := &vm.frames[vm.frameCount-1]
			fmt.Printf("[DEBUG] attempt to index nil in %s pc=%d key=%v\n",
				frame.closure.Proto.Name, frame.pc, key)
		}
		return runtime.NilValue(), fmt.Errorf("attempt to index a %s value", t.TypeName())
	}

	tbl := t.Table()
	v := tbl.RawGet(key)
	if !v.IsNil() {
		return v, nil
	}

	mt := tbl.GetMetatable()
	if mt == nil {
		return runtime.NilValue(), nil
	}
	idx := mt.RawGetString("__index")
	if idx.IsNil() {
		return runtime.NilValue(), nil
	}
	if idx.IsTable() {
		return vm.tableGetDepth(runtime.TableValue(idx.Table()), key, depth+1)
	}
	if idx.IsFunction() {
		if result, ok, err := vm.fastIndexStringDispatch(idx, t, key); ok || err != nil {
			return result, err
		}
		args := [2]runtime.Value{t, key}
		results, err := vm.callValue(idx, args[:])
		if err != nil {
			return runtime.NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
		return runtime.NilValue(), nil
	}
	return runtime.NilValue(), nil
}

func (vm *VM) fastIndexStringDispatch(fn, receiver, key runtime.Value) (runtime.Value, bool, error) {
	if !key.IsString() {
		return runtime.NilValue(), false, nil
	}
	cl, ok := closureFromValue(fn)
	if !ok || cl == nil || cl.Proto == nil || cl.Proto.NumParams < 2 {
		return runtime.NilValue(), false, nil
	}
	code := cl.Proto.Code
	constants := cl.Proto.Constants
	want := key.Str()
	pc := 0
	for ; pc+5 < len(code); pc += 6 {
		loadKey := code[pc]
		eq := code[pc+1]
		jmp := code[pc+2]
		getGlobal := code[pc+3]
		getField := code[pc+4]
		ret := code[pc+5]
		if DecodeOp(loadKey) != OP_LOADK || DecodeOp(eq) != OP_EQ || DecodeOp(jmp) != OP_JMP ||
			DecodeOp(getGlobal) != OP_GETGLOBAL || DecodeOp(getField) != OP_GETFIELD || DecodeOp(ret) != OP_RETURN {
			break
		}
		keyConstReg := DecodeA(loadKey)
		keyConstIdx := DecodeBx(loadKey)
		if keyConstIdx < 0 || keyConstIdx >= len(constants) || !constants[keyConstIdx].IsString() {
			return runtime.NilValue(), false, nil
		}
		if DecodeB(eq) != 1 || DecodeC(eq) != keyConstReg || DecodesBx(jmp) != 3 {
			return runtime.NilValue(), false, nil
		}
		globalReg := DecodeA(getGlobal)
		globalIdx := DecodeBx(getGlobal)
		resultReg := DecodeA(getField)
		fieldIdx := DecodeC(getField)
		if DecodeB(getField) != globalReg || DecodeA(ret) != resultReg || DecodeB(ret) != 2 ||
			globalIdx < 0 || globalIdx >= len(constants) || fieldIdx < 0 || fieldIdx >= len(constants) ||
			!constants[globalIdx].IsString() || !constants[fieldIdx].IsString() {
			return runtime.NilValue(), false, nil
		}
		if constants[keyConstIdx].Str() != want {
			continue
		}
		global := vm.GetGlobal(constants[globalIdx].Str())
		if !global.IsTable() {
			return runtime.NilValue(), true, nil
		}
		fieldName := constants[fieldIdx].Str()
		if tbl := global.Table(); tbl.GetMetatable() == nil {
			return tbl.RawGetString(fieldName), true, nil
		}
		result, err := vm.tableGet(global, runtime.StringValue(fieldName))
		return result, true, err
	}
	if cl.Proto.RuntimeSpecs.IndexRawSlotFallbackShape == 0 {
		if matchIndexRawSlotFallbackShape(code, constants, pc) {
			cl.Proto.RuntimeSpecs.IndexRawSlotFallbackShape = 1
			cl.Proto.RuntimeSpecs.IndexRawSlotFallbackPC = pc
		} else {
			cl.Proto.RuntimeSpecs.IndexRawSlotFallbackShape = -1
		}
	}
	if cl.Proto.RuntimeSpecs.IndexRawSlotFallbackShape > 0 {
		if result, ok, err := vm.fastIndexRawSlotFallback(receiver, key, code, constants, cl.Proto.RuntimeSpecs.IndexRawSlotFallbackPC); ok || err != nil {
			return result, ok, err
		}
	}
	return runtime.NilValue(), false, nil
}

func matchIndexRawSlotFallbackShape(code []uint32, constants []runtime.Value, pc int) bool {
	if pc+18 >= len(code) {
		return false
	}
	loadRawGet1 := code[pc]
	moveObj1 := code[pc+1]
	loadSlotKey := code[pc+2]
	callRawGet1 := code[pc+3]
	moveLookupKey := code[pc+4]
	getSlotValue := code[pc+5]
	loadNil := code[pc+6]
	eqNil := code[pc+7]
	jmpFallback := code[pc+8]
	moveReturnSlot := code[pc+9]
	returnSlot := code[pc+10]
	loadRawGet2 := code[pc+11]
	moveObj2 := code[pc+12]
	loadBaseKey := code[pc+13]
	callRawGet2 := code[pc+14]
	moveLenKey := code[pc+15]
	lenKey := code[pc+16]
	addFallback := code[pc+17]
	returnFallback := code[pc+18]

	if DecodeOp(loadRawGet1) != OP_GETGLOBAL || DecodeOp(moveObj1) != OP_MOVE || DecodeOp(loadSlotKey) != OP_LOADK ||
		DecodeOp(callRawGet1) != OP_CALL || DecodeOp(moveLookupKey) != OP_MOVE || DecodeOp(getSlotValue) != OP_GETTABLE ||
		DecodeOp(loadNil) != OP_LOADNIL || DecodeOp(eqNil) != OP_EQ || DecodeOp(jmpFallback) != OP_JMP ||
		DecodeOp(moveReturnSlot) != OP_MOVE || DecodeOp(returnSlot) != OP_RETURN ||
		DecodeOp(loadRawGet2) != OP_GETGLOBAL || DecodeOp(moveObj2) != OP_MOVE || DecodeOp(loadBaseKey) != OP_LOADK ||
		DecodeOp(callRawGet2) != OP_CALL || DecodeOp(moveLenKey) != OP_MOVE || DecodeOp(lenKey) != OP_LEN ||
		DecodeOp(addFallback) != OP_ADD || DecodeOp(returnFallback) != OP_RETURN {
		return false
	}

	rawGetReg := DecodeA(loadRawGet1)
	rawGetConst := DecodeBx(loadRawGet1)
	slotKeyReg := DecodeA(loadSlotKey)
	slotKeyConst := DecodeBx(loadSlotKey)
	slotTableReg := DecodeA(callRawGet1)
	lookupKeyReg := DecodeA(moveLookupKey)
	slotValueReg := DecodeA(getSlotValue)
	nilRegStart := DecodeA(loadNil)
	nilRegCount := DecodeB(loadNil)
	returnSlotReg := DecodeA(moveReturnSlot)
	rawGet2Reg := DecodeA(loadRawGet2)
	rawGet2Const := DecodeBx(loadRawGet2)
	baseKeyReg := DecodeA(loadBaseKey)
	baseKeyConst := DecodeBx(loadBaseKey)
	baseValueReg := DecodeA(callRawGet2)
	lenMoveReg := DecodeA(moveLenKey)
	lenReg := DecodeA(lenKey)
	fallbackReg := DecodeA(addFallback)
	if rawGetConst < 0 || rawGetConst >= len(constants) || rawGet2Const < 0 || rawGet2Const >= len(constants) ||
		slotKeyConst < 0 || slotKeyConst >= len(constants) || baseKeyConst < 0 || baseKeyConst >= len(constants) ||
		!constants[rawGetConst].IsString() || !constants[rawGet2Const].IsString() ||
		!constants[slotKeyConst].IsString() || !constants[baseKeyConst].IsString() {
		return false
	}
	if constants[rawGetConst].Str() != "rawget" || constants[rawGet2Const].Str() != "rawget" {
		return false
	}
	return DecodeB(moveObj1) == 0 && DecodeB(moveObj2) == 0 &&
		DecodeA(callRawGet1) == rawGetReg && DecodeB(callRawGet1) == 3 && DecodeC(callRawGet1) == 2 &&
		slotTableReg == rawGetReg && DecodeB(getSlotValue) == slotTableReg && DecodeB(moveLookupKey) == 1 && DecodeC(getSlotValue) == lookupKeyReg &&
		nilRegCount == 0 && DecodeA(eqNil) == 1 && DecodeB(eqNil) == slotValueReg && DecodeC(eqNil) == nilRegStart && DecodesBx(jmpFallback) == 2 &&
		DecodeB(moveReturnSlot) == slotValueReg && DecodeA(returnSlot) == returnSlotReg && DecodeB(returnSlot) == 2 &&
		DecodeA(callRawGet2) == rawGet2Reg && rawGet2Reg == baseValueReg && DecodeB(callRawGet2) == 3 && DecodeC(callRawGet2) == 2 &&
		DecodeB(moveLenKey) == 1 && DecodeA(lenKey) == lenReg && DecodeB(lenKey) == lenMoveReg &&
		DecodeA(addFallback) == fallbackReg && DecodeB(addFallback) == baseValueReg && DecodeC(addFallback) == lenReg &&
		DecodeA(returnFallback) == fallbackReg && DecodeB(returnFallback) == 2 &&
		DecodeA(moveObj1) == rawGetReg+1 && slotKeyReg == rawGetReg+2 &&
		DecodeA(moveObj2) == rawGet2Reg+1 && baseKeyReg == rawGet2Reg+2
}

func (vm *VM) fastIndexRawSlotFallback(receiver, key runtime.Value, code []uint32, constants []runtime.Value, pc int) (runtime.Value, bool, error) {
	if !receiver.IsTable() || !key.IsString() || pc+18 >= len(code) {
		return runtime.NilValue(), false, nil
	}
	if !vm.globalIsStdRawGet("rawget") {
		return runtime.NilValue(), false, nil
	}

	loadSlotKey := code[pc+2]
	loadBaseKey := code[pc+13]
	slotKeyConst := DecodeBx(loadSlotKey)
	baseKeyConst := DecodeBx(loadBaseKey)
	if slotKeyConst < 0 || slotKeyConst >= len(constants) || baseKeyConst < 0 || baseKeyConst >= len(constants) ||
		!constants[slotKeyConst].IsString() || !constants[baseKeyConst].IsString() {
		return runtime.NilValue(), false, nil
	}

	receiverTable := receiver.Table()
	slotTable := receiverTable.RawGetString(constants[slotKeyConst].Str())
	if !slotTable.IsTable() {
		return runtime.NilValue(), true, fmt.Errorf("attempt to index a %s value", slotTable.TypeName())
	}
	var slotValue runtime.Value
	if tbl := slotTable.Table(); tbl.GetMetatable() == nil && key.IsString() {
		slotValue = tbl.RawGetString(key.Str())
	} else {
		var err error
		slotValue, err = vm.tableGet(slotTable, key)
		if err != nil {
			return runtime.NilValue(), true, err
		}
	}
	if !slotValue.IsNil() {
		return slotValue, true, nil
	}
	baseValue := receiverTable.RawGetString(constants[baseKeyConst].Str())
	keyLen, err := vm.length(key)
	if err != nil {
		return runtime.NilValue(), true, err
	}
	result, err := vm.arith(baseValue, keyLen, "__add", func(x, y float64) float64 { return x + y })
	return result, true, err
}

func (vm *VM) globalIsStdRawGet(name string) bool {
	global := vm.GetGlobal(name)
	gf := global.GoFunction()
	return gf != nil && gf.NativeKind == runtime.NativeKindStdRawGet && gf.NativeData == runtime.StdRawGetIdentityPtr()
}

func (vm *VM) globalIsStdType(name string) bool {
	global := vm.GetGlobal(name)
	gf := global.GoFunction()
	return gf != nil && gf.NativeKind == runtime.NativeKindStdType && gf.NativeData == runtime.StdTypeIdentityPtr()
}

// tableSet performs table assignment with __newindex metamethod support.

func (vm *VM) tableSet(t runtime.Value, key runtime.Value, val runtime.Value) error {
	return vm.tableSetDepth(t, key, val, 0)
}

func (vm *VM) tableSetDepth(t runtime.Value, key runtime.Value, val runtime.Value, depth int) error {
	if depth > maxMetaDepth {
		return fmt.Errorf("__newindex chain too deep")
	}
	if t.IsSoA() {
		if handled, err := t.SoA().SetIndex(key, val); handled || err != nil {
			return err
		}
		return fmt.Errorf("attempt to index a %s value", t.TypeName())
	}
	if !t.IsTable() {
		return fmt.Errorf("attempt to index a %s value", t.TypeName())
	}
	tbl := t.Table()

	existing := tbl.RawGet(key)
	if existing.IsNil() {
		mt := tbl.GetMetatable()
		if mt != nil {
			ni := mt.RawGetString("__newindex")
			if !ni.IsNil() {
				if ni.IsFunction() {
					if handled, err := vm.fastNewIndexRawSlotSet(ni, t, key, val); handled || err != nil {
						return err
					}
					args := [3]runtime.Value{t, key, val}
					_, err := vm.callValue(ni, args[:])
					return err
				}
				if ni.IsTable() {
					return vm.tableSetDepth(runtime.TableValue(ni.Table()), key, val, depth+1)
				}
			}
		}
	}

	tbl.RawSet(key, val)
	return nil
}

func (vm *VM) fastNewIndexRawSlotSet(fn, receiver, key, val runtime.Value) (bool, error) {
	cl, ok := closureFromValue(fn)
	if !ok || cl == nil || cl.Proto == nil || cl.Proto.NumParams < 3 {
		return false, nil
	}
	proto := cl.Proto
	if proto.RuntimeSpecs.IndexRawSlotFallbackShape == 0 {
		if matchNewIndexRawSlotSetShape(proto.Code, proto.Constants) {
			proto.RuntimeSpecs.IndexRawSlotFallbackShape = 1
		} else {
			proto.RuntimeSpecs.IndexRawSlotFallbackShape = -1
		}
	}
	if proto.RuntimeSpecs.IndexRawSlotFallbackShape < 1 {
		return false, nil
	}
	if !receiver.IsTable() || !key.IsString() || !vm.globalIsStdRawGet("rawget") || len(proto.Constants) < 2 || !proto.Constants[1].IsString() {
		return false, nil
	}
	slotTable := receiver.Table().RawGet(runtime.StringValue(proto.Constants[1].Str()))
	if !slotTable.IsTable() {
		return true, fmt.Errorf("attempt to index a %s value", slotTable.TypeName())
	}
	if err := vm.tableSet(slotTable, key, val); err != nil {
		return true, err
	}
	runtime.RecordRuntimePathRuntimeSpecializationHit("metamethod", "raw_slot_newindex")
	return true, nil
}

func matchNewIndexRawSlotSetShape(code []uint32, constants []runtime.Value) bool {
	if len(code) != 8 || len(constants) < 2 || !constants[0].IsString() || !constants[1].IsString() ||
		constants[0].Str() != "rawget" {
		return false
	}
	loadRawGet := code[0]
	moveObj := code[1]
	loadSlotKey := code[2]
	callRawGet := code[3]
	moveValue := code[4]
	moveKey := code[5]
	setTable := code[6]
	ret := code[7]
	if DecodeOp(loadRawGet) != OP_GETGLOBAL || DecodeOp(moveObj) != OP_MOVE || DecodeOp(loadSlotKey) != OP_LOADK ||
		DecodeOp(callRawGet) != OP_CALL || DecodeOp(moveValue) != OP_MOVE || DecodeOp(moveKey) != OP_MOVE ||
		DecodeOp(setTable) != OP_SETTABLE || DecodeOp(ret) != OP_RETURN {
		return false
	}
	rawGetReg := DecodeA(loadRawGet)
	if DecodeBx(loadRawGet) != 0 || DecodeB(moveObj) != 0 || DecodeBx(loadSlotKey) != 1 ||
		DecodeA(moveObj) != rawGetReg+1 || DecodeA(loadSlotKey) != rawGetReg+2 ||
		DecodeA(callRawGet) != rawGetReg || DecodeB(callRawGet) != 3 || DecodeC(callRawGet) != 2 {
		return false
	}
	return DecodeB(moveValue) == 2 && DecodeB(moveKey) == 1 &&
		DecodeA(setTable) == rawGetReg && DecodeB(setTable) == DecodeA(moveKey) && DecodeC(setTable) == DecodeA(moveValue) &&
		DecodeB(ret) == 1
}

func (vm *VM) tableLenInt(t runtime.Value) (int64, error) {
	if !t.IsTable() {
		return 0, fmt.Errorf("attempt to get length of a %s value", t.TypeName())
	}
	l, err := vm.length(t)
	if err != nil {
		return 0, err
	}
	return vmToInt(l), nil
}

func vmToInt(v runtime.Value) int64 {
	switch v.Type() {
	case runtime.TypeInt:
		return v.Int()
	case runtime.TypeFloat:
		return int64(v.Float())
	case runtime.TypeString:
		n, ok := v.ToNumber()
		if ok {
			return vmToInt(n)
		}
		return 0
	default:
		return 0
	}
}

// ---- Arithmetic helpers ----

func (vm *VM) arith(a, b runtime.Value, metamethod string, op func(float64, float64) float64) (runtime.Value, error) {
	if a.IsInt() && b.IsInt() {
		switch metamethod {
		case "__add":
			return runtime.IntValue(a.Int() + b.Int()), nil
		case "__sub":
			return runtime.IntValue(a.Int() - b.Int()), nil
		case "__mul":
			return runtime.IntValue(a.Int() * b.Int()), nil
		case "__pow":
			return runtime.FloatValue(math.Pow(float64(a.Int()), float64(b.Int()))), nil
		}
	}
	if a.IsNumber() && b.IsNumber() {
		result := op(a.Number(), b.Number())
		if a.IsInt() && b.IsInt() && metamethod != "__div" && metamethod != "__pow" {
			if floatIsExactInt(result) {
				return runtime.IntValue(int64(result)), nil
			}
		}
		return runtime.FloatValue(result), nil
	}
	ac, aok := a.ToNumber()
	bc, bok := b.ToNumber()
	if aok && bok {
		return vm.arith(ac, bc, metamethod, op)
	}
	mm, err := vm.getMetamethod(a, b, metamethod)
	if err == nil && !mm.IsNil() {
		if result, ok, err := vm.fastBinaryFieldMetamethod(mm, a, b, metamethod); ok || err != nil {
			return result, err
		}
		args := [2]runtime.Value{a, b}
		results, err := vm.callValue(mm, args[:])
		if err != nil {
			return runtime.NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
		return runtime.NilValue(), nil
	}
	return runtime.NilValue(), fmt.Errorf("attempt to perform arithmetic on %s and %s", a.TypeName(), b.TypeName())
}

func (vm *VM) arithMod(a, b runtime.Value) (runtime.Value, error) {
	if a.IsInt() && b.IsInt() {
		bi := b.Int()
		if bi == 0 {
			return runtime.NilValue(), fmt.Errorf("attempt to perform 'n%%0'")
		}
		r := a.Int() % bi
		if r != 0 && (r^bi) < 0 {
			r += bi
		}
		return runtime.IntValue(r), nil
	}
	if a.IsNumber() && b.IsNumber() {
		bf := b.Number()
		if bf == 0 {
			return runtime.NilValue(), fmt.Errorf("attempt to perform 'n%%0'")
		}
		r := math.Mod(a.Number(), bf)
		if r != 0 && (r < 0) != (bf < 0) {
			r += bf
		}
		return runtime.FloatValue(r), nil
	}
	return vm.arith(a, b, "__mod", func(x, y float64) float64 { return math.Mod(x, y) })
}

// ArithmeticForJIT evaluates a bytecode arithmetic opcode using the VM's full
// dynamic semantics. Baseline/native JIT code calls this when operands are not
// both plain numeric values, preserving string coercion, metamethods, and
// runtime errors in one place.
func (vm *VM) ArithmeticForJIT(op Opcode, a, b runtime.Value) (runtime.Value, error) {
	switch op {
	case OP_ADD:
		return vm.arith(a, b, "__add", func(x, y float64) float64 { return x + y })
	case OP_SUB:
		return vm.arith(a, b, "__sub", func(x, y float64) float64 { return x - y })
	case OP_MUL:
		return vm.arith(a, b, "__mul", func(x, y float64) float64 { return x * y })
	case OP_DIV:
		return vm.arith(a, b, "__div", func(x, y float64) float64 { return x / y })
	case OP_MOD:
		return vm.arithMod(a, b)
	case OP_POW:
		return vm.arith(a, b, "__pow", func(x, y float64) float64 { return math.Pow(x, y) })
	default:
		return runtime.NilValue(), fmt.Errorf("unsupported arithmetic opcode for JIT: %s", OpName(op))
	}
}

func bitwiseInt(v runtime.Value) (int64, error) {
	n, ok := v.ToNumber()
	if !ok {
		return 0, fmt.Errorf("attempt to perform bitwise operation on %s", v.TypeName())
	}
	if n.IsInt() {
		return n.Int(), nil
	}
	return int64(n.Float()), nil
}

func bitwiseShift(v runtime.Value) (uint, error) {
	n, err := bitwiseInt(v)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, fmt.Errorf("negative shift count")
	}
	return uint(n), nil
}

func bitwiseBinary(op Opcode, a, b runtime.Value) (runtime.Value, error) {
	x, err := bitwiseInt(a)
	if err != nil {
		return runtime.NilValue(), err
	}
	y, err := bitwiseInt(b)
	if err != nil {
		return runtime.NilValue(), err
	}
	switch op {
	case OP_BAND:
		return runtime.IntValue(x & y), nil
	case OP_BOR:
		return runtime.IntValue(x | y), nil
	case OP_BXOR:
		return runtime.IntValue(x ^ y), nil
	case OP_BANDN:
		return runtime.IntValue(x &^ y), nil
	case OP_SHL:
		shift, err := bitwiseShift(b)
		if err != nil {
			return runtime.NilValue(), err
		}
		if shift >= 64 {
			return runtime.IntValue(0), nil
		}
		return runtime.IntValue(int64(uint64(x) << shift)), nil
	case OP_SHR:
		shift, err := bitwiseShift(b)
		if err != nil {
			return runtime.NilValue(), err
		}
		if shift >= 64 {
			return runtime.IntValue(0), nil
		}
		return runtime.IntValue(int64(uint64(x) >> shift)), nil
	default:
		return runtime.NilValue(), fmt.Errorf("unsupported bitwise opcode %s", OpName(op))
	}
}

func (vm *VM) unaryMinus(v runtime.Value) (runtime.Value, error) {
	if v.IsInt() {
		return runtime.IntValue(-v.Int()), nil
	}
	if v.IsFloat() {
		return runtime.FloatValue(-v.Float()), nil
	}
	if nv, ok := v.ToNumber(); ok {
		return vm.unaryMinus(nv)
	}
	mm, err := vm.getMetamethod(v, v, "__unm")
	if err == nil && !mm.IsNil() {
		if result, ok, err := vm.fastUnaryFieldMetamethod(mm, v); ok || err != nil {
			return result, err
		}
		args := [1]runtime.Value{v}
		results, err := vm.callValue(mm, args[:])
		if err != nil {
			return runtime.NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
	}
	return runtime.NilValue(), fmt.Errorf("attempt to negate a %s value", v.TypeName())
}

func (vm *VM) length(v runtime.Value) (runtime.Value, error) {
	if v.IsString() {
		return runtime.IntValue(int64(runtime.StringLen(v))), nil
	}
	if v.IsTable() {
		mt := v.Table().GetMetatable()
		if mt != nil {
			mm := mt.RawGetString("__len")
			if !mm.IsNil() {
				if result, ok := fastReturnUpvalueClosure(mm); ok {
					return result, nil
				}
				args := [1]runtime.Value{v}
				results, err := vm.callValue(mm, args[:])
				if err != nil {
					return runtime.NilValue(), err
				}
				if len(results) > 0 {
					return results[0], nil
				}
				return runtime.IntValue(0), nil
			}
		}
		return runtime.IntValue(int64(v.Table().Len())), nil
	}
	if v.IsChannel() {
		return runtime.IntValue(int64(v.Channel().Len())), nil
	}
	return runtime.NilValue(), fmt.Errorf("attempt to get length of a %s value", v.TypeName())
}

func fastReturnUpvalueClosure(fn runtime.Value) (runtime.Value, bool) {
	cl, ok := closureFromValue(fn)
	if !ok || cl == nil || cl.Proto == nil || len(cl.Upvalues) == 0 || len(cl.Proto.Code) != 2 {
		return runtime.NilValue(), false
	}
	first := cl.Proto.Code[0]
	if DecodeOp(first) != OP_GETUPVAL {
		return runtime.NilValue(), false
	}
	reg := DecodeA(first)
	upvalue := DecodeB(first)
	if upvalue < 0 || upvalue >= len(cl.Upvalues) || cl.Upvalues[upvalue] == nil {
		return runtime.NilValue(), false
	}
	ret := cl.Proto.Code[1]
	if DecodeOp(ret) != OP_RETURN || DecodeA(ret) != reg || DecodeB(ret) != 2 {
		return runtime.NilValue(), false
	}
	return cl.Upvalues[upvalue].Get(), true
}

func (vm *VM) fastBinaryFieldMetamethod(fn, a, b runtime.Value, metamethod string) (runtime.Value, bool, error) {
	cl, ok := closureFromValue(fn)
	if !ok || cl == nil || cl.Proto == nil || cl.Proto.NumParams < 2 {
		return runtime.NilValue(), false, nil
	}
	code := cl.Proto.Code
	constants := cl.Proto.Constants
	if len(code) == 4 {
		leftGet := code[0]
		rightGet := code[1]
		opInst := code[2]
		ret := code[3]
		op := DecodeOp(opInst)
		if DecodeOp(leftGet) != OP_GETFIELD || DecodeOp(rightGet) != OP_GETFIELD ||
			DecodeB(leftGet) != 0 || DecodeB(rightGet) != 1 ||
			DecodeOp(ret) != OP_RETURN || DecodeA(ret) != DecodeA(opInst) || DecodeB(ret) != 2 {
			return runtime.NilValue(), false, nil
		}
		if !binaryMetamethodOpMatches(metamethod, op) {
			return runtime.NilValue(), false, nil
		}
		left, right, ok, err := vm.fastMetamethodFieldOperands(a, b, constants, DecodeC(leftGet), DecodeC(rightGet))
		if !ok || err != nil {
			return runtime.NilValue(), ok, err
		}
		runtime.RecordRuntimePathRuntimeSpecializationHit("metamethod", "field_binary")
		return vm.evalFastFieldBinaryOp(left, right, op)
	}
	if len(code) == 7 {
		leftGet := code[0]
		rightGet := code[1]
		cmp := code[2]
		jmp := code[3]
		loadTrue := code[4]
		loadFalse := code[5]
		ret := code[6]
		op := DecodeOp(cmp)
		if DecodeOp(leftGet) != OP_GETFIELD || DecodeOp(rightGet) != OP_GETFIELD ||
			DecodeB(leftGet) != 0 || DecodeB(rightGet) != 1 ||
			(op != OP_LT && op != OP_LE) || !binaryMetamethodOpMatches(metamethod, op) ||
			DecodeOp(jmp) != OP_JMP || DecodesBx(jmp) != 1 ||
			DecodeOp(loadTrue) != OP_LOADBOOL || DecodeB(loadTrue) != 1 || DecodeC(loadTrue) != 1 ||
			DecodeOp(loadFalse) != OP_LOADBOOL || DecodeB(loadFalse) != 0 || DecodeC(loadFalse) != 0 ||
			DecodeOp(ret) != OP_RETURN || DecodeA(ret) != DecodeA(loadTrue) || DecodeA(loadFalse) != DecodeA(loadTrue) || DecodeB(ret) != 2 {
			return runtime.NilValue(), false, nil
		}
		left, right, ok, err := vm.fastMetamethodFieldOperands(a, b, constants, DecodeC(leftGet), DecodeC(rightGet))
		if !ok || err != nil {
			return runtime.NilValue(), ok, err
		}
		runtime.RecordRuntimePathRuntimeSpecializationHit("metamethod", "field_compare")
		result, err := vm.evalFastFieldCompareOp(left, right, op)
		return runtime.BoolValue(result), true, err
	}
	return runtime.NilValue(), false, nil
}

func (vm *VM) fastUnaryFieldMetamethod(fn, v runtime.Value) (runtime.Value, bool, error) {
	cl, ok := closureFromValue(fn)
	if !ok || cl == nil || cl.Proto == nil || cl.Proto.NumParams < 1 || len(cl.Proto.Code) != 3 {
		return runtime.NilValue(), false, nil
	}
	code := cl.Proto.Code
	constants := cl.Proto.Constants
	get := code[0]
	op := code[1]
	ret := code[2]
	if DecodeOp(get) != OP_GETFIELD || DecodeB(get) != 0 ||
		DecodeOp(op) != OP_UNM || DecodeB(op) != DecodeA(get) ||
		DecodeOp(ret) != OP_RETURN || DecodeA(ret) != DecodeA(op) || DecodeB(ret) != 2 {
		return runtime.NilValue(), false, nil
	}
	fieldIdx := DecodeC(get)
	if fieldIdx < 0 || fieldIdx >= len(constants) || !constants[fieldIdx].IsString() {
		return runtime.NilValue(), false, nil
	}
	field, err := vm.fastMetamethodFieldOperand(v, constants[fieldIdx].Str())
	if err != nil {
		return runtime.NilValue(), true, err
	}
	if !field.IsNumber() {
		return runtime.NilValue(), false, nil
	}
	runtime.RecordRuntimePathRuntimeSpecializationHit("metamethod", "field_unary")
	result, err := vm.unaryMinus(field)
	return result, true, err
}

func binaryMetamethodOpMatches(metamethod string, op Opcode) bool {
	switch metamethod {
	case "__add":
		return op == OP_ADD
	case "__sub":
		return op == OP_SUB
	case "__mul":
		return op == OP_MUL
	case "__lt":
		return op == OP_LT
	case "__le":
		return op == OP_LE
	default:
		return false
	}
}

func (vm *VM) fastMetamethodFieldOperands(a, b runtime.Value, constants []runtime.Value, leftIdx, rightIdx int) (runtime.Value, runtime.Value, bool, error) {
	if leftIdx < 0 || leftIdx >= len(constants) || rightIdx < 0 || rightIdx >= len(constants) ||
		!constants[leftIdx].IsString() || !constants[rightIdx].IsString() {
		return runtime.NilValue(), runtime.NilValue(), false, nil
	}
	left, err := vm.fastMetamethodFieldOperand(a, constants[leftIdx].Str())
	if err != nil {
		return runtime.NilValue(), runtime.NilValue(), true, err
	}
	right, err := vm.fastMetamethodFieldOperand(b, constants[rightIdx].Str())
	if err != nil {
		return runtime.NilValue(), runtime.NilValue(), true, err
	}
	if !left.IsNumber() || !right.IsNumber() {
		return runtime.NilValue(), runtime.NilValue(), false, nil
	}
	return left, right, true, nil
}

func (vm *VM) fastMetamethodFieldOperand(v runtime.Value, field string) (runtime.Value, error) {
	if v.IsTable() {
		tbl := v.Table()
		if raw := tbl.RawGetString(field); !raw.IsNil() {
			return raw, nil
		}
		if tbl.GetMetatable() == nil {
			return runtime.NilValue(), nil
		}
	}
	return vm.tableGet(v, runtime.StringValue(field))
}

func (vm *VM) evalFastFieldBinaryOp(left, right runtime.Value, op Opcode) (runtime.Value, bool, error) {
	switch op {
	case OP_ADD:
		result, err := vm.arith(left, right, "__add", func(x, y float64) float64 { return x + y })
		return result, true, err
	case OP_SUB:
		result, err := vm.arith(left, right, "__sub", func(x, y float64) float64 { return x - y })
		return result, true, err
	case OP_MUL:
		result, err := vm.arith(left, right, "__mul", func(x, y float64) float64 { return x * y })
		return result, true, err
	default:
		return runtime.NilValue(), false, nil
	}
}

func (vm *VM) evalFastFieldCompareOp(left, right runtime.Value, op Opcode) (bool, error) {
	switch op {
	case OP_LT:
		if result, ok := left.LessThan(right); ok {
			return result, nil
		}
	case OP_LE:
		if result, ok := left.LessThan(right); ok {
			return result || left.Equal(right), nil
		}
	}
	return false, fmt.Errorf("attempt to compare %s with %s", left.TypeName(), right.TypeName())
}

func (vm *VM) ConcatValues(values []runtime.Value) (runtime.Value, error) {
	if len(values) == 0 {
		return runtime.StringValue(""), nil
	}
	allNative := true
	for _, v := range values {
		if !(v.IsString() || v.IsNumber()) {
			allNative = false
			break
		}
	}
	if allNative {
		result := values[0]
		if len(values) == 1 {
			s, _ := runtime.ConcatOperandString(result)
			return runtime.StringValue(s), nil
		}
		for i := 1; i < len(values); i++ {
			result = runtime.LazyStringValue(result, values[i])
		}
		return result, nil
	}

	result := values[len(values)-1]
	for i := len(values) - 2; i >= 0; i-- {
		var err error
		result, err = vm.concatPair(values[i], result)
		if err != nil {
			return runtime.NilValue(), err
		}
	}
	return result, nil
}

func (vm *VM) concatPair(a, b runtime.Value) (runtime.Value, error) {
	if (a.IsString() || a.IsNumber()) && (b.IsString() || b.IsNumber()) {
		return runtime.LazyStringValue(a, b), nil
	}
	mm, err := vm.getMetamethod(a, b, "__concat")
	if err == nil && !mm.IsNil() {
		if result, ok, err := vm.fastConcatTableFieldMetamethod(mm, a, b); ok || err != nil {
			return result, err
		}
		args := [2]runtime.Value{a, b}
		results, err := vm.callValue(mm, args[:])
		if err != nil {
			return runtime.NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
		return runtime.NilValue(), nil
	}
	if !(a.IsString() || a.IsNumber()) {
		return runtime.NilValue(), fmt.Errorf("attempt to concatenate a %s value", a.TypeName())
	}
	return runtime.NilValue(), fmt.Errorf("attempt to concatenate a %s value", b.TypeName())
}

func (vm *VM) fastConcatTableFieldMetamethod(fn, a, b runtime.Value) (runtime.Value, bool, error) {
	cl, ok := closureFromValue(fn)
	if !ok || cl == nil || cl.Proto == nil || cl.Proto.NumParams < 2 || len(cl.Proto.Code) != 20 {
		return runtime.NilValue(), false, nil
	}
	field, ok := matchConcatTableFieldMetamethodShape(cl.Proto)
	if !ok || !vm.globalIsStdType("type") {
		return runtime.NilValue(), false, nil
	}
	left, err := vm.concatTableFieldOperand(a, field)
	if err != nil {
		return runtime.NilValue(), true, err
	}
	right, err := vm.concatTableFieldOperand(b, field)
	if err != nil {
		return runtime.NilValue(), true, err
	}
	if !(left.IsString() || left.IsNumber()) || !(right.IsString() || right.IsNumber()) {
		return runtime.NilValue(), false, nil
	}
	runtime.RecordRuntimePathRuntimeSpecializationHit("metamethod", "table_field_concat")
	return runtime.LazyStringValue(left, right), true, nil
}

func (vm *VM) concatTableFieldOperand(v runtime.Value, field string) (runtime.Value, error) {
	if !v.IsTable() {
		return v, nil
	}
	return vm.fastMetamethodFieldOperand(v, field)
}

func matchConcatTableFieldMetamethodShape(proto *FuncProto) (string, bool) {
	code := proto.Code
	constants := proto.Constants
	if len(constants) < 3 || !constants[0].IsString() || constants[0].Str() != "type" ||
		!constants[1].IsString() || constants[1].Str() != "table" ||
		!constants[2].IsString() {
		return "", false
	}
	expectedOps := []Opcode{
		OP_GETGLOBAL, OP_MOVE, OP_CALL, OP_LOADK, OP_EQ, OP_JMP, OP_GETFIELD, OP_MOVE,
		OP_GETGLOBAL, OP_MOVE, OP_CALL, OP_LOADK, OP_EQ, OP_JMP, OP_GETFIELD, OP_MOVE,
		OP_MOVE, OP_MOVE, OP_CONCAT, OP_RETURN,
	}
	for i, op := range expectedOps {
		if DecodeOp(code[i]) != op {
			return "", false
		}
	}
	if DecodeBx(code[0]) != 0 || DecodeB(code[1]) != 0 || DecodeA(code[2]) != DecodeA(code[0]) ||
		DecodeB(code[2]) != 2 || DecodeC(code[2]) != 2 ||
		DecodeBx(code[3]) != 1 || DecodeB(code[4]) != DecodeA(code[2]) || DecodeC(code[4]) != DecodeA(code[3]) ||
		DecodesBx(code[5]) != 2 || DecodeB(code[6]) != 0 || DecodeC(code[6]) != 2 || DecodeB(code[7]) != DecodeA(code[6]) {
		return "", false
	}
	if DecodeBx(code[8]) != 0 || DecodeB(code[9]) != 1 || DecodeA(code[10]) != DecodeA(code[8]) ||
		DecodeB(code[10]) != 2 || DecodeC(code[10]) != 2 ||
		DecodeBx(code[11]) != 1 || DecodeB(code[12]) != DecodeA(code[10]) || DecodeC(code[12]) != DecodeA(code[11]) ||
		DecodesBx(code[13]) != 2 || DecodeB(code[14]) != 1 || DecodeC(code[14]) != 2 || DecodeB(code[15]) != DecodeA(code[14]) {
		return "", false
	}
	if DecodeB(code[16]) != 0 || DecodeB(code[17]) != 1 || DecodeB(code[18]) != DecodeA(code[16]) ||
		DecodeC(code[18]) != DecodeA(code[17]) || DecodeA(code[19]) != DecodeA(code[18]) || DecodeB(code[19]) != 2 {
		return "", false
	}
	return constants[2].Str(), true
}

func (vm *VM) valueEqual(a, b runtime.Value) (bool, error) {
	if a.IsTable() && b.IsTable() {
		if a.Table() == b.Table() {
			return true, nil
		}
		mm, err := vm.getMetamethod(a, b, "__eq")
		if err == nil && !mm.IsNil() {
			args := [2]runtime.Value{a, b}
			results, err := vm.callValue(mm, args[:])
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

func (vm *VM) valueLessThan(a, b runtime.Value) (bool, error) {
	if lt, ok := a.LessThan(b); ok {
		return lt, nil
	}
	mm, err := vm.getMetamethod(a, b, "__lt")
	if err == nil && !mm.IsNil() {
		if result, ok, err := vm.fastBinaryFieldMetamethod(mm, a, b, "__lt"); ok || err != nil {
			if err != nil {
				return false, err
			}
			return result.Truthy(), nil
		}
		args := [2]runtime.Value{a, b}
		results, err := vm.callValue(mm, args[:])
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

func (vm *VM) valueLessEqual(a, b runtime.Value) (bool, error) {
	if less, ok := a.LessThan(b); ok {
		return less || a.Equal(b), nil
	}
	mm, err := vm.getMetamethod(a, b, "__le")
	if err == nil && !mm.IsNil() {
		if result, ok, err := vm.fastBinaryFieldMetamethod(mm, a, b, "__le"); ok || err != nil {
			if err != nil {
				return false, err
			}
			return result.Truthy(), nil
		}
		args := [2]runtime.Value{a, b}
		results, err := vm.callValue(mm, args[:])
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

func (vm *VM) getMetamethod(a, b runtime.Value, name string) (runtime.Value, error) {
	if a.IsTable() {
		mt := a.Table().GetMetatable()
		if mt != nil {
			mm := mt.RawGetString(name)
			if !mm.IsNil() {
				return mm, nil
			}
		}
	}
	if b.IsTable() {
		mt := b.Table().GetMetatable()
		if mt != nil {
			mm := mt.RawGetString(name)
			if !mm.IsNil() {
				return mm, nil
			}
		}
	}
	return runtime.NilValue(), fmt.Errorf("no metamethod %s", name)
}

// markGlobalTablesConcurrent enables mutex on all top-level global tables.
// Called once when the first OP_GO goroutine is spawned.

func (vm *VM) markGlobalTablesConcurrent() {
	vm.globalsMu.Lock()
	for _, v := range vm.globals {
		if v.IsTable() {
			v.Table().SetConcurrent(true)
		}
	}
	vm.globalsMu.Unlock()
}

// ---- Upvalue management ----

// RegisterOpenUpvalue adds an existing open upvalue to the tracked list so that
// closeUpvalues will close it when the enclosing function returns.
// Used by the baseline JIT's CLOSURE handler.

func floatIsExactInt(f float64) bool {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return false
	}
	return f == math.Trunc(f) && f >= math.MinInt64 && f <= math.MaxInt64
}
