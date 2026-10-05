# planfix

[![CI](https://github.com/6insanes/planfix-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/6insanes/planfix-cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/6insanes/planfix-cli.svg)](https://pkg.go.dev/github.com/6insanes/planfix-cli)

Command-line client for the Planfix REST API. Built for agent use.

## Install

Prebuilt binaries: [Releases](https://github.com/6insanes/planfix-cli/releases)
and per-commit snapshot builds in the artifacts of
[CI runs](https://github.com/6insanes/planfix-cli/actions/workflows/ci.yml).

    go install github.com/6insanes/planfix-cli@latest

Or build from source:

    go build -o planfix .
    cp planfix ~/.local/bin/

## Authenticate

    planfix auth login          # domain + token
    planfix ping                # OK

Or via env (no config file):

    export PLANFIX_DOMAIN=example.planfix.ru
    export PLANFIX_TOKEN=...

## Commands

    planfix task list|view|create|update|open
    planfix comment list|add
    planfix time add|list
    planfix project list
    planfix user list
    planfix ping
    planfix auth login|status|logout

Global flags: `--json`, `--fields`, `-q`, `--profile`.

### Log time

    planfix time add 2276867 --hours 1.5 --note "fixed the bug"
    planfix time add 2276867 --from "2026-10-02 10:00" --to "2026-10-02 12:00"
    planfix time list 2276867

Worklog is stored as a Planfix data tag; the CLI discovers it automatically
and caches the field ids in the config.
