print("case:utf8_validation_helpers_more")

local function checkInvalid(s, pos, clean)
  local a, badpos = utf8.len(s)
  assert(a == nil and badpos == pos)
  assert(clean ~= nil)
end

local validLen = utf8.len("A" .. utf8.char(0x1f600) .. "Z")
assert(validLen == 3)

checkInvalid(string.char(0x80), 1, "?")
checkInvalid("ok" .. string.char(0xe2), 3, "ok?")
checkInvalid(string.char(0xc0, 0x80), 1, "??")
checkInvalid(string.char(0xed, 0xa0, 0x80), 1, "???")
checkInvalid(string.char(0xf4, 0x90, 0x80, 0x80), 1, "????")

print("ok")
