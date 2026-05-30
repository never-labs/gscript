print("case:strings_format_pointer")

local null = "(null)"

assert(string.format("%p", 4) == null)
assert(string.format("%p", true) == null)
assert(string.format("%p", nil) == null)
assert(string.format("%p", {}) ~= null)
assert(string.format("%p", print) ~= null)
assert(string.format("%p", print) == string.format("%p", print))
assert(string.format("%p", print) ~= string.format("%p", assert))

assert(#string.format("%90p", {}) == 90)
assert(#string.format("%-60p", {}) == 60)
assert(string.format("%10p", false) == string.rep(" ", 10 - #null) .. null)
assert(string.format("%-12p", 1.5) == null .. string.rep(" ", 12 - #null))

local t1 = {}
local t2 = {}
assert(string.format("%p", t1) ~= string.format("%p", t2))

print("ok")
