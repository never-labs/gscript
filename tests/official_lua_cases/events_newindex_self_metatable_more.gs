print("case:events_newindex_self_metatable_more")

t := {}
func f(tbl, i, v) {
  rawset(tbl, i, v - 3)
}

setmetatable(t, t)
t.__newindex = f

a := setmetatable({}, t)
a[1] = 30
a.x = 101
a[5] = 200

assert(a[1] == 27 && a.x == 98 && a[5] == 197)

print("ok")
