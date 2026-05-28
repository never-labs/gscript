x := []f64{1, 2, 3, 4}
velocity := []f64{10, 20, 30, 40}
id := [4]i64{101, 102, 103, 104}

points := soa.zip({
    x: x,
    velocity: velocity,
    id: id,
})

print(soa.len(points))
print(soa.row(points, 2).id)

soa.addScaled(points, "x", "velocity", 0.5)
print(soa.column(points, "x"))

soa.affine(points, "x", "velocity", 2, 1)
print(soa.column(points, "x"))
print(soa.sum(points, "x"))
