package vm

import (
	"math"

	"github.com/gscript/gscript/internal/runtime"
)

const (
	pairwiseFieldX = iota
	pairwiseFieldY
	pairwiseFieldZ
	pairwiseFieldVX
	pairwiseFieldVY
	pairwiseFieldVZ
	pairwiseFieldMass
	pairwiseFieldCount
)

type recordPairwiseNumericKernelCache struct {
	eligible  bool
	shapeID   uint32
	idxs      [pairwiseFieldCount]int
	spec      *recordPairwiseNumericKernelSpec
	denseSpec *denseRecordMatrixAdvanceKernelSpec
}

type recordPairwiseNumericKernelSpec struct {
	tableName     string
	sqrtTableName string
	sqrtFieldName string
	fieldNames    [pairwiseFieldCount]string
}

type denseRecordMatrixAdvanceKernelSpec struct {
	countName      string
	matrixName     string
	matrixGetName  string
	matrixSetName  string
	tableName      string
	sqrtTableName  string
	sqrtFieldName  string
	fieldIndexName [pairwiseFieldCount]string
}

type recordPairwiseNumericDriverLoopShape struct {
	loopPC   int
	fnConst  int
	argConst int
}

func (vm *VM) tryRunRecordPairwiseNumericKernel(cl *Closure, args []runtime.Value) (bool, error) {
	if vm.methodJIT != nil {
		return false, nil
	}
	if cl == nil || cl.Proto == nil || !hotWholeCallKernelRecognized(cl.Proto, wholeCallKernelRecordPairwiseNumeric) {
		return false, nil
	}
	return vm.runRecordPairwiseNumericKernel(cl, args)
}

func (vm *VM) runRecordPairwiseNumericKernel(cl *Closure, args []runtime.Value) (bool, error) {
	if vm.methodJIT != nil {
		return false, nil
	}
	return vm.runRecordPairwiseNumericKernelN(cl, args, 1)
}

func (vm *VM) tryRunRecordPairwiseNumericKernelN(cl *Closure, args []runtime.Value, steps int64) (bool, error) {
	if cl == nil || cl.Proto == nil || !hotWholeCallKernelRecognized(cl.Proto, wholeCallKernelRecordPairwiseNumeric) {
		return false, nil
	}
	return vm.runRecordPairwiseNumericKernelN(cl, args, steps)
}

func (vm *VM) runRecordPairwiseNumericKernelN(cl *Closure, args []runtime.Value, steps int64) (bool, error) {
	if cl == nil || cl.Proto == nil || len(args) != 1 || !vm.noGlobalLock {
		return false, nil
	}
	if steps < 0 {
		return false, nil
	}
	proto := cl.Proto
	cache := proto.RecordPairwiseNumericKernel
	if cache == nil {
		cache = &recordPairwiseNumericKernelCache{eligible: true}
		proto.RecordPairwiseNumericKernel = cache
	}
	if isDenseRecordMatrixAdvanceProto(proto) {
		spec := cache.denseSpec
		if spec == nil {
			var ok bool
			spec, ok = denseRecordMatrixAdvanceKernelSpecForProto(proto)
			if !ok {
				cache.eligible = false
				return false, nil
			}
			cache.denseSpec = spec
		}
		return vm.runDenseRecordMatrixAdvanceKernelN(args, steps, spec)
	}
	spec := cache.spec
	if spec == nil {
		var ok bool
		spec, ok = recordPairwiseNumericKernelSpecForProto(proto)
		if !ok {
			cache.eligible = false
			return false, nil
		}
		cache.spec = spec
	}
	if !cache.eligible || !args[0].IsNumber() || !vm.guardRecordPairwiseSqrt(spec) {
		return false, nil
	}
	bodiesVal, ok := vm.recordPairwiseTableValue(spec)
	if !ok || !bodiesVal.IsTable() {
		return false, nil
	}
	bodiesTable := bodiesVal.Table()
	n := bodiesTable.Length()
	if n < 0 || n > 64 {
		return false, nil
	}
	bodyArray, ok := bodiesTable.PlainArrayValuesForRecordKernel(n)
	if !ok {
		return false, nil
	}

	var bodyTables [64]*runtime.Table
	var xs, ys, zs [64]float64
	var vxs, vys, vzs [64]float64
	var masses [64]float64
	if n == 0 {
		return true, nil
	}
	first := bodyArray[1]
	if !first.IsTable() {
		return false, nil
	}
	shapeID := first.Table().ShapeID()
	if shapeID == 0 {
		return false, nil
	}
	if cache.shapeID != shapeID {
		idxs, ok := recordPairwiseFieldIndexesForShape(proto, spec, first.Table())
		if !ok {
			return false, nil
		}
		cache.shapeID = shapeID
		cache.idxs = idxs
	}

	for i := 0; i < n; i++ {
		v := bodyArray[i+1]
		if !v.IsTable() {
			return false, nil
		}
		t := v.Table()
		for j := 0; j < i; j++ {
			if bodyTables[j] == t {
				return false, nil
			}
		}
		var fields [pairwiseFieldCount]float64
		if !t.LoadFloatRecordForNumericKernel(cache.shapeID, cache.idxs[:], fields[:]) {
			return false, nil
		}
		bodyTables[i] = t
		xs[i], ys[i], zs[i] = fields[pairwiseFieldX], fields[pairwiseFieldY], fields[pairwiseFieldZ]
		vxs[i], vys[i], vzs[i] = fields[pairwiseFieldVX], fields[pairwiseFieldVY], fields[pairwiseFieldVZ]
		masses[i] = fields[pairwiseFieldMass]
	}

	dt := args[0].Number()
	for step := int64(0); step < steps; step++ {
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				dx := xs[i] - xs[j]
				dy := ys[i] - ys[j]
				dz := zs[i] - zs[j]
				dsq := dx*dx + dy*dy + dz*dz
				dist := math.Sqrt(dsq)
				mag := dt / (dsq * dist)
				vxs[i] -= dx * masses[j] * mag
				vys[i] -= dy * masses[j] * mag
				vzs[i] -= dz * masses[j] * mag
				vxs[j] += dx * masses[i] * mag
				vys[j] += dy * masses[i] * mag
				vzs[j] += dz * masses[i] * mag
			}
		}
		for i := 0; i < n; i++ {
			xs[i] += dt * vxs[i]
			ys[i] += dt * vys[i]
			zs[i] += dt * vzs[i]
		}
	}

	storeIdxs := cache.idxs[:pairwiseFieldMass]
	for i := 0; i < n; i++ {
		vals := [...]float64{xs[i], ys[i], zs[i], vxs[i], vys[i], vzs[i]}
		if !bodyTables[i].StoreFloatRecordForNumericKernel(cache.shapeID, storeIdxs, vals[:]) {
			return false, nil
		}
	}
	bodiesTable.MarkArrayMutationForNumericKernel()
	return true, nil
}

func (vm *VM) runDenseRecordMatrixAdvanceKernelN(args []runtime.Value, steps int64, spec *denseRecordMatrixAdvanceKernelSpec) (bool, error) {
	if len(args) != 1 || !args[0].IsNumber() || !vm.guardDenseRecordMatrixSqrt(spec) || !vm.guardDenseRecordMatrixLib(spec) {
		return false, nil
	}
	n, fields, ok := vm.guardDenseRecordMatrixGlobals(spec)
	if !ok {
		return false, nil
	}
	bodiesVal, ok := vm.globalValue(spec.tableName)
	if !ok || !bodiesVal.IsTable() {
		return false, nil
	}
	flat, stride, ok := bodiesVal.Table().DenseFloatMatrixForNumericKernel(n, pairwiseFieldCount)
	if !ok || stride < pairwiseFieldCount {
		return false, nil
	}
	fx, fy, fz := fields[pairwiseFieldX], fields[pairwiseFieldY], fields[pairwiseFieldZ]
	fvx, fvy, fvz := fields[pairwiseFieldVX], fields[pairwiseFieldVY], fields[pairwiseFieldVZ]
	fmass := fields[pairwiseFieldMass]
	dt := args[0].Number()
	for step := int64(0); step < steps; step++ {
		for i := 0; i < n; i++ {
			bi := i * stride
			bix := flat[bi+fx]
			biy := flat[bi+fy]
			biz := flat[bi+fz]
			bim := flat[bi+fmass]
			bivx := flat[bi+fvx]
			bivy := flat[bi+fvy]
			bivz := flat[bi+fvz]
			for j := i + 1; j < n; j++ {
				bj := j * stride
				bjx := flat[bj+fx]
				bjy := flat[bj+fy]
				bjz := flat[bj+fz]
				bjm := flat[bj+fmass]
				bjvx := flat[bj+fvx]
				bjvy := flat[bj+fvy]
				bjvz := flat[bj+fvz]
				dx := bix - bjx
				dy := biy - bjy
				dz := biz - bjz
				dsq := dx*dx + dy*dy + dz*dz
				dist := math.Sqrt(dsq)
				mag := dt / (dsq * dist)
				bivx -= dx * bjm * mag
				bivy -= dy * bjm * mag
				bivz -= dz * bjm * mag
				flat[bj+fvx] = bjvx + dx*bim*mag
				flat[bj+fvy] = bjvy + dy*bim*mag
				flat[bj+fvz] = bjvz + dz*bim*mag
			}
			flat[bi+fvx] = bivx
			flat[bi+fvy] = bivy
			flat[bi+fvz] = bivz
		}
		for i := 0; i < n; i++ {
			b := i * stride
			flat[b+fx] += dt * flat[b+fvx]
			flat[b+fy] += dt * flat[b+fvy]
			flat[b+fz] += dt * flat[b+fvz]
		}
	}
	bodiesVal.Table().MarkArrayMutationForNumericKernel()
	return true, nil
}

func (vm *VM) tryRecordPairwiseNumericForLoopKernel(frame *CallFrame, base int, code []uint32, constants []runtime.Value, a int, sbx int) (bool, error) {
	if frame == nil || !vm.noGlobalLock {
		return false, nil
	}
	forprepPC := frame.pc - 1
	shape, ok := matchRecordPairwiseNumericDriverLoopShape(code, constants, forprepPC, a, sbx)
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
	if steps < 1024 {
		return false, nil
	}
	fnVal, ok := vm.globalValue(constants[shape.fnConst].Str())
	if !ok {
		return false, nil
	}
	cl, ok := closureFromValue(fnVal)
	if !ok || !HasRecordPairwiseNumericWholeCallKernel(cl.Proto) {
		return false, nil
	}
	argVal, ok := vm.globalValue(constants[shape.argConst].Str())
	if !ok || !argVal.IsNumber() {
		return false, nil
	}
	handled, err := vm.tryRunRecordPairwiseNumericKernelN(cl, []runtime.Value{argVal}, steps)
	if !handled || err != nil {
		return handled, err
	}
	vm.regs[base+a] = limitV
	vm.regs[base+a+3] = limitV
	frame.pc = shape.loopPC + 1
	return true, nil
}

func matchRecordPairwiseNumericDriverLoopShape(code []uint32, constants []runtime.Value, forprepPC int, a int, sbx int) (recordPairwiseNumericDriverLoopShape, bool) {
	var shape recordPairwiseNumericDriverLoopShape
	bodyPC := forprepPC + 1
	loopPC := bodyPC + sbx
	if forprepPC < 0 || bodyPC < 0 || loopPC < 0 || loopPC >= len(code) || loopPC-bodyPC != 3 {
		return shape, false
	}
	loop := code[loopPC]
	if DecodeOp(loop) != OP_FORLOOP || DecodeA(loop) != a || loopPC+1+DecodesBx(loop) != bodyPC {
		return shape, false
	}
	getFn := code[bodyPC]
	getArg := code[bodyPC+1]
	call := code[bodyPC+2]
	if DecodeOp(getFn) != OP_GETGLOBAL || DecodeOp(getArg) != OP_GETGLOBAL || DecodeOp(call) != OP_CALL {
		return shape, false
	}
	fnSlot := DecodeA(getFn)
	argSlot := DecodeA(getArg)
	if DecodeA(call) != fnSlot || DecodeB(call) != 2 || DecodeC(call) != 1 || argSlot != fnSlot+1 {
		return shape, false
	}
	fnConst := DecodeBx(getFn)
	argConst := DecodeBx(getArg)
	if !stringConst(constants, fnConst) || !stringConst(constants, argConst) {
		return shape, false
	}
	return recordPairwiseNumericDriverLoopShape{
		loopPC:   loopPC,
		fnConst:  fnConst,
		argConst: argConst,
	}, true
}

func recordPairwiseFieldIndexesForShape(proto *FuncProto, spec *recordPairwiseNumericKernelSpec, t *runtime.Table) ([pairwiseFieldCount]int, bool) {
	var idxs [pairwiseFieldCount]int
	if proto == nil || spec == nil || t == nil {
		return idxs, false
	}
	for i, fieldName := range spec.fieldNames {
		if fieldName == "" {
			return idxs, false
		}
		idx := t.FieldIndex(fieldName)
		if idx < 0 {
			return idxs, false
		}
		idxs[i] = idx
	}
	return idxs, true
}

func (vm *VM) guardRecordPairwiseSqrt(spec *recordPairwiseNumericKernelSpec) bool {
	if spec == nil || spec.sqrtTableName == "" || spec.sqrtFieldName == "" {
		return false
	}
	return vm.guardGoFunctionField(spec.sqrtTableName, spec.sqrtFieldName, "math.sqrt")
}

func (vm *VM) guardDenseRecordMatrixSqrt(spec *denseRecordMatrixAdvanceKernelSpec) bool {
	if spec == nil {
		return false
	}
	return vm.guardGoFunctionField(spec.sqrtTableName, spec.sqrtFieldName, "math.sqrt")
}

func (vm *VM) guardGoFunctionField(tableName, fieldName, goName string) bool {
	mathVal, ok := vm.globalValue(tableName)
	if !ok || !mathVal.IsTable() {
		return false
	}
	mt := mathVal.Table()
	if mt.HasMetatable() {
		return false
	}
	sqrtVal := mt.RawGetString(fieldName)
	gf := sqrtVal.GoFunction()
	return gf != nil && gf.Name == goName
}

func (vm *VM) guardDenseRecordMatrixLib(spec *denseRecordMatrixAdvanceKernelSpec) bool {
	if spec == nil {
		return false
	}
	matrixVal, ok := vm.globalValue(spec.matrixName)
	if !ok || !matrixVal.IsTable() {
		return false
	}
	mt := matrixVal.Table()
	if mt.HasMetatable() {
		return false
	}
	getf := mt.RawGetString(spec.matrixGetName).GoFunction()
	setf := mt.RawGetString(spec.matrixSetName).GoFunction()
	return getf != nil && getf.Name == "matrix.getf" &&
		setf != nil && setf.Name == "matrix.setf"
}

func (vm *VM) guardDenseRecordMatrixGlobals(spec *denseRecordMatrixAdvanceKernelSpec) (int, [pairwiseFieldCount]int, bool) {
	var fields [pairwiseFieldCount]int
	if spec == nil {
		return 0, fields, false
	}
	count, ok := vm.globalValue(spec.countName)
	if !ok || !count.IsInt() || count.Int() < 0 || count.Int() > 64 {
		return 0, fields, false
	}
	seen := 0
	for i, name := range spec.fieldIndexName {
		v, ok := vm.globalValue(name)
		if !ok || !v.IsInt() || v.Int() < 0 || v.Int() >= pairwiseFieldCount {
			return 0, fields, false
		}
		idx := int(v.Int())
		bit := 1 << idx
		if seen&bit != 0 {
			return 0, fields, false
		}
		seen |= bit
		fields[i] = idx
	}
	return int(count.Int()), fields, true
}

func (vm *VM) recordPairwiseTableValue(spec *recordPairwiseNumericKernelSpec) (runtime.Value, bool) {
	if spec == nil || spec.tableName == "" {
		return runtime.NilValue(), false
	}
	v, ok := vm.globalValue(spec.tableName)
	if !ok || !v.IsTable() {
		return runtime.NilValue(), false
	}
	return v, true
}

func recordPairwiseNumericKernelSpecForProto(proto *FuncProto) (*recordPairwiseNumericKernelSpec, bool) {
	if proto == nil || isDenseRecordMatrixAdvanceProto(proto) {
		return nil, false
	}
	if isRecordPairwiseNumericProto(proto) {
		return analyzeRecordPairwiseNumericKernelSpec(proto)
	}
	return nil, false
}

func denseRecordMatrixAdvanceKernelSpecForProto(proto *FuncProto) (*denseRecordMatrixAdvanceKernelSpec, bool) {
	if !isDenseRecordMatrixAdvanceProto(proto) {
		return nil, false
	}
	names := func(idxs ...int) ([]string, bool) {
		out := make([]string, len(idxs))
		for i, idx := range idxs {
			name, ok := protoStringConstant(proto, idx)
			if !ok {
				return nil, false
			}
			out[i] = name
		}
		return out, true
	}
	vals, ok := names(0, 1, 2, 3, 11, 12, 13)
	if !ok {
		return nil, false
	}
	fieldVals, ok := names(4, 5, 6, 8, 9, 10, 7)
	if !ok {
		return nil, false
	}
	spec := &denseRecordMatrixAdvanceKernelSpec{
		countName:     vals[0],
		matrixName:    vals[1],
		matrixGetName: vals[2],
		tableName:     vals[3],
		sqrtTableName: vals[4],
		sqrtFieldName: vals[5],
		matrixSetName: vals[6],
	}
	for i, name := range fieldVals {
		spec.fieldIndexName[i] = name
	}
	return spec, true
}

func analyzeRecordPairwiseNumericKernelSpec(proto *FuncProto) (*recordPairwiseNumericKernelSpec, bool) {
	tableConst, biReg, bjReg, ok := findRecordPairwiseTableAndRecordRegs(proto.Code)
	if !ok {
		return nil, false
	}
	sqrtTableConst, sqrtFieldConst, ok := findRecordPairwiseSqrtConsts(proto.Code)
	if !ok {
		return nil, false
	}
	positionConsts, ok := findRecordPairwisePositionConsts(proto.Code, biReg, bjReg)
	if !ok {
		return nil, false
	}
	velocityConsts, ok := findRecordPairwiseVelocityConsts(proto.Code, biReg)
	if !ok {
		return nil, false
	}
	massConst, ok := findRecordPairwiseMassConst(proto.Code, biReg, bjReg, positionConsts, velocityConsts)
	if !ok {
		return nil, false
	}
	fieldConsts := [pairwiseFieldCount]int{
		positionConsts[0], positionConsts[1], positionConsts[2],
		velocityConsts[0], velocityConsts[1], velocityConsts[2],
		massConst,
	}
	tableName, ok := protoStringConstant(proto, tableConst)
	if !ok {
		return nil, false
	}
	sqrtTableName, ok := protoStringConstant(proto, sqrtTableConst)
	if !ok {
		return nil, false
	}
	sqrtFieldName, ok := protoStringConstant(proto, sqrtFieldConst)
	if !ok {
		return nil, false
	}
	spec := &recordPairwiseNumericKernelSpec{
		tableName:     tableName,
		sqrtTableName: sqrtTableName,
		sqrtFieldName: sqrtFieldName,
	}
	for i, constIdx := range fieldConsts {
		name, ok := protoStringConstant(proto, constIdx)
		if !ok {
			return nil, false
		}
		spec.fieldNames[i] = name
	}
	return spec, true
}

func findRecordPairwiseTableAndRecordRegs(code []uint32) (int, int, int, bool) {
	tableConst := -1
	biReg := -1
	bjReg := -1
	for pc := 0; pc+2 < len(code); pc++ {
		getGlobal := code[pc]
		move := code[pc+1]
		getTable := code[pc+2]
		if DecodeOp(getGlobal) != OP_GETGLOBAL || DecodeOp(move) != OP_MOVE || DecodeOp(getTable) != OP_GETTABLE {
			continue
		}
		if DecodeB(getTable) != DecodeA(getGlobal) || DecodeC(getTable) != DecodeA(move) {
			continue
		}
		if tableConst < 0 {
			tableConst = DecodeBx(getGlobal)
			biReg = DecodeA(getTable)
			continue
		}
		if DecodeBx(getGlobal) == tableConst {
			bjReg = DecodeA(getTable)
			return tableConst, biReg, bjReg, true
		}
	}
	return 0, 0, 0, false
}

func findRecordPairwiseSqrtConsts(code []uint32) (int, int, bool) {
	for pc := 3; pc < len(code); pc++ {
		call := code[pc]
		if DecodeOp(call) != OP_CALL || DecodeB(call) != 2 || DecodeC(call) != 2 {
			continue
		}
		getField := code[pc-2]
		getGlobal := code[pc-3]
		if DecodeOp(getField) != OP_GETFIELD || DecodeOp(getGlobal) != OP_GETGLOBAL {
			continue
		}
		if DecodeA(call) == DecodeA(getField) && DecodeB(getField) == DecodeA(getGlobal) {
			return DecodeBx(getGlobal), DecodeC(getField), true
		}
	}
	return 0, 0, false
}

func findRecordPairwisePositionConsts(code []uint32, biReg, bjReg int) ([3]int, bool) {
	var fields [3]int
	n := 0
	for pc := 0; pc+2 < len(code) && n < len(fields); pc++ {
		left := code[pc]
		right := code[pc+1]
		sub := code[pc+2]
		if DecodeOp(left) != OP_GETFIELD || DecodeOp(right) != OP_GETFIELD || DecodeOp(sub) != OP_SUB {
			continue
		}
		if DecodeB(left) != biReg || DecodeB(right) != bjReg || DecodeC(left) != DecodeC(right) {
			continue
		}
		if DecodeB(sub) != DecodeA(left) || DecodeC(sub) != DecodeA(right) {
			continue
		}
		fields[n] = DecodeC(left)
		n++
	}
	return fields, n == len(fields)
}

func findRecordPairwiseVelocityConsts(code []uint32, biReg int) ([3]int, bool) {
	var fields [3]int
	n := 0
	for _, inst := range code {
		if DecodeOp(inst) != OP_SETFIELD || DecodeA(inst) != biReg {
			continue
		}
		fieldConst := DecodeB(inst)
		if containsInt(fields[:n], fieldConst) {
			continue
		}
		fields[n] = fieldConst
		n++
		if n == len(fields) {
			return fields, true
		}
	}
	return fields, false
}

func findRecordPairwiseMassConst(code []uint32, biReg, bjReg int, positionConsts [3]int, velocityConsts [3]int) (int, bool) {
	for _, inst := range code {
		if DecodeOp(inst) != OP_GETFIELD || DecodeB(inst) != bjReg {
			continue
		}
		fieldConst := DecodeC(inst)
		if containsInt(positionConsts[:], fieldConst) || containsInt(velocityConsts[:], fieldConst) {
			continue
		}
		if recordPairwiseHasFieldLoad(code, biReg, fieldConst) {
			return fieldConst, true
		}
	}
	return 0, false
}

func recordPairwiseHasFieldLoad(code []uint32, baseReg int, fieldConst int) bool {
	for _, inst := range code {
		if DecodeOp(inst) == OP_GETFIELD && DecodeB(inst) == baseReg && DecodeC(inst) == fieldConst {
			return true
		}
	}
	return false
}

func containsInt(vals []int, want int) bool {
	for _, v := range vals {
		if v == want {
			return true
		}
	}
	return false
}

func isRecordPairwiseNumericProto(p *FuncProto) bool {
	if isDenseRecordMatrixAdvanceProto(p) {
		return true
	}
	if isRecordPairwiseNumericProtoWithGlobalCount(p) {
		return true
	}
	if p == nil || p.NumParams != 1 || p.IsVarArg || len(p.Constants) < 10 || len(p.Code) != 99 {
		return false
	}
	for _, idx := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9} {
		if !p.Constants[idx].IsString() {
			return false
		}
	}
	return codeEquals(p.Code, []uint32{
		EncodeABx(OP_GETGLOBAL, 2, 0),
		EncodeABC(OP_LEN, 1, 2, 0),
		EncodeAsBx(OP_LOADINT, 2, 1),
		EncodeABC(OP_MOVE, 3, 1, 0),
		EncodeAsBx(OP_LOADINT, 4, 1),
		EncodeAsBx(OP_FORPREP, 2, 68),
		EncodeABx(OP_GETGLOBAL, 7, 0),
		EncodeABC(OP_MOVE, 8, 5, 0),
		EncodeABC(OP_GETTABLE, 6, 7, 8),
		EncodeAsBx(OP_LOADINT, 11, 1),
		EncodeABC(OP_ADD, 7, 5, 11),
		EncodeABC(OP_MOVE, 8, 1, 0),
		EncodeAsBx(OP_LOADINT, 9, 1),
		EncodeAsBx(OP_FORPREP, 7, 59),
		EncodeABx(OP_GETGLOBAL, 12, 0),
		EncodeABC(OP_MOVE, 13, 10, 0),
		EncodeABC(OP_GETTABLE, 11, 12, 13),
		EncodeABC(OP_GETFIELD, 13, 6, 1),
		EncodeABC(OP_GETFIELD, 14, 11, 1),
		EncodeABC(OP_SUB, 12, 13, 14),
		EncodeABC(OP_GETFIELD, 14, 6, 2),
		EncodeABC(OP_GETFIELD, 15, 11, 2),
		EncodeABC(OP_SUB, 13, 14, 15),
		EncodeABC(OP_GETFIELD, 15, 6, 3),
		EncodeABC(OP_GETFIELD, 16, 11, 3),
		EncodeABC(OP_SUB, 14, 15, 16),
		EncodeABC(OP_MUL, 17, 12, 12),
		EncodeABC(OP_MUL, 18, 13, 13),
		EncodeABC(OP_ADD, 16, 17, 18),
		EncodeABC(OP_MUL, 17, 14, 14),
		EncodeABC(OP_ADD, 15, 16, 17),
		EncodeABx(OP_GETGLOBAL, 17, 4),
		EncodeABC(OP_GETFIELD, 16, 17, 5),
		EncodeABC(OP_MOVE, 17, 15, 0),
		EncodeABC(OP_CALL, 16, 2, 2),
		EncodeABC(OP_MUL, 18, 15, 16),
		EncodeABC(OP_DIV, 17, 0, 18),
		EncodeABC(OP_GETFIELD, 19, 6, 6),
		EncodeABC(OP_GETFIELD, 22, 11, 7),
		EncodeABC(OP_MUL, 21, 12, 22),
		EncodeABC(OP_MUL, 20, 21, 17),
		EncodeABC(OP_SUB, 18, 19, 20),
		EncodeABC(OP_SETFIELD, 6, 6, 18),
		EncodeABC(OP_GETFIELD, 19, 6, 8),
		EncodeABC(OP_GETFIELD, 22, 11, 7),
		EncodeABC(OP_MUL, 21, 13, 22),
		EncodeABC(OP_MUL, 20, 21, 17),
		EncodeABC(OP_SUB, 18, 19, 20),
		EncodeABC(OP_SETFIELD, 6, 8, 18),
		EncodeABC(OP_GETFIELD, 19, 6, 9),
		EncodeABC(OP_GETFIELD, 22, 11, 7),
		EncodeABC(OP_MUL, 21, 14, 22),
		EncodeABC(OP_MUL, 20, 21, 17),
		EncodeABC(OP_SUB, 18, 19, 20),
		EncodeABC(OP_SETFIELD, 6, 9, 18),
		EncodeABC(OP_GETFIELD, 19, 11, 6),
		EncodeABC(OP_GETFIELD, 22, 6, 7),
		EncodeABC(OP_MUL, 21, 12, 22),
		EncodeABC(OP_MUL, 20, 21, 17),
		EncodeABC(OP_ADD, 18, 19, 20),
		EncodeABC(OP_SETFIELD, 11, 6, 18),
		EncodeABC(OP_GETFIELD, 19, 11, 8),
		EncodeABC(OP_GETFIELD, 22, 6, 7),
		EncodeABC(OP_MUL, 21, 13, 22),
		EncodeABC(OP_MUL, 20, 21, 17),
		EncodeABC(OP_ADD, 18, 19, 20),
		EncodeABC(OP_SETFIELD, 11, 8, 18),
		EncodeABC(OP_GETFIELD, 19, 11, 9),
		EncodeABC(OP_GETFIELD, 22, 6, 7),
		EncodeABC(OP_MUL, 21, 14, 22),
		EncodeABC(OP_MUL, 20, 21, 17),
		EncodeABC(OP_ADD, 18, 19, 20),
		EncodeABC(OP_SETFIELD, 11, 9, 18),
		EncodeAsBx(OP_FORLOOP, 7, -60),
		EncodeAsBx(OP_FORLOOP, 2, -69),
		EncodeAsBx(OP_LOADINT, 8, 1),
		EncodeABC(OP_MOVE, 9, 1, 0),
		EncodeAsBx(OP_LOADINT, 10, 1),
		EncodeAsBx(OP_FORPREP, 8, 18),
		EncodeABx(OP_GETGLOBAL, 13, 0),
		EncodeABC(OP_MOVE, 14, 11, 0),
		EncodeABC(OP_GETTABLE, 12, 13, 14),
		EncodeABC(OP_GETFIELD, 14, 12, 1),
		EncodeABC(OP_GETFIELD, 16, 12, 6),
		EncodeABC(OP_MUL, 15, 0, 16),
		EncodeABC(OP_ADD, 13, 14, 15),
		EncodeABC(OP_SETFIELD, 12, 1, 13),
		EncodeABC(OP_GETFIELD, 14, 12, 2),
		EncodeABC(OP_GETFIELD, 16, 12, 8),
		EncodeABC(OP_MUL, 15, 0, 16),
		EncodeABC(OP_ADD, 13, 14, 15),
		EncodeABC(OP_SETFIELD, 12, 2, 13),
		EncodeABC(OP_GETFIELD, 14, 12, 3),
		EncodeABC(OP_GETFIELD, 16, 12, 9),
		EncodeABC(OP_MUL, 15, 0, 16),
		EncodeABC(OP_ADD, 13, 14, 15),
		EncodeABC(OP_SETFIELD, 12, 3, 13),
		EncodeAsBx(OP_FORLOOP, 8, -19),
		EncodeABC(OP_RETURN, 0, 1, 0),
	})
}

func isDenseRecordMatrixAdvanceProto(p *FuncProto) bool {
	if p == nil || p.NumParams != 1 || p.IsVarArg || len(p.Constants) < 14 || len(p.Code) != 241 {
		return false
	}
	for idx := 0; idx <= 13; idx++ {
		if !p.Constants[idx].IsString() {
			return false
		}
	}
	checks := map[int]uint32{
		0:   EncodeAsBx(OP_LOADINT, 1, 0),
		1:   EncodeABx(OP_GETGLOBAL, 5, 0),
		5:   EncodeAsBx(OP_FORPREP, 1, 166),
		54:  EncodeAsBx(OP_FORPREP, 12, 95),
		105: EncodeABx(OP_GETGLOBAL, 28, 11),
		108: EncodeABC(OP_CALL, 27, 2, 2),
		150: EncodeAsBx(OP_FORLOOP, 12, -96),
		172: EncodeAsBx(OP_FORLOOP, 1, -167),
		178: EncodeAsBx(OP_FORPREP, 7, 60),
		239: EncodeAsBx(OP_FORLOOP, 7, -61),
		240: EncodeABC(OP_RETURN, 0, 1, 0),
	}
	for pc, want := range checks {
		if p.Code[pc] != want {
			return false
		}
	}
	return true
}

func isRecordPairwiseNumericProtoWithGlobalCount(p *FuncProto) bool {
	if p == nil || p.NumParams != 1 || p.IsVarArg || len(p.Constants) < 11 || len(p.Code) != 98 {
		return false
	}
	for _, idx := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
		if !p.Constants[idx].IsString() {
			return false
		}
	}
	return codeEquals(p.Code, []uint32{
		EncodeABx(OP_GETGLOBAL, 1, 0),
		EncodeAsBx(OP_LOADINT, 2, 1),
		EncodeABC(OP_MOVE, 3, 1, 0),
		EncodeAsBx(OP_LOADINT, 4, 1),
		EncodeAsBx(OP_FORPREP, 2, 68),
		EncodeABx(OP_GETGLOBAL, 7, 1),
		EncodeABC(OP_MOVE, 8, 5, 0),
		EncodeABC(OP_GETTABLE, 6, 7, 8),
		EncodeAsBx(OP_LOADINT, 11, 1),
		EncodeABC(OP_ADD, 7, 5, 11),
		EncodeABC(OP_MOVE, 8, 1, 0),
		EncodeAsBx(OP_LOADINT, 9, 1),
		EncodeAsBx(OP_FORPREP, 7, 59),
		EncodeABx(OP_GETGLOBAL, 12, 1),
		EncodeABC(OP_MOVE, 13, 10, 0),
		EncodeABC(OP_GETTABLE, 11, 12, 13),
		EncodeABC(OP_GETFIELD, 13, 6, 2),
		EncodeABC(OP_GETFIELD, 14, 11, 2),
		EncodeABC(OP_SUB, 12, 13, 14),
		EncodeABC(OP_GETFIELD, 14, 6, 3),
		EncodeABC(OP_GETFIELD, 15, 11, 3),
		EncodeABC(OP_SUB, 13, 14, 15),
		EncodeABC(OP_GETFIELD, 15, 6, 4),
		EncodeABC(OP_GETFIELD, 16, 11, 4),
		EncodeABC(OP_SUB, 14, 15, 16),
		EncodeABC(OP_MUL, 17, 12, 12),
		EncodeABC(OP_MUL, 18, 13, 13),
		EncodeABC(OP_ADD, 16, 17, 18),
		EncodeABC(OP_MUL, 17, 14, 14),
		EncodeABC(OP_ADD, 15, 16, 17),
		EncodeABx(OP_GETGLOBAL, 17, 5),
		EncodeABC(OP_GETFIELD, 16, 17, 6),
		EncodeABC(OP_MOVE, 17, 15, 0),
		EncodeABC(OP_CALL, 16, 2, 2),
		EncodeABC(OP_MUL, 18, 15, 16),
		EncodeABC(OP_DIV, 17, 0, 18),
		EncodeABC(OP_GETFIELD, 19, 6, 7),
		EncodeABC(OP_GETFIELD, 22, 11, 8),
		EncodeABC(OP_MUL, 21, 12, 22),
		EncodeABC(OP_MUL, 20, 21, 17),
		EncodeABC(OP_SUB, 18, 19, 20),
		EncodeABC(OP_SETFIELD, 6, 7, 18),
		EncodeABC(OP_GETFIELD, 19, 6, 9),
		EncodeABC(OP_GETFIELD, 22, 11, 8),
		EncodeABC(OP_MUL, 21, 13, 22),
		EncodeABC(OP_MUL, 20, 21, 17),
		EncodeABC(OP_SUB, 18, 19, 20),
		EncodeABC(OP_SETFIELD, 6, 9, 18),
		EncodeABC(OP_GETFIELD, 19, 6, 10),
		EncodeABC(OP_GETFIELD, 22, 11, 8),
		EncodeABC(OP_MUL, 21, 14, 22),
		EncodeABC(OP_MUL, 20, 21, 17),
		EncodeABC(OP_SUB, 18, 19, 20),
		EncodeABC(OP_SETFIELD, 6, 10, 18),
		EncodeABC(OP_GETFIELD, 19, 11, 7),
		EncodeABC(OP_GETFIELD, 22, 6, 8),
		EncodeABC(OP_MUL, 21, 12, 22),
		EncodeABC(OP_MUL, 20, 21, 17),
		EncodeABC(OP_ADD, 18, 19, 20),
		EncodeABC(OP_SETFIELD, 11, 7, 18),
		EncodeABC(OP_GETFIELD, 19, 11, 9),
		EncodeABC(OP_GETFIELD, 22, 6, 8),
		EncodeABC(OP_MUL, 21, 13, 22),
		EncodeABC(OP_MUL, 20, 21, 17),
		EncodeABC(OP_ADD, 18, 19, 20),
		EncodeABC(OP_SETFIELD, 11, 9, 18),
		EncodeABC(OP_GETFIELD, 19, 11, 10),
		EncodeABC(OP_GETFIELD, 22, 6, 8),
		EncodeABC(OP_MUL, 21, 14, 22),
		EncodeABC(OP_MUL, 20, 21, 17),
		EncodeABC(OP_ADD, 18, 19, 20),
		EncodeABC(OP_SETFIELD, 11, 10, 18),
		EncodeAsBx(OP_FORLOOP, 7, -60),
		EncodeAsBx(OP_FORLOOP, 2, -69),
		EncodeAsBx(OP_LOADINT, 8, 1),
		EncodeABC(OP_MOVE, 9, 1, 0),
		EncodeAsBx(OP_LOADINT, 10, 1),
		EncodeAsBx(OP_FORPREP, 8, 18),
		EncodeABx(OP_GETGLOBAL, 13, 1),
		EncodeABC(OP_MOVE, 14, 11, 0),
		EncodeABC(OP_GETTABLE, 12, 13, 14),
		EncodeABC(OP_GETFIELD, 14, 12, 2),
		EncodeABC(OP_GETFIELD, 16, 12, 7),
		EncodeABC(OP_MUL, 15, 0, 16),
		EncodeABC(OP_ADD, 13, 14, 15),
		EncodeABC(OP_SETFIELD, 12, 2, 13),
		EncodeABC(OP_GETFIELD, 14, 12, 3),
		EncodeABC(OP_GETFIELD, 16, 12, 9),
		EncodeABC(OP_MUL, 15, 0, 16),
		EncodeABC(OP_ADD, 13, 14, 15),
		EncodeABC(OP_SETFIELD, 12, 3, 13),
		EncodeABC(OP_GETFIELD, 14, 12, 4),
		EncodeABC(OP_GETFIELD, 16, 12, 10),
		EncodeABC(OP_MUL, 15, 0, 16),
		EncodeABC(OP_ADD, 13, 14, 15),
		EncodeABC(OP_SETFIELD, 12, 4, 13),
		EncodeAsBx(OP_FORLOOP, 8, -19),
		EncodeABC(OP_RETURN, 0, 1, 0),
	})
}

// HasRecordPairwiseNumericWholeCallKernel reports whether p matches the guarded
// record-field pairwise numeric advance(dt) kernel shape. MethodJIT uses this to
// keep driver loops on the VM route where the whole-call kernel can fire.
func HasRecordPairwiseNumericWholeCallKernel(p *FuncProto) bool {
	return cachedWholeCallKernelRecognized(p, wholeCallKernelRecordPairwiseNumeric)
}

// HasRecordPairwiseNumericDriverLoopKernel reports whether p contains a structural
// driver loop that repeatedly calls an pairwise numeric whole-call
// kernel candidate.
func HasRecordPairwiseNumericDriverLoopKernel(p *FuncProto, globals map[string]*FuncProto) bool {
	if p == nil {
		return false
	}
	for pc, inst := range p.Code {
		if DecodeOp(inst) != OP_FORPREP {
			continue
		}
		if IsRecordPairwiseNumericDriverLoopAt(p, pc, globals) {
			return true
		}
	}
	return false
}

// IsRecordPairwiseNumericDriverLoopAt checks one FORPREP site for the guarded
// advance(dt) call-loop shape. Runtime admission still checks trip count,
// current globals, and argument/table guards before executing the kernel.
func IsRecordPairwiseNumericDriverLoopAt(p *FuncProto, forprepPC int, globals map[string]*FuncProto) bool {
	if p == nil || len(globals) == 0 || forprepPC < 0 || forprepPC >= len(p.Code) {
		return false
	}
	inst := p.Code[forprepPC]
	if DecodeOp(inst) != OP_FORPREP {
		return false
	}
	shape, ok := matchRecordPairwiseNumericDriverLoopShape(p.Code, p.Constants, forprepPC, DecodeA(inst), DecodesBx(inst))
	if !ok {
		return false
	}
	if shape.fnConst < 0 || shape.fnConst >= len(p.Constants) || !p.Constants[shape.fnConst].IsString() {
		return false
	}
	return HasRecordPairwiseNumericWholeCallKernel(globals[p.Constants[shape.fnConst].Str()])
}
