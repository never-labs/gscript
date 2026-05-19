print("case:big_table_growth_more")

local lim = 4096 + 17
local prog = {"local y = {0"}
for i = 1, lim do
  prog[#prog + 1] = i
end
prog[#prog + 1] = "}"

assert(#prog == lim + 2)
assert(prog[1] == "local y = {0")
assert(prog[2] == 1 and prog[lim + 1] == lim and prog[lim + 2] == "}")

local joined = table.concat(prog, ";")
assert(string.sub(joined, 1, 12) == "local y = {0")
assert(string.find(joined, ";1;2;3;4;", 1, true))
assert(string.find(joined, ";" .. lim .. ";}", 1, true))

local y = {0}
for i = 1, lim do
  y[#y + 1] = i
end
assert(y[lim] == lim - 1 and y[lim + 1] == lim and #y == lim + 1)

local sum = 0
for i = lim - 9, lim + 1 do
  sum = sum + y[i]
end
assert(sum == 11 * lim - 55)

print("ok")
