print("case:gc_basic_more")

assert(collectgarbage("isrunning"))
local before = collectgarbage("count")
assert(type(before) == "number")
collectgarbage("stop")
assert(not collectgarbage("isrunning"))
collectgarbage("restart")
assert(collectgarbage("isrunning"))
assert(type(collectgarbage("step", 0)) == "boolean")
collectgarbage("collect")

print("ok")
