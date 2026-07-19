# Release Decisions

Public releases require explicit maintainer decisions. This file records the
decisions that must not be inferred from implementation details, examples, or
local validation runs, including their current resolution status.

## Initial Release Decisions

| Area | Decision Needed | Current Status |
|---|---|---|
| License | Use Apache-2.0; no `NOTICE` file is required unless third-party attribution requirements are added. | Resolved. |
| Security reporting | Use GitHub private security advisories, with security@never-labs.com as the fallback private contact. | Resolved. |
| Platform support | Test and support darwin/linux on amd64/arm64 for CLI, interpreter, and VM; ARM64 JIT support requires release evidence. | Resolved. |
| Release channels | Publish GitHub Releases, `scripts/install.sh`, and `go install`; package managers are not official for the initial public release. | Resolved. |
| Artifact signing | Publish SHA256 checksums for release archives; cosign/GPG signing is not required for the initial public release. | Resolved. |
| Compatibility policy | Use a pre-1.0 compatibility policy: spec-covered language behavior is maintained where practical; experimental dialect, AI, JIT, and provider surfaces may change with release notes. | Resolved. |

## License

The repository is licensed under Apache-2.0. The root `LICENSE` file is the
authoritative license text.

Record:

- license identifier: Apache-2.0;
- `NOTICE` requirement: none for the current repository contents;
- examples and generated artifacts are covered by the repository license unless
  a file states otherwise.

## Security Reporting

`SECURITY.md` defines the confirmed reporting path.

Record:

- GitHub private security advisories are the primary route;
- security@never-labs.com is the fallback private contact;
- target initial response time is five business days;
- `main` and the latest published tag receive security fixes unless release
  notes state otherwise.

## Platform Support

`docs/reference/platforms/index.md` defines support levels. Release notes must
map actual release evidence to those levels.

Initial supported combinations are `darwin/amd64`, `darwin/arm64`,
`linux/amd64`, and `linux/arm64` for CLI, interpreter, and bytecode VM.
Windows archives may be produced as available packaging targets, but Windows is
not support-claimed until release notes include validation evidence. ARM64 JIT
support is claimed only when the release evidence shows it enabled and tested.
Release notes record the exact Go version and execution modes for each tag.

## Release Channels

Release evidence must identify the distribution channels and artifacts that are
official for the tag.

Official initial public channels are GitHub Releases, `scripts/install.sh`,
and these explicit Go install targets:

```bash
go install github.com/never-labs/leia/cmd/leia@TAG
go install github.com/never-labs/leia/cmd/leia-lsp@TAG
```

Package managers are not official release channels until a later decision
records owner, workflow, and validation evidence.

## Artifact Signing

Release evidence must identify the checksum and signing policy that is official
for the tag.

SHA256 checksums are sufficient for the initial public release. Release
artifacts must publish checksums for every archive. cosign, GPG, or another
signing flow can be added later, but is not required for the first public tag.

## Compatibility Policy

The specification, feature matrix, conformance gates, and release notes define
the compatibility surface. Optimizations, JIT availability, typed kernels, and provider integrations are not compatibility guarantees unless the release notes say so explicitly.

Before v1.0, compatibility is best-effort and release-note-bound. The
spec-covered language core, stdlib APIs documented in reference pages,
embedding API, CLI behavior, module resolution, and tagged dialect contracts
are the intended stable surface for a given tag. Experimental surfaces include
AI provider integration, live providers, JIT internals, typed runtime kernels,
diagnostics formats not marked as release evidence, optional extension
semantics, and newly added dialect extensions. User-visible changes must be
listed in release notes with migration guidance.

## Branch Policy

Development uses `main` as the only active branch until a release workflow
explicitly introduces protected release branches.

Record from the 2026-06-29 cleanup:

- local worktrees were pruned to the repository root worktree only;
- local branches were pruned to `main` only;
- GitHub remote branches were pruned to `main` only;
- historical refs were bundled before deletion at
  `/tmp/leia-branch-cleanup-20260629-205123/leia-all-refs.bundle`;
- uncommitted pre-cleanup work was preserved in stash entries named
  `cleanup backup claude-main before branch/worktree prune 20260629` and
  `cleanup backup before branch/worktree prune 20260629`.
