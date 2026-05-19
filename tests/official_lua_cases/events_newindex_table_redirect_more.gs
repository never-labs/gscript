print("case:events_newindex_table_redirect_more")

mt := {}
c := {}
a := setmetatable({}, mt)
mt.__newindex = c
mt.__index = c

a[1] = 10
a[2] = 20
a[3] = 90
for i := 4; i <= 20; i++ {
  a[i] = i * 10
}

assert(a[1] == 10 && a[2] == 20 && a[3] == 90)
for i := 4; i <= 20; i++ {
  assert(a[i] == i * 10)
}
assert(next(a) == nil)

mt2 := {}
mt2.__newindex = mt2
t := setmetatable({}, mt2)
t[1] = 10
assert(mt2[1] == 10)

print("ok")
