---
name: ui-ux-design
description: Designs and reviews SetupForge terminal interfaces before implementation, including information architecture, navigation, wireframes, component composition, responsive layouts, keyboard and mouse interaction, accessibility, states, and visual hierarchy.
---

# SetupForge UI/UX Design Skill

Use this skill before implementing a new screen, major navigation change, complex dialog, dashboard, workflow, or substantial visual redesign.

This skill is for a terminal application. Do not translate web/mobile UI patterns literally.

## Goal

Produce a clear terminal-native design that minimizes cognitive load while preserving discoverability, safety, and efficiency.

## Required workflow

Follow:

`requirements → information architecture → UX flow → wireframe → component selection → state model → implementation guidance → terminal-size review → interaction review`

Do not jump directly from a feature request to Bubble Tea code.

## 1. Requirements

Identify:

- user goal;
- primary action;
- secondary actions;
- information required to make a decision;
- destructive/consequential actions;
- long-running operations;
- likely errors;
- cancellation points;
- empty/loading/success/error states.

Separate "must see" from "nice to see."

## 2. Information architecture

Decide:

- where the feature belongs in global navigation;
- whether it is a page, sub-page, tab, dialog, command-palette action, or contextual action;
- what concepts belong together;
- what should remain persistent across navigation.

Prefer shallow navigation.

A user should not need to remember a deep hierarchy to perform common tasks.

## 3. Navigation

Define:

- primary navigation;
- local navigation;
- back behavior;
- focus order;
- keyboard shortcuts;
- mouse targets;
- search/filter behavior;
- contextual actions.

Keep global shortcuts stable.

Do not overload common keys with screen-specific meanings unless context makes the behavior obvious.

## 4. Wireframes

Produce an ASCII wireframe before implementation for non-trivial screens.

Example:

```text
┌─────────────────────────────────────────────────────────────┐
│ SetupForge                         Fedora Linux  •  Ready   │
├──────────────┬──────────────────────────────────────────────┤
│ Dashboard    │ Packages                                     │
│ Packages  ●  │ ┌──────────────────────────────────────────┐ │
│ Apps         │ │ Search packages...                    / │ │
│ Fonts        │ └──────────────────────────────────────────┘ │
│ Development  │                                              │
│ Shell        │  Name                  Version       State   │
│ Desktop      │  ─────────────────────────────────────────  │
│ Services     │  git                   2.x           ✓       │
│ Settings     │  ripgrep               14.x          ✓       │
│ Logs         │                                              │
│              │  [Install] [Remove] [Details]               │
├──────────────┴──────────────────────────────────────────────┤
│ ↑↓ Navigate  Enter Select  / Search  Esc Back  Ctrl+K Menu  │
└─────────────────────────────────────────────────────────────┘
```

Treat this as a design artifact, not a fixed implementation.

## 5. Component selection

Prefer semantic components:

- list for navigable collections;
- table for comparable structured data;
- viewport for long text;
- text input for search/name input;
- textarea for multi-line configuration;
- progress for determinate progress;
- spinner for indeterminate work;
- help for contextual shortcuts;
- dialog/modal for focused confirmation or short decisions.

Do not use a table when the user really needs a selectable list.

Do not use a modal when inline contextual feedback would be clearer.

## 6. State model

For each screen define:

- initial state;
- loaded state;
- focused state;
- selection state;
- loading state;
- empty state;
- success state;
- recoverable error;
- fatal error;
- cancellation state;
- confirmation state.

For system-changing screens, explicitly distinguish "preview" from "execute."

## 7. Dialogs and confirmation

A destructive confirmation must state:

- what will happen;
- what object(s) are affected;
- whether elevated privileges are required;
- whether the action is reversible.

Example:

```text
┌─ Remove package ─────────────────────────────┐
│ Remove `example-package` from this system?   │
│                                               │
│ This changes installed system packages and    │
│ requires administrator privileges.            │
│                                               │
│ [Cancel]                         [Remove]     │
└───────────────────────────────────────────────┘
```

Do not make the destructive action the easiest accidental key.

## 8. Progress and async operations

For long-running operations communicate:

1. what is happening;
2. which operation is active;
3. progress when measurable;
4. whether cancellation is available;
5. what completed;
6. what failed.

Avoid fake percentages when no reliable progress exists.

Use stages when exact progress is unavailable.

## 9. Notifications

Use notifications for transient, low-complexity feedback.

Use persistent inline errors for problems that require user action.

Do not make important errors disappear before the user can understand them.

## 10. Responsive design

Design explicitly for:

### 80×24

Prioritize:

- navigation;
- current context;
- primary content;
- primary action;
- essential status.

Hide or collapse secondary metadata.

### 100×30

Allow one compact secondary region where useful.

### 120×35

Normal target layout.

### 160×45+

Use additional whitespace and secondary information without making the UI feel sparse.

Never simply scale everything upward. Use the extra space intentionally.

## 11. Focus and keyboard interaction

Every actionable element must have a visible focus state.

Define:

- Tab/Shift+Tab behavior;
- arrow navigation;
- Enter/Space behavior;
- Esc behavior;
- global quit/help/command palette behavior;
- search/filter shortcuts where relevant.

Do not require the mouse for any core workflow.

## 12. Mouse interaction

Mouse support should complement keyboard workflows.

Clickable elements must have adequate target size and clear visual affordance.

Never make hover the only way to discover an action.

## 13. Visual hierarchy

Use hierarchy through:

1. placement;
2. spacing;
3. weight;
4. borders;
5. semantic color.

Do not solve every hierarchy problem with color.

Limit competing emphasis.

## 14. TUI-native cognitive load reduction

Prefer:

- progressive disclosure;
- concise labels;
- contextual help;
- predictable navigation;
- meaningful defaults;
- grouped actions;
- clear state;
- short confirmation text.

Avoid:

- decorative panels everywhere;
- redundant labels;
- repeated instructions;
- unnecessary animations;
- excessive nesting;
- long paragraphs in small viewports.

## 15. Accessibility review

Check:

- can the workflow be completed without a mouse?
- does focus remain visible?
- does every color-coded state have text/symbol support?
- does the layout remain readable with reduced color capability?
- are status changes understandable without animation?
- are keyboard shortcuts discoverable?
- can users recover from mistakes?

## Deliverable before implementation

For a major screen, provide:

1. purpose;
2. information architecture;
3. user flow;
4. wireframe(s);
5. responsive behavior;
6. component mapping;
7. state matrix;
8. key/mouse interactions;
9. accessibility notes;
10. unresolved UX questions.

Only then proceed to implementation.
