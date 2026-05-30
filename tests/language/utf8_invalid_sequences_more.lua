print("case:utf8_invalid_sequences_more")

local function contains(s, needle)
  return string.find(s, needle) ~= nil
end

local function consumeCodes(s)
  for p, c in utf8.codes(s) do
    return p, c
  end
  return nil
end

local function checkBad(s)
  local a, pos = utf8.len(s)
  assert(a == nil and pos == 1)

  local ok, err = pcall(utf8.codepoint, s, 1)
  assert(not ok and contains(err, "invalid UTF"))

  ok, err = pcall(consumeCodes, s)
  assert(not ok and contains(err, "invalid UTF"))
end

checkBad(string.char(0x80))
checkBad(string.char(0xc0, 0x80))
checkBad(string.char(0xed, 0xa0, 0x80))
checkBad(string.char(0xf4, 0x90, 0x80, 0x80))

local ok, err = pcall(utf8.offset, string.char(0x80), 1)
assert(not ok and contains(err, "continuation byte"))

print("ok")
