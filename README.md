# Plenior Hub

Plenior Hub runs the checks of the items people publish to Plenigon
Communities: apps, flows, agents, plugins, and themes. Communities never
opens a package itself; it starts a run of this repository's workflow for
each version, and the run reports whether the package is the one listed and
whether it reads in its format.

## How a check runs

1. Communities dispatches the [`Item check`](.github/workflows/item-check.yml)
   workflow on `main` with one input, `check_id`.
2. The run asks GitHub for an OIDC token whose audience is the
   `COMMUNITIES_URL` repository variable, and claims the check at
   `GET /v1/item-checks/{check_id}`. The answer names the item, its version,
   its format, the check to run, and a presigned link to the package.
3. It downloads the package and proves its size and SHA-256 are the ones
   the version lists.
4. It runs the `package` check, which reads the package as its media type
   (`application/zip`, `application/gzip`, or `application/json`), then the
   check the format names.
5. It reports every result at `POST /v1/item-checks/{check_id}/result`.

A check never installs or runs what a package holds. Why the hub runs this
way is in [ADR 002](docs/adr/002-check-item-versions-on-hosted-runners.md).

## Repository layout

| Path | Contents |
|---|---|
| [`cmd/item-check/`](cmd/item-check/) | The command the workflow runs |
| [`internal/check/`](internal/check/) | The package check and the registry of format checks |
| [`internal/communities/`](internal/communities/) | The client of the item check endpoints of Communities |
| [`internal/fetch/`](internal/fetch/) | Downloads a package and proves it is the one listed |
| [`internal/oidc/`](internal/oidc/) | Requests the run's OIDC token from GitHub Actions |
| [`docs/adr/`](docs/adr/) | Architecture decision records |

## Configuration

| Name | Kind | Meaning |
|---|---|---|
| `COMMUNITIES_URL` | Repository variable | The https URL of Communities, also the audience of the run's token |

Communities is configured with this repository's numeric id and the
workflow's `job_workflow_ref`,
`plenigon/plenior-hub/.github/workflows/item-check.yml@refs/heads/main`.

## Development

You need Go (the version in [`go.mod`](go.mod)), gitleaks, golangci-lint,
and govulncheck on the `PATH`. Run the gate from the repository root:

```sh
scripts/gate.sh
```

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the rules every change follows.

## License

[Apache License 2.0](LICENSE).
