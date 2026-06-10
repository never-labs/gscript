package runtime

// RuntimePrimitiveID is a stable VM-level identifier for typed runtime
// operations that can be targeted by interpreters, q pipelines, and JIT
// backends without depending on individual helper function names.
type RuntimePrimitiveID string

const (
	RuntimePrimitiveDenseArrayGather RuntimePrimitiveID = "dense_array.gather"
)

// RuntimePrimitiveDescriptor describes a typed runtime operation independently
// of any one frontend syntax.
type RuntimePrimitiveDescriptor struct {
	ID     RuntimePrimitiveID
	Family string
	Op     string
	Shape  string
}

var runtimePrimitiveRegistry = map[RuntimePrimitiveID]RuntimePrimitiveDescriptor{
	RuntimePrimitiveDenseArrayGather: {
		ID:     RuntimePrimitiveDenseArrayGather,
		Family: "vector",
		Op:     "gather",
		Shape:  "dense-array/i64-indexes",
	},
}

// LookupRuntimePrimitive returns the registry descriptor for a VM primitive.
func LookupRuntimePrimitive(id RuntimePrimitiveID) (RuntimePrimitiveDescriptor, bool) {
	desc, ok := runtimePrimitiveRegistry[id]
	return desc, ok
}

func runtimePrimitiveDescriptor(id RuntimePrimitiveID) RuntimePrimitiveDescriptor {
	if desc, ok := LookupRuntimePrimitive(id); ok {
		return desc
	}
	return RuntimePrimitiveDescriptor{ID: id, Family: "unknown", Op: "unknown", Shape: "unknown"}
}

func recordRuntimePrimitiveHit(id RuntimePrimitiveID) {
	RecordRuntimePathPrimitiveHit(runtimePrimitiveDescriptor(id))
}

func recordRuntimePrimitiveError(id RuntimePrimitiveID) {
	RecordRuntimePathPrimitiveError(runtimePrimitiveDescriptor(id))
}
