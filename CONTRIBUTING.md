# Contributing

## Language

Everything committed to this repository is in English: code, identifiers,
comments, documentation, commit messages, branch names, issues, and pull
requests.

Only the result is committed: no transcripts, agent prompts, scratch notes,
logs, local paths, or secrets.

## Branches, commits, and pull requests

- Branches are named `type/short-description`, for example `feat/theme-check`.
- Commits follow [Conventional Commits](https://www.conventionalcommits.org/):
  `type(scope): summary`, imperative mood, subject of at most 72 characters,
  and a body that explains why. `scripts/check_commits.sh` enforces it.
- Every commit carries a `Signed-off-by` trailer with the author's name and
  email, certifying the [Developer Certificate of Origin](https://developercertificate.org/).
  `git commit -s` adds it. Dependency updates opened by Dependabot are
  exempt, because it signs off with a different address than it authors
  with.
- Commits carry no AI authorship trailer.
- Commits on `main` are signed and show as verified on GitHub.
- A pull request has one purpose and stays around 300 lines; split it above
  that. Never mix mechanical changes, such as renames or formatting, with
  behavior changes.
- The description states what changed, why, and the gate result.
- Behavior changes update the documentation and `CHANGELOG.md` in the same
  pull request.

## Gate

Run `scripts/gate.sh` from the repository root. It needs the tools listed in
the README on the `PATH`, checks commit messages, secrets, that no workflow
asks for a self-hosted runner, formatting, lint, tests, and known
vulnerabilities, and exits 0 only if every check passes.

Never change or delete a test to make the gate pass. A skipped test is a gate
failure.

## Engineering principles

- A check only reads a package. Nothing a package holds is installed,
  written out, or run.
- Every job runs on a GitHub-hosted runner, and no workflow holds a secret.
- A check that cannot reach a verdict fails; it never passes by default.
- Messages are written for the item's owner, who sees them in Communities.
- Keep dependencies to the standard library unless a check needs more.

## Decisions

Architecture decisions are recorded in [`docs/adr/`](docs/adr/). A change that
reverses a decision supersedes its record with a new one.
