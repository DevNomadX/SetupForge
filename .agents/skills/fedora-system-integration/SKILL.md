---
name: fedora-system-integration
description: Designs and implements Fedora-specific SetupForge system integration, including OS detection, DNF/package operations, Flatpak, repositories, systemd, files, privileges, downloads, and safe command execution.
---

# Fedora System Integration Skill

Use for Fedora/Linux integration or any feature that changes or interrogates the host system.

## Mandatory preparation

Read:

- `.agents/rules/architecture.md`
- `.agents/rules/system-safety.md`
- `.agents/rules/testing.md`

Inspect existing system-operation abstractions before adding new ones.

## Fedora scope

The first release targets Fedora.

Do not add Ubuntu/Debian/Arch/openSUSE behavior unless the task explicitly requests it.

Do isolate Fedora-specific behavior behind meaningful interfaces so future implementations can be added without changing UI/application policy.

## Detection

OS detection must be explicit and testable.

Do not infer Fedora merely because a command happens to exist.

Expose structured platform information such as:

- distribution;
- release/version;
- architecture;
- package manager capabilities;
- desktop environment where relevant.

Handle unknown/unsupported systems safely.

## Package management

Package operations should be represented as structured requests.

Conceptually:

```text
PackageService
  → PackageManager interface
    → Fedora DNF adapter
```

The UI must never construct DNF commands.

Consider:

- install;
- remove;
- upgrade/update;
- search;
- info;
- installed-state query.

Operations should be idempotent where meaningful.

Before mutation:

- validate identifiers;
- determine whether the requested state is already satisfied;
- produce a clear plan/preview when useful;
- request confirmation for destructive operations.

## DNF integration

Treat DNF output and behavior as external dependencies.

Do not make business logic depend on fragile human-readable output when a stable machine-readable mode/API is available.

Capture:

- command/operation identity;
- exit status;
- stdout/stderr as appropriate;
- cancellation;
- useful diagnostics.

Test the adapter with fakes and controlled integration tests.

## Flatpak

Treat Flatpak as a separate package/application backend rather than pretending it is DNF.

Use structured application IDs where available.

Do not silently mix package sources in a single action.

Clearly identify installation source to users.

## Repositories

Repository changes are high-impact.

Show:

- repository identity;
- source;
- enabled/disabled state;
- effect of changing it.

Require explicit confirmation for enabling/disabling/removing repositories when the operation can alter package trust or system behavior.

Never add arbitrary repository URLs from untrusted configuration without validation and explicit user intent.

## Applications

"Application installation" is a policy layer above concrete installers.

Possible sources include:

- Fedora packages;
- Flatpak;
- approved external repositories;
- other explicitly supported mechanisms.

Each source must have a defined trust model and validation strategy.

Do not turn arbitrary URLs into executable installers.

## Fonts

Fonts are user/system assets, not ordinary packages.

Separate:

- discovery;
- download;
- validation;
- installation;
- refresh/update;
- removal.

Prefer user-local installation when system-wide installation is not required.

Prevent archive path traversal and unsafe overwrites.

## systemd

Represent operations structurally:

- unit;
- action;
- scope;
- expected privilege;
- current state;
- result.

Do not expose arbitrary systemctl command construction to UI.

## Files and configuration

Use safe filesystem APIs.

For writes:

- determine target path explicitly;
- validate parent/target;
- preserve backups when appropriate;
- use restrictive permissions for sensitive data;
- write atomically where practical;
- handle partial failure.

## Privileges

Never silently elevate.

The service layer should know whether an operation requires privileges before execution and communicate that to the UI.

## External command abstraction

Use a controlled executor abstraction so unit tests can replace real subprocesses.

The executor should support:

- context cancellation;
- argument arrays;
- stdout/stderr capture;
- exit status;
- environment control;
- working directory;
- optional stdin policy.

Do not allow arbitrary caller-provided shell strings through the abstraction.

## Verification

Before declaring a Fedora feature complete:

- unit test service policy;
- test adapter argument construction;
- test failure/cancellation;
- test permission failures;
- run appropriate Fedora integration tests in a controlled environment;
- document Fedora version assumptions.
