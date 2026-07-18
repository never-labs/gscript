# Leia

Leia is a general-purpose scripting language designed to run standalone or inside Go applications.
It combines a compact, Go-shaped syntax with dynamic values, modules,
concurrency, an embeddable VM, and opt-in domain dialects.

```leia
func greet(name) {
    return "Hello, " .. name .. "!"
}

numbers := [1, 2, 3, 4, 5]
total := 0
for _, n := range ipairs(numbers) {
    total += n
}

print(greet("Leia"))
print("sum:", total)
```

Save the program as `hello.leia`, then run it with `leia run hello.leia`.

## Getting Started

Leia v0.1.0 requires Go 1.25.12 or later when installed from source. Install the
CLI and language server at the release tag:

```bash
go install github.com/never-labs/leia/cmd/leia@v0.1.0
go install github.com/never-labs/leia/cmd/leia-lsp@v0.1.0
```

Release archives contain both binaries and are verified against the published
`SHA256SUMS`. Review the installer before running it, then install to a directory
on your `PATH`:

```bash
curl -fsSLo /tmp/leia-install.sh https://raw.githubusercontent.com/never-labs/leia/v0.1.0/scripts/install.sh
less /tmp/leia-install.sh
bash /tmp/leia-install.sh --version v0.1.0 --bin-dir "$HOME/.local/bin"
```

Check the installation and run a one-line program:

```bash
leia version
leia eval 'print("hello from leia")'
```

See [Getting started](docs/tutorial/getting-started.md) for repository examples,
formatting, linting, testing, and project modules.

## Language At A Glance

Leia provides dynamic values, lexical closures, arrays and tables, functions,
multiple returns, errors, modules, coroutines, and Go-style goroutines and
channels. The [language specification](docs/spec/index.md) is the normative
syntax and semantics reference; the [standard library index](docs/reference/stdlib/index.md)
lists the modules available to scripts.

Tagged dialects let a host add domain syntax without changing the core grammar.
Data-oriented libraries, scientific helpers, hot reload, and provider-backed AI
workflows are available for applications that need them. Start with the core
language and enable host-facing capabilities explicitly.

## Embed In Go

The root Go package is the supported embedding API. A minimal trusted-local
host looks like this:

    package main

    import leia "github.com/never-labs/leia"

    func main() {
        vm := leia.New()
        if err := vm.Exec(`print("hello from embedded Leia")`); err != nil {
            panic(err)
        }
    }

`VM` is mutable and is not safe for concurrent use. Use one VM per script
instance or `leia.Pool` for concurrent hosts. Compile reusable programs once,
but do not run the same compiled `Program` concurrently. The
[embedding guide](docs/guides/embedding.md) covers host modules, values,
cancellation, resource budgets, pooling, and hot reload.

## Security

`leia.New()` is intended for trusted local code and is not a sandbox. For
untrusted scripts, begin with `SecuritySandbox()`, add explicit resource
budgets, and expose only narrowly scoped Go bindings:

    vm := leia.New(
        leia.SecuritySandbox(),
        leia.WithMaxSteps(100_000),
        leia.WithMaxNativeCalls(1_000),
    )

Treat filesystem, environment, network, process, dynamic evaluation, module
loading, and host callbacks as capabilities owned by the embedding application.
Do not put provider credentials in Leia source. Read the
[sandbox reference](docs/reference/security/index.md) before running untrusted
code, and report vulnerabilities privately as described in
[SECURITY.md](SECURITY.md).

## Compatibility And Experimental Features

v0.1.0 is the first public release and remains pre-1.0. The spec-covered core
language, documented standard-library APIs, CLI, module resolution, and root Go
embedding API are the intended compatibility surface for this tag. Necessary
pre-1.0 changes will be documented in release notes, but strict backward
compatibility is not yet guaranteed.

The following remain experimental: AI providers and live-provider behavior,
new or optional dialect extensions, JIT internals and availability, typed
runtime kernels, and diagnostics not explicitly documented as stable. Programs
must not depend on JIT compilation; the interpreter defines semantics and the
bytecode VM and JIT must preserve them. See the
[platform policy](docs/reference/platforms/index.md) and
[performance methodology](docs/reference/performance/index.md). Benchmark
comparisons, including LuaJIT references, apply only to the recorded workloads,
hardware, versions, and execution modes; Leia makes no whole-language
"LuaJIT-class" performance guarantee.

## Platforms

The initial support target is the CLI, interpreter, and bytecode VM on macOS
and Linux for amd64 and arm64. A combination is considered tested only when its
release notes contain execution evidence. Windows amd64 and arm64 archives may
be published as available artifacts, but Windows is not support-claimed for
v0.1.0 without matching release evidence. Native JIT acceleration is limited to
eligible ARM64 builds and remains experimental.

Run `leia capabilities --json` to inspect the modes and optional surfaces in a
specific binary. The [platform matrix](docs/reference/platforms/index.md) is the
authoritative support policy.

## Documentation

- [Getting started](docs/tutorial/getting-started.md)
- [Language specification](docs/spec/index.md)
- [CLI reference](docs/reference/cli/index.md)
- [Go embedding API](docs/reference/embedding/index.md)
- [Standard library](docs/reference/stdlib/index.md)
- [Modules](docs/reference/modules/index.md)
- [Dialects](docs/reference/dialects/index.md)
- [Examples](docs/examples/index.md)
- [Security and sandboxing](docs/reference/security/index.md)
- [Platforms and execution modes](docs/reference/platforms/index.md)
- [Release process and compatibility policy](docs/release/index.md)

Leia is licensed under the [Apache License 2.0](LICENSE). Contributions are
covered by [CONTRIBUTING.md](CONTRIBUTING.md) and the
[Code of Conduct](CODE_OF_CONDUCT.md).
