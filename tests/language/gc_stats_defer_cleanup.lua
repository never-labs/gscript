print("case:gc_stats_defer_cleanup")

local function checkStats()
  assert(type(collectgarbage("count")) == "number")
  assert(type(collectgarbage("isrunning")) == "boolean")
  return collectgarbage("isrunning")
end

local running = checkStats()
assert(type(running) == "boolean")

collectgarbage("collect")

print("ok")
