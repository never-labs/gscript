package methodjit

import (
	"fmt"
	"sort"
	"unsafe"

	"github.com/never-labs/gscript/internal/runtime"
)

// StableFieldCalleeGuardPass replaces repeated in-loop field callee guards with
// one entry guard when runtime feedback proves the shape field has held the
// same VM closure identity. The rewrite is structural: it keys on shape, field
// index and closure pointer, never on function or benchmark names.
func StableFieldCalleeGuardPass(fn *Function) (*Function, error) {
	return StableFieldCalleeGuardPassWith(nil)(fn)
}

var stableFieldCalleeGuardPassAllowedDomains = allowedDomainsForModule(
	analysisFacts(AnalysisFactFieldPolyShapeFacts),
	nil,
	nil,
)

func StableFieldCalleeGuardPassWith(registry *CompilationDependencyRegistry) PassFunc {
	return func(fn *Function) (*Function, error) {
		return StableFieldCalleeGuardPassCtx(registry)(newPassContext(fn, nil, stableFieldCalleeGuardPassAllowedDomains, false))
	}
}

func StableFieldCalleeGuardPassCtx(registry *CompilationDependencyRegistry) func(*PassContext) (*Function, error) {
	return func(ctx *PassContext) (*Function, error) {
		if ctx == nil {
			return nil, nil
		}
		return stableFieldCalleeGuardPass(ctx.Func(), ctx.TableShape(), registry)
	}
}

func stableFieldCalleeGuardPass(fn *Function, tableShapes *TableShapeFacts, registry *CompilationDependencyRegistry) (*Function, error) {
	if fn == nil || len(fn.Blocks) == 0 {
		return fn, nil
	}
	fn.ensureAnalysis()
	uses := computeUseCounts(fn)
	mutatedFields, hasUnknownStringFieldMutation := stableFieldCalleeMutations(fn)
	type guardKey struct {
		shapeID  uint32
		fieldIdx int
		closure  uintptr
	}
	needed := make(map[guardKey]bool)
	changed := false
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		out := block.Instrs[:0]
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpGuardFieldCalleeProto || uses[instr.ID] != 0 {
				out = append(out, instr)
				continue
			}
			shapeID := uint32(instr.Aux2 >> 32)
			fieldIdx := int(int32(instr.Aux2 & 0xFFFFFFFF))
			if hasUnknownStringFieldMutation || stableFieldCalleeFieldMayMutate(mutatedFields, shapeID, fieldIdx) {
				out = append(out, instr)
				functionRemarks(fn).Add("StableFieldCalleeGuard", "missed", block.ID, instr.ID, instr.Op,
					fmt.Sprintf("shape=%d field=%d may be written in this function", shapeID, fieldIdx))
				continue
			}
			closure := stableFieldCalleeExactClosure(fn, tableShapes, instr, shapeID, fieldIdx)
			if closure == 0 || !runtimeShapeFieldClosureStable(shapeID, fieldIdx, closure) {
				out = append(out, instr)
				continue
			}
			needed[guardKey{shapeID: shapeID, fieldIdx: fieldIdx, closure: closure}] = true
			if registry != nil {
				registry.RecordShapeField(shapeID, fieldIdx)
			}
			changed = true
			functionRemarks(fn).Add("StableFieldCalleeGuard", "changed", block.ID, instr.ID, instr.Op,
				fmt.Sprintf("removed repeated field callee guard for shape=%d field=%d", shapeID, fieldIdx))
		}
		block.Instrs = out
	}
	if !changed {
		return fn, nil
	}
	entry := fn.Entry
	if entry == nil {
		entry = fn.Blocks[0]
	}
	keys := make([]guardKey, 0, len(needed))
	for k := range needed {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].shapeID != keys[j].shapeID {
			return keys[i].shapeID < keys[j].shapeID
		}
		if keys[i].fieldIdx != keys[j].fieldIdx {
			return keys[i].fieldIdx < keys[j].fieldIdx
		}
		return keys[i].closure < keys[j].closure
	})
	for _, k := range keys {
		guard := &Instr{
			ID:    fn.newValueID(),
			Op:    OpGuardShapeFieldVMClosure,
			Type:  TypeBool,
			Aux:   int64(k.shapeID)<<32 | int64(uint32(k.fieldIdx)),
			Aux2:  int64(k.closure),
			Block: entry,
		}
		insertBeforeTerminator(entry, guard)
		functionRemarks(fn).Add("StableFieldCalleeGuard", "changed", entry.ID, guard.ID, guard.Op,
			fmt.Sprintf("guard stable VM closure for shape=%d field=%d", k.shapeID, k.fieldIdx))
	}
	return fn, nil
}

func stableFieldCalleeMutations(fn *Function) (map[uint32]map[int]bool, bool) {
	mutated := make(map[uint32]map[int]bool)
	if fn == nil {
		return mutated, false
	}
	unknownStringFieldMutation := false
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			switch instr.Op {
			case OpFieldStore:
				if len(instr.Args) == 0 || instr.Args[0] == nil || instr.Args[0].Def == nil {
					unknownStringFieldMutation = true
					continue
				}
				svals := instr.Args[0].Def
				if svals.Op != OpFieldSvals || svals.Aux == 0 || instr.Aux < 0 {
					unknownStringFieldMutation = true
					continue
				}
				shapeID := uint32(svals.Aux)
				fieldIdx := int(instr.Aux)
				if mutated[shapeID] == nil {
					mutated[shapeID] = make(map[int]bool)
				}
				mutated[shapeID][fieldIdx] = true
			case OpSetField, OpSetTable:
				unknownStringFieldMutation = true
			}
		}
	}
	return mutated, unknownStringFieldMutation
}

func stableFieldCalleeFieldMayMutate(mutated map[uint32]map[int]bool, shapeID uint32, fieldIdx int) bool {
	if shapeID == 0 || fieldIdx < 0 {
		return true
	}
	return mutated[shapeID] != nil && mutated[shapeID][fieldIdx]
}

func runtimeShapeFieldClosureStable(shapeID uint32, fieldIdx int, closure uintptr) bool {
	got, stable := runtime.ShapeFieldStableVMClosure(shapeID, fieldIdx)
	return stable && got == closure && runtime.ShapeFieldVMClosureEpochPtr(shapeID, fieldIdx) != nil
}

func stableFieldCalleeExactClosure(fn *Function, tableShapes *TableShapeFacts, instr *Instr, shapeID uint32, fieldIdx int) uintptr {
	if fn == nil || instr == nil || shapeID == 0 || fieldIdx < 0 || instr.Aux == 0 {
		return 0
	}
	if tableShapes == nil {
		return 0
	}
	cases, _ := tableShapes.FieldPolyShapeCases(instr.ID)
	if len(cases) != 1 {
		return 0
	}
	c := cases[0]
	if c.ShapeID != shapeID || c.FieldIdx != fieldIdx || c.VMClosure == 0 || c.VMProto == nil {
		return 0
	}
	if uintptr(instr.Aux) != uintptr(unsafe.Pointer(c.VMProto)) {
		return 0
	}
	return c.VMClosure
}
