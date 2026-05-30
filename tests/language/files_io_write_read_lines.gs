print("case:files_io_write_read_lines")

file := os.tmpname()

f := io.open(file, "w")
assert(f)
assert(f:write("a line\n"))
assert(f:write("another line\n"))
assert(f:write("last"))
assert(f:close())

f = io.open(file, "r")
assert(f)
assert(f:read("l") == "a line")
assert(f:read("l") == "another line")
assert(f:read("l") == "last")
assert(f:read("l") == nil)
assert(f:close())

n := 0
for l := range io.lines(file) {
  n = n + 1
}
assert(n == 3)

assert(os.remove(file))

print("ok")
