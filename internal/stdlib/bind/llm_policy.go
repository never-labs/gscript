package bind

import (
	"fmt"
	"strings"
)

var llmDefaultDeniedCapabilityClasses = []string{
	"trading",
	"portfolio",
	"generated-code",
	"network",
	"credential",
	"publish",
}

func (b *llmLibBuilder) registerPolicyHelpers() {
	b.set("policy", func(args []Value) ([]Value, error) {
		if len(args) > 1 {
			return nil, fmt.Errorf("bad argument to 'llm.policy' (optional table expected)")
		}
		var opts *Table
		if len(args) == 1 {
			if !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'llm.policy' (table expected)")
			}
			opts = args[0].Table()
		}
		return []Value{TableValue(llmCapabilityPolicyValue(opts))}, nil
	})
	checkPolicy := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.check_policy' (tool or tools table expected)")
		}
		policy := llmCapabilityPolicyValue(nil)
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.check_policy' (policy table expected)")
			}
			policy = args[1].Table()
		}
		if err := llmCheckCapabilityPolicy(args[0], policy); !err.IsNil() {
			return []Value{NilValue(), err}, nil
		}
		return []Value{BoolValue(true), NilValue()}, nil
	}
	b.set("check_policy", checkPolicy)
	b.set("checkPolicy", checkPolicy)
}

func llmCapabilityPolicyValue(opts *Table) *Table {
	policy := NewTable()
	policy.RawSetString("kind", StringValue("capability_policy"))
	policy.RawSetString("version", StringValue("capability_policy.v1"))
	policy.RawSetString("default", StringValue("deny_high_risk"))
	deny := NewSequentialArrayTable(len(llmDefaultDeniedCapabilityClasses))
	for i, class := range llmDefaultDeniedCapabilityClasses {
		deny.RawSet(IntValue(int64(i+1)), StringValue(class))
	}
	policy.RawSetString("deny", TableValue(deny))
	if opts != nil {
		if allow := opts.RawGetString("allow"); allow.IsTable() {
			policy.RawSetString("allow", allow)
		}
		if denyOverride := opts.RawGetString("deny"); denyOverride.IsTable() {
			policy.RawSetString("deny", denyOverride)
		}
	}
	return policy
}

func llmCheckCapabilityPolicy(v Value, policy *Table) Value {
	if policy == nil {
		policy = llmCapabilityPolicyValue(nil)
	}
	for _, cap := range llmPolicyCapabilities(v) {
		if llmPolicyAllowsCapability(cap, policy) {
			continue
		}
		if class := llmDeniedCapabilityClass(cap, policy); class != "" {
			err := llmErrorValue("policy", "capability denied by policy: "+cap)
			et := err.Table()
			et.RawSetString("capability", StringValue(cap))
			et.RawSetString("class", StringValue(class))
			et.RawSetString("policy", policy.RawGetString("version"))
			return err
		}
	}
	return NilValue()
}

func llmPolicyCapabilities(v Value) []string {
	if !v.IsTable() {
		return nil
	}
	t := v.Table()
	if llmLooksLikeToolTable(t) {
		return llmStringSliceFromValue(llmToolCapabilitiesValue(t))
	}
	var out []string
	for i := 1; i <= t.Length(); i++ {
		tv := t.RawGet(IntValue(int64(i)))
		if tv.IsTable() && llmLooksLikeToolTable(tv.Table()) {
			out = append(out, llmStringSliceFromValue(llmToolCapabilitiesValue(tv.Table()))...)
		}
	}
	return out
}

func llmPolicyAllowsCapability(cap string, policy *Table) bool {
	return llmPolicyListContains(policy.RawGetString("allow"), cap)
}

func llmDeniedCapabilityClass(cap string, policy *Table) string {
	deny := policy.RawGetString("deny")
	if !deny.IsTable() {
		return ""
	}
	for _, class := range llmStringSliceFromValue(deny) {
		if llmCapabilityMatchesClass(cap, class) {
			return class
		}
	}
	return ""
}

func llmPolicyListContains(v Value, cap string) bool {
	if !v.IsTable() {
		return false
	}
	for _, allowed := range llmStringSliceFromValue(v) {
		if allowed == cap {
			return true
		}
	}
	return false
}

func llmCapabilityMatchesClass(cap, class string) bool {
	cap = strings.ToLower(strings.TrimSpace(cap))
	class = strings.ToLower(strings.TrimSpace(class))
	if cap == "" || class == "" || cap == "none" {
		return false
	}
	if cap == class {
		return true
	}
	prefixes := []string{class + ".", class + ":", class + "-", class + "_"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(cap, prefix) {
			return true
		}
	}
	switch class {
	case "generated-code":
		return strings.HasPrefix(cap, "generated_code.") || strings.HasPrefix(cap, "generated.code.") || strings.Contains(cap, "generated-code")
	case "network":
		return strings.HasPrefix(cap, "net.") || strings.HasPrefix(cap, "network.")
	case "credential":
		return strings.HasPrefix(cap, "secret.") || strings.HasPrefix(cap, "secrets.") || strings.HasPrefix(cap, "credential.")
	}
	return false
}
