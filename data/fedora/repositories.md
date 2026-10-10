# Fedora Repository and Source Management

## Repository/source classes

SetupForge must distinguish:
1. Fedora official repositories
2. RPM Fusion Free
3. RPM Fusion Nonfree
4. RPM Fusion Nonfree Tainted
5. COPR
6. Vendor RPM repositories
7. Flatpak remotes
8. Official installer scripts

## RPM Fusion

The supplied inventory specifies Free/Nonfree release packages, Cisco OpenH264 configuration, RPM Fusion AppStream metadata, ffmpeg swap, multimedia packages, hardware-specific codecs, and optional tainted firmware.

Model each as an idempotent typed operation. Repository changes are privileged and system-wide.

Before enabling a third-party repository show its name, purpose, source, dependency reason, privilege requirement, and expected change.

## COPR

Supplied Yazi workflow:
- enable `lihaohong/yazi`
- install `yazi`

Keep repository enablement and package installation as separate operations.

## Vendor repositories

Visual Studio Code is represented by a Microsoft RPM repository. Store vendor, repository name, base URL, signing-key source, packages, enabled state, provenance, and confirmation requirement.

Never execute arbitrary repository definitions from untrusted configuration.

## Flatpak

Resolve application IDs against configured remotes before installation. Do not assume an ID is available merely because it is in the catalog.

## Idempotency

Repeated operations should report already-configured/already-installed states instead of duplicating configuration.

Query state before and after mutations.
