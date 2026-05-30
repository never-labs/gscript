print("case:events_index_function_parent_more")

a := {10, 20, 30, x: "10", y: "20"}
t := {}

func f(tbl, i, e) {
  assert(!e)
  p := rawget(tbl, "parent")
  if p {
    return p[i] + 3, "dummy return"
  }
  return nil, "dummy return"
}

t.__index = f
a.parent = {z: 25, x: 12}
a.parent[4] = 24
setmetatable(a, t)
assert(a[1] == 10 && a.z == 28 && a[4] == 27 && a.x == "10")

called := false
b := {}
setmetatable(b, {
  __index: func(tbl, key, extra) {
    assert(tbl == b && extra == nil)
    called = key == "missing"
    return "fallback"
  },
})
assert(b.missing == "fallback" && called)

print("ok")
