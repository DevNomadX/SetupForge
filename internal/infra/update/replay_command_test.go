package update_test

import (
	"reflect"
	"testing"

	"github.com/setupforge/setupforge/internal/infra/update"
)

func TestBuildDNF5ReplayCommand_ExactIdentityAndSafety(t *testing.T) {
	opID := "op-1234-abcd"
	artifactDir := "/var/tmp/setupforge-1000/transactions/tx-test"

	cmd := update.BuildDNF5ReplayCommand("/usr/bin/dnf5", opID, artifactDir)

	if cmd.Executable != "/usr/bin/dnf5" {
		t.Errorf("expected executable /usr/bin/dnf5, got: %s", cmd.Executable)
	}

	expectedArgs := []string{
		"--comment=setupforge:op-1234-abcd",
		"-y",
		"replay",
		"/var/tmp/setupforge-1000/transactions/tx-test",
	}
	if !reflect.DeepEqual(cmd.Args, expectedArgs) {
		t.Errorf("expected args %v, got: %v", expectedArgs, cmd.Args)
	}

	// Verify working directory is root "/"
	if cmd.Dir != "/" {
		t.Errorf("expected Dir to be '/', got: %s", cmd.Dir)
	}

	// Verify environment is sanitized and contains PATH and locale
	hasPath := false
	hasLocale := false
	for _, env := range cmd.Env {
		if env == "PATH=/usr/bin:/bin:/usr/sbin:/sbin" {
			hasPath = true
		}
		if env == "LC_ALL=C.UTF-8" {
			hasLocale = true
		}
	}
	if !hasPath || !hasLocale {
		t.Errorf("expected sanitized environment with PATH and LC_ALL, got: %v", cmd.Env)
	}

	// Verify zero relaxation flags
	forbiddenFlags := []string{
		"--ignore-installed",
		"--ignore-extras",
		"--skip-broken",
		"--skip-unavailable",
		"--allowerasing",
		"--nobest",
	}
	for _, arg := range cmd.Args {
		for _, forbidden := range forbiddenFlags {
			if arg == forbidden {
				t.Fatalf("CRITICAL SECURITY VIOLATION: command contains relaxation flag %q", arg)
			}
		}
	}
}

func TestBuildDNF5ReplayCommand_DefaultPath(t *testing.T) {
	cmd := update.BuildDNF5ReplayCommand("", "test-op", "/tmp/art")
	if cmd.Executable != "/usr/bin/dnf5" {
		t.Errorf("expected default executable /usr/bin/dnf5, got: %s", cmd.Executable)
	}
}
