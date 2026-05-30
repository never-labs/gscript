print("case:events_dynamic_call_metatable")

u = table.pack
for i = 1, 4 do
  u = setmetatable({i}, {__call = u})
end

packed = u("x", "y")
assert(packed.n == 6)
for i = 1, 4 do
  assert(packed[i][1] == i)
end
assert(packed[5] == "x" and packed[6] == "y")

mt = {
  __call = function(self, ...)
    if self.next then
      return self.next(...)
    end
    return table.pack(...)
  end
}

tail = setmetatable({}, mt)
mid = setmetatable({next = tail}, mt)
head = setmetatable({next = mid}, mt)
out = head("a", nil, "c")
assert(out.n == 3)
assert(out[1] == "a" and out[2] == nil and out[3] == "c")

print("ok")
