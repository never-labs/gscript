print("case:strings_format_quote_literals")

assert(string.format("%q", string.char(0)) == "\"\\0\"")
assert(string.format("%q", 12) == "12")
assert(string.format("%q", true) == "true")
assert(string.format("%q", false) == "false")
assert(string.format("%q", nil) == "nil")
assert(string.format("%q", math.huge) == "1e9999")
assert(string.format("%q", -math.huge) == "-1e9999")

ok, err := pcall(string.format, "%q", {})
pos := string.find(err, "no literal")
assert(!ok && type(err) == "string" && pos != nil)

print("ok")
