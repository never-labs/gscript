print("case:events_dynamic_call_metatable")

u := table.pack
for i := 1; i <= 4; i++ {
  u = setmetatable({i}, {__call: u})
}

packed := u("x", "y")
assert(packed.n == 6)
for i := 1; i <= 4; i++ {
  assert(packed[i][1] == i)
}
assert(packed[5] == "x" && packed[6] == "y")

mt := {
  __call: func(self, ...) {
    if self.next {
      return self.next(...)
    }
    return table.pack(...)
  }
}

tail := setmetatable({}, mt)
mid := setmetatable({next: tail}, mt)
head := setmetatable({next: mid}, mt)
out := head("a", nil, "c")
assert(out.n == 3)
assert(out[1] == "a" && out[2] == nil && out[3] == "c")

print("ok")
