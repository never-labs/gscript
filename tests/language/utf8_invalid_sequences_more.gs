print("case:utf8_invalid_sequences_more")

func contains(s, needle) {
  return string.find(s, needle) != nil
}

func consumeCodes(s) {
  for p, c := range utf8.codes(s) {
    return p, c
  }
  return nil
}

func checkBad(s) {
  a, pos := utf8.len(s)
  assert(a == nil && pos == 1)
  assert(!utf8.valid(s))

  ok, err := pcall(utf8.codepoint, s, 1)
  assert(!ok && contains(err, "invalid UTF"))

  ok, err = pcall(consumeCodes, s)
  assert(!ok && contains(err, "invalid UTF"))
}

checkBad(string.char(0x80))
checkBad(string.char(0xc0, 0x80))
checkBad(string.char(0xed, 0xa0, 0x80))
checkBad(string.char(0xf4, 0x90, 0x80, 0x80))

ok, err := pcall(utf8.offset, string.char(0x80), 1)
assert(!ok && contains(err, "continuation byte"))

print("ok")
