package modules

import "github.com/never-labs/leia/internal/stdlibrt"

func defaultTableOptions() stdlibrt.TableOptions {
	return stdlibrt.TableOptions{
		Less: func(a, b Value) (bool, error) {
			less, ok := a.LessThan(b)
			return ok && less, nil
		},
		Len: func(t Value) (int64, error) {
			return int64(t.Table().Length()), nil
		},
		Get: func(t Value, key Value) (Value, error) {
			return t.Table().RawGet(key), nil
		},
		Set: func(t Value, key Value, val Value) error {
			t.Table().RawSet(key, val)
			return nil
		},
		TryPlainSort: func(t Value, length int64) bool {
			return t.Table().TryPlainArraySort(length)
		},
		TryPlainMove: func(src, dst Value, first, last, target int64) bool {
			return dst.Table().TryPlainArrayMove(src.Table(), first, last, target)
		},
		TryPlainInsert: func(t Value, pos int64, val Value, length int64) bool {
			return t.Table().TryPlainArrayInsertKnownLength(pos, val, length)
		},
		TryPlainRemove: func(t Value, pos int64, length int64) (Value, bool) {
			return t.Table().TryPlainArrayRemoveKnownLength(pos, length)
		},
	}
}

func mergeTableOptions(base stdlibrt.TableOptions, override stdlibrt.TableOptions) stdlibrt.TableOptions {
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
