print("case:sort_callback_helpers_more")

items := {
  {name: "beta", rank: 2},
  {name: "alpha", rank: 2},
  {name: "gamma", rank: 1},
  {name: "delta", rank: 3},
}

sort.by(items, func(a, b) {
  if a.rank == b.rank {
    return a.name < b.name
  }
  return a.rank < b.rank
})
assert(items[1].name == "gamma")
assert(items[2].name == "alpha")
assert(items[3].name == "beta")
assert(items[4].name == "delta")

words := {"tiny", "alphabet", "go", "lua"}
shortest := sort.min(words, func(s) {
  return #s
})
longest := sort.max(words, func(s) {
  return #s
})
assert(shortest == "go")
assert(longest == "alphabet")

limit := 3
badSortByInput := {3, 1, 2}
badSortBy := func(a, b) {
  if a == limit || b == limit {
    error("sort.by callback boom")
  }
  return a < b
}
ok, err := pcall(func() {
  sort.by(badSortByInput, badSortBy)
})
assert(!ok && err == "sort.by callback boom")

badMinInput := {"aa", "b"}
ok, err = pcall(func() {
  sort.min(badMinInput, func(s) {
    if #s == 1 {
      error("sort.min key boom")
    }
    return #s
  })
})
assert(!ok && err == "sort.min key boom")

badMaxInput := {"aa", "b"}
ok, err = pcall(func() {
  sort.max(badMaxInput, func(s) {
    if #s == 1 {
      error("sort.max key boom")
    }
    return #s
  })
})
assert(!ok && err == "sort.max key boom")

print("ok")
