print("case:gc_collectgarbage_protocol")

assert(type(collectgarbage) == "function")
assert(type(collectgarbage("count")) == "number")
assert(collectgarbage("count") >= 0)

assert(collectgarbage() == 0)
assert(collectgarbage("collect") == 0)
assert(type(collectgarbage("step", 0)) == "boolean")
assert(type(collectgarbage("step", 10000)) == "boolean")

collectgarbage("stop")
assert(collectgarbage("isrunning") == false)
collectgarbage("restart")
assert(collectgarbage("isrunning") == true)

local ok, err = pcall(collectgarbage, "invalid-option")
assert(not ok and type(err) == "string")

print("ok")
