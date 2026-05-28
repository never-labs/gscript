// Smoke test and usage tour for the current structure-of-arrays stdlib.
// Run with:
//   go run ./cmd/gscript examples/data_oriented/soa_kernels.gs

func check(name, ok) {
    assert(ok)
    print("ok", name)
}

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

check("len", soa.len(points) == 4)
check("columns sorted", soa.columns(points)[1] == "id")
check("shape", soa.shape(points).columns[1].dtype == "i64")

row := soa.row(points, 2)
check("row copy", row.id == 102 && row.x == 2)
row.x = 99
check("row does not mutate", soa.column(points, "x")[2] == 2)

check("setRow", soa.setRow(points, 2, {
    id: 202,
    velocity: 20,
    vy: 2,
    x: 12,
    y: 7,
}))
check("setRow writes columns", soa.row(points, 2).x == 12 && soa.row(points, 2).id == 202)

check("addScaled", soa.addScaled(points, "x", "velocity", 0.5))
check("addScaled result", soa.column(points, "x")[1] == 6 && soa.column(points, "x")[2] == 22)

check("affine", soa.affine(points, "x", "velocity", 2, 1))
check("affine result", soa.column(points, "x")[3] == 61)
check("sum", soa.sum(points, "x") == 204)

check("affineMany", soa.affineMany(points, {
    {dst: "x", src: "velocity", scale: 0.25, bias: 0.5},
    {dst: "y", src: "vy", scale: 10, bias: 1},
}))
check("affineMany x", soa.column(points, "x")[4] == 10.5)
check("affineMany y", soa.column(points, "y")[4] == 41)

window := soa.slice(points, 2, 4)
check("slice inclusive", soa.len(window) == 3 && soa.column(window, "id")[1] == 202)

active := soa.filter(points, []bool{true, false, true, false})
check("filter", soa.len(active) == 2 && soa.column(active, "x")[2] == 8)

columns := soa.unzip(points)
columns.velocity[1] = 999
check("unzip copies", soa.column(points, "velocity")[1] == 10)

print("soa_kernels smoke passed")
