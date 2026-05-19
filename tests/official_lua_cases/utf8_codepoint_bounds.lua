print("case:utf8_codepoint_bounds")

local function checkerror (msg, f, ...)
  local s, err = pcall(f, ...)
  assert(not s and string.find(err, msg))
end

local s = "áéí"
local a, b, c = utf8.codepoint(s, 1, #s)
assert(a == 225 and b == 233 and c == 237)

local x = {utf8.codepoint(s, 4, 3)}
assert(#x == 0)

checkerror("out of bounds", utf8.codepoint, s, #s + 1)
checkerror("out of bounds", utf8.codepoint, s, -(#s + 1), 1)
checkerror("out of bounds", utf8.codepoint, s, 1, #s + 1)

assert(utf8.codepoint(utf8.char(0x10FFFF)) == 0x10FFFF)

print("ok")
