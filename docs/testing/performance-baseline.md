# Performance Baseline (Phase 1, S14)

Recorded **2026-11-23** on a local development machine. The scripts in [tests/k6](../../tests/k6) fail their run when
a threshold is crossed, so these numbers are a regression baseline, not a service-level promise. Production sizing
depends on deployment (replicas, managed Postgres, network) and is out of scope for Phase 1.

## Environment

| | |
|---|---|
| Machine | Apple M1, 8 GiB RAM, macOS 26.3 |
| Application | one `hugi-api` and one `hugi-worker`, built from this tree with Go 1.27.3 |
| Data stores | PostgreSQL 18.3 (Homebrew), Redis 7.0.12, NATS 2.15.3 JetStream — all local, same machine |
| Load generator | k6 v2.3.0, same machine (shares CPU with the stack) |
| Stack | api `127.0.0.1:58080`, worker `127.0.0.1:58081`, NATS `127.0.0.1:54222`, Redis database 3, flushed before the runs |

The three scripts each create their own organization and workspace in `setup()`, so they can run against a shared
development stack. Authentication rate limits apply to the load generator like to anyone (10 signups/hour per IP,
10 logins per account and 30 per IP per 5 minutes), which is why the VU counts stay below 10.

## Results

| Script | Load | Thresholds | Observed |
|---|---|---|---|
| `api-baseline.js` | 8 VUs × 30 s | reads p95 < 150 ms, p99 < 400 ms; overview p95 < 250 ms; errors < 1 % | 14,933 iterations (2,084 req/s); reads p50 2.5 ms / p95 6.0 ms; overview p50 3.7 ms / p95 7.2 ms; 0 failed requests |
| `workflow-baseline.js` | 4 executions/s × 30 s | accept p95 < 200 ms; completion p95 < 3 s; no unfinished execution | 120 executions (3.7/s); accept p50 9.7 ms / p95 66.8 ms; completion p50 1.86 s / p95 2.18 s; 0 unfinished, 1 dropped iteration at the VU cap |
| `webhook-baseline.js` | 50 req/s × 30 s | p95 < 100 ms, p99 < 250 ms; errors < 1 % | 1,501 accepted (49.5/s); p50 3.3 ms / p95 27.9 ms / p99 132 ms; 0 failed requests |

What the numbers cover:

- **API reads:** session check, workspace list, workflow list, connector catalog and the workspace overview (the
  heaviest read — three KPI sections, a zero-filled series and the top failing workflows).
- **Workflow:** a manual execution of `trigger.manual → logic.if → logic.parallel → action.transform ×2 →
  logic.merge`, end to end through the outbox relay, JetStream and `hugi-worker`. Completion is dominated by the
  outbox relay's 500 ms idle poll across four sequential hops, not by CPU.
- **Webhook ingress:** Standard Webhooks signature verification, replay dedupe check, transactional insert and the
  outbox event, answered `202`.

Maximum latencies in the summaries (200 ms for the overview, 457 ms for workflow polling) are cold-start and GC
spikes on a shared 8 GiB machine; percentiles are the signal.

## Reproduce

```bash
k6 run -e HUGI_BASE_URL=http://127.0.0.1:58080 tests/k6/api-baseline.js
k6 run -e HUGI_BASE_URL=http://127.0.0.1:58080 tests/k6/workflow-baseline.js
k6 run -e HUGI_BASE_URL=http://127.0.0.1:58080 tests/k6/webhook-baseline.js
```

Override the load with `-e VUS=…`, `-e RATE=…`, `-e DURATION=…`. The workflow baseline needs a running
`hugi-worker` and JetStream; the webhook baseline needs an endpoint whose `rate_limit_per_minute` is above the
arrival rate (the script creates one at 10,000/min). On a disposable stack, flush the Redis database between runs so
signup/login counters do not leak into the next run.

The same scripts also passed a reduced-load smoke run against the Docker Compose stack (two worker replicas,
Postgres/Redis/NATS in containers) during the S14 acceptance: api p95 6.6 ms / p99 11.1 ms, workflow completion p95
0.98 s with none unfinished, webhook p95 13.7 ms / p99 18.8 ms with 0 errors. Those numbers are not comparable to the
table above (5–8 s scenarios, shared machine) but prove the deployment path works end to end.
