package data

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

const (
	columnarManifestFile  = "_leia_frame.json"
	partitionManifestFile = "_leia_partitions.json"
	columnarFormat        = "leia-columnar-frame"
	partitionedFormat     = "leia-partitioned-frame"
	columnarVersion       = 1
)

type StoredColumn struct {
	Name       string   `json:"name"`
	Kind       Kind     `json:"kind"`
	File       string   `json:"file"`
	Attributes []string `json:"attributes,omitempty"`
	Domain     []any    `json:"domain,omitempty"`
	Codes      []int32  `json:"codes,omitempty"`
}

type FrameStoreInfo struct {
	Format  string         `json:"format"`
	Version int            `json:"version"`
	Rows    int            `json:"rows"`
	Columns []StoredColumn `json:"columns"`
}

type StoredPartition struct {
	Path   string         `json:"path"`
	Rows   int            `json:"rows"`
	Values map[string]any `json:"values"`
}

type PartitionedStoreInfo struct {
	Format           string            `json:"format"`
	Version          int               `json:"version"`
	Rows             int               `json:"rows"`
	PartitionColumns []string          `json:"partition_columns"`
	Columns          []StoredColumn    `json:"columns"`
	Partitions       []StoredPartition `json:"partitions"`
}

func SaveFrameDir(dir string, frame Frame) error {
	if dir == "" {
		return fmt.Errorf("save frame directory must not be empty")
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	info := FrameStoreInfo{
		Format:  columnarFormat,
		Version: columnarVersion,
		Rows:    frame.Len(),
		Columns: make([]StoredColumn, 0, len(frame.schema.names)),
	}
	for _, name := range frame.schema.names {
		col := frame.columns[name]
		file := string(name) + ".json"
		info.Columns = append(info.Columns, storedColumnForArray(name, col, file))
		if err := writeJSON(filepath.Join(dir, file), storedValues(col)); err != nil {
			return fmt.Errorf("save column %q: %w", name, err)
		}
	}
	return writeJSON(filepath.Join(dir, columnarManifestFile), info)
}

func LoadFrameDir(dir string) (Frame, error) {
	info, err := ReadFrameStoreInfo(dir)
	if err != nil {
		return Frame{}, err
	}
	return loadFrameDirWithInfo(dir, info)
}

func ReadFrameStoreInfo(dir string) (FrameStoreInfo, error) {
	var info FrameStoreInfo
	if err := readJSON(filepath.Join(dir, columnarManifestFile), &info); err != nil {
		return FrameStoreInfo{}, err
	}
	if info.Format != columnarFormat {
		return FrameStoreInfo{}, fmt.Errorf("unsupported frame store format %q", info.Format)
	}
	if info.Version != columnarVersion {
		return FrameStoreInfo{}, fmt.Errorf("unsupported frame store version %d", info.Version)
	}
	return info, nil
}

func SavePartitionedFrameDir(dir string, frame Frame, partitionColumns ...Symbol) error {
	if dir == "" {
		return fmt.Errorf("save partitioned frame directory must not be empty")
	}
	if len(partitionColumns) == 0 {
		return fmt.Errorf("partitioned frame requires at least one partition column")
	}
	for _, name := range partitionColumns {
		if _, ok := frame.Column(name); !ok {
			return fmt.Errorf("partition column %q does not exist", name)
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	rowsByKey, valuesByKey, order, err := partitionRows(frame, partitionColumns)
	if err != nil {
		return err
	}
	info := PartitionedStoreInfo{
		Format:           partitionedFormat,
		Version:          columnarVersion,
		Rows:             frame.Len(),
		PartitionColumns: symbolNames(partitionColumns),
		Columns:          storedColumns(frame),
		Partitions:       make([]StoredPartition, 0, len(order)),
	}
	for i, key := range order {
		partDir := "part-" + strconv.Itoa(i+1)
		gathered, err := frame.Gather(rowsByKey[key])
		if err != nil {
			return err
		}
		if err := SaveFrameDir(filepath.Join(dir, partDir), gathered); err != nil {
			return err
		}
		info.Partitions = append(info.Partitions, StoredPartition{
			Path:   partDir,
			Rows:   gathered.Len(),
			Values: valuesByKey[key],
		})
	}
	return writeJSON(filepath.Join(dir, partitionManifestFile), info)
}

func LoadPartitionedFrameDir(dir string, filters map[Symbol]any) (Frame, error) {
	info, err := ReadPartitionedStoreInfo(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return LoadFrameDir(dir)
		}
		return Frame{}, err
	}
	var out Frame
	loaded := false
	for _, part := range info.Partitions {
		if !partitionMatches(part.Values, filters) {
			continue
		}
		frame, err := LoadFrameDir(filepath.Join(dir, part.Path))
		if err != nil {
			return Frame{}, err
		}
		if !loaded {
			out = frame
			loaded = true
			continue
		}
		out, err = AppendFrames(out, frame)
		if err != nil {
			return Frame{}, err
		}
	}
	if loaded {
		return out, nil
	}
	return emptyFrameFromStoredColumns(info.Columns)
}

func ReadPartitionedStoreInfo(dir string) (PartitionedStoreInfo, error) {
	var info PartitionedStoreInfo
	if err := readJSON(filepath.Join(dir, partitionManifestFile), &info); err != nil {
		return PartitionedStoreInfo{}, err
	}
	if info.Format != partitionedFormat {
		return PartitionedStoreInfo{}, fmt.Errorf("unsupported partitioned frame format %q", info.Format)
	}
	if info.Version != columnarVersion {
		return PartitionedStoreInfo{}, fmt.Errorf("unsupported partitioned frame version %d", info.Version)
	}
	return info, nil
}

func AppendFrames(left, right Frame) (Frame, error) {
	if !SameSchema(left, right) {
		return Frame{}, fmt.Errorf("append frame schema mismatch")
	}
	cols := make([]Column, 0, len(left.schema.names))
	for _, name := range left.schema.names {
		leftCol := left.columns[name]
		rightCol := right.columns[name]
		values := append(leftCol.Values(), rightCol.Values()...)
		col, err := columnWithKind(name, leftCol.Kind(), values)
		if err != nil {
			return Frame{}, err
		}
		cols = append(cols, col)
	}
	return NewFrame(cols...)
}

func loadFrameDirWithInfo(dir string, info FrameStoreInfo) (Frame, error) {
	cols := make([]Column, 0, len(info.Columns))
	for _, stored := range info.Columns {
		var raw []any
		if err := readJSON(filepath.Join(dir, stored.File), &raw); err != nil {
			return Frame{}, fmt.Errorf("load column %q: %w", stored.Name, err)
		}
		values, err := decodeStoredValues(stored.Kind, raw)
		if err != nil {
			return Frame{}, fmt.Errorf("load column %q: %w", stored.Name, err)
		}
		if stored.Domain != nil || stored.Codes != nil {
			domain, err := decodeStoredValues(stored.Kind, stored.Domain)
			if err != nil {
				return Frame{}, fmt.Errorf("load column %q domain: %w", stored.Name, err)
			}
			array, err := NewEncoded(stored.Kind, domain, stored.Codes)
			if err != nil {
				return Frame{}, fmt.Errorf("load column %q encoded: %w", stored.Name, err)
			}
			cols = append(cols, Column{Name: Symbol(stored.Name), Data: applyStoredAttributes(array, stored.Attributes)})
			continue
		}
		col, err := columnWithKind(Symbol(stored.Name), stored.Kind, values)
		if err != nil {
			return Frame{}, err
		}
		col.Data = applyStoredAttributes(col.Data, stored.Attributes)
		cols = append(cols, col)
	}
	frame, err := NewFrame(cols...)
	if err != nil {
		return Frame{}, err
	}
	if frame.Len() != info.Rows {
		return Frame{}, fmt.Errorf("frame row count %d does not match manifest %d", frame.Len(), info.Rows)
	}
	return frame, nil
}

func partitionRows(frame Frame, columns []Symbol) (map[string][]int, map[string]map[string]any, []string, error) {
	rowsByKey := make(map[string][]int)
	valuesByKey := make(map[string]map[string]any)
	var order []string
	for row := 0; row < frame.Len(); row++ {
		values := make(map[string]any, len(columns))
		keyParts := make([]any, len(columns))
		for i, name := range columns {
			col, _ := frame.Column(name)
			value, ok := col.At(row)
			if !ok {
				return nil, nil, nil, fmt.Errorf("partition column %q row %d out of range", name, row)
			}
			stored := storedScalar(value)
			values[string(name)] = stored
			keyParts[i] = stored
		}
		keyBytes, err := json.Marshal(keyParts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("encode partition key: %w", err)
		}
		key := string(keyBytes)
		if _, ok := rowsByKey[key]; !ok {
			order = append(order, key)
			valuesByKey[key] = values
		}
		rowsByKey[key] = append(rowsByKey[key], row)
	}
	return rowsByKey, valuesByKey, order, nil
}

func partitionMatches(values map[string]any, filters map[Symbol]any) bool {
	for name, want := range filters {
		got, ok := values[string(name)]
		if !ok {
			continue
		}
		if fmt.Sprint(got) != fmt.Sprint(storedScalar(want)) {
			return false
		}
	}
	return true
}

func emptyFrameFromStoredColumns(columns []StoredColumn) (Frame, error) {
	cols := make([]Column, 0, len(columns))
	for _, stored := range columns {
		col, err := columnWithKind(Symbol(stored.Name), stored.Kind, nil)
		if err != nil {
			return Frame{}, err
		}
		cols = append(cols, col)
	}
	return NewFrame(cols...)
}

func storedColumns(frame Frame) []StoredColumn {
	out := make([]StoredColumn, 0, len(frame.schema.names))
	for _, name := range frame.schema.names {
		col := frame.columns[name]
		out = append(out, storedColumnForArray(name, col, string(name)+".json"))
	}
	return out
}

func storedColumnForArray(name Symbol, array Array, file string) StoredColumn {
	stored := StoredColumn{Name: string(name), Kind: array.Kind(), File: file}
	metadata := ArrayMetadataOf(array)
	if len(metadata.Attributes) > 0 {
		stored.Attributes = make([]string, len(metadata.Attributes))
		for i, attr := range metadata.Attributes {
			stored.Attributes[i] = string(attr)
		}
	}
	if domain, codes, ok := storedEncodedSidecar(array); ok {
		stored.Domain = storedValues(NewAny(domain))
		stored.Codes = append([]int32(nil), codes...)
	}
	return stored
}

func applyStoredAttributes(array Array, attrs []string) Array {
	if _, ok := array.(EncodedArrayInfo); ok && len(attrs) > 0 {
		metadata := ArrayMetadata{Attributes: make([]Symbol, 0, len(attrs))}
		for _, attr := range attrs {
			symbol := Symbol(attr)
			metadata.Attributes = append(metadata.Attributes, symbol)
			if symbol == ArrayAttributeGrouped || symbol == ArrayAttributeUnique {
				if index, err := BuildArrayIndex(array, symbol); err == nil {
					if metadata.Indexes == nil {
						metadata.Indexes = make(map[Symbol]ArrayIndex, 1)
					}
					metadata.Indexes[symbol] = index
				}
			}
		}
		return storedAttributedEncodedArray{array: array, metadata: metadata}
	}
	for _, attr := range attrs {
		array = WithArrayAttribute(array, Symbol(attr))
	}
	return array
}

func storedEncodedSidecar(array Array) ([]any, []int32, bool) {
	if encoded, ok := array.(EncodedArrayInfo); ok {
		return encoded.EncodedDomain(), encoded.EncodedCodes(), true
	}
	if attributed, ok := array.(attributedArray); ok {
		if encoded, ok := attributed.array.(EncodedArrayInfo); ok {
			return encoded.EncodedDomain(), encoded.EncodedCodes(), true
		}
	}
	return nil, nil, false
}

type storedAttributedEncodedArray struct {
	array    Array
	metadata ArrayMetadata
}

func (a storedAttributedEncodedArray) Kind() Kind { return a.array.Kind() }

func (a storedAttributedEncodedArray) Len() int { return a.array.Len() }

func (a storedAttributedEncodedArray) At(row int) (any, bool) { return a.array.At(row) }

func (a storedAttributedEncodedArray) Values() []any { return a.array.Values() }

func (a storedAttributedEncodedArray) Gather(indexes []int) Array {
	gathered := a.array.Gather(indexes)
	return storedAttributedEncodedArray{array: gathered, metadata: a.metadata.cloneWithRebuiltIndexes(gathered)}
}

func (a storedAttributedEncodedArray) ArrayMetadata() ArrayMetadata {
	return a.metadata.clone()
}

func (a storedAttributedEncodedArray) EncodedDomain() []any {
	if encoded, ok := a.array.(EncodedArrayInfo); ok {
		return encoded.EncodedDomain()
	}
	return nil
}

func (a storedAttributedEncodedArray) EncodedCodes() []int32 {
	if encoded, ok := a.array.(EncodedArrayInfo); ok {
		return encoded.EncodedCodes()
	}
	return nil
}

func symbolNames(symbols []Symbol) []string {
	out := make([]string, len(symbols))
	for i, symbol := range symbols {
		out[i] = string(symbol)
	}
	return out
}

func storedValues(array Array) []any {
	values := array.Values()
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = storedScalar(value)
	}
	return out
}

func storedScalar(value any) any {
	switch x := value.(type) {
	case nil:
		return nil
	case Null:
		return nil
	case Symbol:
		return string(x)
	case Month:
		return int64(x)
	case Date:
		return int64(x)
	case DateTime:
		return int64(x)
	case Timespan:
		return int64(x)
	case Minute:
		return int64(x)
	case Second:
		return int64(x)
	case Time:
		return int64(x)
	case Timestamp:
		return int64(x)
	default:
		return x
	}
}

func decodeStoredValues(kind Kind, raw []any) ([]any, error) {
	out := make([]any, len(raw))
	for i, value := range raw {
		decoded, err := decodeStoredScalar(kind, value)
		if err != nil {
			return nil, fmt.Errorf("value %d: %w", i+1, err)
		}
		out[i] = decoded
	}
	return out, nil
}

func decodeStoredScalar(kind Kind, value any) (any, error) {
	if value == nil {
		return NullValue, nil
	}
	switch kind {
	case KindBool:
		b, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool")
		}
		return b, nil
	case KindI8, KindI16, KindI32, KindI64:
		return jsonNumberInt64(value)
	case KindU8, KindU16, KindU32, KindU64:
		return jsonNumberUint64(value)
	case KindF32:
		n, err := jsonNumberFloat64(value)
		if err != nil {
			return nil, err
		}
		return float32(n), nil
	case KindF64:
		return jsonNumberFloat64(value)
	case KindString:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string")
		}
		return s, nil
	case KindSymbol:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected symbol string")
		}
		return Symbol(s), nil
	case KindMonth:
		n, err := jsonNumberInt64(value)
		return Month(n), err
	case KindDate:
		n, err := jsonNumberInt64(value)
		return Date(n), err
	case KindDateTime:
		n, err := jsonNumberInt64(value)
		return DateTime(n), err
	case KindTimespan:
		n, err := jsonNumberInt64(value)
		return Timespan(n), err
	case KindMinute:
		n, err := jsonNumberInt64(value)
		return Minute(n), err
	case KindSecond:
		n, err := jsonNumberInt64(value)
		return Second(n), err
	case KindTime:
		n, err := jsonNumberInt64(value)
		return Time(n), err
	case KindTimestamp:
		n, err := jsonNumberInt64(value)
		return Timestamp(n), err
	default:
		return value, nil
	}
}

func jsonNumberInt64(value any) (int64, error) {
	switch n := value.(type) {
	case json.Number:
		return n.Int64()
	case float64:
		return int64(n), nil
	default:
		return 0, fmt.Errorf("expected number")
	}
}

func jsonNumberUint64(value any) (uint64, error) {
	switch n := value.(type) {
	case json.Number:
		return strconv.ParseUint(n.String(), 10, 64)
	case float64:
		return uint64(n), nil
	default:
		return 0, fmt.Errorf("expected number")
	}
}

func jsonNumberFloat64(value any) (float64, error) {
	switch n := value.(type) {
	case json.Number:
		return n.Float64()
	case float64:
		return n, nil
	default:
		return 0, fmt.Errorf("expected number")
	}
}

func coerceSymbol(value any) (Symbol, bool) {
	switch x := value.(type) {
	case Symbol:
		return x, true
	case string:
		return Symbol(x), true
	default:
		return "", false
	}
}

func writeJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func readJSON(path string, out any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	dec := json.NewDecoder(file)
	dec.UseNumber()
	if err := dec.Decode(out); err != nil {
		if err == io.EOF {
			return fmt.Errorf("empty json file %s", path)
		}
		return err
	}
	return nil
}
