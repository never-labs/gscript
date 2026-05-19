print("case:errors_error_edge_values")

local ok, err = pcall(error)
assert(not ok and type(err) == "string" and string.find(err, "error"))

ok, err = pcall(error, "hi", 0)
assert(not ok and err == "hi")

ok, err = pcall(function()
  local t = {}
  return t[#t] + 1
end)
assert(not ok and type(err) == "string")

ok, err = pcall(tonumber)
assert(not ok and type(err) == "string")

print("ok")
