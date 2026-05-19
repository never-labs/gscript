package runtime

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type binaryField struct {
	kind  string
	count int
}

type binaryFormat struct {
	order  binary.ByteOrder
	fields []binaryField
}

// buildBinaryLib creates the "binary" standard library table.
func buildBinaryLib() *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "binary." + name,
			Fn:   fn,
		}))
	}

	// binary.pack(format, ...) -> string
	//
	// Format tokens are whitespace/comma separated. The default byte order is
	// little-endian; use a leading "le"/"<" or "be"/">" token or prefix
	// ("be:u16") to switch. Supported fields:
	// i8/u8/i16/u16/i32/u32/i64/u64/f32/f64/string/str/bytes.
	// string and bytes are length-prefixed with a u32 unless written as
	// string:N or bytes:N, which encodes exactly N raw bytes.
	set("pack", func(args []Value) ([]Value, error) { return binaryPackValues("binary.pack", args) })

	// binary.unpack(format, data [, offset]) -> values..., nextOffset
	// Offset is 1-based, matching GScript string positions.
	set("unpack", func(args []Value) ([]Value, error) { return binaryUnpackValues("binary.unpack", args) })

	// binary.size(format) -> byte count, or nil,err for variable-size formats.
	set("size", func(args []Value) ([]Value, error) { return binarySizeValues("binary.size", args) })

	return t
}

func binaryPackValues(apiName string, args []Value) ([]Value, error) {
	if len(args) < 1 || !args[0].IsString() {
		return nil, fmt.Errorf("bad argument #1 to '%s' (format string expected)", apiName)
	}
	format, err := parseBinaryFormat(args[0].Str())
	if err != nil {
		return nil, err
	}
	if len(args)-1 != len(format.fields) {
		return nil, fmt.Errorf("%s: got %d values for %d fields", apiName, len(args)-1, len(format.fields))
	}
	var buf bytes.Buffer
	for i, field := range format.fields {
		if err := packBinaryField(&buf, format.order, field, args[i+1]); err != nil {
			return nil, fmt.Errorf("%s", strings.Replace(err.Error(), "binary.pack", apiName, 1))
		}
	}
	return []Value{StringValue(buf.String())}, nil
}

func binaryUnpackValues(apiName string, args []Value) ([]Value, error) {
	if len(args) < 2 || !args[0].IsString() || !args[1].IsString() {
		return nil, fmt.Errorf("bad argument to '%s' (format and data strings expected)", apiName)
	}
	format, err := parseBinaryFormat(args[0].Str())
	if err != nil {
		return nil, err
	}
	offset := 1
	if len(args) >= 3 {
		offset = int(toInt(args[2]))
	}
	if offset < 1 {
		return []Value{NilValue(), StringValue(apiName + ": offset out of range")}, nil
	}
	data := []byte(args[1].Str())
	pos := offset - 1
	results := make([]Value, 0, len(format.fields)+1)
	for _, field := range format.fields {
		v, next, err := unpackBinaryField(data, pos, format.order, field)
		if err != nil {
			return []Value{NilValue(), StringValue(strings.Replace(err.Error(), "binary.unpack", apiName, 1))}, nil
		}
		results = append(results, v)
		pos = next
	}
	results = append(results, IntValue(int64(pos+1)))
	return results, nil
}

func binarySizeValues(apiName string, args []Value) ([]Value, error) {
	if len(args) < 1 || !args[0].IsString() {
		return nil, fmt.Errorf("bad argument #1 to '%s' (format string expected)", apiName)
	}
	format, err := parseBinaryFormat(args[0].Str())
	if err != nil {
		return nil, err
	}
	total := 0
	for _, field := range format.fields {
		n, fixed := binaryFieldSize(field)
		if !fixed {
			return []Value{NilValue(), StringValue(apiName + ": variable-size field in format")}, nil
		}
		total += n
	}
	return []Value{IntValue(int64(total))}, nil
}

func parseBinaryFormat(format string) (binaryFormat, error) {
	result := binaryFormat{order: binary.LittleEndian}
	tokens := strings.FieldsFunc(format, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ','
	})
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "<") || strings.HasPrefix(token, ">") {
			if token[0] == '<' {
				result.order = binary.LittleEndian
			} else {
				result.order = binary.BigEndian
			}
			token = token[1:]
			if token == "" {
				continue
			}
		}
		if strings.Contains(token, ":") {
			parts := strings.SplitN(token, ":", 2)
			if setBinaryOrder(&result, parts[0]) {
				token = parts[1]
			}
		}
		if setBinaryOrder(&result, token) {
			continue
		}
		field, err := parseBinaryField(token)
		if err != nil {
			return result, err
		}
		result.fields = append(result.fields, field)
	}
	return result, nil
}

func setBinaryOrder(format *binaryFormat, token string) bool {
	switch strings.ToLower(token) {
	case "le", "little", "littleendian":
		format.order = binary.LittleEndian
		return true
	case "be", "big", "bigendian":
		format.order = binary.BigEndian
		return true
	default:
		return false
	}
}

func parseBinaryField(token string) (binaryField, error) {
	field := binaryField{kind: strings.ToLower(token), count: -1}
	if strings.Contains(field.kind, ":") {
		parts := strings.SplitN(field.kind, ":", 2)
		field.kind = parts[0]
		n, err := strconv.Atoi(parts[1])
		if err != nil || n < 0 {
			return field, fmt.Errorf("binary: invalid field size %q", parts[1])
		}
		field.count = n
	}
	switch field.kind {
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

func binaryFieldSize(field binaryField) (int, bool) {
	switch field.kind {
	case "i8", "int8", "u8", "uint8":
		return 1, true
	case "i16", "int16", "u16", "uint16":
		return 2, true
	case "i32", "int32", "u32", "uint32", "f32", "float32":
		return 4, true
	case "i64", "int64", "u64", "uint64", "f64", "float64":
		return 8, true
	case "string", "str", "bytes":
		if field.count >= 0 {
			return field.count, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func packBinaryField(buf *bytes.Buffer, order binary.ByteOrder, field binaryField, value Value) error {
	switch field.kind {
	case "i8", "int8":
		n, err := requireSigned(value, -128, 127)
		if err != nil {
			return err
		}
		return buf.WriteByte(byte(int8(n)))
	case "u8", "uint8":
		n, err := requireUnsigned(value, math.MaxUint8)
		if err != nil {
			return err
		}
		return buf.WriteByte(byte(n))
	case "i16", "int16":
		n, err := requireSigned(value, math.MinInt16, math.MaxInt16)
		if err != nil {
			return err
		}
		b := make([]byte, 2)
		order.PutUint16(b, uint16(int16(n)))
		_, err = buf.Write(b)
		return err
	case "u16", "uint16":
		n, err := requireUnsigned(value, math.MaxUint16)
		if err != nil {
			return err
		}
		b := make([]byte, 2)
		order.PutUint16(b, uint16(n))
		_, err = buf.Write(b)
		return err
	case "i32", "int32":
		n, err := requireSigned(value, math.MinInt32, math.MaxInt32)
		if err != nil {
			return err
		}
		b := make([]byte, 4)
		order.PutUint32(b, uint32(int32(n)))
		_, err = buf.Write(b)
		return err
	case "u32", "uint32":
		n, err := requireUnsigned(value, math.MaxUint32)
		if err != nil {
			return err
		}
		b := make([]byte, 4)
		order.PutUint32(b, uint32(n))
		_, err = buf.Write(b)
		return err
	case "i64", "int64":
		n, err := requireSigned(value, math.MinInt64, math.MaxInt64)
		if err != nil {
			return err
		}
		b := make([]byte, 8)
		order.PutUint64(b, uint64(n))
		_, err = buf.Write(b)
		return err
	case "u64", "uint64":
		n, err := requireUnsigned(value, math.MaxUint64)
		if err != nil {
			return err
		}
		b := make([]byte, 8)
		order.PutUint64(b, n)
		_, err = buf.Write(b)
		return err
	case "f32", "float32":
		b := make([]byte, 4)
		order.PutUint32(b, math.Float32bits(float32(toFloat(value))))
		_, err := buf.Write(b)
		return err
	case "f64", "float64":
		b := make([]byte, 8)
		order.PutUint64(b, math.Float64bits(toFloat(value)))
		_, err := buf.Write(b)
		return err
	case "string", "str", "bytes":
		if !value.IsString() {
			return fmt.Errorf("binary.pack: string expected for %s", field.kind)
		}
		data := []byte(value.Str())
		if field.count >= 0 {
			if len(data) != field.count {
				return fmt.Errorf("binary.pack: %s:%d got %d bytes", field.kind, field.count, len(data))
			}
			_, err := buf.Write(data)
			return err
		}
		if len(data) > math.MaxUint32 {
			return fmt.Errorf("binary.pack: string too large")
		}
		b := make([]byte, 4)
		order.PutUint32(b, uint32(len(data)))
		if _, err := buf.Write(b); err != nil {
			return err
		}
		_, err := buf.Write(data)
		return err
	default:
		return fmt.Errorf("binary: unknown field type %q", field.kind)
	}
}

func unpackBinaryField(data []byte, pos int, order binary.ByteOrder, field binaryField) (Value, int, error) {
	need, fixed := binaryFieldSize(field)
	if fixed {
		if pos < 0 || pos+need > len(data) {
			return NilValue(), pos, fmt.Errorf("binary.unpack: data too short")
		}
	}
	switch field.kind {
	case "i8", "int8":
		return IntValue(int64(int8(data[pos]))), pos + 1, nil
	case "u8", "uint8":
		return IntValue(int64(data[pos])), pos + 1, nil
	case "i16", "int16":
		return IntValue(int64(int16(order.Uint16(data[pos:])))), pos + 2, nil
	case "u16", "uint16":
		return IntValue(int64(order.Uint16(data[pos:]))), pos + 2, nil
	case "i32", "int32":
		return IntValue(int64(int32(order.Uint32(data[pos:])))), pos + 4, nil
	case "u32", "uint32":
		return IntValue(int64(order.Uint32(data[pos:]))), pos + 4, nil
	case "i64", "int64":
		return IntValue(int64(order.Uint64(data[pos:]))), pos + 8, nil
	case "u64", "uint64":
		u := order.Uint64(data[pos:])
		if u > uint64(maxInt48) {
			return FloatValue(float64(u)), pos + 8, nil
		}
		return IntValue(int64(u)), pos + 8, nil
	case "f32", "float32":
		return FloatValue(float64(math.Float32frombits(order.Uint32(data[pos:])))), pos + 4, nil
	case "f64", "float64":
		return FloatValue(math.Float64frombits(order.Uint64(data[pos:]))), pos + 8, nil
	case "string", "str", "bytes":
		if field.count >= 0 {
			return StringValue(string(data[pos : pos+field.count])), pos + field.count, nil
		}
		if pos+4 > len(data) {
			return NilValue(), pos, fmt.Errorf("binary.unpack: data too short")
		}
		n := int(order.Uint32(data[pos:]))
		pos += 4
		if pos+n > len(data) {
			return NilValue(), pos, fmt.Errorf("binary.unpack: data too short")
		}
		return StringValue(string(data[pos : pos+n])), pos + n, nil
	default:
		return NilValue(), pos, fmt.Errorf("binary: unknown field type %q", field.kind)
	}
}

func requireSigned(v Value, min, max int64) (int64, error) {
	if !v.IsNumber() && !v.IsString() {
		return 0, fmt.Errorf("binary.pack: number expected")
	}
	n := toInt(v)
	if n < min || n > max {
		return 0, fmt.Errorf("binary.pack: value %d out of range [%d, %d]", n, min, max)
	}
	return n, nil
}

func requireUnsigned(v Value, max uint64) (uint64, error) {
	if !v.IsNumber() && !v.IsString() {
		return 0, fmt.Errorf("binary.pack: number expected")
	}
	if v.IsFloat() && v.Float() < 0 {
		return 0, fmt.Errorf("binary.pack: negative value for unsigned field")
	}
	n := toInt(v)
	if n < 0 {
		return 0, fmt.Errorf("binary.pack: negative value for unsigned field")
	}
	u := uint64(n)
	if u > max {
		return 0, fmt.Errorf("binary.pack: value %d out of range [0, %d]", u, max)
	}
	return u, nil
}
