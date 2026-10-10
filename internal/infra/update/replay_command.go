package update

import "github.com/setupforge/setupforge/internal/domain"

// BuildDNF5ReplayCommand constructs the PrivilegedCommand for dnf5 replay
// bound strictly to the sealed artifact directory and OperationID comment.
//
// Invariants enforced:
// 1. Exact reviewed artifact directory is passed as target.
// 2. Exact OperationID is passed in --comment="setupforge:<opID>".
// 3. Non-interactive execution (-y).
// 4. Zero relaxation flags (--ignore-installed, --ignore-extras, --skip-broken are strictly forbidden).
// 5. Environment sanitized (PATH, C.UTF-8 locale).
// 6. Working directory set to root ("/").
func BuildDNF5ReplayCommand(dnf5Path, opID, artifactDir string) domain.PrivilegedCommand {
	if dnf5Path == "" {
		dnf5Path = "/usr/bin/dnf5"
	}
	return domain.PrivilegedCommand{
		Executable: dnf5Path,
		Args: []string{
			"--comment=setupforge:" + opID,
			"-y",
			"replay",
			artifactDir,
		},
		Dir: "/",
		Env: []string{
			"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
			"LC_ALL=C.UTF-8",
			"LANG=C.UTF-8",
		},
	}
}
