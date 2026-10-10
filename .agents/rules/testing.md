---
trigger: model_decision
description: "Testing standards for SetupForge Go code, TUI state, system adapters, command execution, and Fedora integration."
---

# Testing Rules

Testing must maximize confidence without requiring every test to mutate a developer's real Fedora installation.

## Test layers

### Unit tests

Use for:

- domain validation;
- state transitions;
- command argument construction;
- configuration parsing;
- package/application identifier validation;
- formatting and layout calculations;
- error mapping.

Unit tests must be deterministic.

### Application/service tests

Use fakes for:

- package managers;
- filesystem operations;
- privilege providers;
- service managers;
- download clients;
- OS detectors.

Verify orchestration, confirmation policy, sequencing, cancellation, and failure handling.

### UI state tests

Test meaningful model transitions:

- navigation;
- selection;
- focus;
- confirmation;
- async progress;
- success/failure;
- cancellation;
- resize behavior.

Do not make rendering snapshots the only form of UI testing.

### Integration tests

Use controlled environments for:

- Fedora detection;
- DNF adapter behavior;
- systemd adapter behavior;
- filesystem integration;
- real command execution where safe.

Integration tests that modify the system should be explicitly marked and documented.

## Command execution tests

Test the exact executable and argument vector passed to the executor.

Do not test by asserting a single concatenated shell string when the production implementation uses structured arguments.

Cover:

- normal success;
- non-zero exit;
- missing executable;
- permission failure;
- cancellation;
- timeout;
- malformed output;
- partial output;
- stderr diagnostics.

## System mutation tests

Never make ordinary unit tests install/remove packages on the developer's machine.

Use:

- fake executors;
- fake package managers;
- temporary directories;
- containers/VMs when available;
- dedicated integration environments.

## Failure scenarios

Every non-trivial system operation should consider:

- no network;
- repository unavailable;
- package not found;
- package already installed;
- package already absent;
- permission denied;
- operation cancelled;
- conflicting package transaction;
- invalid configuration;
- unexpected command output;
- interrupted operation.

## Terminal dimensions

Manually or programmatically exercise at least:

- 80×24;
- 100×30;
- 120×35;
- 160×45+.

Check for clipping, overflow, unreadable text, broken borders, and inaccessible controls.

## Verification reporting

When reporting work, distinguish:

- tests passed;
- tests not run;
- manual verification performed;
- environment-specific checks unavailable.

Never state "fully tested" when only unit tests were run.
