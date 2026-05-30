print("case:api_metamethod_compare_len_concat_more")

cmp_mt := {}
cmp_mt.__lt = func(a, b) { return a[1] < b[1] }
cmp_mt.__le = func(a, b) { return a[1] <= b[1] }
cmp_mt.__eq = func(a, b) { return a[1] == b[1] }

o1 := {1}
o2 := {2}
o3a := {3}
o3b := {3}
o4a := {4}
o4b := {4}
setmetatable(o1, cmp_mt)
setmetatable(o2, cmp_mt)
setmetatable(o3a, cmp_mt)
setmetatable(o3b, cmp_mt)
setmetatable(o4a, cmp_mt)
setmetatable(o4b, cmp_mt)

assert(o1 < o2)
assert(!(o3a <= o2))
assert(o3a == o3b)
assert(o4a > o3a)
assert(o4a >= o4b)

len_target := setmetatable({x: 20}, {__len: func(v) { return v.x }})
assert(#len_target == 20)
len_target.x = "234"
len_target[1] = 20
assert(#len_target == "234")

concat_target := setmetatable({x: "u"}, {
  __concat: func(l, r) { return l.x .. "." .. r.x },
})
assert(concat_target .. concat_target == "u.u")
assert("" .. "xuxu" == "xuxu")

print("ok")
