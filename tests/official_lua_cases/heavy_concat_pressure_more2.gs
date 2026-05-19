print("case:heavy_concat_pressure_more2")

parts := {}
for i := 1; i <= 96; i++ {
  parts[i] = string.rep(string.char(64 + (i % 26)), i % 7)
}

joined := table.concat(parts, ":")
assert(#parts == 96)
assert(#joined == 383)
assert(string.sub(joined, 1, 5) == "A:BB:")
assert(string.find(joined, "FFFFFF:", 1, true))

s := "ab"
for i := 1; i <= 9; i++ {
  s = s .. s
}
assert(#s == 1024)
assert(string.sub(s, 1, 6) == "ababab")
assert(string.sub(s, -6) == "ababab")

print("ok")
