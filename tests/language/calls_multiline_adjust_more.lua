print("case:calls_multiline_adjust_more")

t = nil
function f(a,b,c)
  local d = 'a'
  t = {a,b,c,d}
end

f(
  1,2)
assert(t[1] == 1 and t[2] == 2 and t[3] == nil and t[4] == 'a')
f(1,2,
  3,4)
assert(t[1] == 1 and t[2] == 2 and t[3] == 3 and t[4] == 'a')

local function h(a, b, c)
  return a, b, c
end
local a, b, c = h(
  10,
  20)
assert(a == 10 and b == 20 and c == nil)

a, b, c = h(10, 20,
  30, 40)
assert(a == 10 and b == 20 and c == 30)

print("ok")
