print("case:matrix_host_dense_more")

m := matrix.dense(2, 3)
matrix.setf(m, 0, 0, 1.25)
matrix.setf(m, 0, 1, 2)
matrix.setf(m, 1, 2, 7.5)

assert(matrix.getf(m, 0, 0) == 1.25)
assert(matrix.getf(m, 0, 1) == 2)
assert(matrix.getf(m, 1, 2) == 7.5)
assert(m[0][0] == 1.25)
assert(m[1][2] == 7.5)

ok, err := pcall(matrix.dense, -1, 2)
assert(ok == false)
assert(string.find(err, "non-negative", 1, true) != nil)

ok, err = pcall(matrix.getf, m, 0.5, 1)
assert(ok == false)
assert(string.find(err, "integers", 1, true) != nil)

ok, err = pcall(matrix.getf, m, 9, 0)
assert(ok == false)
assert(string.find(err, "out of range", 1, true) != nil)

ok, err = pcall(matrix.setf, m, 0, 0, "x")
assert(ok == false)
assert(string.find(err, "numeric", 1, true) != nil)

print("ok")
