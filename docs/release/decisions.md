# Release Decisions

Public releases require explicit maintainer decisions. This file records the
open decisions that must not be inferred from implementation details, examples,
or local validation runs.

## Required Before Public Release

| Area | Decision Needed | Current Status |
|---|---|---|
| License | Choose the repository license and whether a `NOTICE` file is required. | Open. |
| Security reporting | Confirm the private reporting route, contact path, and disclosure policy. | Open. |
| Platform support | Define tested and supported OS/architecture combinations for the release. | Open. |
| Release channels | Decide which channels are public: GitHub Releases, install script, `go install`, package managers, or others. | Open. |
| Artifact signing | Decide whether SHA256 checksums are sufficient or whether cosign, GPG, or another signing flow is required. | Open. |
| Compatibility policy | Define the pre-1.0 compatibility promise and the intended v1.0 stable surface. | Open. |

## License

The repository has no selected license until a root `LICENSE` file exists.
Release notes and README text must not imply open-source reuse terms before
that decision is made.

Record:

- license identifier;
- whether a `NOTICE` file is required;
- whether examples, generated artifacts, or vendored assets need separate
  notices.

## Security Reporting

`SECURITY.md` describes the current conservative reporting path. A public
release still needs an explicit maintainer decision for any confirmed contact
route or disclosure process.

Record:

- whether GitHub private security advisories are enabled;
- whether an email address, PGP key, CVE process, embargo policy, or response
  target is provided;
- which versions or tags receive security fixes.

## Platform Support

`docs/reference/platforms/index.md` defines support levels. Release notes must
map actual release evidence to those levels.

Record:

- tested OS/architecture combinations;
- Go version;
- interpreter, bytecode VM, and JIT modes tested;
- any disabled capabilities, live providers, editor integrations, or package
  manager channels.

## Release Channels

Release evidence must identify the distribution channels and artifacts that are
official for the tag.

Record:

- published archive names;
- whether `scripts/install.sh` is a supported install path;
- whether `go install github.com/never-labs/leia/...@TAG` is supported;
- whether package managers are supported.

## Artifact Signing

Release evidence must identify the checksum and signing policy that is official
for the tag.

Record:

- whether SHA256 checksums are sufficient;
- whether cosign, GPG, or another signing flow is required;
- which files are signed;
- checksum and signing requirements.

## Compatibility Policy

The specification, feature matrix, conformance gates, and release notes define
the compatibility surface. Optimizations, JIT availability, typed kernels, and provider integrations are not compatibility guarantees unless the release notes say so explicitly.

Record:

- stable language, q, stdlib, embedding, CLI, module, and dialect surfaces;
- experimental surfaces;
- implementation-defined behavior;
- migration notes for user-visible changes.
