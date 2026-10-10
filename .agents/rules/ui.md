---
trigger: glob
globs: "**/*.go"
description: "UI constraints for Bubble Tea TUI work: terminal-first interaction, responsive layouts, accessibility, and shared design language."
---

# TUI UI Rules

These rules apply whenever Go UI code is being edited.

## Terminal-first design

SetupForge is a TUI, not a web page rendered in text.

Never assume:

- a mouse;
- a fixed viewport;
- true color;
- Unicode support beyond reasonable terminal capabilities;
- large screen dimensions;
- persistent visual space.

Keyboard interaction is the primary interaction model. Mouse support is a complement.

## Screen contract

A normal application screen should have a predictable structure:

1. application/header context;
2. navigation/content region;
3. contextual actions or status;
4. compact footer/help where useful.

Not every screen needs every region. Avoid decorative regions that consume space without helping the task.

## Navigation

Use a stable navigation model across the application.

Users should be able to determine:

- where they are;
- what has focus;
- what can be selected;
- how to go back;
- how to search/filter;
- how to open help;
- how to invoke relevant actions.

Do not make every screen invent different keybindings for the same concept.

## Responsive behavior

At minimum reason about:

- 80×24;
- 100×30;
- 120×35;
- 160×45+.

At small sizes:

- remove secondary decoration;
- reduce explanatory copy;
- collapse or stack panels;
- preserve primary action and context;
- use scrolling rather than clipping;
- avoid tiny unreadable columns.

Never allow important controls to render outside the viewport.

## Component consistency

Prefer shared components/styles for:

- page headers;
- panels;
- status badges;
- selection/focus;
- dialogs;
- notifications;
- progress;
- tables;
- lists;
- footer help.

Do not duplicate styling logic across screens.

## State visibility

Every asynchronous or potentially destructive operation needs an explicit state model.

At minimum consider:

- idle;
- loading/preparing;
- running;
- cancellation requested;
- success;
- recoverable failure;
- unrecoverable failure.

Avoid ambiguous "busy" indicators with no explanation.

## Accessibility

Never encode meaning by color alone.

Every important semantic state must have textual or symbolic reinforcement:

- success;
- warning;
- error;
- disabled;
- selected;
- focused.

Respect terminal contrast and color-profile limitations.

Avoid excessive animation, blinking, or rapidly changing content.

## TUI-specific anti-patterns

Avoid:

- web-style card grids that collapse badly;
- huge hero banners;
- nested modal stacks;
- excessive rounded boxes;
- dense dashboards with no task hierarchy;
- mouse-only affordances;
- hover-only information;
- horizontal scrolling for routine tasks when a better layout is possible;
- decorative Unicode that makes alignment unreliable;
- fixed-width assumptions.

## Rendering

Calculate dimensions from the current terminal size.

Do not scatter magic width/height constants throughout rendering code.

Use display width rather than byte length when aligning terminal text.

Keep rendering pure where practical: rendering should consume state and produce a view, not mutate system state.
