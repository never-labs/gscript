print("case:sort_metatable_lt_more")

tt := {
  __lt: func(a, b) {
    return a.val < b.val
  },
}

a := {}
for i := 1; i <= 10; i++ {
  a[i] = {val: 11 - i}
  setmetatable(a[i], tt)
}

table.sort(a)
for i := 2; i <= #a; i++ {
  assert(!(a[i] < a[i - 1]))
}
for i := 1; i <= 10; i++ {
  assert(a[i].val == i)
}

print("ok")
