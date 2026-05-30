print("case:files_io_read_numbers")

local file = os.tmpname()

local f = assert(io.open(file, "w"))
assert(f:write("1234\n"))
assert(f:write("3.45\n"))
assert(f:write("-17\n"))
assert(f:write("not-a-number\n"))
assert(f:close())

f = assert(io.open(file, "r"))
assert(f:read("n") == 1234)
assert(f:read("n") == 3.45)
assert(f:read("*n") == -17)
assert(f:read("n") == nil)
assert(f:close())

assert(os.remove(file))

print("ok")
