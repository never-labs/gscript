package q

import (
	"math"
	"strings"
	"testing"
)

// TestEvalSessionPlannedMatchesEval pins the planned-handle contract: a
// pinned handle executes the same cached plan chain as EvalSession.Eval for
// the pinned source, binds session state at execution time (not at plan
// time), and pins the same cache entry Eval resolves per call.
func TestEvalSessionPlannedMatchesEval(t *testing.T) {
	session := NewEvalSession(nil)
	if _, err := session.Eval("x:41"); err != nil {
		t.Fatalf("EvalSession.Eval(x:41): %v", err)
	}

	planned := session.Planned("x+1")
	if planned == nil {
		t.Fatal("EvalSession.Planned returned nil handle")
	}
	got, err := planned.Eval()
	if err != nil || got != int64(42) {
		t.Fatalf("planned.Eval = %#v,%v; want 42,nil", got, err)
	}
	want, err := session.Eval("x+1")
	if err != nil || want != got {
		t.Fatalf("EvalSession.Eval(x+1) = %#v,%v; want planned result %#v", want, err, got)
	}

	// State binds at execution time: rebinding x must be visible through the
	// already-pinned handle, exactly as a fresh Eval would see it.
	if _, err := session.Eval("x:100"); err != nil {
		t.Fatalf("EvalSession.Eval(x:100): %v", err)
	}
	got, err = planned.Eval()
	if err != nil || got != int64(101) {
		t.Fatalf("planned.Eval after rebind = %#v,%v; want 101,nil", got, err)
	}

	// The handle pins the same cache entry Eval resolves per call.
	if entry := session.cache["x+1"]; entry == nil || entry != planned.entry {
		t.Fatalf("planned handle entry %p != session cache entry %p", planned.entry, session.cache["x+1"])
	}

	// Executable-pipeline sources go through the same chain.
	plannedPipeline := session.Planned("x:til 8192;idx:where x>=0;+/idx")
	if plannedPipeline == nil {
		t.Fatal("EvalSession.Planned(script pipeline) returned nil handle")
	}
	for i := 0; i < 2; i++ {
		got, err = plannedPipeline.Eval()
		if err != nil || got != int64(33550336) {
			t.Fatalf("planned pipeline Eval #%d = %#v,%v; want 33550336,nil", i, got, err)
		}
	}

	// Nil-safety mirrors the rest of the session API.
	if (*EvalSession)(nil).Planned("1+2") != nil {
		t.Fatal("nil session Planned must return nil")
	}
	if _, err := (*EvalSessionPlanned)(nil).Eval(); err == nil {
		t.Fatal("nil planned handle Eval must error")
	}
}

func TestEvalSessionWarmKeepsScriptPlanPriority(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	session := NewEvalSession(nil)
	tests := []struct {
		src  string
		want any
	}{
		{
			src:  "x:8192#0Nh 1h 5h 9h;v:til 8192;idx:where x>2h;(+/v[idx])+count idx",
			want: int64(16783360),
		},
		{
			src:  "x:8192#0Ni 1i 5i 9i;v:til 8192;idx:where x<6i;(+/v[idx])+count idx",
			want: int64(25165824),
		},
	}
	for _, tt := range tests {
		for i := 0; i < 2; i++ {
			got, err := session.Eval(tt.src)
			if err != nil || got != tt.want {
				t.Fatalf("EvalSession.Eval #%d = %#v,%v; want %#v,nil", i, got, err, tt.want)
			}
		}
	}

	hits := uint64(0)
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "QScriptWhereIndexSumPlan" && stat.Shape == "where-index-reduce/periodic-sum-count" && stat.Outcome == "hit" {
			hits += stat.Count
		}
	}
	if hits != 4 {
		t.Fatalf("QScriptWhereIndexSumPlan hits = %d, want 4; stats=%#v", hits, RuntimeKernelExecutionStats())
	}
}

func TestQScriptWhereIndexSumPlanRecognizesCompositePredicates(t *testing.T) {
	tests := []string{
		"x:((til 128) mod 16)*0.5;idx:where x within 2.5 5.0;(+/x[idx])+count idx",
		"x:til 128;y:(x mod 16)+5;idx:where (y>8) and (y<16) and (not (y in 10 12));(+/y[idx])+count idx",
		"x:til 8192;y:(x mod 97)+5;idx:where (y>20) and (y<90) and ((y mod 4)=1) and (not (y in 25 33 41));(+/y[idx])+count idx",
		"x:til 8192;sym:8192#`AAPL`MSFT`NVDA`TSLA;v:x*3;idx:where (sym in `AAPL`NVDA) and (x>128) and ((x mod 3)=0);(+/v[idx])+count idx",
		"x:til 8192;p:(x mod 64)*0.25;idx:where (p>4.5) and ((x mod 3)=0) and (x<7500);g:x[idx];(+/g)+count idx",
	}
	for _, src := range tests {
		plan := buildQScriptPlan(src)
		if plan.whereIndexSum == nil {
			t.Fatalf("where-index sum plan missing for %q", src)
		}
		if _, _, _, ok := qScriptWhereIndexSum(plan.whereIndexSum); !ok {
			t.Fatalf("where-index sum plan did not close for %q: %#v", src, plan.whereIndexSum)
		}
	}
}

func TestQScriptWhereIndexFbySumPlan(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	session := NewEvalSession(nil)
	tests := []struct {
		src  string
		want int64
	}{
		{
			src:  "px:100+((til 16) mod 8);sym:16#`A`B;idx:where px>=104;vals:(100+(idx mod 8))*(1+(idx mod 3));n:@[16#0;idx;+;vals];s:sum n fby sym;(+/s)+count idx",
			want: 12664,
		},
		{
			src:  "px:96#100 0Ni 105 110 0Ni 115;sz:1+((til 96) mod 20);sym:96#`AAPL`MSFT`NVDA`TSLA;clean:0^px;idx:where not null px;vals:(clean[idx])*(1+(idx mod 20));n:@[96#0;idx;+;vals];s:sum n fby sym;(+/s)+count where null px",
			want: qScriptWhereIndexFbySumNullAwareWant(96),
		},
	}
	for _, tt := range tests {
		plan := buildQScriptPlan(tt.src)
		if plan.whereIndexFbySum == nil {
			t.Fatalf("where-index fby sum plan missing for %q", tt.src)
		}
		got, err := session.Eval(tt.src)
		if err != nil || got != tt.want {
			t.Fatalf("EvalSession.Eval = %#v,%v; want %d,nil", got, err, tt.want)
		}
	}
	hits := uint64(0)
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "QScriptWhereIndexFbySumPlan" && stat.Outcome == "hit" {
			hits += stat.Count
		}
	}
	if hits != uint64(len(tests)) {
		t.Fatalf("QScriptWhereIndexFbySumPlan hits = %d, want %d; stats=%#v", hits, len(tests), RuntimeKernelExecutionStats())
	}
}

func qScriptWhereIndexFbySumNullAwareWant(rows int) int64 {
	values := []int64{100, 0, 105, 110, 0, 115}
	nulls := map[int]bool{1: true, 4: true}
	var groupSums, groupCounts [4]int64
	var nullCount int64
	for row := 0; row < rows; row++ {
		groupCounts[row%4]++
		slot := row % len(values)
		if nulls[slot] {
			nullCount++
			continue
		}
		groupSums[row%4] += values[slot] * int64(1+row%20)
	}
	var total int64
	for group, sum := range groupSums {
		total += sum * groupCounts[group]
	}
	return total + nullCount
}

func TestQScriptWhereIndexWindowPlan(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	src := "px:100+((til 64) mod 50);sz:1+((til 64) mod 32);idx:where (px>=120) and (sz>=10);vals:(100+(idx mod 50))*(1+(idx mod 32));v:@[64#0;idx;+;vals];(+/16 msum v)+(+/16 mavg v)+count idx"
	plan := buildQScriptPlan(src)
	if plan.whereIndexWindow == nil {
		t.Fatalf("where-index window plan missing for %q", src)
	}
	session := NewEvalSession(nil)
	got, err := session.Eval(src)
	want := qScriptWhereIndexWindowWant(64)
	if err != nil {
		t.Fatalf("EvalSession.Eval returned error: %v", err)
	}
	value, ok := got.(float64)
	if !ok || math.Abs(value-want) > 1e-9 {
		t.Fatalf("EvalSession.Eval = %#v; want %.17g", got, want)
	}
	hits := uint64(0)
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "QScriptWhereIndexWindowPlan" && stat.Outcome == "hit" {
			hits += stat.Count
		}
	}
	if hits != 1 {
		t.Fatalf("QScriptWhereIndexWindowPlan hits = %d, want 1; stats=%#v", hits, RuntimeKernelExecutionStats())
	}
}

func qScriptWhereIndexWindowWant(rows int) float64 {
	values := make([]int64, rows)
	var selected int64
	for row := 0; row < rows; row++ {
		px := int64(100 + row%50)
		sz := int64(1 + row%32)
		if px >= 120 && sz >= 10 {
			values[row] = px * sz
			selected++
		}
	}
	var msumTotal int64
	var mavgTotal float64
	for row := range values {
		start := row - 15
		if start < 0 {
			start = 0
		}
		var window int64
		for j := start; j <= row; j++ {
			window += values[j]
		}
		msumTotal += window
		mavgTotal += float64(window) / float64(row-start+1)
	}
	return float64(msumTotal) + mavgTotal + float64(selected)
}

func TestQScriptWhereIndexSumPlanClosesLinearConstrainedPredicates(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	tests := []struct {
		src  string
		want any
	}{
		{
			src:  "x:til 8192;y:(x*3)+7;idx:where ((x mod 3)=1) and (x>64) and (not (x in 70 73 76));(+/y[idx])+count idx",
			want: int64(33577374),
		},
		{
			src:  "x:til 8192;v:(x*2)+1;idx:where (x within 100 7000) and ((x mod 5)>2) and ((x mod 7)<>0);(+/v[idx])+count idx",
			want: int64(16804124),
		},
		{
			src:  "x:til 8192;v:x+13;idx:where (not ((x mod 11)=3)) and (x within 50 8000) and (x>100);(+/v[idx])+count idx",
			want: int64(29186815),
		},
		{
			src:  "x:til 8192;v:(x*2)+1;idx:where x>64;(+/v[idx])+count idx",
			want: int64(67112766),
		},
		{
			src:  "x:til 8192;sym:8192#`AAPL`MSFT`NVDA`TSLA;v:x*3;idx:where (sym in `AAPL`NVDA) and (x>128) and ((x mod 3)=0);(+/v[idx])+count idx",
			want: int64(16778496),
		},
		{
			src:  "x:til 8192;p:(x mod 64)*0.25;idx:where (p>4.5) and ((x mod 3)=0) and (x<7500);g:x[idx];(+/g)+count idx",
			want: int64(6588270),
		},
		{
			src:  "x:8192#0N 3 6 9;v:til 8192;idx:where (not null x) and (x>4) and ((x mod 3)=0);(+/v[idx])+count idx",
			want: int64(16783360),
		},
		{
			src:  `x:til 8192;y:x mod 2;m:"b"$y;idx:where m;g:x[idx];(+/g)+count idx`,
			want: int64(16781312),
		},
	}
	for _, tt := range tests {
		got, err := NewEvalState(nil).Eval(tt.src)
		if err != nil || got != tt.want {
			t.Fatalf("Eval(%q) = %#v,%v; want %#v,nil", tt.src, got, err, tt.want)
		}
	}
	hits := uint64(0)
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "QScriptWhereIndexSumPlan" && stat.Shape == "where-index-reduce/periodic-sum-count" && stat.Outcome == "hit" {
			hits += stat.Count
		}
	}
	if hits != uint64(len(tests)) {
		t.Fatalf("QScriptWhereIndexSumPlan constrained hits = %d, want %d; stats=%#v", hits, len(tests), RuntimeKernelExecutionStats())
	}
}

func TestQScriptCountWherePlanClosesPeriodicPredicates(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	tests := []struct {
		src  string
		want int64
	}{
		{
			src:  "x:((til 8192) mod 8)*0.25;n:8192#0N 3 6 9;c1:count where x=1.25;c2:count where x<>0.5;c3:count where null n;c4:count where n=0N;c1+c2+c3+c4",
			want: 12288,
		},
		{
			src:  "x:til 8192;idx:where x>=4096;count idx",
			want: 4096,
		},
	}
	for _, tc := range tests {
		plan := buildQScriptPlan(tc.src)
		if plan.countWhere == nil {
			t.Fatalf("count-where plan missing for %q", tc.src)
		}
		got, err := NewEvalState(nil).Eval(tc.src)
		if err != nil || got != tc.want {
			t.Fatalf("Eval(%q) = %#v,%v; want %d,nil", tc.src, got, err, tc.want)
		}
	}
	hits := 0
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "QScriptCountWherePlan" && stat.Shape == "count-where/periodic" && stat.Outcome == "hit" {
			hits += int(stat.Count)
		}
	}
	if hits != len(tests) {
		t.Fatalf("QScriptCountWherePlan hits = %d, want %d; stats=%#v", hits, len(tests), RuntimeKernelExecutionStats())
	}
}

func TestQScriptWhereIndexOnlySumPlan(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	session := NewEvalSession(nil)
	tests := []struct {
		src  string
		want int64
	}{
		{
			src:  "x:til 8192;idx:where x>=128;+/idx",
			want: qArithmeticSeriesSumI64(128, 1, 8192-128),
		},
		{
			src:  "x:til 8192;idx:where x>=4096;sum idx",
			want: qArithmeticSeriesSumI64(4096, 1, 8192-4096),
		},
		{
			src:  "x:til 8192;m:x mod 10;idx:where (m>=3) and m<8;+/idx",
			want: qWhereIndexOnlySumModuloBandWant(8192),
		},
		{
			src:  "x:til 8192;idx:where ((x mod 3)=1) and (x>64) and (not (x in 70 73 76));+/idx",
			want: qWhereIndexOnlySumConstrainedWant(8192),
		},
	}
	for _, tt := range tests {
		for i := 0; i < 2; i++ {
			got, err := session.Eval(tt.src)
			if err != nil || got != tt.want {
				t.Fatalf("EvalSession.Eval #%d %q = %#v,%v; want %d,nil", i, tt.src, got, err, tt.want)
			}
		}
	}
	hits := uint64(0)
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "QScriptWhereIndexOnlySumPlan" && stat.Shape == "where-index-reduce/sum" && stat.Outcome == "hit" {
			hits += stat.Count
		}
	}
	if hits != uint64(len(tests)*2) {
		t.Fatalf("QScriptWhereIndexOnlySumPlan hits = %d, want %d; stats=%#v", hits, len(tests)*2, RuntimeKernelExecutionStats())
	}
}

func TestQScriptNumericMultiSumPlan(t *testing.T) {
	ClearRuntimeKernelExecutionStats()
	t.Cleanup(ClearRuntimeKernelExecutionStats)

	session := NewEvalSession(nil)
	tests := []struct {
		src  string
		want int64
	}{
		{
			src:  "x:til 8192;y:(x*3)+7;z:y-(x*2);+/z",
			want: qArithmeticSeriesSumI64(7, 1, 8192),
		},
		{
			src:  "x:til 8192;y:(x*2)+1;(+/y)+(+/y mod 3)",
			want: qNumericMultiSumModuloWant(8192, 2, 1, 3),
		},
		{
			src:  "x:til 8192;y:(x*5)+11;(+/y)+(+/y mod 7)+(+/x)",
			want: qNumericMultiSumModuloWant(8192, 5, 11, 7) + qArithmeticSeriesSumI64(0, 1, 8192),
		},
	}
	for _, tt := range tests {
		plan := buildQScriptPlan(tt.src)
		if strings.Count(tt.src, "+/") > 1 {
			if plan.numericMultiSum == nil {
				t.Fatalf("numeric multi-sum plan missing for %q", tt.src)
			}
		} else if plan.numericSum == nil {
			t.Fatalf("numeric sum plan missing for %q", tt.src)
		}
		for i := 0; i < 2; i++ {
			got, err := session.Eval(tt.src)
			if err != nil || got != tt.want {
				t.Fatalf("EvalSession.Eval #%d %q = %#v,%v; want %d,nil", i, tt.src, got, err, tt.want)
			}
		}
	}
	hits := uint64(0)
	for _, stat := range RuntimeKernelExecutionStats() {
		if stat.Kernel == "QScriptNumericMultiSumPlan" && stat.Shape == "multi-vector-reduce/int-expr-sum" && stat.Outcome == "hit" {
			hits += stat.Count
		}
	}
	wantHits := uint64((len(tests) - 1) * 2)
	if hits != wantHits {
		t.Fatalf("QScriptNumericMultiSumPlan hits = %d, want %d; stats=%#v", hits, wantHits, RuntimeKernelExecutionStats())
	}
}

func qNumericMultiSumModuloWant(rows int, mul, add, mod int64) int64 {
	var sum int64
	for i := int64(0); i < int64(rows); i++ {
		y := i*mul + add
		sum += y + qTestPositiveMod(y, mod)
	}
	return sum
}

func qTestPositiveMod(value, mod int64) int64 {
	out := value % mod
	if out < 0 {
		out += mod
	}
	return out
}

func qWhereIndexOnlySumModuloBandWant(rows int) int64 {
	var sum int64
	for i := 0; i < rows; i++ {
		m := i % 10
		if m >= 3 && m < 8 {
			sum += int64(i)
		}
	}
	return sum
}

func qWhereIndexOnlySumConstrainedWant(rows int) int64 {
	var sum int64
	for i := 0; i < rows; i++ {
		if i%3 == 1 && i > 64 && i != 70 && i != 73 && i != 76 {
			sum += int64(i)
		}
	}
	return sum
}
