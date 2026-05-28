# soa

The `soa` library stores a logical sequence of records as named dense columns.
It is intended for data-oriented code that wants columnar access without
manually keeping parallel arrays aligned.

SoA values are built from dense arrays such as `[]f64{...}`, `[]bool{...}`, and
`[N]i64{...}`. All columns must have the same length.

```gscript
points := soa.zip({
    x: []f64{1, 2, 3},
    y: []f64{4, 5, 6},
    id: [3]i64{101, 102, 103},
})

print(soa.len(points))        // 3
print(soa.row(points, 2).id)  // 102
```

## Layout and Indexing

- Row indexes are one-based, matching GScript table conventions.
- `soa.slice(s, first, last)` uses one-based inclusive bounds. For example,
  `soa.slice(s, 2, 4)` returns rows 2, 3, and 4.
- Column names are strings. `soa.columns` and `soa.shape().columns` report them
  in sorted order.
- `soa.column` returns the live dense column. Mutating the returned dense array
  mutates the SoA.
- `soa.row` returns a copied ordinary table. Mutating that table does not write
  back; use `soa.setRow` for row-style mutation.
- `soa.slice`, `soa.filter`, `soa.gather`, and `soa.unzip` return independent
  dense-column copies.
- `soa.mask` builds a bool dense array from a column comparison. The mask can
  feed `soa.filter`, `soa.compact`, `soa.affineWhere`, and `*Where` reducers.
- `soa.indicesWhere` converts a bool mask into a one-based `[]i64` index vector
  for gather/scatter pipelines.
- `soa.sumWhere` reduces a numeric column over a bool mask without
  materializing a filtered SoA first.

## Introspection

### `soa.zip(columns) -> soa`

Builds a SoA from a plain string-keyed table of dense arrays.

Errors if `columns` is not a table, any key is not a string, any value is not a
dense array, the table is empty, a name is empty, or column lengths differ.

### `soa.unzip(s) -> table`

Returns a table mapping each column name to a cloned dense array.

### `soa.len(s) -> integer`

Returns the row count.

### `soa.columns(s) -> table`

Returns a one-based table of column names in sorted order.

### `soa.column(s, name) -> dense array | nil`

Returns the live dense column named `name`, or `nil` if the column does not
exist.

### `soa.withColumn(s, name, column) -> soa`

Returns a new SoA layout containing all current columns plus `column` under
`name`. The new column must be a dense array with the same length as `s`.

Existing columns and the supplied column are kept by reference, matching
`soa.zip`; use `soa.unzip` first when independent column copies are required.

### `soa.dropColumn(s, name) -> soa`

Returns a new SoA layout without `name`. At least one column must remain.

Remaining columns are kept by reference, matching `soa.zip`.

### `soa.resize(s, length) -> true`

Resizes every column in place and updates the SoA row count. Growing fills new
slots with the element type's zero value (`0`, `0.0`, or `false`); shrinking
truncates all columns together.

### `soa.appendRow(s, row) -> true`

Appends one row in place. `row` must be a table containing a non-nil field for
every column, and each value must match the column element type.

### `soa.fill(s, column, value) -> true`

Fills one column in place with `value`. The value must match the column element
type: numeric for `f64`, integer for `i64`, and boolean for `bool`.

### `soa.fillWhere(s, column, mask, value) -> true`

Fills only mask-true rows in one column. `mask` must be a bool dense array with
the same length as `s`.

### `soa.shape(s) -> table`

Returns layout diagnostics:

```gscript
shape := soa.shape(points)
print(shape.length)
print(shape.version)
print(shape.columns[1].name)
print(shape.columns[1].dtype)
print(shape.columns[1].length)
print(shape.columns[1].version)
```

`shape.version` identifies the SoA layout. Each column also has a version used
by guarded runtime specializations to validate that the dense column they saw is
still current.

## Rows and Subsets

### `soa.row(s, index) -> table`

Returns a copied row table with one field per column. It is best for boundary
code, debugging, and tests. It is not the hot-path access pattern.

### `soa.setRow(s, index, row) -> true`

Writes every SoA column from fields in `row`. The row table must contain a
non-nil value for every column, and each value must be accepted by that dense
column's element kind.

### `soa.slice(s, first, last) -> soa`

Returns an independent SoA containing rows `first..last`, inclusive.

### `soa.filter(s, mask) -> soa`

Returns an independent SoA containing rows whose corresponding mask element is
`true`. `mask` must be a bool dense array with the same length as `s`.

Use `soa.filter` as the current compact/filter operation: build a bool dense
mask from a predicate, compact all columns together, then continue with column
kernels on the returned SoA.

```gscript
active := soa.filter(points, []bool{true, false, true})
activeX := soa.sum(active, "x")
```

### `soa.gather(s, indices) -> soa`

Returns an independent SoA containing rows addressed by an i64 dense index
vector. Indexes are one-based, matching `soa.row` and `soa.slice`. Duplicate
indexes preserve duplicate rows, and output order matches the index vector.

Use gather when the selection is already represented as positions rather than
a predicate mask: sparse lookup, permutation, join output, stable top-K, or
replaying an externally computed order.

```gscript
picked := soa.gather(points, [3]i64{3, 1, 3})
```

### `soa.indicesWhere(s, mask) -> []i64`

Returns the one-based indexes where `mask` is true. The mask length must match
the SoA length.

```gscript
moving := soa.mask(points, "velocity", ">", 0)
movingRows := soa.indicesWhere(points, moving)
picked := soa.gather(points, movingRows)
```

### `soa.scatterInto(s, column, indices, values) -> true`

Writes `values` into `column` at one-based `indices`. `values` can be a scalar
or a dense array with the same length as `indices`; duplicate indexes use
last-write-wins order.

```gscript
rows := soa.indicesWhere(points, moving)
soa.scatterInto(points, "visible", rows, true)
soa.scatterInto(points, "score", rows, []f64{1.0, 2.0, 3.0})
```

### `soa.compact(s, mask) -> soa`

Alias of `soa.filter` for callers that want the array-programming term
"compact" after computing a bool mask.

### `soa.mask(s, column, op, rhs) -> []bool`

Returns a bool dense array produced by comparing `column` with `rhs`.

`op` accepts symbolic operators (`==`, `!=`, `<`, `<=`, `>`, `>=`) and word
aliases (`eq`, `ne`, `lt`, `le`, `gt`, `ge`). `rhs` may be a numeric or bool
scalar, or a string naming another column in the same SoA.

```gscript
moving := soa.mask(points, "velocity", ">", 0)
ahead := soa.mask(points, "x", ">=", "target_x")
active := soa.compact(points, moving)
```

### `soa.select(s, mask, if_true, if_false) -> dense array`

Returns a dense array by selecting one value per row from `if_true` when
`mask[i]` is true, otherwise from `if_false`.

`if_true` and `if_false` may be numeric or bool scalars, or strings naming
columns in `s`. Numeric selections return `i64` when both sides are integer,
otherwise `f64`; bool selections require both sides to be bool.

```gscript
moving := soa.mask(points, "velocity", ">", 0)
signed_speed := soa.select(points, moving, "velocity", 0)
visible := soa.select(points, moving, true, false)
```

### `soa.selectInto(s, dst, mask, if_true, if_false) -> true`

Writes the same selection result into an existing destination column. Use this
form in hot loops when the output shape is stable and a reusable scratch column
can avoid allocating a new dense array every iteration.

### `soa.sumSelect(s, mask, if_true, if_false) -> number`

Fuses `soa.select` with a numeric sum reduction. Use this when the selected
temporary is only needed for an aggregate:

```gscript
total := soa.sumSelect(points, moving, "velocity", 0)
```

## Fused Column Kernels

The current native SoA kernels operate over numeric dense columns and mutate the
destination column in place.

### `soa.addScaled(s, dst, src, scale) -> true`

Computes:

```text
dst[i] = dst[i] + src[i] * scale
```

### `soa.affine(s, dst, src, scale, bias) -> true`

Computes:

```text
dst[i] = src[i] * scale + bias
```

### `soa.affineWhere(s, dst, src, scale, mask, bias) -> true`

Computes the same affine update only for mask-true rows:

```text
if mask[i] { dst[i] = src[i] * scale + bias }
```

`dst` must be an f64 column, `src` must be numeric, and `mask` must be a bool
dense array with the same length as `s`.

### `soa.affineMany(s, terms) -> true`

Runs multiple independent affine updates in one call:

```gscript
soa.affineMany(points, {
    {dst: "x", src: "vx", scale: dt, bias: 0},
    {dst: "y", src: "vy", scale: dt, bias: 0},
})
```

Each term must have string `dst` and `src`, numeric `scale`, and optional
numeric `bias` defaulting to zero. A destination column may appear only once.
Source columns may not also be written by the same `affineMany` call; split
dependent updates into separate calls to preserve order.

### `soa.sum(s, column) -> number`

Reduces one numeric dense column and returns its sum.

### `soa.scan(s, column) -> dense array`

Returns an inclusive prefix sum for a numeric dense column. The output dtype
matches the source column.

```gscript
offsets := soa.scan(points, "count")
```

### `soa.scanInto(s, dst, src) -> true`

Writes the inclusive prefix sum of `src` into `dst`. `dst` and `src` must have
the same length; an `i64` source may write into an `f64` destination.

### `soa.clamp(s, column, min, max) -> dense array`

Returns a dense array where each numeric value in `column` is clamped into
`min..max`. Bounds must be numeric for `f64` columns and integer-compatible
for `i64` columns.

```gscript
safe_speed := soa.clamp(points, "velocity", 0, 100)
```

### `soa.clampInto(s, dst, src, min, max) -> true`

Writes clamped values from `src` into `dst`. The destination and source must
have the same length; an `i64` source may write into an `f64` destination.

### `soa.dot(s, left, right) -> number`

Returns the dot product of two numeric dense columns with the same length.
When both columns are `i64`, the result is an integer; mixed or `f64` columns
return a float.

```gscript
energy := soa.dot(points, "velocity", "mass")
```

### `soa.sumWhere(s, column, mask) -> number`

Reduces numeric `column` over mask-true rows without row materialization.
`mask` must be a bool dense array with the same length as `s`.

```gscript
mask := []bool{true, false, true}
total := soa.sumWhere(points, "x", mask)
```

### `soa.dotWhere(s, left, right, mask) -> number`

Masked dot product over rows where `mask` is true. Use this instead of
`soa.filter` plus `soa.dot` when the compacted rows are not needed.

### `soa.minWhere(s, column, mask) -> number`

Returns the minimum numeric column value over mask-true rows. It errors if no
rows are selected.

### `soa.meanWhere(s, column, mask) -> number`

Returns the mean numeric column value over mask-true rows as a float. It errors
if no rows are selected.

### `soa.maxWhere(s, column, mask) -> number`

Returns the maximum numeric column value over mask-true rows. It errors if no
rows are selected.

### `soa.countWhere(s, mask) -> integer`

Returns the number of mask-true rows without materializing a filtered SoA.

### `soa.statsWhere(s, column, mask) -> table`

Computes `count`, `sum`, `min`, `max`, and `mean` for a numeric column over
mask-true rows in one pass. When no rows are selected, `count` is zero, `sum`
is zero, and `min`, `max`, and `mean` are `nil`.

## Hot Path Guidance

- Prefer `soa.column` plus fused kernels for loops that touch a small subset of
  fields.
- Prefer `soa.addScaled`, `soa.affine`, `soa.affineMany`, and `soa.sum` over
  manual row loops when the operation fits their contracts.
- Use `soa.scan` and `soa.scanInto` for prefix sums and offset generation.
- Use `soa.clamp` and `soa.clampInto` for range limiting without row
  materialization.
- Use `soa.dot` and `soa.dotWhere` for vector-style products instead of
  multiplying columns through row materialization.
- Use `soa.mask` to produce reusable dense masks from column comparisons.
- Use `soa.select` for branch-free mask selection into a dense temporary.
- Use `soa.selectInto` when a dense scratch/output column can be reused.
- Use `soa.sumSelect` when a selected dense temporary would only be summed.
- For masked aggregates, prefer `soa.sumWhere` over `soa.filter` plus `soa.sum`
  when you do not need the compacted rows.
- For compact/filter pipelines, keep the mask as a bool dense array and use
  `soa.compact` or `soa.filter` once before downstream kernels.
- For gather-style pipelines, keep selection positions in a dense i64 array so
  `soa.gather` can preserve order and duplicates without row tables.
- Use `soa.indicesWhere` and `soa.scatterInto` for sparse update pipelines where
  the selected row set should stay explicit.
- Keep numeric columns dense and stable. Replacing columns or changing lengths
  prevents specialization from reusing prior layout facts.
- Use `soa.row` and `soa.setRow` at API boundaries, not inside high-iteration
  loops.
- Use `soa.shape` for diagnostics and tests; do not branch application logic on
  version numbers.

## Limitations

- There is no parser-level SoA syntax; SoA is a stdlib/runtime value.
- `soa.zip` does not deep-copy columns. It keeps the provided dense arrays.
- Row views are copies, not live proxies.
- Additional masked aggregate helpers can follow the same `*Where` contract.
- `soa.slice`, `soa.filter`, and `soa.unzip` copy dense columns rather than
  returning zero-copy views.
- The current fast path is a portable runtime kernel layer. Direct SIMD/native
  loop-body emission is future work.
