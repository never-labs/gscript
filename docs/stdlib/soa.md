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
- `soa.slice`, `soa.filter`, and `soa.unzip` return independent dense-column
  copies.

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

## Hot Path Guidance

- Prefer `soa.column` plus fused kernels for loops that touch a small subset of
  fields.
- Prefer `soa.addScaled`, `soa.affine`, `soa.affineMany`, and `soa.sum` over
  manual row loops when the operation fits their contracts.
- Keep numeric columns dense and stable. Replacing columns or changing lengths
  prevents specialization from reusing prior layout facts.
- Use `soa.row` and `soa.setRow` at API boundaries, not inside high-iteration
  loops.
- Use `soa.shape` for diagnostics and tests; do not branch application logic on
  version numbers.

## Limitations

- There is no parser-level SoA syntax; SoA is a stdlib/runtime value.
- Columns cannot be appended, removed, or resized through the `soa` API yet.
- `soa.zip` does not deep-copy columns. It keeps the provided dense arrays.
- Row views are copies, not live proxies.
- `soa.slice`, `soa.filter`, and `soa.unzip` copy dense columns rather than
  returning zero-copy views.
- The current fast path is a portable runtime kernel layer. Direct SIMD/native
  loop-body emission is future work.

