print("case:main_generated_chunk_eval_more")

local env = {name = "generated", base = 10}
local fn = assert(load("return base + 2, name", "generated/chunk.lua", "t", env))
local a, b = fn()
assert(a == 12 and b == "generated")
assert(type(fn) == "function")

local bad, err = load("return !", "generated/bad.lua", "t", env)
assert(bad == nil and type(err) == "string")

print("ok")
