print("case:heavy_string_growth_small")

local a = "x"
local sizes = {}
for i = 1, 4 do
  a = a .. a .. a .. a .. a .. a .. a .. a .. a .. a
  sizes[#sizes + 1] = #a
end

assert(sizes[1] == 10 and sizes[2] == 100)
assert(sizes[3] == 1000 and sizes[4] == 10000)
assert(string.sub(a, 1, 5) == "xxxxx" and string.sub(a, -5) == "xxxxx")

local parts = {}
for i = 1, 64 do
  parts[i] = string.rep("ab", i % 5)
end
local joined = table.concat(parts, "|")
assert(string.find(joined, "abababab|", 1, true))
assert(#joined == 323)

print("ok")
