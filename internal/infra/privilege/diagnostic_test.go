//go:build manual_diagnostic

package privilege_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/setupforge/setupforge/internal/app"
	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/privilege"
	"github.com/setupforge/setupforge/internal/infra/update"
)

// TestDiagnostic_RealSudoRunner traces the actual real-system execution path.
// Run with: go test -v -tags=manual_diagnostic -run TestDiagnostic_RealSudoRunner ./internal/infra/privilege
func TestDiagnostic_RealSudoRunner(t *testing.T) {
	fmt.Println("================== PHASE 2A RUNTIME DIAGNOSTIC ==================")

	// 1. Environment & Sudo info
	euid := os.Geteuid()
	isRoot := (euid == 0)
	sudoPath, err := exec.LookPath("sudo")
	dnf5Path, dnf5Err := exec.LookPath("dnf5")

	fmt.Printf("Current EUID: %d (IsRoot: %v)\n", euid, isRoot)
	fmt.Printf("sudo path: %s (err: %v)\n", sudoPath, err)
	fmt.Printf("dnf5 path: %s (err: %v)\n", dnf5Path, dnf5Err)

	// 2. Direct SudoRunner execution
	runner := privilege.NewSudoRunner()
	fmt.Printf("SudoRunner nonInteractive default: true\n")

	cmd := privilege.PrivilegedCommand{
		Executable: dnf5Path,
		Args:       []string{"check-upgrade", "--json"},
	}
	fmt.Printf("PrivilegedCommand: Executable=%s, Args=%v\n", cmd.Executable, cmd.Args)

	if isRoot {
		fmt.Println("SudoRunner strategy: ROOT BYPASS (direct execution)")
	} else {
		fmt.Printf("SudoRunner strategy: SUDO ELEVATION (sudo -n %s %v)\n", cmd.Executable, cmd.Args)
	}

	res, runErr := runner.Run(context.Background(), cmd)

	fmt.Println("--- SudoRunner.Run Result ---")
	fmt.Printf("ExitCode: %d\n", res.ExitCode)
	fmt.Printf("Stdout (%d bytes): %q\n", len(res.Stdout), string(res.Stdout))
	fmt.Printf("Stderr (%d bytes): %q\n", len(res.Stderr), string(res.Stderr))
	fmt.Printf("Go Error: %v\n", runErr)

	// 3. DNF5UpdateChecker execution
	fmt.Println("--- DNF5UpdateChecker.Check Result ---")
	checker := update.NewDNF5UpdateChecker()
	status, checkErr := checker.Check(context.Background(), domain.CheckOptions{RefreshMetadata: false})

	fmt.Printf("Status.State: %s\n", status.State)
	fmt.Printf("Status.UpdateCount: %d\n", status.UpdateCount)
	fmt.Printf("Status.ErrorMsg: %q\n", status.ErrorMsg)
	fmt.Printf("Check Error: %v\n", checkErr)

	// 4. UpdateService execution
	fmt.Println("--- UpdateService.GetUpdateStatus Result ---")
	service := app.NewUpdateService(checker)
	svcStatus, svcErr := service.GetUpdateStatus(context.Background())

	fmt.Printf("Service Status.State: %s\n", svcStatus.State)
	fmt.Printf("Service Status.UpdateCount: %d\n", svcStatus.UpdateCount)
	fmt.Printf("Service Status.ErrorMsg: %q\n", svcStatus.ErrorMsg)
	fmt.Printf("Service Error: %v\n", svcErr)

	// 5. Unprivileged execution with --setopt=system_state_dir=/var/empty
	fmt.Println("--- Direct Unprivileged Execution (--setopt=system_state_dir=/var/empty) ---")
	rawStdout, rawStderr, rawExit, rawErr := privilege.DefaultRawExecutor(
		context.Background(),
		dnf5Path,
		"check-upgrade",
		"--json",
		"--setopt=system_state_dir=/var/empty",
	)
	fmt.Printf("Direct ExitCode: %d, Err: %v\n", rawExit, rawErr)
	fmt.Printf("Direct Stdout (%d bytes): %s\n", len(rawStdout), string(rawStdout))
	fmt.Printf("Direct Stderr (%d bytes): %s\n", len(rawStderr), string(rawStderr))

	parsedStatus, parseErr := update.ParseDNF5CheckUpgradeJSON(rawStdout)
	fmt.Printf("Parsed Status: State=%s, Updates=%d, Err=%v\n", parsedStatus.State, parsedStatus.UpdateCount, parseErr)
	fmt.Println("================================================================")
}
