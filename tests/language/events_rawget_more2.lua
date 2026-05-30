print("case:events_rawget_more2")

a = {}
rawset(a, "x", 1, 2, 3)
assert(a.x == 1 and rawget(a, "x", 3) == 1)

print("ok")
