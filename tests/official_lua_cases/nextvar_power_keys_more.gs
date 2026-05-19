print("case:nextvar_power_keys_more")

a := {}
for i := 0; i <= 50; i++ { a[math.pow(2, i)] = true }
assert(a[#a])
assert(a[1] && a[2] && a[4])

print("ok")
