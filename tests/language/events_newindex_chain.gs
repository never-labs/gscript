print("case:events_newindex_chain")

writes := {}
backing := {}

proxy := setmetatable({}, {
  __newindex: func(_, k, v) {
    writes[#writes + 1] = k .. "=" .. v
    backing[k] = v
  },
  __index: backing,
})

proxy.a = "one"
proxy.b = "two"
assert(proxy.a == "one" && proxy.b == "two")
assert(writes[1] == "a=one" && writes[2] == "b=two")

fallback := {existing: 4}
child := setmetatable({}, {__index: proxy, __newindex: fallback})
child.c = 9
assert(fallback.c == 9)
assert(child.a == "one")
assert(child.existing == nil)

print("ok")
