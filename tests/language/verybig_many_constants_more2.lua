print("case:verybig_many_constants_more2")

local constants = {
  1, 2, 3, 4, 5, 6, 7, 8,
  9, 10, 11, 12, 13, 14, 15, 16,
  17, 18, 19, 20, 21, 22, 23, 24,
  25, 26, 27, 28, 29, 30, 31, 32,
}

for i = 33, 384 do
  constants[#constants + 1] = i
end

local function check(k)
  return constants[k] + constants[k + 127] + constants[k + 255]
end

assert(#constants == 384)
assert(check(1) == 385)
assert(check(64) == 574)
assert(constants[128] == 128 and constants[256] == 256 and constants[384] == 384)

print("ok")
