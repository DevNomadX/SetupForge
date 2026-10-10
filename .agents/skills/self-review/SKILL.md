---
name: self-review
description: Mandatory pre-completion audit of your own non-trivial SetupForge change covering architecture, UI/design-system conformance, safety, testing, and scope.
---

# SetupForge Self-Review Skill

Run before declaring any non-trivial task complete. For trivial one-line
changes, a quick mental pass over sections 2–5 suffices — say so in your report.

Read your actual diff, not your memory of it.

## 1. Architecture

- Did I preserve the UI → service → domain → infrastructure boundary?
- Did UI code gain any `sudo`/DNF/shell/filesystem/distro-detection logic? (Critical if yes.)
- Did I create an interface without a real boundary, second implementation, or test need?
- Did I duplicate functionality that already existed?
- Is each piece of mutable state owned by exactly one layer/model?

## 2. UI / design system

- Does every new style come from `docs/DESIGN-SYSTEM.md` tokens, not raw values?
- Does the screen follow the spatial system — panels feel like instances of one
  design language, not individually sized boxes?
- Is decoration intentional (identity, hierarchy, state, atmosphere) rather than noise — or absent for its own sake?
- Is focus visible? Are selected vs. focused distinguishable? Is any meaning color-only?
- Did I check 80×24, 100×30, 120×35, 160×45+? Does it degrade ("simpler under pressure, not broken")?
- Did the screen accidentally invent a new visual language or become a generic box layout?
- Keyboard path complete without a mouse?

## 3. Safety

- Are privilege requirements explicit and communicated before execution?
- Is every destructive action behind a specific (not vague) confirmation?
- Is all external input validated at UI *and* service/infrastructure boundaries?
- Any shell-string construction from untrusted input? Any `curl | sh` path?
- Cancellation, typed errors, dry-run/preflight, recovery considered where applicable?
- No secrets in logs; safe temp files; no path traversal in extraction?

## 4. Testing

- Are claims backed by actually-run tests? State precisely what ran and what didn't.
- Do tests use fakes/mocks so they never mutate the developer's real system?
- Are failure paths covered (permission denied, cancellation, network, malformed output)?
- Did I distinguish implemented / tested / manually verified / not verified?

## 5. Scope

- Only task-relevant files changed? No drive-by reformatting, renames, or dependency bumps?
- No fake multi-distro implementations, no speculative abstractions?

## Report

End your task summary with: strengths, findings by severity (Critical/High/
Medium/Low), missing tests, remaining uncertainty. If any Critical/High
finding exists, fix it before declaring completion.
