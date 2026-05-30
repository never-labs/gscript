print("case:errors_runtime_failures_more")

local function checkerr (needle, f)
  local ok, err = pcall(f)
  assert(not ok and type(err) == "string")
end

checkerr("concatenate", function () return print .. "a" end)
checkerr("concatenate", function () return "a" .. false end)
checkerr("concatenate", function () return {} .. 2 end)
checkerr("invalid option", function () collectgarbage("nooption") end)
checkerr("yield", function () coroutine.yield() end)

local a = {}
setmetatable(a, {__index = string})
checkerr("bad self", function () return a:sub() end)

print("ok")
