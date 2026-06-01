package vm

import (
	"strconv"

	"github.com/never-labs/leia/internal/runtime"
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
	pat := newBytecodePattern(code)
	if !pat.hasOps(
		opcodeAt{pc: 0, op: OP_NEWTABLE},
		opcodeAt{pc: 4, op: OP_FORPREP},
		opcodeAt{pc: 5, op: OP_LOADINT},
		opcodeAt{pc: 6, op: OP_MUL},
		opcodeAt{pc: 7, op: OP_LOADINT},
		opcodeAt{pc: 8, op: OP_ADD},
		opcodeAt{pc: 10, op: OP_SETTABLE},
		opcodeAt{pc: 11, op: OP_LOADINT},
		opcodeAt{pc: 12, op: OP_MOD},
		opcodeAt{pc: 14, op: OP_EQ},
		opcodeAt{pc: 16, op: OP_LOADINT},
		opcodeAt{pc: 17, op: OP_MUL},
		opcodeAt{pc: 18, op: OP_LOADINT},
		opcodeAt{pc: 19, op: OP_ADD},
		opcodeAt{pc: 22, op: OP_CONCAT},
		opcodeAt{pc: 23, op: OP_SETTABLE},
		opcodeAt{pc: 24, op: OP_LOADINT},
		opcodeAt{pc: 25, op: OP_MOD},
		opcodeAt{pc: 27, op: OP_EQ},
		opcodeAt{pc: 29, op: OP_LOADINT},
		opcodeAt{pc: 30, op: OP_MUL},
		opcodeAt{pc: 31, op: OP_LOADINT},
		opcodeAt{pc: 32, op: OP_ADD},
		opcodeAt{pc: 34, op: OP_UNM},
		opcodeAt{pc: 35, op: OP_SETTABLE},
		opcodeAt{pc: 36, op: OP_FORLOOP},
		opcodeAt{pc: 37, op: OP_MOVE},
		opcodeAt{pc: 38, op: OP_RETURN},
	) ||
		!pat.hasASBxs(
			asbxAt{pc: 1, op: OP_LOADINT, a: 2, sbx: 1},
			asbxAt{pc: 3, op: OP_LOADINT, a: 4, sbx: 1},
		) ||
		!pat.hasABs(abAt{pc: 2, op: OP_MOVE, a: 3, b: 0}) ||
		!pat.hasBxs(bxAt{pc: 20, op: OP_LOADK, bx: 0}) ||
		!pat.hasSBxs(
			sbxAt{pc: 13, op: OP_LOADINT, sbx: 0},
			sbxAt{pc: 26, op: OP_LOADINT, sbx: 0},
		) {
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
	strHint := int(n / spec.strDiv)
	intMapHint := int(n / spec.negDiv)
	tbl := runtime.NewPlainIntArrayMapTable(int(n), intMapHint, strHint)
	for i := int64(1); i <= n; i++ {
		if !tbl.InitIntArraySlot(i, i*spec.arrayScale+spec.arrayBias) {
			return false, nil, nil
		}
		if i%spec.strDiv == 0 {
			tbl.InitStringMapSlot(spec.strPrefix+strconv.FormatInt(i, 10), runtime.IntValue(i*spec.strScale+spec.strBias))
		}
		if i%spec.negDiv == 0 {
			tbl.InitIntMapSlot(-i, runtime.IntValue(i*spec.negScale+spec.negBias))
		}
	}
	return true, []runtime.Value{runtime.TableValue(tbl)}, nil
}
