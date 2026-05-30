print("case:bitwise_direct_ops_more")

a, b, c, d := 0xF0, 0xCC, 0xAA, 0xFD
assert(a | (b ^ (c & d)) == 0xF4)
assert(^(^a) == a && ^a == (-1 ^ a))
assert((15 &^ 3) == 12)
assert((1 << 8) == 256)
assert((256 >> 4) == 16)
assert((0x12345678 << 4) == 0x123456780)
assert((0x12345678 >> 8) == 0x123456)

x := 0
for i := 0; i <= 7; i++ {
  x = x | (1 << i)
}
assert(x == 255)

print("ok")
