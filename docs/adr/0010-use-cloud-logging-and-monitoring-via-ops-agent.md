# ADR-0010: Use Google Cloud Logging and Monitoring through the Ops Agent

- Status: Accepted
- Date: 2026-10-04
- Task: none
- Deciders: Karthik Reddy
- Area: observability
- Reversibility: cheap: the services emit standard JSON logs and Prometheus metrics; moving means swapping the agent and recreating five alerts

## Context

- One VM, two users (ADR-0002, PRD section 4). Log volume estimate: about 900 jobs per launch × about 5 lines × about 1 KB, around 5 MB per launch.
- The go-api kit emits slog JSON logs to stdout and Prometheus metrics; HLD section 10 found no metrics server planned.
- HLD section 10 names five alerts: CataliftDown, CataliftDiskHigh, CataliftAiFailing, CataliftSpendNearLimit, CataliftBackupMissing. Two of them (down, backup missing) must fire when the VM itself is gone.
- Not in the repository: who receives alerts. Assumed: email to the owner.

## What else was considered

| Option | Why not | Would suit |
| --- | --- | --- |
| Cloud Logging and Monitoring via the Ops Agent (chosen) | tied to Google Cloud; plainer dashboards; not the catalogue default | a single Google Cloud VM where alerts must outlive the VM |
| Grafana stack (Prometheus, Loki, Grafana) in compose on the VM | uses VM memory and dies with the VM, so it cannot alert on its own outage | several services on a cluster with its own monitoring nodes |
| Grafana Cloud free tier with Alloy | another account and agent; free-tier limits (assumption) | vendor-neutral dashboards or a move off Google Cloud |
| Sentry in addition | another SaaS and key for two users | real sellers using it, where error grouping pays off |

## Decision

We will install the Google Cloud Ops Agent on the VM to ship container logs to Cloud Logging and scrape the API's and worker's Prometheus endpoints into Cloud Monitoring, and define the HLD's five alerts there (uptime checks for CataliftDown, a VM disk metric for CataliftDiskHigh, log-based or scraped metrics for the AI, spend and backup alerts), notifying the owner by email, because it runs outside the VM, adds nothing to run on it, and costs about nothing at this volume (estimate).

## Consequences

- The Ops Agent, uptime checks, alert policies and the email notification channel are Terraform resources (ADR-0008).
- Traces stay off; OpenTelemetry in the kit is not exported anywhere.
- The five runbooks named in HLD section 10 are still to write (`runbook`).
- Log retention is Cloud Logging's default (assumption: 30 days); nothing here needs longer.
- Revisit if Catalift leaves Google Cloud, if more than one VM runs it, or if real sellers use it (then add error tracking).

## Commits us to

Google Cloud Ops Agent, Cloud Logging, Cloud Monitoring (uptime checks, alert policies, email notification channel) (outside the standard stack)
