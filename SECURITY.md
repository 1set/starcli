# Security policy

## Reporting a vulnerability

Please use [GitHub private vulnerability reporting](https://github.com/1set/starcli/security/advisories/new)
for security issues. Include the affected version or commit, operating system
and architecture, relevant capability grants and configuration, and a minimal
reproduction with the expected and actual behavior. For source builds, include
the Go version. Use synthetic data and remove credentials from scripts, logs,
and session recordings before sharing them.

Use public issues for ordinary bugs and support. There is no guaranteed
response time; keep vulnerability details private while coordinating a fix.

## Supported versions

Security fixes target the latest release through patch updates. Older releases
are not maintained as separate security branches. Review release notes for
behavior changes before upgrading, and keep a known working binary for rollback.

The `go.mod` version is a compatibility floor. Build production binaries with
a supported Go toolchain containing the latest security fixes, or use the
prebuilt release binaries. See the README for checksum and provenance checks.

## Execution boundary

StarCLI is intended for host-selected, reviewed scripts in a controlled,
single-tenant environment. Capability grants limit available operations; they
do not provide process or memory isolation. The default `open` tier permits
network and filesystem access. `--allow-cmd` authorizes arbitrary host commands
and process environment access, including through `gum` and `runtime`.

Use the narrowest capability grants and import directories that your script
needs. HTTP execution has request and cooperative execution limits, but these
do not preempt arbitrary Go builtins or impose a complete output-byte budget.
Untrusted scripts or public multi-tenant services need a separate constrained
worker, resource limits, and an independent security review.

The interpreter remains at `go.starlark.net@v0.0.0-20260324133313-ffb3f39dd27a`.
It predates the [upstream parser recursion fix](https://github.com/google/starlark-go/commit/5395d018f003e2a08bfbca6dcb2562acee700f62)
(GHSA-wcqm-92f8-mh2v): deeply nested source can terminate the host process.
This includes `check`, execution, generated source and modules reached through
`load()`. Step limits, cancellation and `recover` cannot protect this parsing
phase. Review all source and loaded modules; arbitrary source requires a
separately isolated host deployment. This release does not fix that parser risk.

Treat transcripts from `--record` as sensitive output: they can include input,
results, and errors. Control their access and retention at the host level.

## Optional dependency packages

`golang.org/x/crypto v0.55.0` preserves the existing Go 1.25.8 source-build
floor. The CLI uses its SHA-3 package through request validation. SSH and
OpenPGP are outside the build and test dependency graph; their known advisories
remain in the module and are not described as fixed. CI rejects importing
those packages without another dependency and compatibility review. Final
release scans must still evaluate the actual target artifacts.
