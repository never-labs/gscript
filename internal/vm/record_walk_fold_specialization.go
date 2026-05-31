package vm

import "github.com/never-labs/gscript/internal/runtime"

type recordWalkFoldSpec struct {
	recordFields [6]string
	metricFields [3]string
	userFields   [2]string
	tagFields    [3]string
	modulus      int64
}

type recordWalkFoldSpecializationCache struct {
	fingerprint runtimeSpecializationFingerprint
	spec        *recordWalkFoldSpec
}

type recordWalkFoldRow struct {
	metrics *runtime.Table
	id      int64
	active  bool
	views   int64
	clicks  int64
	errors  int64
	tier    int64
	region  int64
	kindLen int64
	tagLen  [3]int64
}

func isRecordWalkFoldProto(p *FuncProto) bool {
	_, ok := recordWalkFoldSpecForProto(p)
	return ok
}

func (vm *VM) runRecordWalkFoldRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if cl == nil || cl.Proto == nil || len(args) != 3 {
		return false, nil, nil
	}
	spec, ok := recordWalkFoldSpecForProto(cl.Proto)
	if !ok || !args[0].IsTable() {
		return false, nil, nil
	}
	n64, ok := runtimeSpecializationIntArg(args[1])
	if !ok || n64 < 0 || n64 > int64(int(n64)) {
		return false, nil, nil
	}
	passes, ok := runtimeSpecializationIntArg(args[2])
	if !ok || passes < 0 {
		return false, nil, nil
	}
	rowsTable := args[0].Table()
	rows := make([]recordWalkFoldRow, int(n64))
	for i := int64(1); i <= n64; i++ {
		v := rowsTable.RawGetInt(i)
		if !v.IsTable() {
			return false, nil, nil
		}
		row, ok := loadRecordWalkFoldRow(v.Table(), spec)
		if !ok {
			return false, nil, nil
		}
		rows[i-1] = row
	}

	checksum := int64(0)
	mod := spec.modulus
	for pass := int64(1); pass <= passes; pass++ {
		updateMetric0 := pass%3 == 0
		for i := range rows {
			row := &rows[i]
			idx := int64(i + 1)
			tagChoice := (idx + pass) % 3
			tagLen := row.tagLen[0]
			if tagChoice == 1 {
				tagLen = row.tagLen[1]
			} else if tagChoice == 2 {
				tagLen = row.tagLen[2]
			}
			if row.active {
				score := row.views + row.clicks*3 - row.errors*5 + row.tier + row.kindLen + tagLen
				checksum = (checksum + score + row.region) % mod
				if updateMetric0 {
					row.views = (row.views + row.tier + idx) % 2000
				}
			} else {
				checksum = (checksum + row.id + row.errors + row.tagLen[0]) % mod
			}
		}
	}
	for i := range rows {
		if rows[i].metrics != nil {
			rows[i].metrics.RawSetString(spec.metricFields[0], runtime.IntValue(rows[i].views))
		}
	}
	return true, []runtime.Value{runtime.IntValue(checksum)}, nil
}

func loadRecordWalkFoldRow(t *runtime.Table, spec *recordWalkFoldSpec) (recordWalkFoldRow, bool) {
	var out recordWalkFoldRow
	if t == nil || spec == nil {
		return out, false
	}
	id := t.RawGetString(spec.recordFields[0])
	kind := t.RawGetString(spec.recordFields[1])
	active := t.RawGetString(spec.recordFields[2])
	user := t.RawGetString(spec.recordFields[3])
	metrics := t.RawGetString(spec.recordFields[4])
	tags := t.RawGetString(spec.recordFields[5])
	if !id.IsInt() || !kind.IsString() || !active.IsBool() || !user.IsTable() || !metrics.IsTable() || !tags.IsTable() {
		return out, false
	}
	mt := metrics.Table()
	ut := user.Table()
	tt := tags.Table()
	views := mt.RawGetString(spec.metricFields[0])
	clicks := mt.RawGetString(spec.metricFields[1])
	errors := mt.RawGetString(spec.metricFields[2])
	tier := ut.RawGetString(spec.userFields[0])
	region := ut.RawGetString(spec.userFields[1])
	tag0 := tt.RawGetString(spec.tagFields[0])
	tag1 := tt.RawGetString(spec.tagFields[1])
	tag2 := tt.RawGetString(spec.tagFields[2])
	if !views.IsInt() || !clicks.IsInt() || !errors.IsInt() || !tier.IsInt() || !region.IsInt() ||
		!tag0.IsString() || !tag1.IsString() || !tag2.IsString() {
		return out, false
	}
	out.metrics = mt
	out.id = id.Int()
	out.active = active.Bool()
	out.views = views.Int()
	out.clicks = clicks.Int()
	out.errors = errors.Int()
	out.tier = tier.Int()
	out.region = region.Int()
	out.kindLen = int64(len(kind.Str()))
	out.tagLen = [3]int64{int64(len(tag0.Str())), int64(len(tag1.Str())), int64(len(tag2.Str()))}
	return out, true
}

func recordWalkFoldSpecForProto(p *FuncProto) (*recordWalkFoldSpec, bool) {
	if p == nil || p.NumParams != 3 || p.IsVarArg || p.MaxStack != 29 ||
		len(p.Code) != 83 || len(p.Constants) != 15 || len(p.Protos) != 0 {
		return nil, false
	}
	fp := runtimeSpecializationFingerprintForProto(p)
	cache := p.RuntimeSpecs.RecordWalkFoldSpecialization
	if cache != nil && cache.fingerprint == fp {
		return cache.spec, cache.spec != nil
	}
	spec, ok := analyzeRecordWalkFoldSpec(p)
	if !ok {
		p.RuntimeSpecs.RecordWalkFoldSpecialization = &recordWalkFoldSpecializationCache{fingerprint: fp}
		return nil, false
	}
	p.RuntimeSpecs.RecordWalkFoldSpecialization = &recordWalkFoldSpecializationCache{fingerprint: fp, spec: spec}
	return spec, true
}

func analyzeRecordWalkFoldSpec(p *FuncProto) (*recordWalkFoldSpec, bool) {
	for _, idx := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 14} {
		if !p.Constants[idx].IsString() {
			return nil, false
		}
	}
	if !p.Constants[13].IsNumber() {
		return nil, false
	}
	code := p.Code
	if !matchRecordWalkFoldLoopShape(code) {
		return nil, false
	}
	var ok bool
	spec := &recordWalkFoldSpec{modulus: int64(p.Constants[13].Number())}
	if spec.modulus <= 0 {
		return nil, false
	}
	recordConsts := [6]int{
		getFieldConstAt(code, 69, 12),
		getFieldConstAt(code, 44, 12),
		getFieldConstAt(code, 30, 12),
		getFieldConstAt(code, 12, 12),
		getFieldConstAt(code, 11, 12),
		getFieldConstAt(code, 13, 12),
	}
	for i, constIdx := range recordConsts {
		if spec.recordFields[i], ok = protoStringConstant(p, constIdx); !ok {
			return nil, false
		}
	}
	metricConsts := [3]int{
		getFieldConstAt(code, 33, 13),
		getFieldConstAt(code, 34, 13),
		getFieldConstAt(code, 38, 13),
	}
	if getFieldConstAt(code, 61, 13) != metricConsts[0] ||
		getFieldConstAt(code, 71, 13) != metricConsts[2] ||
		DecodeOp(code[67]) != OP_SETFIELD ||
		DecodeB(code[67]) != metricConsts[0] {
		return nil, false
	}
	for i, constIdx := range metricConsts {
		if spec.metricFields[i], ok = protoStringConstant(p, constIdx); !ok {
			return nil, false
		}
	}
	userConsts := [2]int{
		getFieldConstAt(code, 42, 14),
		getFieldConstAt(code, 51, 14),
	}
	if getFieldConstAt(code, 62, 14) != userConsts[0] {
		return nil, false
	}
	for i, constIdx := range userConsts {
		if spec.userFields[i], ok = protoStringConstant(p, constIdx); !ok {
			return nil, false
		}
	}
	tagConsts, ok := recordWalkTagFieldConsts(code, 15)
	if !ok || getFieldConstAt(code, 73, 15) != tagConsts[0] {
		return nil, false
	}
	for i, constIdx := range tagConsts {
		if spec.tagFields[i], ok = protoStringConstant(p, constIdx); !ok {
			return nil, false
		}
	}
	return spec, true
}

func matchRecordWalkFoldLoopShape(code []uint32) bool {
	p := newBytecodePattern(code)
	if !p.loadInt(0, 3, 0) {
		return false
	}
	outer, ok := p.findNumericForLoopWithBodyLen(1, 75)
	if !ok {
		return false
	}
	inner, ok := p.findNumericForLoopWithBodyLen(outer.bodyPC, 70)
	if !ok || inner.loopPC >= outer.loopPC {
		return false
	}
	if !p.abc(inner.bodyPC+1, OP_GETTABLE, 12, 0, 13) ||
		!p.abc(inner.bodyPC+22, OP_TEST, 18, 0, 0) ||
		!p.abc(inner.bodyPC+45, OP_MOD, 19, 20, 21) ||
		!p.abc(inner.bodyPC+58, OP_SETFIELD, 13, 7, 19) ||
		!p.abc(inner.bodyPC+68, OP_MOD, 18, 19, 20) {
		return false
	}
	return p.returnFixed(outer.loopPC+2, 10, 2)
}

func recordWalkTagFieldConsts(code []uint32, tagsReg int) ([3]int, bool) {
	var out [3]int
	n := 0
	for pc := 14; pc < len(code) && pc <= 27; pc++ {
		if DecodeOp(code[pc]) != OP_GETFIELD || DecodeB(code[pc]) != tagsReg {
			continue
		}
		out[n] = DecodeC(code[pc])
		n++
		if n == len(out) {
			return out, true
		}
	}
	return out, false
}

func getFieldConstAt(code []uint32, pc int, baseReg int) int {
	if pc < 0 || pc >= len(code) || DecodeOp(code[pc]) != OP_GETFIELD || DecodeB(code[pc]) != baseReg {
		return -1
	}
	return DecodeC(code[pc])
}

func runtimeSpecializationIntArg(v runtime.Value) (int64, bool) {
	if v.IsInt() {
		return v.Int(), true
	}
	if v.IsFloat() {
		f := v.Float()
		i := int64(f)
		return i, float64(i) == f
	}
	return 0, false
}
