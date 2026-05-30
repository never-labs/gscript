print("case:math_tonumber_base_more")

for i = 2, 36 do
  local i2 = i * i
  local i10 = i2 * i2 * i2 * i2 * i2
  assert(tonumber("\t10000000000\t", i) == i10)
end

assert(tonumber("  001010  ", 2) == 10)
assert(tonumber("  001010  ", 10) == 1010)
assert(tonumber("  -1010  ", 2) == -10)
assert(tonumber("10", 36) == 36)
assert(tonumber("  -10  ", 36) == -36)
assert(tonumber("  +1Z  ", 36) == 36 + 35)
assert(tonumber("  -1z  ", 36) == -36 + -35)

print("ok")
