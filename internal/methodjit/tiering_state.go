//go:build darwin && arm64

package methodjit

import "github.com/gscript/gscript/internal/vm"

type TierStateStore struct {
	compiled map[*vm.FuncProto]*CompiledFunction
	failed   map[*vm.FuncProto]bool
	reasons  map[*vm.FuncProto]string
}

func newTierStateStore(compiled map[*vm.FuncProto]*CompiledFunction, failed map[*vm.FuncProto]bool, reasons map[*vm.FuncProto]string) *TierStateStore {
	return &TierStateStore{
		compiled: compiled,
		failed:   failed,
		reasons:  reasons,
	}
}

func (s *TierStateStore) compiledFor(proto *vm.FuncProto) (*CompiledFunction, bool) {
	if s == nil || proto == nil {
		return nil, false
	}
	cf, ok := s.compiled[proto]
	return cf, ok
}

func (s *TierStateStore) compiledSnapshot() map[*vm.FuncProto]*CompiledFunction {
	out := make(map[*vm.FuncProto]*CompiledFunction)
	if s == nil {
		return out
	}
	for proto, cf := range s.compiled {
		out[proto] = cf
	}
	return out
}

func (s *TierStateStore) forEachCompiled(fn func(*vm.FuncProto, *CompiledFunction)) {
	if s == nil || fn == nil {
		return
	}
	for proto, cf := range s.compiled {
		fn(proto, cf)
	}
}

func (s *TierStateStore) hasFailed(proto *vm.FuncProto) bool {
	return s != nil && proto != nil && s.failed[proto]
}

func (s *TierStateStore) failReason(proto *vm.FuncProto) string {
	if s == nil || proto == nil {
		return ""
	}
	return s.reasons[proto]
}

func (s *TierStateStore) failReasonSnapshot() map[*vm.FuncProto]string {
	out := make(map[*vm.FuncProto]string)
	if s == nil {
		return out
	}
	for proto, reason := range s.reasons {
		out[proto] = reason
	}
	return out
}

func (s *TierStateStore) markCompiled(proto *vm.FuncProto, cf *CompiledFunction) {
	if s == nil || proto == nil || cf == nil {
		return
	}
	s.compiled[proto] = cf
}

func (s *TierStateStore) markFailed(proto *vm.FuncProto, reason string) {
	if s == nil || proto == nil {
		return
	}
	s.failed[proto] = true
	if reason == "" {
		return
	}
	if s.reasons == nil {
		s.reasons = make(map[*vm.FuncProto]string)
	}
	s.reasons[proto] = reason
}

func (s *TierStateStore) clearCompiled(proto *vm.FuncProto) {
	if s == nil || proto == nil {
		return
	}
	delete(s.compiled, proto)
}

func (s *TierStateStore) markJITDisabled(proto *vm.FuncProto) {
	if proto == nil {
		return
	}
	proto.JITDisabled = true
}

func (s *TierStateStore) temporarilyDisableJIT(proto *vm.FuncProto, body func()) {
	if proto == nil || body == nil {
		return
	}
	oldDisabled := proto.JITDisabled
	proto.JITDisabled = true
	defer func() {
		proto.JITDisabled = oldDisabled
	}()
	body()
}

func (tm *TieringManager) tier2CompiledFor(proto *vm.FuncProto) (*CompiledFunction, bool) {
	cf, ok := tm.rawTier2CompiledFor(proto)
	if !ok || cf == nil {
		return nil, false
	}
	if reason, stale := cf.DependencyInvalidationReason(tm.compilationDependencyContext()); stale {
		tm.invalidateStaleTier2Compiled(proto, reason)
		return nil, false
	}
	return cf, true
}

func (tm *TieringManager) rawTier2CompiledFor(proto *vm.FuncProto) (*CompiledFunction, bool) {
	if tm == nil || proto == nil {
		return nil, false
	}
	tm.ensureTierStateStore()
	return tm.tierState.compiledFor(proto)
}

func (tm *TieringManager) tier2HasFailed(proto *vm.FuncProto) bool {
	if tm == nil || proto == nil {
		return false
	}
	tm.ensureTierStateStore()
	return tm.tierState.hasFailed(proto)
}

func (tm *TieringManager) tier2FailReasonFor(proto *vm.FuncProto) string {
	if tm == nil || proto == nil {
		return ""
	}
	tm.ensureTierStateStore()
	return tm.tierState.failReason(proto)
}

func (tm *TieringManager) tier2CompiledSnapshot() map[*vm.FuncProto]*CompiledFunction {
	if tm == nil {
		return nil
	}
	tm.ensureTierStateStore()
	return tm.tierState.compiledSnapshot()
}

func (tm *TieringManager) forEachTier2Compiled(fn func(*vm.FuncProto, *CompiledFunction)) {
	if tm == nil || fn == nil {
		return
	}
	tm.ensureTierStateStore()
	tm.tierState.forEachCompiled(fn)
}

func (tm *TieringManager) tier2FailReasonSnapshot() map[*vm.FuncProto]string {
	if tm == nil {
		return nil
	}
	tm.ensureTierStateStore()
	return tm.tierState.failReasonSnapshot()
}

func (tm *TieringManager) markTier2Compiled(proto *vm.FuncProto, cf *CompiledFunction) {
	if tm == nil || proto == nil || cf == nil {
		return
	}
	profile := tm.currentTier2SpeculationProfile(proto)
	cf.SpeculationSnapshot = profile.Snapshot
	if cf.SpecializationVersion.Hash == 0 {
		cf.SpecializationVersion = profile.Version
	}
	tm.ensureTierStateStore()
	delete(tm.tier2InvalidReason, proto)
	tm.publishTier2Compiled(proto, cf)
}

// publishTier2Compiled is the single transactional entry that makes a freshly
// compiled Tier 2 function observable. It establishes the invariant:
//
//	cf is NOT observable as "published" (via tier2CompiledFor /
//	rawTier2CompiledFor) until BOTH its install pointers AND its reverse
//	spec-dependency registration are complete.
//
// Ordering rationale (load-bearing analysis):
//   - installTier2 writes only proto/cf entry-pointer fields; it never reads the
//     tierState compiled map, so it does not require markCompiled first.
//   - registerTier2SpecDependencies reads cf.SpecDependencyProtos and writes the
//     tm.specDependents reverse index keyed by callee; it never reads proto's own
//     compiled-map entry, so it does not require markCompiled first.
//
// Because neither install nor dependency registration depends on the compiled
// map, the observable publish (markCompiled) is performed LAST, after install +
// registration, so no concurrent/async/refresh path can ever observe a
// published-but-not-fully-registered intermediate state.
//
// queueSpecDependentsForRefresh runs after the publish: it enqueues OTHER protos
// (dependents of proto as a callee), not proto itself, so its position relative
// to markCompiled is immaterial for proto's own visibility.
//
// clearTier2Install / clearTier2InstallPointers form the symmetric teardown of
// the pointer/flag state established here.
func (tm *TieringManager) publishTier2Compiled(proto *vm.FuncProto, cf *CompiledFunction) {
	if tm == nil || proto == nil || cf == nil {
		return
	}
	tm.installTier2(proto, cf)
	tm.registerTier2SpecDependencies(proto, cf)
	// Observable publish: must be last so cf is never visible before its
	// install pointers and dependency registration above are complete.
	tm.tierState.markCompiled(proto, cf)
	tm.queueSpecDependentsForRefresh(proto, "spec_dependency_compiled")
}

func (tm *TieringManager) InvalidateTier2CompiledDependencies(proto *vm.FuncProto, ctx CompilationDependencyContext) (bool, string) {
	if tm == nil || proto == nil {
		return false, ""
	}
	cf, ok := tm.rawTier2CompiledFor(proto)
	if !ok || cf == nil {
		return false, ""
	}
	reason, stale := cf.DependencyInvalidationReason(ctx)
	if !stale {
		return false, ""
	}
	tm.invalidateStaleTier2Compiled(proto, reason)
	return true, reason
}

func (tm *TieringManager) compilationDependencyContext() CompilationDependencyContext {
	if tm == nil {
		return CompilationDependencyContext{}
	}
	return CompilationDependencyContext{Globals: tm.callVM}
}

func (tm *TieringManager) invalidateStaleTier2Compiled(proto *vm.FuncProto, reason string) {
	if tm == nil || proto == nil {
		return
	}
	if reason == "" {
		reason = "tier2 compilation dependency invalidated"
	}
	if tm.tier2InvalidReason == nil {
		tm.tier2InvalidReason = make(map[*vm.FuncProto]string)
	}
	tm.tier2InvalidReason[proto] = reason
	tm.traceEvent("tier2_dependency_invalidated", "tier2", proto, map[string]any{
		"reason":  reason,
		"install": "cleared",
	})
	tm.clearTier2Install(proto)
}

func (tm *TieringManager) tier2InvalidReasonFor(proto *vm.FuncProto) string {
	if tm == nil || proto == nil {
		return ""
	}
	return tm.tier2InvalidReason[proto]
}

func (tm *TieringManager) markTier2Failed(proto *vm.FuncProto, reason string) {
	if tm == nil || proto == nil {
		return
	}
	tm.ensureTierStateStore()
	tm.tierState.markFailed(proto, reason)
}

// clearTier2InstallPointers is the SINGLE SOURCE OF TRUTH for the mechanical
// teardown of every proto field that an installed Tier 2 function publishes. It
// performs ONLY the pointer/flag zeroing — it does NOT touch any policy state
// (the tier2Compiled map, spec dependencies, tier1 caches, recompile queue);
// each caller layers its own policy around this call.
//
// The canonical install field set is:
//   - Direct/Tier2 entry pointers + ABI (via clearFuncProtoDirectEntries):
//     DirectEntryPtr, Tier2DirectEntryPtr, Tier2LeafEntryPtr,
//     Tier2NumericEntryPtr, Tier2TypedEntryPtr, Tier2TypedClobberEntryPtr,
//     Tier2TypedEntryABI
//   - Tier2GlobalCachePtr, Tier2GlobalCacheGenPtr, Tier2GlobalIndexPtr
//   - Tier2Promoted
//
// IMPORTANT: clearFuncProtoDirectEntries zeroes the entry pointers AND bumps
// proto.DirectEntryVersion in one atomic operation; JIT'd callers validate that
// version before reusing Tier2DirectEntryPtr. Always route through it here — do
// not reimplement the field writes or bump the version separately.
func clearTier2InstallPointers(proto *vm.FuncProto) {
	if proto == nil {
		return
	}
	clearFuncProtoDirectEntries(proto)
	proto.Tier2Promoted = false
	proto.Tier2GlobalCachePtr = 0
	proto.Tier2GlobalCacheGenPtr = 0
	proto.Tier2GlobalIndexPtr = 0
}

func (tm *TieringManager) clearTier2Install(proto *vm.FuncProto) {
	if tm == nil || proto == nil {
		return
	}
	tm.ensureTierStateStore()
	tm.tierState.clearCompiled(proto)
	tm.clearTier2SpecDependenciesForCaller(proto)
	clearTier2InstallPointers(proto)
}

func (tm *TieringManager) markJITDisabled(proto *vm.FuncProto) {
	if tm == nil || proto == nil {
		return
	}
	tm.ensureTierStateStore()
	tm.tierState.markJITDisabled(proto)
}

func (tm *TieringManager) withJITTemporarilyDisabled(proto *vm.FuncProto, body func()) {
	if tm == nil || proto == nil || body == nil {
		return
	}
	tm.ensureTierStateStore()
	tm.tierState.temporarilyDisableJIT(proto, body)
}

func (tm *TieringManager) ensureTierStateStore() {
	if tm.tierState != nil {
		return
	}
	if tm.tier2Compiled == nil {
		tm.tier2Compiled = make(map[*vm.FuncProto]*CompiledFunction)
	}
	if tm.tier2Failed == nil {
		tm.tier2Failed = make(map[*vm.FuncProto]bool)
	}
	if tm.tier2FailReason == nil {
		tm.tier2FailReason = make(map[*vm.FuncProto]string)
	}
	if tm.tier2InvalidReason == nil {
		tm.tier2InvalidReason = make(map[*vm.FuncProto]string)
	}
	tm.tierState = newTierStateStore(tm.tier2Compiled, tm.tier2Failed, tm.tier2FailReason)
}
