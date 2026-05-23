// validator.go checks structural invariants on the CFG SSA IR.
// Run Validate(fn) after every compilation pass to catch bugs immediately.
// Returns a list of errors — empty list means the IR is well-formed.
// This is a safety net: if a pass produces invalid IR, the validator
// catches it before it causes mysterious failures downstream.
//
// Invariants checked:
//   - Entry block exists and is in Blocks
//   - All blocks terminated (last instr is terminator, no terminator in middle)
//   - Succ/Pred consistency (bidirectional edges match)
//   - Terminator successor counts (Branch=2, Jump=1, Return=0)
//   - Branch arg count (exactly 1 condition)
//   - Unique value IDs and block IDs
//   - No nil successors or predecessors
//   - Reachability from entry (warning for orphan blocks)

package methodjit

import (
	"fmt"
)

const opArgMaxAny = -1

// validatorArgPolicy describes the SSA argument-count contract for an IR op.
// A zero-value policy means the validator does not enforce argument count.
type validatorArgPolicy struct {
	Min int
	Max int
	Set bool
}

func fixedOpArgs(n int) validatorArgPolicy {
	return validatorArgPolicy{Min: n, Max: n, Set: true}
}

func rangedOpArgs(min, max int) validatorArgPolicy {
	return validatorArgPolicy{Min: min, Max: max, Set: true}
}

func (p validatorArgPolicy) accepts(got int) bool {
	if !p.Set {
		return true
	}
	if got < p.Min {
		return false
	}
	return p.Max == opArgMaxAny || got <= p.Max
}

func (p validatorArgPolicy) describe() string {
	if p.Max == opArgMaxAny {
		return fmt.Sprintf("at least %d", p.Min)
	}
	return fmt.Sprintf("%d..%d", p.Min, p.Max)
}

type validatorOpContract struct {
	Args       validatorArgPolicy
	Terminator bool
	SuccCount  int // opArgMaxAny means successor count is not checked.
}

func validatorContractForOp(op Op) validatorOpContract {
	contract := validatorOpContract{SuccCount: opArgMaxAny}
	if spec, ok := op.Spec(); ok {
		contract.Terminator = spec.Terminator
	}
	if int(op) >= 0 && int(op) < len(validatorOpContracts) {
		contract.Args = validatorOpContracts[op].Args
		if validatorOpContracts[op].SuccCount != opArgMaxAny {
			contract.SuccCount = validatorOpContracts[op].SuccCount
		}
	}
	return contract
}

var validatorOpContracts = func() [OpMax]validatorOpContract {
	var contracts [OpMax]validatorOpContract
	for op := range contracts {
		contracts[op].SuccCount = opArgMaxAny
	}

	contracts[OpJump] = validatorOpContract{Args: fixedOpArgs(0), SuccCount: 1}
	contracts[OpBranch] = validatorOpContract{Args: fixedOpArgs(1), SuccCount: 2}
	contracts[OpReturn] = validatorOpContract{Args: rangedOpArgs(0, opArgMaxAny), SuccCount: 0}

	contracts[OpSetTable].Args = fixedOpArgs(3)
	contracts[OpTableArrayStore].Args = rangedOpArgs(5, 6)
	contracts[OpTableShapeID].Args = fixedOpArgs(1)
	contracts[OpTableArraySwapPairs].Args = fixedOpArgs(3)
	contracts[OpGuardType].Args = fixedOpArgs(1)
	contracts[OpGuardGlobalConst].Args = fixedOpArgs(0)
	contracts[OpGuardConstString].Args = fixedOpArgs(1)
	contracts[OpGuardTableKind].Args = fixedOpArgs(1)
	contracts[OpGuardCalleeProto].Args = fixedOpArgs(1)
	contracts[OpGuardNonNil].Args = fixedOpArgs(1)
	contracts[OpGuardTruthy].Args = fixedOpArgs(1)
	contracts[OpGuardFieldCalleeProto].Args = fixedOpArgs(1)
	contracts[OpGuardShapeFieldType].Args = fixedOpArgs(0)
	contracts[OpGuardShapeFieldTypeMask].Args = fixedOpArgs(0)
	contracts[OpFieldSvals].Args = fixedOpArgs(1)
	contracts[OpFieldLoad].Args = fixedOpArgs(1)
	contracts[OpFieldLoadNumToFloat].Args = fixedOpArgs(1)
	contracts[OpFieldStore].Args = fixedOpArgs(2)
	contracts[OpFieldPolyLen].Args = fixedOpArgs(1)
	contracts[OpRecordArrayLoopKernel].Args = rangedOpArgs(3, 16)

	return contracts
}()

// Validate checks all structural invariants of a Function's IR.
// Returns nil if the IR is well-formed, or a list of errors describing violations.
func Validate(fn *Function) []error {
	if fn == nil {
		return []error{fmt.Errorf("function is nil")}
	}
	v := &validator{fn: fn}
	v.run()
	if len(v.errs) == 0 {
		return nil
	}
	return v.errs
}

// VerifyIRLightweight checks cheap structural invariants suitable for optional
// per-module optimizer verification. It intentionally avoids whole-graph
// proofs such as reachability, dominance, use-def validation, and op-specific
// contracts covered by Validate.
func VerifyIRLightweight(fn *Function) error {
	v := &lightweightVerifier{fn: fn}
	v.run()
	if len(v.errs) == 0 {
		return nil
	}
	if len(v.errs) == 1 {
		return v.errs[0]
	}
	return fmt.Errorf("%v (and %d more)", v.errs[0], len(v.errs)-1)
}

type lightweightVerifier struct {
	fn     *Function
	errs   []error
	blocks map[*Block]bool
}

func (v *lightweightVerifier) errorf(format string, args ...interface{}) {
	v.errs = append(v.errs, fmt.Errorf(format, args...))
}

func (v *lightweightVerifier) run() {
	if v.fn == nil {
		v.errorf("function is nil")
		return
	}
	if v.fn.Entry == nil {
		v.errorf("entry block is nil")
	}
	if len(v.fn.Blocks) == 0 {
		v.errorf("function has no blocks")
		return
	}

	v.blocks = make(map[*Block]bool, len(v.fn.Blocks))
	blockIDs := make(map[int]*Block, len(v.fn.Blocks))
	entryInBlocks := false
	for i, blk := range v.fn.Blocks {
		if blk == nil {
			v.errorf("nil block at index %d", i)
			continue
		}
		if blk == v.fn.Entry {
			entryInBlocks = true
		}
		if v.blocks[blk] {
			v.errorf("duplicate block pointer B%d", blk.ID)
		}
		v.blocks[blk] = true
		if prev := blockIDs[blk.ID]; prev != nil && prev != blk {
			v.errorf("duplicate block ID %d", blk.ID)
		}
		blockIDs[blk.ID] = blk
	}
	if v.fn.Entry != nil && !entryInBlocks {
		v.errorf("entry block B%d is not in fn.Blocks", v.fn.Entry.ID)
	}

	for _, blk := range v.fn.Blocks {
		if blk == nil {
			continue
		}
		v.checkInstrs(blk)
		v.checkEdges(blk)
	}
}

func (v *lightweightVerifier) checkInstrs(blk *Block) {
	for i, instr := range blk.Instrs {
		if instr == nil {
			v.errorf("B%d: nil instruction at index %d", blk.ID, i)
			continue
		}
		if instr.Block != nil && instr.Block != blk {
			v.errorf("B%d: instruction v%d has owner B%d", blk.ID, instr.ID, instr.Block.ID)
		}
		for argIdx, arg := range instr.Args {
			if arg == nil {
				v.errorf("B%d: %s (v%d) has nil arg at index %d", blk.ID, instr.Op, instr.ID, argIdx)
				continue
			}
			if arg.Def != nil && arg.Def.Block != nil && !v.blocks[arg.Def.Block] {
				v.errorf("B%d: %s (v%d) arg %d defined in block outside function", blk.ID, instr.Op, instr.ID, argIdx)
			}
		}
	}
}

func (v *lightweightVerifier) checkEdges(blk *Block) {
	for i, succ := range blk.Succs {
		if succ == nil {
			v.errorf("B%d: nil successor at index %d", blk.ID, i)
			continue
		}
		if !v.blocks[succ] {
			v.errorf("B%d: successor B%d is not in fn.Blocks", blk.ID, succ.ID)
			continue
		}
		if !containsBlock(succ.Preds, blk) {
			v.errorf("B%d in Succs of B%d, but B%d not in Preds of B%d", succ.ID, blk.ID, blk.ID, succ.ID)
		}
	}
	for i, pred := range blk.Preds {
		if pred == nil {
			v.errorf("B%d: nil predecessor at index %d", blk.ID, i)
			continue
		}
		if !v.blocks[pred] {
			v.errorf("B%d: predecessor B%d is not in fn.Blocks", blk.ID, pred.ID)
			continue
		}
		if !containsBlock(pred.Succs, blk) {
			v.errorf("B%d in Preds of B%d, but B%d not in Succs of B%d", pred.ID, blk.ID, blk.ID, pred.ID)
		}
	}
}

// validator holds state for a single validation pass.
type validator struct {
	fn   *Function
	errs []error
}

func (v *validator) errorf(format string, args ...interface{}) {
	v.errs = append(v.errs, fmt.Errorf(format, args...))
}

func (v *validator) run() {
	// 1. Entry block exists.
	if v.fn.Entry == nil {
		v.errorf("entry block is nil")
		return // can't check anything else without an entry
	}

	// 2. Entry is in Blocks.
	if !v.checkEntryInBlocks() {
		return // block list is inconsistent, further checks unreliable
	}

	// 3. Block ID uniqueness.
	v.checkBlockIDUniqueness()

	// 4. No nil successors or predecessors.
	v.checkNilEdges()

	// 5. All blocks terminated + no terminator in middle.
	v.checkTerminators()

	// 6. OpSpec argument and successor counts.
	v.checkOpSpecs()

	// 7. Succ/Pred consistency.
	v.checkSuccPredConsistency()

	// 8. Safety-critical operation contracts.
	v.checkOpContracts()

	// 9. Unique value IDs.
	v.checkValueIDUniqueness()

	// 10. Reachability.
	v.checkReachability()
}

// checkEntryInBlocks verifies fn.Entry is present in fn.Blocks.
func (v *validator) checkEntryInBlocks() bool {
	for _, blk := range v.fn.Blocks {
		if blk == v.fn.Entry {
			return true
		}
	}
	v.errorf("entry block B%d is not in fn.Blocks", v.fn.Entry.ID)
	return false
}

// checkBlockIDUniqueness verifies no two blocks share an ID.
func (v *validator) checkBlockIDUniqueness() {
	seen := make(map[int]*Block)
	for _, blk := range v.fn.Blocks {
		if prev, ok := seen[blk.ID]; ok {
			_ = prev
			v.errorf("duplicate block ID %d", blk.ID)
		}
		seen[blk.ID] = blk
	}
}

// checkNilEdges checks for nil entries in Succs and Preds.
func (v *validator) checkNilEdges() {
	for _, blk := range v.fn.Blocks {
		for i, succ := range blk.Succs {
			if succ == nil {
				v.errorf("B%d: nil successor at index %d", blk.ID, i)
			}
		}
		for i, pred := range blk.Preds {
			if pred == nil {
				v.errorf("B%d: nil predecessor at index %d", blk.ID, i)
			}
		}
	}
}

// checkTerminators verifies every block ends with a terminator and has no
// terminator in the middle.
func (v *validator) checkTerminators() {
	for _, blk := range v.fn.Blocks {
		if len(blk.Instrs) == 0 {
			v.errorf("B%d: block has no instructions (missing terminator)", blk.ID)
			continue
		}

		// Last instruction must be a terminator.
		last := blk.Instrs[len(blk.Instrs)-1]
		if !validatorContractForOp(last.Op).Terminator {
			v.errorf("B%d: last instruction %s (v%d) is not a terminator",
				blk.ID, last.Op, last.ID)
		}

		// No terminator should appear in the middle.
		for i := 0; i < len(blk.Instrs)-1; i++ {
			if validatorContractForOp(blk.Instrs[i].Op).Terminator {
				v.errorf("B%d: terminator %s (v%d) in middle of block at position %d",
					blk.ID, blk.Instrs[i].Op, blk.Instrs[i].ID, i)
			}
		}
	}
}

// checkOpSpecs verifies the basic operation contracts described by OpSpec.
func (v *validator) checkOpSpecs() {
	for _, blk := range v.fn.Blocks {
		for i, instr := range blk.Instrs {
			contract := validatorContractForOp(instr.Op)
			v.checkArgPolicy(blk, instr, contract.Args)
			if contract.SuccCount == opArgMaxAny || i != len(blk.Instrs)-1 {
				continue
			}
			nSuccs := len(blk.Succs)
			if nSuccs != contract.SuccCount {
				v.errorf("B%d: %s must have %d successors, got %d", blk.ID, instr.Op, contract.SuccCount, nSuccs)
			}
		}
	}
}

func (v *validator) checkArgPolicy(blk *Block, instr *Instr, policy validatorArgPolicy) {
	if instr == nil || policy.accepts(len(instr.Args)) {
		return
	}
	if policy.Min == policy.Max {
		v.errorf("B%d: %s (v%d) must have exactly %d args, got %d",
			blk.ID, instr.Op, instr.ID, policy.Min, len(instr.Args))
		return
	}
	v.errorf("B%d: %s (v%d) must have %s args, got %d",
		blk.ID, instr.Op, instr.ID, policy.describe(), len(instr.Args))
}

// checkSuccPredConsistency verifies that if B is in A.Succs then A is in B.Preds,
// and vice versa.
func (v *validator) checkSuccPredConsistency() {
	// Forward: A.Succs contains B → B.Preds must contain A.
	for _, blk := range v.fn.Blocks {
		for _, succ := range blk.Succs {
			if succ == nil {
				continue // nil edges caught separately
			}
			if !containsBlock(succ.Preds, blk) {
				v.errorf("B%d in Succs of B%d, but B%d not in Preds of B%d",
					succ.ID, blk.ID, blk.ID, succ.ID)
			}
		}
	}

	// Reverse: A.Preds contains B → B.Succs must contain A.
	for _, blk := range v.fn.Blocks {
		for _, pred := range blk.Preds {
			if pred == nil {
				continue
			}
			if !containsBlock(pred.Succs, blk) {
				v.errorf("B%d in Preds of B%d, but B%d not in Succs of B%d",
					pred.ID, blk.ID, blk.ID, pred.ID)
			}
		}
	}
}

func (v *validator) checkOpContracts() {
	for _, blk := range v.fn.Blocks {
		for _, instr := range blk.Instrs {
			switch instr.Op {
			case OpGuardType:
				if Type(instr.Aux) == TypeUnknown || Type(instr.Aux) == TypeAny {
					v.errorf("B%d: GuardType (v%d) must carry a concrete type in Aux, got %s",
						blk.ID, instr.ID, Type(instr.Aux))
				}
			case OpGuardGlobalConst:
				if instr.Aux < 0 {
					v.errorf("B%d: GuardGlobalConst (v%d) must carry a non-negative constant index in Aux, got %d",
						blk.ID, instr.ID, instr.Aux)
				}
			case OpGuardConstString:
				if instr.Aux < 0 {
					v.errorf("B%d: GuardConstString (v%d) must carry a non-negative constant index in Aux, got %d",
						blk.ID, instr.ID, instr.Aux)
				}
			case OpGuardTableKind:
				if _, ok := fbKindToAK(instr.Aux); !ok {
					v.errorf("B%d: GuardTableKind (v%d) must carry a concrete table array kind in Aux, got %d",
						blk.ID, instr.ID, instr.Aux)
				}
			case OpGuardFieldCalleeProto:
				shapeID := uint32(instr.Aux2 >> 32)
				fieldIdx := int(int32(instr.Aux2 & 0xFFFFFFFF))
				if instr.Aux == 0 || shapeID == 0 || fieldIdx < 0 {
					v.errorf("B%d: GuardFieldCalleeProto (v%d) must carry proto Aux and shape/field Aux2, got Aux=%d Aux2=%d",
						blk.ID, instr.ID, instr.Aux, instr.Aux2)
				}
			case OpGuardShapeFieldType:
				shapeID := uint32(instr.Aux >> 32)
				fieldIdx := int(int32(instr.Aux & 0xFFFFFFFF))
				if shapeID == 0 || fieldIdx < 0 || Type(instr.Aux2) == TypeAny || Type(instr.Aux2) == TypeUnknown {
					v.errorf("B%d: GuardShapeFieldType (v%d) must carry shape/field Aux and concrete Aux2 type, got Aux=%d Aux2=%d",
						blk.ID, instr.ID, instr.Aux, instr.Aux2)
				}
			case OpGuardShapeFieldTypeMask:
				shapeID := uint32(instr.Aux >> 32)
				typ := Type(uint32(instr.Aux))
				if shapeID == 0 || typ == TypeAny || typ == TypeUnknown || instr.Aux2 == 0 {
					v.errorf("B%d: GuardShapeFieldTypeMask (v%d) must carry shape/type Aux and non-empty Aux2 mask, got Aux=%d Aux2=%d",
						blk.ID, instr.ID, instr.Aux, instr.Aux2)
				}
			case OpFieldSvals:
				if instr.Aux <= 0 {
					v.errorf("B%d: FieldSvals (v%d) must carry a positive shape id in Aux, got %d",
						blk.ID, instr.ID, instr.Aux)
				}
			case OpFieldLoad, OpFieldLoadNumToFloat:
				if instr.Aux < 0 {
					v.errorf("B%d: %s (v%d) must carry a non-negative field index in Aux, got %d",
						blk.ID, instr.Op, instr.ID, instr.Aux)
				}
			case OpFieldStore:
				if instr.Aux < 0 {
					v.errorf("B%d: FieldStore (v%d) must carry a non-negative field index in Aux, got %d",
						blk.ID, instr.ID, instr.Aux)
				}
			case OpFieldPolyLen:
				if instr.Aux < 0 {
					v.errorf("B%d: FieldPolyLen (v%d) must carry a non-negative constant index in Aux, got %d",
						blk.ID, instr.ID, instr.Aux)
				}
			case OpRecordArrayLoopKernel:
				if _, ok := functionLoopSpecializationFacts(v.fn).RecordArrayLoopSpecialization(instr.ID); !ok {
					v.errorf("B%d: RecordArrayLoopKernel (v%d) must have a kernel spec", blk.ID, instr.ID)
				}
			}
		}
	}
}

// checkValueIDUniqueness verifies no two instructions share a value ID.
func (v *validator) checkValueIDUniqueness() {
	type loc struct {
		blockID int
		instrOp Op
	}
	seen := make(map[int]loc)
	for _, blk := range v.fn.Blocks {
		for _, instr := range blk.Instrs {
			if prev, ok := seen[instr.ID]; ok {
				v.errorf("duplicate value ID v%d: in B%d (%s) and B%d (%s)",
					instr.ID, prev.blockID, prev.instrOp, blk.ID, instr.Op)
			}
			seen[instr.ID] = loc{blockID: blk.ID, instrOp: instr.Op}
		}
	}
}

// checkReachability verifies all blocks are reachable from fn.Entry.
// Unreachable blocks are reported as warnings (still errors in the return value).
func (v *validator) checkReachability() {
	reachable := make(map[*Block]bool)
	var walk func(b *Block)
	walk = func(b *Block) {
		if reachable[b] {
			return
		}
		reachable[b] = true
		for _, succ := range b.Succs {
			if succ != nil {
				walk(succ)
			}
		}
	}
	walk(v.fn.Entry)

	for _, blk := range v.fn.Blocks {
		if !reachable[blk] {
			v.errorf("B%d is unreachable from entry", blk.ID)
		}
	}
}

// containsBlock returns true if blocks contains target.
func containsBlock(blocks []*Block, target *Block) bool {
	for _, b := range blocks {
		if b == target {
			return true
		}
	}
	return false
}
