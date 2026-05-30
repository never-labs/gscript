print("case:constructs_recursive_multireturn_more")

local function f(i)
  if type(i) ~= "number" then return i, "jojo" end
  if i > 0 then return i, f(i - 1) end
end

local x = {f(3), f(5), f(10)}
assert(x[1] == 3 and x[2] == 5 and x[3] == 10 and x[4] == 9 and x[12] == 1)
assert(x[nil] == nil)

x = {f("alo"), f("xixi"), nil}
assert(x[1] == "alo" and x[2] == "xixi" and x[3] == nil)

x = {f("alo") .. "xixi"}
assert(x[1] == "aloxixi")

x = {f({})}
assert(x[2] == "jojo" and type(x[1]) == "table")

print("ok")
