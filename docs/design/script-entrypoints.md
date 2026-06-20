# Script Entrypoints

Leia repository automation uses one shell launcher:

```sh
scripts/run.sh <task> [args...]
```

The launcher is the boot boundary. It may locate the repository, select a Leia
binary for `.leia` tasks, and dispatch existing legacy tasks. It must not grow
repository business logic.

Allowed standalone shell entrypoints:

- `scripts/install.sh`: first install path; Leia may not exist yet.
- `scripts/run.sh`: single repository launcher.

All other repository automation should move toward one of these forms:

```sh
scripts/run.sh docs
scripts/run.sh perf --smoke
scripts/run.sh release-check --build
go run ./cmd/leia <subcommand>
leia <subcommand>
```

Legacy `scripts/*.sh` task files may remain while tests, docs, and CI still
reference them, but new automation should not add another top-level wrapper.
When a task is rewritten, put the implementation in Leia or Go and keep
`scripts/run.sh` as the compatibility dispatcher.
