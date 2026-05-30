print("case:nextvar_empty_loops_more")

assert(next({}) == nil)
assert(next({}, nil) == nil)
for a, b := range pairs({}) { error("not here") }
for i := 1; i <= 0; i++ { error("not here") }
for i := 0; i >= 1; i-- { error("not here") }
a := nil
for i := 1; i <= 1; i++ { assert(!a); a = 1 }
assert(a)
a = nil
for i := 1; i >= 1; i-- { assert(!a); a = 1 }
assert(a)

print("ok")
