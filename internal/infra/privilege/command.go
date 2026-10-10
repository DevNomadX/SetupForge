package privilege

import "github.com/setupforge/setupforge/internal/domain"

// PrivilegedCommand represents a structured system command requiring elevated permissions.
// Executable and Args are explicit slices; no shell interpolation is permitted.
type PrivilegedCommand = domain.PrivilegedCommand

// CommandResult contains the process execution evidence (aliased to domain.ProcessResult).
type CommandResult = domain.ProcessResult

// PrivilegedRunner executes structured privileged commands via an operating system authorization provider.
type PrivilegedRunner = domain.PrivilegedRunner
