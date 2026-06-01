print("case:nextvar_pairs_delete_strings_more")

t := {}
for i := 1; i <= 20; i++ { t["k" .. i] = i }

n := 0
for k, v := range pairs(t) {
  n = n + 1
  assert(t[k] == v)
  t[k] = nil
}
assert(n == 20 && next(t) == nil)

print("ok")
