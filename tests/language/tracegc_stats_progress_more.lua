print("case:tracegc_stats_progress_more")

local before = collectgarbage("count")
local running = collectgarbage("isrunning")
assert(type(before) == "number")
assert(type(running) == "boolean")

local t = {}
for i = 1, 200 do
  t[i] = {i, tostring(i)}
end

collectgarbage("collect")
local after = collectgarbage("count")
assert(type(after) == "number")

print("ok")
