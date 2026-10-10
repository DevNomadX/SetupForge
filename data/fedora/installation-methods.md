# Fedora Installation Methods

SetupForge should convert catalog entries into typed installation plans:

`Catalog → Resolve → Preflight → Confirm → Execute → Verify`

## DNF package
Use the package-management service. UI code must not construct shell commands.

## DNF group
Represent groups explicitly. Verify the group and show its scope before substantial installation.

## Flatpak
Resolve app and remote → confirm → install → verify.

## COPR
Check repository → enable if needed → refresh metadata → install → verify.

## Vendor repository
Verify vendor source → configure signing key/repository → refresh metadata → install → verify.

## Official installer scripts

The supplied list includes scripts for Superfile, fnm, uv, Brave, Zed, and herdr.

Do not make a UI click blindly run `curl | sh`.

Preferred flow:
1. obtain the official installer;
2. use a secure temporary file;
3. verify download success;
4. record/display provenance;
5. check vendor checksum/signature when available;
6. explain intended changes;
7. require explicit confirmation;
8. execute in a controlled environment;
9. verify installed binary/version;
10. clean temporary files.

Prefer a safer official package/repository method when one exists.

## Hardware-specific packages

Examples include Intel media drivers, Mesa freeworld codecs/drivers, and tainted firmware. Never install unconditionally. Detect applicability where reliable and let the user inspect/select.

## Development bundles

Provide selectable bundles for C/C++, Go, Rust, Node.js, Python, Java, PHP, Ruby, Perl, databases, API/HTTP, and Git/GitHub.

## Services

Installing PostgreSQL/MariaDB/Redis packages is distinct from enabling or starting their services. Starting/enabling requires explicit intent.

## Dry run

Before a substantial plan show repository changes, packages, Flatpaks, user-space installers, services, and whether privileged operations are required.

## Failure states

Track completed, cancelled, failed, and partially completed operations. Never report a multi-step plan as successful when one required step failed.
