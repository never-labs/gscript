print("case:constructs_loop_break_count")

for i = 1, 1000 do
  break
end

local n = 100
local i = 3
local t = {}
local a = nil

while not a do
  a = 0
  for j = 1, n do
    for k = j, 1, -1 do
      a = a + 1
      t[k] = 1
    end
  end
end

assert(a == n * (n + 1) / 2 and i == 3)
assert(t[1] and t[n] and not t[n + 1])

print("ok")
