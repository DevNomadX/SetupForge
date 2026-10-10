# Security Policy

SetupForge is a system configuration application that executes package operations, service modifications, and privileged tasks on Fedora Linux workstations. We take the security and integrity of user systems seriously.

---

## Supported Versions

SetupForge is currently in active developer preview. Security updates are applied directly to the primary branch:

| Version | Supported | Notes |
|---|---|---|
| `main` | :white_check_mark: | Active development branch |
| `< 0.1.0` | :x: | Pre-alpha development snapshots |

---

## Reporting a Vulnerability

If you discover a security vulnerability or safety flaw in SetupForge, please practice **responsible disclosure**:

> [!CAUTION]
> **Do not open public GitHub issues or public pull requests for security vulnerabilities with exploit details.**

### How to Report
1. **GitHub Private Vulnerability Reporting (Preferred)**:
   Once the repository is published on GitHub, please submit vulnerability details privately via the **Security** tab → **Advisories** → **Report a vulnerability** feature.
2. **Direct Maintainer Contact**:
   If private vulnerability reporting is unavailable, reach out directly to the maintainer via their GitHub profile ([@DevNomadX](https://github.com/DevNomadX)).

### What to Include
To help us triage and resolve the issue efficiently, please provide:
- A clear description of the vulnerability and its potential impact.
- Step-by-step reproduction instructions or a minimal proof of concept.
- Affected components (e.g. `internal/infra/privilege`, `dnf5_planner`, journal, UI).
- Host environment details (Fedora version, DNF5 version, Go version).
- Any suggested remediations or patches, if available.

### Response & Handling
- **Triage**: As an open-source project maintained by community developers, we aim to acknowledge receipt of security reports in a timely manner.
- **Remediation**: We will investigate, develop a fix in private, and verify the patch against system safety invariants.
- **Coordinated Disclosure**: We will coordinate with the reporter on a timeline before public advisory release.

---

## Security Architecture & Threat Model

For details on SetupForge's architectural defense-in-depth, non-interactive privilege boundaries, structured argument execution, and transaction sealing, consult:

- [`docs/SECURITY.md`](docs/SECURITY.md) — Comprehensive Security Threat Model & Rationale
- [`.agents/rules/system-safety.md`](.agents/rules/system-safety.md) — Authoritative System Safety Invariants
