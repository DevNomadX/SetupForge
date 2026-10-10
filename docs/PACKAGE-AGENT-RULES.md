# Package Catalog Usage Rules for Agents

Before implementing package/software installation, read:
- `data/fedora/software-catalog.md`
- `data/fedora/repositories.md` when repositories are involved
- `data/fedora/installation-methods.md`
- `docs/PACKAGE-UI.md`
- applicable architecture, safety, testing, UI, and project-state docs

## Required behavior

Search the existing repository first and reuse existing abstractions.

Do not:
- hard-code package commands inside Bubble Tea views;
- duplicate package entries;
- execute arbitrary catalog command strings;
- silently enable repositories;
- silently start services;
- silently execute remote shell scripts;
- assume Flatpak availability;
- install hardware-specific packages unconditionally;
- remove GCC because Clang is preferred;
- ignore user-choice groups;
- claim success without verification.

## Architecture

Conceptually:

`TUI → Application service → Installation planner → Package/Repository/Flatpak/Script services → Command runner → Fedora`

The TUI must never directly invoke system commands.

## Testing

Package logic should use mocks/fakes. Real Fedora integration tests must be isolated and must not unintentionally modify the developer's machine.

## Source verification

The supplied package list is project input, not a guarantee that every package name or installer command remains current. When current external facts are required, verify official Fedora/vendor sources and update the catalog/documentation rather than silently changing the requested inventory.
