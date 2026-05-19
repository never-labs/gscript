print("case:nextvar_power_keys_more")

a = {}
for i = 0, 50 do a[2^i] = true end
assert(a[#a])
assert(a[1] and a[2] and a[4])

print("ok")
