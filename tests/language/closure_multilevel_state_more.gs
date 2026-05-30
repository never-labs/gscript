print("case:closure_multilevel_state_more")

w := nil

func f(x) {
  return func(y) {
    return func(z) {
      return w + x + y + z
    }
  }
}

y := f(10)
w = 1.345
assert(y(20)(30) == 60 + w)

func make(x) {
  a := "xuxu"
  return func(op, y) {
    if op == "set" {
      a = x + y
    } else {
      return a
    }
  }
}

b1 := make(1)
b2 := make(4)
assert(b1("get") == "xuxu" && b2("get") == "xuxu")
b1("set", 10)
b2("set", 10)
assert(b1("get") == 11 && b2("get") == 14)

print("ok")
