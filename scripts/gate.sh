#!/usr/bin/env bash
# Gate: every check must pass. New languages and tools add their checks here.
set -uo pipefail

cd "$(dirname "$0")/.." || exit 2

failed=0

# check NAME COMMAND...: runs COMMAND and records NAME as failed on a non-zero exit.
check() {
    local name="$1"
    shift
    if "$@"; then
        echo "gate: $name ok"
    else
        echo "gate: $name FAILED"
        failed=1
    fi
}

# require TOOL: fails the gate when TOOL is not on the PATH.
require() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "gate: $1 not installed"
        failed=1
        return 1
    fi
}

gofmt_clean() {
    local unformatted
    unformatted="$(gofmt -l .)"
    if [ -n "$unformatted" ]; then
        printf 'gofmt: not formatted:\n%s\n' "$unformatted"
        return 1
    fi
}

# Every workflow job runs on a GitHub-hosted runner: this repository is
# public, and a self-hosted runner would run code from any pull request.
hosted_runners_only() {
    if grep -nE '^\s*runs-on:.*self-hosted' .github/workflows/*.yml; then
        echo "workflows: a job asks for a self-hosted runner"
        return 1
    fi
}

check commits scripts/check_commits.sh
if require gitleaks; then
    check secrets gitleaks git --no-banner --redact --log-opts=HEAD .
fi
check hosted-runners hosted_runners_only

if require go; then
    check gofmt gofmt_clean
    check vet go vet ./...
    check test go test -race ./...
fi
if require golangci-lint; then
    check lint golangci-lint run ./...
fi
if require govulncheck; then
    check vulnerabilities govulncheck ./...
fi

if [ "$failed" -eq 0 ]; then
    echo "gate: all checks passed"
fi
exit "$failed"
