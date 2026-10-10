package domain

import (
	"context"
	"errors"
	"strings"
)

var (
	// ErrUnsupportedDistribution is returned when running on a non-Fedora host.
	ErrUnsupportedDistribution = errors.New("unsupported distribution: SetupForge currently supports Fedora Linux")

	// ErrOSReleaseNotFound is returned when /etc/os-release cannot be located or read.
	ErrOSReleaseNotFound = errors.New("operating system release information not found")
)

// OSRelease contains structured information parsed from /etc/os-release.
type OSRelease struct {
	ID           string // e.g. "fedora"
	IDLike       string // e.g. "rhel centos"
	Name         string // e.g. "Fedora Linux"
	Version      string // e.g. "44 (Workstation Edition)"
	VersionID    string // e.g. "44"
	Variant      string // e.g. "Workstation Edition"
	VariantID    string // e.g. "workstation"
	PrettyName   string // e.g. "Fedora Linux 44 (Workstation Edition)"
	HomeURL      string
	BugReportURL string
	SupportEnd   string
}

// IsFedora reports whether this release is Fedora.
func (r OSRelease) IsFedora() bool {
	return strings.EqualFold(r.ID, "fedora")
}

// SystemInfo aggregates detected host facts for display and capability checks.
type SystemInfo struct {
	OS           OSRelease
	Kernel       string
	Architecture string
	Hostname     string
	IsSupported  bool
}

// OSDetector is the domain capability interface for detecting the host operating system.
// Implementations must not leak command runners or presentation types.
type OSDetector interface {
	Detect(ctx context.Context) (SystemInfo, error)
}
