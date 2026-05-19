print("case:bwcoercion_bit32_numeric_strings")

func toint(x) {
  x = tonumber(x)
  if x == nil || x == false {
    return false
  }
  y := math.tointeger(x)
  if y == nil {
    return false
  }
  return y
}

func checkargs(x, y) {
  xi := toint(x)
  yi := toint(y)
  if xi != false && yi != false {
    return xi, yi
  }
  error("not integer")
}

func bandstr(x, y) {
  xi, yi := checkargs(x, y)
  return bit32.band(xi, yi)
}

func borstr(x, y) {
  xi, yi := checkargs(x, y)
  return bit32.bor(xi, yi)
}

func xorstr(x, y) {
  xi, yi := checkargs(x, y)
  return bit32.bxor(xi, yi)
}

func shlstr(x, y) {
  xi, yi := checkargs(x, y)
  return bit32.lshift(xi, yi)
}

func shrstr(x, y) {
  xi, yi := checkargs(x, y)
  return bit32.rshift(xi, yi)
}

func bnotstr(x) {
  xi := toint(x)
  if xi == false {
    error("not integer")
  }
  return bit32.bnot(xi)
}

assert(bandstr("7", "3.0") == 3)
assert(borstr("0x10", " 5 ") == 21)
assert(xorstr("255", "15") == 240)
assert(shlstr("3", "4") == 48)
assert(shrstr("256", "3") == 32)
assert(bnotstr("0") == 4294967295)
assert(toint("3.5") == false)
assert(toint("not a number") == false)
assert(!pcall(bandstr, "3.5", "1"))

print("ok")
