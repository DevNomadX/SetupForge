---
trigger: model_decision
description: "Safety constraints for Fedora/Linux system modifications, privileged commands, package installation, downloads, files, services, and configuration."
---

# Linux System Safety Rules

SetupForge can change a user's operating system. Safety takes priority over implementation convenience.

## Privilege

Never silently elevate privileges.

Before a privileged operation:

1. identify that privilege is required;
2. tell the user what will change;
3. show the relevant operation in human-readable form;
4. require explicit confirmation when the action is consequential;
5. execute through a controlled adapter;
6. report success/failure clearly.

Do not ask the user to paste passwords into SetupForge.

Prefer established privilege mechanisms and non-interactive operation modes where safe and appropriate.

## Destructive actions

Explicit confirmation is required for operations such as:

- removing packages;
- disabling/removing services;
- deleting user/system configuration;
- overwriting files;
- replacing repositories;
- destructive cleanup;
- reset/restore actions that overwrite current configuration.

Confirmation text must identify the actual effect. Avoid vague "Are you sure?" prompts.

For high-impact actions, provide a review step or dry-run when feasible.

## Command execution

Never construct shell command strings from untrusted input and execute them through a shell.

Prefer:

- executable path + argument array;
- controlled environment;
- explicit working directory;
- context cancellation;
- captured stdout/stderr;
- exit-code inspection.

Avoid `sh -c`/`bash -c` unless shell semantics are genuinely required and the input is fully controlled. If shell execution is unavoidable, isolate and justify it.

Configuration files must never become a general-purpose shell script.

## Validation

Validate:

- package names;
- application identifiers;
- font identifiers;
- repository identifiers;
- filesystem paths;
- URLs;
- service names;
- configuration values.

Validation is not only a UI concern. Enforce it again at the service/infrastructure boundary.

Prefer allowlists or structured identifiers where the supported universe is known.

## Downloads

For remote downloads:

- use HTTPS where supported;
- validate URLs against the intended source policy;
- use safe temporary directories;
- avoid predictable temporary filenames;
- set reasonable permissions;
- verify expected content/type where practical;
- validate archives before extraction;
- prevent path traversal during extraction;
- do not execute downloaded content automatically;
- preserve enough metadata for troubleshooting.

Do not add "curl | sh" style installation paths.

## Filesystem

Use appropriate permissions.

Do not write secrets to world-readable files.

Do not follow untrusted symlinks blindly when performing privileged filesystem operations.

Avoid recursive deletion unless the target is explicitly constrained and verified.

## Services/systemd

Service operations must identify:

- unit;
- requested action;
- current state when known;
- whether root is required;
- whether the action persists across reboot.

Do not disable or mask services merely to make a feature work unless that is an explicit user-selected action.

## Recovery

For multi-step operations:

- define what can be rolled back;
- preserve backups where practical;
- stop safely after a failed prerequisite;
- do not claim atomicity where it does not exist;
- report partial completion accurately.

A rollback must not be more destructive than the original operation.

## Logging and secrets

Never log:

- passwords;
- authentication tokens;
- private keys;
- secret environment values.

Redact sensitive arguments and environment variables.

System command logs should contain enough context to diagnose failures without exposing secrets.

## Dry-run

Use dry-run/preflight behavior where the underlying tool supports it or where SetupForge can safely model the intended operation.

A dry run must not secretly perform the operation.
