package runtime

// Metamethod and metatable-aware access helpers for the tree-walking
// interpreter: getMetamethod, luaToString, tableGet/tableSet (and their depth
// variants), tableLenInt, and opToMetamethod.
// Moved verbatim from interpreter.go (pure code movement).

import (
	"fmt"
	"strings"
)

// ====================================================================
// Metamethod helpers
// ====================================================================

const maxMetaDepth = 50

// getMetamethod returns the metamethod for the given event (__add, etc.)
// Returns the metamethod Value and true if found.
func (interp *Interpreter) getMetamethod(val Value, event string) (Value, bool) {
	var mt *Table
	if val.IsTable() {
		mt = val.Table().GetMetatable()
	}
	if mt == nil {
		return NilValue(), false
	}
	mm := mt.RawGet(StringValue(event))
	if mm.IsNil() {
		return NilValue(), false
	}
	return mm, true
}

func (interp *Interpreter) luaToString(v Value) (string, error) {
	if v.IsTable() {
		if mt := v.Table().GetMetatable(); mt != nil {
			if mm := mt.RawGetString("__tostring"); !mm.IsNil() {
				results, err := interp.callFunction(mm, []Value{v})
				if err != nil {
					return "", err
				}
				if len(results) == 0 || !results[0].IsString() {
					return "", fmt.Errorf("'__tostring' must return a string")
				}
				return results[0].Str(), nil
			}
			if name := mt.RawGetString("__name"); name.IsString() {
				return name.Str() + ": " + strings.TrimPrefix(v.String(), "table: "), nil
			}
		}
	}
	return v.String(), nil
}

// tableGet retrieves a value from a table, with __index metamethod support.
func (interp *Interpreter) tableGet(t Value, key Value) (Value, error) {
	return interp.tableGetDepth(t, key, 0)
}

func (interp *Interpreter) tableGetDepth(t Value, key Value, depth int) (Value, error) {
	if depth > maxMetaDepth {
		return NilValue(), fmt.Errorf("'__index' chain too long; possible loop")
	}
	if t.IsDenseArray() {
		if i, ok, err := DenseArrayIndexFromValue(key, t.DenseArray().Len()); ok || err != nil {
			if err != nil {
				return NilValue(), err
			}
			return t.DenseArray().At(i)
		}
		return NilValue(), fmt.Errorf("attempt to index a %s value", t.TypeName())
	}
	if t.IsSoA() {
		if v, ok, err := t.SoA().GetIndex(key); ok || err != nil {
			return v, err
		}
		return NilValue(), fmt.Errorf("attempt to index a %s value", t.TypeName())
	}
	if !t.IsTable() {
		return NilValue(), fmt.Errorf("attempt to index a %s value", t.TypeName())
	}

	tbl := t.Table()
	val := tbl.RawGet(key)

	if !val.IsNil() {
		return val, nil
	}

	// Try __index
	mt := tbl.GetMetatable()
	if mt == nil {
		return NilValue(), nil
	}

	index := mt.RawGetString("__index")
	if index.IsNil() {
		return NilValue(), nil
	}

	if index.IsTable() {
		return interp.tableGetDepth(TableValue(index.Table()), key, depth+1)
	}

	if index.IsFunction() {
		results, err := interp.callFunction(index, []Value{t, key})
		if err != nil {
			return NilValue(), err
		}
		if len(results) > 0 {
			return results[0], nil
		}
		return NilValue(), nil
	}

	return NilValue(), nil
}

// tableSet assigns a value to a table, with __newindex metamethod support.
func (interp *Interpreter) tableSet(t Value, key, val Value) error {
	return interp.tableSetDepth(t, key, val, 0)
}

func (interp *Interpreter) tableSetDepth(t Value, key, val Value, depth int) error {
	if depth > maxMetaDepth {
		return fmt.Errorf("'__newindex' chain too long; possible loop")
	}
	if t.IsSoA() {
		if handled, err := t.SoA().SetIndex(key, val); handled || err != nil {
			return err
		}
		return fmt.Errorf("attempt to index a %s value", t.TypeName())
	}
	if !t.IsTable() {
		return fmt.Errorf("attempt to index a %s value", t.TypeName())
	}

	tbl := t.Table()

	// Check if key already exists (rawget) - if so, just set it directly
	existing := tbl.RawGet(key)
	if !existing.IsNil() {
		tbl.RawSet(key, val)
		return nil
	}

	// Check __newindex
	mt := tbl.GetMetatable()
	if mt != nil {
		newindex := mt.RawGetString("__newindex")
		if newindex.IsFunction() {
			_, err := interp.callFunction(newindex, []Value{t, key, val})
			return err
		}
		if newindex.IsTable() {
			return interp.tableSetDepth(TableValue(newindex.Table()), key, val, depth+1)
		}
	}

	tbl.RawSet(key, val)
	return nil
}

func (interp *Interpreter) tableLenInt(t Value) (int64, error) {
	if !t.IsTable() {
		return 0, fmt.Errorf("attempt to get length of a %s value", t.TypeName())
	}
	if mm, ok := interp.getMetamethod(t, "__len"); ok {
		results, err := interp.callFunction(mm, []Value{t})
		if err != nil {
			return 0, err
		}
		if len(results) == 0 {
			return 0, nil
		}
		return toInt(results[0]), nil
	}
	return int64(t.Table().Length()), nil
}

// opToMetamethod maps arithmetic operators to metamethod names.
func opToMetamethod(op string) string {
	switch op {
	case "+":
		return "__add"
	case "-":
		return "__sub"
	case "*":
		return "__mul"
	case "/":
		return "__div"
	case "%":
		return "__mod"
	case "**":
		return "__pow"
	default:
		return ""
	}
}
