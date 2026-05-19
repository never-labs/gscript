print("case:attrib_require_builtin_modules_more")

local s = require("string")
local m = require("math")
local t = require("table")

assert(s == string)
assert(m == math)
assert(t == table)
assert(require("string") == s)
assert(package.loaded.string == string)
assert(package.loaded.math == math)
assert(package.loaded.table == table)

print("ok")
