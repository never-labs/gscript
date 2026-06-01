print("case:utf8_go_helpers_more")

mixed := "A" .. utf8.char(0x4e2d) .. utf8.char(0x1f600) .. "z"
assert(utf8.reverse(mixed) == "z" .. utf8.char(0x1f600) .. utf8.char(0x4e2d) .. "A")
assert(utf8.sub(mixed, 2, 3) == utf8.char(0x4e2d) .. utf8.char(0x1f600))
assert(utf8.upper("héllo") == "HÉLLO")
assert(utf8.lower("HÉLLO") == "héllo")
assert(utf8.charclass(65) == "L")
assert(utf8.charclass(57) == "N")
assert(utf8.charclass(32) == "S")

invalid := string.char(0xc0, 0x80)
report := utf8.validate(invalid)
assert(report.valid == false && report.pos == 1)
assert(utf8.sanitize(invalid, "?") == "??")

print("ok")
