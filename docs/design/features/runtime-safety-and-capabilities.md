# Runtime Safety and Capabilities

## Goal

Leia can be high-level and side-effectful, but embedding hosts must be able to
control filesystem, process, network, LLM, Go binding, and package access.

Capability control is a runtime and tooling foundation. It should not be a
showy user-facing DSL in the first version.

## Capability Declarations

File-level:

```leia
//leia:cap fs.read,host.process,llm.turn
```

CLI:

```text
leia run app.leia --cap fs.read,llm.turn
```

Embedding:

```text
WithCapabilities("fs.read", "llm.turn")
```

## Capability as Value

Later, scoped capability values may be useful:

```leia
with cap.fs.read(path`games/stonebridge`) {
    files := glob`**/*.leia`
}
```

This should narrow access, not widen it silently.

## Dialect Requirements

Every side-effecting dialect must declare required capabilities:

- `sh` / `cmd`: process execution;
- `glob` / `path` with filesystem inspection: filesystem read;
- `web` / `api` / `http`: network client/server;
- `mail`: mail/network/secret access;
- `agent` / `turn`: LLM provider access;
- `goimport` / `binding`: host binding access.

## Secret Handling

Secrets should be explicit values from environment, host injection, or secret
providers. Source examples should not encourage checked-in API keys.

## Sandbox Block

A future block form may exist:

```leia
sandbox {
    caps: ["fs.read"]
    run: fn() {
        files := glob`**/*.leia`
    }
}
```

This is not a first-phase priority. The important requirement is that the
capability model works without a sandbox DSL.

## Non-Goals

- Do not let dialects bypass capability checks.
- Do not make sandbox syntax a primary product feature before the capability
  model is stable.
- Do not expose arbitrary Go reflection without allowlists.
