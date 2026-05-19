print("case:gc_basic_more")

assert(collectgarbage("isrunning"))
before := collectgarbage("count")
assert(type(before) == "number")
collectgarbage("stop")
assert(!collectgarbage("isrunning"))
collectgarbage("restart")
assert(collectgarbage("isrunning"))
assert(type(collectgarbage("step", 0)) == "boolean")
collectgarbage("collect")

print("ok")
