package domain

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrUpdateCheckFailed is returned when the package manager fails to check for available updates.
	ErrUpdateCheckFailed = errors.New("system update check failed")

	// ErrDNF5NotFound is returned when the dnf5 executable is not available on the host.
	ErrDNF5NotFound = errors.New("dnf5 package manager executable not found")

	// ErrMalformedUpdateOutput is returned when update check output cannot be parsed.
	ErrMalformedUpdateOutput = errors.New("malformed update check output")
)

// UpdateState represents the discrete lifecycle state of the update checker.
type UpdateState string

const (
	UpdateStateUnknown   UpdateState = "unknown"
	UpdateStateChecking  UpdateState = "checking"
	UpdateStateUpToDate  UpdateState = "up_to_date"
	UpdateStateAvailable UpdateState = "available"
	UpdateStateFailed    UpdateState = "failed"
)

// PackageUpdate represents an individual package upgrade candidate available from repositories.
type PackageUpdate struct {
	Name       string
	Arch       string
	EVR        string
	Repository string
}

// UpdateStatus contains the aggregated result of a system update check.
type UpdateStatus struct {
	State       UpdateState
	UpdateCount int
	Updates     []PackageUpdate
	LastChecked time.Time
	ErrorMsg    string
}

// IsAvailable reports whether package updates are available.
func (s UpdateStatus) IsAvailable() bool {
	return s.State == UpdateStateAvailable && s.UpdateCount > 0
}

// IsUpToDate reports whether the system has been verified as up to date.
func (s UpdateStatus) IsUpToDate() bool {
	return s.State == UpdateStateUpToDate
}

// CheckOptions specifies execution options for an update check.
type CheckOptions struct {
	RefreshMetadata bool
}

// UpdateChecker is the domain capability interface for querying available package updates.
// Implementations must not leak subprocess details or presentation types.
type UpdateChecker interface {
	Check(ctx context.Context, opts CheckOptions) (UpdateStatus, error)
}
