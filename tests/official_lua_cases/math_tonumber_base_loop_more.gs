print("case:math_tonumber_base_loop_more")

for i := 2; i <= 36; i++ {
  i2 := i * i
  i10 := i2 * i2 * i2 * i2 * i2
  assert(tonumber("\t10000000000\t", i) == i10)
}
assert(tonumber("10", 36) == 36)
assert(tonumber("  -10  ", 36) == -36)
assert(tonumber("  +1Z  ", 36) == 36 + 35)
assert(tonumber("  -1z  ", 36) == -36 + -35)

print("ok")
