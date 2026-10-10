---
trigger: model_decision
description: "Go, Bubble Tea, Bubbles, and Lip Gloss implementation rules for SetupForge."
---

# Go + Bubble Tea Rules

## Go

Follow standard Go conventions and the repository's formatter/linter configuration.

Prefer:

- small cohesive packages;
- explicit error handling;
- context propagation for cancellable work;
- typed domain errors where useful;
- dependency injection at real boundaries;
- standard library facilities before new dependencies.

Avoid:

- global mutable state;
- hidden goroutines;
- `panic` for ordinary runtime failures;
- reflection-heavy abstractions;
- giant god structs;
- giant switch statements that combine navigation, business logic, and system operations.

## Bubble Tea

Keep the Bubble Tea model responsible for UI state and message handling.

Use the standard model lifecycle:

- `Init` for initial commands;
- `Update` for messages/state transitions;
- `View` for rendering.

Do not put blocking system work directly in `Update`.

Long operations should run outside the event loop and return structured messages.

Prefer explicit message types over ambiguous strings.

Commands should represent intent and return results. They should not silently mutate unrelated state.

## Model composition

For complex screens, compose focused child models/components.

A parent model should own:

- page-level navigation;
- focus routing;
- lifecycle;
- orchestration.

A child model should own:

- its local interaction state;
- local rendering;
- component-specific key handling.

Do not create child models merely to split a few lines of code.

## Bubbles

Use an existing Bubbles component when it closely matches the interaction.

Common choices include:

- list: searchable/selectable collections;
- table: structured rows/columns;
- viewport: long scrollable content;
- textinput: single-line input;
- textarea: multi-line input;
- progress: determinate progress;
- spinner: indeterminate activity;
- paginator: page navigation;
- help: contextual key help;
- filepicker: filesystem selection.

Customize components through supported APIs rather than forking them.

## Lip Gloss

Centralize styles. A screen should consume semantic styles rather than invent raw colors.

Use Lip Gloss for:

- hierarchy;
- spacing;
- borders;
- alignment;
- responsive styling;
- terminal-safe colors.

Avoid color as the sole semantic signal.

## Messages and effects

Distinguish:

- user input;
- window-size changes;
- async operation progress;
- async operation completion;
- errors;
- notifications;
- navigation intents.

Do not make arbitrary side effects happen merely because a view was rendered.

## Concurrency

Do not update UI state from arbitrary goroutines.

Use Bubble Tea messages to return results to the model.

All subprocesses and network operations must have clear ownership and cancellation behavior.

## Testing

Test:

- state transitions;
- command construction;
- service behavior;
- failure paths;
- boundary dimensions;
- key handling where practical.

Do not rely exclusively on snapshot tests for behavior.
