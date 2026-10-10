---
trigger: model_decision
description: "Git, commit, branch, pull-request, changelog, and change-scope conventions for SetupForge."
---

# Git and Contribution Rules

## Branches

Use short, descriptive branch names.

Preferred forms:

- `feat/<short-description>`
- `fix/<short-description>`
- `refactor/<short-description>`
- `docs/<short-description>`
- `test/<short-description>`
- `chore/<short-description>`

Do not create branch names containing secrets or unnecessary issue details.

## Commits

Prefer small, logically focused commits.

Commit messages should communicate intent, for example:

- `feat: add package search service`
- `fix: handle cancelled dnf transaction`
- `refactor: isolate Fedora package adapter`
- `test: cover package validation`
- `docs: document system operation safety`

Do not combine unrelated refactors with feature changes.

## Pull requests

A PR should explain:

- what changed;
- why it changed;
- architecture impact;
- safety impact;
- UI impact;
- tests run;
- manual verification;
- known limitations.

For UI changes include terminal-size screenshots or recordings when the project workflow supports them.

For system integration include the Fedora version/environment used.

## Changelog

User-visible changes should be represented in the project's changelog/release notes according to repository convention.

Mention:

- new capabilities;
- behavior changes;
- safety changes;
- breaking changes;
- supported Fedora/version changes.

## Agent changes

An AI agent must not rewrite history, force-push, delete branches, or modify unrelated commits unless the user explicitly requests it.

Do not commit automatically unless the task explicitly asks for a commit.

## Review scope

Review the actual diff, not only the files the agent remembers changing.

Look for:

- accidental formatting;
- generated files;
- debug output;
- secrets;
- unrelated modifications;
- architectural boundary violations.
