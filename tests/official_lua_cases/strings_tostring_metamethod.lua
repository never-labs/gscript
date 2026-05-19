print("case:strings_tostring_metamethod")

local m = setmetatable({}, {
  __tostring = function () return "hello" end,
  __name = "hi",
})

assert(tostring(m) == "hello")
getmetatable(m).__tostring = nil
assert(string.sub(tostring(m), 1, 4) == "hi: ")

getmetatable(m).__tostring = function () return {} end
assert(not pcall(tostring, m))

print("ok")
