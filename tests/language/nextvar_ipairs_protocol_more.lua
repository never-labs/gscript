print("case:nextvar_ipairs_protocol_more")

local x = 0
for k, v in ipairs{10, 20, 30; x = 12} do
  x = x + 1
  assert(k == x and v == x * 10)
end

for _ in ipairs{x = 12, y = 24} do
  assert(nil)
end

assert(type(ipairs{}) == "function" and ipairs{} == ipairs{})

print("ok")
