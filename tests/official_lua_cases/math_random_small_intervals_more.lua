print("case:math_random_small_intervals_more")

local random = math.random
local function aux(x1, x2)
  local mark = {}
  local count = 0
  for i = 1, 1000 do
    local t = random(x1, x2)
    assert(x1 <= t and t <= x2)
    if not mark[t] then
      mark[t] = true
      count = count + 1
      if count == x2 - x1 + 1 then return end
    end
  end
  assert(false)
end
aux(-10, 0)
aux(1, 6)
aux(1, 2)
aux(-10, -10)
aux(math.mininteger, math.mininteger)
aux(math.maxinteger, math.maxinteger)

print("ok")
