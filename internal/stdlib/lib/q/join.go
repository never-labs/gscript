package q

// Canonical kdb+/q dyadic `,` (join).
//
// Semantics matrix (joinValue):
//
//	atom,atom       -> 2-vector: like kinds make a typed vector, unlike kinds
//	                   a generic list (join never promotes numeric kinds).
//	vector,atom     -> append; atom,vector -> prepend (kind rules as above).
//	vector,vector   -> concatenation: same kind stays typed (dense typed
//	                   kernel, no boxing), mixed kinds build a generic list.
//	list,list       -> generic concatenation.
//	dict,dict       -> key-union merge, right wins on collisions (canonical
//	                   upsert semantics).
//	table,table     -> row append for matching schemas; mismatched schemas
//	                   error (canonical q 'mismatch).
//	keyed,keyed     -> row upsert by key for matching schemas/keys.
//	string,string   -> string concatenation. Strings are atoms in this
//	                   dialect, but canonical q strings are char lists whose
//	                   join concatenates; "ab","cd" -> "abcd" keeps the
//	                   canonical observable behavior.
//	(),x and x,()   -> empty operands contribute no kind: the result adopts
//	                   the other side's kind ((),1 is the typed one-element
//	                   vector 1#1, matching canonical type relaxation).
//	,x              -> prefix `,` (leftmost, empty left operand) is enlist.
//
// Parser/evaluator integration: `,` joins the single-precedence dyadic verb
// level. In the string evaluator it is claimed by qTopLevelJoinSplit just
// before the loose-binding dyadic probes (composite compares, ~, ?, word
// verbs, # _ $ !) whenever the `,` is the LEFTMOST top-level dyadic form, so
// canonical right-to-left grouping holds: `1+2,3 4` splits at `+` first
// (findDyadic carries ','), while `(`a`b!1 2),`b`c!20 30` splits at `,`
// before the dict `!` probe can claim it. When a looser pre-existing form
// sits LEFT of the comma (x~y,z / x#y,z / x!y,z), the established probe keeps
// the outermost position and the comma evaluates inside its right operand,
// which is exactly canonical right-to-left grouping for those sources too.

import (
	"fmt"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func joinValue(left, right any) (any, error) {
	if leftDict, ok := left.(EvalDict); ok {
		rightDict, ok := right.(EvalDict)
		if !ok {
			return nil, fmt.Errorf("join: dict can only be joined with a dict (got %T)", right)
		}
		return joinDicts(leftDict, rightDict), nil
	}
	if _, ok := right.(EvalDict); ok {
		return nil, fmt.Errorf("join: dict can only be joined with a dict (got %T)", left)
	}
	if leftFrame, ok := left.(data.Frame); ok {
		rightFrame, ok := right.(data.Frame)
		if !ok {
			return nil, fmt.Errorf("join: table can only be joined with a table (got %T)", right)
		}
		out, err := data.AppendFrames(leftFrame, rightFrame)
		if err != nil {
			return nil, fmt.Errorf("join: %w", err)
		}
		return out, nil
	}
	if _, ok := right.(data.Frame); ok {
		return nil, fmt.Errorf("join: table can only be joined with a table (got %T)", left)
	}
	if leftKeyed, ok := left.(data.KeyedFrame); ok {
		rightKeyed, ok := right.(data.KeyedFrame)
		if !ok {
			return nil, fmt.Errorf("join: keyed table can only be joined with a keyed table (got %T)", right)
		}
		return joinKeyedFrames(leftKeyed, rightKeyed)
	}
	if _, ok := right.(data.KeyedFrame); ok {
		return nil, fmt.Errorf("join: keyed table can only be joined with a keyed table (got %T)", left)
	}
	if leftString, ok := left.(string); ok {
		if rightString, ok := right.(string); ok {
			// Canonical q strings are char lists; their join concatenates.
			return leftString + rightString, nil
		}
	}
	return joinSequences(left, right)
}

// joinDicts is the canonical dict,dict key-union merge: left entries keep
// their order, right entries overwrite matching keys in place (right wins)
// and unseen right keys append.
func joinDicts(left, right EvalDict) EvalDict {
	keys := append([]any(nil), left.Keys...)
	values := append([]any(nil), left.Values...)
	for i, key := range right.Keys {
		if i >= len(right.Values) {
			break
		}
		found := false
		for j, existing := range keys {
			if equalValue(existing, key) {
				values[j] = right.Values[i]
				found = true
				break
			}
		}
		if !found {
			keys = append(keys, key)
			values = append(values, right.Values[i])
		}
	}
	return EvalDict{Keys: keys, Values: values}
}

// joinKeyedFrames is the canonical keyed-table,keyed-table upsert: right rows
// update matching keys and append new keys. Schemas (columns and key
// columns) must match.
func joinKeyedFrames(left, right data.KeyedFrame) (any, error) {
	columns := data.KeyedFrameColumnNames(left)
	rightColumns := data.KeyedFrameColumnNames(right)
	if !symbolSlicesEqual(columns, rightColumns) {
		return nil, fmt.Errorf("join: keyed table schema mismatch")
	}
	if !symbolSlicesEqual(left.Keys(), right.Keys()) {
		return nil, fmt.Errorf("join: keyed table key mismatch")
	}
	rows, err := insertRowsForColumns(columns, right.Frame())
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}
	out := left
	for _, row := range rows {
		out, err = out.UpsertRow(columns, row)
		if err != nil {
			return nil, fmt.Errorf("join: %w", err)
		}
	}
	return out, nil
}

func symbolSlicesEqual(left, right []data.Symbol) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// joinSequences joins two atom/vector/list operands. Same-kind dense typed
// vectors concatenate through the typed kernel without boxing; everything
// else goes through the boxed kind-merge path.
func joinSequences(left, right any) (any, error) {
	leftArray, leftIsArray := left.(data.Array)
	rightArray, rightIsArray := right.(data.Array)
	if leftIsArray && rightIsArray {
		// Empty operands adopt the other side (canonical type relaxation);
		// returning the other operand directly is safe under the q
		// copy-on-write contract.
		if leftArray.Len() == 0 && rightArray.Len() > 0 {
			return right, nil
		}
		if rightArray.Len() == 0 && leftArray.Len() > 0 {
			return left, nil
		}
		if out, handled := data.TryTypedConcat(leftArray, rightArray); handled {
			recordRuntimeKernelProbe("ArrayConcat", "join/"+string(leftArray.Kind()), handled, nil)
			return out, nil
		}
	}
	leftValues, leftKind := joinOperandValues(left, leftArray, leftIsArray)
	rightValues, rightKind := joinOperandValues(right, rightArray, rightIsArray)
	values := make([]any, 0, len(leftValues)+len(rightValues))
	values = append(values, leftValues...)
	values = append(values, rightValues...)
	kind, generic := mergeJoinKinds(leftKind, rightKind)
	if generic || kind == "" {
		return data.NewAny(values), nil
	}
	column, err := data.NewColumnWithKind("_", kind, values)
	if err != nil {
		return data.NewAny(values), nil
	}
	return column.Data, nil
}

// joinOperandValues returns one operand's element values and the kind it
// contributes to the join result. Empty operands and untyped-null atoms
// contribute no kind; generic lists and unkindable atoms contribute the
// generic marker data.KindAny.
func joinOperandValues(v any, array data.Array, isArray bool) ([]any, data.Kind) {
	if isArray {
		if array.Len() == 0 {
			return nil, ""
		}
		kind := array.Kind()
		if kind == "" || kind == data.KindNull {
			return array.Values(), ""
		}
		return array.Values(), kind
	}
	kind := qKindOfValue(v)
	if kind == data.KindNull {
		kind = ""
	}
	if kind == "" && !data.IsNull(v) {
		kind = data.KindAny
	}
	return []any{v}, kind
}

// mergeJoinKinds merges the two contributed kinds. Join never promotes:
// unlike non-empty kinds force a generic list (canonical 1 2,3.5 is a mixed
// list, not a float vector).
func mergeJoinKinds(left, right data.Kind) (data.Kind, bool) {
	if left == data.KindAny || right == data.KindAny {
		return "", true
	}
	switch {
	case left == "":
		return right, false
	case right == "" || left == right:
		return left, false
	default:
		return "", true
	}
}

// qTopLevelJoinSplit reports the position of a top-level `,` join verb that
// is the LEFTMOST top-level dyadic form in src, i.e. the canonical outermost
// operation under q's right-to-left evaluation. It declines when:
//
//   - the leftmost findDyadic verb is not `,` (an arithmetic/compare verb
//     left of the comma is the outermost operation),
//   - src is a bare top-level `a;b` statement list,
//   - the text LEFT of the comma carries a top-level form one of the
//     established looser probes claims first (~ ? # _ $ ! splits): those keep
//     their pre-existing outermost role and the comma evaluates inside their
//     right operand,
//   - a top-level word-form dyadic verb appears ANYWHERE in src: this
//     dialect's word verbs split outermost over the symbol verbs (the
//     pipeline plans and the word-map probe both claim `1+2 xbar 4` as
//     (1+2) xbar 4), and some of those claims (xexp/xlog/... pipeline
//     shapes) run before this probe in the string cascade, so the comma
//     must yield to keep the string/compiled routes identical.
//
// idx == 0 (empty left operand) is the prefix `,x` enlist form.
func qTopLevelJoinSplit(src string) (int, bool) {
	idx, op, ok := findDyadic(src)
	if !ok || op != ',' {
		return -1, false
	}
	if len(splitTopLevelDelim(src, ';')) > 1 {
		return -1, false
	}
	if findTopLevel(src[:idx], "~?#_$!") >= 0 {
		return -1, false
	}
	if _, _, _, ok := splitTopLevelDyadicWordMap(src, qDyadicWordOps); ok {
		return -1, false
	}
	if _, _, _, ok := splitTopLevelDyadicWordMap(src, qSetWordOps); ok {
		return -1, false
	}
	return idx, true
}
