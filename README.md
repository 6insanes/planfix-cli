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

## Quick start

    planfix auth login          # domain + token
    planfix ping                # OK

Or via env (no config file):

    export PLANFIX_DOMAIN=example.planfix.ru
    export PLANFIX_TOKEN=...

## Configuration

Credentials and profiles live in a YAML file, by default
`~/.config/planfix/config.yml`; it is written atomically with mode 0600 —
keep it out of version control. `planfix auth login` maintains it for you.

    current_profile: work
    profiles:
      work:
        domain: example.planfix.ru
        token: <api-token>
        worklog:
          datatag: "Фактическое время"
          datatag_id: 2
          field_date: 456
          field_time: 457
          field_work_type: 458
          field_note: 459
          field_employee: 460
          work_type_directory: 12
          work_type_values: ["Разработка", "Поддержка"]
          time_in_minutes: false

### Environment variables

| Variable          | Meaning                                        |
| ----------------- | ---------------------------------------------- |
| `PLANFIX_CONFIG`  | config file path                               |
| `PLANFIX_PROFILE` | active profile name                            |
| `PLANFIX_DOMAIN`  | account domain, overrides the profile value    |
| `PLANFIX_TOKEN`   | API token, overrides the profile value         |

Active profile precedence: `--profile` > `PLANFIX_PROFILE` > `current_profile`
> `default`. `PLANFIX_DOMAIN` + `PLANFIX_TOKEN` alone are enough for
env-only auth (CI/agents) with no config file at all.

### Worklog cache

Work time is stored as a Planfix data tag. On the first `time` command the CLI
discovers the tag and caches its field ids in the profile's `worklog` section;
nothing there needs to be written by hand.

- `datatag` — optional pin: data tag id or exact name. Change it and the
  remaining field ids are re-resolved. `--data-tag` overrides the pin
  per invocation.
- `time_in_minutes` — set when the tag's time field is a spent-minutes number
  field instead of a from/to period-of-time field.
- `--refresh-worklog-meta` forces re-resolution when the tag schema changed.

## Commands

    planfix task list|view|statuses|create|update|take|open
    planfix comment list|add|edit|delete
    planfix time add|list
    planfix project list
    planfix user list
    planfix ping
    planfix auth login|status|logout

Global flags: `--json`, `--fields`, `-q`, `--plain`, `--profile`.

`--plain` strips HTML from rendered values: `task view --plain` prints the
description as plain text instead of raw markup.

### Tasks

    planfix task list --saved-filter :in
    planfix task list --filter '[{"field":"status","operator":"neq","values":["6"]}]'
    planfix task view 2276867
    planfix task view 2276867 --plain
    planfix task statuses 2276867
    planfix task create --name "fix the bug" --assignees user:123 --end-date 2026-10-10
    planfix task update 2276867 --status 2 --assignees user:123
    planfix task take 2276867 --status 2
    planfix task open 2276867

`task take` assigns the task to the current user (via `/userinfo`) and sets the
status, `2` ("В работе") by default. `task statuses` lists the status ids
available for the task. `task open` prints the task URL.

### Comments

    planfix comment add 2276867 --body "looks good"
    planfix comment edit 2276867 36742694 --body "updated text"
    planfix comment delete 2276867 36742694

`--body` reads stdin when empty; `--silent` skips notifications.

### Log time

    planfix time add 2276867 --hours 1.5 --note "fixed the bug"
    planfix time add 2276867 --from "2026-10-02 10:00" --to "2026-10-02 12:00"
    planfix time add 2276867 --hours 2 --work-type "Поддержка" --date 2026-10-02
    planfix time list 2276867

See [Worklog cache](#worklog-cache) for how the data tag is discovered.

### Projects and users

    planfix project list
    planfix user list

### Auth

    planfix auth login --name work --domain example.planfix.ru
    planfix auth status
    planfix auth logout --profile work

`auth login` stores the profile and makes it current; existing worklog cache is
preserved on re-login. `auth status` shows the active profile and checks API
access. `auth logout` removes the selected profile.
