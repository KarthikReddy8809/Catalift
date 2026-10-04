# ADR-0011: Host Catalift on GitHub and run CI with GitHub Actions

- Status: Accepted
- Date: 2026-10-04
- Task: none
- Deciders: Karthik Reddy
- Area: ci and delivery
- Reversibility: cheap: every job calls a make target, so moving hosts means rewriting the workflow files, not the build
- Supersedes: ADR-0009

## Context

- From the request (2026-10-04, while creating the repository): "i need to push the repo in github".
- ADR-0009 chose GitLab and GitLab CI, assuming GitLab from the repo plan's group paths; the owner has a GitHub account (KarthikReddy8809, signed in through the GitHub CLI on this machine) and no GitLab group.
- Eng review D4 made Catalift one repository, so there is one pipeline, not three.
- The delivery shape from ADR-0009 still holds: `make check` on every change, images built on `main`, manual deploy and manual `terraform apply`, keyless sign-in to Google Cloud through Workload Identity Federation (D10), and only the owner deploys.

## What else was considered

| Option | Why not | Would suit |
| --- | --- | --- |
| GitHub with GitHub Actions (chosen) | GitHub environments with required reviewers stand in for GitLab's protected manual jobs; the repo plan's subgroup paths flatten to one repository | an owner who already works on GitHub |
| Keep GitLab CI (ADR-0009) | the owner wants the code on GitHub; mirroring to GitLab only for CI adds a second host | a team already on GitLab |

## Decision

We will host the single Catalift repository at github.com/KarthikReddy8809/catalift and run GitHub Actions: `make check` on every pull request and push; on `main`, build and push images to Artifact Registry; a deploy workflow and a `terraform apply` workflow that run only by manual dispatch against the GitHub environment `prod`, which requires the owner's approval. Workflows sign in to Google Cloud through Workload Identity Federation with GitHub's OIDC token, trusting only this repository's `main` branch and the `prod` environment, with no stored service account key. We chose it because the owner hosts on GitHub, and every guarantee ADR-0009 gave carries over.

## Consequences

- ADR-0009 is superseded; its index rows point here.
- The Go module path is `github.com/KarthikReddy8809/catalift`; CODEOWNERS uses `@KarthikReddy8809`.
- The D10 trust condition becomes: repository `KarthikReddy8809/catalift`, ref `refs/heads/main`, environment `prod`.
- The repo plan's `git_path` (GitLab-style `Catalift/Server/CataliftApp`) no longer matches; the next HLD revision updates it.
- Revisit if the code moves to an organisation account (then the module path and WIF condition change) or to another host.

## Commits us to

GitHub, GitHub Actions, GitHub environments (prod with required reviewer), GitHub OIDC with Workload Identity Federation, Artifact Registry
