---
trigger: always_on
description: "SetupForge architecture boundaries, layering, dependency direction, and Fedora isolation."
---

# Architecture Rules

## Core dependency direction

The preferred dependency graph is:

```text
Bubble Tea / TUI
      ↓
Application services / use cases
      ↓
Domain interfaces + models
      ↓
Infrastructure / platform adapters
      ↓
Linux / Fedora / external tools
```

Dependencies must point inward toward stable concepts. Infrastructure must not leak implementation details into presentation code.

## Layer responsibilities

### Presentation

Responsible for:

- Bubble Tea models and messages;
- focus and navigation;
- rendering;
- user input;
- mapping service results into view state;
- presentation-level validation;
- dialogs and notifications.

Must not:

- invoke DNF;
- invoke `sudo`;
- mutate `/etc`, system configuration, or user configuration directly;
- perform distro detection;
- own package-manager policy;
- parse arbitrary command output for business rules when an application/infrastructure boundary should do it.

### Application

Responsible for:

- use-case orchestration;
- sequencing operations;
- policy decisions;
- confirmation requirements;
- progress events;
- cancellation propagation;
- translating infrastructure failures into useful application errors.

Application services should be testable with fakes.

### Domain

Responsible for:

- stable models;
- interfaces;
- value objects;
- operation results;
- domain-level validation and invariants.

Keep the domain independent of Bubble Tea and OS command implementations.

### Infrastructure

Responsible for:

- command execution;
- filesystem operations;
- package managers;
- repository APIs;
- systemd interaction;
- OS detection;
- network/download behavior;
- Fedora-specific adapters.

Infrastructure code must expose safe, typed operations to the application layer rather than making the UI understand commands.

## Fedora-first, not Fedora-everywhere

The first release may contain only Fedora implementations.

Do not create fake implementations for Ubuntu, Debian, Arch, or openSUSE merely to demonstrate extensibility.

Do create boundaries where platform variance is real, such as:

- `PackageManager`;
- `OSDetector`;
- `ServiceManager`;
- `RepositoryManager`;
- `ApplicationInstaller`;
- `SystemConfigurator`.

Prefer capability interfaces over a giant `Distribution` interface.

Do not force all distributions to support every capability.

## Avoid abstraction theater

Do not create an interface solely because "interfaces are good."

Create an abstraction when at least one is true:

- a platform boundary exists;
- a component has multiple plausible implementations;
- testing requires substitution;
- the boundary protects safety;
- the boundary isolates unstable external behavior.

A single internal helper does not automatically need an interface.

## State ownership

Each piece of mutable state must have one clear owner.

Do not duplicate authoritative state between:

- parent and child models;
- services and UI models;
- cache and domain model;
- configuration and runtime state.

Use explicit messages/events for cross-component communication.

## Long-running operations

Never block the Bubble Tea event loop with slow package installation, network activity, filesystem scans, or system commands.

Represent asynchronous work as commands/tasks that report structured progress/results back to the UI.

The UI must remain able to:

- render;
- accept safe navigation;
- show progress;
- surface cancellation;
- show errors.

## Configuration and persistence

Separate:

- configuration schema;
- runtime state;
- secrets;
- operation history/logs.

Configuration must not become an arbitrary command-execution language.

Any configuration that selects packages, applications, fonts, repositories, or actions must be validated against an explicit schema and safe operation model.

## File organization

Follow the existing repository structure once established. Do not reorganize the project merely to match a preferred architecture.

When a new package is needed, keep its public API narrow and document why it exists.
