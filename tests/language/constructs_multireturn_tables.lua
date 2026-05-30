print("case:constructs_multireturn_tables")

local function f (i)
  if type(i) ~= "number" then
    return i, "jojo"
  end
  if i > 0 then
    return i, f(i - 1)
  end
end

local x = {f("alo"), f("xixi"), nil}
assert(x[1] == "alo" and x[2] == "xixi" and x[3] == nil)

x = {f("alo") .. "xixi"}
assert(x[1] == "aloxixi")

print("ok")
