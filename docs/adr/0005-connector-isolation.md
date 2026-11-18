# ADR-0005: Connector trust tiers; custom code runs out-of-process

**Status:** Proposed

## Decision
Three tiers: **built-in** (first-party Go, compiled in), **declarative** (org-defined REST connectors described as
data, executed by the built-in HTTP engine — no custom code), **custom code** (built with `pkg/connectorsdk`, runs as a
separate process/container speaking a versioned gRPC Connector Protocol; core never `dlopen`s Go plugins).
The core always resolves credentials and passes only what an invocation needs; custom connectors never get DB/NATS
access. Connector versions are semver; publish of a new version runs schema compatibility checks against
operations used by published workflows/compositions.

## Alternatives
Go `plugin` package (same address space, version-locked toolchain — rejected per PRD), WASM (promising for
transformation functions; to be evaluated for Phase 3 plugins).

## Consequences
Phase 1 ships built-in + declarative + protocol/SDK with a reference sidecar. Managed sandbox runtime for untrusted
third-party code is Phase 3.
