print("case:pm_gsub_table_replacement_more")

func checkerror(msg, f, ...) {
  s, err := pcall(f, ...)
  assert(!s && string.find(err, msg))
}

assert(string.gsub("x and x and x", "x", {x: "apple"}) == "apple and apple and apple")
assert(string.gsub("first second", "%w+", {first: "1st"}) == "1st second")
assert(string.gsub("a=b c=d", "(%w+)=(%w+)", {a: "A", c: "C"}) == "A C")
assert(string.gsub("a", "a", {}) == "a")
checkerror("invalid replacement value %(a table%)", string.gsub, "alo", ".", {a: {}})

print("ok")
