package q

import "testing"

// qSupportedFormCanonicalExprs maps every supported special form to one
// canonical minimal expression exercising it. TestSupportedEvalFormsAllEvaluate
// evaluates each through Eval, so SupportedEvalForms cannot accumulate dead
// names and this map cannot drift from the list.
var qSupportedFormCanonicalExprs = map[string]string{
	"if":            "a:0;if[1;a:42];a",
	"do":            "a:0;do[3;a:a+2];a",
	"while":         "i:0;while[i<3;i:i+1];i",
	"cond":          "$[1;2;3]",
	"each":          "count each (1 2;3 4 5)",
	"each-prior":    "(-':)[10 15 14 20]",
	"each-left":     "(-\\:)[10;1 2 3]",
	"each-right":    "(-/:)[10 20 30;1]",
	"over":          "+/1 2 3",
	"scan":          "+\\1 2 3",
	"apply-at":      "@[10 20 30;1;+;5]",
	"apply-dot":     ".[{x+y};(2;3)]",
	"dict-literal":  "`a`b!1 2",
	"table-literal": "([] a:1 2; b:3 4)",
	"fby":           "sum 1 2 3 4 fby `a`a`b`b",
	"projection":    "p:{x+y}[1;];p[10]",
	"composition":   "(first reverse) 1 2 3",
}

func TestSupportedEvalFormsAllEvaluate(t *testing.T) {
	forms := SupportedEvalForms()
	if len(forms) == 0 {
		t.Fatal("SupportedEvalForms returned no forms")
	}
	seen := make(map[string]bool, len(forms))
	for _, form := range forms {
		if seen[form] {
			t.Errorf("SupportedEvalForms lists %q more than once", form)
		}
		seen[form] = true
		src, ok := qSupportedFormCanonicalExprs[form]
		if !ok {
			t.Errorf("form %q has no canonical expression; add one to qSupportedFormCanonicalExprs", form)
			continue
		}
		if _, err := Eval(src); err != nil {
			t.Errorf("form %q canonical expression %q failed to evaluate: %v", form, src, err)
		}
	}
	for form := range qSupportedFormCanonicalExprs {
		if !seen[form] {
			t.Errorf("qSupportedFormCanonicalExprs has stale entry %q not present in SupportedEvalForms", form)
		}
	}
}
