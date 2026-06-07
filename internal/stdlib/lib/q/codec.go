package q

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

const codecVersion = 1

type wireEnvelope struct {
	Schema  string    `json:"schema"`
	Version int       `json:"version"`
	Value   wireValue `json:"value"`
}

type wireValue struct {
	Type       string       `json:"type"`
	Kind       string       `json:"kind,omitempty"`
	Value      any          `json:"value,omitempty"`
	Values     []wireValue  `json:"values,omitempty"`
	Columns    []wireColumn `json:"columns,omitempty"`
	Keys       []string     `json:"keys,omitempty"`
	Attributes []string     `json:"attributes,omitempty"`
	Domain     []wireValue  `json:"domain,omitempty"`
	Codes      []int32      `json:"codes,omitempty"`
	EnumDomain string       `json:"enum_domain,omitempty"`
}

type wireColumn struct {
	Name       string      `json:"name"`
	Kind       string      `json:"kind"`
	Values     []wireValue `json:"values"`
	Attributes []string    `json:"attributes,omitempty"`
	Domain     []wireValue `json:"domain,omitempty"`
	Codes      []int32     `json:"codes,omitempty"`
	EnumDomain string      `json:"enum_domain,omitempty"`
}

// Marshal encodes q/data values into Leia's stable q wire format. It is not
// kdb+ IPC; it is the repository's portable round-trip format for tests,
// replay, local cache, and future interop adapters.
func Marshal(v any) ([]byte, error) {
	w, err := encodeWireValue(v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(wireEnvelope{
		Schema:  "leia.q",
		Version: codecVersion,
		Value:   w,
	})
}

// Unmarshal decodes values produced by Marshal.
func Unmarshal(buf []byte) (any, error) {
	var env wireEnvelope
	if err := json.Unmarshal(buf, &env); err != nil {
		return nil, err
	}
	if env.Schema != "leia.q" {
		return nil, fmt.Errorf("q codec: unsupported schema %q", env.Schema)
	}
	if env.Version != codecVersion {
		return nil, fmt.Errorf("q codec: unsupported version %d", env.Version)
	}
	return decodeWireValue(env.Value)
}

func encodeWireValue(v any) (wireValue, error) {
	if data.IsNull(v) {
		if kind, ok := data.NullKind(v); ok && kind != data.KindNull && kind != data.KindAny {
			return wireValue{Type: "null", Kind: string(kind)}, nil
		}
		return wireValue{Type: "null"}, nil
	}
	switch x := v.(type) {
	case nil:
		return wireValue{Type: "null"}, nil
	case bool:
		return wireValue{Type: "scalar", Kind: string(data.KindBool), Value: x}, nil
	case int8:
		return wireValue{Type: "scalar", Kind: string(data.KindI8), Value: strconv.FormatInt(int64(x), 10)}, nil
	case int16:
		return wireValue{Type: "scalar", Kind: string(data.KindI16), Value: strconv.FormatInt(int64(x), 10)}, nil
	case int32:
		return wireValue{Type: "scalar", Kind: string(data.KindI32), Value: strconv.FormatInt(int64(x), 10)}, nil
	case int:
		return wireValue{Type: "scalar", Kind: string(data.KindI64), Value: strconv.FormatInt(int64(x), 10)}, nil
	case int64:
		return wireValue{Type: "scalar", Kind: string(data.KindI64), Value: strconv.FormatInt(x, 10)}, nil
	case uint8:
		return wireValue{Type: "scalar", Kind: string(data.KindU8), Value: strconv.FormatUint(uint64(x), 10)}, nil
	case uint16:
		return wireValue{Type: "scalar", Kind: string(data.KindU16), Value: strconv.FormatUint(uint64(x), 10)}, nil
	case uint32:
		return wireValue{Type: "scalar", Kind: string(data.KindU32), Value: strconv.FormatUint(uint64(x), 10)}, nil
	case uint64:
		return wireValue{Type: "scalar", Kind: string(data.KindU64), Value: strconv.FormatUint(x, 10)}, nil
	case float32:
		return wireValue{Type: "scalar", Kind: string(data.KindF32), Value: float64(x)}, nil
	case float64:
		return wireValue{Type: "scalar", Kind: string(data.KindF64), Value: x}, nil
	case string:
		return wireValue{Type: "scalar", Kind: string(data.KindString), Value: x}, nil
	case data.Symbol:
		return wireValue{Type: "scalar", Kind: string(data.KindSymbol), Value: string(x)}, nil
	case data.Month, data.Date, data.DateTime, data.Timespan, data.Minute, data.Second, data.Time, data.Timestamp:
		text, ok := FormatTemporal(x)
		if !ok {
			return wireValue{}, fmt.Errorf("q codec: unsupported temporal %T", x)
		}
		return wireValue{Type: "scalar", Kind: string(kindOfDataValue(x)), Value: text}, nil
	case qEnumVector:
		return encodeEnumVector(x)
	case qAttributedVector:
		w, err := encodeArray(x.vector)
		if err != nil {
			return wireValue{}, err
		}
		w.Type = "attributed_vector"
		if !hasWireAttribute(w.Attributes, string(x.attribute)) {
			w.Attributes = append(w.Attributes, string(x.attribute))
		}
		return w, nil
	case data.Array:
		return encodeArray(x)
	case Dict:
		if len(x.Keys) != len(x.Values) {
			return wireValue{}, fmt.Errorf("q codec: dict key/value length mismatch")
		}
		keys := make([]wireValue, len(x.Keys))
		values := make([]wireValue, len(x.Values))
		for i := range x.Keys {
			key, err := encodeWireValue(x.Keys[i])
			if err != nil {
				return wireValue{}, err
			}
			value, err := encodeWireValue(x.Values[i])
			if err != nil {
				return wireValue{}, err
			}
			keys[i] = key
			values[i] = value
		}
		return wireValue{Type: "dict", Keys: nil, Values: values, Columns: []wireColumn{{Name: "keys", Values: keys}}}, nil
	case data.Frame:
		return encodeFrame(x)
	case data.KeyedFrame:
		frame, err := encodeFrame(x.Frame())
		if err != nil {
			return wireValue{}, err
		}
		keys := make([]string, len(x.Keys()))
		for i, key := range x.Keys() {
			keys[i] = string(key)
		}
		frame.Type = "keyed_table"
		frame.Keys = keys
		return frame, nil
	default:
		return wireValue{}, fmt.Errorf("q codec: unsupported %T", v)
	}
}

func encodeArray(array data.Array) (wireValue, error) {
	if enum, ok := array.(qEnumVector); ok {
		return encodeEnumVector(enum)
	}
	domain, encoded := data.EncodedDomainOf(array)
	if encoded {
		codes, _ := data.EncodedCodesOf(array)
		w, err := encodeEncodedArray("encoded_vector", array.Kind(), domain, codes)
		if err != nil {
			return wireValue{}, err
		}
		w.Attributes = encodeArrayAttributes(array)
		return w, nil
	}
	values := make([]wireValue, array.Len())
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok {
			return wireValue{}, fmt.Errorf("q codec: array row %d out of range", i)
		}
		w, err := encodeWireValue(item)
		if err != nil {
			return wireValue{}, err
		}
		values[i] = w
	}
	return wireValue{Type: "vector", Kind: string(array.Kind()), Values: values, Attributes: encodeArrayAttributes(array)}, nil
}

func encodeEnumVector(enum qEnumVector) (wireValue, error) {
	w, err := encodeEncodedArray("enum", enum.Kind(), enum.EncodedDomain(), enum.EncodedCodes())
	if err != nil {
		return wireValue{}, err
	}
	w.EnumDomain = string(enum.domain)
	return w, nil
}

func encodeEncodedArray(typ string, kind data.Kind, domain []any, codes []int32) (wireValue, error) {
	encodedDomain := make([]wireValue, len(domain))
	for i, value := range domain {
		w, err := encodeWireValue(value)
		if err != nil {
			return wireValue{}, err
		}
		encodedDomain[i] = w
	}
	return wireValue{Type: typ, Kind: string(kind), Domain: encodedDomain, Codes: append([]int32(nil), codes...)}, nil
}

func encodeArrayAttributes(array data.Array) []string {
	metadata := data.ArrayMetadataOf(array)
	if len(metadata.Attributes) == 0 {
		return nil
	}
	attrs := make([]string, len(metadata.Attributes))
	for i, attr := range metadata.Attributes {
		attrs[i] = string(attr)
	}
	return attrs
}

func encodeFrame(frame data.Frame) (wireValue, error) {
	names := frame.Schema().Names()
	cols := make([]wireColumn, len(names))
	for i, name := range names {
		array, ok := frame.Column(name)
		if !ok {
			return wireValue{}, fmt.Errorf("q codec: column %q not found", name)
		}
		values := make([]wireValue, array.Len())
		for row := 0; row < array.Len(); row++ {
			item, ok := array.At(row)
			if !ok {
				return wireValue{}, fmt.Errorf("q codec: column %q row %d out of range", name, row)
			}
			w, err := encodeWireValue(item)
			if err != nil {
				return wireValue{}, err
			}
			values[row] = w
		}
		encoded, err := encodeArray(array)
		if err != nil {
			return wireValue{}, err
		}
		cols[i] = wireColumn{
			Name:       string(name),
			Kind:       string(array.Kind()),
			Values:     values,
			Attributes: encoded.Attributes,
			Domain:     encoded.Domain,
			Codes:      encoded.Codes,
			EnumDomain: encoded.EnumDomain,
		}
	}
	return wireValue{Type: "table", Columns: cols}, nil
}

func decodeWireValue(w wireValue) (any, error) {
	switch w.Type {
	case "null":
		if w.Kind != "" {
			return data.NullForKind(data.Kind(w.Kind)), nil
		}
		return data.NullValue, nil
	case "scalar":
		return decodeScalar(data.Kind(w.Kind), w.Value)
	case "vector":
		return decodeWireArray(w)
	case "encoded_vector":
		return decodeWireArray(w)
	case "enum":
		return decodeWireArray(w)
	case "attributed_vector":
		array, err := decodeWireArray(w)
		if err != nil {
			return nil, err
		}
		attrs := decodeArrayAttributes(w.Attributes)
		if len(attrs) == 0 {
			return array, nil
		}
		return qAttributedVector{attribute: attrs[0], vector: array}, nil
	case "dict":
		if len(w.Columns) != 1 || w.Columns[0].Name != "keys" {
			return nil, fmt.Errorf("q codec: dict keys are missing")
		}
		keys, err := decodeWireValues(w.Columns[0].Values)
		if err != nil {
			return nil, err
		}
		values, err := decodeWireValues(w.Values)
		if err != nil {
			return nil, err
		}
		if len(keys) != len(values) {
			return nil, fmt.Errorf("q codec: dict key/value length mismatch")
		}
		return Dict{Keys: keys, Values: values}, nil
	case "table", "keyed_table":
		frame, err := decodeFrame(w)
		if err != nil {
			return nil, err
		}
		if w.Type == "table" {
			return frame, nil
		}
		keys := make([]data.Symbol, len(w.Keys))
		for i, key := range w.Keys {
			keys[i] = data.Symbol(key)
		}
		return data.KeyBy(frame, keys...)
	default:
		return nil, fmt.Errorf("q codec: unsupported type %q", w.Type)
	}
}

func decodeWireArray(w wireValue) (data.Array, error) {
	switch w.Type {
	case "vector", "attributed_vector":
		values, err := decodeWireValues(w.Values)
		if err != nil {
			return nil, err
		}
		col, err := data.NewColumnWithKind("q", data.Kind(w.Kind), values)
		if err != nil {
			return nil, fmt.Errorf("q codec: vector: %w", err)
		}
		return applyArrayAttributes(col.Data, decodeArrayAttributes(w.Attributes)), nil
	case "encoded_vector", "enum":
		domain, err := decodeWireValues(w.Domain)
		if err != nil {
			return nil, err
		}
		array, err := data.NewEncoded(data.Kind(w.Kind), domain, w.Codes)
		if err != nil {
			return nil, fmt.Errorf("q codec: encoded vector: %w", err)
		}
		array = applyArrayAttributes(array, decodeArrayAttributes(w.Attributes))
		if w.Type == "enum" {
			return qEnumVector{domain: data.Symbol(w.EnumDomain), encoded: array}, nil
		}
		return array, nil
	default:
		return nil, fmt.Errorf("q codec: unsupported array type %q", w.Type)
	}
}

func decodeWireValues(values []wireValue) ([]any, error) {
	out := make([]any, len(values))
	for i, value := range values {
		item, err := decodeWireValue(value)
		if err != nil {
			return nil, err
		}
		out[i] = item
	}
	return out, nil
}

func decodeFrame(w wireValue) (data.Frame, error) {
	cols := make([]data.Column, len(w.Columns))
	for i, col := range w.Columns {
		array, err := decodeWireArray(wireValue{
			Type:       columnWireType(col),
			Kind:       col.Kind,
			Values:     col.Values,
			Attributes: col.Attributes,
			Domain:     col.Domain,
			Codes:      col.Codes,
			EnumDomain: col.EnumDomain,
		})
		if err != nil {
			return data.Frame{}, fmt.Errorf("q codec: column %q: %w", col.Name, err)
		}
		cols[i] = data.Column{Name: data.Symbol(col.Name), Data: array}
	}
	return data.NewFrame(cols...)
}

func columnWireType(col wireColumn) string {
	switch {
	case col.EnumDomain != "":
		return "enum"
	case col.Domain != nil || col.Codes != nil:
		return "encoded_vector"
	default:
		return "vector"
	}
}

func decodeArrayAttributes(attrs []string) []data.Symbol {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]data.Symbol, len(attrs))
	for i, attr := range attrs {
		out[i] = data.Symbol(attr)
	}
	return out
}

func applyArrayAttributes(array data.Array, attrs []data.Symbol) data.Array {
	for _, attr := range attrs {
		array = data.WithArrayAttribute(array, attr)
	}
	return array
}

func hasWireAttribute(attrs []string, want string) bool {
	for _, attr := range attrs {
		if attr == want {
			return true
		}
	}
	return false
}

func decodeScalar(kind data.Kind, value any) (any, error) {
	switch kind {
	case data.KindBool:
		v, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("q codec: bool scalar has %T", value)
		}
		return v, nil
	case data.KindI8:
		n, err := decodeSignedScalar(kind, value, 8)
		if err != nil {
			return nil, err
		}
		return int8(n), nil
	case data.KindI16:
		n, err := decodeSignedScalar(kind, value, 16)
		if err != nil {
			return nil, err
		}
		return int16(n), nil
	case data.KindI32:
		n, err := decodeSignedScalar(kind, value, 32)
		if err != nil {
			return nil, err
		}
		return int32(n), nil
	case data.KindI64:
		return decodeSignedScalar(kind, value, 64)
	case data.KindU8:
		n, err := decodeUnsignedScalar(kind, value, 8)
		if err != nil {
			return nil, err
		}
		return uint8(n), nil
	case data.KindU16:
		n, err := decodeUnsignedScalar(kind, value, 16)
		if err != nil {
			return nil, err
		}
		return uint16(n), nil
	case data.KindU32:
		n, err := decodeUnsignedScalar(kind, value, 32)
		if err != nil {
			return nil, err
		}
		return uint32(n), nil
	case data.KindU64:
		return decodeUnsignedScalar(kind, value, 64)
	case data.KindF32:
		v, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("q codec: f32 scalar has %T", value)
		}
		return float32(v), nil
	case data.KindF64:
		v, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("q codec: f64 scalar has %T", value)
		}
		return v, nil
	case data.KindString:
		v, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("q codec: string scalar has %T", value)
		}
		return v, nil
	case data.KindSymbol:
		v, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("q codec: symbol scalar has %T", value)
		}
		return data.Symbol(v), nil
	case data.KindMonth, data.KindDate, data.KindDateTime, data.KindTimespan, data.KindMinute, data.KindSecond, data.KindTime, data.KindTimestamp:
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("q codec: temporal scalar has %T", value)
		}
		return ParseTemporal(string(kind), text)
	default:
		return nil, fmt.Errorf("q codec: unsupported scalar kind %q", kind)
	}
}

func decodeSignedScalar(kind data.Kind, value any, bitSize int) (int64, error) {
	switch x := value.(type) {
	case string:
		return strconv.ParseInt(x, 10, bitSize)
	case float64:
		return int64(x), nil
	default:
		return 0, fmt.Errorf("q codec: %s scalar has %T", kind, value)
	}
}

func decodeUnsignedScalar(kind data.Kind, value any, bitSize int) (uint64, error) {
	switch x := value.(type) {
	case string:
		return strconv.ParseUint(x, 10, bitSize)
	case float64:
		return uint64(x), nil
	default:
		return 0, fmt.Errorf("q codec: %s scalar has %T", kind, value)
	}
}

func kindOfDataValue(v any) data.Kind {
	switch v.(type) {
	case data.Month:
		return data.KindMonth
	case data.Date:
		return data.KindDate
	case data.DateTime:
		return data.KindDateTime
	case data.Timespan:
		return data.KindTimespan
	case data.Minute:
		return data.KindMinute
	case data.Second:
		return data.KindSecond
	case data.Time:
		return data.KindTime
	case data.Timestamp:
		return data.KindTimestamp
	default:
		return data.KindAny
	}
}
