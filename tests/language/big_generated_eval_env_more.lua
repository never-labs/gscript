print("case:big_generated_eval_env_more")

local lim = 128 + 7
local prog = {"local y = {0"}
for i = 1, lim do
  prog[#prog + 1] = i
end
prog[#prog + 1] = "}"
prog[#prog + 1] = ("assert(y[%d] == %d)"):format(lim, lim - 1)
prog[#prog + 1] = ("assert(y[%d] == %d)"):format(lim + 1, lim)
prog[#prog + 1] = "X = y"
prog[#prog + 1] = "return #y"

local env = {assert = assert}
local f = assert(load(table.concat(prog, ";"), "generated-big", "t", env))
assert(f() == lim + 1)
assert(env.X[lim] == lim - 1 and env.X[lim + 1] == lim)

print("ok")
