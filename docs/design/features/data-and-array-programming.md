# Data and Array Programming

## Goal

`data` should mean real data processing, not text formats. JSON, CSV, YAML,
headers, JWT, and gzip are format/codec utilities. The `data` domain should
cover in-memory transformation, columnar data, matrix/vector computation,
aggregation, joins, and high-performance workloads.

## Core Concepts

```text
table
column
soa
matrix
vec
groupby
join
aggregate
window
stats
array
```

## Table Processing

```leia
rows := csv`...`

report := data {
    from: rows
    group_by: ["region", "status"]
    aggregate: {
        count: count()
        revenue: sum("amount")
    }
    sort: ["region", "-revenue"]
}
```

Field-block data DSL should support common reporting tasks without requiring
hand-written loops.

## Pipeline Style

```leia
report := rows
    |> filter(fn(r) { return r.status == "paid" })
    |> group_by(fn(r) { return r.region })
    |> map_values(fn(xs) { return sum(xs, "amount") })
```

The data APIs should work well both as block dialects and ordinary functions.

## SoA and Columnar Data

Requirements:

- create columnar structures from arrays/tables;
- access columns directly;
- transform many rows efficiently;
- support predictable memory layout where possible;
- expose enough metadata for JIT/runtime specialization.

## Matrix and Vector

```leia
v := vec`1 2 3`
m := matrix {
    rows: 4
    cols: 4
    values: nums`1 2 3 4 ...`
}
```

Requirements:

- element-wise arithmetic;
- dot products;
- matrix multiplication;
- reductions;
- statistics;
- clear scalar/vector/matrix behavior;
- operator overloading through metamethods where appropriate.

## Array DSL

Readable APL/K-inspired capability without symbolic noise:

```leia
array {
    z := x + y * 2
    avg := mean(z)
    top := take(sort(z, desc), 10)
}
```

Requirements:

- readable syntax;
- vectorized operations;
- clear broadcasting rules;
- no obscure glyph dependency;
- performance-oriented semantics.

## Format Boundary

Formats feed data, but are not data:

```leia
rows := csv`...`      // format
report := data { ... } // data processing
```

## Non-Goals

- Do not put every codec under `data`.
- Do not require users to understand memory layout for ordinary table work.
- Do not copy APL/K syntax directly.
