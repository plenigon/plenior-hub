# 001 — Record architecture decisions

- Date: 2026-11-23
- Status: accepted

## Context

Plenior Hub starts from an empty repository. It runs checks that Plenigon
Communities trusts, so decisions about what it runs, where, and with which
credentials must stay traceable for every contributor and reviewer.

## Decision

Significant architecture decisions are recorded as numbered files in
`docs/adr/`, in the format described in `docs/adr/README.md`. A pull request
that makes such a decision adds or supersedes a record in the same change.

## Consequences

The reasoning behind the hub is reviewable next to the code. A decision
cannot be changed silently: it needs a new record that supersedes the old
one.
