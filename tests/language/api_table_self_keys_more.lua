print("case:api_table_self_keys_more")

do
  local a = {}
  a[a] = 10
  assert(a[a] == 10)

  rawset(a, a, 20)
  assert(rawget(a, a) == 20)

  rawset(a, 30, a)
  assert(a[30] == a)

  a[40] = a
  assert(rawget(a, 40) == a)
end

do
  local a = {x = 0, y = 12}
  assert(a.x == 0 and a.y == 12)
  a.x = 15
  assert(a.x == 15)

  a[a] = print
  assert(a[a] == print)
  a[a] = "xuxu"
  assert(rawget(a, a) == "xuxu")
end

print("ok")
