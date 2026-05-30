print("case:code_explicit_spread_more")

func pair() {
  return 20, 30
}

func collect(...) {
  return table.pack(...)
}

t := collect(10, spread(pair()), 40, table.spread({50, 60}))
assert(t.n == 6)
assert(t[1] == 10 && t[2] == 20 && t[3] == 30)
assert(t[4] == 40 && t[5] == 50 && t[6] == 60)

values := {1, spread(pair()), 4, table.spread({5, 6})}
assert(#values == 6)
assert(values[1] == 1 && values[2] == 20 && values[3] == 30)
assert(values[4] == 4 && values[5] == 5 && values[6] == 6)

print("ok")
