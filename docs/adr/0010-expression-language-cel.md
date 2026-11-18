# ADR-0010: CEL for conditions, mapping expressions and policies

**Status:** Proposed

## Context
Workflow conditions, mapping expressions, composition `when`, webhook event filters and (Phase 3) ABAC policy
conditions all need user-authored expressions. PRD: "Do not execute arbitrary expressions from untrusted policy
documents".

## Decision
Google **CEL** (`cel-go`): non-Turing-complete, no I/O, type-checked at publish time, with a per-evaluation **cost
limit** and a fixed, documented function library (string/date/math helpers we add explicitly). Simple-mode UIs generate
CEL; users never need to see it.

## Alternatives
JavaScript (goja/V8 isolates — too powerful to sandbox cheaply), expr-lang (good, but no standard across policy
engines; CEL is also used by Kubernetes, Envoy and Cedar-adjacent tooling), JSONLogic (verbose, weak typing).
