print("case:main_multiline_chunk_values")

local function f (x)
  local a = [[
xuxu
]]
  local b = "\
xuxu\n"
  if x == 11 then return 1 + 12, 2 + 20 end
  return x + 1
end

assert(f(100) == 101)
local a, b = f(11)
assert(a == 13 and b == 22)

print("ok")
