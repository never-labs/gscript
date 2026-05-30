print("case:bitwise_bit32_wrap_more")

local bit32 = bit32 or {}
bit32.band = bit32.band or function(...)
  local r = 0xffffffff
  for i = 1, select("#", ...) do
    r = r & select(i, ...)
  end
  return r & 0xffffffff
end

assert(bit32.band(-1) == 0xffffffff)
assert(bit32.band((2^33) - 1) == 0xffffffff)
assert(bit32.band(-(2^33) - 1) == 0xffffffff)
assert(bit32.band((2^33) + 1) == 1)
assert(bit32.band(-(2^33) + 1) == 1)
assert(bit32.band(-(2^40)) == 0)
assert(bit32.band(2^40) == 0)
assert(bit32.band(-(2^40) - 2) == 0xfffffffe)
assert(bit32.band((2^40) - 4) == 0xfffffffc)

print("ok")
