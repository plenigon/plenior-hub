# 002 — Check item versions on hosted runners

- Date: 2026-11-23
- Status: accepted

## Context

Plenigon Communities never opens the packages people publish (its ADR 006
and ADR 018). Each version is checked in an isolated runner instead, which
claims the check with a short-lived OIDC token, downloads the package
through a presigned link, and reports the result. A package is untrusted
input, and this repository is public, so anyone can open a pull request.

## Decision

- The `Item check` workflow runs only on `workflow_dispatch` from `main`.
  Communities pins its `job_workflow_ref` and repository id, and trusts no
  token of another workflow or branch.
- Every job of every workflow runs on a GitHub-hosted runner, a disposable
  machine per run. No job asks for a self-hosted runner; the gate fails if
  one does.
- The workflow holds no secret. Its only credential is the OIDC token of
  the run, for the audience in the `COMMUNITIES_URL` variable.
- A check only reads the package: it is never unpacked to disk, installed,
  or run. Every version gets the `package` check, which proves the bytes are
  the ones the version lists and that they read as its media type. Then the
  check its format names runs; a check this hub does not have fails.
- A failure that may pass later, such as a refused claim or a bucket that
  does not answer, is not reported, and Communities dispatches the check
  again. A package that is not the one listed is reported as failed.

## Consequences

- A malicious package reaches no credential that outlives its run, and no
  machine of Plenigon's.
- Checks depend on GitHub-hosted runners being available to this public
  repository.
- Adding a format's check is a change to this repository, reviewed like any
  other.
