# ADR-0011: Envelope encryption for secrets

**Status:** Proposed

## Decision
Secrets (connector credentials, OAuth tokens, IdP client secrets, webhook signing secrets) are encrypted with
AES-256-GCM using a per-org **data key**; data keys are wrapped by a **key-encryption key** obtained from a `KeyProvider`
(Phase 1: local KEK from env/file with key id for rotation; later AWS KMS / GCP KMS / Vault Transit). Ciphertext
records `kek_id`, `dek_id`, nonce; AAD = `org_id|secret_id` so ciphertext cannot be moved between rows.
Secrets are write-only over the API (responses show `"configured": true`, last-4 hint at most). Resolution happens only
inside the data plane at invocation time; values are never logged (redacting `slog.Handler` + typed `Secret` wrapper
whose `String()`/`MarshalJSON()` return `[REDACTED]`).

## Consequences
KEK rotation = rewrap data keys (cheap). DEK rotation = re-encrypt org secrets (background job, Phase 3 secret rotation).
