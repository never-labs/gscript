package vm

import (
	"unsafe"

	"github.com/never-labs/gscript/internal/jit"
	"github.com/never-labs/gscript/internal/runtime"
)

type genericRecordArrayLoopSpecializationCache struct {
	eligible bool
	spec     *genericRecordArrayLoopSpecializationSpec
	shapeID  uint32
	fields   []int
	native   genericRecordArrayNativePlan
}

type genericRecordArrayDriverLoopShape struct {
	loopPC    int
	fnConst   int
	argConsts []int
}

type genericRecordArrayLoopSpecializationSpec struct {
	tableParam int
	limitParam int
	rowReg     int
	ops        []genericRecordArrayLoopOp
	affine     []genericRecordArrayAffineUpdate
	fieldNames []string
	fieldSlots map[string]int
}

type genericRecordArrayLoopOp struct {
	op    Opcode
	a     int
	b     int
	c     int
	bx    int
	field int
}

type genericRecordArrayAffineUpdate struct {
	dstField int
	addField int
	mulField int
	scalar   genericRecordArrayScalar
}

type genericRecordArrayScalar struct {
	param int
	value float64
}

type genericRecordArrayNativePlan struct {
	code    *jit.CodeBlock
	shapeID uint32
	fields  [genericRecordArrayNativeMaxFields]int
	ok      bool
}

const (
	genericRecordArrayNativeMaxFields  = 16
	genericRecordArrayNativeMaxScalars = 6
)

type genericRecordArrayNativeContext struct {
	arrayData uintptr
	limit     int64
	steps     int64
	scalars   [genericRecordArrayNativeMaxScalars]float64
}

func (vm *VM) tryGenericRecordArrayForLoopRuntimeSpecialization(frame *CallFrame, base int, code []uint32, constants []runtime.Value, a int, sbx int) (bool, error) {
	if frame == nil || !vm.noGlobalLock {
		return false, nil
	}
	forprepPC := frame.pc - 1
	shape, ok := matchGenericRecordArrayDriverLoopShape(code, constants, forprepPC, a, sbx)
	if !ok {
		return false, nil
	}
	initV := vm.regs[base+a]
	limitV := vm.regs[base+a+1]
	stepV := vm.regs[base+a+2]
	if !initV.IsInt() || !limitV.IsInt() || !stepV.IsInt() || stepV.Int() != 1 {
		return false, nil
	}
	start := initV.Int()
	limit := limitV.Int()
	if start > limit {
		return false, nil
	}
	steps := limit - start + 1
	if steps < 128 {
		return false, nil
	}
	fnVal, ok := vm.globalValue(constants[shape.fnConst].Str())
	if !ok {
		return false, nil
	}
	cl, ok := closureFromValue(fnVal)
	if !ok || cl == nil || cl.Proto == nil {
		return false, nil
	}
	args := make([]runtime.Value, len(shape.argConsts))
	for i, constIdx := range shape.argConsts {
		v, ok := vm.globalValue(constants[constIdx].Str())
		if !ok {
			return false, nil
		}
		args[i] = v
	}
	handled, err := vm.runGenericRecordArrayLoopSpecializationN(cl.Proto, args, steps)
	if !handled || err != nil {
		return handled, err
	}
	vm.regs[base+a] = limitV
	vm.regs[base+a+3] = limitV
	frame.pc = shape.loopPC + 1
	return true, nil
}

func matchGenericRecordArrayDriverLoopShape(code []uint32, constants []runtime.Value, forprepPC int, a int, sbx int) (genericRecordArrayDriverLoopShape, bool) {
	var shape genericRecordArrayDriverLoopShape
	bodyPC := forprepPC + 1
	loopPC := bodyPC + sbx
	if forprepPC < 0 || bodyPC < 0 || loopPC < 0 || loopPC >= len(code) || loopPC <= bodyPC {
		return shape, false
	}
	loop := code[loopPC]
	if DecodeOp(loop) != OP_FORLOOP || DecodeA(loop) != a || loopPC+1+DecodesBx(loop) != bodyPC {
		return shape, false
	}
	getFn := code[bodyPC]
	if DecodeOp(getFn) != OP_GETGLOBAL {
		return shape, false
	}
	fnSlot := DecodeA(getFn)
	callPC := loopPC - 1
	call := code[callPC]
	if DecodeOp(call) != OP_CALL || DecodeA(call) != fnSlot || DecodeC(call) != 1 {
		return shape, false
	}
	argc := DecodeB(call) - 1
	if argc < 1 || argc > 8 || callPC-bodyPC != argc+1 {
		return shape, false
	}
	fnConst := DecodeBx(getFn)
	if !stringConst(constants, fnConst) {
		return shape, false
	}
	argConsts := make([]int, argc)
	for i := 0; i < argc; i++ {
		inst := code[bodyPC+1+i]
		if DecodeOp(inst) != OP_GETGLOBAL || DecodeA(inst) != fnSlot+1+i {
			return shape, false
		}
		constIdx := DecodeBx(inst)
		if !stringConst(constants, constIdx) {
			return shape, false
		}
		argConsts[i] = constIdx
	}
	return genericRecordArrayDriverLoopShape{loopPC: loopPC, fnConst: fnConst, argConsts: argConsts}, true
}

// HasGenericRecordArrayDriverLoopRuntimeSpecialization reports whether p contains a structural
// driver loop that repeatedly calls a generic record-array loop callee.
func HasGenericRecordArrayDriverLoopRuntimeSpecialization(p *FuncProto, globals map[string]*FuncProto) bool {
	if p == nil {
		return false
	}
	for pc, inst := range p.Code {
		if DecodeOp(inst) != OP_FORPREP {
			continue
		}
		if IsGenericRecordArrayDriverLoopAt(p, pc, globals) {
			return true
		}
	}
	return false
}

// IsGenericRecordArrayDriverLoopAt checks one FORPREP site for the guarded
// generic record-array call-loop shape. Runtime admission still checks trip
// count, current globals, array layout, and field guards before executing.
func IsGenericRecordArrayDriverLoopAt(p *FuncProto, forprepPC int, globals map[string]*FuncProto) bool {
	if p == nil || len(globals) == 0 || forprepPC < 0 || forprepPC >= len(p.Code) {
		return false
	}
	inst := p.Code[forprepPC]
	if DecodeOp(inst) != OP_FORPREP {
		return false
	}
	shape, ok := matchGenericRecordArrayDriverLoopShape(p.Code, p.Constants, forprepPC, DecodeA(inst), DecodesBx(inst))
	if !ok {
		return false
	}
	callee := globals[p.Constants[shape.fnConst].Str()]
	if callee == nil || len(shape.argConsts) != callee.NumParams {
		return false
	}
	_, ok = genericRecordArrayLoopSpecializationSpecForProto(callee)
	return ok
}

func (vm *VM) runGenericRecordArrayLoopSpecializationN(proto *FuncProto, args []runtime.Value, steps int64) (bool, error) {
	if proto == nil || steps <= 0 || proto.IsVarArg || len(args) != proto.NumParams {
		return false, nil
	}
	cache := proto.RuntimeSpecs.GenericRecordArrayLoopSpecialization
	if cache == nil {
		cache = &genericRecordArrayLoopSpecializationCache{eligible: true}
		proto.RuntimeSpecs.GenericRecordArrayLoopSpecialization = cache
	}
	if !cache.eligible {
		return false, nil
	}
	spec := cache.spec
	if spec == nil {
		var ok bool
		spec, ok = genericRecordArrayLoopSpecializationSpecForProto(proto)
		if !ok {
			cache.eligible = false
			return false, nil
		}
		cache.spec = spec
	}
	if spec.tableParam < 0 || spec.tableParam >= len(args) || spec.limitParam < 0 || spec.limitParam >= len(args) {
		return false, nil
	}
	tableVal := args[spec.tableParam]
	limitVal := args[spec.limitParam]
	if !tableVal.IsTable() || !limitVal.IsInt() {
		return false, nil
	}
	limit := int(limitVal.Int())
	if limit < 0 {
		return false, nil
	}
	array, ok := tableVal.Table().PlainArrayValuesForRecordSpecialization(limit)
	if !ok {
		return false, nil
	}
	if limit == 0 {
		return true, nil
	}
	first := array[1]
	if !first.IsTable() {
		return false, nil
	}
	firstTable := first.Table()
	shapeID := firstTable.ShapeID()
	if shapeID == 0 {
		return false, nil
	}
	if cache.shapeID != shapeID || len(cache.fields) != len(spec.fieldNames) {
		fields := make([]int, len(spec.fieldNames))
		for i, name := range spec.fieldNames {
			idx := firstTable.FieldIndex(name)
			if idx < 0 {
				return false, nil
			}
			fields[i] = idx
		}
		cache.shapeID = shapeID
		cache.fields = fields
	}
	paramNums := make([]float64, len(args))
	for i, v := range args {
		if v.IsNumber() {
			paramNums[i] = v.Number()
		}
	}
	if len(spec.affine) > 0 {
		if handled, err := vm.runGenericRecordArrayNativeSpecializationN(tableVal.Table(), array, limit, steps, spec, cache, paramNums); handled || err != nil {
			return handled, err
		}
		return vm.runGenericRecordArrayAffineSpecializationN(tableVal.Table(), array, limit, steps, spec, cache.fields, cache.shapeID, paramNums)
	}
	var regs [256]float64
	var valid [256]bool
	for step := int64(0); step < steps; step++ {
		for i := 1; i <= limit; i++ {
			rowVal := array[i]
			if !rowVal.IsTable() {
				return false, nil
			}
			row := rowVal.Table()
			if row.ShapeID() != cache.shapeID {
				return false, nil
			}
			for r := range valid {
				valid[r] = false
			}
			for p, n := range paramNums {
				if p < len(args) && args[p].IsNumber() {
					regs[p] = n
					valid[p] = true
				}
			}
			for _, op := range spec.ops {
				switch op.op {
				case OP_MOVE:
					if !valid[op.b] {
						return false, nil
					}
					regs[op.a] = regs[op.b]
					valid[op.a] = true
				case OP_LOADINT:
					regs[op.a] = float64(op.bx)
					valid[op.a] = true
				case OP_LOADK:
					v := proto.Constants[op.bx]
					if !v.IsNumber() {
						return false, nil
					}
					regs[op.a] = v.Number()
					valid[op.a] = true
				case OP_GETFIELD:
					idx := cache.fields[op.field]
					v := row.SvalsGet(idx)
					if !v.IsNumber() {
						return false, nil
					}
					regs[op.a] = v.Number()
					valid[op.a] = true
				case OP_ADD, OP_SUB, OP_MUL, OP_DIV:
					lv, ok := genericRecordArrayOperandNumber(op.b, proto.Constants, regs[:], valid[:])
					if !ok {
						return false, nil
					}
					rv, ok := genericRecordArrayOperandNumber(op.c, proto.Constants, regs[:], valid[:])
					if !ok {
						return false, nil
					}
					switch op.op {
					case OP_ADD:
						regs[op.a] = lv + rv
					case OP_SUB:
						regs[op.a] = lv - rv
					case OP_MUL:
						regs[op.a] = lv * rv
					case OP_DIV:
						regs[op.a] = lv / rv
					}
					valid[op.a] = true
				case OP_SETFIELD:
					v, ok := genericRecordArrayOperandNumber(op.c, proto.Constants, regs[:], valid[:])
					if !ok {
						return false, nil
					}
					row.SvalsSet(cache.fields[op.field], runtime.FloatValue(v))
				default:
					return false, nil
				}
			}
		}
	}
	tableVal.Table().MarkArrayMutationForNumericSpecialization()
	return true, nil
}

func (vm *VM) runGenericRecordArrayAffineSpecializationN(table *runtime.Table, array []runtime.Value, limit int, steps int64, spec *genericRecordArrayLoopSpecializationSpec, fields []int, shapeID uint32, params []float64) (bool, error) {
	for step := int64(0); step < steps; step++ {
		for i := 1; i <= limit; i++ {
			rowVal := array[i]
			if !rowVal.IsTable() {
				return false, nil
			}
			svals, ok := rowVal.Table().NumericSvalsForRecordSpecialization(shapeID)
			if !ok {
				return false, nil
			}
			for _, upd := range spec.affine {
				mulVal := svals[fields[upd.mulField]]
				if !mulVal.IsNumber() {
					return false, nil
				}
				scalar := upd.scalar.value
				if upd.scalar.param >= 0 {
					if upd.scalar.param >= len(params) {
						return false, nil
					}
					scalar = params[upd.scalar.param]
				}
				out := mulVal.Number() * scalar
				if upd.addField >= 0 {
					addVal := svals[fields[upd.addField]]
					if !addVal.IsNumber() {
						return false, nil
					}
					out += addVal.Number()
				}
				svals[fields[upd.dstField]] = runtime.FloatValue(out)
			}
		}
	}
	table.MarkArrayMutationForNumericSpecialization()
	return true, nil
}

func (vm *VM) runGenericRecordArrayNativeSpecializationN(table *runtime.Table, array []runtime.Value, limit int, steps int64, spec *genericRecordArrayLoopSpecializationSpec, cache *genericRecordArrayLoopSpecializationCache, params []float64) (bool, error) {
	if table == nil || spec == nil || cache == nil || len(spec.affine) == 0 || len(spec.fieldNames) > genericRecordArrayNativeMaxFields {
		return false, nil
	}
	scalarSlots, ok := genericRecordArrayNativeScalarSlots(spec)
	if !ok {
		return false, nil
	}
	var fieldBytes [genericRecordArrayNativeMaxFields]int
	var fieldSig [genericRecordArrayNativeMaxFields]int
	for i, idx := range cache.fields {
		if idx < 0 || idx*jit.ValueSize > 32760 {
			return false, nil
		}
		fieldBytes[i] = idx * jit.ValueSize
		fieldSig[i] = idx
	}
	for i := 1; i <= limit; i++ {
		rowVal := array[i]
		if !rowVal.IsTable() {
			return false, nil
		}
		svals, ok := rowVal.Table().NumericSvalsForRecordSpecialization(cache.shapeID)
		if !ok {
			return false, nil
		}
		for _, idx := range cache.fields {
			if idx < 0 || idx >= len(svals) || !svals[idx].IsFloat() {
				return false, nil
			}
		}
	}
	if !cache.native.ok || cache.native.shapeID != cache.shapeID || cache.native.fields != fieldSig {
		code, ok := compileGenericRecordArrayNativeSpecialization(spec, fieldBytes, scalarSlots)
		if !ok {
			return false, nil
		}
		cache.native = genericRecordArrayNativePlan{
			code:    code,
			shapeID: cache.shapeID,
			fields:  fieldSig,
			ok:      true,
		}
	}
	var ctx genericRecordArrayNativeContext
	ctx.arrayData = uintptr(unsafe.Pointer(&array[0]))
	ctx.limit = int64(limit)
	ctx.steps = steps
	for scalar, slot := range scalarSlots {
		if slot < 0 || slot >= len(ctx.scalars) {
			return false, nil
		}
		if scalar.param >= 0 {
			if scalar.param >= len(params) {
				return false, nil
			}
			ctx.scalars[slot] = params[scalar.param]
		} else {
			ctx.scalars[slot] = scalar.value
		}
	}
	jit.CallJIT(uintptr(cache.native.code.Ptr()), uintptr(unsafe.Pointer(&ctx)))
	table.MarkArrayMutationForNumericSpecialization()
	runtime.RecordRuntimePathRuntimeSpecializationHit(string(RuntimeSpecializationRouteDriverLoop), "generic_record_array_native_loop")
	return true, nil
}

func genericRecordArrayNativeScalarSlots(spec *genericRecordArrayLoopSpecializationSpec) (map[genericRecordArrayScalar]int, bool) {
	out := make(map[genericRecordArrayScalar]int)
	for _, upd := range spec.affine {
		if _, ok := out[upd.scalar]; ok {
			continue
		}
		if len(out) >= genericRecordArrayNativeMaxScalars {
			return nil, false
		}
		out[upd.scalar] = len(out)
	}
	return out, true
}

func compileGenericRecordArrayNativeSpecialization(spec *genericRecordArrayLoopSpecializationSpec, fieldBytes [genericRecordArrayNativeMaxFields]int, scalarSlots map[genericRecordArrayScalar]int) (*jit.CodeBlock, bool) {
	asm := jit.NewAssembler()
	done := "generic_record_array_native_done"
	outer := "generic_record_array_native_outer"
	inner := "generic_record_array_native_inner"
	innerDone := "generic_record_array_native_inner_done"

	arrayDataOff := int(unsafe.Offsetof(genericRecordArrayNativeContext{}.arrayData))
	limitOff := int(unsafe.Offsetof(genericRecordArrayNativeContext{}.limit))
	stepsOff := int(unsafe.Offsetof(genericRecordArrayNativeContext{}.steps))
	scalarsOff := int(unsafe.Offsetof(genericRecordArrayNativeContext{}.scalars))

	// X0 = *genericRecordArrayNativeContext from callJIT.
	asm.LDR(jit.X1, jit.X0, arrayDataOff)
	asm.LDR(jit.X2, jit.X0, limitOff)
	asm.LDR(jit.X3, jit.X0, stepsOff)
	asm.CMPimm(jit.X2, 0)
	asm.BCond(jit.CondLE, done)
	asm.CMPimm(jit.X3, 0)
	asm.BCond(jit.CondLE, done)

	scalarRegs := [...]jit.FReg{jit.D2, jit.D3, jit.D4, jit.D5, jit.D6, jit.D7}
	for scalar, slot := range scalarSlots {
		_ = scalar
		asm.FLDRd(scalarRegs[slot], jit.X0, scalarsOff+slot*8)
	}

	asm.MOVimm16(jit.X5, 0)
	asm.Label(outer)
	asm.MOVimm16(jit.X6, 1)
	asm.Label(inner)
	asm.LDRreg(jit.X7, jit.X1, jit.X6)
	jit.EmitExtractPtr(asm, jit.X7, jit.X7)
	asm.LDR(jit.X8, jit.X7, jit.TableOffSvals)
	for _, upd := range spec.affine {
		if upd.mulField < 0 || upd.mulField >= len(fieldBytes) || upd.dstField < 0 || upd.dstField >= len(fieldBytes) {
			return nil, false
		}
		slot, ok := scalarSlots[upd.scalar]
		if !ok {
			return nil, false
		}
		asm.FLDRd(jit.D1, jit.X8, fieldBytes[upd.mulField])
		if upd.addField >= 0 {
			if upd.addField >= len(fieldBytes) {
				return nil, false
			}
			asm.FLDRd(jit.D0, jit.X8, fieldBytes[upd.addField])
			asm.FMADDd(jit.D0, jit.D1, scalarRegs[slot], jit.D0)
		} else {
			asm.FMULd(jit.D0, jit.D1, scalarRegs[slot])
		}
		asm.FSTRd(jit.D0, jit.X8, fieldBytes[upd.dstField])
	}
	asm.CMPreg(jit.X6, jit.X2)
	asm.BCond(jit.CondEQ, innerDone)
	asm.ADDimm(jit.X6, jit.X6, 1)
	asm.B(inner)
	asm.Label(innerDone)
	asm.ADDimm(jit.X5, jit.X5, 1)
	asm.CMPreg(jit.X5, jit.X3)
	asm.BCond(jit.CondLT, outer)
	asm.Label(done)
	asm.MOVimm16(jit.X0, 0)
	asm.RET()

	machineCode, err := asm.Finalize()
	if err != nil {
		return nil, false
	}
	block, err := jit.AllocExec(len(machineCode))
	if err != nil {
		return nil, false
	}
	if err := block.WriteCode(machineCode); err != nil {
		_ = block.Free()
		return nil, false
	}
	return block, true
}

func genericRecordArrayOperandNumber(encoded int, constants []runtime.Value, regs []float64, valid []bool) (float64, bool) {
	if encoded >= RKBit {
		idx := encoded - RKBit
		if idx < 0 || idx >= len(constants) || !constants[idx].IsNumber() {
			return 0, false
		}
		return constants[idx].Number(), true
	}
	if encoded < 0 || encoded >= len(valid) || !valid[encoded] {
		return 0, false
	}
	return regs[encoded], true
}

func genericRecordArrayLoopSpecializationSpecForProto(proto *FuncProto) (*genericRecordArrayLoopSpecializationSpec, bool) {
	if proto == nil || proto.NumParams < 2 || proto.IsVarArg || len(proto.Code) < 7 {
		return nil, false
	}
	code := proto.Code
	if DecodeOp(code[0]) != OP_LOADINT || DecodesBx(code[0]) != 1 ||
		DecodeOp(code[1]) != OP_MOVE ||
		DecodeOp(code[2]) != OP_LOADINT || DecodesBx(code[2]) != 1 ||
		DecodeOp(code[3]) != OP_FORPREP {
		return nil, false
	}
	loopBase := DecodeA(code[3])
	if DecodeA(code[0]) != loopBase || DecodeA(code[1]) != loopBase+1 || DecodeA(code[2]) != loopBase+2 {
		return nil, false
	}
	limitParam := DecodeB(code[1])
	if limitParam < 0 || limitParam >= proto.NumParams {
		return nil, false
	}
	bodyPC := 4
	loopPC := bodyPC + DecodesBx(code[3])
	if loopPC <= bodyPC || loopPC >= len(code) || DecodeOp(code[loopPC]) != OP_FORLOOP ||
		DecodeA(code[loopPC]) != loopBase || loopPC+1+DecodesBx(code[loopPC]) != bodyPC {
		return nil, false
	}
	if loopPC+1 >= len(code) || DecodeOp(code[loopPC+1]) != OP_RETURN {
		return nil, false
	}
	if DecodeOp(code[bodyPC]) != OP_MOVE || DecodeB(code[bodyPC]) != loopBase+3 ||
		DecodeOp(code[bodyPC+1]) != OP_GETTABLE {
		return nil, false
	}
	get := code[bodyPC+1]
	rowReg := DecodeA(get)
	tableParam := DecodeB(get)
	keyReg := DecodeC(get)
	if tableParam < 0 || tableParam >= proto.NumParams || keyReg != DecodeA(code[bodyPC]) {
		return nil, false
	}
	spec := &genericRecordArrayLoopSpecializationSpec{
		tableParam: tableParam,
		limitParam: limitParam,
		rowReg:     rowReg,
		fieldSlots: make(map[string]int),
	}
	for pc := bodyPC + 2; pc < loopPC; pc++ {
		inst := code[pc]
		op := DecodeOp(inst)
		gop := genericRecordArrayLoopOp{op: op, a: DecodeA(inst), b: DecodeB(inst), c: DecodeC(inst), bx: DecodeBx(inst)}
		if op == OP_LOADINT {
			gop.bx = DecodesBx(inst)
		}
		switch op {
		case OP_MOVE, OP_LOADINT, OP_LOADK, OP_ADD, OP_SUB, OP_MUL, OP_DIV:
		case OP_GETFIELD:
			if DecodeB(inst) != rowReg || !stringConst(proto.Constants, gop.c) {
				return nil, false
			}
			field, ok := spec.internField(proto.Constants[gop.c].Str())
			if !ok {
				return nil, false
			}
			gop.field = field
		case OP_SETFIELD:
			if DecodeA(inst) != rowReg || !stringConst(proto.Constants, gop.b) {
				return nil, false
			}
			field, ok := spec.internField(proto.Constants[gop.b].Str())
			if !ok {
				return nil, false
			}
			gop.field = field
		default:
			return nil, false
		}
		spec.ops = append(spec.ops, gop)
	}
	if len(spec.ops) == 0 || len(spec.fieldNames) == 0 {
		return nil, false
	}
	spec.affine = buildGenericRecordArrayAffineUpdates(spec, proto.Constants)
	return spec, true
}

func buildGenericRecordArrayAffineUpdates(spec *genericRecordArrayLoopSpecializationSpec, constants []runtime.Value) []genericRecordArrayAffineUpdate {
	if spec == nil || len(spec.ops) == 0 {
		return nil
	}
	var out []genericRecordArrayAffineUpdate
	for pc := 0; pc < len(spec.ops); {
		if pc+4 < len(spec.ops) &&
			spec.ops[pc].op == OP_GETFIELD &&
			spec.ops[pc+1].op == OP_GETFIELD &&
			spec.ops[pc+2].op == OP_MUL &&
			spec.ops[pc+3].op == OP_ADD &&
			spec.ops[pc+4].op == OP_SETFIELD {
			add := spec.ops[pc]
			mulField := spec.ops[pc+1]
			mul := spec.ops[pc+2]
			addOp := spec.ops[pc+3]
			store := spec.ops[pc+4]
			scalar, ok := affineScalarFromMul(mul, mulField.a, constants)
			if ok && addOp.b == add.a && addOp.c == mul.a && store.c == addOp.a {
				out = append(out, genericRecordArrayAffineUpdate{
					dstField: store.field,
					addField: add.field,
					mulField: mulField.field,
					scalar:   scalar,
				})
				pc += 5
				continue
			}
		}
		if pc+3 < len(spec.ops) &&
			spec.ops[pc].op == OP_GETFIELD &&
			(spec.ops[pc+1].op == OP_LOADK || spec.ops[pc+1].op == OP_LOADINT) &&
			spec.ops[pc+2].op == OP_MUL &&
			spec.ops[pc+3].op == OP_SETFIELD {
			mulField := spec.ops[pc]
			load := spec.ops[pc+1]
			mul := spec.ops[pc+2]
			store := spec.ops[pc+3]
			scalar, ok := affineScalarFromLoad(load, constants)
			if ok && ((mul.b == mulField.a && mul.c == load.a) || (mul.c == mulField.a && mul.b == load.a)) && store.c == mul.a {
				out = append(out, genericRecordArrayAffineUpdate{
					dstField: store.field,
					addField: -1,
					mulField: mulField.field,
					scalar:   scalar,
				})
				pc += 4
				continue
			}
		}
		return nil
	}
	return out
}

func affineScalarFromMul(mul genericRecordArrayLoopOp, fieldReg int, constants []runtime.Value) (genericRecordArrayScalar, bool) {
	if mul.b == fieldReg {
		return affineScalarFromOperand(mul.c, constants)
	}
	if mul.c == fieldReg {
		return affineScalarFromOperand(mul.b, constants)
	}
	return genericRecordArrayScalar{}, false
}

func affineScalarFromOperand(encoded int, constants []runtime.Value) (genericRecordArrayScalar, bool) {
	if encoded >= RKBit {
		idx := encoded - RKBit
		if idx < 0 || idx >= len(constants) || !constants[idx].IsNumber() {
			return genericRecordArrayScalar{}, false
		}
		return genericRecordArrayScalar{param: -1, value: constants[idx].Number()}, true
	}
	if encoded < 0 {
		return genericRecordArrayScalar{}, false
	}
	return genericRecordArrayScalar{param: encoded}, true
}

func affineScalarFromLoad(load genericRecordArrayLoopOp, constants []runtime.Value) (genericRecordArrayScalar, bool) {
	switch load.op {
	case OP_LOADK:
		if load.bx < 0 || load.bx >= len(constants) || !constants[load.bx].IsNumber() {
			return genericRecordArrayScalar{}, false
		}
		return genericRecordArrayScalar{param: -1, value: constants[load.bx].Number()}, true
	case OP_LOADINT:
		return genericRecordArrayScalar{param: -1, value: float64(load.bx)}, true
	default:
		return genericRecordArrayScalar{}, false
	}
}

func (s *genericRecordArrayLoopSpecializationSpec) internField(name string) (int, bool) {
	if s == nil || name == "" {
		return 0, false
	}
	if idx, ok := s.fieldSlots[name]; ok {
		return idx, true
	}
	idx := len(s.fieldNames)
	s.fieldNames = append(s.fieldNames, name)
	s.fieldSlots[name] = idx
	return idx, true
}
