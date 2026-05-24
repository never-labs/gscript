package vm

import (
	"strconv"

	"github.com/gscript/gscript/internal/runtime"
)

type mixedAffineTableBuilderSpec struct {
	arrayScale int64
	arrayBias  int64
	strDiv     int64
	strScale   int64
	strBias    int64
	strPrefix  string
	negDiv     int64
	negScale   int64
	negBias    int64
}

func isMixedAffineTableBuilderProto(p *FuncProto) bool {
	_, ok := mixedAffineTableBuilderSpecForProto(p)
	return ok
}

func mixedAffineTableBuilderSpecForProto(p *FuncProto) (mixedAffineTableBuilderSpec, bool) {
	var spec mixedAffineTableBuilderSpec
	if p == nil || p.NumParams != 1 || p.UsesVarargBytecode || len(p.Code) != 39 ||
		len(p.Constants) != 1 || !p.Constants[0].IsString() {
		return spec, false
	}
	code := p.Code
	if DecodeOp(code[0]) != OP_NEWTABLE ||
		DecodeOp(code[1]) != OP_LOADINT || DecodeA(code[1]) != 2 || DecodesBx(code[1]) != 1 ||
		DecodeOp(code[2]) != OP_MOVE || DecodeA(code[2]) != 3 || DecodeB(code[2]) != 0 ||
		DecodeOp(code[3]) != OP_LOADINT || DecodeA(code[3]) != 4 || DecodesBx(code[3]) != 1 ||
		DecodeOp(code[4]) != OP_FORPREP ||
		DecodeOp(code[5]) != OP_LOADINT ||
		DecodeOp(code[6]) != OP_MUL ||
		DecodeOp(code[7]) != OP_LOADINT ||
		DecodeOp(code[8]) != OP_ADD ||
		DecodeOp(code[10]) != OP_SETTABLE ||
		DecodeOp(code[11]) != OP_LOADINT ||
		DecodeOp(code[12]) != OP_MOD ||
		DecodeOp(code[13]) != OP_LOADINT || DecodesBx(code[13]) != 0 ||
		DecodeOp(code[14]) != OP_EQ ||
		DecodeOp(code[16]) != OP_LOADINT ||
		DecodeOp(code[17]) != OP_MUL ||
		DecodeOp(code[18]) != OP_LOADINT ||
		DecodeOp(code[19]) != OP_ADD ||
		DecodeOp(code[20]) != OP_LOADK || DecodeBx(code[20]) != 0 ||
		DecodeOp(code[22]) != OP_CONCAT ||
		DecodeOp(code[23]) != OP_SETTABLE ||
		DecodeOp(code[24]) != OP_LOADINT ||
		DecodeOp(code[25]) != OP_MOD ||
		DecodeOp(code[26]) != OP_LOADINT || DecodesBx(code[26]) != 0 ||
		DecodeOp(code[27]) != OP_EQ ||
		DecodeOp(code[29]) != OP_LOADINT ||
		DecodeOp(code[30]) != OP_MUL ||
		DecodeOp(code[31]) != OP_LOADINT ||
		DecodeOp(code[32]) != OP_ADD ||
		DecodeOp(code[34]) != OP_UNM ||
		DecodeOp(code[35]) != OP_SETTABLE ||
		DecodeOp(code[36]) != OP_FORLOOP ||
		DecodeOp(code[37]) != OP_MOVE ||
		DecodeOp(code[38]) != OP_RETURN {
		return spec, false
	}
	spec = mixedAffineTableBuilderSpec{
		arrayScale: int64(DecodesBx(code[5])),
		arrayBias:  int64(DecodesBx(code[7])),
		strDiv:     int64(DecodesBx(code[11])),
		strScale:   int64(DecodesBx(code[16])),
		strBias:    int64(DecodesBx(code[18])),
		strPrefix:  p.Constants[0].Str(),
		negDiv:     int64(DecodesBx(code[24])),
		negScale:   int64(DecodesBx(code[29])),
		negBias:    int64(DecodesBx(code[31])),
	}
	if spec.strDiv == 0 || spec.negDiv == 0 {
		return mixedAffineTableBuilderSpec{}, false
	}
	return spec, true
}

func (vm *VM) runMixedAffineTableBuilderRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 1 || args[0].RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	spec, ok := mixedAffineTableBuilderSpecForProto(cl.Proto)
	if !ok {
		return false, nil, nil
	}
	n := args[0].RawInt()
	if n < 1 {
		return true, []runtime.Value{runtime.TableValue(runtime.NewTable())}, nil
	}
	if n > int64(int(^uint(0)>>1)-1) {
		return false, nil, nil
	}
	hashHint := int(n/spec.strDiv + n/spec.negDiv + 2)
	tbl := runtime.NewTableSizedKind(int(n), hashHint, runtime.ArrayInt)
	for i := int64(1); i <= n; i++ {
		tbl.RawSetInt(i, runtime.IntValue(i*spec.arrayScale+spec.arrayBias))
		if i%spec.strDiv == 0 {
			tbl.RawSetString(spec.strPrefix+strconv.FormatInt(i, 10), runtime.IntValue(i*spec.strScale+spec.strBias))
		}
		if i%spec.negDiv == 0 {
			tbl.RawSetInt(-i, runtime.IntValue(i*spec.negScale+spec.negBias))
		}
	}
	return true, []runtime.Value{runtime.TableValue(tbl)}, nil
}
