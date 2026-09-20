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

Treat transcripts from `--record` as sensitive output: they can include input,
results, and errors. Control their access and retention at the host level.
