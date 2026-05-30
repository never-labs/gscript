print("case:nextvar_next_sparse_tail")

local a = {}
for i = 1, 1000 do
  a[i] = i
  a[i - 1] = nil
end

local k, v = next(a, nil)
assert(k == 1000 and v == 1000)
assert(next(a, 1000) == nil)

assert(next({}) == nil)
assert(next({}, nil) == nil)

print("ok")
