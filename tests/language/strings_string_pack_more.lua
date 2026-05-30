print("case:strings_string_pack_more")

local pack = string.pack
local unpack = string.unpack
local packsize = string.packsize

assert(unpack("B", pack("B", 0xff)) == 0xff)
assert(unpack("b", pack("b", 0x7f)) == 0x7f)
assert(unpack("b", pack("b", -0x80)) == -0x80)

assert(unpack(">I2", pack(">I2", 0x1234)) == 0x1234)
assert(pack(">I2 <I2", 0x0102, 0x0304) == "\1\2\4\3")
local a, b, pos = unpack(">I2 <I2", "\1\2\4\3")
assert(a == 0x0102 and b == 0x0304 and pos == 5)

assert(pack("c0", "") == "")
assert(packsize("c0") == 0)
assert(unpack("c0", "") == "")
assert(pack("c3", "123") == "123")
assert(pack("c8", "123456") == "123456\0\0")

print("ok")
