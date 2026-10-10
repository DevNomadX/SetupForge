package update

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
)

// MaxHistoryReadBytes limits the maximum size of history data read into memory (10 MB).
// Guards against unbounded memory allocation from untrusted subprocess output.
const MaxHistoryReadBytes = 10 * 1024 * 1024

// CommandExecutor executes a system command in a read-only manner and returns its stdout.
type CommandExecutor func(ctx context.Context, executable string, args ...string) ([]byte, error)

// DNF5HistoryReaderOption configures a DNF5HistoryReader.
type DNF5HistoryReaderOption func(*DNF5HistoryReader)

// WithHistoryDNF5Path sets an explicit path to the dnf5 executable.
func WithHistoryDNF5Path(path string) DNF5HistoryReaderOption {
	return func(r *DNF5HistoryReader) {
		r.dnf5Path = path
	}
}

// WithHistoryCommandExecutor sets a custom CommandExecutor (e.g. for testing with fixtures).
func WithHistoryCommandExecutor(exec CommandExecutor) DNF5HistoryReaderOption {
	return func(r *DNF5HistoryReader) {
		r.exec = exec
	}
}

// DNF5HistoryReader implements domain.HistorySource using unprivileged 'dnf5 history info --json'.
//
// Invariants enforced:
// 1. Strictly read-only: executes only 'history info --json'. Zero mutating flags or replay.
// 2. Unprivileged execution: does not invoke sudo. DNF5 history database is world-readable (0644).
// 3. Direct argument vector: zero shell interpreters.
// 4. Bounded reading: output capped at MaxHistoryReadBytes to protect against memory exhaustion.
// 5. Untrusted parsing: treats all output as untrusted; validates fields defensively.
type DNF5HistoryReader struct {
	dnf5Path string
	exec     CommandExecutor
}

// NewDNF5HistoryReader constructs a DNF5HistoryReader with optional configurations.
func NewDNF5HistoryReader(opts ...DNF5HistoryReaderOption) *DNF5HistoryReader {
	r := &DNF5HistoryReader{
		dnf5Path: "/usr/bin/dnf5",
		exec:     defaultCommandExecutor,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func defaultCommandExecutor(ctx context.Context, executable string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = "/"
	cmd.Env = []string{
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
		"LC_ALL=C.UTF-8",
		"LANG=C.UTF-8",
	}
	return cmd.Output()
}

// QueryHistory queries DNF5 transaction history and parses it into domain HistoryRecords.
func (r *DNF5HistoryReader) QueryHistory(ctx context.Context, criteria domain.HistoryCriteria) ([]domain.HistoryRecord, error) {
	var args []string
	if criteria.TransactionID > 0 {
		args = []string{"history", "info", "--json", fmt.Sprintf("%d", criteria.TransactionID)}
	} else {
		args = []string{"history", "info", "--json"}
	}

	output, err := r.exec(ctx, r.dnf5Path, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to execute %s %s: %v", domain.ErrHistoryUnavailable, r.dnf5Path, strings.Join(args, " "), err)
	}

	records, err := ParseDNF5HistoryJSON(bytes.NewReader(output))
	if err != nil {
		return nil, err
	}

	// Apply SinceTransID filter if specified
	if criteria.SinceTransID > 0 {
		var filtered []domain.HistoryRecord
		for _, rec := range records {
			if rec.ID > criteria.SinceTransID {
				filtered = append(filtered, rec)
			}
		}
		records = filtered
	}

	// Apply Limit if specified
	if criteria.Limit > 0 && len(records) > criteria.Limit {
		records = records[:criteria.Limit]
	}

	return records, nil
}

// rawHistoryEntry models the schema returned by 'dnf5 history info --json' and SQLite trans table.
type rawHistoryEntry struct {
	ID                int64   `json:"id"`
	StartTime         int64   `json:"start_time"`
	DtBegin           int64   `json:"dt_begin"`
	EndTime           *int64  `json:"end_time"`
	DtEnd             *int64  `json:"dt_end"`
	RPMDBVersionBegin string  `json:"rpmdb_version_begin"`
	RPMDBVersionEnd   *string `json:"rpmdb_version_end"`
	Status            string  `json:"status"`
	StateID           int     `json:"state_id"`
	Releasever        string  `json:"releasever"`
	UserID            int     `json:"user_id"`
	UserName          string  `json:"user_name"`
	Description       string  `json:"description"`
	Comment           string  `json:"comment"`
	Packages          []any   `json:"packages"`
	AlteredCount      int     `json:"altered_count"`
}

// ParseDNF5HistoryJSON parses raw JSON output from DNF5 history into normalized domain.HistoryRecords.
// It handles single object or array inputs, enforces MaxHistoryReadBytes, and validates data defensively.
func ParseDNF5HistoryJSON(r io.Reader) ([]domain.HistoryRecord, error) {
	limitedReader := io.LimitReader(r, MaxHistoryReadBytes+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("%w: reading history data: %v", domain.ErrCorruptedHistory, err)
	}
	if int64(len(data)) > MaxHistoryReadBytes {
		return nil, fmt.Errorf("%w: history data exceeds maximum allowed size (%d bytes)", domain.ErrCorruptedHistory, MaxHistoryReadBytes)
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var rawEntries []rawHistoryEntry
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &rawEntries); err != nil {
			return nil, fmt.Errorf("%w: invalid JSON array: %v", domain.ErrCorruptedHistory, err)
		}
	} else if trimmed[0] == '{' {
		var single rawHistoryEntry
		if err := json.Unmarshal(trimmed, &single); err != nil {
			return nil, fmt.Errorf("%w: invalid JSON object: %v", domain.ErrCorruptedHistory, err)
		}
		rawEntries = append(rawEntries, single)
	} else {
		return nil, fmt.Errorf("%w: unexpected JSON token at start: %q", domain.ErrCorruptedHistory, trimmed[0])
	}

	records := make([]domain.HistoryRecord, 0, len(rawEntries))
	for _, entry := range rawEntries {
		rec, err := normalizeHistoryEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid history record: %v", domain.ErrCorruptedHistory, err)
		}
		records = append(records, rec)
	}

	return records, nil
}

func normalizeHistoryEntry(entry rawHistoryEntry) (domain.HistoryRecord, error) {
	if entry.ID <= 0 {
		return domain.HistoryRecord{}, fmt.Errorf("transaction ID must be positive, found %d", entry.ID)
	}

	// Resolve StartTime (start_time from CLI JSON or dt_begin from direct DB)
	startSec := entry.StartTime
	if startSec == 0 {
		startSec = entry.DtBegin
	}
	if startSec <= 0 {
		return domain.HistoryRecord{}, fmt.Errorf("transaction %d has invalid begin timestamp: %d", entry.ID, startSec)
	}
	startTime := time.Unix(startSec, 0)

	// Resolve EndTime (end_time from CLI JSON or dt_end from direct DB)
	var endTime *time.Time
	endSec := entry.EndTime
	if endSec == nil {
		endSec = entry.DtEnd
	}
	if endSec != nil && *endSec > 0 {
		t := time.Unix(*endSec, 0)
		endTime = &t
	}

	// Resolve RPMDB End Checksum
	rpmdbEnd := ""
	if entry.RPMDBVersionEnd != nil {
		rpmdbEnd = *entry.RPMDBVersionEnd
	}

	// Normalize State and StateID
	status := strings.TrimSpace(entry.Status)
	stateID := entry.StateID

	if stateID == 0 {
		switch strings.ToLower(status) {
		case "started":
			stateID = 1
		case "ok":
			stateID = 2
		case "error":
			stateID = 3
		}
	}

	if status == "" && stateID > 0 {
		switch stateID {
		case 1:
			status = "Started"
		case 2:
			status = "Ok"
		case 3:
			status = "Error"
		}
	}

	alteredCount := entry.AlteredCount
	if alteredCount == 0 && len(entry.Packages) > 0 {
		alteredCount = len(entry.Packages)
	}

	return domain.HistoryRecord{
		ID:                entry.ID,
		State:             status,
		StateID:           stateID,
		StartTime:         startTime,
		EndTime:           endTime,
		RPMDBVersionBegin: entry.RPMDBVersionBegin,
		RPMDBVersionEnd:   rpmdbEnd,
		Comment:           entry.Comment,
		Description:       entry.Description,
		Releasever:        entry.Releasever,
		UserID:            entry.UserID,
		UserName:          entry.UserName,
		AlteredCount:      alteredCount,
	}, nil
}
