print("case:gc_collectgarbage_more2")

assert(collectgarbage("isrunning"))
collectgarbage()
local oldmode = collectgarbage("incremental")
assert(collectgarbage("generational") == "incremental")
assert(collectgarbage("generational") == "generational")
assert(collectgarbage("incremental") == "generational")
assert(collectgarbage("incremental") == "incremental")
collectgarbage("incremental")

print("ok")
