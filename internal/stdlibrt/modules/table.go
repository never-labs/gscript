package modules

import (
	"github.com/never-labs/gscript/internal/runtime"
)

// TableOptions provides the execution-engine hooks needed by table helpers
// that must respect script-visible metamethods and callbacks.
type TableOptions struct {
	Call runtime.ScriptFunctionCaller
	Less runtime.TableSortLess
	Len  runtime.TableSortLen
	Get  runtime.TableSortGet
	Set  runtime.TableSortSet

	TryPlainSort   runtime.TableSortTryPlainArraySort
	TryPlainMove   runtime.TableMoveTryPlainArrayMove
	TryPlainInsert runtime.TableInsertTryPlainArrayInsert
	TryPlainRemove runtime.TableRemoveTryPlainArrayRemove
}

// BuildTable creates the "table" standard-library module. By default it uses
// raw table access. Supplying TableOptions upgrades the helpers that need
// execution-engine semantics, such as table.sort callbacks and proxy-aware
// table access.
func BuildTable(options ...TableOptions) *runtime.Table {
	opts := defaultTableOptions()
	if len(options) > 0 {
		opts = mergeTableOptions(opts, options[0])
	}

	lib := runtime.BuildTableLib()
	lib.RawSet(runtime.StringValue("concat"), runtime.FunctionValue(runtime.BuildTableConcatFunction(runtime.TableUnpackLen(opts.Len), runtime.TableUnpackGet(opts.Get))))
	lib.RawSet(runtime.StringValue("insert"), runtime.FunctionValue(runtime.BuildTableInsertFunction(runtime.TableInsertLen(opts.Len), runtime.TableInsertGet(opts.Get), runtime.TableInsertSet(opts.Set), opts.TryPlainInsert)))
	lib.RawSet(runtime.StringValue("remove"), runtime.FunctionValue(runtime.BuildTableRemoveFunction(runtime.TableRemoveLen(opts.Len), runtime.TableRemoveGet(opts.Get), runtime.TableRemoveSet(opts.Set), opts.TryPlainRemove)))
	lib.RawSet(runtime.StringValue("unpack"), runtime.FunctionValue(runtime.BuildTableUnpackFunction("unpack", runtime.TableUnpackLen(opts.Len), runtime.TableUnpackGet(opts.Get))))
	lib.RawSet(runtime.StringValue("spread"), runtime.FunctionValue(runtime.BuildTableUnpackFunction("spread", runtime.TableUnpackLen(opts.Len), runtime.TableUnpackGet(opts.Get))))
	lib.RawSet(runtime.StringValue("move"), runtime.FunctionValue(runtime.BuildTableMoveFunction(runtime.TableMoveGet(opts.Get), runtime.TableMoveSet(opts.Set), opts.TryPlainMove)))

	if opts.Call != nil {
		lib.RawSet(runtime.StringValue("sort"), runtime.FunctionValue(runtime.BuildTableSortFunction(runtime.TableSortCaller(opts.Call), opts.Less, opts.Len, opts.Get, opts.Set, opts.TryPlainSort)))
		runtime.BuildTableHigherOrderLibWithCaller(opts.Call, lib)
	}
	return lib
}

func defaultTableOptions() TableOptions {
	return TableOptions{
		Less: func(a, b runtime.Value) (bool, error) {
			less, ok := a.LessThan(b)
			return ok && less, nil
		},
		Len: func(t runtime.Value) (int64, error) {
			return int64(t.Table().Length()), nil
		},
		Get: func(t runtime.Value, key runtime.Value) (runtime.Value, error) {
			return t.Table().RawGet(key), nil
		},
		Set: func(t runtime.Value, key runtime.Value, val runtime.Value) error {
			t.Table().RawSet(key, val)
			return nil
		},
		TryPlainSort: func(t runtime.Value, length int64) bool {
			return t.Table().TryPlainArraySort(length)
		},
		TryPlainMove: func(src, dst runtime.Value, first, last, target int64) bool {
			return dst.Table().TryPlainArrayMove(src.Table(), first, last, target)
		},
		TryPlainInsert: func(t runtime.Value, pos int64, val runtime.Value, length int64) bool {
			return t.Table().TryPlainArrayInsertKnownLength(pos, val, length)
		},
		TryPlainRemove: func(t runtime.Value, pos int64, length int64) (runtime.Value, bool) {
			return t.Table().TryPlainArrayRemoveKnownLength(pos, length)
		},
	}
}

func mergeTableOptions(base TableOptions, override TableOptions) TableOptions {
	if override.Call != nil {
		base.Call = override.Call
	}
	if override.Less != nil {
		base.Less = override.Less
	}
	if override.Len != nil {
		base.Len = override.Len
	}
	if override.Get != nil {
		base.Get = override.Get
	}
	if override.Set != nil {
		base.Set = override.Set
	}
	if override.TryPlainSort != nil {
		base.TryPlainSort = override.TryPlainSort
	}
	if override.TryPlainMove != nil {
		base.TryPlainMove = override.TryPlainMove
	}
	if override.TryPlainInsert != nil {
		base.TryPlainInsert = override.TryPlainInsert
	}
	if override.TryPlainRemove != nil {
		base.TryPlainRemove = override.TryPlainRemove
	}
	return base
}
