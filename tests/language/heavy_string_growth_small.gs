print("case:heavy_string_growth_small")

a := "x"
sizes := {}
for i := 1; i <= 4; i++ {
  a = a .. a .. a .. a .. a .. a .. a .. a .. a .. a
  sizes[#sizes + 1] = #a
}

assert(sizes[1] == 10 && sizes[2] == 100)
assert(sizes[3] == 1000 && sizes[4] == 10000)
assert(string.sub(a, 1, 5) == "xxxxx" && string.sub(a, -5) == "xxxxx")

parts := {}
for i := 1; i <= 64; i++ {
  parts[i] = string.rep("ab", i % 5)
}
joined := table.concat(parts, "|")
assert(string.find(joined, "abababab|", 1, true))
assert(#joined == 323)

print("ok")
