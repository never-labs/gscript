print("case:utf8_codepoint_bounds")

checkerror := func(msg, f, ... ) {
  s, err := pcall(f, ...)
  assert(!s && string.find(err, msg))
}

s := "áéí"
a, b, c := utf8.codepoint(s, 1, #s)
assert(a == 225 && b == 233 && c == 237)

x := {utf8.codepoint(s, 4, 3)}
assert(#x == 0)

checkerror("out of bounds", utf8.codepoint, s, #s + 1)
checkerror("out of bounds", utf8.codepoint, s, -(#s + 1), 1)
checkerror("out of bounds", utf8.codepoint, s, 1, #s + 1)

assert(utf8.codepoint(utf8.char(1114111)) == 1114111)

print("ok")
