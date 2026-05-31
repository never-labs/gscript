package binfmt

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

type Field struct {
	Kind  string
	Count int
}

type Format struct {
	Order  binary.ByteOrder
	Fields []Field
}

func Parse(format string) (Format, error) {
	result := Format{Order: binary.LittleEndian}
	tokens := strings.FieldsFunc(format, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ','
	})
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "<") || strings.HasPrefix(token, ">") {
			if token[0] == '<' {
				result.Order = binary.LittleEndian
			} else {
				result.Order = binary.BigEndian
			}
			token = token[1:]
			if token == "" {
				continue
			}
		}
		if strings.Contains(token, ":") {
			parts := strings.SplitN(token, ":", 2)
			if setOrder(&result, parts[0]) {
				token = parts[1]
			}
		}
		if setOrder(&result, token) {
			continue
		}
		field, err := ParseField(token)
		if err != nil {
			return result, err
		}
		result.Fields = append(result.Fields, field)
	}
	return result, nil
}

func ParseField(token string) (Field, error) {
	field := Field{Kind: strings.ToLower(token), Count: -1}
	if strings.Contains(field.Kind, ":") {
		parts := strings.SplitN(field.Kind, ":", 2)
		field.Kind = parts[0]
		n, err := strconv.Atoi(parts[1])
		if err != nil || n < 0 {
			return field, fmt.Errorf("binary: invalid field size %q", parts[1])
		}
		field.Count = n
	}
	switch field.Kind {
	case "i8", "int8", "u8", "uint8",
		"i16", "int16", "u16", "uint16",
		"i32", "int32", "u32", "uint32",
		"i64", "int64", "u64", "uint64",
		"f32", "float32", "f64", "float64",
		"string", "str", "bytes":
		return field, nil
	default:
		return field, fmt.Errorf("binary: unknown field type %q", token)
	}
}

func FieldSize(field Field) (int, bool) {
	switch field.Kind {
	case "i8", "int8", "u8", "uint8":
		return 1, true
	case "i16", "int16", "u16", "uint16":
		return 2, true
	case "i32", "int32", "u32", "uint32", "f32", "float32":
		return 4, true
	case "i64", "int64", "u64", "uint64", "f64", "float64":
		return 8, true
	case "string", "str", "bytes":
		if field.Count >= 0 {
			return field.Count, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func setOrder(format *Format, token string) bool {
	switch strings.ToLower(token) {
	case "le", "little", "littleendian":
		format.Order = binary.LittleEndian
		return true
	case "be", "big", "bigendian":
		format.Order = binary.BigEndian
		return true
	default:
		return false
	}
}
