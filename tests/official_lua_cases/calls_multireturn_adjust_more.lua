print("case:calls_multireturn_adjust_more")

function triple()
  return 10, 20, 30
end

function down(n)
  if n <= 0 then return end
  return n, down(n - 1)
end

function count(...)
  return select("#", ...), ...
end

a, b, c = (triple())
assert(a == 10 and b == nil and c == nil)

n, x, y = count((triple()))
assert(n == 1 and x == 10 and y == nil)

unpacked = {1, 2, 3}
n2, u1, u2, u3, u4 = count(0, table.unpack(unpacked), 4)
assert(n2 == 3 and u1 == 0 and u2 == 1 and u3 == 4 and u4 == nil)

t1 = {(triple())}
assert(#t1 == 1 and t1[1] == 10)

t2 = {triple()}
assert(#t2 == 3 and t2[1] == 10 and t2[3] == 30)

t3 = {down(3), down(5), down(4)}
assert(#t3 == 6 and t3[1] == 3 and t3[2] == 5 and t3[3] == 4 and t3[6] == 1)

function sink(a, b, c, d, e, f)
  return table.pack(a, b, c, d, e, f)
end

function forward(prefix, ...)
  return sink(prefix, ...)
end

p = forward(99, 1, nil, 3)
assert(p.n == 6 and p[1] == 99 and p[2] == 1 and p[3] == nil and p[4] == 3 and p[5] == nil)

print("ok")
