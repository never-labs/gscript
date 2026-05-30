print("case:errors_non_string_messages_more")

local t = {}

local res, msg = pcall(function () error(t) end)
assert(not res and msg == t)

local function error_nil ()
  error(nil)
end
res, msg = pcall(error_nil)
assert(not res)

local function fail_table ()
  error({msg = "x"})
end
res, msg = xpcall(fail_table, function (r) return {msg = r.msg .. "y"} end)
assert(not res and msg.msg == "xy")

res, msg = pcall(assert, false, "X", t)
assert(not res and msg == "X")

res, msg = pcall(assert, false, t)
assert(not res)

res, msg = pcall(assert, nil, nil)
assert(not res)

print("ok")
