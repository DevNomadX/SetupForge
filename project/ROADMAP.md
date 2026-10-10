# SetupForge — Project Roadmap

This file is the canonical product roadmap and implementation phase tracking for SetupForge.
Scope for any task is chosen through the feature planning workflow. Agents must not implement roadmap items that were not requested.

---

## Phase Checklist

### Phase 1 — Foundation [COMPLETED]
- [x] Application shell (frame, header, rail, dock, stage)
- [x] 12-core keyboard navigation & single-focus model
- [x] Design token system & M-unit spatial grid
- [x] Theme system (Forge default, Rosé Pine, Gruvbox Dark)
- [x] Domain capability contracts & models (OSDetector, SystemInfo)
- [x] Infrastructure Fedora host detector (/etc/os-release parser)
- [x] Application service layer (SystemService)
- [x] Overview workbench landing page with live host detection
- [x] Responsive layout verification across 80×24, 100×30, 120×35, 160×45+

### Phase 2 — System Updates & Services [IN PROGRESS]
- [x] Phase 2A: System update status engine (DNF5 check-upgrade structured parsing & header badge)
- [x] Phase 2B.1: Transaction artifact model & DNF5 update planner (unprivileged --store, fingerprinting)
- [x] Phase 2B.2: Update plan domain presentation & user confirmation modal
- [x] Phase 2B.4 Slice 1: Privileged execution boundary (`SudoRunner`, `sudo -n true`, artifact revalidation, durable `EXECUTING`, process group isolation)
- [ ] Phase 2B.4 Slice 2: DNF5 transaction history inspection, truthful outcome reconciliation & progress feedback
- [ ] Phase 2B.6: Journal integration & end-to-end Fedora validation
- [ ] Phase 2C: Systemd service status inspection & management
- [ ] Phase 2D: Hardware & system diagnostics detail view

### Phase 3 — Package Management
- [ ] DNF package catalog search & filtering
- [ ] Package detail inspector
- [ ] Multi-package batch selection & marking (Space, a/A)
- [ ] Installation planner (preflight, dependency resolution, execution)
- [ ] Safe package removal with explicit destructive confirmation
- [ ] Transaction progress streaming & cancellation

### Phase 4 — Applications & Toolchains
- [ ] Flatpak application manager & remote resolution
- [ ] Font installation & desktop preview
- [ ] Development toolchain bundles (Go, Rust, C/C++, Node.js, Python, Java)
- [ ] Shell & terminal environment configuration

### Phase 5 — System Polish & History
- [ ] Journal: Structured audit log of system modifications & diagnostics
- [ ] Settings: Preferences, safety confirmation thresholds, theme switcher
- [ ] Command Palette (Ctrl+K) global search & action launcher
- [ ] Extensibility: Preparation for secondary Linux distributions

---

## Release 1 Philosophy: Fedora Foundation

SetupForge prioritizes building a rock-solid, production-grade Fedora control center before generalizing to multiple distributions.
- Build coherent vertical slices end-to-end.
- Keep domain interfaces minimal and capability-focused.
- Do not create speculative or fake multi-distro stubs until a second real distribution is targeted.
