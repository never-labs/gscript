package methodjit

// IntExactDivisionPass narrows integer-valued numeric recurrences that contain
// exact divisions guarded by a dominating modulo-zero branch.
//
// The canonical shape is:
//
//	if x % c == 0 { x = x / c } else { x = k*x + m }
//
// TypeSpecialize initially makes x/c a float, which can force the phi and the
// rest of the recurrence onto float/generic paths. This pass proves the divide
// result is an integer on the guarded edge, solves the surrounding
// int-preserving cycle, and rewrites those ops back to raw integer IR.
func IntExactDivisionPass(fn *Function) (*Function, error) {
	out, _, err := runIntExactDivisionIfCandidate(fn)
	return out, err
}

// IntExactDivisionPassCtx is the domain-scoped form of IntExactDivision. It is a
// pure IR transform reached via ctx.Func() — it computes dominators and use/def
// maps directly from the IR and consults no analysis fact domain. It records
// whether it rewrote anything in the pipeline options so the following
// RangeAnalysis can skip when nothing changed.
func IntExactDivisionPassCtx(ctx *PassContext) (*Function, error) {
	out, changed, err := runIntExactDivisionIfCandidate(ctx.Func())
	if opts := ctx.Opts(); opts != nil {
		opts.LastPassChanged = changed
	}
	return out, err
}

func runIntExactDivisionIfCandidate(fn *Function) (*Function, bool, error) {
	if fn == nil || len(fn.Blocks) == 0 {
		return fn, false, nil
	}

	proven := findModuloProvenDivisions(fn)
	if len(proven) == 0 {
		return fn, false, nil
	}
	candidates := collectIntExactDivCandidates(fn, proven)
	if len(candidates) == 0 {
		return fn, false, nil
	}

	intPossible := solveIntNarrowableValues(fn, proven, candidates)
	changed := false
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if !intPossible[instr.ID] {
				continue
			}
			before := instr.Op
			if instr.Op == OpPhi {
				instr.Type = TypeInt
			} else if narrowed, ok := exactIntNarrowOp(instr.Op); ok && (narrowed != OpDivIntExact || proven[instr.ID]) {
				instr.Op = narrowed
				instr.Type = TypeInt
				if narrowed == OpDivIntExact {
					instr.Aux2 = 1 // exactness was proven by dominating modulo-zero branch
				}
			}
			if instr.Op != before {
				changed = true
				functionRemarks(fn).Add("IntExactDiv", "changed", block.ID, instr.ID, instr.Op,
					"narrowed integer-exact numeric recurrence from "+before.String())
			}
		}
	}

	if !changed {
		return fn, false, nil
	}

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			narrowed, ok := exactIntNarrowOp(instr.Op)
			if !ok || narrowed == OpDivIntExact || len(instr.Args) < 2 ||
				!intArg(instr.Args[0], intPossible) || !intArg(instr.Args[1], intPossible) {
				continue
			}
			if narrowed != OpEqInt && narrowed != OpLtInt && narrowed != OpLeInt {
				continue
			}
			before := instr.Op
			instr.Op = narrowed
			instr.Type = TypeBool
			functionRemarks(fn).Add("IntExactDiv", "changed", block.ID, instr.ID, instr.Op,
				"specialized comparison after integer narrowing from "+before.String())
		}
	}

	return fn, true, nil
}

func exactIntNarrowOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.ExactIntNarrowOp, ok && spec.ExactIntNarrowOp < OpMax
}

func findModuloProvenDivisions(fn *Function) map[int]bool {
	dom := computeDominators(fn)
	proofs := make([]modZeroProof, 0)

	for _, block := range fn.Blocks {
		term := blockTerminator(block)
		if term == nil || term.Op != OpBranch || len(term.Args) == 0 || len(block.Succs) < 1 {
			continue
		}
		proof, ok := parseModZeroProof(term.Args[0])
		if !ok {
			continue
		}
		proof.branch = block
		proof.trueBlock = block.Succs[0]
		proofs = append(proofs, proof)
	}

	out := make(map[int]bool)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op != OpDiv && instr.Op != OpDivFloat {
				continue
			}
			if len(instr.Args) < 2 {
				continue
			}
			divisor, ok := constIntFromValue(instr.Args[1])
			if !ok || divisor == 0 {
				continue
			}
			for _, proof := range proofs {
				if proof.divisor != divisor {
					continue
				}
				if proof.lhs == nil || instr.Args[0] == nil || proof.lhs.ID != instr.Args[0].ID {
					continue
				}
				if dom.dominates(proof.branch.ID, block.ID) && dom.dominates(proof.trueBlock.ID, block.ID) {
					out[instr.ID] = true
					break
				}
			}
		}
	}
	return out
}

type modZeroProof struct {
	lhs       *Value
	divisor   int64
	branch    *Block
	trueBlock *Block
}

func parseModZeroProof(v *Value) (modZeroProof, bool) {
	if v == nil || v.Def == nil {
		return modZeroProof{}, false
	}
	eq := v.Def
	if eq.Op != OpEq && eq.Op != OpEqInt {
		return modZeroProof{}, false
	}
	if len(eq.Args) < 2 {
		return modZeroProof{}, false
	}

	if proof, ok := parseModZeroArgs(eq.Args[0], eq.Args[1]); ok {
		return proof, true
	}
	return parseModZeroArgs(eq.Args[1], eq.Args[0])
}

func parseModZeroArgs(modVal, zeroVal *Value) (modZeroProof, bool) {
	zero, ok := constIntFromValue(zeroVal)
	if !ok || zero != 0 || modVal == nil || modVal.Def == nil {
		return modZeroProof{}, false
	}
	mod := modVal.Def
	if mod.Op != OpMod && mod.Op != OpModInt {
		return modZeroProof{}, false
	}
	if len(mod.Args) < 2 {
		return modZeroProof{}, false
	}
	divisor, ok := constIntFromValue(mod.Args[1])
	if !ok || divisor == 0 {
		return modZeroProof{}, false
	}
	return modZeroProof{lhs: mod.Args[0], divisor: divisor}, true
}

func collectIntExactDivCandidates(fn *Function, provenDiv map[int]bool) map[int]bool {
	uses := buildInstrUses(fn)
	defs := buildInstrDefs(fn)
	out := make(map[int]bool)
	for divID := range provenDiv {
		component := collectExactDivComponent(divID, uses, defs, provenDiv)
		if !componentHasPhi(component, defs) || componentHasObservableUse(component, uses) {
			continue
		}
		for id := range component {
			out[id] = true
		}
	}
	return out
}

func collectExactDivComponent(root int, uses map[int][]*Instr, defs map[int]*Instr, provenDiv map[int]bool) map[int]bool {
	component := make(map[int]bool)
	work := []int{root}
	for len(work) > 0 {
		id := work[len(work)-1]
		work = work[:len(work)-1]
		if component[id] {
			continue
		}
		instr := defs[id]
		if instr == nil || !isExactDivComponentOp(instr, provenDiv) {
			continue
		}
		component[id] = true
		for _, arg := range instr.Args {
			if arg != nil && arg.Def != nil && isExactDivComponentOp(arg.Def, provenDiv) {
				work = append(work, arg.ID)
			}
		}
		for _, use := range uses[id] {
			if use != nil && isExactDivComponentOp(use, provenDiv) {
				work = append(work, use.ID)
			}
		}
	}
	return component
}

func isExactDivComponentOp(instr *Instr, provenDiv map[int]bool) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	if ok && spec.ExactDivComponent {
		return true
	}
	switch instr.Op {
	case OpDiv, OpDivFloat:
		return provenDiv[instr.ID]
	default:
		return false
	}
}

func componentHasPhi(component map[int]bool, defs map[int]*Instr) bool {
	for id := range component {
		if defs[id] != nil && defs[id].Op == OpPhi {
			return true
		}
	}
	return false
}

func componentHasObservableUse(component map[int]bool, uses map[int][]*Instr) bool {
	for id := range component {
		for _, use := range uses[id] {
			if use == nil || component[use.ID] {
				continue
			}
			if isExactDivAllowedExternalUse(use.Op) {
				continue
			}
			return true
		}
	}
	return false
}

func isExactDivAllowedExternalUse(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.ExactDivAllowedExternalUse
}

func buildInstrDefs(fn *Function) map[int]*Instr {
	defs := make(map[int]*Instr)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr != nil && !instr.Op.IsTerminator() {
				defs[instr.ID] = instr
			}
		}
	}
	return defs
}

func solveIntNarrowableValues(fn *Function, provenDiv, candidates map[int]bool) map[int]bool {
	possible := make(map[int]bool)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if candidates[instr.ID] && mayBeIntNarrowed(instr, provenDiv) {
				possible[instr.ID] = true
			}
		}
	}

	changed := true
	for changed {
		changed = false
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if !possible[instr.ID] {
					continue
				}
				if !intNarrowConstraintsHold(instr, possible, provenDiv) {
					delete(possible, instr.ID)
					changed = true
				}
			}
		}
	}
	return possible
}

func mayBeIntNarrowed(instr *Instr, provenDiv map[int]bool) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	switch instr.Op {
	case OpConstInt, OpGuardType, OpGuardIntRange, OpUnboxInt:
		return instr.Op != OpGuardType || instr.Type == TypeInt || instr.Aux == int64(TypeInt)
	case OpPhi:
		return instr.Type == TypeInt || instr.Type == TypeFloat || instr.Type == TypeAny || instr.Type == TypeUnknown
	case OpDiv, OpDivFloat:
		return provenDiv[instr.ID]
	default:
		return (ok && spec.IntNarrowCandidate) || instr.Type == TypeInt
	}
}

func intNarrowConstraintsHold(instr *Instr, possible map[int]bool, provenDiv map[int]bool) bool {
	switch instr.Op {
	case OpConstInt:
		return true
	case OpGuardType, OpGuardIntRange:
		return instr.Type == TypeInt || instr.Aux == int64(TypeInt)
	case OpUnboxInt:
		return true
	case OpDiv, OpDivFloat:
		return provenDiv[instr.ID] && len(instr.Args) >= 2 && allArgsInt(instr, possible)
	default:
		spec, ok := instr.Op.Spec()
		if ok && spec.IntNarrowAllArgsConstraint {
			return len(instr.Args) > 0 && allArgsInt(instr, possible)
		}
		return instr.Type == TypeInt
	}
}

func allArgsInt(instr *Instr, possible map[int]bool) bool {
	for _, arg := range instr.Args {
		if !intArg(arg, possible) {
			return false
		}
	}
	return true
}

func intArg(v *Value, possible map[int]bool) bool {
	if v == nil || v.Def == nil {
		return false
	}
	return possible[v.ID] || v.Def.Type == TypeInt || v.Def.Op == OpConstInt
}

func blockTerminator(block *Block) *Instr {
	if block == nil || len(block.Instrs) == 0 {
		return nil
	}
	return block.Instrs[len(block.Instrs)-1]
}
