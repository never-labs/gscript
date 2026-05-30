print("case:events_concat_numeric_operand_more")

local mt = {}
local calls = {}

mt.__concat = function(a, b)
  local av = a
  local bv = b
  if type(a) == "table" then av = a.val end
  if type(b) == "table" then bv = b.val end
  calls[#calls + 1] = type(a) .. ":" .. tostring(av) .. "|" .. type(b) .. ":" .. tostring(bv)
  local out = {val = tostring(av) .. tostring(bv)}
  setmetatable(out, mt)
  return out
end

local t = {val = "T"}
setmetatable(t, mt)

local r1 = t .. 7
assert(type(r1) == "table" and r1.val == "T7")
assert(calls[#calls] == "table:T|number:7")

local r2 = 8 .. t
assert(type(r2) == "table" and r2.val == "8T")
assert(calls[#calls] == "number:8|table:T")

local before = #calls
local r3 = 1 .. t .. 2
assert(type(r3) == "table" and r3.val == "1T2")
assert(calls[before + 1] == "table:T|number:2")
assert(calls[before + 2] == "number:1|table:T2")

before = #calls
local r4 = "a" .. t .. 3
assert(type(r4) == "table" and r4.val == "aT3")
assert(calls[before + 1] == "table:T|number:3")
assert(calls[before + 2] == "string:a|table:T3")

print("ok")
