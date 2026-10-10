# Contributing to SetupForge

> **Note:** For general contribution workflow, branch naming, commit message conventions, and PR guidelines, see [`CONTRIBUTING.md`](../CONTRIBUTING.md). This document details architectural principles and safety requirements.

## Development principles

SetupForge is a safety-sensitive system configuration tool. Contributions should favor:

- correctness;
- explicit behavior;
- maintainability;
- testability;
- safe system operations;
- accessible terminal UX.

## Before opening a change

Read:

- `AGENTS.md`;
- relevant `.agents/rules/*.md`;
- relevant `.agents/skills/*/SKILL.md`.

For new UI, complete the UI/UX design workflow before implementation.

## Code

Use standard Go formatting and repository lint/test configuration.

Keep responsibilities separated:

```text
UI → application service → domain interface → infrastructure
```

Do not execute package manager/system commands from TUI code.

## System operations

Every system mutation must have:

- validated inputs;
- explicit privilege behavior;
- appropriate confirmation;
- cancellation/error handling;
- tests using fakes where practical.

## UI

Keyboard interaction is mandatory for core workflows.

Test at:

- 80×24;
- 100×30;
- 120×35;
- 160×45+.

Check empty, loading, success, error, and confirmation states.

## Tests

At minimum for non-trivial changes:

```text
gofmt
go vet ./...
go test ./...
```

Run additional integration/system checks when the feature requires them.

Never make ordinary tests modify the developer's actual package database or system configuration.

## Documentation

Update documentation when behavior, security assumptions, supported Fedora versions, configuration, or user workflows change.

## Pull requests

Describe:

- problem;
- solution;
- architectural impact;
- UI impact;
- safety impact;
- tests;
- manual verification;
- known limitations.

Keep PRs focused.

## Security

If you discover a security-sensitive issue, avoid publishing exploit details in a routine feature discussion. Follow the project's responsible disclosure process in [`SECURITY.md`](../SECURITY.md).
