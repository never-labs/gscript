package methodjit

import (
	"testing"

	"github.com/never-labs/gscript/internal/vm"
)

// setAllTier2InstallFields populates every proto field that an installed Tier 2
// function publishes, so teardown tests can assert the full canonical set is
// cleared. Keep this in sync with clearTier2InstallPointers.
func setAllTier2InstallFields(proto *vm.FuncProto) {
	proto.DirectEntryPtr = 0x1000
	proto.Tier2DirectEntryPtr = 0x2000
	proto.Tier2LeafEntryPtr = 0x3000
	proto.Tier2NumericEntryPtr = 0x4000
	proto.Tier2TypedEntryPtr = 0x5000
	proto.Tier2TypedClobberEntryPtr = 0x6000
	proto.Tier2TypedEntryABI = 0x7
	proto.Tier2GlobalCachePtr = 0x8000
	proto.Tier2GlobalCacheGenPtr = 0x9000
	proto.Tier2GlobalIndexPtr = 0xA000
	proto.Tier2Promoted = true
}

func assertAllTier2InstallFieldsZero(t *testing.T, proto *vm.FuncProto) {
	t.Helper()
	if proto.DirectEntryPtr != 0 {
		t.Errorf("DirectEntryPtr not cleared: %#x", proto.DirectEntryPtr)
	}
	if proto.Tier2DirectEntryPtr != 0 {
		t.Errorf("Tier2DirectEntryPtr not cleared: %#x", proto.Tier2DirectEntryPtr)
	}
	if proto.Tier2LeafEntryPtr != 0 {
		t.Errorf("Tier2LeafEntryPtr not cleared: %#x", proto.Tier2LeafEntryPtr)
	}
	if proto.Tier2NumericEntryPtr != 0 {
		t.Errorf("Tier2NumericEntryPtr not cleared: %#x", proto.Tier2NumericEntryPtr)
	}
	if proto.Tier2TypedEntryPtr != 0 {
		t.Errorf("Tier2TypedEntryPtr not cleared: %#x", proto.Tier2TypedEntryPtr)
	}
	if proto.Tier2TypedClobberEntryPtr != 0 {
		t.Errorf("Tier2TypedClobberEntryPtr not cleared: %#x", proto.Tier2TypedClobberEntryPtr)
	}
	if proto.Tier2TypedEntryABI != 0 {
		t.Errorf("Tier2TypedEntryABI not cleared: %#x", proto.Tier2TypedEntryABI)
	}
	if proto.Tier2GlobalCachePtr != 0 {
		t.Errorf("Tier2GlobalCachePtr not cleared: %#x", proto.Tier2GlobalCachePtr)
	}
	if proto.Tier2GlobalCacheGenPtr != 0 {
		t.Errorf("Tier2GlobalCacheGenPtr not cleared: %#x", proto.Tier2GlobalCacheGenPtr)
	}
	if proto.Tier2GlobalIndexPtr != 0 {
		t.Errorf("Tier2GlobalIndexPtr not cleared: %#x", proto.Tier2GlobalIndexPtr)
	}
	if proto.Tier2Promoted {
		t.Errorf("Tier2Promoted not cleared")
	}
}

// (a) Full field-set teardown: after clearTier2Install, every Tier 2 install
// pointer/flag field must be zero. Pins the canonical field set.
func TestClearTier2InstallZeroesFullInstallFieldSet(t *testing.T) {
	tm := NewTieringManager()
	proto := &vm.FuncProto{Name: "teardown_full"}
	cf := &CompiledFunction{Proto: proto}
	tm.markTier2Compiled(proto, cf)
	setAllTier2InstallFields(proto)

	tm.clearTier2Install(proto)

	assertAllTier2InstallFieldsZero(t, proto)
	// clearTier2Install policy: the map entry must be removed.
	if _, ok := tm.rawTier2CompiledFor(proto); ok {
		t.Fatalf("clearTier2Install must delete the tier2Compiled map entry")
	}
}

// (b) DirectEntryVersion monotonic bump: teardown bumps the version exactly once
// so JIT'd callers re-validate before reusing a stale Tier2DirectEntryPtr.
func TestClearTier2InstallBumpsDirectEntryVersionExactlyOnce(t *testing.T) {
	tm := NewTieringManager()
	proto := &vm.FuncProto{Name: "teardown_version"}
	cf := &CompiledFunction{Proto: proto}
	tm.markTier2Compiled(proto, cf)
	setAllTier2InstallFields(proto)

	before := proto.DirectEntryVersion
	tm.clearTier2Install(proto)
	after := proto.DirectEntryVersion

	if after <= before {
		t.Fatalf("DirectEntryVersion did not increase: before=%d after=%d", before, after)
	}
	if after != before+1 {
		t.Fatalf("DirectEntryVersion bumped more than once: before=%d after=%d", before, after)
	}
}

// (c) Map retention after loop refresh: retireStaleTier2AfterFeedback's queued
// loop branch zeroes the install pointers but MUST keep the cf in the
// tier2Compiled map (the queued recompile reuses it).
func TestRetireStaleTier2LoopRefreshKeepsMapEntry(t *testing.T) {
	tm := NewTieringManager()
	proto := &vm.FuncProto{
		Name: "teardown_loop_retain",
		Code: []uint32{
			vm.EncodeAsBx(vm.OP_FORPREP, 0, 1),
			vm.EncodeABC(vm.OP_GETFIELD, 1, 0, 0),
			vm.EncodeAsBx(vm.OP_FORLOOP, 0, -2),
		},
	}
	proto.EnsureFeedback()
	proto.FieldAccessFeedback[1].Count = 1
	proto.FieldAccessFeedback[1].ShapeID = 11
	proto.FieldAccessFeedback[1].FieldIdx = 0
	setAllTier2InstallFields(proto)

	// Install cf at an empty (stale) snapshot directly so that the matured
	// field feedback drives the queued loop-refresh branch.
	cf := &CompiledFunction{Proto: proto, SpeculationSnapshot: Tier2FeedbackSnapshot{}}
	tm.tier2Compiled[proto] = cf

	tm.retireStaleTier2AfterFeedback(proto, cf)

	// Install pointers zeroed...
	assertAllTier2InstallFieldsZero(t, proto)
	// ...but the map entry is retained (policy: queued recompile reuses cf).
	got, ok := tm.rawTier2CompiledFor(proto)
	if !ok || got != cf {
		t.Fatalf("loop refresh must keep cf in tier2Compiled map: ok=%v got=%p want=%p", ok, got, cf)
	}
	if _, ok := tm.recompileQueue.take(proto); !ok {
		t.Fatal("loop refresh should have queued a next-entry recompile")
	}
}
