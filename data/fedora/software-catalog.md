# Fedora Software Catalog

Source basis: the supplied `fedora-package-list.md`. This is catalog data, not a command script.

## Metadata model

Each entry should support:
- `id`
- `name`
- `category`
- `description`
- `method` (`dnf`, `dnf-group`, `flatpak`, `copr`, `repo`, `script`, `rpm`, etc.)
- `package`
- `flatpak_id`
- `repository`
- `group`
- `official_url`
- `requires_root`
- `interactive`
- `automation_policy`
- `optional`
- `choice_group`
- `hardware_scope`
- `service_scope`
- `notes`
- `verification_status`

## Inventory from the supplied list

### Core
zoxide, curl, wget, git, trash-cli, stow, fuse-libs, exfat-utils, exfatprogs, flatseal, firewall-config, neovim, android-tools, eza.

Duplicate `curl` entries should become one catalog item.

### Shell — user choice
- zsh
- fish

### Multiplexer / shell
- tmux
- herdr (script installer)

### TUI tools
ripgrep, speedtest-cli, htop, fastfetch, bat, fd-find, fzf, neovim.

### TUI file managers
- Superfile — official installer script
- Yazi — COPR `lihaohong/yazi`, package `yazi`

### Video
- vlc
- celluloid

### Development
The list includes the Fedora `development-tools` group plus make, cmake, ninja-build, meson, pkgconf, autoconf, automake.

Go: golang.

Rust: rust, cargo, rustfmt, clippy.

C/C++: clang and gcc. Clang is preferred by the source, but GCC remains an option and must not be silently removed.

Node.js: fnm installer, then Node.js LTS via `fnm install --lts`.

Python: uv official installer.

Java: java-25-openjdk, java-25-openjdk-devel.

PHP: php, php-cli, php-common, php-devel, php-mbstring, php-xml, php-json, php-pdo, php-mysqlnd, php-pgsql, php-gd, php-curl, php-zip.

Ruby: ruby, ruby-devel, rubygems.

Perl: perl, perl-devel.

### Git / GitHub
gh, git-email.

### API / HTTP / data
httpie, curl, jq, yq, xmlstarlet.

### Databases
sqlite, sqlite-devel, postgresql, postgresql-server, mariadb, mariadb-server, redis.

Database package installation must be separate from enabling/starting services.

### GNOME
nautilus-python, gnome-tweaks.

### GNOME extensions
gnome-shell-extension-gsconnect, gnome-shell-extension-dash-to-dock, gnome-shell-extension-user-themes, gnome-shell-extension-blur-my-shell, gnome-shell-extension-caffeine.

### Browser
Brave — vendor installer/repository workflow.

### Editors — user choice
- Zed — official installer
- Visual Studio Code — Microsoft RPM repository

### Flatpak — user choice
it.mijorus.gearlever
com.protonvpn.www
fr.handbrake.ghb
com.mattjakeman.ExtensionManager
com.github.unrud.VideoDownloader
org.telegram.desktop
md.obsidian.Obsidian
app.zen_browser.zen
com.bitwarden.desktop
org.freecad.FreeCAD
com.rtosta.zapzap
com.discordapp.Discord

Resolve IDs against configured remotes before installation.

### Archive
7zip, unrar, unzip, peazip, tar.

### RPM Fusion workflows
The source includes enabling Free/Nonfree, Cisco OpenH264 configuration, AppStream metadata, ffmpeg swap, multimedia packages, hardware codecs, and optional tainted firmware. These are repository/system workflows, not ordinary software rows.

## Important implementation rule

Never make the UI execute raw command strings from this Markdown. Use:

`SoftwareCatalog → InstallationPlan → Package/Repository/Flatpak/Script services → CommandRunner`
