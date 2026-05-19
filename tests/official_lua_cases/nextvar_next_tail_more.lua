print("case:nextvar_next_tail_more")

a = {}
for i = 1, 1000 do
  a[i] = i
  a[i - 1] = nil
end
assert(next(a, nil) == 1000 and next(a, 1000) == nil)

print("ok")
