print("case:strings_tostring_metamethod")

mt := {
  __tostring: func(self) { return "hello" },
  __name: "hi",
}
m := setmetatable({}, mt)

assert(tostring(m) == "hello")
getmetatable(m).__tostring = nil
assert(string.sub(tostring(m), 1, 4) == "hi: ")

getmetatable(m).__tostring = func(self) { return {} }
assert(!pcall(tostring, m))

print("ok")
