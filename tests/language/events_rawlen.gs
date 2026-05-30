print("case:events_rawlen")

mt := {}
mt.__len = func() { return 10 }
t := {1, 2, 3}
setmetatable(t, mt)
assert(#t == 10 && rawlen(t) == 3)
assert(rawlen("abc") == 3)
assert(!pcall(rawlen, 34))
assert(!pcall(rawlen))
assert(rawlen(string.rep("a", 1000)) == 1000)

print("ok")
