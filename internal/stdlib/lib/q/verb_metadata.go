package q

// Verb metadata: the single registry classifying every q verb/form name for
// source-eligibility decisions. The four memoization predicates
// (EvalSourceCacheable, qEvalConstantStatementSource,
// qScriptBindingPlanTextEligible, cachedValueExprTextEligible) each keep
// their own structural rules (assignments, lambdas, literals, bare-verb
// shapes) but derive every VERB judgment — deterministic vs random vs
// stateful vs clock-reading — from this table, so the judgment cannot drift
// between layers.
//
// Population is mechanical: every verb dispatched by lookupUnaryVerb /
// lookupDyadicVerb / lookupDyadicVerbFunc (including the numeric descriptor
// maps they fall back to) gets a default deterministic entry, the
// qEvalConstantWords adverb/word forms get deterministic entries, and
// qVerbMetadataExceptions overlays the explicit non-deterministic names.
// TestQVerbMetadataCompleteness extracts the dispatch switches from eval.go
// via go/ast and fails if a new verb lacks a metadata entry.

import (
	"sort"
	"strings"
)

// qVerbProps classifies one verb or form name for memoization purposes.
type qVerbProps struct {
	// Deterministic: the result depends only on the argument values, so
	// memo layers may cache results/plans mentioning the name. Always the
	// complement of the three flags below (TestQVerbMetadataInvariants).
	Deterministic bool
	// Stateful: reads or writes workspace/session/external state
	// (insert/upsert/get/set/hopen/system forms, \x commands, `:handles).
	Stateful bool
	// ReadsClock: observes the wall clock / session environment (.z.*).
	ReadsClock bool
	// Random: draws from the session PRNG (dyadic ? roll/deal, rand).
	Random bool
	// marker is the textual scan fragment qSourceVerbFlags uses to detect
	// the name in raw source (lowercased, string literals blanked). Empty
	// for names detected structurally instead: the dyadic `?` roll/deal
	// operator and the `rand` word go through the random-verb scan, and \x
	// system commands are a statement-prefix rule. The trailing space on
	// set/get/system preserves the historical rule that only the
	// prefix-application spelling disqualifies a source.
	marker string
}

// Synthetic form names for non-word table entries.
const (
	qVerbNameRoll          = "?"   // dyadic ? : roll/deal (atom LHS), find (vector LHS)
	qVerbNameSystemCommand = "\\"  // \x system commands
	qVerbNameFileHandle    = "`:"  // file-handle symbol literals
	qVerbNameClock         = ".z." // .z namespace reads
)

// qVerbMetadataExceptions lists every name that is not a plain deterministic
// verb. Everything else in the table defaults to deterministic.
var qVerbMetadataExceptions = map[string]qVerbProps{
	// Random: results draw from the session PRNG and must never be
	// memoized or folded into cached plans (R9 invariant).
	"rand":        {Random: true},
	qVerbNameRoll: {Random: true},
	// Stateful: workspace mutation, workspace reads, IPC, filesystem,
	// system commands.
	"insert":               {Stateful: true, marker: "insert"},
	"upsert":               {Stateful: true, marker: "upsert"},
	"set":                  {Stateful: true, marker: "set "},
	"get":                  {Stateful: true, marker: "get "},
	"hopen":                {Stateful: true, marker: "hopen"},
	"hsym":                 {Stateful: true, marker: "hsym"},
	"system":               {Stateful: true, marker: "system "},
	qVerbNameSystemCommand: {Stateful: true},
	qVerbNameFileHandle:    {Stateful: true, marker: qVerbNameFileHandle},
	// Clock: per-call environment reads.
	qVerbNameClock: {ReadsClock: true, marker: qVerbNameClock},
}

// qDispatchVerbNames lists every verb name accepted by the dispatch switches
// in lookupDyadicVerb, lookupDyadicVerbFunc, and lookupUnaryVerb (their
// numeric descriptor-map fallbacks are merged mechanically in
// buildQVerbMetadataTable). TestQVerbMetadataCompleteness keeps this list in
// lockstep with the switches.
var qDispatchVerbNames = []string{
	// lookupDyadicVerb
	"+", "plus", "-", "minus", "*", "times", "%", "divide", "div", "mod",
	"^", "fill", "~", "match", "=", "equal", "equals", "<", "less",
	">", "more", "greater", "min", "max", "[", "left", "]", "right",
	"&", "and", "|", "or",
	// lookupDyadicVerbFunc
	"bin", "binr", "xbar", "xrank", "msum", "mavg", "mcount", "mmin",
	"mmax", "mdev", "ema", "xprev", "xcols", "xkey", "xgroup", "xasc",
	"xdesc", "rotate", "cut", "sublist", "cross", "ss", "ssr", "sv", "vs",
	"mmu", "wavg", "wsum", "cov", "scov", "cor", "within", "where",
	"like", "in", "intersect", "inter", "except", "union",
	// lookupUnaryVerb
	"count", "enlist", "type", "string", "lower", "upper", "trim",
	"ltrim", "rtrim", "distinct", "first", "last", "keys", "key",
	"value", "cols", "meta", "attr", "codes", "domain", "group",
	"ungroup", "raze", "avg", "var", "dev", "svar", "sdev", "med",
	"prd", "reverse", "prior", "prev", "next", "deltas", "fills",
	"differ", "ratios", "asc", "desc", "iasc", "idesc", "rank", "neg",
	"inv", "sum", "sums", "prds", "mins", "maxs", "avgs", "all", "any",
	"not", "null",
}

var qVerbMetadataTable = buildQVerbMetadataTable()

func buildQVerbMetadataTable() map[string]qVerbProps {
	deterministic := qVerbProps{Deterministic: true}
	table := make(map[string]qVerbProps, 192)
	for _, name := range qDispatchVerbNames {
		table[name] = deterministic
	}
	for name := range qNumericUnaryVerbDescriptors {
		table[name] = deterministic
	}
	for name := range qNumericDyadicFloatVerbDescriptors {
		table[name] = deterministic
	}
	// Adverb words, dyadic word forms, and literal keywords accepted by the
	// constant-statement scanner.
	for name := range qEvalConstantWords {
		table[name] = deterministic
	}
	for name, props := range qVerbMetadataExceptions {
		table[name] = props
	}
	return table
}

// qVerbMetadataLookup returns the metadata for name; ok is false when name is
// not a registered verb/form (i.e. it is a user name).
func qVerbMetadataLookup(name string) (qVerbProps, bool) {
	props, ok := qVerbMetadataTable[name]
	return props, ok
}

// qVerbMetadata returns the metadata for name, defaulting unknown names to
// deterministic (a user name contributes no verb flags of its own).
func qVerbMetadata(name string) qVerbProps {
	if props, ok := qVerbMetadataTable[name]; ok {
		return props
	}
	return qVerbProps{Deterministic: true}
}

// qVerbMemoUnsafe reports whether props disqualifies a source from
// value/plan memoization.
func qVerbMemoUnsafe(props qVerbProps) bool {
	return props.Stateful || props.ReadsClock || props.Random
}

// qVerbScanMarker pairs a textual scan fragment with the table entry it
// detects.
type qVerbScanMarker struct {
	marker string
	props  qVerbProps
}

var qVerbScanMarkers = buildQVerbScanMarkers()

func buildQVerbScanMarkers() []qVerbScanMarker {
	names := make([]string, 0, len(qVerbMetadataExceptions))
	for name, props := range qVerbMetadataExceptions {
		if props.marker != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	markers := make([]qVerbScanMarker, 0, len(names))
	for _, name := range names {
		props := qVerbMetadataExceptions[name]
		markers = append(markers, qVerbScanMarker{marker: props.marker, props: props})
	}
	return markers
}

// qSourceVerbFlags scans src and ORs together the metadata flags of every
// verb/form the source may mention. String literal contents are blanked
// before scanning; matching is conservative (a flagged marker anywhere in
// the source counts).
func qSourceVerbFlags(src string) qVerbProps {
	var flags qVerbProps
	scan := qEvalCacheScanText(src)
	lower := strings.ToLower(scan)
	for _, part := range splitQScriptStatements(lower) {
		if strings.HasPrefix(strings.TrimSpace(part), "\\") {
			flags = qVerbFlagsUnion(flags, qVerbMetadata(qVerbNameSystemCommand))
			break
		}
	}
	for _, m := range qVerbScanMarkers {
		if strings.Contains(lower, m.marker) {
			flags = qVerbFlagsUnion(flags, m.props)
		}
	}
	if qScanMentionsRandomVerb(scan) {
		flags.Random = true
	}
	return flags
}

func qVerbFlagsUnion(a, b qVerbProps) qVerbProps {
	return qVerbProps{
		Stateful:   a.Stateful || b.Stateful,
		ReadsClock: a.ReadsClock || b.ReadsClock,
		Random:     a.Random || b.Random,
	}
}

// qScanMentionsRandomVerb conservatively reports whether the (literal-
// blanked) scan text may invoke a Random-flagged verb: the dyadic `?`
// roll/deal operator or a random word verb (rand). Any dyadic `?` counts
// unless it is the functional query prefix `?[...]` or a prefix `?x`
// distinct; the textual scan cannot know whether a name LHS is an integer
// atom, so vector-LHS find expressions are also (safely) excluded from
// memoization.
func qScanMentionsRandomVerb(scan string) bool {
	for i := 0; i < len(scan); i++ {
		ch := scan[i]
		if ch == '`' {
			i = qSymbolLiteralEnd(scan, i) - 1
			continue
		}
		if ch == '?' {
			// Functional query `?[t;...]` is deterministic.
			j := i + 1
			for j < len(scan) && isQWhitespace(scan[j]) {
				j++
			}
			if j < len(scan) && scan[j] == '[' {
				continue
			}
			// Prefix `?x` distinct: `?` at expression start.
			k := i - 1
			for k >= 0 && isQWhitespace(scan[k]) {
				k--
			}
			if k < 0 {
				continue
			}
			switch scan[k] {
			case '(', ';', '[', ',', '!', '+', '-', '*', '%', '&', '|', '^', '=', '<', '>', ':', '#', '_', '@', '~':
				continue
			}
			if qVerbMetadata(qVerbNameRoll).Random {
				return true
			}
			continue
		}
		if isQIdentStart(ch) {
			start := i
			j := i
			for j < len(scan) && isQIdentByte(scan[j]) {
				j++
			}
			word := scan[start:j]
			i = j - 1
			if start > 0 && isQIdentByte(scan[start-1]) {
				continue
			}
			if qVerbMetadata(word).Random {
				return true
			}
		}
	}
	return false
}
