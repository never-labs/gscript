print("case:errors_pcall_xpcall_values")

local ok, a, b, c = pcall(assert, true, "a", "b")
assert(ok and a == true and b == "a" and c == "b")

local err = {tag = "error-object"}
ok, a = pcall(error, err)
assert(not ok and a == err)

ok, a = xpcall(error, tostring, "xpcall-error")
assert(not ok and a == "xpcall-error")

local handled = nil
local function handler(e)
  handled = e
  return e.tag
end

ok, a = xpcall(error, handler, err)
assert(not ok and handled == err and a == "error-object")

print("ok")
