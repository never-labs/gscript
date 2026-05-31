package binfmt

import (
	"fmt"
	"math"

	"github.com/never-labs/gscript/internal/outputlimit"
)

const maxInt48 = (1 << 47) - 1

type PackValue struct {
	IsNumber bool
	IsString bool
	IsFloat  bool
	Int      int64
	Float    float64
	String   string
}

type UnpackedKind uint8

const (
	UnpackedInt UnpackedKind = iota
	UnpackedFloat
	UnpackedString
)

type UnpackedValue struct {
	Kind   UnpackedKind
	Int    int64
	Float  float64
	String string
}

type ResultError string

func (e ResultError) Error() string {
	return string(e)
}

func Pack(apiName, formatText string, values []PackValue, maxHostResult int64) (string, error) {
	format, err := Parse(formatText)
	if err != nil {
		return "", err
	}
	if len(values) != len(format.Fields) {
		return "", fmt.Errorf("%s: got %d values for %d fields", apiName, len(values), len(format.Fields))
	}
	buf, _ := outputlimit.NewBuffers(maxHostResult)
	for i, field := range format.Fields {
		if err := packField(apiName, buf, format, field, values[i]); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func Unpack(apiName, formatText, data string, offset int) ([]UnpackedValue, int, error) {
	format, err := Parse(formatText)
	if err != nil {
		return nil, 0, err
	}
	if offset < 1 {
		return nil, 0, ResultError(apiName + ": offset out of range")
	}
	pos := offset - 1
	buf := []byte(data)
	results := make([]UnpackedValue, 0, len(format.Fields))
	for _, field := range format.Fields {
		v, next, err := unpackField(apiName, buf, pos, format, field)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, v)
		pos = next
	}
	return results, pos + 1, nil
}

func Size(apiName, formatText string) (int, bool, error) {
	format, err := Parse(formatText)
	if err != nil {
		return 0, false, err
	}
	total := 0
	for _, field := range format.Fields {
		n, fixed := FieldSize(field)
		if !fixed {
			return 0, false, nil
		}
		total += n
	}
	return total, true, nil
}

func packField(apiName string, buf interface {
	Write([]byte) (int, error)
}, format Format, field Field, value PackValue) error {
	switch field.Kind {
	case "i8", "int8":
		n, err := requireSigned(apiName, value, -128, 127)
		if err != nil {
			return err
		}
		_, err = buf.Write([]byte{byte(int8(n))})
		return err
	case "u8", "uint8":
		n, err := requireUnsigned(apiName, value, math.MaxUint8)
		if err != nil {
			return err
		}
		_, err = buf.Write([]byte{byte(n)})
		return err
	case "i16", "int16":
		n, err := requireSigned(apiName, value, math.MinInt16, math.MaxInt16)
		if err != nil {
			return err
		}
		b := make([]byte, 2)
		format.Order.PutUint16(b, uint16(int16(n)))
		_, err = buf.Write(b)
		return err
	case "u16", "uint16":
		n, err := requireUnsigned(apiName, value, math.MaxUint16)
		if err != nil {
			return err
		}
		b := make([]byte, 2)
		format.Order.PutUint16(b, uint16(n))
		_, err = buf.Write(b)
		return err
	case "i32", "int32":
		n, err := requireSigned(apiName, value, math.MinInt32, math.MaxInt32)
		if err != nil {
			return err
		}
		b := make([]byte, 4)
		format.Order.PutUint32(b, uint32(int32(n)))
		_, err = buf.Write(b)
		return err
	case "u32", "uint32":
		n, err := requireUnsigned(apiName, value, math.MaxUint32)
		if err != nil {
			return err
		}
		b := make([]byte, 4)
		format.Order.PutUint32(b, uint32(n))
		_, err = buf.Write(b)
		return err
	case "i64", "int64":
		n, err := requireSigned(apiName, value, math.MinInt64, math.MaxInt64)
		if err != nil {
			return err
		}
		b := make([]byte, 8)
		format.Order.PutUint64(b, uint64(n))
		_, err = buf.Write(b)
		return err
	case "u64", "uint64":
		n, err := requireUnsigned(apiName, value, math.MaxUint64)
		if err != nil {
			return err
		}
		b := make([]byte, 8)
		format.Order.PutUint64(b, n)
		_, err = buf.Write(b)
		return err
	case "f32", "float32":
		b := make([]byte, 4)
		format.Order.PutUint32(b, math.Float32bits(float32(value.Float)))
		_, err := buf.Write(b)
		return err
	case "f64", "float64":
		b := make([]byte, 8)
		format.Order.PutUint64(b, math.Float64bits(value.Float))
		_, err := buf.Write(b)
		return err
	case "string", "str", "bytes":
		if !value.IsString {
			return fmt.Errorf("%s: string expected for %s", apiName, field.Kind)
		}
		data := []byte(value.String)
		if field.Count >= 0 {
			if len(data) != field.Count {
				return fmt.Errorf("%s: %s:%d got %d bytes", apiName, field.Kind, field.Count, len(data))
			}
			_, err := buf.Write(data)
			return err
		}
		if len(data) > math.MaxUint32 {
			return fmt.Errorf("%s: string too large", apiName)
		}
		b := make([]byte, 4)
		format.Order.PutUint32(b, uint32(len(data)))
		if _, err := buf.Write(b); err != nil {
			return err
		}
		_, err := buf.Write(data)
		return err
	default:
		return fmt.Errorf("binary: unknown field type %q", field.Kind)
	}
}

func unpackField(apiName string, data []byte, pos int, format Format, field Field) (UnpackedValue, int, error) {
	need, fixed := FieldSize(field)
	if fixed {
		if pos < 0 || pos+need > len(data) {
			return UnpackedValue{}, pos, ResultError(apiName + ": data too short")
		}
	}
	switch field.Kind {
	case "i8", "int8":
		return UnpackedValue{Kind: UnpackedInt, Int: int64(int8(data[pos]))}, pos + 1, nil
	case "u8", "uint8":
		return UnpackedValue{Kind: UnpackedInt, Int: int64(data[pos])}, pos + 1, nil
	case "i16", "int16":
		return UnpackedValue{Kind: UnpackedInt, Int: int64(int16(format.Order.Uint16(data[pos:])))}, pos + 2, nil
	case "u16", "uint16":
		return UnpackedValue{Kind: UnpackedInt, Int: int64(format.Order.Uint16(data[pos:]))}, pos + 2, nil
	case "i32", "int32":
		return UnpackedValue{Kind: UnpackedInt, Int: int64(int32(format.Order.Uint32(data[pos:])))}, pos + 4, nil
	case "u32", "uint32":
		return UnpackedValue{Kind: UnpackedInt, Int: int64(format.Order.Uint32(data[pos:]))}, pos + 4, nil
	case "i64", "int64":
		return UnpackedValue{Kind: UnpackedInt, Int: int64(format.Order.Uint64(data[pos:]))}, pos + 8, nil
	case "u64", "uint64":
		u := format.Order.Uint64(data[pos:])
		if u > uint64(maxInt48) {
			return UnpackedValue{Kind: UnpackedFloat, Float: float64(u)}, pos + 8, nil
		}
		return UnpackedValue{Kind: UnpackedInt, Int: int64(u)}, pos + 8, nil
	case "f32", "float32":
		return UnpackedValue{Kind: UnpackedFloat, Float: float64(math.Float32frombits(format.Order.Uint32(data[pos:])))}, pos + 4, nil
	case "f64", "float64":
		return UnpackedValue{Kind: UnpackedFloat, Float: math.Float64frombits(format.Order.Uint64(data[pos:]))}, pos + 8, nil
	case "string", "str", "bytes":
		if field.Count >= 0 {
			return UnpackedValue{Kind: UnpackedString, String: string(data[pos : pos+field.Count])}, pos + field.Count, nil
		}
		if pos+4 > len(data) {
			return UnpackedValue{}, pos, ResultError(apiName + ": data too short")
		}
		n := int(format.Order.Uint32(data[pos:]))
		pos += 4
		if pos+n > len(data) {
			return UnpackedValue{}, pos, ResultError(apiName + ": data too short")
		}
		return UnpackedValue{Kind: UnpackedString, String: string(data[pos : pos+n])}, pos + n, nil
	default:
		return UnpackedValue{}, pos, fmt.Errorf("binary: unknown field type %q", field.Kind)
	}
}

func requireSigned(apiName string, v PackValue, min, max int64) (int64, error) {
	if !v.IsNumber && !v.IsString {
		return 0, fmt.Errorf("%s: number expected", apiName)
	}
	n := v.Int
	if n < min || n > max {
		return 0, fmt.Errorf("%s: value %d out of range [%d, %d]", apiName, n, min, max)
	}
	return n, nil
}

func requireUnsigned(apiName string, v PackValue, max uint64) (uint64, error) {
	if !v.IsNumber && !v.IsString {
		return 0, fmt.Errorf("%s: number expected", apiName)
	}
	if v.IsFloat && v.Float < 0 {
		return 0, fmt.Errorf("%s: negative value for unsigned field", apiName)
	}
	n := v.Int
	if n < 0 {
		return 0, fmt.Errorf("%s: negative value for unsigned field", apiName)
	}
	u := uint64(n)
	if u > max {
		return 0, fmt.Errorf("%s: value %d out of range [0, %d]", apiName, u, max)
	}
	return u, nil
}
