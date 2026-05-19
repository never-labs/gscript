print("case:errors_common_runtime_failures")

local function fails (f)
  local ok, err = pcall(f)
  assert(not ok and type(err) == "string")
end

fails(function () return math.sin() end)
fails(function () return assert(false) end)
fails(function () return assert(nil) end)
fails(function ()
  local t = {}
  return t[#t] + 1
end)

assert(pcall(tostring, 1))
assert(not pcall(tostring))
assert(not pcall(tonumber))

print("ok")
