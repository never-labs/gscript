print("case:files_io_write_read_lines")

local file = os.tmpname()

local f = assert(io.open(file, "w"))
assert(f:write("a line\n"))
assert(f:write("another line\n"))
assert(f:write("last"))
assert(f:close())

f = assert(io.open(file, "r"))
assert(f:read("l") == "a line")
assert(f:read("l") == "another line")
assert(f:read("l") == "last")
assert(f:read("l") == nil)
assert(f:close())

local n = 0
for l in io.lines(file) do
  n = n + 1
end
assert(n == 3)

assert(os.remove(file))

print("ok")
