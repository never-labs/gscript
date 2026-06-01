print("case:io_current_stream_more")

out_path := os.tmpname()
out_file_path := os.tmpname()
input_path := os.tmpname()
current_lines_path := os.tmpname()
lines_path := os.tmpname()

f := io.open(input_path, "w")
assert(f)
assert(f:write("one\ntwo\nthree"))
assert(f:close())

f = io.open(current_lines_path, "w")
assert(f)
assert(f:write("alpha\nbeta\ngamma\n"))
assert(f:close())

f = io.open(lines_path, "w")
assert(f)
assert(f:write("red\ngreen\nblue\n"))
assert(f:close())

old_out := io.output()
path_out := io.output(out_path)
assert(path_out)
assert(io.type(path_out) == "file")
io.write("path", "-", 12, "\n")
assert(io.flush())
assert(io.output(old_out) == old_out)
assert(path_out:close())

f = io.open(out_path, "r")
assert(f)
assert(f:read("a") == "path-12\n")
assert(f:close())

explicit_out := io.open(out_file_path, "w")
assert(explicit_out)
old_out = io.output()
assert(io.output(explicit_out) == explicit_out)
io.write("file", "-", "handle", "\n")
assert(io.flush())
assert(io.output(old_out) == old_out)
assert(explicit_out:close())

f = io.open(out_file_path, "r")
assert(f)
assert(f:read("a") == "file-handle\n")
assert(f:close())

old_in := io.input()
path_in := io.input(input_path)
assert(path_in)
assert(io.type(path_in) == "file")
a, b, c := io.read("l", "L", "a")
assert(a == "one")
assert(b == "two\n")
assert(c == "three")
assert(io.input(old_in) == old_in)
assert(path_in:close())

explicit_in := io.open(current_lines_path, "r")
assert(explicit_in)
old_in = io.input()
assert(io.input(explicit_in) == explicit_in)
assert(io.read() == "alpha")
seen := {}
for line := range io.lines() {
  seen[#seen + 1] = line
}
assert(#seen == 2 && seen[1] == "beta" && seen[2] == "gamma")
assert(io.input(old_in) == old_in)
assert(explicit_in:close())

n := 0
last := ""
for line := range io.lines(lines_path) {
  n = n + 1
  last = line
}
assert(n == 3 && last == "blue")

closed := io.tmpfile()
assert(closed)
assert(io.type(closed) == "file")
assert(closed:close())
assert(io.type(closed) == "closed file")
assert(io.type({}) == nil)

bad_out, bad_out_err := io.output(closed)
assert(bad_out == nil && type(bad_out_err) == "string")
bad_in, bad_in_err := io.input(closed)
assert(bad_in == nil && type(bad_in_err) == "string")

assert(os.remove(out_path))
assert(os.remove(out_file_path))
assert(os.remove(input_path))
assert(os.remove(current_lines_path))
assert(os.remove(lines_path))

print("ok")
