package update_test

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/update"
)

// TestRealFedora_ReadOnlyHistoryQuery verifies read-only DNF5 history inspection
// against the live host Fedora DNF5 transaction database.
//
// Invariants enforced:
// 1. Strictly unprivileged execution: runs without sudo.
// 2. Strictly read-only: executes only '/usr/bin/dnf5 history info --json'.
// 3. Zero package mutation, zero replay, zero journal modification.
func TestRealFedora_ReadOnlyHistoryQuery(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping Fedora integration test on non-Linux platform")
	}

	dnf5Path := "/usr/bin/dnf5"
	if _, err := os.Stat(dnf5Path); os.IsNotExist(err) {
		t.Skipf("skipping test: %s not installed on this host", dnf5Path)
	}

	// Verify the live DNF5 history database exists on host
	historyDB := "/usr/lib/sysimage/libdnf5/transaction_history.sqlite"
	if _, err := os.Stat(historyDB); os.IsNotExist(err) {
		t.Skipf("skipping test: %s not found on this host", historyDB)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Instantiate real DNF5HistoryReader using the actual /usr/bin/dnf5 executable
	// and unprivileged CommandExecutor.
	reader := update.NewDNF5HistoryReader(
		update.WithHistoryDNF5Path(dnf5Path),
	)

	// Exercised command: /usr/bin/dnf5 history info --json (strictly read-only query)
	records, err := reader.QueryHistory(ctx, domain.HistoryCriteria{Limit: 5})
	if err != nil {
		t.Fatalf("failed to query real DNF5 history via %s: %v", dnf5Path, err)
	}

	if len(records) == 0 {
		t.Log("warning: host DNF5 history database has 0 records; skipping record inspection assertions")
		return
	}

	latest := records[0]
	t.Logf("Successfully inspected real Fedora DNF5 transaction: ID=%d, State=%s, StartTime=%s, Packages=%d",
		latest.ID, latest.State, latest.StartTime.UTC().Format(time.RFC3339), latest.AlteredCount)

	if latest.ID <= 0 {
		t.Errorf("expected positive transaction ID, got %d", latest.ID)
	}
	if latest.StartTime.IsZero() {
		t.Error("expected non-zero start timestamp on real transaction record")
	}
	if latest.State == "" {
		t.Error("expected non-empty state string on real transaction record")
	}

	// Correlate against a non-existent SetupForge OperationID
	// Invariant: must conservatively return EvidenceNotFound without false matches
	baseline := domain.ExecutionBaseline{
		OperationID: "non-existent-synthetic-op-id-99999",
	}
	ev := domain.CorrelateHistory(baseline, records)
	if ev.Classification != domain.EvidenceNotFound {
		t.Errorf("expected EvidenceNotFound for synthetic opID against real history, got %s", ev.Classification)
	}
	if ev.Record != nil {
		t.Errorf("expected nil record for unmatched correlation, got %+v", ev.Record)
	}
}
