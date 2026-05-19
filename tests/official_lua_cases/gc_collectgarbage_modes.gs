print("case:gc_collectgarbage_modes")

oldmode := collectgarbage("incremental")
assert(oldmode == "incremental" || oldmode == "generational")

assert(collectgarbage("generational") == "incremental")
assert(collectgarbage("generational") == "generational")
assert(collectgarbage("incremental") == "generational")
assert(collectgarbage("incremental") == "incremental")

ok, err := pcall(collectgarbage, "setpause", 200)
assert(!ok && type(err) == "string")
ok, err = pcall(collectgarbage, "setstepmul", 200)
assert(!ok && type(err) == "string")
ok, err = pcall(collectgarbage, "setstepsize", 13)
assert(!ok && type(err) == "string")

collectgarbage(oldmode)

print("ok")
