print("case:errors_xpcall_handler_more")

local res, msg = xpcall(error, function (m) return "handled:" .. m end, "boom")
assert(not res and msg == "handled:boom")

res, msg = xpcall(error, error, "inner")
assert(not res and type(msg) == "string")

local seen
res, msg = xpcall(function () error({code = 7}) end, function (r)
  seen = r
  return r.code + 1
end)
assert(not res and seen.code == 7 and msg == 8)

print("ok")
