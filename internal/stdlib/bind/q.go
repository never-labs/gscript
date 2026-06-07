package bind

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

const qSymbolVectorMarker = "__q_symbol_vector"
const qKeyedFrameMarker = "__q_keyed_frame"
const qDictKeysMarker = "__q_dict_keys"

const qSQLPlanCacheLimit = 128
const qQueryKernelSupportCacheLimit = 128
const qEvalCacheLimit = 256

const (
	qFallbackKernelUnsupported = "kernel_unsupported"
	qFallbackKernelCompileErr  = "kernel_compile_error"
	qFallbackSourceErr         = "source_error"
	qFallbackJoinErr           = "join_error"
	qFallbackMutationPlan      = "mutation_plan"
	qQueryKernelSupported      = "query_kernel_supported"
	qFallbackQueryKernel       = "query_kernel_unsupported"

	qKernelReasonSupported         = "kernel_supported"
	qKernelReasonMutationPlan      = "mutation_plan"
	qKernelReasonSourceUnavailable = "source_unavailable"
	qKernelReasonJoinUnavailable   = "join_unavailable"
	qKernelReasonCompileError      = "kernel_compile_error"
	qKernelReasonUnsupported       = "kernel_unsupported"
	qQueryKernelReasonUnsupported  = "query_kernel_unsupported"
	qQueryKernelReasonSelect       = "query_select_expression"
	qQueryKernelReasonOrder        = "query_order_expression"
)

type qSQLPlanTemplate struct {
	source       string
	sourcePath   string
	op           stdq.QueryKind
	literalFrame *data.Frame
	literalKeys  []data.Symbol
	hiddenCols   []data.Symbol
	plan         data.QueryPlan
	execDict     *stdq.ExecDictPlan
	mutation     *stdq.MutationPlan
	join         *stdq.JoinPlan
	joins        []*stdq.JoinPlan
}

type qSQLPlanCacheStats struct {
	TemplateHits      int
	TemplateMisses    int
	TemplateEvictions int
	AlignedHits       int
	AlignedMisses     int
	AlignedEvictions  int
	KernelHits        int
	KernelMisses      int
	KernelEvictions   int
}

type qEvalCacheStats struct {
	Hits      int
	Misses    int
	Evictions int
}

type qQueryKernelSupportCacheStats struct {
	Hits      int
	Misses    int
	Evictions int
}

type qQueryKernelSupportCacheEntry struct {
	Supported  bool
	ReasonCode string
	Reason     string
	SchemaHash string
}

type qFallbackStats struct {
	KernelUnsupported int
	KernelCompileErr  int
	SourceErr         int
	JoinErr           int
	Mutation          int
	QueryKernelHit    int
	QueryKernel       int
	ByReasonCode      map[qFallbackReasonCodeKey]int
	ByReason          map[qFallbackReasonKey]int
}

type qFallbackReasonCodeKey struct {
	Code       string
	ReasonCode string
}

type qFallbackReasonKey struct {
	Code   string
	Reason string
}

type qSQLArgsResult struct {
	frameValue    Value
	source        string
	resolveSource bool
	envValue      Value
}

var (
	qSQLTemplateCacheMu sync.Mutex
	qSQLTemplateCache   = make(map[string]qSQLPlanTemplate)
	qSQLTemplateOrder   []string
	qSQLTemplateStats   qSQLPlanCacheStats

	qSQLAlignedPlanCacheMu   sync.Mutex
	qSQLAlignedPlanCache     = make(map[string]data.QueryPlan)
	qSQLAlignedPlanOrder     []string
	qSQLAlignedMutationCache = make(map[string]*stdq.MutationPlan)
	qSQLAlignedMutationOrder []string
	qSQLKernelCache          = make(map[string]*data.QueryKernel)
	qSQLKernelOrder          []string
	qSQLAlignedStats         qSQLPlanCacheStats

	qEvalCacheMu    sync.Mutex
	qEvalCache      = make(map[string]any)
	qEvalCacheOrder []string
	qEvalStats      qEvalCacheStats

	qQueryKernelSupportCacheMu    sync.Mutex
	qQueryKernelSupportCache      = make(map[string]qQueryKernelSupportCacheEntry)
	qQueryKernelSupportCacheOrder []string
	qQueryKernelSupportStats      qQueryKernelSupportCacheStats

	qFallbackStatsMu  sync.Mutex
	qFallbackCounters qFallbackStats
)

// BuildQ creates the "q" column-query helper library. q.query keeps the
// ordinary Leia table-plan API, while q.eval and q`...` expose a small
// q/kdb+-style symbolic vector subset.
func BuildQ() *Table {
	t := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "q." + name, Fn: fn}))
	}
	set("query", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsSoA() {
			return nil, fmt.Errorf("q.query: argument 1 must be soa")
		}
		if len(args) < 2 || !args[1].IsTable() {
			return nil, fmt.Errorf("q.query: argument 2 must be a query plan table")
		}
		out, err := qRunQuery(args[0].SoA(), args[1].Table())
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(out)}, nil
	})
	set("explain_query", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsSoA() {
			return nil, fmt.Errorf("q.explain_query: argument 1 must be soa")
		}
		if len(args) < 2 || !args[1].IsTable() {
			return nil, fmt.Errorf("q.explain_query: argument 2 must be a query plan table")
		}
		out, err := qExplainQuery(args[0].SoA(), args[1].Table())
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(out)}, nil
	})
	set("eval", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("q.eval: argument 1 must be a q source string")
		}
		return qEvalSymbolic(args[0].Str())
	})
	set("session", func(args []Value) ([]Value, error) {
		if len(args) > 0 {
			return nil, fmt.Errorf("q.session: expected no arguments")
		}
		return []Value{TableValue(qSessionValue())}, nil
	})
	set("workspace", func(args []Value) ([]Value, error) {
		if len(args) > 0 {
			return nil, fmt.Errorf("q.workspace: expected no arguments")
		}
		return []Value{TableValue(qSessionValue())}, nil
	})
	set("key_by", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("q.key_by: expected frame and at least one key")
		}
		frame, err := qDataFrameFromValue(args[0], "")
		if err != nil {
			return nil, fmt.Errorf("q.key_by: %w", err)
		}
		keys, err := qSymbolsFromArgs("q.key_by", args[1:])
		if err != nil {
			return nil, err
		}
		keyed, err := data.KeyBy(frame, keys...)
		if err != nil {
			return nil, fmt.Errorf("q.key_by: %w", err)
		}
		return []Value{qKeyedFrameToValue(keyed)}, nil
	})
	set("lookup", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() {
			return nil, fmt.Errorf("q.lookup: expected keyed frame and at least one key value")
		}
		keyed, err := qKeyedFrameFromValue(args[0])
		if err != nil {
			return nil, fmt.Errorf("q.lookup: %w", err)
		}
		values, err := qLookupKeyValues(keyed, args[1:])
		if err != nil {
			return nil, fmt.Errorf("q.lookup: %w", err)
		}
		out, err := keyed.LookupValueByKey(values...)
		if err != nil {
			return nil, fmt.Errorf("q.lookup: %w", err)
		}
		rows, err := qRowsFromDataFrame(out)
		if err != nil {
			return nil, fmt.Errorf("q.lookup: %w", err)
		}
		return []Value{TableValue(rows)}, nil
	})
	set("amend", func(args []Value) ([]Value, error) {
		keyed, delta, valueColumns, err := qKeyedMutationArgs("q.amend", args)
		if err != nil {
			return nil, err
		}
		out, err := data.AmendKeyed(keyed, delta, valueColumns...)
		if err != nil {
			return nil, fmt.Errorf("q.amend: %w", err)
		}
		return []Value{qKeyedFrameToValue(out)}, nil
	})
	set("upsert", func(args []Value) ([]Value, error) {
		keyed, delta, valueColumns, err := qKeyedMutationArgs("q.upsert", args)
		if err != nil {
			return nil, err
		}
		out, err := data.UpsertKeyed(keyed, delta, valueColumns...)
		if err != nil {
			return nil, fmt.Errorf("q.upsert: %w", err)
		}
		return []Value{qKeyedFrameToValue(out)}, nil
	})
	set("keys", func(args []Value) ([]Value, error) {
		return qKeys(args, false)
	})
	set("key", func(args []Value) ([]Value, error) {
		return qKeys(args, true)
	})
	set("value", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("q.value: argument 1 required")
		}
		if keyed, err := qKeyedFrameFromValue(args[0]); err == nil {
			frame, err := keyed.ValueFrame()
			if err != nil {
				return nil, fmt.Errorf("q.value: %w", err)
			}
			rows, err := qRowsFromDataFrame(frame)
			if err != nil {
				return nil, fmt.Errorf("q.value: %w", err)
			}
			return []Value{TableValue(rows)}, nil
		}
		if values, ok := qDictionaryValues(args[0]); ok {
			return []Value{values}, nil
		}
		return []Value{args[0]}, nil
	})
	set("cols", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("q.cols: argument 1 required")
		}
		frame, err := qFrameLikeValue(args[0])
		if err != nil {
			return nil, fmt.Errorf("q.cols: %w", err)
		}
		return []Value{qDataSymbolListValue(frame.Schema().Names())}, nil
	})
	set("meta", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("q.meta: argument 1 required")
		}
		frame, err := qFrameLikeValue(args[0])
		if err != nil {
			return nil, fmt.Errorf("q.meta: %w", err)
		}
		rows, err := qRowsFromDataFrame(qMetaFrame(frame))
		if err != nil {
			return nil, fmt.Errorf("q.meta: %w", err)
		}
		return []Value{TableValue(rows)}, nil
	})
	set("encode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("q.encode: argument 1 required")
		}
		v, err := qWireValueFromValue(args[0])
		if err != nil {
			return nil, fmt.Errorf("q.encode: %w", err)
		}
		buf, err := stdq.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("q.encode: %w", err)
		}
		return []Value{StringValue(string(buf))}, nil
	})
	set("decode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("q.decode: argument 1 must be encoded string")
		}
		v, err := stdq.Unmarshal([]byte(args[0].Str()))
		if err != nil {
			return nil, fmt.Errorf("q.decode: %w", err)
		}
		out, err := qEvalValueToValue(v)
		if err != nil {
			return nil, fmt.Errorf("q.decode: %w", err)
		}
		return []Value{out}, nil
	})
	qsql := func(name string, args []Value) ([]Value, error) {
		parsed, err := qSQLArgs(name, args)
		if err != nil {
			return nil, err
		}
		out, err := qRunSQL(name, parsed)
		if err != nil {
			return nil, err
		}
		return []Value{out}, nil
	}
	setQSQL := func(name string) {
		nativeKind := NativeKindStdQSQL
		nativeData := StdQSQLIdentityPtr()
		if name == "select" {
			nativeKind = NativeKindStdQSelect
			nativeData = StdQSelectIdentityPtr()
		}
		t.RawSetString(name, FunctionValue(&GoFunction{
			Name:       "q." + name,
			NativeKind: nativeKind,
			NativeData: nativeData,
			Fn: func(args []Value) ([]Value, error) {
				return qsql("q."+name, args)
			},
			FastArg2: func(a, b Value) (Value, error) {
				return qSQLFastArg2("q."+name, a, b)
			},
		}))
	}
	setQSQL("sql")
	setQSQL("select")
	set("explain", func(args []Value) ([]Value, error) {
		parsed, err := qSQLArgs("q.explain", args)
		if err != nil {
			return nil, err
		}
		out, err := qExplainSQL(parsed)
		if err != nil {
			return nil, err
		}
		return []Value{out}, nil
	})
	set("cache_stats", func(args []Value) ([]Value, error) {
		if len(args) > 0 {
			return nil, fmt.Errorf("q.cache_stats: expected no arguments")
		}
		return []Value{TableValue(qCacheStatsTable())}, nil
	})
	set("fallback_stats", func(args []Value) ([]Value, error) {
		if len(args) > 0 {
			return nil, fmt.Errorf("q.fallback_stats: expected no arguments")
		}
		return []Value{TableValue(qFallbackStatsTable())}, nil
	})
	set("cache_clear", func(args []Value) ([]Value, error) {
		if len(args) > 0 {
			return nil, fmt.Errorf("q.cache_clear: expected no arguments")
		}
		qClearCaches()
		return []Value{TableValue(qCacheStatsTable())}, nil
	})
	set("set", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("q.set: expected path and frame")
		}
		path, err := qPathStringArg("q.set", args, 0, "path")
		if err != nil {
			return nil, err
		}
		frame, err := qDataFrameFromValue(args[1], "")
		if err != nil {
			return nil, fmt.Errorf("q.set: %w", err)
		}
		if err := data.SaveFrameDir(path, frame); err != nil {
			return nil, fmt.Errorf("q.set: %w", err)
		}
		return []Value{args[1]}, nil
	})
	set("save_splayed", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("q.save_splayed: expected frame and path")
		}
		frame, err := qDataFrameFromValue(args[0], "")
		if err != nil {
			return nil, fmt.Errorf("q.save_splayed: %w", err)
		}
		path, err := qPathStringArg("q.save_splayed", args, 1, "path")
		if err != nil {
			return nil, err
		}
		if err := data.SaveFrameDir(path, frame); err != nil {
			return nil, fmt.Errorf("q.save_splayed: %w", err)
		}
		return []Value{BoolValue(true)}, nil
	})
	set("load_splayed", func(args []Value) ([]Value, error) {
		path, err := qPathStringArg("q.load_splayed", args, 0, "path")
		if err != nil {
			return nil, err
		}
		frame, err := data.LoadFrameDir(path)
		if err != nil {
			return nil, fmt.Errorf("q.load_splayed: %w", err)
		}
		rows, err := qRowsFromDataFrame(frame)
		if err != nil {
			return nil, fmt.Errorf("q.load_splayed: %w", err)
		}
		return []Value{TableValue(rows)}, nil
	})
	set("save_partitioned", func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("q.save_partitioned: expected frame, path, and partition columns")
		}
		frame, err := qDataFrameFromValue(args[0], "")
		if err != nil {
			return nil, fmt.Errorf("q.save_partitioned: %w", err)
		}
		path, err := qPathStringArg("q.save_partitioned", args, 1, "path")
		if err != nil {
			return nil, err
		}
		cols, err := dataPartitionColumnArgs("q.save_partitioned", args[2:])
		if err != nil {
			return nil, err
		}
		if err := data.SavePartitionedFrameDir(path, frame, cols...); err != nil {
			return nil, fmt.Errorf("q.save_partitioned: %w", err)
		}
		return []Value{BoolValue(true)}, nil
	})
	set("load_partitioned", func(args []Value) ([]Value, error) {
		path, err := qPathStringArg("q.load_partitioned", args, 0, "path")
		if err != nil {
			return nil, err
		}
		filters, err := dataPartitionFilters("q.load_partitioned", args[1:])
		if err != nil {
			return nil, err
		}
		frame, err := data.LoadPartitionedFrameDir(path, filters)
		if err != nil {
			return nil, fmt.Errorf("q.load_partitioned: %w", err)
		}
		rows, err := qRowsFromDataFrame(frame)
		if err != nil {
			return nil, fmt.Errorf("q.load_partitioned: %w", err)
		}
		return []Value{TableValue(rows)}, nil
	})
	set("count", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("q.count: argument 1 required")
		}
		return []Value{IntValue(int64(qCount(args[0])))}, nil
	})
	return t
}

func qSQLArgs(name string, args []Value) (qSQLArgsResult, error) {
	if len(args) < 2 {
		return qSQLArgsResult{}, fmt.Errorf("%s: expected (frame, qSQL source) or (qSQL source, frames)", name)
	}
	if parsed, ok, err := qSQLArgs2(name, args[0], args[1]); ok || err != nil {
		if err != nil {
			return qSQLArgsResult{}, err
		}
		env := NilValue()
		if len(args) > 2 && args[1].IsString() {
			env = args[2]
			parsed.envValue = env
		}
		return parsed, nil
	}
	return qSQLArgsResult{}, fmt.Errorf("%s: expected one qSQL source string argument", name)
}

func qSQLArgs2(name string, a, b Value) (qSQLArgsResult, bool, error) {
	if a.IsString() {
		return qSQLArgsResult{frameValue: b, source: a.Str(), resolveSource: true, envValue: b}, true, nil
	}
	if b.IsString() {
		return qSQLArgsResult{frameValue: a, source: b.Str(), envValue: NilValue()}, true, nil
	}
	return qSQLArgsResult{}, false, nil
}

func qSQLFastArg2(name string, a, b Value) (Value, error) {
	parsed, ok, err := qSQLArgs2(name, a, b)
	if err != nil {
		return NilValue(), err
	}
	if !ok {
		return NilValue(), fmt.Errorf("%s: expected one qSQL source string argument", name)
	}
	return qRunSQL(name, parsed)
}

func dialectQ(body Value, opts *Table) ([]Value, error) {
	mode := dialectMode(opts)
	if mode != "" && mode != "eval" && mode != "parse" {
		return dialectUnknownMode("q", mode)
	}
	return qEvalSymbolic(body.String())
}

func qEvalSymbolic(src string) ([]Value, error) {
	cacheable := stdq.EvalSourceCacheable(src)
	var out any
	if cacheable {
		qEvalCacheMu.Lock()
		cached, ok := qEvalCache[src]
		if ok {
			qEvalStats.Hits++
		} else {
			qEvalStats.Misses++
		}
		qEvalCacheMu.Unlock()
		if ok {
			v, err := qEvalValueToValue(cached)
			if err != nil {
				return nil, fmt.Errorf("q dialect: %w", err)
			}
			return []Value{v}, nil
		}
	}
	out, err := stdq.Eval(src)
	if err != nil {
		return nil, fmt.Errorf("q dialect: %w", err)
	}
	if cacheable && stdq.EvalValueCacheable(out) {
		qEvalCacheMu.Lock()
		qEvalCacheStoreLocked(src, out)
		qEvalCacheMu.Unlock()
	}
	v, err := qEvalValueToValue(out)
	if err != nil {
		return nil, fmt.Errorf("q dialect: %w", err)
	}
	return []Value{v}, nil
}

func qSessionValue() *Table {
	state := stdq.NewEvalState(nil)
	var mu sync.Mutex
	t := NewTable()
	t.RawSetString("kind", StringValue("q_session"))
	t.RawSetString("eval", FunctionValue(&GoFunction{Name: "q.session.eval", Fn: func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("q.session.eval: argument 1 must be a q source string")
		}
		mu.Lock()
		out, err := state.Eval(args[0].Str())
		mu.Unlock()
		if err != nil {
			return nil, fmt.Errorf("q session: %w", err)
		}
		v, err := qEvalValueToValue(out)
		if err != nil {
			return nil, fmt.Errorf("q session: %w", err)
		}
		return []Value{v}, nil
	}}))
	return t
}

func qEvalCacheStoreLocked(src string, value any) {
	if _, ok := qEvalCache[src]; !ok {
		qEvalCacheOrder = append(qEvalCacheOrder, src)
	}
	qEvalCache[src] = value
	for len(qEvalCacheOrder) > qEvalCacheLimit {
		evict := qEvalCacheOrder[0]
		qEvalCacheOrder = qEvalCacheOrder[1:]
		delete(qEvalCache, evict)
		qEvalStats.Evictions++
	}
}

func qQueryKernelSupportCacheProbe(key string) (qQueryKernelSupportCacheEntry, bool) {
	qQueryKernelSupportCacheMu.Lock()
	entry, ok := qQueryKernelSupportCache[key]
	if ok {
		qQueryKernelSupportStats.Hits++
	} else {
		qQueryKernelSupportStats.Misses++
	}
	qQueryKernelSupportCacheMu.Unlock()
	return entry, ok
}

func qQueryKernelSupportCachePeek(key string) (qQueryKernelSupportCacheEntry, bool) {
	qQueryKernelSupportCacheMu.Lock()
	entry, ok := qQueryKernelSupportCache[key]
	qQueryKernelSupportCacheMu.Unlock()
	return entry, ok
}

func qQueryKernelSupportCacheStore(key string, entry qQueryKernelSupportCacheEntry) {
	if key == "" {
		return
	}
	qQueryKernelSupportCacheMu.Lock()
	if _, ok := qQueryKernelSupportCache[key]; !ok {
		qQueryKernelSupportCacheOrder = append(qQueryKernelSupportCacheOrder, key)
	}
	qQueryKernelSupportCache[key] = entry
	for len(qQueryKernelSupportCacheOrder) > qQueryKernelSupportCacheLimit {
		evict := qQueryKernelSupportCacheOrder[0]
		qQueryKernelSupportCacheOrder = qQueryKernelSupportCacheOrder[1:]
		delete(qQueryKernelSupportCache, evict)
		qQueryKernelSupportStats.Evictions++
	}
	qQueryKernelSupportCacheMu.Unlock()
}

func qQueryKernelSupportCacheKey(s *SoA, spec *Table, selects []qSelect) (string, bool) {
	if s == nil || spec == nil || len(selects) == 0 {
		return "", false
	}
	parts := make([]string, 0, len(selects)+4)
	parts = append(parts, "source="+qQueryNativeSoASchemaHash(s))
	for _, sel := range selects {
		sig, ok := qQueryKernelExprSignature(sel.Expr, 0)
		if !ok {
			return "", false
		}
		parts = append(parts, "select:"+sel.Name+"="+sig)
	}
	order, err := qOrderSpecs(spec.RawGetString("order_by"))
	if err != nil {
		return "", false
	}
	for _, ord := range order {
		dir := "asc"
		if ord.Desc {
			dir = "desc"
		}
		parts = append(parts, "order:"+ord.Column+":"+dir)
	}
	limit, err := qLimit(spec.RawGetString("limit"))
	if err != nil {
		return "", false
	}
	parts = append(parts, "limit="+strconv.Itoa(limit))
	return strings.Join(parts, "|"), true
}

func qQueryKernelExprSignature(expr Value, depth int) (string, bool) {
	if depth > 8 {
		return "", false
	}
	switch {
	case expr.IsNil():
		return "nil", true
	case expr.IsString():
		return "s:" + strconv.Quote(expr.Str()), true
	case expr.IsInt():
		return "i:" + strconv.FormatInt(expr.Int(), 10), true
	case expr.IsFloat():
		return "f:" + strconv.FormatFloat(expr.Float(), 'g', -1, 64), true
	case expr.IsBool():
		return "b:" + strconv.FormatBool(expr.Bool()), true
	case expr.IsTable():
		tbl := expr.Table()
		op := tbl.RawGetString("op")
		if op.IsNil() {
			op = tbl.RawGetInt(1)
		}
		if !op.IsString() {
			return "", false
		}
		left := tbl.RawGetString("left")
		if left.IsNil() {
			left = tbl.RawGetInt(2)
		}
		right := tbl.RawGetString("right")
		if right.IsNil() {
			right = tbl.RawGetInt(3)
		}
		leftSig, ok := qQueryKernelExprSignature(left, depth+1)
		if !ok {
			return "", false
		}
		rightSig, ok := qQueryKernelExprSignature(right, depth+1)
		if !ok {
			return "", false
		}
		return "op:" + strconv.Quote(op.Str()) + "(" + leftSig + "," + rightSig + ")", true
	default:
		return "", false
	}
}

func qEvalValueToValue(v any) (Value, error) {
	if data.IsNull(v) {
		if kind, ok := data.NullKind(v); ok {
			return dataTypedNullValue(kind), nil
		}
		return dataNullValue(), nil
	}
	switch x := v.(type) {
	case nil:
		return NilValue(), nil
	case bool:
		return BoolValue(x), nil
	case int:
		return IntValue(int64(x)), nil
	case int8:
		return IntValue(int64(x)), nil
	case int16:
		return IntValue(int64(x)), nil
	case int32:
		return IntValue(int64(x)), nil
	case int64:
		return IntValue(x), nil
	case uint8:
		return IntValue(int64(x)), nil
	case uint16:
		return IntValue(int64(x)), nil
	case uint32:
		return IntValue(int64(x)), nil
	case uint64:
		return IntValue(int64(x)), nil
	case float32:
		return FloatValue(float64(x)), nil
	case float64:
		return FloatValue(x), nil
	case string:
		return StringValue(x), nil
	case data.Symbol:
		return StringValue(string(x)), nil
	case data.Month, data.Date, data.DateTime, data.Timespan, data.Minute, data.Second, data.Time, data.Timestamp:
		if s, ok := stdq.FormatTemporal(x); ok {
			return StringValue(s), nil
		}
		return StringValue(fmt.Sprint(x)), nil
	case data.Array:
		return qEvalArrayValue(x)
	case stdq.Dict:
		return qEvalDictValue(x)
	case data.Frame:
		rows, err := qRowsFromDataFrame(x)
		if err != nil {
			return NilValue(), err
		}
		return TableValue(rows), nil
	case data.KeyedFrame:
		return qKeyedFrameToValue(x), nil
	default:
		return StringValue(fmt.Sprint(x)), nil
	}
}

func qEvalArrayValue(array data.Array) (Value, error) {
	if array.Len() == 0 {
		if kind := array.Kind(); kind != "" && kind != data.KindAny && kind != data.KindNull {
			return qEvalGenericArrayTable(array)
		}
	}
	switch array.Kind() {
	case data.KindI64:
		if dataArrayHasNull(array) {
			return qEvalGenericArrayTable(array)
		}
		xs := make([]int64, array.Len())
		for i := range xs {
			v, ok := array.At(i)
			if !ok {
				return NilValue(), fmt.Errorf("array row %d out of range", i)
			}
			n, ok := v.(int64)
			if !ok {
				return NilValue(), fmt.Errorf("i64 array row %d has %T", i, v)
			}
			xs[i] = n
		}
		return DenseArrayValue(NewDenseArrayI64(xs)), nil
	case data.KindF64:
		if dataArrayHasNull(array) {
			return qEvalGenericArrayTable(array)
		}
		xs := make([]float64, array.Len())
		for i := range xs {
			v, ok := array.At(i)
			if !ok {
				return NilValue(), fmt.Errorf("array row %d out of range", i)
			}
			switch n := v.(type) {
			case float64:
				xs[i] = n
			case int64:
				xs[i] = float64(n)
			default:
				return NilValue(), fmt.Errorf("f64 array row %d has %T", i, v)
			}
		}
		return DenseArrayValue(NewDenseArrayF64(xs)), nil
	case data.KindBool:
		if dataArrayHasNull(array) {
			return qEvalGenericArrayTable(array)
		}
		out, err := NewDenseArrayOfLen(DenseArrayBool, array.Len())
		if err != nil {
			return NilValue(), err
		}
		for i := 0; i < array.Len(); i++ {
			v, ok := array.At(i)
			if !ok {
				return NilValue(), fmt.Errorf("array row %d out of range", i)
			}
			b, ok := v.(bool)
			if !ok {
				return NilValue(), fmt.Errorf("bool array row %d has %T", i, v)
			}
			if err := out.Set(i, BoolValue(b)); err != nil {
				return NilValue(), err
			}
		}
		return DenseArrayValue(out), nil
	case data.KindSymbol:
		if dataArrayHasNull(array) {
			return qEvalGenericArrayTable(array)
		}
		keys := make([]string, array.Len())
		for i := range keys {
			v, ok := array.At(i)
			if !ok {
				return NilValue(), fmt.Errorf("array row %d out of range", i)
			}
			switch s := v.(type) {
			case data.Symbol:
				keys[i] = string(s)
			case string:
				keys[i] = s
			default:
				return NilValue(), fmt.Errorf("symbol array row %d has %T", i, v)
			}
		}
		return qSymbolListValue(keys), nil
	case data.KindMonth, data.KindDate, data.KindDateTime, data.KindTimespan, data.KindMinute, data.KindSecond, data.KindTime, data.KindTimestamp:
		out := NewAppendArrayTable(array.Len())
		for i := 0; i < array.Len(); i++ {
			item, ok := array.At(i)
			if !ok {
				return NilValue(), fmt.Errorf("array row %d out of range", i)
			}
			if data.IsNull(item) {
				out.RawSetInt(int64(i+1), qAnyToColumnValue(data.NullForKind(array.Kind())))
				continue
			}
			text, ok := stdq.FormatTemporal(item)
			if !ok {
				return NilValue(), fmt.Errorf("%s array row %d has %T", array.Kind(), i, item)
			}
			out.RawSetInt(int64(i+1), StringValue(text))
		}
		return TableValue(out), nil
	default:
		return qEvalGenericArrayTable(array)
	}
}

func dataArrayHasNull(array data.Array) bool {
	for i := 0; i < array.Len(); i++ {
		item, ok := array.At(i)
		if !ok || data.IsNull(item) {
			return true
		}
	}
	return false
}

func qEvalGenericArrayTable(array data.Array) (Value, error) {
	return dataArrayFacadeValue(array, qAnyToColumnValue), nil
}

func qEvalDictValue(dict stdq.Dict) (Value, error) {
	if len(dict.Keys) != len(dict.Values) {
		return NilValue(), fmt.Errorf("dict key/value length mismatch")
	}
	out := NewTable()
	order := NewAppendArrayTable(len(dict.Keys))
	for i, key := range dict.Keys {
		v, err := qEvalValueToValue(dict.Values[i])
		if err != nil {
			return NilValue(), err
		}
		keyString := qEvalDictKeyString(key)
		out.RawSetString(keyString, v)
		order.RawSetInt(int64(i+1), StringValue(keyString))
	}
	out.RawSetString(qDictKeysMarker, TableValue(order))
	return TableValue(out), nil
}

func qEvalDictKeyString(key any) string {
	switch x := key.(type) {
	case data.Symbol:
		return string(x)
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

func qSymbolListValue(keys []string) Value {
	out := NewAppendArrayTable(len(keys))
	for i, key := range keys {
		out.RawSetInt(int64(i+1), StringValue(key))
	}
	out.RawSetString(qSymbolVectorMarker, BoolValue(true))
	return TableValue(out)
}

func qDataSymbolListValue(keys []data.Symbol) Value {
	out := NewAppendArrayTable(len(keys))
	for i, key := range keys {
		out.RawSetInt(int64(i+1), StringValue(string(key)))
	}
	out.RawSetString(qSymbolVectorMarker, BoolValue(true))
	return TableValue(out)
}

func qIsSymbolVector(v Value) bool {
	return v.IsTable() && v.Table().RawGetString(qSymbolVectorMarker).Truthy()
}

func qSymbolVectorSymbols(v Value) ([]data.Symbol, bool) {
	if !v.IsTable() {
		return nil, false
	}
	tbl := v.Table()
	items := make(map[int64]string)
	tbl.ForEachPlainRaw(func(key, val Value) bool {
		if key.IsInt() && key.Int() >= 1 && val.IsString() {
			items[key.Int()] = val.Str()
		}
		return true
	})
	if len(items) == 0 {
		if qIsSymbolVector(v) {
			return []data.Symbol{}, true
		}
		return nil, false
	}
	keys := make([]int64, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([]data.Symbol, 0, len(keys))
	for _, key := range keys {
		out = append(out, data.Symbol(items[key]))
	}
	return out, true
}

func qDictionaryValues(v Value) (Value, bool) {
	if !v.IsTable() {
		return NilValue(), false
	}
	tbl := v.Table()
	if qLooksLikeFrame(tbl) || qIsKeyedFrameTable(tbl) {
		return NilValue(), false
	}
	if keys, ok := qDictionaryKeyOrder(tbl); ok {
		out := NewAppendArrayTable(len(keys))
		for i, key := range keys {
			out.RawSetInt(int64(i+1), tbl.RawGetString(string(key)))
		}
		return TableValue(out), true
	}
	keys := make([]string, 0)
	tbl.ForEachPlainRaw(func(key, _ Value) bool {
		if key.IsString() {
			keys = append(keys, key.Str())
		}
		return true
	})
	if len(keys) == 0 {
		return NilValue(), false
	}
	sort.Strings(keys)
	out := NewAppendArrayTable(len(keys))
	for i, key := range keys {
		out.RawSetInt(int64(i+1), tbl.RawGetString(key))
	}
	return TableValue(out), true
}

func qDictionaryKeyOrder(tbl *Table) ([]data.Symbol, bool) {
	order := tbl.RawGetString(qDictKeysMarker)
	if !order.IsTable() {
		return nil, false
	}
	keys, err := qSymbolsFromArgs("q.keys", []Value{order})
	if err != nil {
		return nil, false
	}
	return keys, true
}

func qVectorLen(v Value) int {
	if v.IsDenseArray() {
		return v.DenseArray().Len()
	}
	if v.IsTable() {
		return v.Table().Length()
	}
	return 0
}

func qCount(v Value) int {
	switch {
	case v.IsNil():
		return 0
	case v.IsDenseArray(), v.IsTable():
		return qVectorLen(v)
	case v.IsString():
		return len(v.Str())
	default:
		return 1
	}
}

func qRunQuery(s *SoA, spec *Table) (*Table, error) {
	mask, err := qQueryMask(s, spec.RawGetString("where"))
	if err != nil {
		return nil, err
	}
	selects, err := qQuerySelects(spec.RawGetString("select"))
	if err != nil {
		return nil, err
	}
	if len(selects) == 0 {
		for _, name := range s.ColumnNames() {
			selects = append(selects, qSelect{Name: name, Expr: StringValue(name)})
		}
	}
	by, err := qStringList("q.query by", spec.RawGetString("by"))
	if err != nil {
		return nil, err
	}
	aggs, err := qAggregates(spec.RawGetString("aggregate"))
	if err != nil {
		return nil, err
	}
	var rows *Table
	var nativeRows *SoA
	nativeReasonCode := ""
	nativeReason := ""
	nativeCacheKey := ""
	nativeCacheable := false
	nativeCacheEntry := qQueryKernelSupportCacheEntry{}
	nativeCacheHit := false
	if len(aggs) == 0 {
		nativeCacheKey, nativeCacheable = qQueryKernelSupportCacheKey(s, spec, selects)
		if nativeCacheable {
			nativeCacheEntry, nativeCacheHit = qQueryKernelSupportCacheProbe(nativeCacheKey)
		}
		if nativeCacheHit && !nativeCacheEntry.Supported {
			nativeReasonCode = nativeCacheEntry.ReasonCode
			nativeReason = nativeCacheEntry.Reason
		} else {
			nativeRows, nativeReasonCode, nativeReason = qSimpleSelectRowsNativeSoA(s, mask, selects)
		}
		rows, err = qRows(s, mask, selects)
	} else {
		rows, err = qGroupedRows(s, mask, by, selects, aggs)
	}
	if err != nil {
		return nil, err
	}
	rows, err = qApplyOrderAndLimit(rows, spec)
	if err != nil {
		return nil, err
	}
	nativeRows, nativeResultOK, nativeResultReasonCode, nativeResultReason := qQueryNativeRowsForResult(spec, nativeRows)
	if nativeResultOK {
		qAttachRowsNativeSoAPayload(rows, nativeRows)
		if len(aggs) == 0 {
			if nativeCacheable {
				qQueryKernelSupportCacheStore(nativeCacheKey, qQueryKernelSupportCacheEntry{
					Supported:  true,
					ReasonCode: qKernelReasonSupported,
					Reason:     qKernelReasonSupported,
					SchemaHash: qQueryNativeSoASchemaHash(nativeRows),
				})
			}
			qRecordQueryKernelHit()
		}
	} else {
		if len(aggs) == 0 {
			if nativeReason == "" {
				nativeReasonCode = nativeResultReasonCode
				nativeReason = nativeResultReason
			}
			if nativeReason == "" {
				nativeReasonCode = qQueryKernelReasonOrder
				nativeReason = "query native kernel could not preserve order or limit"
			}
			if nativeCacheable {
				qQueryKernelSupportCacheStore(nativeCacheKey, qQueryKernelSupportCacheEntry{
					Supported:  false,
					ReasonCode: nativeReasonCode,
					Reason:     nativeReason,
				})
			}
			qRecordFallbackReason(qFallbackQueryKernel, nativeReasonCode, nativeReason)
		}
		qAttachRowsNativeFramePayload(rows)
	}
	return rows, nil
}

func qExplainQuery(s *SoA, spec *Table) (*Table, error) {
	mask, err := qQueryMask(s, spec.RawGetString("where"))
	if err != nil {
		return nil, err
	}
	selects, err := qQuerySelects(spec.RawGetString("select"))
	if err != nil {
		return nil, err
	}
	if len(selects) == 0 {
		for _, name := range s.ColumnNames() {
			selects = append(selects, qSelect{Name: name, Expr: StringValue(name)})
		}
	}
	by, err := qStringList("q.query by", spec.RawGetString("by"))
	if err != nil {
		return nil, err
	}
	aggs, err := qAggregates(spec.RawGetString("aggregate"))
	if err != nil {
		return nil, err
	}
	out := NewTable()
	out.RawSetString("op", StringValue("query"))
	out.RawSetString("source_rows", IntValue(int64(s.Len())))
	out.RawSetString("where_mask_rows", IntValue(int64(mask.Len())))
	out.RawSetString("select_count", IntValue(int64(len(selects))))
	out.RawSetString("by_count", IntValue(int64(len(by))))
	out.RawSetString("aggregate_count", IntValue(int64(len(aggs))))
	kernelCached := false
	if len(aggs) == 0 {
		if key, ok := qQueryKernelSupportCacheKey(s, spec, selects); ok {
			_, kernelCached = qQueryKernelSupportCachePeek(key)
		}
	}
	out.RawSetString("kernel_cached", BoolValue(kernelCached))
	if len(aggs) != 0 {
		out.RawSetString("kernel_supported", BoolValue(false))
		out.RawSetString("kernel_reason_code", StringValue(qQueryKernelReasonUnsupported))
		out.RawSetString("kernel_reason", StringValue("query native kernel supports non-aggregate selects only"))
		out.RawSetString("source_schema_hash", StringValue(""))
		out.RawSetString("kernel_schema_hash", StringValue(""))
		out.RawSetString("kernel_schema", TableValue(NewAppendArrayTable(0)))
		out.RawSetString("kernel_rows", IntValue(0))
		out.RawSetString("kernel_columns", IntValue(0))
		return out, nil
	}
	nativeRows, reasonCode, reason := qSimpleSelectRowsNativeSoA(s, mask, selects)
	nativeRows, ok, resultReasonCode, resultReason := qQueryNativeRowsForResult(spec, nativeRows)
	if !ok {
		if reason == "" {
			reasonCode = resultReasonCode
			reason = resultReason
		}
		if reason == "" {
			reasonCode = qQueryKernelReasonOrder
			reason = "query native kernel could not preserve order or limit"
		}
		out.RawSetString("kernel_supported", BoolValue(false))
		out.RawSetString("kernel_reason_code", StringValue(reasonCode))
		out.RawSetString("kernel_reason", StringValue(reason))
		out.RawSetString("source_schema_hash", StringValue(""))
		out.RawSetString("kernel_schema_hash", StringValue(""))
		out.RawSetString("kernel_schema", TableValue(NewAppendArrayTable(0)))
		out.RawSetString("kernel_rows", IntValue(0))
		out.RawSetString("kernel_columns", IntValue(0))
		return out, nil
	}
	schemaHash := qQueryNativeSoASchemaHash(nativeRows)
	out.RawSetString("kernel_supported", BoolValue(true))
	out.RawSetString("kernel_reason_code", StringValue(qKernelReasonSupported))
	out.RawSetString("kernel_reason", StringValue(qKernelReasonSupported))
	out.RawSetString("source_schema_hash", StringValue(schemaHash))
	out.RawSetString("kernel_schema_hash", StringValue(schemaHash))
	out.RawSetString("kernel_schema", qExplainSoASchemaValue(nativeRows))
	out.RawSetString("kernel_rows", IntValue(int64(nativeRows.Len())))
	out.RawSetString("kernel_columns", IntValue(int64(len(nativeRows.ColumnNames()))))
	return out, nil
}

func qRunSQL(name string, args qSQLArgsResult) (Value, error) {
	tmpl, err := qSQLCachedPlanTemplate(name, args.source)
	if err != nil {
		return NilValue(), err
	}
	sourceName := ""
	if args.resolveSource {
		sourceName = tmpl.source
	}
	var frame data.Frame
	var source qSQLSourceCarrier
	hasSourceCarrier := false
	if tmpl.literalFrame != nil {
		frame = *tmpl.literalFrame
	} else if tmpl.sourcePath != "" {
		frame, err = data.LoadPartitionedFrameDir(tmpl.sourcePath, nil)
		if err != nil {
			qRecordFallbackReason(qFallbackSourceErr, qKernelReasonSourceUnavailable, err.Error())
			return NilValue(), fmt.Errorf("%s: load q path source %q: %w", name, tmpl.sourcePath, err)
		}
	} else {
		source, err = qSQLSourceCarrierFromValue(args.frameValue, sourceName)
		if err != nil {
			qRecordFallbackReason(qFallbackSourceErr, qKernelReasonSourceUnavailable, err.Error())
			return NilValue(), fmt.Errorf("%s: %w", name, err)
		}
		frame = source.frame
		hasSourceCarrier = true
	}
	bindings := qSQLScalarBindingsFromValue(args.envValue)
	if tmpl.mutation != nil {
		qRecordFallbackReason(qFallbackMutationPlan, qKernelReasonMutationPlan, "mutation plan cache requires QueryPlan fallback")
		var keyedSource data.KeyedFrame
		hasKeyedSource := false
		if hasSourceCarrier {
			keyedSource = source.keyed
			hasKeyedSource = source.hasKeyed
		}
		mutation := qSQLMutationForFrame(args.source, tmpl.mutation, frame)
		qBindMutationOuterScalars(mutation, frame.Schema(), bindings)
		if hasKeyedSource && (mutation.Kind == stdq.InsertQuery || mutation.Kind == stdq.UpsertQuery) {
			keyedOut, err := qRunSQLKeyedMutation(keyedSource, mutation)
			if err != nil {
				return NilValue(), fmt.Errorf("%s: exec: keyed mutation: %w", name, err)
			}
			return qKeyedFrameToValue(keyedOut), nil
		}
		out, err := qRunSQLMutation(frame, mutation)
		if err != nil {
			return NilValue(), fmt.Errorf("%s: exec: %w", name, err)
		}
		if len(tmpl.literalKeys) > 0 {
			keyedOut, err := data.KeyBy(out, tmpl.literalKeys...)
			if err != nil {
				return NilValue(), fmt.Errorf("%s: exec: keyed literal mutation: %w", name, err)
			}
			return qKeyedFrameToValue(keyedOut), nil
		}
		if hasKeyedSource {
			keyedOut, err := data.KeyBy(out, keyedSource.Keys()...)
			if err != nil {
				return NilValue(), fmt.Errorf("%s: exec: keyed mutation: %w", name, err)
			}
			return qKeyedFrameToValue(keyedOut), nil
		}
		return qDataFrameValue(out)
	}
	if len(qSQLTemplateJoins(tmpl)) > 0 {
		if !args.resolveSource {
			err := fmt.Errorf("join queries require a source table map")
			qRecordFallbackReason(qFallbackJoinErr, qKernelReasonJoinUnavailable, err.Error())
			return NilValue(), fmt.Errorf("%s: %w", name, err)
		}
		for _, join := range qSQLTemplateJoins(tmpl) {
			var err error
			frame, err = qApplySQLJoin(args.frameValue, frame, join)
			if err != nil {
				qRecordFallbackReason(qFallbackJoinErr, qKernelReasonJoinUnavailable, err.Error())
				return NilValue(), fmt.Errorf("%s: join: %w", name, err)
			}
		}
	}
	plan := qPrepareSQLPlanForFrame(args.source, tmpl.plan, frame, bindings, true)
	out, err := qRunSQLPlan(args.source, plan, frame)
	if err != nil {
		return NilValue(), fmt.Errorf("%s: exec: %w", name, err)
	}
	out, err = qDropHiddenSQLColumns(out, tmpl.hiddenCols)
	if err != nil {
		return NilValue(), fmt.Errorf("%s: exec: %w", name, err)
	}
	if tmpl.op == stdq.ExecQuery {
		if tmpl.execDict != nil {
			return qExecDictResultValue(out, tmpl.execDict)
		}
		return qExecResultValue(out)
	}
	return qDataFrameValue(out)
}

func qRunSQLPlan(src string, plan data.QueryPlan, frame data.Frame) (data.Frame, error) {
	kernel, ok, err := qSQLKernelForFrame(src, plan, frame)
	if err != nil {
		qRecordFallbackReason(qFallbackKernelCompileErr, qKernelReasonCompileError, err.Error())
		return data.Frame{}, err
	}
	if !ok {
		_, reason := data.QueryKernelSupportReason(plan)
		qRecordFallbackReason(qFallbackKernelUnsupported, stdq.KernelFallbackReasonCode(reason), reason)
		kernel = nil
	}
	return data.ExecQueryKernelOrPlan(kernel, plan, frame)
}

func qSQLTemplateJoins(tmpl qSQLPlanTemplate) []*stdq.JoinPlan {
	if len(tmpl.joins) > 0 {
		return tmpl.joins
	}
	if tmpl.join != nil {
		return []*stdq.JoinPlan{tmpl.join}
	}
	return nil
}

func qApplySQLJoin(sources Value, left data.Frame, join *stdq.JoinPlan) (data.Frame, error) {
	if join == nil {
		return left, nil
	}
	right, err := qSQLSourceCarrierFromValue(sources, join.Right)
	if err != nil {
		return data.Frame{}, err
	}
	switch join.Kind {
	case "inner":
		if right.hasKeyed {
			return data.InnerJoinKeyedOn(left, right.keyed, join.Keys...)
		}
		return data.InnerJoinOn(left, right.frame, join.Keys...)
	case "left":
		if right.hasKeyed {
			return data.LeftJoinKeyedOn(left, right.keyed, join.Keys...)
		}
		return data.LeftJoinOn(left, right.frame, join.Keys...)
	case "union":
		return data.UnionJoinOn(left, right.frame, join.Keys...)
	case "plus":
		return data.PlusJoinOn(left, right.frame, join.Keys...)
	case "asof", "asof0", "asof_fill", "asof_fill0":
		if len(join.Keys) < 1 {
			return data.Frame{}, fmt.Errorf("q asof join requires a time key")
		}
		timeKey := join.Keys[len(join.Keys)-1]
		partitionKeys := join.Keys[:len(join.Keys)-1]
		return data.AsofJoinOnWithOptions(left, right.frame, data.AsofJoinOptions{
			TimeKey:           timeKey,
			PartitionKeys:     partitionKeys,
			PreserveRightTime: join.Kind == "asof0" || join.Kind == "asof_fill0",
		})
	case "window", "window1":
		if len(join.Keys) < 1 {
			return data.Frame{}, fmt.Errorf("q window join requires a time key")
		}
		timeKey := join.Keys[len(join.Keys)-1]
		partitionKeys := join.Keys[:len(join.Keys)-1]
		if join.HasWindow || join.Kind == "window1" {
			return data.WindowJoinOnWithOptions(left, right.frame, data.WindowJoinOptions{
				TimeKey:       timeKey,
				PartitionKeys: partitionKeys,
				Low:           join.WindowLow,
				High:          join.WindowHigh,
				HasBounds:     join.HasWindow,
				Last:          join.Kind == "window1",
			})
		}
		return data.WindowJoinOn(left, right.frame, timeKey, partitionKeys...)
	default:
		return data.Frame{}, fmt.Errorf("q join kind %q is not supported", join.Kind)
	}
}

func qExplainSQL(args qSQLArgsResult) (Value, error) {
	tmpl, err := qSQLCachedPlanTemplate("q.explain", args.source)
	if err != nil {
		return NilValue(), err
	}
	out := NewTable()
	out.RawSetString("op", StringValue(string(tmpl.op)))
	out.RawSetString("source_query", StringValue(args.source))
	out.RawSetString("source", StringValue(tmpl.source))
	out.RawSetString("source_path", StringValue(tmpl.sourcePath))
	out.RawSetString("has_literal_frame", BoolValue(tmpl.literalFrame != nil))
	out.RawSetString("literal_keys", qSymbolsExplainValue(tmpl.literalKeys))
	out.RawSetString("hidden_columns", qSymbolsExplainValue(tmpl.hiddenCols))
	out.RawSetString("distinct", BoolValue(tmpl.plan.Distinct))
	out.RawSetString("where", StringValue(qExplainExpr(tmpl.plan.Where)))
	out.RawSetString("by", qSymbolsExplainValue(tmpl.plan.By))
	out.RawSetString("by_exprs", qExplainSelectItemsValue(tmpl.plan.ByExprs))
	out.RawSetString("select", qExplainSelectItemsValue(tmpl.plan.Select))
	out.RawSetString("aggregates", qExplainAggregatesValue(tmpl.plan.Aggregates))
	out.RawSetString("order_by", qExplainOrdersValue(tmpl.plan.OrderBy))
	out.RawSetString("limit", IntValue(int64(tmpl.plan.LimitN)))
	out.RawSetString("pre_project_order", BoolValue(tmpl.plan.PreProjectOrder))
	out.RawSetString("exec_dict", qExplainExecDictValue(tmpl.execDict))
	out.RawSetString("join", qExplainJoinValue(tmpl.join))
	joins := qSQLTemplateJoins(tmpl)
	out.RawSetString("joins", qExplainJoinsValue(joins))
	out.RawSetString("join_count", IntValue(int64(len(joins))))
	out.RawSetString("mutation", qExplainMutationValue(tmpl.mutation))
	out.RawSetString("cacheable", BoolValue(true))
	sourceInfo := qExplainSourceBridgeInfo(args, tmpl)
	out.RawSetString("source_bridge", StringValue(sourceInfo.bridge))
	out.RawSetString("source_native", BoolValue(sourceInfo.native))
	out.RawSetString("source_keyed", BoolValue(sourceInfo.keyed))
	out.RawSetString("source_keys", qSymbolsExplainValue(sourceInfo.keys))
	out.RawSetString("source_rows", IntValue(int64(sourceInfo.rows)))
	kernelInfo := qExplainKernelInfo(args, tmpl)
	out.RawSetString("source_schema", qExplainSchemaValue(kernelInfo.schema))
	out.RawSetString("source_schema_hash", StringValue(qExplainKernelSchemaHash(kernelInfo)))
	out.RawSetString("kernel_supported", BoolValue(kernelInfo.supported))
	out.RawSetString("kernel_cached", BoolValue(kernelInfo.cached))
	out.RawSetString("kernel_reason_code", StringValue(kernelInfo.reasonCode))
	out.RawSetString("kernel_reason", StringValue(kernelInfo.reason))
	return TableValue(out), nil
}

type qExplainKernelResult struct {
	schema     data.Schema
	schemaHash string
	supported  bool
	cached     bool
	reasonCode string
	reason     string
}

type qExplainSourceBridgeResult struct {
	bridge string
	native bool
	keyed  bool
	keys   []data.Symbol
	rows   int
}

type qSQLSourceCarrier struct {
	frame    data.Frame
	keyed    data.KeyedFrame
	hasKeyed bool
	info     NativePayloadInfo
	hasInfo  bool
	bridge   string
	native   bool
	keys     []data.Symbol
	rows     int
}

type qSQLSourceNotFoundError struct {
	name string
}

func (e qSQLSourceNotFoundError) Error() string {
	return fmt.Sprintf("source %q not found", e.name)
}

func qExplainSourceBridgeInfo(args qSQLArgsResult, tmpl qSQLPlanTemplate) qExplainSourceBridgeResult {
	switch {
	case tmpl.literalFrame != nil:
		return qExplainSourceBridgeResult{bridge: "literal_frame", native: true, rows: tmpl.literalFrame.Len()}
	case tmpl.sourcePath != "":
		frame, err := data.LoadPartitionedFrameDir(tmpl.sourcePath, nil)
		if err != nil {
			return qExplainSourceBridgeResult{bridge: "partitioned_frame_unavailable"}
		}
		return qExplainSourceBridgeResult{bridge: "partitioned_frame", native: true, rows: frame.Len()}
	}
	sourceName := ""
	if args.resolveSource {
		sourceName = tmpl.source
	}
	carrier, err := qSQLSourceCarrierFromValue(args.frameValue, sourceName)
	if err != nil {
		if _, ok := err.(qSQLSourceNotFoundError); ok {
			return qExplainSourceBridgeResult{bridge: "source_not_found"}
		}
		value, resolveErr := qResolveSQLSourceValue(args.frameValue, sourceName)
		if resolveErr != nil {
			value = args.frameValue
		}
		return qExplainSourceBridgeResult{bridge: qExplainUnavailableBridge(value)}
	}
	return qExplainSourceBridgeResult{
		bridge: carrier.bridge,
		native: carrier.native,
		keyed:  carrier.hasKeyed,
		keys:   carrier.keys,
		rows:   carrier.rows,
	}
}

func qExplainUnavailableBridge(value Value) string {
	if value.IsSoA() {
		return "soa_unavailable"
	}
	if !value.IsTable() {
		return "unsupported_source"
	}
	tbl := value.Table()
	if kind, ok := qNativeFrameRuntimeKind(tbl); ok {
		switch kind {
		case NativePayloadKeyedFrame:
			return "keyed_frame_unavailable"
		case NativePayloadDataFrame:
			return "frame_native_unavailable"
		}
		return "frame_native_unavailable"
	}
	if qIsKeyedFrameTable(tbl) {
		return "keyed_frame_unavailable"
	}
	if qLooksLikeFrame(tbl) {
		return "frame_wrapper_unavailable"
	}
	return "row_table_unavailable"
}

func qSQLSourceCarrierFromValue(v Value, sourceName string) (qSQLSourceCarrier, error) {
	source, err := qResolveSQLSourceValue(v, sourceName)
	if err != nil {
		return qSQLSourceCarrier{}, err
	}
	return qSQLSourceCarrierFromResolvedValue(source)
}

func qResolveSQLSourceValue(v Value, sourceName string) (Value, error) {
	if sourceName == "" || !qCanResolveSQLSourceFromTable(v) {
		return v, nil
	}
	source := v.Table().RawGetString(sourceName)
	if source.IsNil() {
		return NilValue(), qSQLSourceNotFoundError{name: sourceName}
	}
	return source, nil
}

func qCanResolveSQLSourceFromTable(v Value) bool {
	if !v.IsTable() {
		return false
	}
	tbl := v.Table()
	if _, ok := qNativeFrameRuntimeKind(tbl); ok {
		return false
	}
	return !qLooksLikeFrame(tbl) && !qIsKeyedFrameTable(tbl)
}

func qSQLSourceCarrierFromResolvedValue(v Value) (qSQLSourceCarrier, error) {
	if v.IsSoA() {
		frame, err := qDataFrameFromSoA(v.SoA())
		if err != nil {
			return qSQLSourceCarrier{}, err
		}
		return qSQLSourceCarrier{frame: frame, bridge: "soa", rows: frame.Len()}, nil
	}
	if !v.IsTable() {
		return qSQLSourceCarrier{}, fmt.Errorf("argument 1 must be a frame table or soa")
	}
	tbl := v.Table()
	if carrier, ok, err := qSQLNativeSourceCarrierFromTable(tbl); err != nil {
		return qSQLSourceCarrier{}, err
	} else if ok {
		return carrier, nil
	}
	if qIsKeyedFrameTable(tbl) {
		keyed, err := qKeyedFrameFromValue(v)
		if err != nil {
			return qSQLSourceCarrier{}, err
		}
		info, hasInfo := qFramePayloadInfo(tbl, NativePayloadKeyedFrame)
		native := false
		if qNativeFrameRuntimeKindMatches(tbl, NativePayloadKeyedFrame) {
			native = true
		}
		bridge := "keyed_frame_wrapper"
		if native {
			bridge = "keyed_frame_native"
		}
		frame := keyed.Frame()
		rows := qSourceCarrierRows(frame.Len(), info, hasInfo)
		return qSQLSourceCarrier{
			frame:    frame,
			keyed:    keyed,
			hasKeyed: true,
			info:     info,
			hasInfo:  hasInfo,
			bridge:   bridge,
			native:   native,
			keys:     keyed.Keys(),
			rows:     rows,
		}, nil
	}
	bridge := "row_table"
	native := false
	info, hasInfo := qFramePayloadInfo(tbl, NativePayloadDataFrame)
	if qNativeFrameRuntimeKindMatches(tbl, NativePayloadDataFrame) {
		bridge = "frame_native"
		native = true
	} else if qLooksLikeFrame(tbl) {
		bridge = "frame_wrapper"
	}
	frame, err := qDataFrameFromValue(v, "")
	if err != nil {
		return qSQLSourceCarrier{}, err
	}
	rows := qSourceCarrierRows(frame.Len(), info, hasInfo)
	return qSQLSourceCarrier{frame: frame, info: info, hasInfo: hasInfo, bridge: bridge, native: native, rows: rows}, nil
}

func qSQLNativeSourceCarrierFromTable(tbl *Table) (qSQLSourceCarrier, bool, error) {
	if tbl == nil {
		return qSQLSourceCarrier{}, false, nil
	}
	kind, ok := qNativeFrameRuntimeKind(tbl)
	if !ok {
		return qSQLSourceCarrier{}, false, nil
	}
	switch kind {
	case NativePayloadKeyedFrame:
		keyed, ok, err := qNativeKeyedFramePayload(tbl)
		if err != nil {
			return qSQLSourceCarrier{}, false, err
		}
		if !ok {
			return qSQLSourceCarrier{}, false, fmt.Errorf("native keyed frame payload is invalid")
		}
		frame := keyed.Frame()
		info, hasInfo := qFramePayloadInfo(tbl, NativePayloadKeyedFrame)
		rows := qSourceCarrierRows(frame.Len(), info, hasInfo)
		return qSQLSourceCarrier{
			frame:    frame,
			keyed:    keyed,
			hasKeyed: true,
			info:     info,
			hasInfo:  hasInfo,
			bridge:   "keyed_frame_native",
			native:   true,
			keys:     keyed.Keys(),
			rows:     rows,
		}, true, nil
	case NativePayloadDataFrame:
		frame, ok, err := qNativeDataFramePayload(tbl)
		if err != nil {
			return qSQLSourceCarrier{}, false, err
		}
		if !ok {
			return qSQLSourceCarrier{}, false, fmt.Errorf("native data frame payload is invalid")
		}
		info, hasInfo := qFramePayloadInfo(tbl, NativePayloadDataFrame)
		rows := qSourceCarrierRows(frame.Len(), info, hasInfo)
		return qSQLSourceCarrier{
			frame:   frame,
			info:    info,
			hasInfo: hasInfo,
			bridge:  "frame_native",
			native:  true,
			rows:    rows,
		}, true, nil
	default:
		return qSQLSourceCarrier{}, false, nil
	}
}

func qSourceCarrierRows(frameRows int, info NativePayloadInfo, hasInfo bool) int {
	if hasInfo && info.Rows == frameRows {
		return info.Rows
	}
	return frameRows
}

func qSourceCarrierSchemaHash(frame data.Frame, info NativePayloadInfo, hasInfo bool) string {
	actual := frame.SchemaFingerprint()
	if hasInfo && info.SchemaHash == actual {
		return info.SchemaHash
	}
	return actual
}

func qFramePayloadInfo(tbl *Table, kind NativePayloadKind) (NativePayloadInfo, bool) {
	if tbl == nil {
		return NativePayloadInfo{}, false
	}
	_, info, ok := qTypedNativeFramePayload(tbl)
	if !ok || info.Kind != kind {
		return NativePayloadInfo{}, false
	}
	return info, true
}

func qNativeFrameRuntimeKind(tbl *Table) (NativePayloadKind, bool) {
	if tbl == nil {
		return NativePayloadNone, false
	}
	if _, info, ok := qTypedNativeFramePayload(tbl); ok {
		return info.Kind, true
	}
	if kind, ok := tbl.NativePayloadKind(); ok {
		switch kind {
		case NativePayloadDataFrame, NativePayloadKeyedFrame:
			return kind, true
		default:
			return NativePayloadNone, false
		}
	}
	switch tbl.NativePayload().(type) {
	case data.Frame:
		return NativePayloadDataFrame, true
	case data.KeyedFrame:
		return NativePayloadKeyedFrame, true
	default:
		return NativePayloadNone, false
	}
}

func qNativeFrameRuntimeKindMatches(tbl *Table, want NativePayloadKind) bool {
	kind, ok := qNativeFrameRuntimeKind(tbl)
	return ok && kind == want
}

func qTypedNativeFramePayload(tbl *Table) (any, NativePayloadInfo, bool) {
	if tbl == nil {
		return nil, NativePayloadInfo{}, false
	}
	return tbl.NativeFramePayload()
}

func qTypedNativeDataFramePayload(tbl *Table) (data.Frame, bool, error) {
	payload, info, ok := qTypedNativeFramePayload(tbl)
	if !ok {
		return data.Frame{}, false, nil
	}
	if info.Kind != NativePayloadDataFrame {
		return data.Frame{}, false, nil
	}
	switch native := payload.(type) {
	case data.Frame:
		return native, true, nil
	case *SoA:
		frame, err := qDataFrameFromSoA(native)
		if err != nil {
			return data.Frame{}, false, err
		}
		return frame, true, nil
	default:
		return data.Frame{}, false, fmt.Errorf("native data frame payload is invalid")
	}
}

func qTypedNativeKeyedFramePayload(tbl *Table) (data.KeyedFrame, bool, error) {
	payload, info, ok := qTypedNativeFramePayload(tbl)
	if !ok {
		return data.KeyedFrame{}, false, nil
	}
	if info.Kind != NativePayloadKeyedFrame {
		return data.KeyedFrame{}, false, nil
	}
	native, hasPayload := payload.(data.KeyedFrame)
	if !hasPayload {
		return data.KeyedFrame{}, false, fmt.Errorf("native keyed frame payload is invalid")
	}
	return native, true, nil
}

func qExplainKernelInfo(args qSQLArgsResult, tmpl qSQLPlanTemplate) qExplainKernelResult {
	if tmpl.mutation != nil {
		return qExplainKernelResult{reasonCode: qKernelReasonMutationPlan, reason: "mutation queries use mutation plan cache"}
	}
	var frame data.Frame
	var source qSQLSourceCarrier
	var hasSource bool
	var err error
	sourceName := tmpl.source
	switch {
	case tmpl.literalFrame != nil:
		frame = *tmpl.literalFrame
	case tmpl.sourcePath != "":
		frame, err = data.LoadPartitionedFrameDir(tmpl.sourcePath, nil)
	default:
		if args.resolveSource {
			sourceName = tmpl.source
		} else {
			sourceName = ""
		}
		source, err = qSQLSourceCarrierFromValue(args.frameValue, sourceName)
		hasSource = true
		frame = source.frame
	}
	if err != nil {
		return qExplainKernelResult{reasonCode: qKernelReasonSourceUnavailable, reason: err.Error()}
	}
	if len(qSQLTemplateJoins(tmpl)) > 0 {
		frame, err = qExplainJoinFrame(args, tmpl, frame)
		if err != nil {
			return qExplainKernelResult{schema: frame.Schema(), reasonCode: qKernelReasonJoinUnavailable, reason: err.Error()}
		}
		hasSource = false
	}
	bindings := qSQLScalarBindingsFromValue(args.envValue)
	plan := qPrepareSQLPlanForFrame(args.source, tmpl.plan, frame, bindings, false)
	kernelKey := data.QueryKernelCacheKey(args.source, frame, plan)
	qSQLAlignedPlanCacheMu.Lock()
	_, cached := qSQLKernelCache[kernelKey]
	qSQLAlignedPlanCacheMu.Unlock()
	schemaHash := ""
	if hasSource && source.hasInfo {
		schemaHash = qSourceCarrierSchemaHash(frame, source.info, source.hasInfo)
	}
	supported, reason, err := data.QueryKernelCompileReason(frame, plan)
	if err != nil {
		return qExplainKernelResult{schema: frame.Schema(), schemaHash: schemaHash, cached: cached, reasonCode: qKernelReasonCompileError, reason: err.Error()}
	}
	if !supported {
		return qExplainKernelResult{schema: frame.Schema(), schemaHash: schemaHash, cached: cached, reasonCode: stdq.KernelFallbackReasonCode(reason), reason: reason}
	}
	return qExplainKernelResult{schema: frame.Schema(), schemaHash: schemaHash, supported: true, cached: cached, reasonCode: qKernelReasonSupported, reason: reason}
}

func qExplainKernelSchemaHash(info qExplainKernelResult) string {
	if info.schemaHash != "" {
		return info.schemaHash
	}
	return info.schema.Fingerprint()
}

func qExplainJoinFrame(args qSQLArgsResult, tmpl qSQLPlanTemplate, frame data.Frame) (data.Frame, error) {
	if len(qSQLTemplateJoins(tmpl)) == 0 {
		return frame, nil
	}
	if !args.resolveSource {
		return data.Frame{}, fmt.Errorf("join queries require a source table map")
	}
	for _, join := range qSQLTemplateJoins(tmpl) {
		var err error
		frame, err = qApplySQLJoin(args.frameValue, frame, join)
		if err != nil {
			return data.Frame{}, fmt.Errorf("join: %w", err)
		}
	}
	return frame, nil
}

func qExplainSchemaValue(schema data.Schema) Value {
	names := schema.Names()
	out := NewAppendArrayTable(len(names))
	for i, name := range names {
		row := NewTable()
		row.RawSetString("name", StringValue(string(name)))
		if kind, ok := schema.Kind(name); ok {
			row.RawSetString("kind", StringValue(string(kind)))
		} else {
			row.RawSetString("kind", StringValue(""))
		}
		out.RawSetInt(int64(i+1), TableValue(row))
	}
	return TableValue(out)
}

func qExplainSoASchemaValue(soa *SoA) Value {
	names := soa.ColumnNames()
	out := NewAppendArrayTable(len(names))
	for i, name := range names {
		row := NewTable()
		row.RawSetString("name", StringValue(name))
		kind := ""
		if col, ok := soa.Column(name); ok && col != nil {
			kind = col.DType().String()
		}
		row.RawSetString("kind", StringValue(kind))
		out.RawSetInt(int64(i+1), TableValue(row))
	}
	return TableValue(out)
}

func qSymbolsExplainValue(symbols []data.Symbol) Value {
	out := NewAppendArrayTable(len(symbols))
	for i, sym := range symbols {
		out.RawSetInt(int64(i+1), StringValue(string(sym)))
	}
	return TableValue(out)
}

func qExplainSelectItemsValue(items []data.SelectItem) Value {
	out := NewAppendArrayTable(len(items))
	for i, item := range items {
		row := NewTable()
		row.RawSetString("name", StringValue(string(item.Name)))
		row.RawSetString("expr", StringValue(qExplainExpr(item.Expr)))
		out.RawSetInt(int64(i+1), TableValue(row))
	}
	return TableValue(out)
}

func qExplainAggregatesValue(items []data.Aggregate) Value {
	out := NewAppendArrayTable(len(items))
	for i, item := range items {
		row := NewTable()
		row.RawSetString("name", StringValue(string(item.Name)))
		row.RawSetString("fn", StringValue(item.Func))
		row.RawSetString("expr", StringValue(qExplainExpr(item.Expr)))
		row.RawSetString("weight", StringValue(qExplainExpr(item.Weight)))
		out.RawSetInt(int64(i+1), TableValue(row))
	}
	return TableValue(out)
}

func qExplainOrdersValue(items []data.OrderSpec) Value {
	out := NewAppendArrayTable(len(items))
	for i, item := range items {
		row := NewTable()
		row.RawSetString("column", StringValue(string(item.Column)))
		row.RawSetString("desc", BoolValue(item.Desc))
		out.RawSetInt(int64(i+1), TableValue(row))
	}
	return TableValue(out)
}

func qExplainExecDictValue(plan *stdq.ExecDictPlan) Value {
	if plan == nil {
		return NilValue()
	}
	out := NewTable()
	out.RawSetString("key", StringValue(string(plan.KeyName)))
	out.RawSetString("value", StringValue(string(plan.ValueName)))
	return TableValue(out)
}

func qExplainJoinValue(plan *stdq.JoinPlan) Value {
	return qExplainJoinValueAt(plan, 0)
}

func qExplainJoinValueAt(plan *stdq.JoinPlan, ordinal int) Value {
	if plan == nil {
		return NilValue()
	}
	out := NewTable()
	if ordinal > 0 {
		out.RawSetString("ordinal", IntValue(int64(ordinal)))
	}
	out.RawSetString("kind", StringValue(plan.Kind))
	out.RawSetString("right", StringValue(plan.Right))
	out.RawSetString("key_count", IntValue(int64(len(plan.Keys))))
	keys := NewAppendArrayTable(len(plan.Keys))
	leftPartitions := make([]data.Symbol, 0, len(plan.Keys))
	rightPartitions := make([]data.Symbol, 0, len(plan.Keys))
	for i, key := range plan.Keys {
		row := NewTable()
		row.RawSetString("left", StringValue(string(key.Left)))
		row.RawSetString("right", StringValue(string(key.Right)))
		keys.RawSetInt(int64(i+1), TableValue(row))
		if i < len(plan.Keys)-1 {
			leftPartitions = append(leftPartitions, key.Left)
			rightPartitions = append(rightPartitions, key.Right)
		}
	}
	out.RawSetString("keys", TableValue(keys))
	if qJoinKindUsesTimeKey(plan.Kind) && len(plan.Keys) > 0 {
		timeKey := plan.Keys[len(plan.Keys)-1]
		out.RawSetString("time_key", StringValue(string(timeKey.Left)))
		out.RawSetString("left_time_key", StringValue(string(timeKey.Left)))
		out.RawSetString("right_time_key", StringValue(string(timeKey.Right)))
		out.RawSetString("partition_keys", qSymbolsExplainValue(leftPartitions))
		out.RawSetString("left_partition_keys", qSymbolsExplainValue(leftPartitions))
		out.RawSetString("right_partition_keys", qSymbolsExplainValue(rightPartitions))
	}
	out.RawSetString("has_window", BoolValue(plan.HasWindow))
	out.RawSetString("window_low", qAnyToValue(plan.WindowLow))
	out.RawSetString("window_high", qAnyToValue(plan.WindowHigh))
	return TableValue(out)
}

func qExplainJoinsValue(plans []*stdq.JoinPlan) Value {
	out := NewAppendArrayTable(len(plans))
	for i, plan := range plans {
		out.RawSetInt(int64(i+1), qExplainJoinValueAt(plan, i+1))
	}
	return TableValue(out)
}

func qJoinKindUsesTimeKey(kind string) bool {
	switch kind {
	case "asof", "asof0", "asof_fill", "asof_fill0", "window", "window1":
		return true
	default:
		return false
	}
}

func qExplainMutationValue(plan *stdq.MutationPlan) Value {
	if plan == nil {
		return NilValue()
	}
	out := NewTable()
	out.RawSetString("kind", StringValue(string(plan.Kind)))
	out.RawSetString("where", StringValue(qExplainExpr(plan.Where)))
	out.RawSetString("by_exprs", qExplainSelectItemsValue(plan.ByExprs))
	assignments := NewAppendArrayTable(len(plan.Assignments))
	for i, item := range plan.Assignments {
		row := NewTable()
		row.RawSetString("name", StringValue(string(item.Name)))
		row.RawSetString("fn", StringValue(item.Func))
		row.RawSetString("expr", StringValue(qExplainExpr(item.Expr)))
		assignments.RawSetInt(int64(i+1), TableValue(row))
	}
	out.RawSetString("assignments", TableValue(assignments))
	out.RawSetString("delete_columns", qSymbolsExplainValue(plan.DeleteColumns))
	out.RawSetString("insert_columns", qSymbolsExplainValue(plan.InsertColumns))
	insertValues := NewAppendArrayTable(len(plan.InsertValues))
	for i, item := range plan.InsertValues {
		insertValues.RawSetInt(int64(i+1), StringValue(qExplainLiteral(item.Value)))
	}
	out.RawSetString("insert_values", TableValue(insertValues))
	return TableValue(out)
}

func qExplainExpr(expr data.Expr) string {
	switch e := expr.(type) {
	case nil:
		return ""
	case data.ColumnRef:
		return string(e.Name)
	case data.Literal:
		return qExplainLiteral(e.Value)
	case data.Binary:
		return "(" + qExplainExpr(e.Left) + " " + string(e.Op) + " " + qExplainExpr(e.Right) + ")"
	case data.Logical:
		return "(" + qExplainExpr(e.Left) + " " + e.Op + " " + qExplainExpr(e.Right) + ")"
	case data.Not:
		return "not " + qExplainExpr(e.Expr)
	case data.In:
		parts := make([]string, 0, len(e.Values))
		for _, value := range e.Values {
			parts = append(parts, qExplainLiteral(value))
		}
		return qExplainExpr(e.Expr) + " in (" + strings.Join(parts, " ") + ")"
	case data.Within:
		return qExplainExpr(e.Expr) + " within (" + qExplainLiteral(e.Low) + " " + qExplainLiteral(e.High) + ")"
	case data.BucketFloorExpr:
		return "xbar " + qExplainLiteral(e.Interval) + " " + qExplainExpr(e.Expr)
	case data.ListAggregateExpr:
		return e.Func + " " + qExplainExpr(e.Expr)
	case data.VectorTransformExpr:
		if e.Arg != nil {
			return qExplainExpr(e.Arg) + " " + e.Func + " " + qExplainExpr(e.Expr)
		}
		return e.Func + " " + qExplainExpr(e.Expr)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

func qExplainLiteral(value any) string {
	if data.IsNull(value) {
		return "null"
	}
	switch x := value.(type) {
	case string:
		return strconv.Quote(x)
	case data.Symbol:
		return "`" + string(x)
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case float32:
		return strconv.FormatFloat(float64(x), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case []any:
		parts := make([]string, 0, len(x))
		for _, item := range x {
			parts = append(parts, qExplainLiteral(item))
		}
		return "(" + strings.Join(parts, " ") + ")"
	}
	if s, ok := stdq.FormatTemporal(value); ok {
		return s
	}
	return fmt.Sprint(value)
}

func qSQLCachedPlanTemplate(name, src string) (qSQLPlanTemplate, error) {
	qSQLTemplateCacheMu.Lock()
	if tmpl, ok := qSQLTemplateCache[src]; ok {
		qSQLTemplateStats.TemplateHits++
		qSQLTemplateCacheMu.Unlock()
		tmpl.plan = qCloneDataQueryPlan(tmpl.plan)
		tmpl.sourcePath = qNormalizePathString(tmpl.sourcePath)
		tmpl.literalFrame = qCloneDataFramePtr(tmpl.literalFrame)
		tmpl.literalKeys = qCloneDataSymbols(tmpl.literalKeys)
		tmpl.hiddenCols = qCloneDataSymbols(tmpl.hiddenCols)
		tmpl.execDict = qCloneQExecDictPlan(tmpl.execDict)
		tmpl.mutation = qCloneQMutationPlan(tmpl.mutation)
		tmpl.join = qCloneQJoinPlan(tmpl.join)
		tmpl.joins = qCloneQJoinPlans(tmpl.joins)
		return tmpl, nil
	}
	qSQLTemplateStats.TemplateMisses++
	qSQLTemplateCacheMu.Unlock()

	query, err := stdq.Parse(strings.TrimSpace(src))
	if err != nil {
		return qSQLPlanTemplate{}, fmt.Errorf("%s: parse: %w", name, err)
	}
	lowered, err := stdq.Lower(query)
	if err != nil {
		return qSQLPlanTemplate{}, fmt.Errorf("%s: lower: %w", name, err)
	}
	if lowered.Mutation != nil {
		mutation := qCloneQMutationPlan(lowered.Mutation)
		qNormalizeMutationLiterals(mutation)
		tmpl := qSQLPlanTemplate{
			source:       lowered.Source,
			sourcePath:   qNormalizePathString(lowered.SourcePath),
			op:           lowered.Op,
			literalFrame: qCloneDataFramePtr(lowered.Frame),
			literalKeys:  qCloneDataSymbols(lowered.LiteralKeys),
			mutation:     qCloneQMutationPlan(mutation),
		}

		qSQLTemplateCacheMu.Lock()
		qSQLTemplateCacheStoreLocked(src, tmpl)
		qSQLTemplateCacheMu.Unlock()

		tmpl.literalFrame = qCloneDataFramePtr(tmpl.literalFrame)
		tmpl.literalKeys = qCloneDataSymbols(tmpl.literalKeys)
		tmpl.mutation = qCloneQMutationPlan(tmpl.mutation)
		return tmpl, nil
	}
	if lowered.Distinct {
		lowered.Plan.Distinct = true
	}
	qNormalizePlanLiterals(&lowered.Plan)
	if lowered.Frame == nil {
		lowered.Plan.Source = data.Frame{}
	}
	tmpl := qSQLPlanTemplate{
		source:       lowered.Source,
		sourcePath:   qNormalizePathString(lowered.SourcePath),
		op:           lowered.Op,
		literalFrame: qCloneDataFramePtr(lowered.Frame),
		literalKeys:  qCloneDataSymbols(lowered.LiteralKeys),
		hiddenCols:   qCloneDataSymbols(lowered.HiddenCols),
		plan:         qCloneDataQueryPlan(lowered.Plan),
		execDict:     qCloneQExecDictPlan(lowered.ExecDict),
		join:         qCloneQJoinPlan(lowered.Join),
		joins:        qCloneQJoinPlans(lowered.Joins),
	}

	qSQLTemplateCacheMu.Lock()
	qSQLTemplateCacheStoreLocked(src, tmpl)
	qSQLTemplateCacheMu.Unlock()

	tmpl.plan = qCloneDataQueryPlan(tmpl.plan)
	tmpl.literalFrame = qCloneDataFramePtr(tmpl.literalFrame)
	tmpl.literalKeys = qCloneDataSymbols(tmpl.literalKeys)
	tmpl.hiddenCols = qCloneDataSymbols(tmpl.hiddenCols)
	tmpl.execDict = qCloneQExecDictPlan(tmpl.execDict)
	tmpl.mutation = qCloneQMutationPlan(tmpl.mutation)
	tmpl.join = qCloneQJoinPlan(tmpl.join)
	tmpl.joins = qCloneQJoinPlans(tmpl.joins)
	return tmpl, nil
}

func qDropHiddenSQLColumns(frame data.Frame, hidden []data.Symbol) (data.Frame, error) {
	if len(hidden) == 0 {
		return frame, nil
	}
	return data.DropColumns(frame, hidden...)
}

func qPrepareSQLPlanForFrame(src string, tmpl data.QueryPlan, frame data.Frame, bindings map[data.Symbol]any, useAlignedCache bool) data.QueryPlan {
	var plan data.QueryPlan
	if useAlignedCache {
		plan = qSQLPlanForFrame(src, tmpl, frame)
	} else {
		plan = qAlignSQLPlanForFrame(tmpl, frame)
	}
	qBindPlanOuterScalars(&plan, frame.Schema(), bindings)
	plan.Source = frame
	return plan
}

func qSQLPlanForFrame(src string, tmpl data.QueryPlan, frame data.Frame) data.QueryPlan {
	key := data.QueryAlignedPlanCacheKey(src, frame)
	qSQLAlignedPlanCacheMu.Lock()
	if plan, ok := qSQLAlignedPlanCache[key]; ok {
		qSQLAlignedStats.AlignedHits++
		qSQLAlignedPlanCacheMu.Unlock()
		return qCloneDataQueryPlan(plan)
	}
	qSQLAlignedStats.AlignedMisses++
	qSQLAlignedPlanCacheMu.Unlock()

	plan := qAlignSQLPlanForFrame(tmpl, frame)

	qSQLAlignedPlanCacheMu.Lock()
	qSQLAlignedPlanCacheStoreLocked(key, plan)
	qSQLAlignedPlanCacheMu.Unlock()

	return qCloneDataQueryPlan(plan)
}

func qAlignSQLPlanForFrame(tmpl data.QueryPlan, frame data.Frame) data.QueryPlan {
	plan := qCloneDataQueryPlan(tmpl)
	qAlignPlanLiteralsToFrame(&plan, frame)
	qExpandAllColumnSelects(&plan, frame)
	qSetPreProjectOrder(&plan, frame)
	return plan
}

func qSQLMutationForFrame(src string, tmpl *stdq.MutationPlan, frame data.Frame) *stdq.MutationPlan {
	key := data.QueryAlignedMutationCacheKey(src, frame)
	qSQLAlignedPlanCacheMu.Lock()
	if mutation, ok := qSQLAlignedMutationCache[key]; ok {
		qSQLAlignedStats.AlignedHits++
		qSQLAlignedPlanCacheMu.Unlock()
		return qCloneQMutationPlan(mutation)
	}
	qSQLAlignedStats.AlignedMisses++
	qSQLAlignedPlanCacheMu.Unlock()

	mutation := qCloneQMutationPlan(tmpl)
	qAlignMutationLiteralsToFrame(mutation, frame)

	qSQLAlignedPlanCacheMu.Lock()
	qSQLAlignedMutationCacheStoreLocked(key, mutation)
	qSQLAlignedPlanCacheMu.Unlock()

	return qCloneQMutationPlan(mutation)
}

func qSQLKernelForFrame(src string, plan data.QueryPlan, frame data.Frame) (*data.QueryKernel, bool, error) {
	key := data.QueryKernelCacheKey(src, frame, plan)
	qSQLAlignedPlanCacheMu.Lock()
	if kernel, ok := qSQLKernelCache[key]; ok {
		qSQLAlignedStats.KernelHits++
		qSQLAlignedPlanCacheMu.Unlock()
		return kernel, true, nil
	}
	qSQLAlignedPlanCacheMu.Unlock()

	kernel, ok, err := data.CompileQueryKernel(frame, plan)
	if err != nil || !ok {
		return nil, ok, err
	}

	qSQLAlignedPlanCacheMu.Lock()
	if cached, ok := qSQLKernelCache[key]; ok {
		qSQLAlignedStats.KernelHits++
		qSQLAlignedPlanCacheMu.Unlock()
		return cached, true, nil
	}
	qSQLAlignedStats.KernelMisses++
	qSQLKernelCacheStoreLocked(key, kernel)
	qSQLAlignedPlanCacheMu.Unlock()
	return kernel, true, nil
}

func qRunSQLMutation(frame data.Frame, mutation *stdq.MutationPlan) (data.Frame, error) {
	if mutation == nil {
		return data.Frame{}, fmt.Errorf("mutation plan is nil")
	}
	switch mutation.Kind {
	case stdq.UpdateQuery:
		if len(mutation.ByExprs) > 0 {
			assignments := make([]data.GroupedAssignment, len(mutation.Assignments))
			for i, assign := range mutation.Assignments {
				assignments[i] = data.GroupedAssignment{Name: assign.Name, Func: assign.Func, Expr: assign.Expr}
			}
			return data.UpdateBy(frame, mutation.Where, mutation.ByExprs, assignments)
		}
		assignments := make(map[data.Symbol]data.Expr, len(mutation.Assignments))
		for _, assign := range mutation.Assignments {
			assignments[assign.Name] = assign.Expr
		}
		return data.UpdateWhere(frame, mutation.Where, assignments)
	case stdq.DeleteQuery:
		if len(mutation.DeleteColumns) > 0 {
			return data.DropColumns(frame, mutation.DeleteColumns...)
		}
		return data.DeleteWhere(frame, mutation.Where)
	case stdq.InsertQuery:
		if err := qValidateSQLInsertMutation(frame.Schema(), nil, mutation); err != nil {
			return data.Frame{}, err
		}
		return data.InsertRow(frame, mutation.InsertColumns, qSQLMutationValues(mutation))
	case stdq.UpsertQuery:
		if err := qValidateSQLInsertMutation(frame.Schema(), nil, mutation); err != nil {
			return data.Frame{}, err
		}
		return data.UpsertRow(frame, mutation.InsertColumns, qSQLMutationValues(mutation))
	default:
		return data.Frame{}, fmt.Errorf("unsupported q mutation kind %q", mutation.Kind)
	}
}

func qRunSQLKeyedMutation(keyed data.KeyedFrame, mutation *stdq.MutationPlan) (data.KeyedFrame, error) {
	if mutation == nil {
		return data.KeyedFrame{}, fmt.Errorf("mutation plan is nil")
	}
	switch mutation.Kind {
	case stdq.InsertQuery:
		if err := qValidateSQLInsertMutation(keyed.Frame().Schema(), keyed.Keys(), mutation); err != nil {
			return data.KeyedFrame{}, err
		}
		return data.InsertRowKeyed(keyed, mutation.InsertColumns, qSQLMutationValues(mutation))
	case stdq.UpsertQuery:
		if err := qValidateSQLInsertMutation(keyed.Frame().Schema(), keyed.Keys(), mutation); err != nil {
			return data.KeyedFrame{}, err
		}
		return data.UpsertRowKeyed(keyed, mutation.InsertColumns, qSQLMutationValues(mutation))
	default:
		frame, err := qRunSQLMutation(keyed.Frame(), mutation)
		if err != nil {
			return data.KeyedFrame{}, err
		}
		return data.KeyBy(frame, keyed.Keys()...)
	}
}

func qValidateSQLInsertMutation(schema data.Schema, keys []data.Symbol, mutation *stdq.MutationPlan) error {
	if mutation == nil {
		return fmt.Errorf("mutation plan is nil")
	}
	op := string(mutation.Kind)
	values := len(mutation.InsertValues)
	if values == 0 {
		return fmt.Errorf("q %s requires at least one value", op)
	}
	names := schema.Names()
	if len(mutation.InsertColumns) == 0 {
		if values != len(names) {
			return fmt.Errorf("q %s values count %d does not match target schema column count %d (%s)", op, values, len(names), qFormatSymbolList(names))
		}
		return nil
	}
	if len(mutation.InsertColumns) != values {
		return fmt.Errorf("q %s column count %d does not match values count %d", op, len(mutation.InsertColumns), values)
	}
	seen := make(map[data.Symbol]struct{}, len(mutation.InsertColumns))
	for _, column := range mutation.InsertColumns {
		if column == "" {
			return fmt.Errorf("q %s column name must not be empty", op)
		}
		if _, ok := schema.Kind(column); !ok {
			return fmt.Errorf("q %s column %q does not exist in target schema (%s)", op, column, qFormatSymbolList(names))
		}
		if _, ok := seen[column]; ok {
			return fmt.Errorf("q %s column %q is duplicated", op, column)
		}
		seen[column] = struct{}{}
	}
	for _, key := range keys {
		if _, ok := seen[key]; !ok {
			return fmt.Errorf("q %s keyed mutation requires key column %q in insert column list (%s)", op, key, qFormatSymbolList(keys))
		}
	}
	return nil
}

func qFormatSymbolList(symbols []data.Symbol) string {
	if len(symbols) == 0 {
		return "<empty>"
	}
	parts := make([]string, len(symbols))
	for i, symbol := range symbols {
		parts[i] = string(symbol)
	}
	return strings.Join(parts, ",")
}

func qSQLMutationValues(mutation *stdq.MutationPlan) []any {
	values := make([]any, len(mutation.InsertValues))
	for i, value := range mutation.InsertValues {
		values[i] = value.Value
	}
	return values
}

func qSQLTemplateCacheStoreLocked(key string, tmpl qSQLPlanTemplate) {
	if _, ok := qSQLTemplateCache[key]; !ok {
		qSQLTemplateOrder = append(qSQLTemplateOrder, key)
	}
	tmpl.plan = qCloneDataQueryPlan(tmpl.plan)
	tmpl.literalFrame = qCloneDataFramePtr(tmpl.literalFrame)
	tmpl.literalKeys = qCloneDataSymbols(tmpl.literalKeys)
	tmpl.hiddenCols = qCloneDataSymbols(tmpl.hiddenCols)
	tmpl.execDict = qCloneQExecDictPlan(tmpl.execDict)
	tmpl.mutation = qCloneQMutationPlan(tmpl.mutation)
	tmpl.join = qCloneQJoinPlan(tmpl.join)
	tmpl.joins = qCloneQJoinPlans(tmpl.joins)
	qSQLTemplateCache[key] = tmpl
	for len(qSQLTemplateOrder) > qSQLPlanCacheLimit {
		evict := qSQLTemplateOrder[0]
		qSQLTemplateOrder = qSQLTemplateOrder[1:]
		delete(qSQLTemplateCache, evict)
		qSQLTemplateStats.TemplateEvictions++
	}
}

func qSQLAlignedPlanCacheStoreLocked(key string, plan data.QueryPlan) {
	if _, ok := qSQLAlignedPlanCache[key]; !ok {
		qSQLAlignedPlanOrder = append(qSQLAlignedPlanOrder, key)
	}
	qSQLAlignedPlanCache[key] = qCloneDataQueryPlan(plan)
	for len(qSQLAlignedPlanOrder) > qSQLPlanCacheLimit {
		evict := qSQLAlignedPlanOrder[0]
		qSQLAlignedPlanOrder = qSQLAlignedPlanOrder[1:]
		delete(qSQLAlignedPlanCache, evict)
		qSQLAlignedStats.AlignedEvictions++
	}
}

func qSQLAlignedMutationCacheStoreLocked(key string, mutation *stdq.MutationPlan) {
	if _, ok := qSQLAlignedMutationCache[key]; !ok {
		qSQLAlignedMutationOrder = append(qSQLAlignedMutationOrder, key)
	}
	qSQLAlignedMutationCache[key] = qCloneQMutationPlan(mutation)
	for len(qSQLAlignedMutationOrder) > qSQLPlanCacheLimit {
		evict := qSQLAlignedMutationOrder[0]
		qSQLAlignedMutationOrder = qSQLAlignedMutationOrder[1:]
		delete(qSQLAlignedMutationCache, evict)
		qSQLAlignedStats.AlignedEvictions++
	}
}

func qSQLKernelCacheStoreLocked(key string, kernel *data.QueryKernel) {
	if _, ok := qSQLKernelCache[key]; !ok {
		qSQLKernelOrder = append(qSQLKernelOrder, key)
	}
	qSQLKernelCache[key] = kernel
	for len(qSQLKernelOrder) > qSQLPlanCacheLimit {
		evict := qSQLKernelOrder[0]
		qSQLKernelOrder = qSQLKernelOrder[1:]
		delete(qSQLKernelCache, evict)
		qSQLAlignedStats.KernelEvictions++
	}
}

func qSQLPlanCacheStatsSnapshot() qSQLPlanCacheStats {
	qSQLTemplateCacheMu.Lock()
	stats := qSQLTemplateStats
	qSQLTemplateCacheMu.Unlock()

	qSQLAlignedPlanCacheMu.Lock()
	stats.AlignedHits = qSQLAlignedStats.AlignedHits
	stats.AlignedMisses = qSQLAlignedStats.AlignedMisses
	stats.AlignedEvictions = qSQLAlignedStats.AlignedEvictions
	stats.KernelHits = qSQLAlignedStats.KernelHits
	stats.KernelMisses = qSQLAlignedStats.KernelMisses
	stats.KernelEvictions = qSQLAlignedStats.KernelEvictions
	qSQLAlignedPlanCacheMu.Unlock()

	return stats
}

func qCacheStatsTable() *Table {
	qSQLTemplateCacheMu.Lock()
	templateEntries := len(qSQLTemplateCache)
	templateStats := qSQLTemplateStats
	qSQLTemplateCacheMu.Unlock()

	qSQLAlignedPlanCacheMu.Lock()
	alignedEntries := len(qSQLAlignedPlanCache) + len(qSQLAlignedMutationCache)
	kernelEntries := len(qSQLKernelCache)
	alignedStats := qSQLAlignedStats
	qSQLAlignedPlanCacheMu.Unlock()

	qEvalCacheMu.Lock()
	evalEntries := len(qEvalCache)
	evalStats := qEvalStats
	qEvalCacheMu.Unlock()

	qQueryKernelSupportCacheMu.Lock()
	queryKernelEntries := len(qQueryKernelSupportCache)
	queryKernelStats := qQueryKernelSupportStats
	qQueryKernelSupportCacheMu.Unlock()

	rows := NewAppendArrayTable(5)
	rows.RawSetInt(1, TableValue(qCacheStatsRow(
		"qsql_template",
		templateEntries,
		templateStats.TemplateHits,
		templateStats.TemplateMisses,
		templateStats.TemplateEvictions,
		qSQLPlanCacheLimit,
	)))
	rows.RawSetInt(2, TableValue(qCacheStatsRow(
		"qsql_aligned",
		alignedEntries,
		alignedStats.AlignedHits,
		alignedStats.AlignedMisses,
		alignedStats.AlignedEvictions,
		qSQLPlanCacheLimit,
	)))
	rows.RawSetInt(3, TableValue(qCacheStatsRow(
		"qsql_kernel",
		kernelEntries,
		alignedStats.KernelHits,
		alignedStats.KernelMisses,
		alignedStats.KernelEvictions,
		qSQLPlanCacheLimit,
	)))
	rows.RawSetInt(4, TableValue(qCacheStatsRow(
		"q_query_kernel",
		queryKernelEntries,
		queryKernelStats.Hits,
		queryKernelStats.Misses,
		queryKernelStats.Evictions,
		qQueryKernelSupportCacheLimit,
	)))
	rows.RawSetInt(5, TableValue(qCacheStatsRow(
		"q_eval",
		evalEntries,
		evalStats.Hits,
		evalStats.Misses,
		evalStats.Evictions,
		qEvalCacheLimit,
	)))
	return rows
}

func qCacheStatsRow(name string, entries, hits, misses, evictions, limit int) *Table {
	row := NewTable()
	row.RawSetString("cache", StringValue(name))
	row.RawSetString("entries", IntValue(int64(entries)))
	row.RawSetString("hits", IntValue(int64(hits)))
	row.RawSetString("misses", IntValue(int64(misses)))
	row.RawSetString("evictions", IntValue(int64(evictions)))
	row.RawSetString("limit", IntValue(int64(limit)))
	return row
}

func qRecordFallback(code string) {
	qRecordFallbackReason(code, "", "")
}

func qRecordQueryKernelHit() {
	qFallbackStatsMu.Lock()
	qFallbackCounters.QueryKernelHit++
	qFallbackStatsMu.Unlock()
}

func qRecordFallbackReason(code, reasonCode, reason string) {
	qFallbackStatsMu.Lock()
	if qFallbackCounters.ByReasonCode == nil {
		qFallbackCounters.ByReasonCode = make(map[qFallbackReasonCodeKey]int)
	}
	if qFallbackCounters.ByReason == nil {
		qFallbackCounters.ByReason = make(map[qFallbackReasonKey]int)
	}
	switch code {
	case qFallbackKernelUnsupported:
		qFallbackCounters.KernelUnsupported++
	case qFallbackKernelCompileErr:
		qFallbackCounters.KernelCompileErr++
	case qFallbackSourceErr:
		qFallbackCounters.SourceErr++
	case qFallbackJoinErr:
		qFallbackCounters.JoinErr++
	case qFallbackMutationPlan:
		qFallbackCounters.Mutation++
	case qFallbackQueryKernel:
		qFallbackCounters.QueryKernel++
	}
	reason = qNormalizeFallbackReason(reason)
	reasonCode = qNormalizeFallbackReasonCode(code, reasonCode, reason)
	if reasonCode != "" {
		qFallbackCounters.ByReasonCode[qFallbackReasonCodeKey{Code: code, ReasonCode: reasonCode}]++
	}
	if reason != "" {
		qFallbackCounters.ByReason[qFallbackReasonKey{Code: code, Reason: reason}]++
	}
	qFallbackStatsMu.Unlock()
}

func qFallbackStatsTable() *Table {
	qFallbackStatsMu.Lock()
	stats := qCloneFallbackStatsLocked()
	qFallbackStatsMu.Unlock()

	detailRows := qFallbackTopRows(stats, 10)
	rows := NewAppendArrayTable(7 + len(detailRows))
	rows.RawSetInt(1, TableValue(qFallbackStatsRow(qFallbackKernelUnsupported, stats.KernelUnsupported)))
	rows.RawSetInt(2, TableValue(qFallbackStatsRow(qFallbackKernelCompileErr, stats.KernelCompileErr)))
	rows.RawSetInt(3, TableValue(qFallbackStatsRow(qFallbackSourceErr, stats.SourceErr)))
	rows.RawSetInt(4, TableValue(qFallbackStatsRow(qFallbackJoinErr, stats.JoinErr)))
	rows.RawSetInt(5, TableValue(qFallbackStatsRow(qFallbackMutationPlan, stats.Mutation)))
	rows.RawSetInt(6, TableValue(qFallbackStatsRow(qQueryKernelSupported, stats.QueryKernelHit)))
	rows.RawSetInt(7, TableValue(qFallbackStatsRow(qFallbackQueryKernel, stats.QueryKernel)))
	for i, row := range detailRows {
		rows.RawSetInt(int64(i+8), TableValue(row))
	}
	return rows
}

func qFallbackStatsRow(code string, count int) *Table {
	row := NewTable()
	row.RawSetString("kind", StringValue("code"))
	row.RawSetString("code", StringValue(code))
	row.RawSetString("reason_code", StringValue(""))
	row.RawSetString("reason", StringValue(""))
	row.RawSetString("count", IntValue(int64(count)))
	return row
}

func qCloneFallbackStatsLocked() qFallbackStats {
	stats := qFallbackCounters
	if len(qFallbackCounters.ByReasonCode) > 0 {
		stats.ByReasonCode = make(map[qFallbackReasonCodeKey]int, len(qFallbackCounters.ByReasonCode))
		for key, count := range qFallbackCounters.ByReasonCode {
			stats.ByReasonCode[key] = count
		}
	}
	if len(qFallbackCounters.ByReason) > 0 {
		stats.ByReason = make(map[qFallbackReasonKey]int, len(qFallbackCounters.ByReason))
		for key, count := range qFallbackCounters.ByReason {
			stats.ByReason[key] = count
		}
	}
	return stats
}

func qFallbackTopRows(stats qFallbackStats, limit int) []*Table {
	rows := make([]*Table, 0, len(stats.ByReasonCode)+len(stats.ByReason))
	reasonCodeRows := make([]qFallbackReasonCodeRow, 0, len(stats.ByReasonCode))
	for key, count := range stats.ByReasonCode {
		reasonCodeRows = append(reasonCodeRows, qFallbackReasonCodeRow{Key: key, Count: count})
	}
	sort.Slice(reasonCodeRows, func(i, j int) bool {
		a, b := reasonCodeRows[i], reasonCodeRows[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Key.Code != b.Key.Code {
			return a.Key.Code < b.Key.Code
		}
		return a.Key.ReasonCode < b.Key.ReasonCode
	})
	for i, item := range reasonCodeRows {
		if i >= limit {
			break
		}
		rows = append(rows, qFallbackDetailStatsRow("reason_code", item.Key.Code, item.Key.ReasonCode, "", item.Count))
	}

	reasonRows := make([]qFallbackReasonRow, 0, len(stats.ByReason))
	for key, count := range stats.ByReason {
		reasonRows = append(reasonRows, qFallbackReasonRow{Key: key, Count: count})
	}
	sort.Slice(reasonRows, func(i, j int) bool {
		a, b := reasonRows[i], reasonRows[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Key.Code != b.Key.Code {
			return a.Key.Code < b.Key.Code
		}
		return a.Key.Reason < b.Key.Reason
	})
	for i, item := range reasonRows {
		if i >= limit {
			break
		}
		rows = append(rows, qFallbackDetailStatsRow("reason", item.Key.Code, "", item.Key.Reason, item.Count))
	}
	return rows
}

type qFallbackReasonCodeRow struct {
	Key   qFallbackReasonCodeKey
	Count int
}

type qFallbackReasonRow struct {
	Key   qFallbackReasonKey
	Count int
}

func qFallbackDetailStatsRow(kind, code, reasonCode, reason string, count int) *Table {
	row := NewTable()
	row.RawSetString("kind", StringValue(kind))
	row.RawSetString("code", StringValue(code))
	row.RawSetString("reason_code", StringValue(reasonCode))
	row.RawSetString("reason", StringValue(reason))
	row.RawSetString("count", IntValue(int64(count)))
	return row
}

func qNormalizeFallbackReasonCode(code, reasonCode, reason string) string {
	if code == qFallbackKernelUnsupported && (reasonCode == "" || reasonCode == qKernelReasonUnsupported) {
		return stdq.KernelFallbackReasonCode(reason)
	}
	if reasonCode != "" {
		return reasonCode
	}
	return code
}

func qNormalizeFallbackReason(reason string) string {
	reason = strings.Join(strings.Fields(reason), " ")
	const maxReasonLen = 240
	if len(reason) <= maxReasonLen {
		return reason
	}
	return reason[:maxReasonLen]
}

func qClearCaches() {
	qSQLTemplateCacheMu.Lock()
	qSQLTemplateCache = make(map[string]qSQLPlanTemplate)
	qSQLTemplateOrder = nil
	qSQLTemplateStats = qSQLPlanCacheStats{}
	qSQLTemplateCacheMu.Unlock()

	qSQLAlignedPlanCacheMu.Lock()
	qSQLAlignedPlanCache = make(map[string]data.QueryPlan)
	qSQLAlignedPlanOrder = nil
	qSQLAlignedMutationCache = make(map[string]*stdq.MutationPlan)
	qSQLAlignedMutationOrder = nil
	qSQLKernelCache = make(map[string]*data.QueryKernel)
	qSQLKernelOrder = nil
	qSQLAlignedStats = qSQLPlanCacheStats{}
	qSQLAlignedPlanCacheMu.Unlock()

	qEvalCacheMu.Lock()
	qEvalCache = make(map[string]any)
	qEvalCacheOrder = nil
	qEvalStats = qEvalCacheStats{}
	qEvalCacheMu.Unlock()

	qQueryKernelSupportCacheMu.Lock()
	qQueryKernelSupportCache = make(map[string]qQueryKernelSupportCacheEntry)
	qQueryKernelSupportCacheOrder = nil
	qQueryKernelSupportStats = qQueryKernelSupportCacheStats{}
	qQueryKernelSupportCacheMu.Unlock()

	qFallbackStatsMu.Lock()
	qFallbackCounters = qFallbackStats{}
	qFallbackStatsMu.Unlock()
}

func qCloneDataQueryPlan(plan data.QueryPlan) data.QueryPlan {
	plan.Source = data.Frame{}
	plan.Where = qCloneDataExpr(plan.Where)
	plan.By = append([]data.Symbol(nil), plan.By...)
	if plan.ByExprs != nil {
		byExprs := make([]data.SelectItem, len(plan.ByExprs))
		for i, item := range plan.ByExprs {
			byExprs[i] = data.SelectItem{Name: item.Name, Expr: qCloneDataExpr(item.Expr)}
		}
		plan.ByExprs = byExprs
	}
	plan.OrderBy = append([]data.OrderSpec(nil), plan.OrderBy...)
	if plan.Select != nil {
		selects := make([]data.SelectItem, len(plan.Select))
		for i, item := range plan.Select {
			selects[i] = data.SelectItem{Name: item.Name, Expr: qCloneDataExpr(item.Expr)}
		}
		plan.Select = selects
	}
	if plan.Aggregates != nil {
		aggs := make([]data.Aggregate, len(plan.Aggregates))
		for i, agg := range plan.Aggregates {
			aggs[i] = data.Aggregate{Name: agg.Name, Func: agg.Func, Expr: qCloneDataExpr(agg.Expr), Weight: qCloneDataExpr(agg.Weight)}
		}
		plan.Aggregates = aggs
	}
	return plan
}

func qCloneDataFramePtr(frame *data.Frame) *data.Frame {
	if frame == nil {
		return nil
	}
	out, err := frame.Gather(dataFrameAllIndexes(*frame))
	if err != nil {
		clone := *frame
		return &clone
	}
	return &out
}

func qCloneDataSymbols(in []data.Symbol) []data.Symbol {
	if in == nil {
		return nil
	}
	return append([]data.Symbol(nil), in...)
}

func dataFrameAllIndexes(frame data.Frame) []int {
	indexes := make([]int, frame.Len())
	for i := range indexes {
		indexes[i] = i
	}
	return indexes
}

func qCloneQMutationPlan(plan *stdq.MutationPlan) *stdq.MutationPlan {
	if plan == nil {
		return nil
	}
	out := *plan
	out.Where = qCloneDataExpr(out.Where)
	if plan.ByExprs != nil {
		out.ByExprs = make([]data.SelectItem, len(plan.ByExprs))
		for i, item := range plan.ByExprs {
			out.ByExprs[i] = data.SelectItem{Name: item.Name, Expr: qCloneDataExpr(item.Expr)}
		}
	}
	if plan.Assignments != nil {
		out.Assignments = make([]stdq.MutationAssignment, len(plan.Assignments))
		for i, assign := range plan.Assignments {
			out.Assignments[i] = stdq.MutationAssignment{Name: assign.Name, Func: assign.Func, Expr: qCloneDataExpr(assign.Expr)}
		}
	}
	out.DeleteColumns = append([]data.Symbol(nil), plan.DeleteColumns...)
	out.InsertColumns = append([]data.Symbol(nil), plan.InsertColumns...)
	out.InsertValues = append([]data.Literal(nil), plan.InsertValues...)
	return &out
}

func qCloneQJoinPlan(plan *stdq.JoinPlan) *stdq.JoinPlan {
	if plan == nil {
		return nil
	}
	out := *plan
	out.Keys = append([]data.JoinKey(nil), plan.Keys...)
	return &out
}

func qCloneQJoinPlans(plans []*stdq.JoinPlan) []*stdq.JoinPlan {
	if plans == nil {
		return nil
	}
	out := make([]*stdq.JoinPlan, len(plans))
	for i, plan := range plans {
		out[i] = qCloneQJoinPlan(plan)
	}
	return out
}

func qCloneQExecDictPlan(plan *stdq.ExecDictPlan) *stdq.ExecDictPlan {
	if plan == nil {
		return nil
	}
	out := *plan
	return &out
}

func qCloneDataExpr(expr data.Expr) data.Expr {
	switch x := expr.(type) {
	case nil:
		return nil
	case data.Binary:
		x.Left = qCloneDataExpr(x.Left)
		x.Right = qCloneDataExpr(x.Right)
		return x
	case data.Logical:
		x.Left = qCloneDataExpr(x.Left)
		x.Right = qCloneDataExpr(x.Right)
		return x
	case data.Not:
		x.Expr = qCloneDataExpr(x.Expr)
		return x
	case data.In:
		x.Expr = qCloneDataExpr(x.Expr)
		x.Values = append([]any(nil), x.Values...)
		return x
	case data.Within:
		x.Expr = qCloneDataExpr(x.Expr)
		return x
	case data.BucketFloorExpr:
		x.Expr = qCloneDataExpr(x.Expr)
		return x
	case data.ListAggregateExpr:
		x.Expr = qCloneDataExpr(x.Expr)
		return x
	case data.VectorTransformExpr:
		x.Expr = qCloneDataExpr(x.Expr)
		x.Arg = qCloneDataExpr(x.Arg)
		return x
	default:
		return expr
	}
}

func qSQLScalarBindingsFromValue(v Value) map[data.Symbol]any {
	if !v.IsTable() {
		return nil
	}
	bindings := make(map[data.Symbol]any)
	v.Table().ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() {
			return true
		}
		scalar, ok := qWireScalarFromValue(val)
		if !ok {
			return true
		}
		bindings[data.Symbol(key.Str())] = scalar
		return true
	})
	if len(bindings) == 0 {
		return nil
	}
	return bindings
}

func qBindPlanOuterScalars(plan *data.QueryPlan, schema data.Schema, bindings map[data.Symbol]any) {
	if plan == nil || len(bindings) == 0 {
		return
	}
	plan.Where = qAlignDataExpr(qBindDataExprOuterScalars(plan.Where, schema, bindings), schema)
	for i := range plan.Select {
		plan.Select[i].Expr = qAlignDataExpr(qBindDataExprOuterScalars(plan.Select[i].Expr, schema, bindings), schema)
	}
	for i := range plan.ByExprs {
		plan.ByExprs[i].Expr = qAlignDataExpr(qBindDataExprOuterScalars(plan.ByExprs[i].Expr, schema, bindings), schema)
	}
	for i := range plan.Aggregates {
		plan.Aggregates[i].Expr = qAlignDataExpr(qBindDataExprOuterScalars(plan.Aggregates[i].Expr, schema, bindings), schema)
		if plan.Aggregates[i].Weight != nil {
			plan.Aggregates[i].Weight = qAlignDataExpr(qBindDataExprOuterScalars(plan.Aggregates[i].Weight, schema, bindings), schema)
		}
	}
}

func qBindMutationOuterScalars(plan *stdq.MutationPlan, schema data.Schema, bindings map[data.Symbol]any) {
	if plan == nil || len(bindings) == 0 {
		return
	}
	plan.Where = qAlignDataExpr(qBindDataExprOuterScalars(plan.Where, schema, bindings), schema)
	for i := range plan.ByExprs {
		plan.ByExprs[i].Expr = qAlignDataExpr(qBindDataExprOuterScalars(plan.ByExprs[i].Expr, schema, bindings), schema)
	}
	for i := range plan.Assignments {
		plan.Assignments[i].Expr = qAlignAssignmentExpr(plan.Assignments[i].Name, qBindDataExprOuterScalars(plan.Assignments[i].Expr, schema, bindings), schema)
	}
}

func qBindDataExprOuterScalars(expr data.Expr, schema data.Schema, bindings map[data.Symbol]any) data.Expr {
	switch x := expr.(type) {
	case nil:
		return nil
	case data.ColumnRef:
		if _, ok := schema.Kind(x.Name); ok {
			return x
		}
		if value, ok := bindings[x.Name]; ok {
			return data.Literal{Value: value}
		}
		return x
	case data.Binary:
		x.Left = qBindDataExprOuterScalars(x.Left, schema, bindings)
		x.Right = qBindDataExprOuterScalars(x.Right, schema, bindings)
		return x
	case data.Logical:
		x.Left = qBindDataExprOuterScalars(x.Left, schema, bindings)
		x.Right = qBindDataExprOuterScalars(x.Right, schema, bindings)
		return x
	case data.Not:
		x.Expr = qBindDataExprOuterScalars(x.Expr, schema, bindings)
		return x
	case data.In:
		x.Expr = qBindDataExprOuterScalars(x.Expr, schema, bindings)
		return x
	case data.Within:
		x.Expr = qBindDataExprOuterScalars(x.Expr, schema, bindings)
		return x
	case data.BucketFloorExpr:
		x.Expr = qBindDataExprOuterScalars(x.Expr, schema, bindings)
		return x
	case data.ListAggregateExpr:
		x.Expr = qBindDataExprOuterScalars(x.Expr, schema, bindings)
		return x
	case data.VectorTransformExpr:
		x.Expr = qBindDataExprOuterScalars(x.Expr, schema, bindings)
		x.Arg = qBindDataExprOuterScalars(x.Arg, schema, bindings)
		return x
	default:
		return expr
	}
}

func qAlignPlanLiteralsToFrame(plan *data.QueryPlan, frame data.Frame) {
	if plan == nil {
		return
	}
	schema := frame.Schema()
	plan.Where = qAlignDataExpr(plan.Where, schema)
	for i := range plan.Select {
		plan.Select[i].Expr = qAlignDataExpr(plan.Select[i].Expr, schema)
	}
	for i := range plan.ByExprs {
		plan.ByExprs[i].Expr = qAlignDataExpr(plan.ByExprs[i].Expr, schema)
	}
	for i := range plan.Aggregates {
		plan.Aggregates[i].Expr = qAlignDataExpr(plan.Aggregates[i].Expr, schema)
		if plan.Aggregates[i].Weight != nil {
			plan.Aggregates[i].Weight = qAlignDataExpr(plan.Aggregates[i].Weight, schema)
		}
	}
}

func qExpandAllColumnSelects(plan *data.QueryPlan, frame data.Frame) {
	if plan == nil || len(plan.Select) == 0 {
		return
	}
	schema := frame.Schema()
	var expanded []data.SelectItem
	changed := false
	for _, item := range plan.Select {
		lit, ok := item.Expr.(data.Literal)
		if !ok {
			expanded = append(expanded, item)
			continue
		}
		if _, ok := lit.Value.(stdq.AllColumns); !ok {
			expanded = append(expanded, item)
			continue
		}
		changed = true
		for _, name := range schema.Names() {
			expanded = append(expanded, data.SelectItem{Name: name, Expr: data.ColumnRef{Name: name}})
		}
	}
	if changed {
		plan.Select = expanded
	}
}

func qSetPreProjectOrder(plan *data.QueryPlan, frame data.Frame) {
	if plan == nil || len(plan.OrderBy) == 0 || len(plan.Aggregates) > 0 || len(plan.By) > 0 || len(plan.ByExprs) > 0 {
		return
	}
	sourceSchema := frame.Schema()
	projected := make(map[data.Symbol]struct{}, len(plan.Select))
	for _, item := range plan.Select {
		projected[item.Name] = struct{}{}
	}
	for _, spec := range plan.OrderBy {
		if _, ok := projected[spec.Column]; ok {
			continue
		}
		if _, ok := sourceSchema.Kind(spec.Column); ok {
			plan.PreProjectOrder = true
			return
		}
	}
}

func qAlignMutationLiteralsToFrame(plan *stdq.MutationPlan, frame data.Frame) {
	if plan == nil {
		return
	}
	schema := frame.Schema()
	plan.Where = qAlignDataExpr(plan.Where, schema)
	for i := range plan.ByExprs {
		plan.ByExprs[i].Expr = qAlignDataExpr(plan.ByExprs[i].Expr, schema)
	}
	for i := range plan.Assignments {
		if plan.Assignments[i].Func != "" {
			plan.Assignments[i].Expr = qAlignDataExpr(plan.Assignments[i].Expr, schema)
		} else {
			plan.Assignments[i].Expr = qAlignAssignmentExpr(plan.Assignments[i].Name, plan.Assignments[i].Expr, schema)
		}
	}
	for i := range plan.InsertValues {
		if len(plan.InsertColumns) > 0 {
			if i >= len(plan.InsertColumns) {
				continue
			}
			plan.InsertValues[i] = qAlignMutationInsertLiteral(plan.InsertColumns[i], plan.InsertValues[i], schema)
			continue
		}
		names := schema.Names()
		if i >= len(names) {
			continue
		}
		plan.InsertValues[i] = qAlignMutationInsertLiteral(names[i], plan.InsertValues[i], schema)
	}
}

func qAlignMutationInsertLiteral(name data.Symbol, lit data.Literal, schema data.Schema) data.Literal {
	kind, ok := schema.Kind(name)
	if !ok {
		return lit
	}
	aligned := qAlignLiteralForKind(lit, kind)
	if out, ok := aligned.(data.Literal); ok {
		return out
	}
	return lit
}

func qAlignAssignmentExpr(name data.Symbol, expr data.Expr, schema data.Schema) data.Expr {
	expr = qAlignDataExpr(expr, schema)
	lit, ok := expr.(data.Literal)
	if !ok {
		return expr
	}
	kind, ok := schema.Kind(name)
	if !ok {
		return expr
	}
	return qAlignLiteralForKind(lit, kind)
}

func qAlignLiteralForKind(lit data.Literal, kind data.Kind) data.Expr {
	switch kind {
	case data.KindString:
		if sym, ok := lit.Value.(data.Symbol); ok {
			return data.Literal{Value: string(sym)}
		}
	case data.KindSymbol:
		if s, ok := lit.Value.(string); ok {
			return data.Literal{Value: data.Symbol(s)}
		}
	case data.KindMonth, data.KindDate, data.KindDateTime, data.KindTimespan,
		data.KindMinute, data.KindSecond, data.KindTime, data.KindTimestamp:
		if v, err := data.NormalizeValueForKind(kind, lit.Value); err == nil {
			return data.Literal{Value: v}
		}
	}
	return lit
}

func qAlignDataExpr(expr data.Expr, schema data.Schema) data.Expr {
	switch x := expr.(type) {
	case data.Binary:
		x.Left = qAlignDataExpr(x.Left, schema)
		x.Right = qAlignDataExpr(x.Right, schema)
		if x.Op == data.OpEQ || x.Op == data.OpNE {
			x.Left, x.Right = qAlignBinaryLiteral(x.Left, x.Right, schema)
			x.Right, x.Left = qAlignBinaryLiteral(x.Right, x.Left, schema)
		}
		return x
	case data.Logical:
		x.Left = qAlignDataExpr(x.Left, schema)
		x.Right = qAlignDataExpr(x.Right, schema)
		return x
	case data.Not:
		x.Expr = qAlignDataExpr(x.Expr, schema)
		return x
	case data.In:
		x.Expr = qAlignDataExpr(x.Expr, schema)
		x.Values = qAlignInValues(x.Expr, x.Values, schema)
		return x
	case data.Within:
		x.Expr = qAlignDataExpr(x.Expr, schema)
		x.Low = qAlignPredicateValue(x.Expr, x.Low, schema)
		x.High = qAlignPredicateValue(x.Expr, x.High, schema)
		return x
	case data.BucketFloorExpr:
		x.Expr = qAlignDataExpr(x.Expr, schema)
		return x
	case data.ListAggregateExpr:
		x.Expr = qAlignDataExpr(x.Expr, schema)
		return x
	case data.VectorTransformExpr:
		x.Expr = qAlignDataExpr(x.Expr, schema)
		x.Arg = qAlignDataExpr(x.Arg, schema)
		return x
	default:
		return expr
	}
}

func qAlignInValues(expr data.Expr, values []any, schema data.Schema) []any {
	out := append([]any(nil), values...)
	for i, v := range out {
		out[i] = qAlignPredicateValue(expr, v, schema)
	}
	return out
}

func qAlignPredicateValue(expr data.Expr, value any, schema data.Schema) any {
	col, ok := expr.(data.ColumnRef)
	if !ok {
		return value
	}
	kind, ok := schema.Kind(col.Name)
	if !ok {
		return value
	}
	lit := qAlignLiteralForKind(data.Literal{Value: value}, kind)
	if aligned, ok := lit.(data.Literal); ok {
		return aligned.Value
	}
	return value
}

func qAlignBinaryLiteral(left, right data.Expr, schema data.Schema) (data.Expr, data.Expr) {
	col, ok := left.(data.ColumnRef)
	if !ok {
		return left, right
	}
	lit, ok := right.(data.Literal)
	if !ok {
		return left, right
	}
	kind, ok := schema.Kind(col.Name)
	if !ok {
		return left, right
	}
	switch kind {
	case data.KindString, data.KindSymbol,
		data.KindMonth, data.KindDate, data.KindDateTime, data.KindTimespan,
		data.KindMinute, data.KindSecond, data.KindTime, data.KindTimestamp:
		right = qAlignLiteralForKind(lit, kind)
	}
	return left, right
}

func qNormalizePlanLiterals(plan *data.QueryPlan) {
	if plan == nil {
		return
	}
	plan.Where = qNormalizeDataExpr(plan.Where)
	for i := range plan.Select {
		plan.Select[i].Expr = qNormalizeDataExpr(plan.Select[i].Expr)
	}
	for i := range plan.ByExprs {
		plan.ByExprs[i].Expr = qNormalizeDataExpr(plan.ByExprs[i].Expr)
	}
	for i := range plan.Aggregates {
		plan.Aggregates[i].Expr = qNormalizeDataExpr(plan.Aggregates[i].Expr)
		if plan.Aggregates[i].Weight != nil {
			plan.Aggregates[i].Weight = qNormalizeDataExpr(plan.Aggregates[i].Weight)
		}
	}
}

func qNormalizeMutationLiterals(plan *stdq.MutationPlan) {
	if plan == nil {
		return
	}
	plan.Where = qNormalizeDataExpr(plan.Where)
	for i := range plan.ByExprs {
		plan.ByExprs[i].Expr = qNormalizeDataExpr(plan.ByExprs[i].Expr)
	}
	for i := range plan.Assignments {
		plan.Assignments[i].Expr = qNormalizeDataExpr(plan.Assignments[i].Expr)
	}
	for i := range plan.InsertValues {
		if normalized, ok := qNormalizeDataExpr(plan.InsertValues[i]).(data.Literal); ok {
			plan.InsertValues[i] = normalized
		}
	}
}

func qNormalizeDataExpr(expr data.Expr) data.Expr {
	switch x := expr.(type) {
	case nil:
		return nil
	case data.ColumnRef:
		switch strings.ToLower(string(x.Name)) {
		case "true":
			return data.Literal{Value: true}
		case "false":
			return data.Literal{Value: false}
		case "null":
			return data.Literal{Value: nil}
		default:
			return x
		}
	case data.Binary:
		x.Left = qNormalizeDataExpr(x.Left)
		x.Right = qNormalizeDataExpr(x.Right)
		return x
	case data.ListAggregateExpr:
		x.Expr = qNormalizeDataExpr(x.Expr)
		return x
	default:
		return x
	}
}

type qSQLOptions struct {
	order []data.OrderSpec
	limit int
}

func (o qSQLOptions) apply(plan *data.QueryPlan) {
	if len(o.order) > 0 {
		plan.OrderBy = o.order
	}
	if o.limit >= 0 {
		plan.LimitN = o.limit
	}
}

func qSQLSourceAndOptions(src string) (string, qSQLOptions, error) {
	opts := qSQLOptions{limit: -1}
	base := strings.TrimSpace(src)
	if idx := qLastKeywordIndex(base, "limit"); idx >= 0 {
		n, err := strconv.Atoi(strings.TrimSpace(base[idx+len("limit"):]))
		if err != nil || n < 0 {
			return "", opts, fmt.Errorf("limit must be a non-negative integer")
		}
		opts.limit = n
		base = strings.TrimSpace(base[:idx])
	}
	if idx := qLastKeywordIndex(base, "order by"); idx >= 0 {
		specs, err := qSQLOrderSpecs(strings.TrimSpace(base[idx+len("order by"):]))
		if err != nil {
			return "", opts, err
		}
		opts.order = specs
		base = strings.TrimSpace(base[:idx])
	}
	return base, opts, nil
}

func qLastKeywordIndex(src, keyword string) int {
	fields := strings.Fields(src)
	if len(fields) == 0 {
		return -1
	}
	lowerKeyword := strings.Fields(strings.ToLower(keyword))
	positions := make([]int, 0, len(fields))
	pos := 0
	for _, field := range fields {
		idx := strings.Index(src[pos:], field)
		if idx < 0 {
			return -1
		}
		pos += idx
		positions = append(positions, pos)
		pos += len(field)
	}
	for i := len(fields) - len(lowerKeyword); i >= 0; i-- {
		match := true
		for j, want := range lowerKeyword {
			if strings.ToLower(fields[i+j]) != want {
				match = false
				break
			}
		}
		if match {
			return positions[i]
		}
	}
	return -1
}

func qSQLOrderSpecs(src string) ([]data.OrderSpec, error) {
	if src == "" {
		return nil, fmt.Errorf("order by requires at least one column")
	}
	parts := strings.Split(src, ",")
	specs := make([]data.OrderSpec, 0, len(parts))
	for _, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 || len(fields) > 2 {
			return nil, fmt.Errorf("invalid order by specification %q", part)
		}
		spec := data.OrderSpec{Column: data.Symbol(fields[0])}
		if len(fields) == 2 {
			switch strings.ToLower(fields[1]) {
			case "asc":
			case "desc":
				spec.Desc = true
			default:
				return nil, fmt.Errorf("invalid order direction %q", fields[1])
			}
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func qDataFrameFromValue(v Value, sourceName string) (data.Frame, error) {
	source, err := qResolveSQLSourceValue(v, sourceName)
	if err != nil {
		return data.Frame{}, err
	}
	if source != v {
		return qDataFrameFromValue(source, "")
	}
	if keyed, err := qKeyedFrameFromValue(v); err == nil {
		return keyed.Frame(), nil
	}
	if v.IsSoA() {
		return qDataFrameFromSoA(v.SoA())
	}
	if !v.IsTable() {
		return data.Frame{}, fmt.Errorf("argument 1 must be a frame table or soa")
	}
	tbl := v.Table()
	if qNativeFrameRuntimeKindMatches(tbl, NativePayloadKeyedFrame) {
		keyed, ok, err := qNativeKeyedFramePayload(tbl)
		if err != nil {
			return data.Frame{}, err
		}
		if !ok {
			return data.Frame{}, fmt.Errorf("native keyed frame payload is invalid")
		}
		return keyed.Frame(), nil
	}
	if native, ok, err := qNativeDataFramePayload(tbl); err != nil {
		return data.Frame{}, err
	} else if ok {
		return native, nil
	}
	if cols := tbl.RawGetString("columns"); cols.IsTable() {
		if names := tbl.RawGetString("column_names"); names.IsTable() {
			return qDataFrameFromColumns(cols.Table(), names.Table(), qFrameColumnKinds(tbl))
		}
	}
	if soa := tbl.RawGetString("soa"); soa.IsSoA() {
		return qDataFrameFromSoA(soa.SoA())
	}
	return qDataFrameFromRowTable(tbl)
}

func qNativeDataFramePayload(tbl *Table) (data.Frame, bool, error) {
	if tbl == nil {
		return data.Frame{}, false, nil
	}
	if native, ok, err := qTypedNativeDataFramePayload(tbl); ok || err != nil {
		return native, ok, err
	}
	if blocks, err := qNativePayloadKindBlocksLegacyFrameFallback(tbl, NativePayloadDataFrame, "native data frame payload is invalid"); blocks || err != nil {
		return data.Frame{}, false, err
	}
	if native, ok := tbl.NativePayload().(data.Frame); ok {
		return native, true, nil
	}
	return data.Frame{}, false, nil
}

func qSymbolsFromArgs(name string, args []Value) ([]data.Symbol, error) {
	var keys []data.Symbol
	for _, arg := range args {
		switch {
		case arg.IsString():
			keys = append(keys, data.Symbol(arg.Str()))
		case arg.IsTable():
			symbols, ok := qSymbolVectorSymbols(arg)
			if !ok {
				return nil, fmt.Errorf("%s: keys must be strings or an array table of strings", name)
			}
			keys = append(keys, symbols...)
		default:
			return nil, fmt.Errorf("%s: keys must be strings or an array table of strings", name)
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%s: at least one key is required", name)
	}
	return keys, nil
}

func qKeys(args []Value, keyFrame bool) ([]Value, error) {
	name := "q.keys"
	if keyFrame {
		name = "q.key"
	}
	if len(args) < 1 {
		return nil, fmt.Errorf("%s: argument 1 required", name)
	}
	if keyed, err := qKeyedFrameFromValue(args[0]); err == nil {
		if keyFrame {
			frame, err := keyed.KeyFrame()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			rows, err := qRowsFromDataFrame(frame)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			return []Value{TableValue(rows)}, nil
		}
		return []Value{qDataSymbolListValue(keyed.Keys())}, nil
	}
	if _, err := qDataFrameFromValue(args[0], ""); err == nil {
		return []Value{qDataSymbolListValue(nil)}, nil
	}
	if args[0].IsTable() {
		if keys, ok := qDictionaryKeyOrder(args[0].Table()); ok {
			return []Value{qDataSymbolListValue(keys)}, nil
		}
		keys := make([]data.Symbol, 0)
		args[0].Table().ForEachPlainRaw(func(key, _ Value) bool {
			if key.IsString() {
				keys = append(keys, data.Symbol(key.Str()))
			}
			return true
		})
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		return []Value{qDataSymbolListValue(keys)}, nil
	}
	return nil, fmt.Errorf("%s: expected dictionary, table, or keyed frame", name)
}

func qKeyedFrameToValue(keyed data.KeyedFrame) Value {
	t := NewTable()
	t.RawSetString(qKeyedFrameMarker, BoolValue(true))
	t.RawSetString("keys", qDataSymbolListValue(keyed.Keys()))
	if rows, err := qRowsFromDataFrame(keyed.Frame()); err == nil {
		t.RawSetString("frame", TableValue(rows))
	}
	setQKeyedFrameNativePayload(t, keyed)
	return TableValue(t)
}

func setQKeyedFrameNativePayload(table *Table, keyed data.KeyedFrame) {
	if table == nil {
		return
	}
	frame := keyed.Frame()
	table.SetNativePayloadWithInfo(keyed, NativePayloadInfo{
		Kind:       NativePayloadKeyedFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
}

func qKeyedFrameFromValue(v Value) (data.KeyedFrame, error) {
	if !v.IsTable() {
		return data.KeyedFrame{}, fmt.Errorf("argument 1 must be a keyed frame")
	}
	tbl := v.Table()
	if !qIsKeyedFrameTable(tbl) {
		return data.KeyedFrame{}, fmt.Errorf("argument 1 must be a keyed frame")
	}
	if native, ok, err := qNativeKeyedFramePayload(tbl); err != nil {
		return data.KeyedFrame{}, err
	} else if ok {
		return native, nil
	}
	frame, err := qDataFrameFromValue(tbl.RawGetString("frame"), "")
	if err != nil {
		return data.KeyedFrame{}, err
	}
	keys, err := qSymbolsFromArgs("q.lookup", []Value{tbl.RawGetString("keys")})
	if err != nil {
		return data.KeyedFrame{}, err
	}
	return data.KeyBy(frame, keys...)
}

func qNativeKeyedFramePayload(tbl *Table) (data.KeyedFrame, bool, error) {
	if tbl == nil {
		return data.KeyedFrame{}, false, nil
	}
	if native, ok, err := qTypedNativeKeyedFramePayload(tbl); ok || err != nil {
		return native, ok, err
	}
	if blocks, err := qNativePayloadKindBlocksLegacyFrameFallback(tbl, NativePayloadKeyedFrame, "native keyed frame payload is invalid"); blocks || err != nil {
		return data.KeyedFrame{}, false, err
	}
	if native, ok := tbl.NativePayload().(data.KeyedFrame); ok {
		return native, true, nil
	}
	return data.KeyedFrame{}, false, nil
}

func qNativePayloadKindBlocksLegacyFrameFallback(tbl *Table, want NativePayloadKind, invalid string) (bool, error) {
	if tbl == nil {
		return false, nil
	}
	kind, ok := tbl.NativePayloadKind()
	if !ok {
		return false, nil
	}
	if kind == want {
		return true, fmt.Errorf("%s", invalid)
	}
	return true, nil
}

func qLookupKeyValues(keyed data.KeyedFrame, args []Value) ([]any, error) {
	if len(args) == 1 && args[0].IsTable() && !qIsSymbolVector(args[0]) {
		tbl := args[0].Table()
		if values, ok := qLookupPositionalKeyValues(keyed, tbl); ok {
			return values, nil
		}
		values := make([]any, 0, len(keyed.Keys()))
		for _, key := range keyed.Keys() {
			v := tbl.RawGetString(string(key))
			if v.IsNil() {
				return nil, fmt.Errorf("key %q is missing", key)
			}
			values = append(values, qValueToAny(v))
		}
		return values, nil
	}
	values := make([]any, 0, len(args))
	for _, arg := range args {
		values = append(values, qValueToAny(arg))
	}
	return values, nil
}

func qLookupPositionalKeyValues(keyed data.KeyedFrame, tbl *Table) ([]any, bool) {
	keys := keyed.Keys()
	if len(keys) == 0 || tbl.Length() != len(keys) {
		return nil, false
	}
	values := make([]any, len(keys))
	for i := range keys {
		v := tbl.RawGetInt(int64(i + 1))
		if v.IsNil() {
			return nil, false
		}
		values[i] = qValueToAny(v)
	}
	return values, true
}

func qKeyedMutationArgs(name string, args []Value) (data.KeyedFrame, data.Frame, []data.Symbol, error) {
	if len(args) < 2 {
		return data.KeyedFrame{}, data.Frame{}, nil, fmt.Errorf("%s: expected keyed frame and delta frame", name)
	}
	keyed, err := qKeyedFrameFromValue(args[0])
	if err != nil {
		return data.KeyedFrame{}, data.Frame{}, nil, fmt.Errorf("%s: %w", name, err)
	}
	delta, err := qDataFrameFromValue(args[1], "")
	if err != nil {
		return data.KeyedFrame{}, data.Frame{}, nil, fmt.Errorf("%s: delta: %w", name, err)
	}
	valueColumns, err := qSymbolsFromArgs(name, args[2:])
	if err != nil && len(args) > 2 {
		return data.KeyedFrame{}, data.Frame{}, nil, err
	}
	return keyed, delta, valueColumns, nil
}

func qFrameLikeValue(v Value) (data.Frame, error) {
	if keyed, err := qKeyedFrameFromValue(v); err == nil {
		return keyed.Frame(), nil
	}
	return qDataFrameFromValue(v, "")
}

func qWireValueFromValue(v Value) (any, error) {
	if keyed, err := qKeyedFrameFromValue(v); err == nil {
		return keyed, nil
	}
	if dict, ok, err := qDictFromValue(v); ok || err != nil {
		return dict, err
	}
	if symbols, ok := qSymbolVectorSymbols(v); ok {
		names := make([]string, len(symbols))
		for i, sym := range symbols {
			names[i] = string(sym)
		}
		return data.NewSymbols(names), nil
	}
	if v.IsDenseArray() {
		return qDenseArrayToDataArray(v.DenseArray())
	}
	if frame, err := qDataFrameFromValue(v, ""); err == nil {
		return frame, nil
	}
	if scalar, ok := qWireScalarFromValue(v); ok {
		return scalar, nil
	}
	return nil, fmt.Errorf("unsupported value type %v", v.Type())
}

func qDictFromValue(v Value) (stdq.Dict, bool, error) {
	if !v.IsTable() {
		return stdq.Dict{}, false, nil
	}
	tbl := v.Table()
	if qLooksLikeFrame(tbl) || qIsKeyedFrameTable(tbl) || qIsSymbolVector(v) {
		return stdq.Dict{}, false, nil
	}
	keys, ok := qDictionaryKeyOrder(tbl)
	if !ok {
		return stdq.Dict{}, false, nil
	}
	out := stdq.Dict{
		Keys:   make([]any, len(keys)),
		Values: make([]any, len(keys)),
	}
	for i, key := range keys {
		out.Keys[i] = key
		value, err := qWireValueFromValue(tbl.RawGetString(string(key)))
		if err != nil {
			return stdq.Dict{}, true, err
		}
		out.Values[i] = value
	}
	return out, true, nil
}

func qDenseArrayToDataArray(array *DenseArray) (data.Array, error) {
	switch array.DType() {
	case DenseArrayI64:
		xs := make([]int64, array.Len())
		for i := range xs {
			v, err := array.At(i)
			if err != nil || !v.IsInt() {
				return nil, fmt.Errorf("i64 array row %d is not int", i)
			}
			xs[i] = v.Int()
		}
		return data.NewI64(xs), nil
	case DenseArrayF64:
		xs := make([]float64, array.Len())
		for i := range xs {
			v, err := array.At(i)
			if err != nil || !v.IsFloat() {
				return nil, fmt.Errorf("f64 array row %d is not float", i)
			}
			xs[i] = v.Float()
		}
		return data.NewF64(xs), nil
	case DenseArrayBool:
		xs := make([]bool, array.Len())
		for i := range xs {
			v, err := array.At(i)
			if err != nil || !v.IsBool() {
				return nil, fmt.Errorf("bool array row %d is not bool", i)
			}
			xs[i] = v.Bool()
		}
		return data.NewBool(xs), nil
	default:
		return nil, fmt.Errorf("unsupported dense array dtype %s", array.DType())
	}
}

func qMetaFrame(frame data.Frame) data.Frame {
	names := frame.Schema().Names()
	types := make([]string, len(names))
	for i, name := range names {
		kind, _ := frame.Schema().Kind(name)
		types[i] = string(kind)
	}
	meta, err := data.NewFrame(
		data.Column{Name: "c", Data: data.NewSymbols(symbolNames(names))},
		data.Column{Name: "t", Data: data.NewString(types)},
	)
	if err != nil {
		panic(err)
	}
	return meta
}

func symbolNames(symbols []data.Symbol) []string {
	out := make([]string, len(symbols))
	for i, sym := range symbols {
		out[i] = string(sym)
	}
	return out
}

func qLooksLikeFrame(tbl *Table) bool {
	if tbl == nil {
		return false
	}
	if kind, ok := qNativeFrameRuntimeKind(tbl); ok {
		return kind == NativePayloadDataFrame
	}
	return isDataFrameTable(tbl) || qLooksLikeScriptDataFrameFacade(tbl)
}

func qLooksLikeScriptDataFrameFacade(tbl *Table) bool {
	if tbl == nil {
		return false
	}
	if kind := tbl.RawGetString("kind"); kind.IsString() && kind.Str() == "data_frame" {
		return true
	}
	if typ := tbl.RawGetString("type"); typ.IsString() && typ.Str() == "data_frame" {
		return true
	}
	return tbl.RawGetString("columns").IsTable() && tbl.RawGetString("column_names").IsTable()
}

func qIsKeyedFrameTable(tbl *Table) bool {
	if tbl == nil {
		return false
	}
	if kind, ok := qNativeFrameRuntimeKind(tbl); ok {
		return kind == NativePayloadKeyedFrame
	}
	return tbl.RawGetString(qKeyedFrameMarker).Truthy()
}

func qDataFrameFromSoA(s *SoA) (data.Frame, error) {
	if s == nil {
		return data.Frame{}, fmt.Errorf("soa is nil")
	}
	cols := make([]data.Column, 0, len(s.ColumnNames()))
	for _, name := range s.ColumnNames() {
		col, ok := s.Column(name)
		if !ok {
			return data.Frame{}, fmt.Errorf("soa column %q not found", name)
		}
		array, err := qDataArrayFromDense(col)
		if err != nil {
			return data.Frame{}, fmt.Errorf("column %q: %w", name, err)
		}
		cols = append(cols, data.Column{Name: data.Symbol(name), Data: array})
	}
	return data.NewFrame(cols...)
}

func qDataFrameFromColumns(columns, columnNames *Table, kinds map[string]data.Kind) (data.Frame, error) {
	cols := make([]data.Column, 0, columnNames.Length())
	for i := 1; i <= columnNames.Length(); i++ {
		nameValue := columnNames.RawGetInt(int64(i))
		if !nameValue.IsString() {
			return data.Frame{}, fmt.Errorf("frame column_names[%d] must be a string", i)
		}
		name := nameValue.Str()
		col, err := qDataColumnFromVector(data.Symbol(name), columns.RawGetString(name), kinds[name])
		if err != nil {
			return data.Frame{}, fmt.Errorf("column %q: %w", name, err)
		}
		cols = append(cols, col)
	}
	return data.NewFrame(cols...)
}

func qFrameColumnKinds(frame *Table) map[string]data.Kind {
	if frame == nil {
		return nil
	}
	out := map[string]data.Kind{}
	addKinds := func(v Value) {
		if !v.IsTable() {
			return
		}
		v.Table().ForEachPlainRaw(func(key, val Value) bool {
			if key.IsString() && val.IsString() {
				if kind, ok := qDataKind(val.Str()); ok {
					name := key.Str()
					if existing, exists := out[name]; exists {
						if kind == data.KindAny || kind == data.KindNull {
							return true
						}
						if existing != data.KindAny && existing != data.KindNull {
							return true
						}
					}
					out[name] = kind
				}
			}
			return true
		})
	}
	addKinds(frame.RawGetString("column_kinds"))
	addKinds(frame.RawGetString("kinds"))
	if schema := frame.RawGetString("schema"); schema.IsTable() {
		addKinds(schema.Table().RawGetString("kinds"))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func qDataKind(kind string) (data.Kind, bool) {
	switch strings.ToLower(kind) {
	case "i64", "int", "integer":
		return data.KindI64, true
	case "f64", "float", "real", "number":
		return data.KindF64, true
	case "bool", "boolean":
		return data.KindBool, true
	case "string", "text":
		return data.KindString, true
	case "symbol":
		return data.KindSymbol, true
	case "month":
		return data.KindMonth, true
	case "date":
		return data.KindDate, true
	case "datetime":
		return data.KindDateTime, true
	case "timespan":
		return data.KindTimespan, true
	case "minute":
		return data.KindMinute, true
	case "second":
		return data.KindSecond, true
	case "time":
		return data.KindTime, true
	case "timestamp":
		return data.KindTimestamp, true
	case "null":
		return data.KindNull, true
	case "any":
		return data.KindAny, true
	default:
		return "", false
	}
}

func qDataColumnFromVector(name data.Symbol, v Value, kind data.Kind) (data.Column, error) {
	if native, ok := dataNativeArrayFromValue(dataColumnWrappedValues(v)); ok {
		return data.Column{Name: name, Data: native}, nil
	}
	values, err := qAnyValuesFromVector(v)
	if err != nil {
		return data.Column{}, err
	}
	if kind == "" || kind == data.KindAny {
		return data.NewColumn(name, values), nil
	}
	typedValues := qCoerceDataValues(values, kind)
	if col, err := data.NewColumnWithKind(name, kind, typedValues); err == nil {
		return col, nil
	}
	array, ok := qTypedDataArray(typedValues, kind)
	if !ok {
		return data.NewColumn(name, values), nil
	}
	return data.Column{Name: name, Data: array}, nil
}

func qTypedDataArray(values []any, kind data.Kind) (data.Array, bool) {
	values = qCoerceDataValues(values, kind)
	for _, v := range values {
		if data.IsNull(v) {
			col, err := data.NewColumnWithKind("_", kind, values)
			if err != nil {
				return nil, false
			}
			return col.Data, true
		}
	}
	switch kind {
	case data.KindBool:
		xs := make([]bool, len(values))
		for i, v := range values {
			b, ok := v.(bool)
			if !ok {
				return nil, false
			}
			xs[i] = b
		}
		return data.NewBool(xs), true
	case data.KindI64:
		xs := make([]int64, len(values))
		for i, v := range values {
			switch n := v.(type) {
			case int:
				xs[i] = int64(n)
			case int64:
				xs[i] = n
			default:
				return nil, false
			}
		}
		return data.NewI64(xs), true
	case data.KindF64:
		xs := make([]float64, len(values))
		for i, v := range values {
			n, ok := qNumericAny(v)
			if !ok {
				return nil, false
			}
			xs[i] = n
		}
		return data.NewF64(xs), true
	case data.KindString:
		xs := make([]string, len(values))
		for i, v := range values {
			s, ok := v.(string)
			if !ok {
				return nil, false
			}
			xs[i] = s
		}
		return data.NewString(xs), true
	case data.KindSymbol:
		xs := make([]string, len(values))
		for i, v := range values {
			switch s := v.(type) {
			case data.Symbol:
				xs[i] = string(s)
			case string:
				xs[i] = s
			default:
				return nil, false
			}
		}
		return data.NewSymbols(xs), true
	case data.KindMonth:
		xs := make([]data.Month, len(values))
		for i, v := range values {
			n, ok := v.(data.Month)
			if !ok {
				return nil, false
			}
			xs[i] = n
		}
		return data.NewMonth(xs), true
	case data.KindDate:
		xs := make([]data.Date, len(values))
		for i, v := range values {
			d, ok := v.(data.Date)
			if !ok {
				return nil, false
			}
			xs[i] = d
		}
		return data.NewDate(xs), true
	case data.KindDateTime:
		xs := make([]data.DateTime, len(values))
		for i, v := range values {
			n, ok := v.(data.DateTime)
			if !ok {
				return nil, false
			}
			xs[i] = n
		}
		return data.NewDateTime(xs), true
	case data.KindTimespan:
		xs := make([]data.Timespan, len(values))
		for i, v := range values {
			n, ok := v.(data.Timespan)
			if !ok {
				return nil, false
			}
			xs[i] = n
		}
		return data.NewTimespan(xs), true
	case data.KindMinute:
		xs := make([]data.Minute, len(values))
		for i, v := range values {
			n, ok := v.(data.Minute)
			if !ok {
				return nil, false
			}
			xs[i] = n
		}
		return data.NewMinute(xs), true
	case data.KindSecond:
		xs := make([]data.Second, len(values))
		for i, v := range values {
			n, ok := v.(data.Second)
			if !ok {
				return nil, false
			}
			xs[i] = n
		}
		return data.NewSecond(xs), true
	case data.KindTime:
		xs := make([]data.Time, len(values))
		for i, v := range values {
			t, ok := v.(data.Time)
			if !ok {
				return nil, false
			}
			xs[i] = t
		}
		return data.NewTime(xs), true
	case data.KindTimestamp:
		xs := make([]data.Timestamp, len(values))
		for i, v := range values {
			ts, ok := v.(data.Timestamp)
			if !ok {
				return nil, false
			}
			xs[i] = ts
		}
		return data.NewTimestamp(xs), true
	default:
		return nil, false
	}
}

func qCoerceDataValues(values []any, kind data.Kind) []any {
	out := append([]any(nil), values...)
	for i, v := range out {
		if data.IsNull(v) {
			continue
		}
		switch kind {
		case data.KindSymbol:
			if s, ok := v.(string); ok {
				out[i] = data.Symbol(s)
			}
		case data.KindMonth, data.KindDate, data.KindDateTime, data.KindTimespan, data.KindMinute, data.KindSecond, data.KindTime, data.KindTimestamp:
			if parsed, ok := qParseTemporalAny(kind, v); ok {
				out[i] = parsed
			} else if parsed, err := data.NormalizeValueForKind(kind, v); err == nil {
				out[i] = parsed
			}
		}
	}
	return out
}

func qParseTemporalAny(kind data.Kind, v any) (any, bool) {
	switch kind {
	case data.KindMonth:
		switch x := v.(type) {
		case data.Month:
			return x, true
		case int64:
			return data.MonthFromMonths(x), true
		case string:
			if parsed, err := stdq.ParseTemporal("month", x); err == nil {
				return parsed, true
			}
		}
	case data.KindDate:
		switch x := v.(type) {
		case data.Date:
			return x, true
		case int64:
			return data.DateFromDays(x), true
		case string:
			for _, layout := range []string{"2006-01-02", "2006.01.02"} {
				if tm, err := time.Parse(layout, x); err == nil {
					return data.DateFromDays(tm.Unix() / 86400), true
				}
			}
		}
	case data.KindDateTime:
		switch x := v.(type) {
		case data.DateTime:
			return x, true
		case int64:
			return data.DateTimeFromUnixNanos(x), true
		case string:
			if parsed, err := stdq.ParseTemporal("datetime", x); err == nil {
				return parsed, true
			}
		}
	case data.KindTimespan:
		switch x := v.(type) {
		case data.Timespan:
			return x, true
		case int64:
			return data.TimespanFromNanos(x), true
		case string:
			if parsed, err := stdq.ParseTemporal("timespan", x); err == nil {
				return parsed, true
			}
		}
	case data.KindMinute:
		switch x := v.(type) {
		case data.Minute:
			return x, true
		case int64:
			return data.MinuteFromMinutes(x), true
		case string:
			if parsed, err := stdq.ParseTemporal("minute", x); err == nil {
				return parsed, true
			}
		}
	case data.KindSecond:
		switch x := v.(type) {
		case data.Second:
			return x, true
		case int64:
			return data.SecondFromSeconds(x), true
		case string:
			if parsed, err := stdq.ParseTemporal("second", x); err == nil {
				return parsed, true
			}
		}
	case data.KindTime:
		switch x := v.(type) {
		case data.Time:
			return x, true
		case int64:
			return data.TimeFromNanos(x), true
		case string:
			for _, layout := range []string{"15:04:05.999999999", "15:04:05.999999", "15:04:05.999", "15:04:05"} {
				if tm, err := time.Parse(layout, x); err == nil {
					nanos := int64(tm.Hour())*3600*1_000_000_000 + int64(tm.Minute())*60*1_000_000_000 + int64(tm.Second())*1_000_000_000 + int64(tm.Nanosecond())
					return data.TimeFromNanos(nanos), true
				}
			}
		}
	case data.KindTimestamp:
		switch x := v.(type) {
		case data.Timestamp:
			return x, true
		case int64:
			return data.TimestampFromUnixNanos(x), true
		case string:
			if tm, err := time.Parse(time.RFC3339Nano, x); err == nil {
				return data.TimestampFromUnixNanos(tm.UnixNano()), true
			}
			if tm, err := time.Parse("2006-01-02T15:04:05", x); err == nil {
				return data.TimestampFromUnixNanos(tm.UnixNano()), true
			}
			if tm, err := time.Parse("2006.01.02D15:04:05.999999999", x); err == nil {
				return data.TimestampFromUnixNanos(tm.UnixNano()), true
			}
			if tm, err := time.Parse("2006.01.02D15:04:05", x); err == nil {
				return data.TimestampFromUnixNanos(tm.UnixNano()), true
			}
		}
	}
	return nil, false
}

func qNumericAny(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func qDataFrameFromRowTable(rows *Table) (data.Frame, error) {
	if rows == nil || rows.Length() == 0 {
		return data.Frame{}, fmt.Errorf("frame requires at least one row")
	}
	first := rows.RawGetInt(1)
	if !first.IsTable() {
		return data.Frame{}, fmt.Errorf("frame rows must be tables")
	}
	names, err := qRowColumnNames(first.Table())
	if err != nil {
		return data.Frame{}, err
	}
	values := make(map[string][]any, len(names))
	for _, name := range names {
		values[name] = make([]any, 0, rows.Length())
	}
	for i := 1; i <= rows.Length(); i++ {
		rowValue := rows.RawGetInt(int64(i))
		if !rowValue.IsTable() {
			return data.Frame{}, fmt.Errorf("frame row %d must be a table", i)
		}
		row := rowValue.Table()
		for _, name := range names {
			values[name] = append(values[name], qValueToAny(row.RawGetString(name)))
		}
	}
	cols := make([]data.Column, 0, len(names))
	kinds := qFrameColumnKinds(rows)
	for _, name := range names {
		kind := data.Kind("")
		if kinds != nil {
			kind = kinds[name]
		}
		if kind != "" && kind != data.KindAny {
			if array, ok := qTypedDataArray(values[name], kind); ok {
				cols = append(cols, data.Column{Name: data.Symbol(name), Data: array})
				continue
			}
		}
		cols = append(cols, data.NewColumn(data.Symbol(name), values[name]))
	}
	return data.NewFrame(cols...)
}

func qAttachRowsNativeFramePayload(rows *Table) {
	if rows == nil || rows.Length() == 0 {
		return
	}
	frame, err := qDataFrameFromRowTable(rows)
	if err != nil {
		return
	}
	setDataFrameNativePayload(rows, frame)
}

func qAttachRowsNativeSoAPayload(rows *Table, soa *SoA) {
	if rows == nil || soa == nil || rows.Length() != soa.Len() {
		return
	}
	rows.SetNativePayloadWithInfo(soa, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    len(soa.ColumnNames()),
		SchemaHash: qQueryNativeSoASchemaHash(soa),
	})
}

func qSimpleSelectRowsNativeSoA(s *SoA, mask *DenseArray, selects []qSelect) (*SoA, string, string) {
	if s == nil || mask == nil || len(selects) == 0 {
		return nil, qQueryKernelReasonUnsupported, "query native kernel requires source, mask, and select items"
	}
	filtered, err := s.Filter(mask)
	if err != nil {
		return nil, qKernelReasonUnsupported, "query native kernel filter failed: " + err.Error()
	}
	cols := make(map[string]*DenseArray, len(selects))
	for _, sel := range selects {
		col, ok := qEvalNativeExpr(filtered, sel.Expr)
		if !ok {
			return nil, qQueryKernelReasonSelect, fmt.Sprintf("select expression %q is not supported by q query native kernel", sel.Name)
		}
		cols[sel.Name] = col
	}
	out, err := NewSoA(cols)
	if err != nil {
		return nil, qQueryKernelReasonSelect, "query native kernel projection failed: " + err.Error()
	}
	return out, "", ""
}

func qEvalNativeExpr(s *SoA, expr Value) (*DenseArray, bool) {
	if s == nil {
		return nil, false
	}
	if expr.IsString() {
		col, ok := s.Column(expr.Str())
		return col, ok
	}
	if expr.IsInt() {
		out, err := NewDenseArrayOfLen(DenseArrayI64, s.Len())
		if err != nil {
			return nil, false
		}
		if err := out.Fill(expr); err != nil {
			return nil, false
		}
		return out, true
	}
	if expr.IsFloat() {
		out, err := NewDenseArrayOfLen(DenseArrayF64, s.Len())
		if err != nil {
			return nil, false
		}
		if err := out.Fill(expr); err != nil {
			return nil, false
		}
		return out, true
	}
	if expr.IsBool() {
		out, err := NewDenseArrayOfLen(DenseArrayBool, s.Len())
		if err != nil {
			return nil, false
		}
		if err := out.Fill(expr); err != nil {
			return nil, false
		}
		return out, true
	}
	if !expr.IsTable() {
		return nil, false
	}
	tbl := expr.Table()
	opValue := tbl.RawGetString("op")
	if opValue.IsNil() {
		opValue = tbl.RawGetInt(1)
	}
	if !opValue.IsString() {
		return nil, false
	}
	op, ok := qNativeDenseArrayBinaryOp(opValue.Str())
	if !ok {
		return nil, false
	}
	left := tbl.RawGetString("left")
	if left.IsNil() {
		left = tbl.RawGetInt(2)
	}
	right := tbl.RawGetString("right")
	if right.IsNil() {
		right = tbl.RawGetInt(3)
	}
	leftValue, ok := qNativeExprOperand(s, left)
	if !ok {
		return nil, false
	}
	rightValue, ok := qNativeExprOperand(s, right)
	if !ok {
		return nil, false
	}
	out, err := DenseArrayElementwise(op, leftValue, rightValue)
	if err != nil || !out.IsDenseArray() {
		return nil, false
	}
	return out.DenseArray(), true
}

func qNativeExprOperand(s *SoA, expr Value) (Value, bool) {
	if expr.IsString() {
		col, ok := s.Column(expr.Str())
		if !ok {
			return NilValue(), false
		}
		return DenseArrayValue(col), true
	}
	if expr.IsNumber() {
		return expr, true
	}
	if expr.IsBool() {
		return expr, true
	}
	if expr.IsTable() {
		col, ok := qEvalNativeExpr(s, expr)
		if !ok {
			return NilValue(), false
		}
		return DenseArrayValue(col), true
	}
	return NilValue(), false
}

func qNativeDenseArrayBinaryOp(op string) (DenseArrayBinaryOp, bool) {
	switch op {
	case "+":
		return DenseArrayAdd, true
	case "-":
		return DenseArraySub, true
	case "*":
		return DenseArrayMul, true
	case "/":
		return DenseArrayDiv, true
	case "==":
		return DenseArrayEQ, true
	case "!=":
		return DenseArrayNE, true
	case "<":
		return DenseArrayLT, true
	case "<=":
		return DenseArrayLE, true
	case ">":
		return DenseArrayGT, true
	case ">=":
		return DenseArrayGE, true
	default:
		return DenseArrayAdd, false
	}
}

func qQueryNativeRowsForResult(spec *Table, nativeRows *SoA) (*SoA, bool, string, string) {
	if nativeRows == nil {
		return nil, false, qQueryKernelReasonUnsupported, "query native kernel produced no native rows"
	}
	if spec == nil {
		return nativeRows, true, "", ""
	}
	order, err := qOrderSpecs(spec.RawGetString("order_by"))
	if err != nil {
		return nil, false, qQueryKernelReasonOrder, err.Error()
	}
	limit, err := qLimit(spec.RawGetString("limit"))
	if err != nil {
		return nil, false, qQueryKernelReasonOrder, err.Error()
	}
	if len(order) > 0 {
		return qOrderedNativeRowsForResult(nativeRows, order, limit)
	}
	if limit < 0 || limit >= nativeRows.Len() {
		return nativeRows, true, "", ""
	}
	sliced, handled, err := qNativeRowsFrameCarrier(nativeRows).NativeFrameSlice(0, limit)
	if err != nil {
		return nil, false, qQueryKernelReasonOrder, "query native kernel limit failed: " + err.Error()
	}
	if !handled {
		return nil, false, qQueryKernelReasonOrder, "query native kernel limit requires native frame payload"
	}
	out, ok := qNativeRowsFromFrameValue(sliced)
	if !ok {
		return nil, false, qQueryKernelReasonOrder, "query native kernel limit produced unsupported native rows"
	}
	return out, true, "", ""
}

func qOrderedNativeRowsForResult(nativeRows *SoA, order []qOrderSpec, limit int) (*SoA, bool, string, string) {
	if nativeRows == nil || len(order) == 0 {
		if nativeRows == nil {
			return nil, false, qQueryKernelReasonUnsupported, "query native kernel produced no native rows"
		}
		return nativeRows, true, "", ""
	}
	carrier := qNativeRowsFrameCarrier(nativeRows)
	indexValue, handled, err := carrier.NativeFrameOrderIndexes(qOrderColumns(order), qOrderDescFlags(order), limit)
	if err != nil {
		return nil, false, qQueryKernelReasonOrder, "query native kernel order failed: " + err.Error()
	}
	if !handled || !indexValue.IsDenseArray() {
		return nil, false, qQueryKernelReasonOrder, "query native kernel order requires native frame indexes"
	}
	indexes := indexValue.DenseArray()
	gathered, handled, err := carrier.NativeFrameGather(indexes)
	if err != nil {
		return nil, false, qQueryKernelReasonOrder, "query native kernel ordered gather failed: " + err.Error()
	}
	if !handled {
		return nil, false, qQueryKernelReasonOrder, "query native kernel ordered gather requires native frame payload"
	}
	out, ok := qNativeRowsFromFrameValue(gathered)
	if !ok {
		return nil, false, qQueryKernelReasonOrder, "query native kernel ordered gather produced unsupported native rows"
	}
	return out, true, "", ""
}

func qOrderColumns(order []qOrderSpec) []string {
	out := make([]string, len(order))
	for i, ord := range order {
		out[i] = ord.Column
	}
	return out
}

func qOrderDescFlags(order []qOrderSpec) []bool {
	out := make([]bool, len(order))
	for i, ord := range order {
		out[i] = ord.Desc
	}
	return out
}

func qNativeRowsFrameCarrier(nativeRows *SoA) Value {
	carrier := NewTable()
	carrier.SetNativePayloadWithInfo(nativeRows, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       nativeRows.Len(),
		Columns:    len(nativeRows.ColumnNames()),
		SchemaHash: qQueryNativeSoASchemaHash(nativeRows),
	})
	return TableValue(carrier)
}

func qNativeRowsFromFrameValue(value Value) (*SoA, bool) {
	if !value.IsTable() {
		return nil, false
	}
	out, _, ok := value.Table().NativeFramePayload()
	if !ok {
		return nil, false
	}
	soa, ok := out.(*SoA)
	if !ok {
		return nil, false
	}
	return soa, true
}

func qQueryNativeSoASchemaHash(soa *SoA) string {
	if soa == nil {
		return "q.query.kernel:"
	}
	names := soa.ColumnNames()
	parts := make([]string, 0, len(names))
	for _, name := range names {
		col, ok := soa.Column(name)
		if !ok {
			continue
		}
		parts = append(parts, name+":"+col.DType().String())
	}
	return "q.query.kernel:" + strings.Join(parts, ",")
}

func qRowColumnNames(row *Table) ([]string, error) {
	names := make([]string, 0, row.Length())
	ok := row.ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() {
			return false
		}
		names = append(names, key.Str())
		return true
	})
	if !ok {
		return nil, fmt.Errorf("frame row must be a plain string-keyed table")
	}
	sort.Strings(names)
	return names, nil
}

func qDataArrayFromDense(col *DenseArray) (data.Array, error) {
	switch col.DType() {
	case DenseArrayF64:
		xs := make([]float64, col.Len())
		for i := range xs {
			v, err := col.At(i)
			if err != nil {
				return nil, err
			}
			xs[i] = v.Number()
		}
		return data.NewF64(xs), nil
	case DenseArrayI64:
		xs := make([]int64, col.Len())
		for i := range xs {
			v, err := col.At(i)
			if err != nil {
				return nil, err
			}
			xs[i] = v.Int()
		}
		return data.NewI64(xs), nil
	case DenseArrayBool:
		xs := make([]bool, col.Len())
		for i := range xs {
			v, err := col.At(i)
			if err != nil {
				return nil, err
			}
			xs[i] = v.Bool()
		}
		return data.NewBool(xs), nil
	default:
		return nil, fmt.Errorf("unsupported dense array type %s", col.DType())
	}
}

func qAnyValuesFromVector(v Value) ([]any, error) {
	if v.IsDenseArray() {
		col := v.DenseArray()
		out := make([]any, col.Len())
		for i := range out {
			item, err := col.At(i)
			if err != nil {
				return nil, err
			}
			out[i] = qValueToAny(item)
		}
		return out, nil
	}
	if v.IsTable() {
		tbl := v.Table()
		out := make([]any, tbl.Length())
		for i := range out {
			out[i] = qValueToAny(tbl.RawGetInt(int64(i + 1)))
		}
		return out, nil
	}
	return nil, fmt.Errorf("must be a dense array or array table")
}

func qDataFrameValue(frame data.Frame) (Value, error) {
	return qDataFrameFacadeValue(frame)
}

func qExecDictResultValue(frame data.Frame, plan *stdq.ExecDictPlan) (Value, error) {
	if plan == nil {
		return qExecResultValue(frame)
	}
	keyColumn, ok := frame.Column(plan.KeyName)
	if !ok {
		return NilValue(), fmt.Errorf("q exec dictionary key column %q not found", plan.KeyName)
	}
	valueColumn, ok := frame.Column(plan.ValueName)
	if !ok {
		return NilValue(), fmt.Errorf("q exec dictionary value column %q not found", plan.ValueName)
	}
	keys := make([]any, frame.Len())
	values := make([]any, frame.Len())
	for i := 0; i < frame.Len(); i++ {
		key, ok := keyColumn.At(i)
		if !ok {
			return NilValue(), fmt.Errorf("q exec dictionary key row %d out of range", i)
		}
		value, ok := valueColumn.At(i)
		if !ok {
			return NilValue(), fmt.Errorf("q exec dictionary value row %d out of range", i)
		}
		keys[i] = key
		values[i] = value
	}
	return qEvalDictValue(stdq.Dict{Keys: keys, Values: values})
}

func qExecResultValue(frame data.Frame) (Value, error) {
	names := frame.Schema().Names()
	if len(names) != 1 {
		keys := make([]any, 0, len(names))
		values := make([]any, 0, len(names))
		for _, name := range names {
			col, ok := frame.Column(name)
			if !ok {
				return NilValue(), fmt.Errorf("column %q not found", name)
			}
			keys = append(keys, name)
			values = append(values, col)
		}
		return qEvalDictValue(stdq.Dict{Keys: keys, Values: values})
	}
	col, ok := frame.Column(names[0])
	if !ok {
		return NilValue(), fmt.Errorf("column %q not found", names[0])
	}
	return qEvalArrayValue(col)
}

func qRowsFromDataFrame(frame data.Frame) (*Table, error) {
	value, err := qDataFrameFacadeValue(frame)
	if err != nil {
		return nil, err
	}
	return value.Table(), nil
}

func qDataFrameFacadeValue(frame data.Frame) (Value, error) {
	names := frame.Schema().Names()
	cols := NewTable()
	kindNames := make([]string, 0, len(names))
	kindMap := make(map[string]string, len(names))
	for _, name := range names {
		col, ok := frame.Column(name)
		if !ok {
			return NilValue(), fmt.Errorf("column %q not found", name)
		}
		kind, _ := frame.Schema().Kind(name)
		cols.RawSetString(string(name), dataColumnValue(kind, dataArrayFacadeValue(col, qAnyToColumnValue)))
		kindNames = append(kindNames, string(name))
		kindMap[string(name)] = string(kind)
	}
	out := NewTable()
	out.RawSetString(dataFrameMarker, BoolValue(true))
	out.RawSetString("len", IntValue(int64(frame.Len())))
	out.RawSetString("columns", TableValue(cols))
	out.RawSetString("column_names", TableValue(dataColumnNamesTable(kindNames)))
	out.RawSetString("column_kinds", TableValue(dataColumnKindsTable(kindNames, kindMap)))
	out.RawSetString("schema", TableValue(dataSchemaTable(kindNames, kindMap)))
	out.RawSetString("row", FunctionValue(&GoFunction{Name: "data.frame.row", Fn: dataFrameRowMethod(out)}))
	out.RawSetString("gather", FunctionValue(&GoFunction{Name: "data.frame.gather", Fn: dataFrameGatherMethod(out)}))
	for _, name := range kindNames {
		out.RawSetString(name, dataColumnWrappedValues(cols.RawGetString(name)))
	}
	dataDecorateFrameTable(out, nil)
	qInstallLazyDataFrameRows(out, frame)
	setDataFrameNativePayload(out, frame)
	return TableValue(out), nil
}

func qInstallLazyDataFrameRows(out *Table, frame data.Frame) {
	if out == nil || frame.Len() <= 0 {
		return
	}
	names := frame.Schema().Names()
	cache := make(map[int64]Value)
	getRow := func(key int64) (Value, bool) {
		if key < 1 || key > int64(frame.Len()) {
			return NilValue(), false
		}
		if row, ok := cache[key]; ok {
			return row, true
		}
		row := NewTable()
		rowIndex := int(key - 1)
		for _, name := range names {
			col, ok := frame.Column(name)
			if !ok {
				return NilValue(), false
			}
			v, ok := col.At(rowIndex)
			if !ok {
				return NilValue(), false
			}
			row.RawSetString(string(name), qAnyToRowValue(v))
		}
		value := TableValue(row)
		cache[key] = value
		return value, true
	}
	rows := NewTable()
	rows.RawSetString("len", IntValue(int64(frame.Len())))
	rows.SetLazyIntGetter(frame.Len(), getRow)
	out.RawSetString("rows", TableValue(rows))
	out.SetLazyIntGetter(frame.Len(), getRow)
}

func qAnyToRowValue(v any) Value {
	if data.IsNull(v) {
		return NilValue()
	}
	return qAnyToValue(v)
}

func qValueToAny(v Value) any {
	switch {
	case v.IsNil():
		return nil
	case isDataNullValue(v):
		if kind := dataNullValueKind(v); kind != "" {
			return data.NullForKind(kind)
		}
		return data.NullValue
	case v.IsBool():
		return v.Bool()
	case v.IsInt():
		return v.Int()
	case v.IsFloat():
		return v.Float()
	case v.IsString():
		return v.Str()
	case v.IsTable() && v.Table().Length() == 0:
		return nil
	default:
		return v.String()
	}
}

func qWireScalarFromValue(v Value) (any, bool) {
	switch {
	case v.IsNil():
		return nil, true
	case isDataNullValue(v):
		if kind := dataNullValueKind(v); kind != "" {
			return data.NullForKind(kind), true
		}
		return data.NullValue, true
	case v.IsBool():
		return v.Bool(), true
	case v.IsInt():
		return v.Int(), true
	case v.IsFloat():
		return v.Float(), true
	case v.IsString():
		return v.Str(), true
	case v.IsTable() && v.Table().Length() == 0:
		return nil, true
	default:
		return nil, false
	}
}

func qAnyToValue(v any) Value {
	if data.IsNull(v) {
		if kind, ok := data.NullKind(v); ok && kind != data.KindNull {
			return dataTypedNullValue(kind)
		}
		return NilValue()
	}
	switch x := v.(type) {
	case nil:
		return NilValue()
	case bool:
		return BoolValue(x)
	case int:
		return IntValue(int64(x))
	case int8:
		return IntValue(int64(x))
	case int16:
		return IntValue(int64(x))
	case int32:
		return IntValue(int64(x))
	case int64:
		return IntValue(x)
	case uint8:
		return IntValue(int64(x))
	case uint16:
		return IntValue(int64(x))
	case uint32:
		return IntValue(int64(x))
	case uint64:
		return IntValue(int64(x))
	case float32:
		return FloatValue(float64(x))
	case float64:
		return FloatValue(x)
	case string:
		return StringValue(x)
	case data.Array:
		return dataArrayFacadeValue(x, qAnyToColumnValue)
	case stdq.Dict:
		v, err := qEvalDictValue(x)
		if err != nil {
			return StringValue(fmt.Sprint(x))
		}
		return v
	case []any:
		t := NewAppendArrayTable(len(x))
		for i, item := range x {
			t.RawSetInt(int64(i+1), qAnyToColumnValue(item))
		}
		return TableValue(t)
	case data.Symbol:
		return StringValue(string(x))
	case data.Month, data.Date, data.DateTime, data.Timespan, data.Minute, data.Second, data.Time, data.Timestamp:
		if s, ok := stdq.FormatTemporal(x); ok {
			return StringValue(s)
		}
		return StringValue(fmt.Sprint(x))
	default:
		return StringValue(fmt.Sprint(x))
	}
}

func qAnyToColumnValue(v any) Value {
	if data.IsNull(v) {
		if kind, ok := data.NullKind(v); ok {
			return dataTypedNullValue(kind)
		}
		return dataNullValue()
	}
	return qAnyToValue(v)
}

func qQueryMask(s *SoA, where Value) (*DenseArray, error) {
	if where.IsNil() {
		mask, err := NewDenseArrayOfLen(DenseArrayBool, s.Len())
		if err != nil {
			return nil, err
		}
		if err := mask.Fill(BoolValue(true)); err != nil {
			return nil, err
		}
		return mask, nil
	}
	if where.IsDenseArray() {
		mask := where.DenseArray()
		if mask.DType() != DenseArrayBool {
			return nil, fmt.Errorf("q.query where mask must be a bool dense array")
		}
		if mask.Len() != s.Len() {
			return nil, fmt.Errorf("q.query where mask length mismatch")
		}
		return mask, nil
	}
	if !where.IsTable() {
		return nil, fmt.Errorf("q.query where must be a bool dense array or condition table")
	}
	tbl := where.Table()
	column := tbl.RawGetString("column")
	if column.IsNil() {
		column = tbl.RawGetInt(1)
	}
	op := tbl.RawGetString("op")
	if op.IsNil() {
		op = tbl.RawGetInt(2)
	}
	value := tbl.RawGetString("value")
	if value.IsNil() {
		value = tbl.RawGetInt(3)
	}
	if !column.IsString() || !op.IsString() {
		return nil, fmt.Errorf("q.query where condition must provide column and op")
	}
	return s.Mask(column.Str(), op.Str(), value)
}

type qSelect struct {
	Name string
	Expr Value
}

func qQuerySelects(v Value) ([]qSelect, error) {
	if v.IsNil() {
		return nil, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("q.query select must be a table")
	}
	var selects []qSelect
	ok := v.Table().ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() {
			return false
		}
		selects = append(selects, qSelect{Name: key.Str(), Expr: val})
		return true
	})
	if !ok {
		return nil, fmt.Errorf("q.query select must be a plain string-keyed table")
	}
	sort.Slice(selects, func(i, j int) bool { return selects[i].Name < selects[j].Name })
	return selects, nil
}

func qStringList(name string, v Value) ([]string, error) {
	if v.IsNil() {
		return nil, nil
	}
	if v.IsString() {
		return []string{v.Str()}, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("%s must be a string or string array", name)
	}
	n := v.Table().Length()
	out := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		item := v.Table().RawGetInt(int64(i))
		if !item.IsString() {
			return nil, fmt.Errorf("%s item %d must be a string", name, i)
		}
		out = append(out, item.Str())
	}
	return out, nil
}

func qAggregates(v Value) (map[string]string, error) {
	if v.IsNil() {
		return nil, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("q.query aggregate must be a table")
	}
	out := make(map[string]string)
	ok := v.Table().ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() || !val.IsString() {
			return false
		}
		agg := val.Str()
		switch agg {
		case "sum", "mean", "min", "max", "count":
			out[key.Str()] = agg
			return true
		default:
			return false
		}
	})
	if !ok {
		return nil, fmt.Errorf("q.query aggregate must be a plain table of supported aggregate names")
	}
	return out, nil
}

func qRows(s *SoA, mask *DenseArray, selects []qSelect) (*Table, error) {
	rows := NewTable()
	rowIndex := int64(1)
	for i := 0; i < s.Len(); i++ {
		include, err := qMaskAt(mask, i)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}
		row := NewTable()
		for _, sel := range selects {
			v, err := qEvalExprAt(s, sel.Expr, i)
			if err != nil {
				return nil, fmt.Errorf("q.query select %s: %w", sel.Name, err)
			}
			row.RawSetString(sel.Name, v)
		}
		rows.RawSetInt(rowIndex, TableValue(row))
		rowIndex++
	}
	return rows, nil
}

func qGroupedRows(s *SoA, mask *DenseArray, by []string, selects []qSelect, aggs map[string]string) (*Table, error) {
	groups := make(map[string]*qGroup)
	var order []string
	for i := 0; i < s.Len(); i++ {
		include, err := qMaskAt(mask, i)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}
		key, vals, err := qGroupKey(s, by, i)
		if err != nil {
			return nil, err
		}
		g := groups[key]
		if g == nil {
			g = &qGroup{ByValues: vals, Aggregates: make(map[string]*qAggState)}
			groups[key] = g
			order = append(order, key)
		}
		for _, sel := range selects {
			agg, ok := aggs[sel.Name]
			if !ok {
				continue
			}
			state := g.Aggregates[sel.Name]
			if state == nil {
				state = &qAggState{Kind: agg}
				g.Aggregates[sel.Name] = state
			}
			if agg == "count" {
				state.Add(IntValue(1))
				continue
			}
			v, err := qEvalExprAt(s, sel.Expr, i)
			if err != nil {
				return nil, fmt.Errorf("q.query aggregate %s: %w", sel.Name, err)
			}
			if !v.IsNumber() {
				return nil, fmt.Errorf("q.query aggregate %s requires numeric values", sel.Name)
			}
			state.Add(v)
		}
	}
	sort.Strings(order)
	rows := NewTable()
	for i, key := range order {
		g := groups[key]
		row := NewTable()
		for idx, name := range by {
			row.RawSetString(name, g.ByValues[idx])
		}
		names := make([]string, 0, len(g.Aggregates))
		for name := range g.Aggregates {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			row.RawSetString(name, g.Aggregates[name].Value())
		}
		rows.RawSetInt(int64(i+1), TableValue(row))
	}
	return rows, nil
}

type qOrderSpec struct {
	Column string
	Desc   bool
}

func qApplyOrderAndLimit(rows *Table, spec *Table) (*Table, error) {
	order, err := qOrderSpecs(spec.RawGetString("order_by"))
	if err != nil {
		return nil, err
	}
	limit, err := qLimit(spec.RawGetString("limit"))
	if err != nil {
		return nil, err
	}
	if len(order) == 0 && limit < 0 {
		return rows, nil
	}
	values := make([]Value, 0, rows.Length())
	for i := 1; i <= rows.Length(); i++ {
		row := rows.RawGetInt(int64(i))
		if !row.IsNil() {
			values = append(values, row)
		}
	}
	if len(order) > 0 {
		sort.SliceStable(values, func(i, j int) bool {
			left := values[i].Table()
			right := values[j].Table()
			for _, ord := range order {
				cmp := qCompareValues(left.RawGetString(ord.Column), right.RawGetString(ord.Column))
				if cmp == 0 {
					continue
				}
				if ord.Desc {
					return cmp > 0
				}
				return cmp < 0
			}
			return false
		})
	}
	if limit >= 0 && limit < len(values) {
		values = values[:limit]
	}
	out := NewTable()
	for i, row := range values {
		out.RawSetInt(int64(i+1), row)
	}
	return out, nil
}

func qOrderSpecs(v Value) ([]qOrderSpec, error) {
	if v.IsNil() {
		return nil, nil
	}
	if v.IsString() {
		return []qOrderSpec{{Column: v.Str()}}, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("q.query order_by must be a string, table, or array")
	}
	tbl := v.Table()
	if col := tbl.RawGetString("column"); col.IsString() {
		return []qOrderSpec{{
			Column: col.Str(),
			Desc:   qTruthy(tbl.RawGetString("desc")),
		}}, nil
	}
	n := tbl.Length()
	out := make([]qOrderSpec, 0, n)
	for i := 1; i <= n; i++ {
		item := tbl.RawGetInt(int64(i))
		switch {
		case item.IsString():
			out = append(out, qOrderSpec{Column: item.Str()})
		case item.IsTable():
			col := item.Table().RawGetString("column")
			if !col.IsString() {
				return nil, fmt.Errorf("q.query order_by item %d must provide column", i)
			}
			out = append(out, qOrderSpec{
				Column: col.Str(),
				Desc:   qTruthy(item.Table().RawGetString("desc")),
			})
		default:
			return nil, fmt.Errorf("q.query order_by item %d must be a string or table", i)
		}
	}
	return out, nil
}

func qLimit(v Value) (int, error) {
	if v.IsNil() {
		return -1, nil
	}
	if !v.IsInt() {
		return 0, fmt.Errorf("q.query limit must be an integer")
	}
	if v.Int() < 0 {
		return 0, fmt.Errorf("q.query limit must be non-negative")
	}
	return int(v.Int()), nil
}

func qCompareValues(left, right Value) int {
	switch {
	case left.IsNumber() && right.IsNumber():
		lf, rf := left.Number(), right.Number()
		if lf < rf {
			return -1
		}
		if lf > rf {
			return 1
		}
		return 0
	case left.IsString() && right.IsString():
		if left.Str() < right.Str() {
			return -1
		}
		if left.Str() > right.Str() {
			return 1
		}
		return 0
	case left.IsBool() && right.IsBool():
		if left.Bool() == right.Bool() {
			return 0
		}
		if !left.Bool() {
			return -1
		}
		return 1
	case left.IsNil() && right.IsNil():
		return 0
	case left.IsNil():
		return -1
	case right.IsNil():
		return 1
	default:
		ls, rs := left.String(), right.String()
		if ls < rs {
			return -1
		}
		if ls > rs {
			return 1
		}
		return 0
	}
}

func qTruthy(v Value) bool {
	return !(v.IsNil() || (v.IsBool() && !v.Bool()))
}

type qGroup struct {
	ByValues   []Value
	Aggregates map[string]*qAggState
}

type qAggState struct {
	Kind  string
	Count int64
	Sum   float64
	Min   float64
	Max   float64
}

func (a *qAggState) Add(v Value) {
	a.Count++
	if a.Kind == "count" {
		a.Sum++
		return
	}
	x := v.Number()
	a.Sum += x
	if a.Count == 1 || x < a.Min {
		a.Min = x
	}
	if a.Count == 1 || x > a.Max {
		a.Max = x
	}
}

func (a *qAggState) Value() Value {
	switch a.Kind {
	case "count":
		return IntValue(a.Count)
	case "mean":
		if a.Count == 0 {
			return NilValue()
		}
		return FloatValue(a.Sum / float64(a.Count))
	case "min":
		if a.Count == 0 {
			return NilValue()
		}
		return FloatValue(a.Min)
	case "max":
		if a.Count == 0 {
			return NilValue()
		}
		return FloatValue(a.Max)
	default:
		return FloatValue(a.Sum)
	}
}

func qGroupKey(s *SoA, by []string, row int) (string, []Value, error) {
	if len(by) == 0 {
		return "", nil, nil
	}
	parts := make([]string, len(by))
	values := make([]Value, len(by))
	for i, name := range by {
		col, ok := s.Column(name)
		if !ok {
			return "", nil, fmt.Errorf("q.query by column %q not found", name)
		}
		v, err := col.At(row)
		if err != nil {
			return "", nil, err
		}
		values[i] = v
		parts[i] = qValueKey(v)
	}
	return strings.Join(parts, "\x00"), values, nil
}

func qValueKey(v Value) string {
	switch {
	case v.IsInt():
		return fmt.Sprintf("i:%d", v.Int())
	case v.IsFloat():
		return fmt.Sprintf("f:%g", v.Float())
	case v.IsBool():
		return fmt.Sprintf("b:%t", v.Bool())
	case v.IsString():
		return "s:" + v.Str()
	case v.IsNil():
		return "nil"
	default:
		return v.String()
	}
}

func qEvalExprAt(s *SoA, expr Value, row int) (Value, error) {
	if expr.IsString() {
		col, ok := s.Column(expr.Str())
		if !ok {
			return NilValue(), fmt.Errorf("column %q not found", expr.Str())
		}
		return col.At(row)
	}
	if !expr.IsTable() {
		return expr, nil
	}
	tbl := expr.Table()
	op := tbl.RawGetString("op")
	if op.IsNil() {
		op = tbl.RawGetInt(1)
	}
	if !op.IsString() {
		return NilValue(), fmt.Errorf("expression table must start with an operator")
	}
	left := tbl.RawGetString("left")
	if left.IsNil() {
		left = tbl.RawGetInt(2)
	}
	right := tbl.RawGetString("right")
	if right.IsNil() {
		right = tbl.RawGetInt(3)
	}
	lv, err := qEvalExprAt(s, left, row)
	if err != nil {
		return NilValue(), err
	}
	rv, err := qEvalExprAt(s, right, row)
	if err != nil {
		return NilValue(), err
	}
	switch op.Str() {
	case "+":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return FloatValue(lv.Number() + rv.Number()), nil
	case "-":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return FloatValue(lv.Number() - rv.Number()), nil
	case "*":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return FloatValue(lv.Number() * rv.Number()), nil
	case "/":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return FloatValue(lv.Number() / rv.Number()), nil
	case "==":
		if lv.IsBool() && rv.IsBool() {
			return BoolValue(lv.Bool() == rv.Bool()), nil
		}
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return BoolValue(lv.Number() == rv.Number()), nil
	case "!=":
		if lv.IsBool() && rv.IsBool() {
			return BoolValue(lv.Bool() != rv.Bool()), nil
		}
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return BoolValue(lv.Number() != rv.Number()), nil
	case "<":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return BoolValue(lv.Number() < rv.Number()), nil
	case "<=":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return BoolValue(lv.Number() <= rv.Number()), nil
	case ">":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return BoolValue(lv.Number() > rv.Number()), nil
	case ">=":
		if !lv.IsNumber() || !rv.IsNumber() {
			return NilValue(), fmt.Errorf("operator %q requires numeric operands", op.Str())
		}
		return BoolValue(lv.Number() >= rv.Number()), nil
	default:
		return NilValue(), fmt.Errorf("operator %q is not supported", op.Str())
	}
}

func qMaskAt(mask *DenseArray, i int) (bool, error) {
	v, err := mask.At(i)
	if err != nil {
		return false, err
	}
	return v.Bool(), nil
}
