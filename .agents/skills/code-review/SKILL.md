---
name: code-review
description: Reviews SetupForge changes for correctness, architecture, UI quality, Linux safety, testing, performance, accessibility, documentation, and accidental scope creep.
---

# SetupForge Code Review Skill

Use when reviewing a diff, pull request, completed feature, or agent-generated change.

## Review order

### 1. Scope

Check:

- does the change match the requested task?
- are unrelated files modified?
- are dependencies changed unnecessarily?
- are generated/debug artifacts present?

### 2. Architecture

Verify:

- UI does not execute system commands;
- application services own orchestration;
- domain boundaries remain stable;
- Fedora-specific behavior is isolated;
- infrastructure does not leak into presentation;
- abstractions are justified.

### 3. Safety

Look specifically for:

- silent privilege escalation;
- shell injection;
- arbitrary command execution;
- unvalidated package/application identifiers;
- unsafe file paths;
- unsafe extraction;
- untrusted downloads;
- missing destructive confirmation;
- missing cancellation;
- secret leakage;
- unsafe temporary files;
- destructive operations without recovery consideration.

Safety findings are high priority.

### 4. UI/UX

For UI changes check:

- information hierarchy;
- consistent navigation;
- visible focus;
- keyboard completeness;
- mouse behavior;
- small terminal sizes;
- loading/empty/error/success states;
- excessive decoration;
- inconsistent styles;
- color-only meaning;
- clipped content.

### 5. Correctness

Check:

- happy path;
- already-satisfied state;
- failure;
- cancellation;
- permission denial;
- malformed external output;
- network failure;
- repeated execution/idempotence.

### 6. Testing

Verify tests cover meaningful behavior.

Look for tests that accidentally:

- depend on the host;
- mutate the real package database;
- require network without isolation;
- rely on timing;
- assert fragile rendering details only.

### 7. Performance

Look for:

- blocking work on the Bubble Tea event loop;
- repeated expensive filesystem scans;
- unnecessary subprocesses;
- unbounded output buffering;
- unnecessary redraw churn;
- expensive work performed on every `View`.

### 8. Documentation

Check that user-visible behavior, security implications, and supported-platform assumptions are documented where necessary.

## Findings

Report findings by severity:

- **Critical** — security/data-loss/system integrity issue.
- **High** — correctness or architecture issue likely to break important behavior.
- **Medium** — meaningful defect or maintainability risk.
- **Low** — minor issue or polish.

Do not invent findings merely to produce a long review.

End with:

- verified strengths;
- findings;
- missing tests;
- remaining uncertainty.
