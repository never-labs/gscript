package q

import "testing"

func TestQScriptBindingPlanMemoizesCacheabilityWithoutCachingNames(t *testing.T) {
	plan := buildQScriptBindingPlanForRHS("x+1", nil)
	if plan.kind == qScriptBindingInvalid {
		t.Fatalf("buildQScriptBindingPlanForRHS returned invalid plan")
	}
	state := NewEvalState(map[string]any{"x": int64(10)})
	got, handled, err := state.evalQScriptBindingPlan(&plan)
	if err != nil || !handled || got != int64(11) {
		t.Fatalf("first evalQScriptBindingPlan = %#v,%v,%v; want 11,true,nil", got, handled, err)
	}
	if !plan.cacheableKnown {
		t.Fatalf("plan cacheability was not memoized")
	}
	if plan.cacheable {
		t.Fatalf("plan with name binding was marked value-cacheable")
	}
	if plan.cached {
		t.Fatalf("plan with name binding cached its result")
	}
	state.env["x"] = int64(20)
	got, handled, err = state.evalQScriptBindingPlan(&plan)
	if err != nil || !handled || got != int64(21) {
		t.Fatalf("second evalQScriptBindingPlan = %#v,%v,%v; want 21,true,nil", got, handled, err)
	}
}

func TestQScriptBindingPlanCachesLiteralDerivedValue(t *testing.T) {
	plan := buildQScriptBindingPlanForRHS("til 8", nil)
	if plan.kind == qScriptBindingInvalid {
		t.Fatalf("buildQScriptBindingPlanForRHS returned invalid plan")
	}
	state := NewEvalState(nil)
	got, handled, err := state.evalQScriptBindingPlan(&plan)
	if err != nil || !handled {
		t.Fatalf("first evalQScriptBindingPlan = %#v,%v,%v", got, handled, err)
	}
	if !plan.cacheableKnown || !plan.cacheable {
		t.Fatalf("literal-derived plan cacheability = known:%v cacheable:%v, want true,true", plan.cacheableKnown, plan.cacheable)
	}
	if !plan.cached || plan.cache == nil {
		t.Fatalf("literal-derived plan did not cache its value")
	}
	got2, handled, err := state.evalQScriptBindingPlan(&plan)
	if err != nil || !handled || got2 != got {
		t.Fatalf("second evalQScriptBindingPlan = %#v,%v,%v; want cached %#v,true,nil", got2, handled, err, got)
	}
}
