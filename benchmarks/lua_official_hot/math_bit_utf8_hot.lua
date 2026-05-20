print("official_hot/math_bit_utf8_hot")

local bit32 = bit32

if not bit32 then
  local ok, bit = pcall(require, "bit")
  if ok and bit then
    local function u32(x)
      x = bit.tobit(x)
      if x < 0 then return x + 4294967296 end
      return x
    end

    bit32 = {
      band = function(...)
        local n = select("#", ...)
        if n == 0 then return 0xffffffff end
        local r = select(1, ...)
        for i = 2, n do r = bit.band(r, (select(i, ...))) end
        return u32(r)
      end,
      bor = function(...)
        local n = select("#", ...)
        if n == 0 then return 0 end
        local r = select(1, ...)
        for i = 2, n do r = bit.bor(r, (select(i, ...))) end
        return u32(r)
      end,
      bxor = function(...)
        local r = 0
        for i = 1, select("#", ...) do r = bit.bxor(r, (select(i, ...))) end
        return u32(r)
      end,
      lshift = function(n, d)
        if d < 0 then return u32(bit.rshift(n, -d)) end
        if d >= 32 then return 0 end
        return u32(bit.lshift(n, d))
      end,
      rshift = function(n, d)
        if d < 0 then
          if -d >= 32 then return 0 end
          return u32(bit.lshift(n, -d))
        end
        if d >= 32 then return 0 end
        return u32(bit.rshift(n, d))
      end,
      arshift = function(n, d)
        if d < 0 then
          if -d >= 32 then return 0 end
          return u32(bit.lshift(n, -d))
        end
        if d >= 32 then d = 31 end
        return u32(bit.arshift(n, d))
      end,
      lrotate = function(n, d)
        d = d % 32
        return u32(bit.rol(n, d))
      end,
    }

    function bit32.rrotate(n, d)
      return bit32.lrotate(n, -d)
    end

    function bit32.extract(n, field, width)
      width = width or 1
      if field < 0 or field >= 32 or width <= 0 or width > 32 - field then error("bad field or width") end
      return bit32.band(bit32.rshift(n, field), bit32.rshift(0xffffffff, 32 - width))
    end

    function bit32.replace(n, v, field, width)
      width = width or 1
      if field < 0 or field >= 32 or width <= 0 or width > 32 - field then error("bad field or width") end
      local mask = bit32.lshift(bit32.rshift(0xffffffff, 32 - width), field)
      return bit32.bor(bit32.band(n, bit.bnot(mask)), bit32.band(bit32.lshift(v, field), mask))
    end
  else
    local loader = load or loadstring
    bit32 = assert(loader([[
      local bit32 = {}
      bit32.band = function(...)
        local n = select("#", ...)
        if n == 0 then return 0xffffffff end
        local r = select(1, ...) & 0xffffffff
        for i = 2, n do r = r & select(i, ...) end
        return r & 0xffffffff
      end
      bit32.bor = function(...)
        local n = select("#", ...)
        if n == 0 then return 0 end
        local r = select(1, ...) & 0xffffffff
        for i = 2, n do r = r | select(i, ...) end
        return r & 0xffffffff
      end
      bit32.bxor = function(...)
        local r = 0
        for i = 1, select("#", ...) do r = r ~ select(i, ...) end
        return r & 0xffffffff
      end
      bit32.lshift = function(n, d)
        n = n & 0xffffffff
        if d < 0 then return (n >> -d) & 0xffffffff end
        if d >= 32 then return 0 end
        return (n << d) & 0xffffffff
      end
      bit32.rshift = function(n, d)
        n = n & 0xffffffff
        if d < 0 then return bit32.lshift(n, -d) end
        if d >= 32 then return 0 end
        return (n >> d) & 0xffffffff
      end
      bit32.arshift = function(n, d)
        n = n & 0xffffffff
        if d < 0 then return bit32.lshift(n, -d) end
        if d >= 32 then return (n & 0x80000000) ~= 0 and 0xffffffff or 0 end
        if (n & 0x80000000) ~= 0 then return ((n >> d) | (0xffffffff << (32 - d))) & 0xffffffff end
        return (n >> d) & 0xffffffff
      end
      bit32.lrotate = function(n, d)
        n = n & 0xffffffff
        d = d & 31
        return ((n << d) | (n >> (32 - d))) & 0xffffffff
      end
      bit32.rrotate = function(n, d) return bit32.lrotate(n, -d) end
      bit32.extract = function(n, field, width)
        width = width or 1
        if field < 0 or field >= 32 or width <= 0 or width > 32 - field then error("bad field or width") end
        return (n >> field) & ((1 << width) - 1)
      end
      bit32.replace = function(n, v, field, width)
        width = width or 1
        if field < 0 or field >= 32 or width <= 0 or width > 32 - field then error("bad field or width") end
        local mask = ((1 << width) - 1) << field
        return ((n & ~mask) | ((v << field) & mask)) & 0xffffffff
      end
      return bit32
    ]]))()
  end
end

local utf8 = utf8 or {}

if not utf8.char then
  function utf8.char(...)
    local out = {}
    for i = 1, select("#", ...) do
      local cp = select(i, ...)
      if cp < 0x80 then
        out[#out + 1] = string.char(cp)
      elseif cp < 0x800 then
        out[#out + 1] = string.char(0xc0 + math.floor(cp / 0x40), 0x80 + (cp % 0x40))
      elseif cp < 0x10000 then
        out[#out + 1] = string.char(0xe0 + math.floor(cp / 0x1000), 0x80 + (math.floor(cp / 0x40) % 0x40), 0x80 + (cp % 0x40))
      else
        out[#out + 1] = string.char(0xf0 + math.floor(cp / 0x40000), 0x80 + (math.floor(cp / 0x1000) % 0x40), 0x80 + (math.floor(cp / 0x40) % 0x40), 0x80 + (cp % 0x40))
      end
    end
    return table.concat(out)
  end

  local function decode_at(s, i)
    local b1 = string.byte(s, i)
    if not b1 then return nil, i end
    if b1 < 0x80 then return b1, i + 1 end
    if b1 < 0xe0 then
      local b2 = string.byte(s, i + 1)
      return (b1 - 0xc0) * 0x40 + (b2 - 0x80), i + 2
    end
    if b1 < 0xf0 then
      local b2, b3 = string.byte(s, i + 1, i + 2)
      return (b1 - 0xe0) * 0x1000 + (b2 - 0x80) * 0x40 + (b3 - 0x80), i + 3
    end
    local b2, b3, b4 = string.byte(s, i + 1, i + 3)
    return (b1 - 0xf0) * 0x40000 + (b2 - 0x80) * 0x1000 + (b3 - 0x80) * 0x40 + (b4 - 0x80), i + 4
  end

  function utf8.codes(s)
    local next_pos = 1
    return function()
      if next_pos > #s then return nil end
      local pos = next_pos
      local cp
      cp, next_pos = decode_at(s, next_pos)
      return pos, cp
    end
  end

  function utf8.len(s)
    local n = 0
    for _ in utf8.codes(s) do n = n + 1 end
    return n
  end

  function utf8.codepoint(s, i)
    local cp = decode_at(s, i or 1)
    return cp
  end
end

local N = 180000
local MOD = 1000000007
local MASK32 = 4294967295

local inputs = {"101010", "755", "1f", "Z", "12345", "-1011", "+1Z", "777777"}
local bases = {2, 8, 16, 36, 10, 2, 36, 8}
local text = "Az" .. utf8.char(0x4e2d) .. utf8.char(0x03bb) .. utf8.char(0x1f600)
local textLen = utf8.len(text)
local textOffsets = {1, 2, 3, 6, 8}
local tointeger = math.tointeger or function(n)
  if n == math.floor(n) then return n end
  return nil
end

local function toint(x)
  local n = tonumber(x)
  if not n then return false end
  return tointeger(n) or false
end

local function bench(n)
  local checksum = 0
  local rolling = 305419896
  local inputCount = #inputs

  for i = 1, n do
    local idx = (i % inputCount) + 1
    local parsed = tonumber(inputs[idx], bases[idx])
    if not parsed then parsed = 0 end

    local folded = math.floor(math.fmod(parsed * 13 + i * 17, 1048573))
    local floored = math.floor((i * 97.0) / 11.0)
    local fmodded = math.floor(math.fmod(floored + folded, 251))
    local modulo = (i * 31 + fmodded) % 65521

    local shift = (i % 63) - 31
    local left = bit32.lshift(folded, shift)
    local right = bit32.rshift(rolling, -shift)
    local arith = bit32.arshift(rolling, shift)
    local rot = bit32.bxor(bit32.lrotate(rolling, shift), bit32.rrotate(folded, shift))
    local field = i % 24
    local width = (i % 8) + 1
    local extracted = bit32.extract(rot, field, width)
    local replaced = bit32.replace(rolling, extracted, i % 16, 4)

    local xi = toint(tostring(i % 256))
    local yi = toint(tostring((i * 3) % 1024))
    local coerced = bit32.bxor(bit32.bor(xi, yi), bit32.band(replaced, 65535))

    local cpSum = 0
    for pos, cp in utf8.codes(text) do
      cpSum = cpSum + cp + pos
    end
    local cpAt = utf8.codepoint(text, textOffsets[((i + cpSum) % textLen) + 1])

    rolling = bit32.band(bit32.bxor(replaced, coerced, cpSum, cpAt, left, right, arith, modulo), MASK32)
    checksum = (checksum + (rolling % MOD) + extracted + textLen + fmodded + modulo) % MOD
  end

  return checksum
end

local t0 = os.clock()
local checksum = bench(N)
local elapsed = os.clock() - t0

print(string.format("checksum: %d", checksum))
print(string.format("Time: %.3fs", elapsed))
