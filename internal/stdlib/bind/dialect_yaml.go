package bind

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func dialectYAML(body Value, opts *Table, maxHostResult func() int64) ([]Value, error) {
	mode := dialectMode(opts)
	if !dialectModeAllowed(mode, "", "parse", "decode", "encode", "format") {
		return dialectUnknownMode("yaml", mode)
	}
	if body.IsString() && mode != "encode" && mode != "format" {
		value, err := parseYAMLLite(body.Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{value}, nil
	}
	text, err := encodeYAMLLite(body, hostResultLimit(maxHostResult))
	if err != nil {
		return nil, err
	}
	return []Value{StringValue(text)}, nil
}

type yamlLine struct {
	no     int
	indent int
	text   string
}

func parseYAMLLite(src string) (Value, error) {
	lines, err := yamlLogicalLines(src)
	if err != nil {
		return NilValue(), err
	}
	if len(lines) == 0 {
		return TableValue(NewTable()), nil
	}
	value, idx, err := parseYAMLBlock(lines, 0, lines[0].indent)
	if err != nil {
		return NilValue(), err
	}
	if idx != len(lines) {
		return NilValue(), fmt.Errorf("yaml dialect: line %d: unexpected indentation", lines[idx].no)
	}
	return value, nil
}

func yamlLogicalLines(src string) ([]yamlLine, error) {
	raw := strings.Split(strings.ReplaceAll(strings.ReplaceAll(src, "\r\n", "\n"), "\r", "\n"), "\n")
	lines := make([]yamlLine, 0, len(raw))
	for i, line := range raw {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "\t") {
			return nil, fmt.Errorf("yaml dialect: line %d: tabs are not supported for indentation", i+1)
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		text := strings.TrimSpace(stripYAMLComment(line[indent:]))
		if text == "" {
			continue
		}
		lines = append(lines, yamlLine{no: i + 1, indent: indent, text: text})
	}
	return lines, nil
}

func stripYAMLComment(line string) string {
	inSingle, inDouble := false, false
	for i, ch := range line {
		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble && (i == 0 || line[i-1] == ' ') {
				return strings.TrimSpace(line[:i])
			}
		}
	}
	return strings.TrimSpace(line)
}

func parseYAMLBlock(lines []yamlLine, idx, indent int) (Value, int, error) {
	if idx >= len(lines) {
		return TableValue(NewTable()), idx, nil
	}
	if lines[idx].indent < indent {
		return TableValue(NewTable()), idx, nil
	}
	if lines[idx].indent != indent {
		return NilValue(), idx, fmt.Errorf("yaml dialect: line %d: expected indent %d", lines[idx].no, indent)
	}
	if strings.HasPrefix(lines[idx].text, "- ") || lines[idx].text == "-" {
		return parseYAMLSeq(lines, idx, indent)
	}
	return parseYAMLMap(lines, idx, indent)
}

func parseYAMLMap(lines []yamlLine, idx, indent int) (Value, int, error) {
	out := NewTable()
	for idx < len(lines) {
		line := lines[idx]
		if line.indent < indent {
			break
		}
		if line.indent != indent {
			return NilValue(), idx, fmt.Errorf("yaml dialect: line %d: unexpected indent", line.no)
		}
		if strings.HasPrefix(line.text, "- ") || line.text == "-" {
			break
		}
		key, rest, ok := strings.Cut(line.text, ":")
		if !ok {
			return NilValue(), idx, fmt.Errorf("yaml dialect: line %d: missing ':'", line.no)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return NilValue(), idx, fmt.Errorf("yaml dialect: line %d: empty key", line.no)
		}
		rest = strings.TrimSpace(rest)
		if rest != "" {
			out.RawSetString(key, parseYAMLScalar(rest))
			idx++
			continue
		}
		if idx+1 >= len(lines) || lines[idx+1].indent <= indent {
			out.RawSetString(key, TableValue(NewTable()))
			idx++
			continue
		}
		child, next, err := parseYAMLBlock(lines, idx+1, lines[idx+1].indent)
		if err != nil {
			return NilValue(), idx, err
		}
		out.RawSetString(key, child)
		idx = next
	}
	return TableValue(out), idx, nil
}

func parseYAMLSeq(lines []yamlLine, idx, indent int) (Value, int, error) {
	out := NewAppendArrayTable(0)
	pos := int64(1)
	for idx < len(lines) {
		line := lines[idx]
		if line.indent < indent {
			break
		}
		if line.indent != indent || !(strings.HasPrefix(line.text, "- ") || line.text == "-") {
			break
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line.text, "-"))
		if rest == "" {
			if idx+1 >= len(lines) || lines[idx+1].indent <= indent {
				out.RawSetInt(pos, NilValue())
				pos++
				idx++
				continue
			}
			child, next, err := parseYAMLBlock(lines, idx+1, lines[idx+1].indent)
			if err != nil {
				return NilValue(), idx, err
			}
			out.RawSetInt(pos, child)
			pos++
			idx = next
			continue
		}
		if key, val, ok := strings.Cut(rest, ":"); ok && strings.TrimSpace(key) != "" {
			item := NewTable()
			key = strings.TrimSpace(key)
			val = strings.TrimSpace(val)
			if val == "" {
				item.RawSetString(key, TableValue(NewTable()))
			} else {
				item.RawSetString(key, parseYAMLScalar(val))
			}
			idx++
			for idx < len(lines) && lines[idx].indent > indent {
				field := lines[idx]
				fieldKey, fieldVal, ok := strings.Cut(field.text, ":")
				if !ok {
					return NilValue(), idx, fmt.Errorf("yaml dialect: line %d: missing ':'", field.no)
				}
				fieldKey = strings.TrimSpace(fieldKey)
				fieldVal = strings.TrimSpace(fieldVal)
				if fieldVal != "" {
					item.RawSetString(fieldKey, parseYAMLScalar(fieldVal))
					idx++
					continue
				}
				if idx+1 >= len(lines) || lines[idx+1].indent <= field.indent {
					item.RawSetString(fieldKey, TableValue(NewTable()))
					idx++
					continue
				}
				child, next, err := parseYAMLBlock(lines, idx+1, lines[idx+1].indent)
				if err != nil {
					return NilValue(), idx, err
				}
				item.RawSetString(fieldKey, child)
				idx = next
			}
			out.RawSetInt(pos, TableValue(item))
			pos++
			continue
		}
		out.RawSetInt(pos, parseYAMLScalar(rest))
		pos++
		idx++
	}
	return TableValue(out), idx, nil
}

func parseYAMLScalar(raw string) Value {
	raw = strings.TrimSpace(raw)
	switch strings.ToLower(raw) {
	case "null", "nil", "~":
		return NilValue()
	case "true":
		return BoolValue(true)
	case "false":
		return BoolValue(false)
	}
	if len(raw) >= 2 {
		if raw[0] == '\'' && raw[len(raw)-1] == '\'' {
			return StringValue(strings.ReplaceAll(raw[1:len(raw)-1], "''", "'"))
		}
		if raw[0] == '"' && raw[len(raw)-1] == '"' {
			if text, err := strconv.Unquote(raw); err == nil {
				return StringValue(text)
			}
		}
	}
	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return IntValue(i)
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil && strings.ContainsAny(raw, ".eE") {
		return FloatValue(f)
	}
	return StringValue(raw)
}

func encodeYAMLLite(v Value, limit int64) (string, error) {
	var b strings.Builder
	if err := writeYAMLValue(&b, v, 0, true); err != nil {
		return "", err
	}
	if err := CheckProjectedHostStringBytes(limit, b.Len()); err != nil {
		return "", err
	}
	return b.String(), nil
}

func writeYAMLValue(b *strings.Builder, v Value, indent int, root bool) error {
	if !v.IsTable() {
		b.WriteString(yamlScalarString(v))
		if root {
			b.WriteByte('\n')
		}
		return nil
	}
	tbl := v.Table()
	if isPlainArrayTable(tbl) {
		for i := 1; i <= tbl.Length(); i++ {
			item := tbl.RawGetInt(int64(i))
			writeYAMLIndent(b, indent)
			b.WriteString("-")
			if item.IsTable() {
				b.WriteByte('\n')
				if err := writeYAMLValue(b, item, indent+2, false); err != nil {
					return err
				}
			} else {
				b.WriteByte(' ')
				b.WriteString(yamlScalarString(item))
				b.WriteByte('\n')
			}
		}
		return nil
	}
	keys := plainStringKeys(tbl)
	sort.Strings(keys)
	for _, key := range keys {
		if strings.ContainsAny(key, ":\r\n") || strings.TrimSpace(key) != key || key == "" {
			return fmt.Errorf("yaml dialect: invalid key %q", key)
		}
		value := tbl.RawGetString(key)
		writeYAMLIndent(b, indent)
		b.WriteString(key)
		b.WriteByte(':')
		if value.IsTable() {
			b.WriteByte('\n')
			if err := writeYAMLValue(b, value, indent+2, false); err != nil {
				return err
			}
		} else {
			b.WriteByte(' ')
			b.WriteString(yamlScalarString(value))
			b.WriteByte('\n')
		}
	}
	return nil
}

func isPlainArrayTable(tbl *Table) bool {
	if tableHasAnyStringKey(tbl) {
		return false
	}
	return tbl.Length() > 0
}

func tableHasAnyStringKey(tbl *Table) bool {
	has := false
	tbl.ForEachPlainRaw(func(k, _ Value) bool {
		if k.IsString() {
			has = true
			return false
		}
		return true
	})
	return has
}

func plainStringKeys(tbl *Table) []string {
	keys := make([]string, 0)
	tbl.ForEachPlainRaw(func(k, _ Value) bool {
		if k.IsString() {
			keys = append(keys, k.Str())
		}
		return true
	})
	return keys
}

func writeYAMLIndent(b *strings.Builder, indent int) {
	for i := 0; i < indent; i++ {
		b.WriteByte(' ')
	}
}

func yamlScalarString(v Value) string {
	switch {
	case v.IsNil():
		return "null"
	case v.IsBool():
		if v.Bool() {
			return "true"
		}
		return "false"
	case v.IsInt(), v.IsFloat():
		return v.String()
	default:
		s := v.String()
		if s == "" || strings.ContainsAny(s, ":\n\r#\"'") || strings.TrimSpace(s) != s ||
			strings.EqualFold(s, "true") || strings.EqualFold(s, "false") || strings.EqualFold(s, "null") {
			return strconv.Quote(s)
		}
		return s
	}
}
