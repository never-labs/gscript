print("case:constructs_loops_tables")

local f = function (i)
  if i < 10 then return 'a'
  elseif i < 20 then return 'b'
  elseif i < 30 then return 'c'
  else return 8
  end
end

assert(f(3) == 'a' and f(12) == 'b' and f(26) == 'c' and f(100) == 8)

local n = 100
local i = 3
local t = {}
local a = nil
while not a do
  a = 0
  for i = 1, n do
    for i = i, 1, -1 do
      a = a + 1
      t[i] = 1
    end
  end
end
assert(a == n * (n + 1) / 2 and i == 3)
assert(t[1] and t[n] and not t[n + 1])

local a, b = nil, 23
x = {f(100) * 2 + 3 or a, a or b + 2}
assert(x[1] == 19 and x[2] == 25)
x = {f = 2 + 3 or a, a = b + 2}
assert(x.f == 5 and x.a == 25)

print("ok")
