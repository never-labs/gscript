print("case:utf8_char_range_errors")

checkerror := func(msg, f, ... ) {
  s, err := pcall(f, ...)
  assert(!s && string.find(err, msg))
}

assert(utf8.char() == "")
assert(utf8.char(97, 98, 99) == "abc")

checkerror("value out of range", utf8.char, 2147483648)
checkerror("value out of range", utf8.char, -1)

print("ok")
