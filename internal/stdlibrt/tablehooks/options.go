package tablehooks

import "github.com/never-labs/gscript/internal/runtime"

type Caller func(runtime.Value, []runtime.Value) ([]runtime.Value, error)
type Less func(runtime.Value, runtime.Value) (bool, error)
type Len func(runtime.Value) (int64, error)
type Get func(runtime.Value, runtime.Value) (runtime.Value, error)
type Set func(runtime.Value, runtime.Value, runtime.Value) error
type TryPlainArraySort func(runtime.Value, int64) bool
type MoveGet func(runtime.Value, runtime.Value) (runtime.Value, error)
type MoveSet func(runtime.Value, runtime.Value, runtime.Value) error
type TryPlainArrayMove func(src, dst runtime.Value, first, last, target int64) bool
type InsertLen func(runtime.Value) (int64, error)
type InsertGet func(runtime.Value, runtime.Value) (runtime.Value, error)
type InsertSet func(runtime.Value, runtime.Value, runtime.Value) error
type TryPlainArrayInsert func(runtime.Value, int64, runtime.Value, int64) bool
type RemoveLen func(runtime.Value) (int64, error)
type RemoveGet func(runtime.Value, runtime.Value) (runtime.Value, error)
type RemoveSet func(runtime.Value, runtime.Value, runtime.Value) error
type TryPlainArrayRemove func(runtime.Value, int64, int64) (runtime.Value, bool)
type UnpackLen func(runtime.Value) (int64, error)
type UnpackGet func(runtime.Value, runtime.Value) (runtime.Value, error)

// Options provides execution-engine hooks needed by table helpers that must
// respect script-visible metamethods and callbacks.
type Options struct {
	Call Caller
	Less Less
	Len  Len
	Get  Get
	Set  Set

	TryPlainSort   TryPlainArraySort
	TryPlainMove   TryPlainArrayMove
	TryPlainInsert TryPlainArrayInsert
	TryPlainRemove TryPlainArrayRemove
}
