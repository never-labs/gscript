---
layout: page
title: Playground
---

# Playground

Run the local backend-powered playground from a checkout:

```sh
go run ./cmd/leia playground --addr 127.0.0.1:8080
```

Or run an installed binary:

```sh
leia playground --addr 127.0.0.1:8080
```

The playground uses the same parser, VM, standard library, dialect registry,
budgets, and diagnostics as `leia run`. Use `--timeout`, `--max-source-bytes`,
and `--max-steps` to bound execution.

Reference: [`leia playground`](reference/cli/index.md).
