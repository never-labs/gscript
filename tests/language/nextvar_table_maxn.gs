print("case:nextvar_table_maxn")

table.maxn = func(t) {
  max := 0
  for k := range pairs(t) {
    if type(k) == "number" {
      max = math.max(max, k)
    }
  }
  return max
}

assert(table.maxn({}) == 0)

a := {}
a["1000"] = true
assert(table.maxn(a) == 0)

a = {}
a["1000"] = true
a[24.5] = 3
assert(table.maxn(a) == 24.5)

a = {}
a[1000] = true
assert(table.maxn(a) == 1000)

a = {}
a[10] = true
a[100 * math.pi] = print
assert(table.maxn(a) == 100 * math.pi)

table.maxn = nil
assert(table.maxn == nil)

print("ok")
