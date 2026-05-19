print("case:db_nested_call_flow_more")

func f(x, name) {
  name = name || "f"
  return x, name
}

g := {}
f(g).x = f(2) && f(10) + f(9)
assert(g.x == f(19))

func h(x) {
  if !x { return 3 }
  return x("a", "x")
}

assert(h(f) == "a")
assert(h(nil) == 3)

func make() {
  count := 0
  return func(delta) {
    count = count + delta
    return count
  }
}

a := make()
b := make()
assert(a(2) == 2 && a(3) == 5)
assert(b(7) == 7 && a(1) == 6)

print("ok")
