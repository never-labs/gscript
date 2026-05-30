print("case:binary_numeric_fields_more")

local function close(a, b, eps)
  return math.abs(a - b) <= eps
end

local packed = string.pack("!1<h i4 i8 B I4 I8 f d",
  -0x1234, -0x1234567, -0x123456789ab, 0xfe, 0x89abcdef, 0x123456789ab, 1.5, -2.25)
local a, b, c, d, e, f, g, h, pos = string.unpack("!1<h i4 i8 B I4 I8 f d", packed)
assert(a == -0x1234 and b == -0x1234567 and c == -0x123456789ab)
assert(d == 0xfe and e == 0x89abcdef and f == 0x123456789ab)
assert(close(g, 1.5, 0.000001) and close(h, -2.25, 0.000001) and pos == 40)
assert(string.packsize("!1<h i4 i8 B I4 I8 f d") == 39)

local big = string.pack(">h I4", -2, 0x01020304)
local x, y, nextpos = string.unpack(">h I4", "zz" .. big, 3)
assert(x == -2 and y == 0x01020304 and nextpos == 9)

assert(string.pack(">I4", 0x01020304) == "\1\2\3\4")
assert(string.pack("<I4", 0x01020304) == "\4\3\2\1")

local ok, err = pcall(string.unpack, "I4", "\1\2\3")
assert(not ok and type(err) == "string")
ok, err = pcall(string.unpack, "B", "abc", 5)
assert(not ok and type(err) == "string")
ok, err = pcall(string.pack, "h", 32768)
assert(not ok and type(err) == "string")
ok, err = pcall(string.pack, "B", -1)
assert(not ok and type(err) == "string")
ok, err = pcall(string.pack, "B", 256)
assert(not ok and type(err) == "string")

print("ok")
