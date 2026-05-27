package methodjit

import "fmt"

const OpCountAny = -1

// OpCountPolicy describes an optional validator count contract. When Set is
// false, the validator does not enforce the count.
type OpCountPolicy struct {
	Min int
	Max int
	Set bool
}

func OpFixedCount(n int) OpCountPolicy {
	return OpCountPolicy{Min: n, Max: n, Set: true}
}

func OpRangedCount(min, max int) OpCountPolicy {
	return OpCountPolicy{Min: min, Max: max, Set: true}
}

func (p OpCountPolicy) accepts(got int) bool {
	if !p.Set {
		return true
	}
	if got < p.Min {
		return false
	}
	return p.Max == OpCountAny || got <= p.Max
}

func (p OpCountPolicy) describe() string {
	if p.Max == OpCountAny {
		return fmt.Sprintf("at least %d", p.Min)
	}
	return fmt.Sprintf("%d..%d", p.Min, p.Max)
}
