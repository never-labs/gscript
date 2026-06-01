print("case:events_concat_numeric_operand_more")

mt := {}
calls := {}

mt.__concat = func(a, b) {
  av := a
  bv := b
  if type(a) == "table" { av = a.val }
  if type(b) == "table" { bv = b.val }
  calls[#calls + 1] = type(a) .. ":" .. tostring(av) .. "|" .. type(b) .. ":" .. tostring(bv)
  out := {val: tostring(av) .. tostring(bv)}
  setmetatable(out, mt)
  return out
}

t := {val: "T"}
setmetatable(t, mt)

r1 := t .. 7
assert(type(r1) == "table" && r1.val == "T7")
assert(calls[#calls] == "table:T|number:7")

r2 := 8 .. t
assert(type(r2) == "table" && r2.val == "8T")
assert(calls[#calls] == "number:8|table:T")

before := #calls
r3 := 1 .. t .. 2
assert(type(r3) == "table" && r3.val == "1T2")
assert(calls[before + 1] == "table:T|number:2")
assert(calls[before + 2] == "number:1|table:T2")

before = #calls
r4 := "a" .. t .. 3
assert(type(r4) == "table" && r4.val == "aT3")
assert(calls[before + 1] == "table:T|number:3")
assert(calls[before + 2] == "string:a|table:T3")

print("ok")
