print("case:errors_non_string_messages_deeper")

local t = {msg = "x"}

local res, msg = pcall(function () error(t) end)
assert(not res and msg == t)

local function f() error({msg = "x"}) end
res, msg = xpcall(f, function (r) return {msg = r.msg .. "y"} end)
assert(not res and msg.msg == "xy")

print("ok")
