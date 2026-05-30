print("case:gc_generational_table_barrier_slice")

local oldmode = collectgarbage("generational")

local U = {}
collectgarbage()
U[1] = {x = {234}}
collectgarbage("step", 0)
collectgarbage("step", 0)
assert(U[1].x[1] == 234)

assert(collectgarbage("isrunning"))
collectgarbage(oldmode)

print("ok")
