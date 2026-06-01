print("case:xpcall_iterator_edges_more")

p := table.pack(xpcall(func() { error("boom") }, func(e) {
  assert(type(e) == "string")
  return nil, "ignored"
}))
assert(p.n == 2 && p[1] == false && p[2] == nil && p[3] == nil)

p = table.pack(xpcall(func() { error("again") }, func(e) {
  assert(type(e) == "string")
  return 1, 2, 3
}))
assert(p.n == 2 && p[1] == false && p[2] == 1 && p[3] == nil)

func edge_iter(_, i) {
  i = i + 1
  if i == 1 {
    return i, "one", "extra-one"
  }
  if i == 2 {
    return i, "two", "extra-two"
  }
  return nil, "stop-extra", "ignored"
}

func edge_source() {
  return edge_iter, nil, 0
}

seen := 0
for i, label := range edge_source() {
  seen = seen + 1
  if seen == 1 {
    assert(i == 1 && label == "one")
  } else {
    assert(seen == 2 && i == 2 && label == "two")
  }
}
assert(seen == 2)

for i := range edge_source() {
  seen = seen + 10
  assert(i == 1 || i == 2)
}
assert(seen == 22)

print("ok")
