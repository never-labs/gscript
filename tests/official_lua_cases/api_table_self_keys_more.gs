print("case:api_table_self_keys_more")

self_key := {}
self_key[self_key] = 10
assert(self_key[self_key] == 10)

rawset(self_key, self_key, 20)
assert(rawget(self_key, self_key) == 20)

rawset(self_key, 30, self_key)
assert(self_key[30] == self_key)

self_key[40] = self_key
assert(rawget(self_key, 40) == self_key)

fields := {x: 0, y: 12}
assert(fields.x == 0 && fields.y == 12)
fields.x = 15
assert(fields.x == 15)

fields[fields] = print
assert(fields[fields] == print)
fields[fields] = "xuxu"
assert(rawget(fields, fields) == "xuxu")

print("ok")
