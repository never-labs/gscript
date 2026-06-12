package q

import (
	"fmt"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

// evalNamedInsertUpsert implements the canonical q name-targeted mutation
// verbs `` `t insert rows`` and `` `t upsert rows``: the named table in the
// session environment is mutated in place. insert appends and returns the
// new row indexes (erroring on duplicate keys for keyed tables); upsert
// appends for plain tables and key-merges for keyed tables, returning the
// table name symbol.
func (s *EvalState) evalNamedInsertUpsert(src string) (any, bool, error) {
	if !strings.HasPrefix(src, "`") {
		return nil, false, nil
	}
	rest := src[1:]
	if rest == "" || !isQIdentStart(rest[0]) {
		return nil, false, nil
	}
	end := 0
	for end < len(rest) && isQIdentRest(rest[end]) {
		end++
	}
	name := rest[:end]
	tail := rest[end:]
	if tail == "" || !isQWhitespace(tail[0]) {
		return nil, false, nil
	}
	tail = strings.TrimSpace(tail)
	verb := ""
	for _, candidate := range []string{"insert", "upsert"} {
		if strings.HasPrefix(tail, candidate) &&
			(len(tail) == len(candidate) || !isQIdentRest(tail[len(candidate)])) {
			verb = candidate
			break
		}
	}
	if verb == "" {
		return nil, false, nil
	}
	argSrc := strings.TrimSpace(tail[len(verb):])
	if argSrc == "" {
		return nil, true, fmt.Errorf("%s expects row values after the table name", verb)
	}
	value, err := s.eval(argSrc)
	if err != nil {
		return nil, true, err
	}
	out, err := s.applyNamedInsertUpsert(name, verb, value)
	return out, true, err
}

func (s *EvalState) applyNamedInsertUpsert(name, verb string, value any) (any, error) {
	resolved := s.resolveAssignmentName(name)
	target, ok := s.env[resolved]
	if !ok {
		return nil, fmt.Errorf("%s target `%s is not defined", verb, name)
	}
	switch table := target.(type) {
	case data.Frame:
		columns := data.FrameColumnNames(table)
		rows, err := insertRowsForColumns(columns, value)
		if err != nil {
			return nil, fmt.Errorf("%s `%s: %w", verb, name, err)
		}
		before := table.Len()
		out := table
		for _, row := range rows {
			out, err = data.InsertRow(out, columns, row)
			if err != nil {
				return nil, fmt.Errorf("%s `%s: %w", verb, name, err)
			}
		}
		s.env[resolved] = out
		if verb == "upsert" {
			return data.Symbol(name), nil
		}
		indexes := make([]int64, len(rows))
		for i := range indexes {
			indexes[i] = int64(before + i)
		}
		return data.NewI64(indexes), nil
	case data.KeyedFrame:
		columns := data.KeyedFrameColumnNames(table)
		rows, err := insertRowsForColumns(columns, value)
		if err != nil {
			return nil, fmt.Errorf("%s `%s: %w", verb, name, err)
		}
		out := table
		for _, row := range rows {
			if verb == "insert" {
				out, err = out.InsertRow(columns, row)
			} else {
				out, err = out.UpsertRow(columns, row)
			}
			if err != nil {
				return nil, fmt.Errorf("%s `%s: %w", verb, name, err)
			}
		}
		s.env[resolved] = out
		if verb == "upsert" {
			return data.Symbol(name), nil
		}
		// Canonical insert returns the appended row indexes; for keyed
		// tables every inserted key is new, appended at the end.
		before := data.KeyedFrameLen(table)
		indexes := make([]int64, len(rows))
		for i := range indexes {
			indexes[i] = int64(before + i)
		}
		return data.NewI64(indexes), nil
	default:
		return nil, fmt.Errorf("%s target `%s is not a table (got %T)", verb, name, target)
	}
}

// insertRowsForColumns normalizes an insert/upsert RHS into rows of values
// aligned with columns:
//   - a Frame contributes its rows (matched by column name)
//   - a generic list of atoms is one row
//   - a generic list of lists is column vectors (q's multi-row insert form)
//   - a dictionary is one row matched by key
//   - an atom or typed vector feeds a single-column table
func insertRowsForColumns(columns []data.Symbol, value any) ([][]any, error) {
	switch x := value.(type) {
	case data.Frame:
		names := data.FrameColumnNames(x)
		if len(names) != len(columns) {
			return nil, fmt.Errorf("row table has %d columns, want %d", len(names), len(columns))
		}
		rows := make([][]any, x.Len())
		for i := range rows {
			row := make([]any, len(columns))
			for j, column := range columns {
				columnData, ok := x.Column(column)
				if !ok {
					return nil, fmt.Errorf("row table is missing column %s", column)
				}
				cell, ok := columnData.At(i)
				if !ok {
					return nil, fmt.Errorf("row table column %s row %d out of range", column, i)
				}
				row[j] = cell
			}
			rows[i] = row
		}
		return rows, nil
	case EvalDict:
		row := make([]any, len(columns))
		for j, column := range columns {
			found := false
			for k, key := range x.Keys {
				if sym, ok := key.(data.Symbol); ok && sym == column {
					row[j] = x.Values[k]
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("row dictionary is missing column %s", column)
			}
		}
		return [][]any{row}, nil
	case data.Array:
		if x.Kind() == data.KindAny {
			items := x.Values()
			if len(items) != len(columns) {
				return nil, fmt.Errorf("row has %d values, want %d columns", len(items), len(columns))
			}
			allLists := len(items) > 0
			for _, item := range items {
				if _, ok := item.(data.Array); !ok {
					allLists = false
					break
				}
			}
			if !allLists {
				return [][]any{items}, nil
			}
			// Column-vector form: every item is the value list of one column.
			length := items[0].(data.Array).Len()
			for _, item := range items {
				if item.(data.Array).Len() != length {
					return nil, fmt.Errorf("insert column lengths differ")
				}
			}
			rows := make([][]any, length)
			for i := range rows {
				row := make([]any, len(columns))
				for j, item := range items {
					cell, ok := item.(data.Array).At(i)
					if !ok {
						return nil, fmt.Errorf("insert column %d row %d out of range", j, i)
					}
					row[j] = cell
				}
				rows[i] = row
			}
			return rows, nil
		}
		// Typed vector: a column of values for a single-column table, or a
		// single row when the width matches.
		if len(columns) == 1 {
			values := x.Values()
			rows := make([][]any, len(values))
			for i, cell := range values {
				rows[i] = []any{cell}
			}
			return rows, nil
		}
		values := x.Values()
		if len(values) == len(columns) {
			return [][]any{values}, nil
		}
		return nil, fmt.Errorf("row has %d values, want %d columns", len(values), len(columns))
	default:
		if len(columns) != 1 {
			return nil, fmt.Errorf("row atom needs a single-column table, got %d columns", len(columns))
		}
		return [][]any{{value}}, nil
	}
}
