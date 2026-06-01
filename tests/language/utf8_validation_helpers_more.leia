print("case:utf8_validation_helpers_more")

func checkInvalid(s, pos, clean) {
  report := utf8.validate(s)
  assert(report.valid == false)
  assert(report.pos == pos)
  assert(report.runeCount >= 0)
  assert(report.byteCount == #s)
  assert(type(report.error) == "string")
  assert(utf8.valid(s) == false)
  assert(utf8.sanitize(s, "?") == clean)
}

valid := utf8.validate("A" .. utf8.char(0x1f600) .. "Z")
assert(valid.valid)
assert(valid.pos == nil)
assert(valid.error == nil)
assert(valid.runeCount == 3)
assert(valid.byteCount == 6)

checkInvalid(string.char(0x80), 1, "?")
checkInvalid("ok" .. string.char(0xe2), 3, "ok?")
checkInvalid(string.char(0xc0, 0x80), 1, "??")
checkInvalid(string.char(0xed, 0xa0, 0x80), 1, "???")
checkInvalid(string.char(0xf4, 0x90, 0x80, 0x80), 1, "????")

assert(utf8.sanitize("a" .. string.char(0x80) .. "b") == "a" .. utf8.char(0xfffd) .. "b")

print("ok")
