print("case:gengc_running_mode_restore_more")

assert(collectgarbage("isrunning"))
collectgarbage()
local oldmode = collectgarbage("generational")
assert(collectgarbage("isrunning"))
collectgarbage("step", 0)
collectgarbage(oldmode)
assert(collectgarbage("isrunning"))

print("ok")
