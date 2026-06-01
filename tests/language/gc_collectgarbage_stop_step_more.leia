print("case:gc_collectgarbage_stop_step_more")

collectgarbage("stop")
assert(!collectgarbage("isrunning"))
assert(type(collectgarbage("step", 0)) == "boolean")
assert(type(collectgarbage("step", 20000)) == "boolean")
assert(!collectgarbage("isrunning"))
collectgarbage("restart")
assert(collectgarbage("isrunning"))

print("ok")
