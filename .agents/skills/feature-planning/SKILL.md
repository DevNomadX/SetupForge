---
name: feature-planning
description: Plans SetupForge features before implementation by inspecting the repository, defining requirements, architecture boundaries, UI/UX flows, safety behavior, tests, documentation, and an incremental implementation plan.
---

# Feature Planning Skill

Use before implementing a new feature or a change spanning multiple layers.

## 1. Inspect first

Search the repository for:

- existing related services;
- domain models;
- interfaces;
- UI components;
- styles;
- tests;
- configuration;
- logging;
- documentation.

Do not propose a new abstraction until checking whether one already exists.

## 2. Define the feature

Write:

- problem;
- user goal;
- in-scope behavior;
- out-of-scope behavior;
- success criteria;
- constraints;
- dependencies;
- safety impact.

Keep roadmap items out of the current implementation unless needed.

## 3. Architecture

Map the feature through:

```text
UI
 ↓
Application service/use case
 ↓
Domain interface/model
 ↓
Infrastructure adapter
```

Identify exactly which layer owns each responsibility.

List new interfaces only where justified.

## 4. UX

For UI work use the UI/UX skill.

Produce:

- information architecture;
- navigation changes;
- user flow;
- wireframe;
- state matrix;
- keyboard/mouse behavior;
- responsive behavior.

## 5. Safety

For system-changing features identify:

- privilege requirements;
- destructive actions;
- confirmation points;
- input validation;
- download trust;
- cancellation;
- rollback/recovery;
- logging/redaction.

## 6. Testing plan

Define:

- unit tests;
- service tests with fakes;
- UI state tests;
- integration tests;
- Fedora-specific verification;
- failure scenarios;
- terminal-size checks.

## 7. Documentation

Identify updates needed for:

- user documentation;
- architecture docs;
- supported Fedora versions;
- security docs;
- troubleshooting;
- changelog.

## 8. Implementation sequence

Prefer vertical slices.

Example:

1. domain model/interface;
2. fake/test contract;
3. Fedora adapter;
4. application service;
5. UI state;
6. UI rendering;
7. error/cancellation;
8. integration tests;
9. documentation;
10. review.

Avoid building an empty abstraction framework before the first useful behavior.

## 9. Plan output

End with:

- files to add/change;
- responsibilities;
- dependencies;
- tests;
- docs;
- risks;
- explicit non-goals.

Do not implement code while operating in planning mode.
