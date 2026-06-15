# Leia Data-Oriented Programming

Leia includes data-oriented standard libraries for numeric scripts that need
columnar layout, typed dense arrays, vectors, matrices, masks, and hot-loop
kernels. The `q` dialect is the core high-performance in-memory columnar
analytics DSL in this area. This is a language feature area, not only a library
convenience: runtime and JIT paths can specialize these shapes while preserving
normal script semantics.

## Dense Arrays

Dense arrays store one primitive element kind and are the preferred input for
SoA and numeric kernels.

```leia
x := []f64{1, 2, 3, 4}
id := [4]i64{101, 102, 103, 104}
mask := []bool{true, false, true, false}
```

Common element families are floating point, signed integers, and booleans.
Dense arrays use normal indexing syntax:

```leia
x[1] = x[1] + 0.5
```

## Structure Of Arrays

`soa.zip` builds a columnar value from named dense arrays:

```leia
points := soa.zip({
    x: []f64{1, 2, 3, 4},
    y: []f64{0, 0, 0, 0},
    velocity: []f64{10, 20, 30, 40},
    id: [4]i64{101, 102, 103, 104},
})
```

Columns must have compatible lengths. Columns are addressable by helper or
field syntax:

```leia
points.x[1] = 11
vx := soa.column(points, "velocity")
```

Rows are materialized as ordinary tables when requested:

```leia
row := soa.row(points, 2)
row.x = 12
soa.setRow(points, 2, row)
```

Prefer column kernels for hot paths. Row materialization is useful at API
boundaries but usually less efficient.

## Database Frames And Data Projects

The built-in `db` module can bridge host-facing SQLite workflows into the same
columnar data path. `db.frame(sql, params)` returns a result object with normal
row tables for compatibility and column-oriented fields for analysis:

```leia
conn := db.open(":memory:")
conn.exec("create table sales(channel_id integer, amount real)")
conn.exec("insert into sales values (?, ?)", [1, 120])

frame := conn.frame("select channel_id, amount from sales", {})
total := q.query(frame.soa, {
    by: ["channel_id"]
    select: {amount: "amount"}
    aggregate: {amount: "sum"}
    order_by: {column: "amount", desc: true}
})
```

Use row data when crossing API boundaries, and use `frame.soa` / numeric
columns when feeding `q.query` or SoA kernels. Spreadsheet dialects complete the
round trip:

```leia
workbook := dialect.eval("xlsx", total, {
    mode: "encode"
    headers: ["channel_id", "amount"]
    sheet: "summary"
})
roundtrip := dialect.eval("excel", workbook, {headers: true})
```

The runnable project `examples/data/db_q_frame_project` exercises SQLite
`db.frame`, SoA-backed `q.query`, and `xlsx`/`excel` import/export together. It
keeps published data-oriented claims tied to an executable example. Larger
tooling workflows use the same bridge alongside shell, optional LLM, and web
dialects; string categories are mapped to stable numeric ids before entering the
SoA/q aggregation path.

## Shape And Schema

| API | Purpose |
|---|---|
| `soa.len(s)` | Row count. |
| `soa.columns(s)` | Sorted column names. |
| `soa.shape(s)` | Column metadata: names, dtype, length, and version. |
| `soa.column(s, name)` | Dense column or `nil`. |
| `soa.withColumn(s, name, col)` | Return a layout with one additional column. |
| `soa.dropColumn(s, name)` | Return a layout without one column. |
| `soa.unzip(s)` | Copy columns into an ordinary table. |

## Mutation

| API | Purpose |
|---|---|
| `soa.resize(s, n)` | Grow or shrink all columns together. |
| `soa.appendRow(s, row)` | Append one table row. |
| `soa.fill(s, column, value)` | Fill one column. |
| `soa.fillWhere(s, column, mask, value)` | Fill masked rows. |
| `soa.scatterInto(s, column, indices, valueOrDense)` | Scatter scalar or dense values into indexed rows. |

Mutation updates internal version metadata so cached masks and reductions can
stay correct.

## Masks And Selection

Masks are dense boolean arrays. Use them to keep filtering and aggregation on
columns:

```leia
fast := soa.mask(points, "velocity", ">=", 20)
selected := soa.select(points, fast, "velocity", 0)
soa.selectInto(points, "y", fast, "velocity", 0)
sum := soa.sumSelect(points, fast, "velocity", 0)
```

Comparison operators for `soa.mask` are string values such as `">="`, `"<"`,
and `"=="`. The right-hand side may be a scalar or another column name.

| API | Purpose |
|---|---|
| `soa.mask(s, column, op, rhs)` | Build a boolean mask. |
| `soa.countWhere(s, mask)` | Count true rows. |
| `soa.indicesWhere(s, mask)` | Return 1-based row indices. |
| `soa.filter(s, mask)` / `soa.compact(s, mask)` | Return a compacted SoA. |
| `soa.gather(s, indices)` | Gather rows in explicit order, preserving duplicates. |
| `soa.select(s, mask, ifTrue, ifFalse)` | Dense select over scalar or column operands. |
| `soa.selectInto(s, dst, mask, ifTrue, ifFalse)` | Write dense select into a column. |

## Numeric Kernels

| API | Purpose |
|---|---|
| `soa.addScaled(s, dst, src, scale)` | `dst += src * scale`. |
| `soa.affine(s, dst, src, scale, bias)` | `dst = src * scale + bias`. |
| `soa.affineWhere(s, dst, src, scale, mask, bias)` | Masked affine update. |
| `soa.affineMany(s, terms)` | Batch multiple affine updates. |
| `soa.sum/min/max/mean(s, column)` | Column reductions. |
| `soa.stats(s, column)` | Count, sum, min, max, and mean. |
| `soa.sumWhere/minWhere/maxWhere/meanWhere(s, column, mask)` | Masked reductions. |
| `soa.statsWhere(s, column, mask)` | Masked stats. |
| `soa.scan(s, column)` | Prefix scan into a new dense array. |
| `soa.scanInto(s, dst, src)` | Prefix scan into a column. |
| `soa.clamp(s, column, lo, hi)` | New clamped dense array. |
| `soa.clampInto(s, dst, src, lo, hi)` | Clamped column write. |
| `soa.dot(s, a, b)` | Dot product. |
| `soa.dotWhere(s, a, b, mask)` | Masked dot product. |

`affineMany` is the preferred spelling when a loop performs several independent
column updates; it gives the runtime a larger shape to specialize.

## Slicing

`soa.slice(s, first, last)` returns an inclusive 1-based row window.

```leia
window := soa.slice(points, 2, 4)
```

## Performance Model

Use SoA when most work touches the same fields across many rows. Avoid
converting every row into a table inside hot loops. Column kernels are designed
to reduce dynamic dispatch, table lookups, and temporary allocation.

The relevant benchmark selectors are under `benchmarks/data/`, including
`data/soa_affine_many`, `data/soa_masked_aggregate`, `data/soa_scan`,
`data/soa_scatter`, and `data/soa_select`.

Run:

```bash
leia bench --bench data/soa_affine_many --runs 5 --warmup 1
leia bench --quick --bench data
```

## Related Modules

- `matrix` for dense matrix-oriented operations;
- `vec` and `color` for compact geometry-style values;
- `array` for dense-array construction and manipulation;
- `math`, `bits`, and `bit32` for scalar numeric support.

The generated [standard-library reference](../stdlib/index.md) lists available
modules and capability layers.
