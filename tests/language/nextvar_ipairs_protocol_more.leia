print("case:nextvar_ipairs_protocol_more")

x := 0
for k, v := range ipairs({10, 20, 30, x: 12}) {
  x = x + 1
  assert(k == x && v == x * 10)
}

for _ := range ipairs({x: 12, y: 24}) {
  assert(nil)
}

assert(type(ipairs({})) == "function" && ipairs({}) == ipairs({}))

print("ok")
