package methodjit

import (
	"fmt"
	"sort"
	"strings"
)

// OptimizationRemark is one structured note produced by the Tier 2 pipeline.
// Remarks are diagnostics, not a compilation control path: production compiles
// pass nil and pay no allocation cost.
type OptimizationRemark struct {
	Pass    string            `json:"pass"`
	Kind    string            `json:"kind"`
	BlockID int               `json:"block_id"`
	ValueID int               `json:"value_id"`
	Op      string            `json:"op,omitempty"`
	Reason  string            `json:"reason"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// OptimizationRemarks collects a bounded, de-duplicated remark stream.
type OptimizationRemarks struct {
	items []OptimizationRemark
	seen  map[string]bool
}

const maxOptimizationRemarks = 256

func (r *OptimizationRemarks) Add(pass, kind string, blockID, valueID int, op Op, reason string) {
	r.AddWithFields(pass, kind, blockID, valueID, op, reason, nil)
}

func (r *OptimizationRemarks) AddWithFields(pass, kind string, blockID, valueID int, op Op, reason string, fields map[string]string) {
	if r == nil || reason == "" || len(r.items) >= maxOptimizationRemarks {
		return
	}
	remark := OptimizationRemark{
		Pass:    pass,
		Kind:    kind,
		BlockID: blockID,
		ValueID: valueID,
		Reason:  reason,
		Fields:  cloneOptimizationRemarkFields(fields),
	}
	if op != OpNop {
		remark.Op = op.String()
	}
	key := fmt.Sprintf("%s|%s|%d|%d|%s|%s|%s", remark.Pass, remark.Kind, remark.BlockID, remark.ValueID, remark.Op, remark.Reason, optimizationRemarkFieldsKey(remark.Fields))
	if r.seen == nil {
		r.seen = make(map[string]bool)
	}
	if r.seen[key] {
		return
	}
	r.seen[key] = true
	r.items = append(r.items, remark)
}

func cloneOptimizationRemarkFields(fields map[string]string) map[string]string {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]string, len(fields))
	for key, value := range fields {
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func optimizationRemarkFieldsKey(fields map[string]string) string {
	if len(fields) == 0 {
		return ""
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		if b.Len() != 0 {
			b.WriteByte(';')
		}
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(fields[key])
	}
	return b.String()
}

func (r *OptimizationRemarks) List() []OptimizationRemark {
	if r == nil || len(r.items) == 0 {
		return nil
	}
	out := make([]OptimizationRemark, len(r.items))
	copy(out, r.items)
	cloneOptimizationRemarkListFields(out)
	return out
}

func (r *OptimizationRemarks) Len() int {
	if r == nil {
		return 0
	}
	return len(r.items)
}

func (r *OptimizationRemarks) Since(start int) []OptimizationRemark {
	if r == nil || start >= len(r.items) {
		return nil
	}
	if start < 0 {
		start = 0
	}
	out := make([]OptimizationRemark, len(r.items[start:]))
	copy(out, r.items[start:])
	cloneOptimizationRemarkListFields(out)
	return out
}

func cloneOptimizationRemarkListFields(remarks []OptimizationRemark) {
	for i := range remarks {
		remarks[i].Fields = cloneOptimizationRemarkFields(remarks[i].Fields)
	}
}

func formatOptimizationRemarks(remarks []OptimizationRemark) string {
	if len(remarks) == 0 {
		return "(none)\n"
	}
	var b strings.Builder
	for _, remark := range remarks {
		loc := ""
		if remark.BlockID > 0 || remark.ValueID > 0 {
			loc = fmt.Sprintf(" B%d/v%d", remark.BlockID, remark.ValueID)
		}
		op := ""
		if remark.Op != "" {
			op = " " + remark.Op
		}
		fmt.Fprintf(&b, "  [%s] %s%s%s: %s\n", remark.Kind, remark.Pass, loc, op, remark.Reason)
	}
	return b.String()
}

func functionRemarks(fn *Function) *OptimizationRemarks {
	if fn == nil {
		return nil
	}
	return fn.Remarks
}
