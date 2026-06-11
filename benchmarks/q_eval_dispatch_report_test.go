//go:build darwin && arm64

package benchmarks

import (
	"fmt"
	"os"
	"sort"
	"testing"

	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

// TestQEvalWarmDispatchDistribution reports, for every qEvalVectorCases
// expression evaluated on a warm session, which statement-level route handled
// each statement: compiled plans (binding/fast/pipeline/value-expr) versus
// per-call string dispatch (s.eval cascade and string probes). It is a
// diagnostic report, enabled with Q_EVAL_DISPATCH_REPORT=1; with the flag it
// prints the distribution and the string-path statement sources sorted by
// warm hit count.
func TestQEvalWarmDispatchDistribution(t *testing.T) {
	if os.Getenv("Q_EVAL_DISPATCH_REPORT") == "" {
		t.Skip("set Q_EVAL_DISPATCH_REPORT=1 to print the warm dispatch distribution")
	}
	type stringHit struct {
		src   string
		route stdq.EvalDispatchRoute
		count uint64
		cases []string
	}
	routeTotals := map[stdq.EvalDispatchRoute]uint64{}
	stringHits := map[string]*stringHit{}
	var totalStatements uint64
	for _, tc := range qEvalVectorCases {
		eval := qSessionEvalVectorEval(t)
		src := tc.expr(qEvalVectorRows)
		// Cold call builds and caches plans; warm call is what we measure.
		qEvalVectorRun(t, eval, src)
		stdq.ResetEvalDispatchTrace()
		stdq.SetEvalDispatchTraceEnabled(true)
		qEvalVectorRun(t, eval, src)
		stdq.SetEvalDispatchTraceEnabled(false)
		for _, entry := range stdq.EvalDispatchTraceSnapshot() {
			for route, count := range entry.Routes {
				routeTotals[route] += count
				totalStatements += count
				if !stdq.IsCompiledEvalDispatchRoute(route) {
					key := string(route) + "\x00" + entry.Src
					hit := stringHits[key]
					if hit == nil {
						hit = &stringHit{src: entry.Src, route: route}
						stringHits[key] = hit
					}
					hit.count += count
					hit.cases = append(hit.cases, tc.name)
				}
			}
		}
		stdq.ResetEvalDispatchTrace()
	}
	var compiled, stringPath uint64
	routes := make([]string, 0, len(routeTotals))
	for route := range routeTotals {
		routes = append(routes, string(route))
	}
	sort.Strings(routes)
	t.Logf("warm statement evals: %d", totalStatements)
	for _, route := range routes {
		count := routeTotals[stdq.EvalDispatchRoute(route)]
		if stdq.IsCompiledEvalDispatchRoute(stdq.EvalDispatchRoute(route)) {
			compiled += count
		} else {
			stringPath += count
		}
		t.Logf("route %-28s %6d (%.1f%%)", route, count, 100*float64(count)/float64(totalStatements))
	}
	t.Logf("compiled: %d (%.1f%%)  string-path: %d (%.1f%%)",
		compiled, 100*float64(compiled)/float64(totalStatements),
		stringPath, 100*float64(stringPath)/float64(totalStatements))
	hits := make([]*stringHit, 0, len(stringHits))
	for _, hit := range stringHits {
		hits = append(hits, hit)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].count != hits[j].count {
			return hits[i].count > hits[j].count
		}
		return hits[i].src < hits[j].src
	})
	limit := len(hits)
	if env := os.Getenv("Q_EVAL_DISPATCH_REPORT_TOP"); env != "" {
		fmt.Sscanf(env, "%d", &limit)
		if limit > len(hits) {
			limit = len(hits)
		}
	}
	for _, hit := range hits[:limit] {
		caseName := hit.cases[0]
		if len(hit.cases) > 1 {
			caseName = fmt.Sprintf("%s(+%d)", caseName, len(hit.cases)-1)
		}
		t.Logf("string %-24s x%-4d %-32s %q", hit.route, hit.count, caseName, hit.src)
	}
}
