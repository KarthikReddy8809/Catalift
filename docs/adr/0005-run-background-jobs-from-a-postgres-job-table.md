# ADR-0005: Run background jobs from a Postgres job table

- Status: Accepted
- Date: 2026-10-01
- Task: none
- Deciders: Karthik Reddy
- Area: messaging
- Reversibility: cheap: jobs are short-lived; swapping the queue means changing the enqueue and claim code, with no long-lived data to migrate

## Context

- Detection, generation and single-field regeneration call the AI model and take seconds each, so they run in the background (US-00-002, US-00-003 AC-6, US-00-008).
- Load estimate: a 300-SKU launch is about 900 AI calls (detection plus two channels per product), from PRD section 1 and REQ-018. At concurrency 4 and a few seconds per call that is roughly 1 job per second and about 15 to 20 minutes per launch (estimate, not measured).
- PostgreSQL in Docker is the store (PRD Constraints); the worker runs as a docker compose service on the VM (ADR-0002); the backend is Go and rebuilds the `brg_jobs` design in Go (ADR-0004).
- Not in the repository: OpenRouter's rate limit for the key, and whether a call that timed out on our side is charged. Assumed: concurrency 4 is well under the limit, and a timed-out call may be charged.

## What else was considered

| Option | Why not | Would suit |
| --- | --- | --- |
| Postgres job table, own code (chosen) | about 200 lines of claim, retry and sweep code to write and test | a single Postgres, low volume, and transactional enqueue |
| River (Go library on Postgres) | a library outside the standard stack needing sign-off, with its own migrations | a team that would rather not own queue code, or needs cron and a jobs UI |
| asynq on Redis | adds Redis to the VM, and enqueue is not in the Postgres transaction, so jobs can be lost or orphaned | high job rates with Redis already running |
| Cloud Pub/Sub | publish after commit needs an outbox to avoid loss; network hop and setup for 900 jobs a launch | several services consuming the same events across machines |

## Decision

We will keep background jobs in a Postgres table, enqueued in the same transaction as the state change they belong to, and claimed by a Go worker running as its own docker compose service, because it is the only option that is transactional with no new infrastructure, and the volume is tiny.

How it answers the asynchronous-work checks:

1. Enqueue commits in the same transaction as the product or listing status change.
2. The consumer is the always-on worker container on the VM (ADR-0002).
3. A timed-out OpenRouter call has no idempotency key: it is recorded as possibly charged, counted against spend, and retried within the attempt limit; the residual is a few cents of double spend, accepted by the product owner within the USD 8 block.
4. Retries back off at 10 seconds, 1 minute and 5 minutes; after the third failure the job ends `failed` and the product shows the reason (US-00-002 AC-2). A stale-claim sweep returns jobs whose worker died, and honours the same attempt limit.
5. Dispatch is capped by worker concurrency 4, so a backlog after an outage drains at the same rate.
6. Each job is keyed by product, step and channel; results are upserted, so a job run twice does not duplicate anything.

## Consequences

- Claims use row locking with skip, so a second worker process can be added without double work.
- Budget checks happen at call time in the gateway, not at enqueue time (US-00-012), so a queued job past the limit is stopped, not run.
- Revisit if job volume passes about 50 per second sustained, or another service needs to consume the same work.

## Commits us to

PostgreSQL (existing), Go worker process (existing language, ADR-0004)
