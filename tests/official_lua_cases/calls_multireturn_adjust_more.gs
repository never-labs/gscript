print("case:calls_multireturn_adjust_more")

func triple() {
  return 10, 20, 30
}

func down(n) {
  if n <= 0 { return }
  return n, down(n - 1)
}

func count(...) {
  return select("#", ...), ...
}

a, b, c := (triple())
assert(a == 10 && b == nil && c == nil)

n, x, y := count((triple()))
assert(n == 1 && x == 10 && y == nil)

unpacked := {1, 2, 3}
n2, u1, u2, u3, u4 := count(0, table.unpack(unpacked), 4)
assert(n2 == 3 && u1 == 0 && u2 == 1 && u3 == 4 && u4 == nil)

t1 := {(triple())}
assert(#t1 == 1 && t1[1] == 10)

t2 := {triple()}
assert(#t2 == 3 && t2[1] == 10 && t2[3] == 30)

t3 := {down(3), down(5), down(4)}
assert(#t3 == 6 && t3[1] == 3 && t3[2] == 5 && t3[3] == 4 && t3[6] == 1)

func sink(a, b, c, d, e, f) {
  return table.pack(a, b, c, d, e, f)
}

func forward(prefix, ...) {
  return sink(prefix, ...)
}

p := forward(99, 1, nil, 3)
assert(p.n == 6 && p[1] == 99 && p[2] == 1 && p[3] == nil && p[4] == 3 && p[5] == nil)

print("ok")
