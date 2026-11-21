# k6 baselines (Phase 1, S14)

Three scripts measure the Phase 1 hot paths against a running stack (api + worker + Postgres + Redis + NATS):

| Script | What it exercises | Default load | Thresholds (fail the run) |
|---|---|---|---|
| `api-baseline.js` | session check, workspace/workflow lists, connector catalog, overview KPIs | 8 VUs × 30 s | p95 < 150 ms (reads), overview p95 < 250 ms, errors < 1 % |
| `workflow-baseline.js` | manual execution of an if → parallel → merge workflow, end to end through outbox, JetStream and the worker | 4 executions/s × 30 s | accept p95 < 200 ms, completion p95 < 3 s, none unfinished |
| `webhook-baseline.js` | Standard Webhooks ingress: verify, dedupe, insert + outbox | 50 req/s × 30 s | p95 < 100 ms, p99 < 250 ms, errors < 1 % |

```bash
k6 run -e HUGI_BASE_URL=http://127.0.0.1:8080 tests/k6/api-baseline.js
```

Each run signs up one organization in `setup()` and every VU signs in as its owner, so a run can target a shared
development stack. Override the load with `-e VUS=…`, `-e RATE=…`, `-e DURATION=…`. The auth rate limits apply to
the load generator like to anyone (10 signups/hour per IP, 10 logins per account and 30 per IP per 5 minutes), which
is why VU counts stay below 10; on a disposable stack, flush its Redis database between runs.
The recorded baseline (hardware, versions, numbers) is in [docs/testing/performance-baseline.md](../../docs/testing/performance-baseline.md).
