# SetupForge Security Model

> **Role of this document.** This file explains *why* SetupForge's safety
> model is designed the way it is — the threat model and rationale.
> The enforceable constraints live in `.agents/rules/system-safety.md`,
> which is authoritative. If this document and the rule ever disagree,
> the rule wins and this document should be corrected. Implementation
> guidance lives in `.agents/skills/fedora-system-integration/SKILL.md`.

SetupForge is a system configuration application and therefore has a larger
security impact than a normal terminal utility: it installs packages, changes
repositories, writes configuration, and manages services on a real machine.

## Principles

- least privilege;
- explicit user intent;
- structured command execution;
- strict input validation;
- safe filesystem handling;
- trusted-source boundaries;
- minimal logging of sensitive data;
- recoverable operations where practical.

## Threat model

SetupForge's main risks, in rough priority order:

1. **Arbitrary command execution** — untrusted input (typed, pasted, or from
   imported configuration) becoming shell syntax.
2. **Silent privilege escalation** — operations running as root the user did
   not understand or approve.
3. **Destructive changes without real consent** — removals, overwrites,
   repository changes confirmed by vague prompts.
4. **Untrusted content execution** — downloaded files or archives running
   code or escaping their extraction target (path traversal).
5. **Supply-chain expansion** — third-party repositories/COPRs or installers
   quietly becoming trusted package sources.
6. **Secret exposure** — credentials leaking into logs or world-readable files.

## Privileged operations

SetupForge must know *before execution* whether an operation requires
elevated privileges, tell the user, and obtain explicit confirmation for
consequential actions. It must never silently escalate, and it must never ask
the user to paste a password into the TUI.

## Command injection

User input and imported configuration must never become arbitrary shell
syntax. Use structured executable/argument execution (`exec.Command`-style),
never shell-string interpolation of untrusted data. Configuration describes
supported SetupForge operations; it is not a script language and must not
become one.

## Downloads

Downloads are untrusted until validated:

- HTTPS only where supported; validate URLs against the intended source policy;
- safe temporary directories, unpredictable filenames, reasonable permissions;
- verify expected content/type where practical; validate archives before
  extraction; block path traversal during extraction;
- never execute downloaded content automatically;
- no "curl | sh" installation paths, ever.

## Package/application sources

Each installation source (Fedora packages, Flatpak, COPRs, external
repositories, vendor installers) needs an explicit, documented trust model.
Adding a repository changes which packages the system trusts — treat enabling,
disabling, or replacing repositories as high-impact and confirm explicitly.
Never add arbitrary repository URLs from untrusted configuration.

## Configuration

Imported configuration can influence system changes, so validate it against
an explicit schema before use. Unknown operations in an imported file must be
rejected, not executed.

## Filesystem

Restrictive permissions for sensitive data, no world-readable secrets, no
blind symlink following during privileged operations, no recursive deletion
unless the target is explicitly constrained and verified, atomic writes where
practical.

## Logs

Never store passwords, tokens, private keys, or sensitive environment values.
Redact sensitive arguments. Logs must contain enough context to diagnose
failures without exposing secrets.

## Recovery

Multi-step operations define what can be rolled back, preserve backups where
practical, stop safely after a failed prerequisite, and report partial
completion accurately. A rollback must never be more destructive than the
original operation.

## Reporting

Security-sensitive defects should be handled through the project's responsible disclosure process. Consult [`SECURITY.md`](../SECURITY.md) for vulnerability reporting guidelines and contacts. Do not file public issues with exploit details.
