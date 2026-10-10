# Package Manager UI Specification

Derived from the supplied Fedora package inventory.

## Flow

`Categories → Software list → Details → Selection → Installation plan → Confirmation → Progress → Verification`

## Categories
Core; Shell; TUI Tools; File Managers; Video Players; Development; Git/GitHub; API/HTTP/Data; Databases; GNOME; GNOME Extensions; Browsers; Editors; Flatpak; Archive Tools; Repositories/Codecs.

## Item states

The final visual language must follow `docs/DESIGN-SYSTEM.md`.

Conceptual states:
- not selected
- selected
- installed
- requires attention
- failed

## Details

Show name, description, category, method, package/app ID, repository, privilege requirement, optional/hardware status, installed state, planned action, notes, and provenance.

## Installation plan

Do not immediately execute selections. Build a plan and group it into:
1. repository changes
2. package installation
3. Flatpak installation
4. user-space installers
5. configuration
6. service actions

Then confirm.

## Progress

Show per-item status, current operation, progress where available, logs/details, and safe cancellation.

Final states must distinguish completed, cancelled, failed, and partially completed.

## Search

Search display name, package name, Flatpak ID, category, and description.

## Installed-state detection

Query the real system. Package installed and service running/enabled are separate states.

## Boundary

Package UI must never directly execute `dnf`, `flatpak`, `rpm`, `curl | sh`, or service commands. It calls application services.
