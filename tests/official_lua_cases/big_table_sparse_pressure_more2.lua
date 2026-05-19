print("case:big_table_sparse_pressure_more2")

local t = {}
for i = 1, 6000 do
  t[i] = i % 97
end

for i = 1, 6000, 3 do
  t[i] = nil
end

local sum = 0
local live = 0
for i = 1, 6000 do
  if t[i] ~= nil then
    sum = sum + t[i]
    live = live + 1
  end
end

assert(live == 4000)
assert(sum == 191728)
assert(t[2] == 2 and t[3] == 3 and t[4] == nil)
assert(t[5999] == 82 and t[6000] == 83)

print("ok")
