print("case:events_concat_metamethod")

mt := {}
mt.__concat = func(a, b) {
  if type(a) == "table" {
    a = a.val
  }
  if type(b) == "table" {
    b = b.val
  }
  return a .. b
}

c := {val: "c"}
d := {val: "d"}
setmetatable(c, mt)
setmetatable(d, mt)

assert(c .. d == "cd")
assert(0 .. "a" .. "b" .. c .. d .. "e" .. "f" .. (5 + 3) .. "g" == "0abcdef8g")

print("ok")
