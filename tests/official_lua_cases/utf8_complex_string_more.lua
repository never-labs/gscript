print("case:utf8_complex_string_more")

local x = "日本語a-4éó"
assert(utf8.len(x) == 8)
local t = {26085, 26412, 35486, 97, 45, 52, 233, 243}
local p = 1
for i = 1, #t do
  assert(utf8.codepoint(x, p, p) == t[i])
  assert(utf8.offset(x, i) == p)
  local np = utf8.offset(x, 2, p)
  if i < #t then assert(np == utf8.offset(x, i + 1)) end
  p = np or (#x + 1)
end

print("ok")
