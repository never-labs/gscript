package data

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// QueryKernel is a schema-stable executable query plan.
//
// It covers the typed q hot paths that can be described by a stable schema and
// plan fingerprint, while keeping QueryPlan.Exec as the semantic fallback for
// unsupported qSQL shapes.
// The kernel stores a source-free plan so it can be reused across frames with
// the same schema and compatible literal bindings.
type QueryKernel struct {
	plan          QueryPlan
	schema        Schema
	reason        string
	shape         string
	pipelineShape string
}

// QueryKernelPipelineDescriptor is the schema-stable lowering contract between
// qSQL planning and typed runtime/JIT backends. Family fields intentionally use
// low-cardinality names; Ops fields preserve coarse operator counts for runtime
// selection without baking in source text, column names, or literal values.
type QueryKernelPipelineDescriptor struct {
	Scan             string
	WhereFamily      string
	WhereOps         string
	FilterFamily     string
	GroupFamily      string
	GroupOps         string
	AggregateFamily  string
	AggregateOps     string
	ProjectionFamily string
	ProjectionOps    string
	OrderFamily      string
	DistinctFamily   string
	LimitFamily      string
}

// String returns the compact pipeline shape used by existing cache/explain
// surfaces. It is derived from the structured descriptor so future backends can
// consume either the typed fields or the stable string.
func (d QueryKernelPipelineDescriptor) String() string {
	stages := []string{"scan=frame"}
	if d.WhereFamily != "" {
		stages = append(stages, "where="+joinPipelineFamilyOps(d.WhereFamily, d.WhereOps))
		if d.FilterFamily != "" {
			stages = append(stages, "filter="+d.FilterFamily)
		}
	}
	if d.GroupFamily != "" {
		stages = append(stages, "group="+joinPipelineFamilyOps(d.GroupFamily, d.GroupOps))
	}
	if d.AggregateFamily != "" {
		stages = append(stages, "aggregate="+joinPipelineFamilyOps(d.AggregateFamily, d.AggregateOps))
	}
	if d.ProjectionFamily != "" {
		stages = append(stages, "project="+joinPipelineFamilyOps(d.ProjectionFamily, d.ProjectionOps))
	}
	if d.OrderFamily != "" {
		stages = append(stages, "order="+d.OrderFamily)
	}
	if d.DistinctFamily != "" {
		stages = append(stages, "distinct="+d.DistinctFamily)
	}
	if d.LimitFamily != "" {
		stages = append(stages, "limit="+d.LimitFamily)
	}
	return strings.Join(stages, "|")
}

func joinPipelineFamilyOps(family, ops string) string {
	if ops == "" || ops == family {
		return family
	}
	return family + "(" + ops + ")"
}

// SchemaStableCacheKeyParts is the decoded representation of cache keys built
// by FrameSchemaCacheKey, QueryAlignedPlanCacheKey, QueryAlignedMutationCacheKey
// and QueryKernelCacheKey.
type SchemaStableCacheKeyParts struct {
	Namespace  string
	Kind       string
	SchemaHash string
	Extra      []string
}

// FrameSchemaCacheKey returns a cache key for artifacts whose validity depends
// on source identity plus frame schema, but not row values.
func FrameSchemaCacheKey(namespace string, frame Frame) string {
	return querySchemaStableCacheKey(namespace, "schema", frame)
}

// QueryKernelCacheKey returns the schema-stable cache key for a query kernel.
// The optional namespace lets callers include source identity while keeping the
// schema and plan fingerprint logic in the data runtime boundary.
func QueryKernelCacheKey(namespace string, frame Frame, plan QueryPlan) string {
	return querySchemaStableCacheKey(namespace, "kernel", frame, QueryKernelPlanFingerprint(plan))
}

// QueryAlignedPlanCacheKey returns the schema-stable cache key for a query plan
// aligned to a frame schema.
func QueryAlignedPlanCacheKey(namespace string, frame Frame) string {
	return querySchemaStableCacheKey(namespace, "plan", frame)
}

// QueryAlignedMutationCacheKey returns the schema-stable cache key for a q
// mutation plan aligned to a frame schema.
func QueryAlignedMutationCacheKey(namespace string, frame Frame) string {
	return querySchemaStableCacheKey(namespace, "mutation", frame)
}

func querySchemaStableCacheKey(namespace, kind string, frame Frame, extra ...string) string {
	var b strings.Builder
	writeQueryCacheKeyPart(&b, namespace)
	writeQueryCacheKeyPart(&b, kind)
	writeQueryCacheKeyPart(&b, frame.SchemaFingerprint())
	for _, part := range extra {
		writeQueryCacheKeyPart(&b, part)
	}
	return b.String()
}

func writeQueryCacheKeyPart(b *strings.Builder, part string) {
	b.WriteString(strconv.Itoa(len(part)))
	b.WriteByte(':')
	b.WriteString(part)
	b.WriteByte(';')
}

// ParseSchemaStableCacheKey decodes a schema-stable cache key produced by this
// package. It returns false for malformed keys or keys with fewer than the
// namespace/kind/schema parts.
func ParseSchemaStableCacheKey(key string) (SchemaStableCacheKeyParts, bool) {
	raw, ok := parseQueryCacheKeyParts(key)
	if !ok || len(raw) < 3 {
		return SchemaStableCacheKeyParts{}, false
	}
	return SchemaStableCacheKeyParts{
		Namespace:  raw[0],
		Kind:       raw[1],
		SchemaHash: raw[2],
		Extra:      append([]string(nil), raw[3:]...),
	}, true
}

func parseQueryCacheKeyParts(key string) ([]string, bool) {
	var parts []string
	for len(key) > 0 {
		colon := strings.IndexByte(key, ':')
		if colon <= 0 {
			return nil, false
		}
		n, err := strconv.Atoi(key[:colon])
		if err != nil || n < 0 {
			return nil, false
		}
		start := colon + 1
		end := start + n
		if end >= len(key) || key[end] != ';' {
			return nil, false
		}
		parts = append(parts, key[start:end])
		key = key[end+1:]
	}
	return parts, true
}

// QueryKernelPlanFingerprint returns a stable structural fingerprint for the
// subset of QueryPlan shape that affects QueryKernel execution.
func QueryKernelPlanFingerprint(plan QueryPlan) string {
	var b strings.Builder
	writeQueryKernelFingerprintPart(&b, "plan")
	writeQueryKernelFingerprintPart(&b, "distinct")
	writeQueryKernelFingerprintPart(&b, strconv.FormatBool(plan.Distinct))
	writeQueryKernelFingerprintPart(&b, "where")
	writeQueryKernelExprFingerprint(&b, plan.Where)
	writeQueryKernelFingerprintPart(&b, "by")
	writeQueryKernelSymbolsFingerprint(&b, plan.By)
	writeQueryKernelFingerprintPart(&b, "by_exprs")
	writeQueryKernelSelectFingerprint(&b, plan.ByExprs)
	writeQueryKernelFingerprintPart(&b, "select")
	writeQueryKernelSelectFingerprint(&b, plan.Select)
	writeQueryKernelFingerprintPart(&b, "aggregates")
	writeQueryKernelFingerprintPart(&b, strconv.Itoa(len(plan.Aggregates)))
	for _, agg := range plan.Aggregates {
		writeQueryKernelFingerprintPart(&b, string(agg.Name))
		writeQueryKernelFingerprintPart(&b, agg.Func)
		writeQueryKernelExprFingerprint(&b, agg.Expr)
		writeQueryKernelFingerprintPart(&b, strconv.FormatBool(agg.Weight != nil))
		if agg.Weight != nil {
			writeQueryKernelExprFingerprint(&b, agg.Weight)
		}
	}
	writeQueryKernelFingerprintPart(&b, "order")
	writeQueryKernelFingerprintPart(&b, strconv.Itoa(len(plan.OrderBy)))
	for _, order := range plan.OrderBy {
		writeQueryKernelFingerprintPart(&b, string(order.Column))
		writeQueryKernelFingerprintPart(&b, strconv.FormatBool(order.Desc))
	}
	writeQueryKernelFingerprintPart(&b, "pre")
	writeQueryKernelFingerprintPart(&b, strconv.FormatBool(plan.PreProjectOrder))
	writeQueryKernelFingerprintPart(&b, "limit")
	writeQueryKernelFingerprintPart(&b, strconv.Itoa(plan.LimitN))
	return b.String()
}

func writeQueryKernelFingerprintPart(b *strings.Builder, part string) {
	writeQueryCacheKeyPart(b, part)
}

func writeQueryKernelSymbolsFingerprint(b *strings.Builder, symbols []Symbol) {
	writeQueryKernelFingerprintPart(b, strconv.Itoa(len(symbols)))
	for _, sym := range symbols {
		writeQueryKernelFingerprintPart(b, string(sym))
	}
}

func writeQueryKernelSelectFingerprint(b *strings.Builder, items []SelectItem) {
	writeQueryKernelFingerprintPart(b, strconv.Itoa(len(items)))
	for _, item := range items {
		writeQueryKernelFingerprintPart(b, string(item.Name))
		writeQueryKernelExprFingerprint(b, item.Expr)
	}
}

func writeQueryKernelExprFingerprint(b *strings.Builder, expr Expr) {
	writeQueryKernelExprFingerprintWithSeen(b, expr, nil)
}

func writeQueryKernelExprFingerprintWithSeen(b *strings.Builder, expr Expr, seen map[queryKernelSliceKey]struct{}) {
	switch e := expr.(type) {
	case nil:
		writeQueryKernelFingerprintPart(b, "nil")
	case ColumnRef:
		writeQueryKernelFingerprintPart(b, "col")
		writeQueryKernelFingerprintPart(b, string(e.Name))
	case Literal:
		writeQueryKernelFingerprintPart(b, "lit")
		writeQueryKernelLiteralFingerprintWithSeen(b, e.Value, seen)
	case Binary:
		writeQueryKernelFingerprintPart(b, "binary")
		writeQueryKernelFingerprintPart(b, string(e.Op))
		writeQueryKernelExprFingerprintWithSeen(b, e.Left, seen)
		writeQueryKernelExprFingerprintWithSeen(b, e.Right, seen)
	case Logical:
		writeQueryKernelFingerprintPart(b, "logical")
		writeQueryKernelFingerprintPart(b, e.Op)
		writeQueryKernelExprFingerprintWithSeen(b, e.Left, seen)
		writeQueryKernelExprFingerprintWithSeen(b, e.Right, seen)
	case Not:
		writeQueryKernelFingerprintPart(b, "not")
		writeQueryKernelExprFingerprintWithSeen(b, e.Expr, seen)
	case Conditional:
		writeQueryKernelFingerprintPart(b, "cond")
		writeQueryKernelExprFingerprintWithSeen(b, e.Cond, seen)
		writeQueryKernelExprFingerprintWithSeen(b, e.Then, seen)
		writeQueryKernelExprFingerprintWithSeen(b, e.Else, seen)
	case In:
		writeQueryKernelFingerprintPart(b, "in")
		writeQueryKernelExprFingerprintWithSeen(b, e.Expr, seen)
		writeQueryKernelFingerprintPart(b, strconv.Itoa(len(e.Values)))
		for _, value := range e.Values {
			writeQueryKernelLiteralFingerprintWithSeen(b, value, seen)
		}
	case Within:
		writeQueryKernelFingerprintPart(b, "within")
		writeQueryKernelExprFingerprintWithSeen(b, e.Expr, seen)
		writeQueryKernelLiteralFingerprintWithSeen(b, e.Low, seen)
		writeQueryKernelLiteralFingerprintWithSeen(b, e.High, seen)
		writeQueryKernelFingerprintPart(b, strconv.FormatBool(e.HighClosed))
	case BucketFloorExpr:
		writeQueryKernelFingerprintPart(b, "bucket")
		writeQueryKernelLiteralFingerprintWithSeen(b, e.Interval, seen)
		writeQueryKernelExprFingerprintWithSeen(b, e.Expr, seen)
	case ListAggregateExpr:
		writeQueryKernelFingerprintPart(b, "listagg")
		writeQueryKernelFingerprintPart(b, e.Func)
		writeQueryKernelExprFingerprintWithSeen(b, e.Expr, seen)
	case VectorTransformExpr:
		writeQueryKernelFingerprintPart(b, "vector")
		writeQueryKernelFingerprintPart(b, e.Func)
		writeQueryKernelExprFingerprintWithSeen(b, e.Expr, seen)
		writeQueryKernelFingerprintPart(b, strconv.FormatBool(e.Arg != nil))
		if e.Arg != nil {
			writeQueryKernelExprFingerprintWithSeen(b, e.Arg, seen)
		}
	default:
		writeQueryKernelFingerprintPart(b, "unsupported")
		writeQueryKernelFingerprintPart(b, fmt.Sprintf("%T", expr))
		writeQueryKernelFingerprintPart(b, fmt.Sprintf("%#v", expr))
	}
}

func writeQueryKernelLiteralFingerprint(b *strings.Builder, value any) {
	writeQueryKernelLiteralFingerprintWithSeen(b, value, nil)
}

func writeQueryKernelLiteralFingerprintWithSeen(b *strings.Builder, value any, seen map[queryKernelSliceKey]struct{}) {
	if IsNull(value) {
		writeQueryKernelFingerprintPart(b, "null")
		if kind, ok := NullKind(value); ok {
			writeQueryKernelFingerprintPart(b, string(kind))
		} else {
			writeQueryKernelFingerprintPart(b, "")
		}
		return
	}
	switch x := value.(type) {
	case string:
		writeQueryKernelFingerprintPart(b, "string")
		writeQueryKernelFingerprintPart(b, x)
	case Symbol:
		writeQueryKernelFingerprintPart(b, "symbol")
		writeQueryKernelFingerprintPart(b, string(x))
	case bool:
		writeQueryKernelFingerprintPart(b, "bool")
		writeQueryKernelFingerprintPart(b, strconv.FormatBool(x))
	case int:
		writeQueryKernelFingerprintPart(b, "int")
		writeQueryKernelFingerprintPart(b, strconv.FormatInt(int64(x), 10))
	case int8:
		writeQueryKernelFingerprintPart(b, "i8")
		writeQueryKernelFingerprintPart(b, strconv.FormatInt(int64(x), 10))
	case int16:
		writeQueryKernelFingerprintPart(b, "i16")
		writeQueryKernelFingerprintPart(b, strconv.FormatInt(int64(x), 10))
	case int32:
		writeQueryKernelFingerprintPart(b, "i32")
		writeQueryKernelFingerprintPart(b, strconv.FormatInt(int64(x), 10))
	case int64:
		writeQueryKernelFingerprintPart(b, "i64")
		writeQueryKernelFingerprintPart(b, strconv.FormatInt(x, 10))
	case Month:
		writeQueryKernelFingerprintPart(b, string(KindMonth))
		writeQueryKernelFingerprintPart(b, strconv.FormatInt(int64(x), 10))
	case Date:
		writeQueryKernelFingerprintPart(b, string(KindDate))
		writeQueryKernelFingerprintPart(b, strconv.FormatInt(int64(x), 10))
	case DateTime:
		writeQueryKernelFingerprintPart(b, string(KindDateTime))
		writeQueryKernelFingerprintPart(b, strconv.FormatInt(int64(x), 10))
	case Timespan:
		writeQueryKernelFingerprintPart(b, string(KindTimespan))
		writeQueryKernelFingerprintPart(b, strconv.FormatInt(int64(x), 10))
	case Minute:
		writeQueryKernelFingerprintPart(b, string(KindMinute))
		writeQueryKernelFingerprintPart(b, strconv.FormatInt(int64(x), 10))
	case Second:
		writeQueryKernelFingerprintPart(b, string(KindSecond))
		writeQueryKernelFingerprintPart(b, strconv.FormatInt(int64(x), 10))
	case Time:
		writeQueryKernelFingerprintPart(b, string(KindTime))
		writeQueryKernelFingerprintPart(b, strconv.FormatInt(int64(x), 10))
	case Timestamp:
		writeQueryKernelFingerprintPart(b, string(KindTimestamp))
		writeQueryKernelFingerprintPart(b, strconv.FormatInt(int64(x), 10))
	case uint8:
		writeQueryKernelFingerprintPart(b, "u8")
		writeQueryKernelFingerprintPart(b, strconv.FormatUint(uint64(x), 10))
	case uint16:
		writeQueryKernelFingerprintPart(b, "u16")
		writeQueryKernelFingerprintPart(b, strconv.FormatUint(uint64(x), 10))
	case uint32:
		writeQueryKernelFingerprintPart(b, "u32")
		writeQueryKernelFingerprintPart(b, strconv.FormatUint(uint64(x), 10))
	case uint64:
		writeQueryKernelFingerprintPart(b, "u64")
		writeQueryKernelFingerprintPart(b, strconv.FormatUint(x, 10))
	case float32:
		writeQueryKernelFingerprintPart(b, "f32")
		writeQueryKernelFloatFingerprint(b, float64(x), 32)
	case float64:
		writeQueryKernelFingerprintPart(b, "f64")
		writeQueryKernelFloatFingerprint(b, x, 64)
	case []any:
		key, ok := queryKernelSliceKeyForValue(reflect.ValueOf(x))
		if ok {
			if _, exists := seen[key]; exists {
				writeQueryKernelFingerprintPart(b, "recursive_list")
				writeQueryKernelFingerprintPart(b, key.typ)
				return
			}
			if seen == nil {
				seen = make(map[queryKernelSliceKey]struct{})
			}
			seen[key] = struct{}{}
			defer delete(seen, key)
		}
		writeQueryKernelFingerprintPart(b, "list")
		writeQueryKernelFingerprintPart(b, strconv.FormatBool(x == nil))
		writeQueryKernelFingerprintPart(b, strconv.Itoa(len(x)))
		for _, item := range x {
			writeQueryKernelLiteralFingerprintWithSeen(b, item, seen)
		}
	default:
		if writeQueryKernelLiteralSliceFingerprint(b, value, seen) {
			return
		}
		if writeQueryKernelLiteralArrayFingerprint(b, value, seen) {
			return
		}
		if writeQueryKernelLiteralMapFingerprint(b, value, seen) {
			return
		}
		if writeQueryKernelLiteralStructFingerprint(b, value, seen) {
			return
		}
		if writeQueryKernelLiteralPointerFingerprint(b, value, seen) {
			return
		}
		writeQueryKernelFingerprintPart(b, fmt.Sprintf("%T", value))
		writeQueryKernelFingerprintPart(b, fmt.Sprintf("%#v", value))
	}
}

type queryKernelSliceKey struct {
	typ string
	ptr uintptr
}

func queryKernelSliceKeyForValue(v reflect.Value) (queryKernelSliceKey, bool) {
	if !v.IsValid() || (v.Kind() != reflect.Slice && v.Kind() != reflect.Map && v.Kind() != reflect.Pointer) || v.IsNil() {
		return queryKernelSliceKey{}, false
	}
	return queryKernelSliceKey{typ: v.Type().String(), ptr: v.Pointer()}, true
}

func writeQueryKernelLiteralSliceFingerprint(b *strings.Builder, value any, seen map[queryKernelSliceKey]struct{}) bool {
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.Slice {
		return false
	}
	key, ok := queryKernelSliceKeyForValue(v)
	if ok {
		if _, exists := seen[key]; exists {
			writeQueryKernelFingerprintPart(b, "recursive_slice")
			writeQueryKernelFingerprintPart(b, key.typ)
			return true
		}
		if seen == nil {
			seen = make(map[queryKernelSliceKey]struct{})
		}
		seen[key] = struct{}{}
		defer delete(seen, key)
	}
	writeQueryKernelFingerprintPart(b, "slice")
	writeQueryKernelFingerprintPart(b, v.Type().String())
	writeQueryKernelFingerprintPart(b, strconv.FormatBool(v.IsNil()))
	writeQueryKernelFingerprintPart(b, strconv.Itoa(v.Len()))
	for i := 0; i < v.Len(); i++ {
		writeQueryKernelLiteralFingerprintWithSeen(b, v.Index(i).Interface(), seen)
	}
	return true
}

func writeQueryKernelLiteralArrayFingerprint(b *strings.Builder, value any, seen map[queryKernelSliceKey]struct{}) bool {
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.Array {
		return false
	}
	writeQueryKernelFingerprintPart(b, "array")
	writeQueryKernelFingerprintPart(b, v.Type().String())
	writeQueryKernelFingerprintPart(b, strconv.Itoa(v.Len()))
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i)
		if !item.CanInterface() {
			writeQueryKernelFingerprintPart(b, item.Type().String())
			writeQueryKernelFingerprintPart(b, fmt.Sprintf("%#v", item))
			continue
		}
		writeQueryKernelLiteralFingerprintWithSeen(b, item.Interface(), seen)
	}
	return true
}

func writeQueryKernelLiteralMapFingerprint(b *strings.Builder, value any, seen map[queryKernelSliceKey]struct{}) bool {
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.Map {
		return false
	}
	key, ok := queryKernelSliceKeyForValue(v)
	if ok {
		if _, exists := seen[key]; exists {
			writeQueryKernelFingerprintPart(b, "recursive_map")
			writeQueryKernelFingerprintPart(b, key.typ)
			return true
		}
		if seen == nil {
			seen = make(map[queryKernelSliceKey]struct{})
		}
		seen[key] = struct{}{}
		defer delete(seen, key)
	}
	writeQueryKernelFingerprintPart(b, "map")
	writeQueryKernelFingerprintPart(b, v.Type().String())
	writeQueryKernelFingerprintPart(b, strconv.FormatBool(v.IsNil()))
	writeQueryKernelFingerprintPart(b, strconv.Itoa(v.Len()))
	entries := make([]string, 0, v.Len())
	for _, mapKey := range v.MapKeys() {
		var entry strings.Builder
		if mapKey.CanInterface() {
			writeQueryKernelLiteralFingerprintWithSeen(&entry, mapKey.Interface(), seen)
		} else {
			writeQueryKernelFingerprintPart(&entry, mapKey.Type().String())
			writeQueryKernelFingerprintPart(&entry, "<unexported>")
		}
		mapValue := v.MapIndex(mapKey)
		if mapValue.CanInterface() {
			writeQueryKernelLiteralFingerprintWithSeen(&entry, mapValue.Interface(), seen)
		} else {
			writeQueryKernelFingerprintPart(&entry, mapValue.Type().String())
			writeQueryKernelFingerprintPart(&entry, "<unexported>")
		}
		entries = append(entries, entry.String())
	}
	sort.Strings(entries)
	for _, entry := range entries {
		writeQueryKernelFingerprintPart(b, entry)
	}
	return true
}

func writeQueryKernelLiteralStructFingerprint(b *strings.Builder, value any, seen map[queryKernelSliceKey]struct{}) bool {
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return false
	}
	t := v.Type()
	writeQueryKernelFingerprintPart(b, "struct")
	writeQueryKernelFingerprintPart(b, t.String())
	writeQueryKernelFingerprintPart(b, strconv.Itoa(v.NumField()))
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		item := v.Field(i)
		writeQueryKernelFingerprintPart(b, field.Name)
		writeQueryKernelFingerprintPart(b, item.Type().String())
		if !item.CanInterface() {
			writeQueryKernelFingerprintPart(b, fmt.Sprintf("%#v", item))
			continue
		}
		writeQueryKernelLiteralFingerprintWithSeen(b, item.Interface(), seen)
	}
	return true
}

func writeQueryKernelLiteralPointerFingerprint(b *strings.Builder, value any, seen map[queryKernelSliceKey]struct{}) bool {
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.Pointer {
		return false
	}
	writeQueryKernelFingerprintPart(b, "ptr")
	writeQueryKernelFingerprintPart(b, v.Type().String())
	writeQueryKernelFingerprintPart(b, strconv.FormatBool(v.IsNil()))
	if v.IsNil() {
		return true
	}
	key, ok := queryKernelSliceKeyForValue(v)
	if ok {
		if _, exists := seen[key]; exists {
			writeQueryKernelFingerprintPart(b, "recursive_ptr")
			writeQueryKernelFingerprintPart(b, key.typ)
			return true
		}
		if seen == nil {
			seen = make(map[queryKernelSliceKey]struct{})
		}
		seen[key] = struct{}{}
		defer delete(seen, key)
	}
	item := v.Elem()
	if !item.CanInterface() {
		writeQueryKernelFingerprintPart(b, item.Type().String())
		writeQueryKernelFingerprintPart(b, "<unexported>")
		return true
	}
	writeQueryKernelLiteralFingerprintWithSeen(b, item.Interface(), seen)
	return true
}

func writeQueryKernelFloatFingerprint(b *strings.Builder, value float64, bitSize int) {
	if math.IsNaN(value) {
		writeQueryKernelFingerprintPart(b, "NaN")
		return
	}
	writeQueryKernelFingerprintPart(b, strconv.FormatFloat(value, 'g', -1, bitSize))
}

// CompileQueryKernel compiles a QueryPlan for repeated execution against
// frames with a stable schema. Unsupported shapes return ok=false so callers
// can fall back to QueryPlan.Exec without changing semantics.
func CompileQueryKernel(frame Frame, plan QueryPlan) (*QueryKernel, bool, error) {
	ok, reason := QueryKernelSupportReason(plan)
	if !ok {
		return nil, false, nil
	}
	compiled := cloneQueryKernelPlan(plan)
	compiled.Source = Frame{}
	if err := validateQueryKernelFrame(frame, compiled); err != nil {
		return nil, true, err
	}
	if frameReason := queryKernelFrameReason(frame, compiled); frameReason != "" {
		reason = frameReason
	}
	return &QueryKernel{
		plan:          compiled,
		schema:        frame.Schema(),
		reason:        reason,
		shape:         QueryKernelPlanShape(compiled),
		pipelineShape: QueryKernelPlanPipelineShape(compiled),
	}, true, nil
}

func cloneQueryKernelPlan(plan QueryPlan) QueryPlan {
	plan.By = append([]Symbol(nil), plan.By...)
	plan.ByExprs = cloneQueryKernelSelectItems(plan.ByExprs)
	plan.Select = cloneQueryKernelSelectItems(plan.Select)
	plan.Aggregates = cloneQueryKernelAggregates(plan.Aggregates)
	plan.OrderBy = append([]OrderSpec(nil), plan.OrderBy...)
	plan.Where = cloneQueryKernelExpr(plan.Where)
	return plan
}

func cloneQueryKernelSelectItems(items []SelectItem) []SelectItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]SelectItem, len(items))
	for i, item := range items {
		out[i] = SelectItem{Name: item.Name, Expr: cloneQueryKernelExpr(item.Expr)}
	}
	return out
}

func cloneQueryKernelAggregates(items []Aggregate) []Aggregate {
	if len(items) == 0 {
		return nil
	}
	out := make([]Aggregate, len(items))
	for i, item := range items {
		out[i] = Aggregate{
			Name:   item.Name,
			Func:   item.Func,
			Expr:   cloneQueryKernelExpr(item.Expr),
			Weight: cloneQueryKernelExpr(item.Weight),
		}
	}
	return out
}

func cloneQueryKernelExpr(expr Expr) Expr {
	switch e := expr.(type) {
	case nil:
		return nil
	case ColumnRef, Literal:
		if lit, ok := e.(Literal); ok {
			lit.Value = cloneQueryKernelLiteral(lit.Value)
			return lit
		}
		return e
	case Binary:
		e.Left = cloneQueryKernelExpr(e.Left)
		e.Right = cloneQueryKernelExpr(e.Right)
		return e
	case Logical:
		e.Left = cloneQueryKernelExpr(e.Left)
		e.Right = cloneQueryKernelExpr(e.Right)
		return e
	case Not:
		e.Expr = cloneQueryKernelExpr(e.Expr)
		return e
	case Conditional:
		e.Cond = cloneQueryKernelExpr(e.Cond)
		e.Then = cloneQueryKernelExpr(e.Then)
		e.Else = cloneQueryKernelExpr(e.Else)
		return e
	case In:
		e.Expr = cloneQueryKernelExpr(e.Expr)
		e.Values = cloneQueryKernelLiteralList(e.Values)
		return e
	case Within:
		e.Expr = cloneQueryKernelExpr(e.Expr)
		e.Low = cloneQueryKernelLiteral(e.Low)
		e.High = cloneQueryKernelLiteral(e.High)
		return e
	case BucketFloorExpr:
		e.Interval = cloneQueryKernelLiteral(e.Interval)
		e.Expr = cloneQueryKernelExpr(e.Expr)
		return e
	case ListAggregateExpr:
		e.Expr = cloneQueryKernelExpr(e.Expr)
		return e
	case VectorTransformExpr:
		e.Expr = cloneQueryKernelExpr(e.Expr)
		e.Arg = cloneQueryKernelExpr(e.Arg)
		return e
	default:
		return e
	}
}

func cloneQueryKernelLiteralList(values []any) []any {
	return cloneQueryKernelLiteralListWithSeen(values, nil)
}

func cloneQueryKernelLiteralListWithSeen(values []any, seen map[queryKernelSliceKey]reflect.Value) []any {
	if values == nil {
		return nil
	}
	if len(values) == 0 {
		return []any{}
	}
	key, ok := queryKernelSliceKeyForValue(reflect.ValueOf(values))
	if ok {
		if cloned, exists := seen[key]; exists {
			if out, ok := cloned.Interface().([]any); ok {
				return out
			}
			return values
		}
		if seen == nil {
			seen = make(map[queryKernelSliceKey]reflect.Value)
		}
		out := make([]any, len(values))
		seen[key] = reflect.ValueOf(out)
		defer delete(seen, key)
		for i, value := range values {
			out[i] = cloneQueryKernelLiteralWithSeen(value, seen)
		}
		return out
	}
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = cloneQueryKernelLiteralWithSeen(value, seen)
	}
	return out
}

func cloneQueryKernelLiteral(value any) any {
	return cloneQueryKernelLiteralWithSeen(value, nil)
}

func cloneQueryKernelLiteralWithSeen(value any, seen map[queryKernelSliceKey]reflect.Value) any {
	switch x := value.(type) {
	case []any:
		return cloneQueryKernelLiteralListWithSeen(x, seen)
	default:
		if cloned, ok := cloneQueryKernelLiteralSlice(value, seen); ok {
			return cloned
		}
		if cloned, ok := cloneQueryKernelLiteralArray(value, seen); ok {
			return cloned
		}
		if cloned, ok := cloneQueryKernelLiteralMap(value, seen); ok {
			return cloned
		}
		if cloned, ok := cloneQueryKernelLiteralStruct(value, seen); ok {
			return cloned
		}
		if cloned, ok := cloneQueryKernelLiteralPointer(value, seen); ok {
			return cloned
		}
		return x
	}
}

func cloneQueryKernelLiteralSlice(value any, seen map[queryKernelSliceKey]reflect.Value) (any, bool) {
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.Slice {
		return nil, false
	}
	if v.IsNil() {
		return value, true
	}
	key, ok := queryKernelSliceKeyForValue(v)
	if ok {
		if cloned, exists := seen[key]; exists {
			return cloned.Interface(), true
		}
		if seen == nil {
			seen = make(map[queryKernelSliceKey]reflect.Value)
		}
	}
	out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
	if ok {
		seen[key] = out
		defer delete(seen, key)
	}
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i)
		cloned := cloneQueryKernelLiteralWithSeen(item.Interface(), seen)
		clonedValue := reflect.ValueOf(cloned)
		if clonedValue.IsValid() && clonedValue.Type().AssignableTo(item.Type()) {
			out.Index(i).Set(clonedValue)
		} else {
			out.Index(i).Set(item)
		}
	}
	return out.Interface(), true
}

func cloneQueryKernelLiteralArray(value any, seen map[queryKernelSliceKey]reflect.Value) (any, bool) {
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.Array {
		return nil, false
	}
	out := reflect.New(v.Type()).Elem()
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i)
		if !item.CanInterface() {
			out.Index(i).Set(item)
			continue
		}
		cloned := cloneQueryKernelLiteralWithSeen(item.Interface(), seen)
		clonedValue := reflect.ValueOf(cloned)
		if clonedValue.IsValid() && clonedValue.Type().AssignableTo(item.Type()) {
			out.Index(i).Set(clonedValue)
		} else {
			out.Index(i).Set(item)
		}
	}
	return out.Interface(), true
}

func cloneQueryKernelLiteralMap(value any, seen map[queryKernelSliceKey]reflect.Value) (any, bool) {
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.Map {
		return nil, false
	}
	if v.IsNil() {
		return value, true
	}
	key, ok := queryKernelSliceKeyForValue(v)
	if ok {
		if cloned, exists := seen[key]; exists {
			return cloned.Interface(), true
		}
		if seen == nil {
			seen = make(map[queryKernelSliceKey]reflect.Value)
		}
	}
	out := reflect.MakeMapWithSize(v.Type(), v.Len())
	if ok {
		seen[key] = out
		defer delete(seen, key)
	}
	for _, mapKey := range v.MapKeys() {
		clonedKey := cloneQueryKernelLiteralWithSeen(mapKey.Interface(), seen)
		clonedKeyValue := reflect.ValueOf(clonedKey)
		if !clonedKeyValue.IsValid() || !clonedKeyValue.Type().AssignableTo(mapKey.Type()) {
			clonedKeyValue = mapKey
		}
		mapValue := v.MapIndex(mapKey)
		clonedValue := cloneQueryKernelLiteralWithSeen(mapValue.Interface(), seen)
		clonedMapValue := reflect.ValueOf(clonedValue)
		if !clonedMapValue.IsValid() || !clonedMapValue.Type().AssignableTo(mapValue.Type()) {
			clonedMapValue = mapValue
		}
		out.SetMapIndex(clonedKeyValue, clonedMapValue)
	}
	return out.Interface(), true
}

func cloneQueryKernelLiteralStruct(value any, seen map[queryKernelSliceKey]reflect.Value) (any, bool) {
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil, false
	}
	out := reflect.New(v.Type()).Elem()
	out.Set(v)
	for i := 0; i < v.NumField(); i++ {
		source := v.Field(i)
		target := out.Field(i)
		if !source.CanInterface() || !target.CanSet() {
			continue
		}
		cloned := cloneQueryKernelLiteralWithSeen(source.Interface(), seen)
		clonedValue := reflect.ValueOf(cloned)
		if clonedValue.IsValid() && clonedValue.Type().AssignableTo(source.Type()) {
			target.Set(clonedValue)
		}
	}
	return out.Interface(), true
}

func cloneQueryKernelLiteralPointer(value any, seen map[queryKernelSliceKey]reflect.Value) (any, bool) {
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.Pointer {
		return nil, false
	}
	if v.IsNil() {
		return value, true
	}
	key, ok := queryKernelSliceKeyForValue(v)
	if ok {
		if cloned, exists := seen[key]; exists {
			return cloned.Interface(), true
		}
		if seen == nil {
			seen = make(map[queryKernelSliceKey]reflect.Value)
		}
	}
	out := reflect.New(v.Type().Elem())
	if ok {
		seen[key] = out
		defer delete(seen, key)
	}
	source := v.Elem()
	if !source.CanInterface() {
		out.Elem().Set(source)
		return out.Interface(), true
	}
	cloned := cloneQueryKernelLiteralWithSeen(source.Interface(), seen)
	clonedValue := reflect.ValueOf(cloned)
	if clonedValue.IsValid() && clonedValue.Type().AssignableTo(source.Type()) {
		out.Elem().Set(clonedValue)
	} else {
		out.Elem().Set(source)
	}
	return out.Interface(), true
}

// QueryKernelCompileReason reports whether a plan can compile for a frame and
// returns the same stable reason string a compiled kernel would expose.
func QueryKernelCompileReason(frame Frame, plan QueryPlan) (bool, string, error) {
	kernel, ok, err := CompileQueryKernel(frame, plan)
	if err != nil {
		return false, "", err
	}
	if !ok {
		_, reason := QueryKernelSupportReason(plan)
		return false, reason, nil
	}
	reason := kernel.Reason()
	if reason == "" {
		reason = "data query kernel supported"
	}
	return true, reason, nil
}

// QueryKernelSupportReason reports whether a query shape can use QueryKernel.
// Unsupported shapes include a stable human-readable fallback reason for
// explain paths; CompileQueryKernel still returns ok=false without an error so
// callers can preserve QueryPlan.Exec fallback semantics.
func QueryKernelSupportReason(plan QueryPlan) (bool, string) {
	if (len(plan.By) > 0 || len(plan.ByExprs) > 0) && len(plan.Aggregates) == 0 && len(plan.Select) == 0 {
		return false, "grouped query without aggregates or select requires QueryPlan fallback"
	}
	for _, item := range plan.ByExprs {
		if reason := queryKernelExprUnsupportedReason(item.Expr); reason != "" {
			return false, fmt.Sprintf("by expression %q is not supported by data query kernel: %s", item.Name, reason)
		}
	}
	for _, agg := range plan.Aggregates {
		if !isSupportedAggregate(agg.Func) {
			return false, fmt.Sprintf("aggregate %q is not supported by data query kernel", agg.Func)
		}
		if agg.Func != "count" {
			if reason := queryKernelExprUnsupportedReason(agg.Expr); reason != "" {
				return false, fmt.Sprintf("aggregate %q expression is not supported by data query kernel: %s", agg.Name, reason)
			}
		}
		if agg.Weight != nil {
			if reason := queryKernelExprUnsupportedReason(agg.Weight); reason != "" {
				return false, fmt.Sprintf("aggregate %q weight is not supported by data query kernel: %s", agg.Name, reason)
			}
		}
	}
	for _, item := range plan.Select {
		if reason := queryKernelExprUnsupportedReason(item.Expr); reason != "" {
			return false, fmt.Sprintf("select expression %q is not supported by data query kernel: %s", item.Name, reason)
		}
	}
	if reason := queryKernelWhereUnsupportedReason(plan.Where); reason != "" {
		return false, fmt.Sprintf("where expression is not supported by data query kernel: %s", reason)
	}
	return true, queryKernelSupportSuccessReason(plan)
}

func queryKernelSupported(plan QueryPlan) bool {
	ok, _ := QueryKernelSupportReason(plan)
	return ok
}

// Reason returns the stable explain text captured when this kernel was
// compiled. It is empty for a nil kernel.
func (k *QueryKernel) Reason() string {
	if k == nil {
		return ""
	}
	return k.reason
}

// Shape returns a stable, machine-readable query-kernel shape captured at
// compile time. It is intended for cache stats and fallback elimination; use
// Reason for human-facing explain text.
func (k *QueryKernel) Shape() string {
	if k == nil {
		return ""
	}
	return k.shape
}

// PipelineShape returns the stable column-pipeline lowering shape captured at
// compile time. Unlike Shape, this describes the backend stages that a typed
// runtime/JIT path can lower to, such as column loads, compare masks, filters
// and projections.
func (k *QueryKernel) PipelineShape() string {
	if k == nil {
		return ""
	}
	return k.pipelineShape
}

func queryKernelFrameReason(frame Frame, plan QueryPlan) string {
	if len(plan.By)+len(plan.ByExprs) > 0 && len(plan.Aggregates) > 0 {
		byInputs, err := bindGroupInputs(frame, groupByItems(plan))
		if err == nil && len(byInputs) == 1 && byInputs[0].column != nil && queryPlanAggregatesAreIndexedMixedFastPath(plan) {
			return "data query kernel supported: grouped aggregate path (indexed single-column grouped mixed aggregate fast path)"
		}
	}
	return ""
}

func queryKernelSupportSuccessReason(plan QueryPlan) string {
	detail := queryKernelReasonDetail(plan)
	switch {
	case len(plan.By)+len(plan.ByExprs) > 0 && len(plan.Aggregates) > 0:
		return queryKernelReason("grouped aggregate path", detail)
	case len(plan.By)+len(plan.ByExprs) > 0:
		return queryKernelReason("grouped projection path", detail)
	case plan.Distinct:
		return queryKernelReason("distinct projection path", detail)
	case len(plan.OrderBy) > 0 && plan.PreProjectOrder:
		return queryKernelReason("pre-project ordered projection path", detail)
	case len(plan.OrderBy) > 0:
		return queryKernelReason("post-project ordered projection path", detail)
	case plan.Where != nil:
		return queryKernelReason("filtered projection path", detail)
	default:
		return queryKernelReason("projection path", detail)
	}
}

// QueryKernelPlanShape returns a stable query-kernel shape string for the
// supported subset of QueryPlan. It intentionally describes semantic shape,
// not column names or literal values, so cache and fallback statistics can be
// compared across compatible schemas and bindings.
func QueryKernelPlanShape(plan QueryPlan) string {
	path := queryKernelPlanPath(plan)
	detail := queryKernelShapeDetail(plan)
	if len(detail) == 0 {
		return path
	}
	return path + "|" + strings.Join(detail, "|")
}

// QueryKernelPlanPipelineShape returns a stable, low-cardinality description of
// the columnar pipeline implied by a QueryPlan. The string intentionally omits
// column names and literal values so it is reusable across schema-stable qSQL
// cache entries and can be consumed by typed runtime/JIT backends.
func QueryKernelPlanPipelineShape(plan QueryPlan) string {
	return QueryKernelPlanPipelineDescriptor(plan).String()
}

// QueryKernelPlanPipelineDescriptor returns the structured low-cardinality
// columnar pipeline shape used by qSQL runtime/JIT handoff.
func QueryKernelPlanPipelineDescriptor(plan QueryPlan) QueryKernelPipelineDescriptor {
	descriptor := QueryKernelPipelineDescriptor{Scan: "frame"}
	if where := queryKernelWherePipelineShape(plan.Where); where != "" {
		descriptor.WhereFamily = queryKernelWherePipelineFamily(plan.Where)
		descriptor.WhereOps = where
		descriptor.FilterFamily = "index"
	}
	if by := queryKernelGroupPipelineShape(plan); by != "" {
		descriptor.GroupFamily = queryKernelGroupPipelineFamily(plan)
		descriptor.GroupOps = by
	}
	if aggregate := queryKernelAggregatePipelineShape(plan); aggregate != "" {
		descriptor.AggregateFamily = queryKernelAggregatePipelineFamily(plan)
		descriptor.AggregateOps = aggregate
	}
	if projection := queryKernelProjectionPipelineShape(plan); projection != "" {
		descriptor.ProjectionFamily = queryKernelProjectionPipelineFamily(plan)
		descriptor.ProjectionOps = projection
	}
	if len(plan.OrderBy) > 0 {
		descriptor.OrderFamily = queryKernelOrderShape(plan)
	}
	if plan.Distinct {
		descriptor.DistinctFamily = "rows"
	}
	if plan.LimitN >= 0 {
		descriptor.LimitFamily = "bounded"
	}
	return descriptor
}

func queryKernelPlanPath(plan QueryPlan) string {
	switch {
	case len(plan.By)+len(plan.ByExprs) > 0 && len(plan.Aggregates) > 0:
		return "grouped_aggregate"
	case len(plan.By)+len(plan.ByExprs) > 0:
		return "grouped_projection"
	case plan.Distinct:
		return "distinct_projection"
	case len(plan.OrderBy) > 0 && plan.PreProjectOrder:
		return "pre_project_ordered_projection"
	case len(plan.OrderBy) > 0:
		return "post_project_ordered_projection"
	case plan.Where != nil:
		return "filtered_projection"
	default:
		return "projection"
	}
}

func queryKernelShapeDetail(plan QueryPlan) []string {
	detail := make([]string, 0, 6)
	if where := queryKernelWhereShape(plan.Where); where != "" {
		detail = append(detail, "where="+where)
	}
	if by := queryKernelByShape(plan); by != "" {
		detail = append(detail, "by="+by)
	}
	if aggs := queryKernelAggregateShape(plan); aggs != "" {
		detail = append(detail, "aggregate="+aggs)
	}
	if projection := queryKernelProjectionShape(plan); projection != "" {
		detail = append(detail, "projection="+projection)
	}
	if len(plan.OrderBy) > 0 {
		detail = append(detail, "order="+queryKernelOrderShape(plan))
	}
	if plan.LimitN >= 0 {
		detail = append(detail, "limit=bounded")
	}
	if plan.Distinct {
		detail = append(detail, "distinct=true")
	}
	return detail
}

func queryKernelReason(path string, detail []string) string {
	if len(detail) == 0 {
		return "data query kernel supported: " + path
	}
	return fmt.Sprintf("data query kernel supported: %s (%s)", path, joinReasonDetail(detail))
}

func joinReasonDetail(detail []string) string {
	if len(detail) == 0 {
		return ""
	}
	out := detail[0]
	for _, item := range detail[1:] {
		out += ", " + item
	}
	return out
}

func queryKernelReasonDetail(plan QueryPlan) []string {
	detail := make([]string, 0, 7)
	if where := queryKernelWhereReasonDetail(plan.Where); where != "" {
		detail = append(detail, where)
	}
	if by := queryKernelByReasonDetail(plan); by != "" {
		detail = append(detail, by)
	}
	if aggs := queryKernelAggregateReasonDetail(plan); aggs != "" {
		detail = append(detail, aggs)
	}
	if projection := queryKernelProjectionReasonDetail(plan); projection != "" {
		detail = append(detail, projection)
	}
	detail = append(detail, queryKernelModifierReasonDetail(plan)...)
	return detail
}

func queryKernelWhereReasonDetail(expr Expr) string {
	switch e := expr.(type) {
	case nil:
		return ""
	case Binary:
		if _, _, _, ok := binaryColumnLiteral(e); ok && isComparisonOp(e.Op) {
			return "typed column-literal filter"
		}
		return "vectorized boolean filter"
	case Within:
		if _, ok := e.Expr.(ColumnRef); ok {
			return "typed within filter"
		}
		return "within expression filter"
	case In:
		if _, ok := e.Expr.(ColumnRef); ok {
			return "typed in filter"
		}
		return "in expression filter"
	case Logical:
		return "logical filter expression"
	case Not:
		return "not filter expression"
	case Conditional:
		return "conditional filter expression"
	case ColumnRef, Literal:
		return "scalar filter expression"
	default:
		return fmt.Sprintf("%T filter expression", expr)
	}
}

func queryKernelWhereShape(expr Expr) string {
	switch e := expr.(type) {
	case nil:
		return ""
	case Binary:
		if _, _, _, ok := binaryColumnLiteral(e); ok && isComparisonOp(e.Op) {
			return "typed_column_literal"
		}
		return "vectorized_boolean"
	case Within:
		if _, ok := e.Expr.(ColumnRef); ok {
			return "typed_within"
		}
		return "within_expression"
	case In:
		if _, ok := e.Expr.(ColumnRef); ok {
			return "typed_in"
		}
		return "in_expression"
	case Logical:
		return "logical"
	case Not:
		return "not"
	case Conditional:
		return "conditional"
	case ColumnRef, Literal:
		return "scalar"
	default:
		return "computed"
	}
}

func queryKernelWherePipelineShape(expr Expr) string {
	switch e := expr.(type) {
	case nil:
		return ""
	case Binary:
		if _, _, _, ok := binaryColumnLiteral(e); ok && isComparisonOp(e.Op) {
			return "compare_mask:column_literal"
		}
		if isComparisonOp(e.Op) {
			return "compare_mask:computed"
		}
		return "bool_mask:binary"
	case Within:
		if _, ok := e.Expr.(ColumnRef); ok {
			return "compare_mask:within_column"
		}
		return "compare_mask:within_computed"
	case In:
		if _, ok := e.Expr.(ColumnRef); ok {
			return "compare_mask:in_column"
		}
		return "compare_mask:in_computed"
	case Logical:
		return "bool_mask:logical_" + e.Op
	case Not:
		return "bool_mask:not"
	case Conditional:
		return "bool_mask:conditional"
	case ColumnRef:
		return "bool_mask:column_load"
	case Literal:
		return "bool_mask:literal"
	default:
		return "bool_mask:computed"
	}
}

func queryKernelWherePipelineFamily(expr Expr) string {
	switch e := expr.(type) {
	case nil:
		return ""
	case Binary:
		if isComparisonOp(e.Op) {
			return "compare_mask"
		}
		return "bool_mask"
	case Within, In:
		return "compare_mask"
	case Logical, Not, Conditional, ColumnRef, Literal:
		return "bool_mask"
	default:
		return "bool_mask"
	}
}

func queryKernelByReasonDetail(plan QueryPlan) string {
	if len(plan.ByExprs) == 0 {
		return ""
	}
	for _, item := range plan.ByExprs {
		switch item.Expr.(type) {
		case BucketFloorExpr:
		default:
			return "computed by expression"
		}
	}
	return "bucketed by expression"
}

func queryKernelByShape(plan QueryPlan) string {
	if len(plan.ByExprs) == 0 {
		if len(plan.By) == 0 {
			return ""
		}
		return "columns"
	}
	for _, item := range plan.ByExprs {
		switch item.Expr.(type) {
		case BucketFloorExpr:
		default:
			return "computed"
		}
	}
	if len(plan.By) > 0 {
		return "columns_bucketed"
	}
	return "bucketed"
}

func queryKernelGroupPipelineShape(plan QueryPlan) string {
	columnCount := len(plan.By)
	exprOps := make(map[string]int, len(plan.ByExprs))
	for _, item := range plan.ByExprs {
		exprOps[queryKernelExprPipelineOp(item.Expr)]++
	}
	parts := make([]string, 0, 2)
	if columnCount > 0 {
		parts = append(parts, fmt.Sprintf("column_load:%d", columnCount))
	}
	parts = appendPipelineOpCounts(parts, exprOps)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ",")
}

func queryKernelGroupPipelineFamily(plan QueryPlan) string {
	switch queryKernelByShape(plan) {
	case "":
		return ""
	case "columns":
		return "key_columns"
	case "bucketed", "columns_bucketed":
		return "key_bucket"
	default:
		return "key_expr"
	}
}

func queryKernelAggregateReasonDetail(plan QueryPlan) string {
	if len(plan.Aggregates) == 0 {
		return ""
	}
	allColumnRefs := true
	hasWeight := false
	for _, agg := range plan.Aggregates {
		if agg.Weight != nil {
			hasWeight = true
		}
		if agg.Func == "count" {
			continue
		}
		if _, ok := agg.Expr.(ColumnRef); !ok {
			allColumnRefs = false
		}
	}
	switch {
	case hasWeight:
		return "weighted aggregate expression"
	case allColumnRefs:
		return "typed column aggregate"
	default:
		return "computed aggregate expression"
	}
}

func queryKernelAggregateShape(plan QueryPlan) string {
	if len(plan.Aggregates) == 0 {
		return ""
	}
	allColumnRefs := true
	hasWeight := false
	for _, agg := range plan.Aggregates {
		if agg.Weight != nil {
			hasWeight = true
		}
		if agg.Func == "count" {
			continue
		}
		if _, ok := agg.Expr.(ColumnRef); !ok {
			allColumnRefs = false
		}
	}
	switch {
	case hasWeight:
		return "weighted"
	case allColumnRefs:
		return "typed_column"
	default:
		return "computed"
	}
}

func queryKernelAggregatePipelineShape(plan QueryPlan) string {
	if len(plan.Aggregates) == 0 {
		return ""
	}
	ops := make(map[string]int, len(plan.Aggregates)*2)
	for _, agg := range plan.Aggregates {
		ops["reduce:"+agg.Func]++
		if agg.Func != "count" {
			ops[queryKernelExprPipelineOp(agg.Expr)]++
		}
		if agg.Weight != nil {
			ops["weight:"+queryKernelExprPipelineOp(agg.Weight)]++
		}
	}
	return strings.Join(appendPipelineOpCounts(nil, ops), ",")
}

func queryKernelAggregatePipelineFamily(plan QueryPlan) string {
	if len(plan.Aggregates) == 0 {
		return ""
	}
	hasWeighted := false
	hasComputed := false
	for _, agg := range plan.Aggregates {
		if agg.Weight != nil {
			hasWeighted = true
		}
		if agg.Func != "count" {
			switch agg.Expr.(type) {
			case ColumnRef:
			case Binary:
				hasComputed = true
			default:
				hasComputed = true
			}
		}
	}
	switch {
	case hasWeighted:
		return "weighted_reduce"
	case hasComputed:
		return "computed_reduce"
	default:
		return "column_reduce"
	}
}

func queryKernelProjectionReasonDetail(plan QueryPlan) string {
	if len(plan.Select) == 0 {
		return ""
	}
	hasColumnOnly := false
	hasBinary := false
	hasConditional := false
	hasBoolean := false
	hasVectorTransform := false
	hasListAggregate := false
	hasBucket := false
	hasOther := false
	for _, item := range plan.Select {
		switch item.Expr.(type) {
		case ColumnRef, Literal:
			hasColumnOnly = true
		case Binary:
			hasBinary = true
		case Conditional:
			hasConditional = true
		case Logical, Not, In, Within:
			hasBoolean = true
		case VectorTransformExpr:
			hasVectorTransform = true
		case ListAggregateExpr:
			hasListAggregate = true
		case BucketFloorExpr:
			hasBucket = true
		default:
			hasOther = true
		}
	}
	switch {
	case hasVectorTransform:
		return "vector transform projection"
	case hasListAggregate:
		return "list aggregate projection"
	case hasConditional:
		return "conditional projection"
	case hasBoolean:
		return "boolean projection"
	case hasBinary:
		return "typed binary projection"
	case hasBucket:
		return "bucket projection"
	case hasOther:
		return "computed projection"
	case hasColumnOnly:
		return "column projection"
	default:
		return ""
	}
}

func queryKernelProjectionShape(plan QueryPlan) string {
	if len(plan.Select) == 0 {
		return ""
	}
	hasColumnOnly := false
	hasBinary := false
	hasConditional := false
	hasBoolean := false
	hasVectorTransform := false
	hasListAggregate := false
	hasBucket := false
	hasOther := false
	for _, item := range plan.Select {
		switch item.Expr.(type) {
		case ColumnRef, Literal:
			hasColumnOnly = true
		case Binary:
			hasBinary = true
		case Conditional:
			hasConditional = true
		case Logical, Not, In, Within:
			hasBoolean = true
		case VectorTransformExpr:
			hasVectorTransform = true
		case ListAggregateExpr:
			hasListAggregate = true
		case BucketFloorExpr:
			hasBucket = true
		default:
			hasOther = true
		}
	}
	switch {
	case hasVectorTransform:
		return "vector_transform"
	case hasListAggregate:
		return "list_aggregate"
	case hasConditional:
		return "conditional"
	case hasBoolean:
		return "boolean"
	case hasBinary:
		return "typed_binary"
	case hasBucket:
		return "bucket"
	case hasOther:
		return "computed"
	case hasColumnOnly:
		return "columns"
	default:
		return ""
	}
}

func queryKernelProjectionPipelineShape(plan QueryPlan) string {
	if len(plan.Select) == 0 {
		return "column_load:all"
	}
	ops := make(map[string]int, len(plan.Select))
	for _, item := range plan.Select {
		ops[queryKernelExprPipelineOp(item.Expr)]++
	}
	return strings.Join(appendPipelineOpCounts(nil, ops), ",")
}

func queryKernelProjectionPipelineFamily(plan QueryPlan) string {
	switch queryKernelProjectionShape(plan) {
	case "":
		return "column_load"
	case "columns":
		return "column_load"
	case "typed_binary":
		return "typed_expr"
	case "boolean":
		return "mask_expr"
	case "conditional":
		return "where_select"
	case "bucket":
		return "bucket"
	case "list_aggregate":
		return "list_reduce"
	case "vector_transform":
		return "vector_transform"
	default:
		return "computed_expr"
	}
}

func queryKernelExprPipelineOp(expr Expr) string {
	switch e := expr.(type) {
	case nil:
		return "none"
	case ColumnRef:
		return "column_load"
	case Literal:
		return "literal"
	case Binary:
		if _, _, _, ok := binaryColumnLiteral(e); ok && isComparisonOp(e.Op) {
			return "compare_mask"
		}
		if isComparisonOp(e.Op) {
			return "compare_mask_computed"
		}
		return "typed_binary"
	case Conditional:
		return "where_select"
	case Logical, Not:
		return "bool_mask"
	case In:
		return "compare_mask_in"
	case Within:
		return "compare_mask_within"
	case BucketFloorExpr:
		return "bucket"
	case ListAggregateExpr:
		return "list_reduce:" + e.Func
	case VectorTransformExpr:
		return "vector_transform:" + e.Func
	default:
		return "computed"
	}
}

func appendPipelineOpCounts(parts []string, counts map[string]int) []string {
	if len(counts) == 0 {
		return parts
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", key, counts[key]))
	}
	return parts
}

func queryKernelOrderShape(plan QueryPlan) string {
	if len(plan.OrderBy) == 0 {
		return ""
	}
	mode := "post_project"
	if plan.PreProjectOrder {
		mode = "pre_project"
	}
	return fmt.Sprintf("%s:%d", mode, len(plan.OrderBy))
}

func queryKernelModifierReasonDetail(plan QueryPlan) []string {
	detail := make([]string, 0, 3)
	if plan.Distinct {
		detail = append(detail, "distinct rows")
	}
	if len(plan.OrderBy) > 0 {
		if plan.PreProjectOrder {
			detail = append(detail, "pre-project order")
		} else {
			detail = append(detail, "post-project order")
		}
	}
	if plan.LimitN >= 0 {
		detail = append(detail, "limit")
	}
	return detail
}

func queryPlanAggregatesAreIndexedMixedFastPath(plan QueryPlan) bool {
	if len(plan.Aggregates) == 0 {
		return false
	}
	for _, agg := range plan.Aggregates {
		switch agg.Func {
		case "count":
		case "sum", "avg", "min", "max":
			if _, ok := agg.Expr.(ColumnRef); ok {
				break
			}
			if bin, ok := agg.Expr.(Binary); ok && isNumericBinaryAggregateOp(bin.Op) {
				_, leftOK := bin.Left.(ColumnRef)
				_, rightOK := bin.Right.(ColumnRef)
				if leftOK && rightOK {
					break
				}
			}
			{
				return false
			}
		default:
			return false
		}
		if agg.Weight != nil {
			return false
		}
	}
	return true
}

func queryKernelWhereUnsupportedReason(expr Expr) string {
	switch e := expr.(type) {
	case nil, ColumnRef, Literal:
		return ""
	case Binary:
		if reason := queryKernelExprUnsupportedReason(e.Left); reason != "" {
			return "binary left operand: " + reason
		}
		if reason := queryKernelExprUnsupportedReason(e.Right); reason != "" {
			return "binary right operand: " + reason
		}
		return ""
	case Conditional:
		if reason := queryKernelWhereUnsupportedReason(e.Cond); reason != "" {
			return "conditional condition: " + reason
		}
		if reason := queryKernelExprUnsupportedReason(e.Then); reason != "" {
			return "conditional then branch: " + reason
		}
		if reason := queryKernelExprUnsupportedReason(e.Else); reason != "" {
			return "conditional else branch: " + reason
		}
		return ""
	case Logical:
		if reason := queryKernelWhereUnsupportedReason(e.Left); reason != "" {
			return "logical left operand: " + reason
		}
		if reason := queryKernelWhereUnsupportedReason(e.Right); reason != "" {
			return "logical right operand: " + reason
		}
		return ""
	case Not:
		if reason := queryKernelWhereUnsupportedReason(e.Expr); reason != "" {
			return "not operand: " + reason
		}
		return ""
	case In:
		return queryKernelExprUnsupportedReason(e.Expr)
	case Within:
		return queryKernelExprUnsupportedReason(e.Expr)
	default:
		return fmt.Sprintf("unsupported expression %T", expr)
	}
}

func queryKernelExprUnsupportedReason(expr Expr) string {
	switch e := expr.(type) {
	case nil, ColumnRef, Literal:
		return ""
	case Binary:
		if reason := queryKernelExprUnsupportedReason(e.Left); reason != "" {
			return "binary left operand: " + reason
		}
		if reason := queryKernelExprUnsupportedReason(e.Right); reason != "" {
			return "binary right operand: " + reason
		}
		return ""
	case Conditional:
		if reason := queryKernelWhereUnsupportedReason(e.Cond); reason != "" {
			return "conditional condition: " + reason
		}
		if reason := queryKernelExprUnsupportedReason(e.Then); reason != "" {
			return "conditional then branch: " + reason
		}
		if reason := queryKernelExprUnsupportedReason(e.Else); reason != "" {
			return "conditional else branch: " + reason
		}
		return ""
	case Logical:
		if reason := queryKernelExprUnsupportedReason(e.Left); reason != "" {
			return "logical left operand: " + reason
		}
		if reason := queryKernelExprUnsupportedReason(e.Right); reason != "" {
			return "logical right operand: " + reason
		}
		return ""
	case Not:
		if reason := queryKernelExprUnsupportedReason(e.Expr); reason != "" {
			return "not operand: " + reason
		}
		return ""
	case In:
		if reason := queryKernelExprUnsupportedReason(e.Expr); reason != "" {
			return "in expression: " + reason
		}
		return ""
	case Within:
		if reason := queryKernelExprUnsupportedReason(e.Expr); reason != "" {
			return "within expression: " + reason
		}
		return ""
	case BucketFloorExpr:
		if reason := queryKernelExprUnsupportedReason(e.Expr); reason != "" {
			return "bucket floor expression: " + reason
		}
		return ""
	case ListAggregateExpr:
		if reason := queryKernelExprUnsupportedReason(e.Expr); reason != "" {
			return fmt.Sprintf("list aggregate %q expression: %s", e.Func, reason)
		}
		return ""
	case VectorTransformExpr:
		if reason := queryKernelExprUnsupportedReason(e.Expr); reason != "" {
			return fmt.Sprintf("vector transform %q expression: %s", e.Func, reason)
		}
		if e.Arg != nil {
			if reason := queryKernelExprUnsupportedReason(e.Arg); reason != "" {
				return fmt.Sprintf("vector transform %q argument: %s", e.Func, reason)
			}
		}
		return ""
	default:
		return fmt.Sprintf("unsupported expression %T", expr)
	}
}

func validateQueryKernelFrame(frame Frame, plan QueryPlan) error {
	if err := validateQueryKernelExpr(frame, plan.Where); err != nil {
		return err
	}
	for _, by := range plan.By {
		if _, ok := frame.Column(by); !ok {
			return fmt.Errorf("unknown by column %q", by)
		}
	}
	for _, item := range plan.ByExprs {
		if item.Name == "" {
			return fmt.Errorf("by item name must not be empty")
		}
		if err := validateQueryKernelExpr(frame, item.Expr); err != nil {
			return err
		}
	}
	for _, agg := range plan.Aggregates {
		if agg.Name == "" {
			return fmt.Errorf("aggregate name must not be empty")
		}
		if !isSupportedAggregate(agg.Func) {
			return fmt.Errorf("unsupported aggregate %q", agg.Func)
		}
		if agg.Func != "count" {
			if err := validateQueryKernelExpr(frame, agg.Expr); err != nil {
				return err
			}
		}
		if agg.Weight != nil {
			if err := validateQueryKernelExpr(frame, agg.Weight); err != nil {
				return err
			}
		}
	}
	for _, item := range plan.Select {
		if item.Name == "" {
			return fmt.Errorf("select item name must not be empty")
		}
		if err := validateQueryKernelExpr(frame, item.Expr); err != nil {
			return err
		}
	}
	if len(plan.OrderBy) > 0 {
		orderColumns := make(map[Symbol]struct{}, len(plan.Select)+len(plan.By)+len(plan.ByExprs)+len(plan.Aggregates))
		if !plan.PreProjectOrder {
			if len(plan.By) > 0 || len(plan.ByExprs) > 0 || len(plan.Aggregates) > 0 {
				for _, item := range groupByItems(plan) {
					orderColumns[item.Name] = struct{}{}
				}
				for _, agg := range plan.Aggregates {
					orderColumns[agg.Name] = struct{}{}
				}
			} else if len(plan.Select) == 0 {
				for _, name := range frame.Schema().Names() {
					orderColumns[name] = struct{}{}
				}
			}
			for _, item := range plan.Select {
				orderColumns[item.Name] = struct{}{}
			}
		}
		for _, order := range plan.OrderBy {
			if plan.PreProjectOrder {
				if _, ok := frame.Column(order.Column); !ok {
					return fmt.Errorf("unknown order column %q", order.Column)
				}
				continue
			}
			if _, ok := orderColumns[order.Column]; !ok {
				return fmt.Errorf("unknown order column %q", order.Column)
			}
		}
	}
	return nil
}

func validateQueryKernelExpr(frame Frame, expr Expr) error {
	switch e := expr.(type) {
	case nil, Literal:
		return nil
	case ColumnRef:
		if _, ok := frame.Column(e.Name); !ok {
			return fmt.Errorf("unknown column %q", e.Name)
		}
		return nil
	case Binary:
		if err := validateQueryKernelExpr(frame, e.Left); err != nil {
			return err
		}
		return validateQueryKernelExpr(frame, e.Right)
	case Conditional:
		if err := validateQueryKernelExpr(frame, e.Cond); err != nil {
			return err
		}
		if err := validateQueryKernelExpr(frame, e.Then); err != nil {
			return err
		}
		return validateQueryKernelExpr(frame, e.Else)
	case Logical:
		if err := validateQueryKernelExpr(frame, e.Left); err != nil {
			return err
		}
		return validateQueryKernelExpr(frame, e.Right)
	case Not:
		return validateQueryKernelExpr(frame, e.Expr)
	case In:
		return validateQueryKernelExpr(frame, e.Expr)
	case Within:
		return validateQueryKernelExpr(frame, e.Expr)
	case BucketFloorExpr:
		return validateQueryKernelExpr(frame, e.Expr)
	case ListAggregateExpr:
		return validateQueryKernelExpr(frame, e.Expr)
	case VectorTransformExpr:
		if err := validateQueryKernelExpr(frame, e.Expr); err != nil {
			return err
		}
		if e.Arg != nil {
			return validateQueryKernelExpr(frame, e.Arg)
		}
		return nil
	default:
		return fmt.Errorf("unsupported kernel expression %T", expr)
	}
}

func (k *QueryKernel) Exec(frame Frame) (Frame, error) {
	if k == nil {
		return Frame{}, fmt.Errorf("query kernel is nil")
	}
	if err := validateQueryKernelSchema(frame, k.schema); err != nil {
		return Frame{}, err
	}
	plan := k.plan
	plan.Source = frame
	if out, ok, err := execGroupedFilteredWhere(frame, plan); ok || err != nil {
		if err != nil {
			return Frame{}, err
		}
		if plan.Distinct {
			out, err = Distinct(out)
			if err != nil {
				return Frame{}, err
			}
		}
		if len(plan.OrderBy) > 0 && !plan.PreProjectOrder {
			out, err = orderFrameLimit(out, plan.OrderBy, plan.LimitN)
			if err != nil {
				return Frame{}, err
			}
		}
		if plan.LimitN >= 0 && plan.LimitN < out.Len() {
			return out.Gather(allIndexes(plan.LimitN))
		}
		return out, nil
	}
	indexes, err := filterIndexes(frame, plan.Where)
	if err != nil {
		return Frame{}, err
	}
	projectedOrderBy, projectOrderBeforeProjection := projectedSourceOrderSpecs(plan)
	if len(plan.OrderBy) > 0 && plan.PreProjectOrder {
		if canLimitBeforeProjection(plan) {
			indexes, err = orderIndexesLimit(frame, indexes, plan.OrderBy, plan.LimitN)
		} else {
			indexes, err = orderIndexes(frame, indexes, plan.OrderBy)
		}
		if err != nil {
			return Frame{}, err
		}
	}
	if len(projectedOrderBy) > 0 && projectOrderBeforeProjection {
		indexes, err = orderIndexesLimit(frame, indexes, projectedOrderBy, plan.LimitN)
		if err != nil {
			return Frame{}, err
		}
	}
	if canLimitBeforeProjection(plan) && plan.LimitN < len(indexes) {
		indexes = indexes[:plan.LimitN]
	}
	var out Frame
	if len(plan.By) > 0 || len(plan.ByExprs) > 0 || len(plan.Aggregates) > 0 {
		out, err = execGrouped(frame, indexes, plan)
	} else {
		out, err = execProject(frame, indexes, plan.Select)
	}
	if err != nil {
		return Frame{}, err
	}
	if plan.Distinct {
		out, err = Distinct(out)
		if err != nil {
			return Frame{}, err
		}
	}
	if len(plan.OrderBy) > 0 && !plan.PreProjectOrder && !projectOrderBeforeProjection {
		out, err = orderFrameLimit(out, plan.OrderBy, plan.LimitN)
		if err != nil {
			return Frame{}, err
		}
	}
	if plan.LimitN >= 0 && plan.LimitN < out.Len() {
		return out.Gather(allIndexes(plan.LimitN))
	}
	return out, nil
}

// ExecQueryKernelOrPlan executes a compiled query kernel when present and
// preserves QueryPlan fallback semantics for unsupported kernel shapes.
func ExecQueryKernelOrPlan(kernel *QueryKernel, plan QueryPlan, frame Frame) (Frame, error) {
	if kernel != nil {
		return kernel.Exec(frame)
	}
	return Exec(frame, plan)
}

func validateQueryKernelSchema(frame Frame, want Schema) error {
	got := frame.Schema()
	if len(got.names) != len(want.names) {
		return fmt.Errorf("query kernel schema mismatch: got %d columns, want %d", len(got.names), len(want.names))
	}
	for i, wantName := range want.names {
		gotName := got.names[i]
		if gotName != wantName {
			return fmt.Errorf("query kernel schema mismatch at column %d: got %q, want %q", i, gotName, wantName)
		}
		gotKind, gotOK := got.Kind(gotName)
		wantKind, wantOK := want.Kind(wantName)
		if !gotOK || !wantOK || gotKind != wantKind {
			return fmt.Errorf("query kernel schema mismatch for column %q: got %s, want %s", wantName, gotKind, wantKind)
		}
	}
	return nil
}
