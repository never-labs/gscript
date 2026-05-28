x := []f64{1, 2, 3, 4}
velocity := []f64{10, 20, 30, 40}
y := []f64{0, 0, 0, 0}
vy := []f64{1, 2, 3, 4}
id := [4]i64{101, 102, 103, 104}

points := soa.zip({
    x: x,
    y: y,
    velocity: velocity,
    vy: vy,
    id: id,
})

print(soa.len(points))
print(soa.row(points, 2).id)
print(soa.shape(points).columns[1].dtype)

soa.addScaled(points, "x", "velocity", 0.5)
print(soa.column(points, "x"))

soa.affine(points, "x", "velocity", 2, 1)
print(soa.column(points, "x"))
print(soa.sum(points, "x"))

soa.affineMany(points, {
    {dst: "x", src: "velocity", scale: 0.25, bias: 0.5},
    {dst: "y", src: "vy", scale: 10, bias: 1},
})
print(soa.column(points, "x"))
print(soa.column(points, "y"))
