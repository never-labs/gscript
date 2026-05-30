print("case:files_append_read_all_more")

file := os.tmpname()

f := io.open(file, "w")
assert(f)
assert(f:write("first\n"))
assert(f:close())

f = io.open(file, "a")
assert(f)
assert(f:write("second\n"))
assert(f:close())

f = io.open(file, "r")
assert(f)
all := f:read("*a")
assert(all == "first\nsecond\n")
assert(f:close())

count := 0
for line := range io.lines(file) {
  count = count + 1
  if count == 1 {
    assert(line == "first")
  }
  if count == 2 {
    assert(line == "second")
  }
}
assert(count == 2)

assert(os.remove(file))

print("ok")
