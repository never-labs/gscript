print("case:heavy_generated_concat_more")

local reps = 64
local src = "local a = 'xy'; return " .. string.rep("a", reps, "..")
local f = assert(load(src, "generated-concat"))
local s = f()
assert(#s == reps * 2)
assert(string.sub(s, 1, 6) == "xyxyxy")
assert(string.sub(s, -6) == "xyxyxy")

print("ok")
