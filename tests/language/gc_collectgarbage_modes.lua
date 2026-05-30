print("case:gc_collectgarbage_modes")

local oldmode = collectgarbage("incremental")
assert(oldmode == "incremental" or oldmode == "generational")

assert(collectgarbage("generational") == "incremental")
assert(collectgarbage("generational") == "generational")
assert(collectgarbage("incremental") == "generational")
assert(collectgarbage("incremental") == "incremental")

local ok, err = pcall(collectgarbage, "setpause", 200)
assert(not ok and type(err) == "string")
ok, err = pcall(collectgarbage, "setstepmul", 200)
assert(not ok and type(err) == "string")
ok, err = pcall(collectgarbage, "setstepsize", 13)
assert(not ok and type(err) == "string")

collectgarbage(oldmode)

print("ok")
