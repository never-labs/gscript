print("case:events_concat_chain_more")

t := {}
t.__concat = func(a, b) {
  if type(a) == "table" { a = a.val }
  if type(b) == "table" { b = b.val }
  return setmetatable({val: a .. b}, t)
}

c := setmetatable({val: "c"}, t)
d := setmetatable({val: "d"}, t)
assert((c .. d .. c .. d).val == "cdcd")
x := c .. d
assert(getmetatable(x) == t && x.val == "cd")
x = 0 .. "a" .. "b" .. c .. d .. "e" .. "f" .. "g"
assert(x.val == "0abcdefg")

print("ok")
