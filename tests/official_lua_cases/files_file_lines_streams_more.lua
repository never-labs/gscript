print("case:files_file_lines_streams_more")

local file = os.tmpname()
local f = assert(io.open(file, "w"))
assert(f:write("alpha\nbeta\ngamma\n"))
assert(f:close())

f = assert(io.open(file, "r"))
local iter = f:lines()
local a = iter()
local b = iter()
local c = iter()
local d = iter()
assert(a == "alpha" and b == "beta" and c == "gamma" and d == nil)
assert(io.type(f) == "file")
assert(f:close())
assert(io.type(f) == "closed file")

assert(io.type(io.stdin) == "file")
assert(io.type(io.stdout) == "file")
assert(io.type(io.stderr) == "file")

os.remove(file)

print("ok")
