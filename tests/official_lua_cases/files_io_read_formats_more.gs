print("case:files_io_read_formats_more")

file := os.tmpname()

f := io.open(file, "w")
assert(f)
assert(f:write("one\nlast"))
assert(f:close())

f = io.open(file, "r")
assert(f)
assert(f:read("L") == "one\n")
assert(f:read("*L") == "last")
assert(f:read("L") == nil)
assert(f:close())

f = io.open(file, "w")
assert(f)
assert(f:write("abcdef\nxyz"))
assert(f:close())

f = io.open(file, "r")
assert(f)
assert(f:read(2) == "ab")
assert(f:read(0) == "")
assert(f:read(4) == "cdef")
assert(f:read(10) == "\nxyz")
assert(f:read(1) == nil)
assert(f:close())

f = io.open(file, "w")
assert(f)
assert(f:write("first\nsecond\nrest"))
assert(f:close())

f = io.open(file, "r")
assert(f)
a, b, c := f:read("L", "l", 4)
assert(a == "first\n")
assert(b == "second")
assert(c == "rest")
assert(f:close())

assert(os.remove(file))

print("ok")
