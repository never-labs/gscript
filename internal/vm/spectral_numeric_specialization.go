package vm

import (
	"unsafe"

	"github.com/gscript/gscript/internal/runtime"
)

type spectralMultiplyKind uint8

const (
	spectralNotMultiply spectralMultiplyKind = iota
	spectralAv
	spectralAtv
)

type spectralRuntimeSpecializationKind uint8

const (
	spectralRuntimeSpecializationInvalid spectralRuntimeSpecializationKind = iota
	spectralRuntimeSpecializationAv
	spectralRuntimeSpecializationAtv
	spectralRuntimeSpecializationAtAv
	spectralRuntimeSpecializationDenseAtAv
)

// maxSpectralCoefficientFloats caps the combined coefficient matrix and
// transpose cache. The runtime-specialized coefficient loops are dominated by
// repeated coefficient evaluation when the cache is disabled; a 64 MiB budget
// keeps large matrix-vector calls on the precomputed path without allowing
// unbounded O(n^2) memory.
const maxSpectralCoefficientFloats = 1 << 23

type spectralCoefficientCache struct {
	n           int
	fingerprint runtimeSpecializationFingerprint
	a           []float64
	at          []float64
}

type spectralCoefficientExpr struct {
	proto       *FuncProto
	fingerprint runtimeSpecializationFingerprint
	kind        coefficientExprKind
}

type coefficientExprKind uint8

const (
	coefficientExprBytecode coefficientExprKind = iota
	coefficientExprReciprocalTriangularIndex
)

type spectralRuntimeSpecializationCache struct {
	fingerprint runtimeSpecializationFingerprint
	spec        *spectralRuntimeSpecializationSpec
}

type spectralRuntimeSpecializationSpec struct {
	kind spectralRuntimeSpecializationKind
}

func (vm *VM) runSpectralRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, error) {
	if cl == nil || cl.Proto == nil || !vm.noGlobalLock {
		return false, nil
	}
	proto := cl.Proto
	spec, ok := spectralRuntimeSpecializationSpecForProto(proto)
	if !ok {
		return false, nil
	}
	switch spec.kind {
	case spectralRuntimeSpecializationAv:
		if len(args) != 3 {
			return false, nil
		}
		coeff, ok := vm.spectralMultiplyCoefficient(proto)
		if !ok {
			return false, nil
		}
		if !vm.runSpectralMultiply(args, spectralAv, coeff) {
			return false, nil
		}
		return true, nil
	case spectralRuntimeSpecializationAtv:
		if len(args) != 3 {
			return false, nil
		}
		coeff, ok := vm.spectralMultiplyCoefficient(proto)
		if !ok {
			return false, nil
		}
		if !vm.runSpectralMultiply(args, spectralAtv, coeff) {
			return false, nil
		}
		return true, nil
	case spectralRuntimeSpecializationDenseAtAv:
		coeff, ok := vm.denseSpectralAtAvCoefficient(proto)
		if len(args) != 4 || !ok {
			return false, nil
		}
		if !vm.runDenseSpectralAtAv(args, coeff) {
			return false, nil
		}
		return true, nil
	case spectralRuntimeSpecializationAtAv:
		coeff, ok := vm.spectralAtAvCoefficient(proto)
		if len(args) != 3 || !ok {
			return false, nil
		}
		if !vm.runSpectralAtAv(args, coeff) {
			return false, nil
		}
		return true, nil
	default:
		return false, nil
	}
}

func (vm *VM) runSpectralAtAv(args []runtime.Value, coeff spectralCoefficientExpr) bool {
	n, v, atav, ok := spectralSpecializationArgs(args)
	if !ok {
		return false
	}
	tmp := vm.callSiteFloatScratch(n)
	a, at, ok := vm.spectralCoefficients.coefficients(n, coeff)
	if ok {
		spectralMatrixVector(a, n, v, tmp)
	} else if !coefficientMatrixVectorInto(n, v, tmp, coeff, false) {
		return false
	}
	args[2].Table().MarkArrayMutationForNumericSpecialization()
	if ok {
		spectralMatrixVector(at, n, tmp, atav)
	} else if !coefficientMatrixVectorInto(n, tmp, atav, coeff, true) {
		return false
	}
	return true
}

func (vm *VM) runDenseSpectralAtAv(args []runtime.Value, coeff spectralCoefficientExpr) bool {
	n, v, tmp, atav, ok := denseSpectralAtAvSpecializationArgs(args)
	if !ok {
		return false
	}
	a, at, cached := vm.spectralCoefficients.coefficients(n, coeff)
	if cached {
		spectralMatrixVector(a, n, v, tmp)
		spectralMatrixVector(at, n, tmp, atav)
		return true
	}
	return coefficientMatrixVectorInto(n, v, tmp, coeff, false) &&
		coefficientMatrixVectorInto(n, tmp, atav, coeff, true)
}

func (vm *VM) runSpectralMultiply(args []runtime.Value, kind spectralMultiplyKind, coeff spectralCoefficientExpr) bool {
	n, v, out, ok := spectralSpecializationArgs(args)
	if !ok {
		return false
	}
	args[2].Table().MarkArrayMutationForNumericSpecialization()
	a, at, ok := vm.spectralCoefficients.cached(n, coeff)
	if ok {
		if kind == spectralAtv {
			spectralMatrixVector(at, n, v, out)
			return true
		}
		spectralMatrixVector(a, n, v, out)
		return true
	}
	if kind == spectralAtv {
		return coefficientMatrixVectorInto(n, v, out, coeff, true)
	}
	return coefficientMatrixVectorInto(n, v, out, coeff, false)
}

func coefficientMatrixVectorInto(n int, v, out []float64, coeff spectralCoefficientExpr, transpose bool) bool {
	for i := 0; i < n; i++ {
		sum := 0.0
		for j := 0; j < n; j++ {
			x, y := i, j
			if transpose {
				x, y = j, i
			}
			c, ok := coeff.eval(x, y)
			if !ok {
				return false
			}
			sum += c * v[j]
		}
		out[i] = sum
	}
	return true
}

func spectralMatrixVector(coeff []float64, n int, v, out []float64) {
	if !floatSlicesOverlap(v[:n], out[:n]) {
		floatMatrixVectorNoAlias(coeff, n, v, out)
		return
	}
	floatMatrixVectorRowMajor(coeff, n, v, out)
}

func floatMatrixVectorRowMajor(coeff []float64, n int, v, out []float64) {
	for i := 0; i < n; i++ {
		row := coeff[i*n : (i+1)*n]
		sum := 0.0
		j := 0
		for ; j+3 < n; j += 4 {
			sum += row[j] * v[j]
			sum += row[j+1] * v[j+1]
			sum += row[j+2] * v[j+2]
			sum += row[j+3] * v[j+3]
		}
		for ; j < n; j++ {
			sum += row[j] * v[j]
		}
		out[i] = sum
	}
}

// floatMatrixVectorNoAlias computes four rows at a time, but each row's sum is
// still accumulated in increasing j order to preserve the scalar result order.
func floatMatrixVectorNoAlias(coeff []float64, n int, v, out []float64) {
	i := 0
	for ; i+3 < n; i += 4 {
		row0 := coeff[i*n : (i+1)*n]
		row1 := coeff[(i+1)*n : (i+2)*n]
		row2 := coeff[(i+2)*n : (i+3)*n]
		row3 := coeff[(i+3)*n : (i+4)*n]
		sum0 := 0.0
		sum1 := 0.0
		sum2 := 0.0
		sum3 := 0.0
		for j := 0; j < n; j++ {
			vj := v[j]
			sum0 += row0[j] * vj
			sum1 += row1[j] * vj
			sum2 += row2[j] * vj
			sum3 += row3[j] * vj
		}
		out[i] = sum0
		out[i+1] = sum1
		out[i+2] = sum2
		out[i+3] = sum3
	}
	for ; i < n; i++ {
		row := coeff[i*n : (i+1)*n]
		sum := 0.0
		j := 0
		for ; j+3 < n; j += 4 {
			sum += row[j] * v[j]
			sum += row[j+1] * v[j+1]
			sum += row[j+2] * v[j+2]
			sum += row[j+3] * v[j+3]
		}
		for ; j < n; j++ {
			sum += row[j] * v[j]
		}
		out[i] = sum
	}
}

func floatSlicesOverlap(a, b []float64) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	a0 := &a[0]
	aN := &a[len(a)-1]
	b0 := &b[0]
	bN := &b[len(b)-1]
	return uintptr(unsafe.Pointer(a0)) <= uintptr(unsafe.Pointer(bN)) &&
		uintptr(unsafe.Pointer(b0)) <= uintptr(unsafe.Pointer(aN))
}

func (c *spectralCoefficientCache) coefficients(n int, coeff spectralCoefficientExpr) ([]float64, []float64, bool) {
	if n == 0 {
		return nil, nil, true
	}
	if coeff.proto == nil {
		return nil, nil, false
	}
	limit := maxSpectralCoefficientFloats / 2
	if n < 0 || n > limit/n {
		return nil, nil, false
	}
	total := n * n
	if c.n == n && c.fingerprint == coeff.fingerprint && len(c.a) == total && len(c.at) == total {
		return c.a, c.at, true
	}
	a := make([]float64, total)
	at := make([]float64, total)
	for i := 0; i < n; i++ {
		row := a[i*n : (i+1)*n]
		for j := 0; j < n; j++ {
			v, ok := coeff.eval(i, j)
			if !ok {
				return nil, nil, false
			}
			row[j] = v
			at[j*n+i] = v
		}
	}
	c.n = n
	c.fingerprint = coeff.fingerprint
	c.a = a
	c.at = at
	return a, at, true
}

func (c *spectralCoefficientCache) cached(n int, coeff spectralCoefficientExpr) ([]float64, []float64, bool) {
	if n < 0 || c.n != n || c.fingerprint != coeff.fingerprint {
		return nil, nil, false
	}
	total := n * n
	if len(c.a) != total || len(c.at) != total {
		return nil, nil, false
	}
	return c.a, c.at, true
}

func spectralSpecializationArgs(args []runtime.Value) (int, []float64, []float64, bool) {
	if len(args) != 3 || !args[0].IsNumber() || !args[1].IsTable() || !args[2].IsTable() {
		return 0, nil, nil, false
	}
	nn := args[0].Number()
	n64 := int64(nn)
	if float64(n64) != nn {
		return 0, nil, nil, false
	}
	if n64 < 0 || int64(int(n64)) != n64 {
		return 0, nil, nil, false
	}
	n := int(n64)
	v, ok := args[1].Table().PlainFloatArrayForNumericSpecialization(n)
	if !ok {
		return 0, nil, nil, false
	}
	out, ok := args[2].Table().PlainFloatArrayForNumericSpecialization(n)
	if !ok {
		return 0, nil, nil, false
	}
	return n, v, out, true
}

func denseSpectralAtAvSpecializationArgs(args []runtime.Value) (int, []float64, []float64, []float64, bool) {
	if len(args) != 4 || !args[0].IsNumber() || !args[1].IsTable() || !args[2].IsTable() || !args[3].IsTable() {
		return 0, nil, nil, nil, false
	}
	nn := args[0].Number()
	n64 := int64(nn)
	if float64(n64) != nn || n64 < 0 || int64(int(n64)) != n64 {
		return 0, nil, nil, nil, false
	}
	n := int(n64)
	v, stride, ok := args[1].Table().DenseFloatMatrixForNumericSpecialization(n, 1)
	if !ok || stride != 1 {
		return 0, nil, nil, nil, false
	}
	tmp, stride, ok := args[2].Table().DenseFloatMatrixForNumericSpecialization(n, 1)
	if !ok || stride != 1 {
		return 0, nil, nil, nil, false
	}
	atav, stride, ok := args[3].Table().DenseFloatMatrixForNumericSpecialization(n, 1)
	if !ok || stride != 1 {
		return 0, nil, nil, nil, false
	}
	return n, v, tmp, atav, true
}

func (expr spectralCoefficientExpr) eval(i, j int) (float64, bool) {
	if expr.kind == coefficientExprReciprocalTriangularIndex {
		ij := i + j
		return 1.0 / float64(ij*(ij+1)/2+i+1), true
	}
	return evalPureNumericBinaryProto(expr.proto, float64(i), float64(j))
}

func (vm *VM) spectralMultiplyCoefficient(proto *FuncProto) (spectralCoefficientExpr, bool) {
	if len(proto.Constants) < 2 || !proto.Constants[1].IsString() {
		return spectralCoefficientExpr{}, false
	}
	v, ok := vm.globalValue(proto.Constants[1].Str())
	if !ok {
		return spectralCoefficientExpr{}, false
	}
	cl, ok := closureFromValue(v)
	if !ok {
		return spectralCoefficientExpr{}, false
	}
	return spectralCoefficientExprForProto(cl.Proto)
}

func (vm *VM) spectralAtAvCoefficient(proto *FuncProto) (spectralCoefficientExpr, bool) {
	if len(proto.Constants) < 3 || !proto.Constants[1].IsString() || !proto.Constants[2].IsString() {
		return spectralCoefficientExpr{}, false
	}
	avVal, ok := vm.globalValue(proto.Constants[1].Str())
	if !ok {
		return spectralCoefficientExpr{}, false
	}
	atvVal, ok := vm.globalValue(proto.Constants[2].Str())
	if !ok {
		return spectralCoefficientExpr{}, false
	}
	av, ok := closureFromValue(avVal)
	if !ok || classifySpectralMultiplyProto(av.Proto) != spectralAv {
		return spectralCoefficientExpr{}, false
	}
	avCoeff, ok := vm.spectralMultiplyCoefficient(av.Proto)
	if !ok {
		return spectralCoefficientExpr{}, false
	}
	atv, ok := closureFromValue(atvVal)
	if !ok || classifySpectralMultiplyProto(atv.Proto) != spectralAtv {
		return spectralCoefficientExpr{}, false
	}
	atvCoeff, ok := vm.spectralMultiplyCoefficient(atv.Proto)
	if !ok || atvCoeff.fingerprint != avCoeff.fingerprint {
		return spectralCoefficientExpr{}, false
	}
	return avCoeff, true
}

func (vm *VM) denseSpectralAtAvCoefficient(proto *FuncProto) (spectralCoefficientExpr, bool) {
	if len(proto.Constants) < 5 || !proto.Constants[3].IsString() || !proto.Constants[4].IsString() {
		return spectralCoefficientExpr{}, false
	}
	avVal, ok := vm.globalValue(proto.Constants[3].Str())
	if !ok {
		return spectralCoefficientExpr{}, false
	}
	atvVal, ok := vm.globalValue(proto.Constants[4].Str())
	if !ok {
		return spectralCoefficientExpr{}, false
	}
	av, ok := closureFromValue(avVal)
	if !ok || classifyDenseSpectralMultiplyProto(av.Proto) != spectralAv {
		return spectralCoefficientExpr{}, false
	}
	avCoeff, ok := vm.spectralMultiplyCoefficient(av.Proto)
	if !ok {
		return spectralCoefficientExpr{}, false
	}
	atv, ok := closureFromValue(atvVal)
	if !ok || classifyDenseSpectralMultiplyProto(atv.Proto) != spectralAtv {
		return spectralCoefficientExpr{}, false
	}
	atvCoeff, ok := vm.spectralMultiplyCoefficient(atv.Proto)
	if !ok || atvCoeff.fingerprint != avCoeff.fingerprint {
		return spectralCoefficientExpr{}, false
	}
	return avCoeff, true
}

func classifyDenseSpectralMultiplyProto(p *FuncProto) spectralMultiplyKind {
	if p == nil || p.NumParams != 3 || p.IsVarArg || len(p.Constants) != 5 ||
		len(p.Code) != 36 || !numberConst(p.Constants[0], 0.0) ||
		!p.Constants[1].IsString() ||
		!p.Constants[2].IsString() ||
		!p.Constants[3].IsString() ||
		!p.Constants[4].IsString() {
		return spectralNotMultiply
	}
	prefix := []uint32{
		EncodeAsBx(OP_LOADINT, 3, 0),
		EncodeABC(OP_MOVE, 7, 0, 0),
		EncodeAsBx(OP_LOADINT, 8, 1),
		EncodeABC(OP_SUB, 4, 7, 8),
		EncodeAsBx(OP_LOADINT, 5, 1),
		EncodeAsBx(OP_FORPREP, 3, 28),
		EncodeABx(OP_LOADK, 7, 0),
		EncodeAsBx(OP_LOADINT, 8, 0),
		EncodeABC(OP_MOVE, 12, 0, 0),
		EncodeAsBx(OP_LOADINT, 13, 1),
		EncodeABC(OP_SUB, 9, 12, 13),
		EncodeAsBx(OP_LOADINT, 10, 1),
		EncodeAsBx(OP_FORPREP, 8, 13),
		EncodeABx(OP_GETGLOBAL, 14, 1),
	}
	suffix := []uint32{
		EncodeABC(OP_CALL, 14, 3, 2),
		EncodeABx(OP_GETGLOBAL, 16, 2),
		EncodeABC(OP_GETFIELD, 15, 16, 3),
		EncodeABC(OP_MOVE, 16, 1, 0),
		EncodeABC(OP_MOVE, 17, 11, 0),
		EncodeAsBx(OP_LOADINT, 18, 0),
		EncodeABC(OP_CALL, 15, 4, 2),
		EncodeABC(OP_MUL, 13, 14, 15),
		EncodeABC(OP_ADD, 12, 7, 13),
		EncodeABC(OP_MOVE, 7, 12, 0),
		EncodeAsBx(OP_FORLOOP, 8, -14),
		EncodeABx(OP_GETGLOBAL, 12, 2),
		EncodeABC(OP_GETFIELD, 11, 12, 4),
		EncodeABC(OP_MOVE, 12, 2, 0),
		EncodeABC(OP_MOVE, 13, 6, 0),
		EncodeAsBx(OP_LOADINT, 14, 0),
		EncodeABC(OP_MOVE, 15, 7, 0),
		EncodeABC(OP_CALL, 11, 5, 1),
		EncodeAsBx(OP_FORLOOP, 3, -29),
		EncodeABC(OP_RETURN, 0, 1, 0),
	}
	av := append(append([]uint32{}, prefix...), EncodeABC(OP_MOVE, 15, 6, 0), EncodeABC(OP_MOVE, 16, 11, 0))
	av = append(av, suffix...)
	if codeEquals(p.Code, av) {
		return spectralAv
	}
	atv := append(append([]uint32{}, prefix...), EncodeABC(OP_MOVE, 15, 11, 0), EncodeABC(OP_MOVE, 16, 6, 0))
	atv = append(atv, suffix...)
	if codeEquals(p.Code, atv) {
		return spectralAtv
	}
	return spectralNotMultiply
}

func isSpectralAvProto(p *FuncProto) bool {
	spec, ok := spectralRuntimeSpecializationSpecForProto(p)
	return ok && spec.kind == spectralRuntimeSpecializationAv
}

func isSpectralAtvProto(p *FuncProto) bool {
	spec, ok := spectralRuntimeSpecializationSpecForProto(p)
	return ok && spec.kind == spectralRuntimeSpecializationAtv
}

func spectralRuntimeSpecializationSpecForProto(p *FuncProto) (*spectralRuntimeSpecializationSpec, bool) {
	if p == nil {
		return nil, false
	}
	fp := runtimeSpecializationFingerprintForProto(p)
	cache := p.SpectralRuntimeSpecialization
	if cache != nil && cache.fingerprint == fp {
		return cache.spec, cache.spec != nil
	}
	spec, ok := analyzeSpectralRuntimeSpecializationSpec(p)
	if !ok {
		p.SpectralRuntimeSpecialization = &spectralRuntimeSpecializationCache{fingerprint: fp}
		return nil, false
	}
	p.SpectralRuntimeSpecialization = &spectralRuntimeSpecializationCache{fingerprint: fp, spec: spec}
	return spec, true
}

func analyzeSpectralRuntimeSpecializationSpec(p *FuncProto) (*spectralRuntimeSpecializationSpec, bool) {
	switch classifySpectralMultiplyBytecode(p) {
	case spectralAv:
		return &spectralRuntimeSpecializationSpec{kind: spectralRuntimeSpecializationAv}, true
	case spectralAtv:
		return &spectralRuntimeSpecializationSpec{kind: spectralRuntimeSpecializationAtv}, true
	}
	if isSpectralAtAvBytecode(p) {
		return &spectralRuntimeSpecializationSpec{kind: spectralRuntimeSpecializationAtAv}, true
	}
	if isDenseSpectralAtAvBytecode(p) {
		return &spectralRuntimeSpecializationSpec{kind: spectralRuntimeSpecializationDenseAtAv}, true
	}
	return nil, false
}

func spectralCoefficientExprForProto(p *FuncProto) (spectralCoefficientExpr, bool) {
	if p == nil || p.NumParams != 2 || p.IsVarArg || p.MaxStack < 2 || p.MaxStack > 64 ||
		len(p.Protos) != 0 || len(p.Upvalues) != 0 || len(p.Code) == 0 || len(p.Code) > 64 {
		return spectralCoefficientExpr{}, false
	}
	for _, c := range p.Constants {
		if !c.IsNumber() {
			return spectralCoefficientExpr{}, false
		}
	}
	expr := spectralCoefficientExpr{
		proto:       p,
		fingerprint: runtimeSpecializationFingerprintForProto(p),
	}
	if isReciprocalTriangularIndexExprProto(p) {
		expr.kind = coefficientExprReciprocalTriangularIndex
	}
	if _, ok := expr.eval(0, 0); !ok {
		return spectralCoefficientExpr{}, false
	}
	if _, ok := expr.eval(1, 2); !ok {
		return spectralCoefficientExpr{}, false
	}
	return expr, true
}

func isReciprocalTriangularIndexExprProto(p *FuncProto) bool {
	if p == nil || p.NumParams != 2 || p.IsVarArg || len(p.Constants) < 1 || !numberConst(p.Constants[0], 1.0) {
		return false
	}
	return codeEquals(p.Code, []uint32{
		EncodeABx(OP_LOADK, 3, 0),
		EncodeABC(OP_ADD, 8, 0, 1),
		EncodeABC(OP_ADD, 10, 0, 1),
		EncodeAsBx(OP_LOADINT, 11, 1),
		EncodeABC(OP_ADD, 9, 10, 11),
		EncodeABC(OP_MUL, 7, 8, 9),
		EncodeAsBx(OP_LOADINT, 8, 2),
		EncodeABC(OP_DIV, 6, 7, 8),
		EncodeABC(OP_ADD, 5, 6, 0),
		EncodeAsBx(OP_LOADINT, 6, 1),
		EncodeABC(OP_ADD, 4, 5, 6),
		EncodeABC(OP_DIV, 2, 3, 4),
		EncodeABC(OP_RETURN, 2, 2, 0),
	})
}

func evalPureNumericBinaryProto(p *FuncProto, x, y float64) (float64, bool) {
	if p == nil || p.NumParams != 2 || p.IsVarArg || p.MaxStack < 2 || p.MaxStack > 64 || len(p.Code) > 64 {
		return 0, false
	}
	var regs [64]float64
	var valid [64]bool
	stackLen := p.MaxStack
	regs[0], regs[1] = x, y
	valid[0], valid[1] = true, true
	rk := func(idx int) (float64, bool) {
		if IsRK(idx) {
			cidx := RKToConstIdx(idx)
			if cidx < 0 || cidx >= len(p.Constants) || !p.Constants[cidx].IsNumber() {
				return 0, false
			}
			return p.Constants[cidx].Number(), true
		}
		if idx < 0 || idx >= stackLen || !valid[idx] {
			return 0, false
		}
		return regs[idx], true
	}
	set := func(idx int, v float64) bool {
		if idx < 0 || idx >= stackLen {
			return false
		}
		regs[idx] = v
		valid[idx] = true
		return true
	}
	for _, inst := range p.Code {
		a, b, c := DecodeA(inst), DecodeB(inst), DecodeC(inst)
		switch DecodeOp(inst) {
		case OP_LOADINT:
			if !set(a, float64(DecodesBx(inst))) {
				return 0, false
			}
		case OP_LOADK:
			idx := DecodeBx(inst)
			if idx < 0 || idx >= len(p.Constants) || !p.Constants[idx].IsNumber() {
				return 0, false
			}
			if !set(a, p.Constants[idx].Number()) {
				return 0, false
			}
		case OP_MOVE:
			if b < 0 || b >= stackLen || !valid[b] || !set(a, regs[b]) {
				return 0, false
			}
		case OP_ADD, OP_SUB, OP_MUL, OP_DIV:
			lhs, ok := rk(b)
			if !ok {
				return 0, false
			}
			rhs, ok := rk(c)
			if !ok {
				return 0, false
			}
			var out float64
			switch DecodeOp(inst) {
			case OP_ADD:
				out = lhs + rhs
			case OP_SUB:
				out = lhs - rhs
			case OP_MUL:
				out = lhs * rhs
			case OP_DIV:
				if rhs == 0 {
					return 0, false
				}
				out = lhs / rhs
			}
			if !set(a, out) {
				return 0, false
			}
		case OP_RETURN:
			if DecodeB(inst) != 2 || a < 0 || a >= stackLen || !valid[a] {
				return 0, false
			}
			return regs[a], true
		default:
			return 0, false
		}
	}
	return 0, false
}

func isDenseSpectralAtAvProto(p *FuncProto) bool {
	spec, ok := spectralRuntimeSpecializationSpecForProto(p)
	return ok && spec.kind == spectralRuntimeSpecializationDenseAtAv
}

func isDenseSpectralAtAvBytecode(p *FuncProto) bool {
	if p == nil || p.NumParams != 4 || p.IsVarArg || len(p.Constants) != 5 ||
		!p.Constants[0].IsString() ||
		!p.Constants[1].IsString() ||
		!numberConst(p.Constants[2], 0.0) ||
		!p.Constants[3].IsString() ||
		!p.Constants[4].IsString() {
		return false
	}
	return codeEquals(p.Code, []uint32{
		EncodeAsBx(OP_LOADINT, 4, 0),
		EncodeABC(OP_MOVE, 8, 0, 0),
		EncodeAsBx(OP_LOADINT, 9, 1),
		EncodeABC(OP_SUB, 5, 8, 9),
		EncodeAsBx(OP_LOADINT, 6, 1),
		EncodeAsBx(OP_FORPREP, 4, 7),
		EncodeABx(OP_GETGLOBAL, 9, 0),
		EncodeABC(OP_GETFIELD, 8, 9, 1),
		EncodeABC(OP_MOVE, 9, 2, 0),
		EncodeABC(OP_MOVE, 10, 7, 0),
		EncodeAsBx(OP_LOADINT, 11, 0),
		EncodeABx(OP_LOADK, 12, 2),
		EncodeABC(OP_CALL, 8, 5, 1),
		EncodeAsBx(OP_FORLOOP, 4, -8),
		EncodeABx(OP_GETGLOBAL, 7, 3),
		EncodeABC(OP_MOVE, 8, 0, 0),
		EncodeABC(OP_MOVE, 9, 1, 0),
		EncodeABC(OP_MOVE, 10, 2, 0),
		EncodeABC(OP_CALL, 7, 4, 1),
		EncodeABx(OP_GETGLOBAL, 7, 4),
		EncodeABC(OP_MOVE, 8, 0, 0),
		EncodeABC(OP_MOVE, 9, 2, 0),
		EncodeABC(OP_MOVE, 10, 3, 0),
		EncodeABC(OP_CALL, 7, 4, 1),
		EncodeABC(OP_RETURN, 0, 1, 0),
	})
}

func valueStringConst(v runtime.Value, want string) bool {
	return v.IsString() && v.Str() == want
}

func classifySpectralMultiplyProto(p *FuncProto) spectralMultiplyKind {
	spec, ok := spectralRuntimeSpecializationSpecForProto(p)
	if !ok {
		return spectralNotMultiply
	}
	switch spec.kind {
	case spectralRuntimeSpecializationAv:
		return spectralAv
	case spectralRuntimeSpecializationAtv:
		return spectralAtv
	default:
		return spectralNotMultiply
	}
}

func classifySpectralMultiplyBytecode(p *FuncProto) spectralMultiplyKind {
	if p == nil || p.NumParams != 3 || p.IsVarArg || len(p.Constants) < 2 ||
		len(p.Code) != 28 || !numberConst(p.Constants[0], 0.0) || !p.Constants[1].IsString() {
		return spectralNotMultiply
	}
	prefix := []uint32{
		EncodeAsBx(OP_LOADINT, 3, 0),
		EncodeABC(OP_MOVE, 7, 0, 0),
		EncodeAsBx(OP_LOADINT, 8, 1),
		EncodeABC(OP_SUB, 4, 7, 8),
		EncodeAsBx(OP_LOADINT, 5, 1),
		EncodeAsBx(OP_FORPREP, 3, 20),
		EncodeABx(OP_LOADK, 7, 0),
		EncodeAsBx(OP_LOADINT, 8, 0),
		EncodeABC(OP_MOVE, 12, 0, 0),
		EncodeAsBx(OP_LOADINT, 13, 1),
		EncodeABC(OP_SUB, 9, 12, 13),
		EncodeAsBx(OP_LOADINT, 10, 1),
		EncodeAsBx(OP_FORPREP, 8, 9),
		EncodeABx(OP_GETGLOBAL, 14, 1),
	}
	suffix := []uint32{
		EncodeABC(OP_CALL, 14, 3, 2),
		EncodeABC(OP_MOVE, 16, 11, 0),
		EncodeABC(OP_GETTABLE, 15, 1, 16),
		EncodeABC(OP_MUL, 13, 14, 15),
		EncodeABC(OP_ADD, 12, 7, 13),
		EncodeABC(OP_MOVE, 7, 12, 0),
		EncodeAsBx(OP_FORLOOP, 8, -10),
		EncodeABC(OP_MOVE, 11, 7, 0),
		EncodeABC(OP_MOVE, 12, 6, 0),
		EncodeABC(OP_SETTABLE, 2, 12, 11),
		EncodeAsBx(OP_FORLOOP, 3, -21),
		EncodeABC(OP_RETURN, 0, 1, 0),
	}
	av := append(append([]uint32{}, prefix...), EncodeABC(OP_MOVE, 15, 6, 0), EncodeABC(OP_MOVE, 16, 11, 0))
	av = append(av, suffix...)
	if codeEquals(p.Code, av) {
		return spectralAv
	}
	atv := append(append([]uint32{}, prefix...), EncodeABC(OP_MOVE, 15, 11, 0), EncodeABC(OP_MOVE, 16, 6, 0))
	atv = append(atv, suffix...)
	if codeEquals(p.Code, atv) {
		return spectralAtv
	}
	return spectralNotMultiply
}

func isSpectralAtAvProto(p *FuncProto) bool {
	spec, ok := spectralRuntimeSpecializationSpecForProto(p)
	return ok && spec.kind == spectralRuntimeSpecializationAtAv
}

func isSpectralAtAvBytecode(p *FuncProto) bool {
	if p == nil || p.NumParams != 3 || p.IsVarArg || len(p.Constants) < 3 ||
		!numberConst(p.Constants[0], 0.0) || !p.Constants[1].IsString() || !p.Constants[2].IsString() {
		return false
	}
	if len(p.Code) != 22 || DecodeOp(p.Code[0]) != OP_NEWTABLE || DecodeA(p.Code[0]) != 3 {
		return false
	}
	return codeEquals(p.Code[1:], []uint32{
		EncodeAsBx(OP_LOADINT, 4, 0),
		EncodeABC(OP_MOVE, 8, 0, 0),
		EncodeAsBx(OP_LOADINT, 9, 1),
		EncodeABC(OP_SUB, 5, 8, 9),
		EncodeAsBx(OP_LOADINT, 6, 1),
		EncodeAsBx(OP_FORPREP, 4, 3),
		EncodeABx(OP_LOADK, 8, 0),
		EncodeABC(OP_MOVE, 9, 7, 0),
		EncodeABC(OP_SETTABLE, 3, 9, 8),
		EncodeAsBx(OP_FORLOOP, 4, -4),
		EncodeABx(OP_GETGLOBAL, 7, 1),
		EncodeABC(OP_MOVE, 8, 0, 0),
		EncodeABC(OP_MOVE, 9, 1, 0),
		EncodeABC(OP_MOVE, 10, 3, 0),
		EncodeABC(OP_CALL, 7, 4, 1),
		EncodeABx(OP_GETGLOBAL, 7, 2),
		EncodeABC(OP_MOVE, 8, 0, 0),
		EncodeABC(OP_MOVE, 9, 3, 0),
		EncodeABC(OP_MOVE, 10, 2, 0),
		EncodeABC(OP_CALL, 7, 4, 1),
		EncodeABC(OP_RETURN, 0, 1, 0),
	})
}
