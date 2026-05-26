package methodjit

import (
	"fmt"
	"hash/fnv"
	"reflect"
	"sort"
	"strings"
)

// AnalysisFactDomainDiff records a lightweight before/after change for one
// AnalysisResult fact map. Top-level compatibility maps use their field name;
// maps owned by a fact domain struct use Domain.Field. It intentionally stores
// only the domain count and content hash, not the map contents.
type AnalysisFactDomainDiff struct {
	Domain      string `json:"domain"`
	BeforeCount int    `json:"before_count"`
	AfterCount  int    `json:"after_count"`
	BeforeHash  uint64 `json:"before_hash"`
	AfterHash   uint64 `json:"after_hash"`
}

type analysisFactDomainSnapshot struct {
	Domain string
	Count  int
	Hash   uint64
}

func snapshotAnalysisFactDomains(fn *Function) map[string]analysisFactDomainSnapshot {
	if fn == nil || fn.Analysis == nil {
		return nil
	}
	return snapshotAnalysisResultDomains(fn.Analysis)
}

func snapshotAnalysisResultDomains(result *AnalysisResult) map[string]analysisFactDomainSnapshot {
	if result == nil {
		return nil
	}
	value := reflect.ValueOf(result)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return nil
	}
	valueType := value.Type()
	out := make(map[string]analysisFactDomainSnapshot, value.NumField())
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if field.Kind() != reflect.Map {
			continue
		}
		name := valueType.Field(i).Name
		recordAnalysisFactMapSnapshot(out, name, field)
	}
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		field, ok := analysisResultDomainStructValue(field)
		if !ok {
			continue
		}
		name := valueType.Field(i).Name
		snapshotAnalysisResultDomainStruct(out, name, field)
	}
	return out
}

func analysisResultDomainStructValue(value reflect.Value) (reflect.Value, bool) {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, false
		}
		value = value.Elem()
	}
	return value, value.Kind() == reflect.Struct
}

func snapshotAnalysisResultDomainStruct(out map[string]analysisFactDomainSnapshot, domain string, value reflect.Value) {
	valueType := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if field.Kind() != reflect.Map {
			continue
		}
		recordAnalysisFactMapSnapshot(out, domain+"."+valueType.Field(i).Name, field)
	}
}

func recordAnalysisFactMapSnapshot(out map[string]analysisFactDomainSnapshot, domain string, field reflect.Value) {
	out[domain] = analysisFactDomainSnapshot{
		Domain: domain,
		Count:  field.Len(),
		Hash:   hashAnalysisFactDomain(field),
	}
}

func diffAnalysisFactDomains(before, after map[string]analysisFactDomainSnapshot) []AnalysisFactDomainDiff {
	if len(before) == 0 && len(after) == 0 {
		return nil
	}
	domains := make(map[string]bool, len(before)+len(after))
	for domain := range before {
		domains[domain] = true
	}
	for domain := range after {
		domains[domain] = true
	}
	names := make([]string, 0, len(domains))
	for domain := range domains {
		names = append(names, domain)
	}
	sort.Strings(names)

	diffs := make([]AnalysisFactDomainDiff, 0)
	for _, domain := range names {
		b := before[domain]
		a := after[domain]
		if b.Count == a.Count && b.Hash == a.Hash {
			continue
		}
		diffs = append(diffs, AnalysisFactDomainDiff{
			Domain:      domain,
			BeforeCount: b.Count,
			AfterCount:  a.Count,
			BeforeHash:  b.Hash,
			AfterHash:   a.Hash,
		})
	}
	return diffs
}

func changedAnalysisFactDomains(diffs []AnalysisFactDomainDiff) []string {
	if len(diffs) == 0 {
		return nil
	}
	out := make([]string, 0, len(diffs))
	for _, diff := range diffs {
		out = append(out, diff.Domain)
	}
	return out
}

func hashAnalysisFactDomain(field reflect.Value) uint64 {
	if field.IsNil() {
		return 0
	}
	entryHashes := make([]uint64, 0, field.Len())
	iter := field.MapRange()
	for iter.Next() {
		h := fnv.New64a()
		fmt.Fprintf(h, "%#v=%#v", iter.Key().Interface(), iter.Value().Interface())
		entryHashes = append(entryHashes, h.Sum64())
	}
	sort.Slice(entryHashes, func(i, j int) bool {
		return entryHashes[i] < entryHashes[j]
	})
	h := fnv.New64a()
	fmt.Fprintf(h, "len=%d", field.Len())
	for _, entryHash := range entryHashes {
		fmt.Fprintf(h, ":%016x", entryHash)
	}
	return h.Sum64()
}

func moduleFactDiffsFromRuns(runs []Tier2ModuleRun) []Tier2ModuleFactDiff {
	if len(runs) == 0 {
		return nil
	}
	out := make([]Tier2ModuleFactDiff, 0)
	for _, run := range runs {
		record, ok := run.FactDiffRecord()
		if !ok {
			continue
		}
		out = append(out, record)
	}
	return out
}

// Tier2ModuleFactDiff records actual AnalysisResult domains changed by one
// optimizer module.
type Tier2ModuleFactDiff struct {
	Phase          Tier2OptimizerPhase      `json:"phase"`
	ModuleName     string                   `json:"module"`
	StageName      string                   `json:"stage"`
	ChangedDomains []string                 `json:"changed_domains,omitempty"`
	ActualFactDiff []AnalysisFactDomainDiff `json:"actual_fact_diff,omitempty"`
}

func (run Tier2ModuleRun) FactDiffRecord() (Tier2ModuleFactDiff, bool) {
	if len(run.ActualFactDiff) == 0 {
		return Tier2ModuleFactDiff{}, false
	}
	return Tier2ModuleFactDiff{
		Phase:          run.Phase,
		ModuleName:     run.ModuleName,
		StageName:      run.StageName,
		ChangedDomains: append([]string(nil), run.ChangedDomains...),
		ActualFactDiff: append([]AnalysisFactDomainDiff(nil), run.ActualFactDiff...),
	}, true
}

func FormatTier2ModuleFactDiffs(records []Tier2ModuleFactDiff) string {
	if len(records) == 0 {
		return "(not recorded)\n"
	}
	var b strings.Builder
	for _, record := range records {
		if len(record.ActualFactDiff) == 0 {
			continue
		}
		fmt.Fprintf(&b, "  %s/%s: %s\n", record.Phase, record.ModuleName, strings.Join(record.ChangedDomains, ", "))
		for _, diff := range record.ActualFactDiff {
			fmt.Fprintf(&b, "    %s: count %d->%d hash %016x->%016x\n",
				diff.Domain, diff.BeforeCount, diff.AfterCount, diff.BeforeHash, diff.AfterHash)
		}
	}
	if b.Len() == 0 {
		return "(not recorded)\n"
	}
	return b.String()
}
