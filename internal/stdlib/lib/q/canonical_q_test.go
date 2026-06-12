package q

// Canonical-q golden harness.
//
// testdata/canonical_q.tsv carries expr -> expected pairs hand-encoded from
// canonical kdb+/q semantics (NOT derived from this implementation). Every
// entry carries a status:
//
//   match     Leia agrees with canonical q today. The entry must keep
//             matching the canonical expectation.
//   deviation Leia deliberately diverges (dialect choice). The entry must
//             reproduce the DOCUMENTED Leia behavior (leia column) and must
//             NOT match canonical q — silent drift in either direction is
//             caught. If a deviation starts matching canonical q the test
//             demands promotion to match.
//   gap       Leia is wrong or missing the feature and should converge
//             later. The entry must keep failing the canonical expectation;
//             once it starts passing the test demands promotion to match
//             (shrink-only gap count). An optional leia column pins the
//             current behavior so drift inside a gap is visible too.
//
// Expected values are written as q literals/expressions evaluated through
// Leia and compared with q match (~) semantics, or as ERROR / ERROR:substr
// for canonical errors. Set Q_CANONICAL_CLASSIFY=1 to log the empirical
// classification of every entry instead of asserting (maintenance mode).

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Shrink-only ratchets: counts may go down (promotions) but never up without
// an explicit, reviewed bump.
const (
	canonicalQGapMax       = 44
	canonicalQDeviationMax = 30
	canonicalQEntryMin     = 300
)

type canonicalQEntry struct {
	line      int
	status    string
	expr      string
	canonical string
	leia      string
	reason    string
}

func loadCanonicalQEntries(t *testing.T) []canonicalQEntry {
	t.Helper()
	path := filepath.Join("testdata", "canonical_q.tsv")
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	var entries []canonicalQEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<16), 1<<16)
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if strings.TrimSpace(text) == "" || strings.HasPrefix(strings.TrimSpace(text), "#") {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) < 3 {
			t.Fatalf("%s:%d: need at least status<TAB>expr<TAB>canonical, got %q", path, line, text)
		}
		entry := canonicalQEntry{
			line:      line,
			status:    strings.TrimSpace(fields[0]),
			expr:      fields[1],
			canonical: fields[2],
		}
		if len(fields) > 3 {
			entry.leia = fields[3]
		}
		if len(fields) > 4 {
			entry.reason = fields[4]
		}
		switch entry.status {
		case "match", "deviation", "gap":
		default:
			t.Fatalf("%s:%d: unknown status %q", path, line, entry.status)
		}
		if entry.status == "deviation" && strings.TrimSpace(entry.leia) == "" {
			t.Fatalf("%s:%d: deviation entries must document Leia behavior in the leia column", path, line)
		}
		if entry.status != "match" && strings.TrimSpace(entry.reason) == "" {
			t.Fatalf("%s:%d: %s entries must carry a reason", path, line, entry.status)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return entries
}

// canonicalQOutcome evaluates src on a fresh session, converting panics into
// a reported value so known crashers can be encoded as gap entries.
func canonicalQOutcome(src string) (value any, err error, panicked any) {
	defer func() {
		if r := recover(); r != nil {
			panicked = r
		}
	}()
	value, err = Eval(src)
	return value, err, nil
}

// canonicalQSpecMatches reports whether the observed outcome satisfies spec.
// spec is either ERROR / ERROR:substr or a q expression whose fresh-session
// value must q-match the observed value. The detail string describes the
// observed outcome for diagnostics.
func canonicalQSpecMatches(spec string, value any, err error, panicked any) (bool, string) {
	detail := describeCanonicalQOutcome(value, err, panicked)
	if panicked != nil {
		return false, detail
	}
	spec = strings.TrimSpace(spec)
	if spec == "NAN" {
		f, ok := value.(float64)
		return ok && err == nil && math.IsNaN(f), detail
	}
	if spec == "ERROR" || strings.HasPrefix(spec, "ERROR:") {
		if err == nil {
			return false, detail
		}
		want := strings.TrimPrefix(spec, "ERROR")
		want = strings.TrimPrefix(want, ":")
		return strings.Contains(err.Error(), want), detail
	}
	if err != nil {
		return false, detail
	}
	want, werr, wpanic := canonicalQOutcome(spec)
	if wpanic != nil || werr != nil {
		// The spec itself does not evaluate in Leia: cannot match. The
		// detail notes it so match entries fail loudly.
		return false, detail + fmt.Sprintf(" [spec %q does not evaluate: err=%v panic=%v]", spec, werr, wpanic)
	}
	return matchValue(value, want), detail
}

func describeCanonicalQOutcome(value any, err error, panicked any) string {
	switch {
	case panicked != nil:
		return fmt.Sprintf("PANIC: %v", panicked)
	case err != nil:
		return fmt.Sprintf("ERROR: %v", err)
	default:
		return fmt.Sprintf("%v (%T)", value, value)
	}
}

func TestCanonicalQGolden(t *testing.T) {
	entries := loadCanonicalQEntries(t)
	classify := os.Getenv("Q_CANONICAL_CLASSIFY") == "1"

	counts := map[string]int{}
	for _, entry := range entries {
		counts[entry.status]++
		entry := entry
		t.Run(fmt.Sprintf("line%03d", entry.line), func(t *testing.T) {
			value, err, panicked := canonicalQOutcome(entry.expr)
			canonOK, detail := canonicalQSpecMatches(entry.canonical, value, err, panicked)
			if classify {
				bucket := "MATCHES-CANONICAL"
				if !canonOK {
					bucket = "DIVERGES"
				}
				t.Logf("classify status=%s %s expr=%q canonical=%q got=%s", entry.status, bucket, entry.expr, entry.canonical, detail)
				return
			}
			switch entry.status {
			case "match":
				if !canonOK {
					t.Errorf("match entry diverged from canonical q\n  expr      %q\n  canonical %q\n  got       %s", entry.expr, entry.canonical, detail)
				}
			case "deviation":
				leiaOK, _ := canonicalQSpecMatches(entry.leia, value, err, panicked)
				if !leiaOK {
					t.Errorf("deviation entry drifted from documented Leia behavior (%s)\n  expr %q\n  leia %q\n  got  %s", entry.reason, entry.expr, entry.leia, detail)
				}
				if canonOK {
					t.Errorf("deviation entry now matches canonical q: promote to match and drop the allowlist reason (%s)\n  expr %q", entry.reason, entry.expr)
				}
			case "gap":
				if canonOK {
					t.Errorf("gap entry now matches canonical q: promote status to match (shrink-only)\n  expr %q (reason: %s)", entry.expr, entry.reason)
				}
				if strings.TrimSpace(entry.leia) != "" {
					leiaOK, _ := canonicalQSpecMatches(entry.leia, value, err, panicked)
					if !leiaOK {
						t.Errorf("gap entry drifted from pinned Leia behavior (%s)\n  expr %q\n  pin  %q\n  got  %s", entry.reason, entry.expr, entry.leia, detail)
					}
				}
			}
		})
	}

	t.Logf("canonical q harness: %d entries (match=%d deviation=%d gap=%d)",
		len(entries), counts["match"], counts["deviation"], counts["gap"])
	if len(entries) < canonicalQEntryMin {
		t.Errorf("canonical corpus shrank: %d entries, floor %d", len(entries), canonicalQEntryMin)
	}
	if counts["gap"] > canonicalQGapMax {
		t.Errorf("gap count grew: %d > %d (gaps are shrink-only; fix Leia or justify a reviewed bump)", counts["gap"], canonicalQGapMax)
	}
	if counts["deviation"] > canonicalQDeviationMax {
		t.Errorf("deviation count grew: %d > %d (deviations are shrink-only; new dialect choices need review)", counts["deviation"], canonicalQDeviationMax)
	}
}
