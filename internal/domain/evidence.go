package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrCorruptedHistory is returned when DNF5 transaction history output cannot be parsed or is malformed.
	ErrCorruptedHistory = errors.New("dnf5 transaction history corrupted or invalid")

	// ErrHistoryUnavailable is returned when DNF5 transaction history cannot be accessed or queried.
	ErrHistoryUnavailable = errors.New("dnf5 transaction history unavailable")
)

// TransactionCommentPrefix is the canonical prefix used in DNF5 transaction comments for SetupForge correlation.
const TransactionCommentPrefix = "setupforge:"

// EvidenceClassification categorizes the evidentiary conclusion drawn from DNF5 transaction history.
type EvidenceClassification string

const (
	// EvidenceProvenSuccess indicates a uniquely correlated history record with state 'Ok'
	// and complete end-state timestamps and RPMDB digest.
	//
	// What it proves:
	// - DNF5 completed rpmtsRun() and recorded a successful 'Ok' transaction in history.
	// - Transaction end timestamp and final RPMDB checksum were recorded.
	//
	// What it does NOT prove:
	// - Does not prove that external system services or application configurations were updated.
	// - Does not prove that arbitrary posttrans scriptlets had no internal runtime errors.
	// - Does not guarantee physical disk consistency if un-flushed writes existed prior to sudden power loss.
	EvidenceProvenSuccess EvidenceClassification = "PROVEN_SUCCESS"

	// EvidenceProvenFailure indicates a uniquely correlated history record with state 'Error'.
	//
	// What it proves:
	// - DNF5 actively recorded an 'Error' outcome for this transaction in persistent history.
	//
	// What it does NOT prove:
	// - Does not prove that the system was left in an unmodified baseline state; packages or
	//   scriptlets may have partially executed before the failure was recorded.
	EvidenceProvenFailure EvidenceClassification = "PROVEN_FAILURE"

	// EvidenceInconclusive indicates a matching record that is in progress ('Started'), has missing
	// end timestamps, or has an incomplete/unrecognized status.
	//
	// What it proves:
	// - DNF5 initialized the transaction and recorded a matching entry in persistent history.
	//
	// What it does NOT prove:
	// - Does NOT prove success.
	// - Does NOT prove clean failure. The system may be in a partially unpacked or corrupted RPM state.
	//   Must NEVER be treated as clean failure.
	EvidenceInconclusive EvidenceClassification = "INCONCLUSIVE"

	// EvidenceNotFound indicates that no history record matched the OperationID comment.
	//
	// What it proves:
	// - No transaction record with the expected comment exists in the queried history.
	//
	// What it does NOT prove:
	// - Does NOT prove that the transaction did not execute or fail cleanly (e.g. process may have
	//   been terminated before the initial SQLite WAL frame was flushed, or history was cleared).
	//   Must NEVER be treated as clean failure.
	EvidenceNotFound EvidenceClassification = "NOT_FOUND"

	// EvidenceAmbiguousConflict indicates that multiple distinct records matched the OperationID comment,
	// or contradictory outcomes were encountered.
	//
	// What it proves:
	// - History evidence is contradictory, duplicated, or non-deterministic.
	//
	// What it does NOT prove:
	// - Does not prove which record (if any) represents the intended transaction attempt.
	//   Must NEVER select the newest or nearest record automatically.
	EvidenceAmbiguousConflict EvidenceClassification = "AMBIGUOUS_CONFLICT"

	// EvidenceCorroborationFailed indicates a record matched the primary OperationID comment,
	// but corroborating consistency checks (transaction ID monotonicity, start timestamp bounds,
	// or baseline RPMDB checksum) contradicted the recorded execution baseline.
	//
	// What it proves:
	// - A record with the expected comment exists, but its operational context contradicts
	//   the recorded pre-execution baseline (e.g. stale record from an earlier attempt,
	//   or execution against a drifted host RPM state).
	//
	// What it does NOT prove:
	// - Does not prove clean execution or safe failure.
	EvidenceCorroborationFailed EvidenceClassification = "CORROBORATION_FAILED"

	// EvidenceUnavailable indicates that transaction history could not be queried or parsed.
	//
	// What it proves:
	// - SetupForge is unable to inspect external DNF5 transaction history.
	//
	// What it does NOT prove:
	// - Does not prove anything about whether the transaction ran, succeeded, or failed.
	//   Must NEVER be treated as clean failure.
	EvidenceUnavailable EvidenceClassification = "UNAVAILABLE"
)

// HistoryRecord represents a parsed, normalized record from DNF5 transaction history.
type HistoryRecord struct {
	ID                int64      `json:"id"`
	State             string     `json:"state"`   // "Ok", "Started", "Error", or raw string
	StateID           int        `json:"stateId"` // 1=Started, 2=Ok, 3=Error
	StartTime         time.Time  `json:"startTime"`
	EndTime           *time.Time `json:"endTime,omitempty"`
	RPMDBVersionBegin string     `json:"rpmdbVersionBegin"`
	RPMDBVersionEnd   string     `json:"rpmdbVersionEnd,omitempty"`
	Comment           string     `json:"comment"`
	Description       string     `json:"description,omitempty"`
	Releasever        string     `json:"releasever,omitempty"`
	UserID            int        `json:"userId"`
	UserName          string     `json:"userName,omitempty"`
	AlteredCount      int        `json:"alteredCount,omitempty"`
}

// ExecutionBaseline captures the expected host state and operational bounds
// recorded prior to privileged transaction execution.
type ExecutionBaseline struct {
	// OperationID is the primary correlation identity generated by SetupForge.
	OperationID string

	// BaselineLastTransID is the highest known DNF5 history transaction ID prior to execution spawn.
	// If > 0, any valid transaction record for this execution must have ID > BaselineLastTransID.
	BaselineLastTransID int64

	// BaselineStartTime is the wall-clock timestamp when execution was initiated.
	// Any valid transaction record must have StartTime >= BaselineStartTime - 60s (clock skew tolerance).
	BaselineStartTime time.Time

	// BaselineRPMDBVersion is the expected host RPMDB SHA-256 checksum prior to execution.
	// If non-empty, candidate records must match this checksum.
	BaselineRPMDBVersion string
}

// ExecutionEvidence represents the comprehensive evaluation of DNF5 transaction history
// against an execution baseline.
type ExecutionEvidence struct {
	Classification   EvidenceClassification `json:"classification"`
	Record           *HistoryRecord         `json:"record,omitempty"`
	CandidateCount   int                    `json:"candidateCount"`
	Diagnostic       string                 `json:"diagnostic"`
	CorroborationErr string                 `json:"corroborationErr,omitempty"`
}

// HistoryCriteria specifies filter and query bounds for reading history records.
type HistoryCriteria struct {
	SinceTransID  int64 // If > 0, return only transactions with ID > SinceTransID
	TransactionID int64 // If > 0, return specific transaction ID
	Limit         int   // If > 0, maximum number of records to return
}

// HistorySource is the domain interface for querying DNF5 transaction history in a read-only manner.
type HistorySource interface {
	QueryHistory(ctx context.Context, criteria HistoryCriteria) ([]HistoryRecord, error)
}

// CorrelateHistory evaluates a slice of candidate DNF5 history records against an ExecutionBaseline.
// It implements the strict evidence hierarchy:
// 1. Primary Identity: exact OperationID matching via "setupforge:<OperationID>" comment.
// 2. Ambiguity Rejection: multiple matching comments immediately fail with AMBIGUOUS_CONFLICT.
// 3. Corroborating Consistency Checks: monotonic transaction ID, start timestamp sanity, and baseline RPMDB checksum.
// 4. Outcome Classification: PROVEN_SUCCESS, PROVEN_FAILURE, or INCONCLUSIVE.
//
// Invariants enforced:
// - Never selects newest or nearest record if multiple candidates match.
// - An absent record, "Started" record, or corroboration failure is NEVER treated as clean failure.
// - Pure domain logic: zero side effects, zero I/O, zero journal mutations.
func CorrelateHistory(baseline ExecutionBaseline, records []HistoryRecord) ExecutionEvidence {
	if baseline.OperationID == "" {
		return ExecutionEvidence{
			Classification: EvidenceNotFound,
			CandidateCount: 0,
			Diagnostic:     "baseline operation ID is empty; cannot correlate",
		}
	}

	targetComment := TransactionCommentPrefix + baseline.OperationID

	// 1. Primary Identity Filtering: find records matching target comment
	var candidates []HistoryRecord
	for _, rec := range records {
		if strings.TrimSpace(rec.Comment) == targetComment {
			candidates = append(candidates, rec)
		}
	}

	// 2. Evaluate Candidate Count
	if len(candidates) == 0 {
		return ExecutionEvidence{
			Classification: EvidenceNotFound,
			CandidateCount: 0,
			Diagnostic:     fmt.Sprintf("no history record found matching operation ID %q", baseline.OperationID),
		}
	}

	if len(candidates) > 1 {
		ids := make([]string, len(candidates))
		for i, c := range candidates {
			ids[i] = fmt.Sprintf("%d", c.ID)
		}
		return ExecutionEvidence{
			Classification: EvidenceAmbiguousConflict,
			CandidateCount: len(candidates),
			Diagnostic: fmt.Sprintf("multiple matching history records (%d records, IDs: %s) found for operation ID %q; rejecting ambiguous match conservatively",
				len(candidates), strings.Join(ids, ", "), baseline.OperationID),
		}
	}

	candidate := candidates[0]

	// 3. Corroborating Consistency Checks
	// Check A: Monotonic Transaction ID check (detect stale historical records)
	if baseline.BaselineLastTransID > 0 && candidate.ID <= baseline.BaselineLastTransID {
		errDetail := fmt.Sprintf("candidate transaction ID %d is not greater than baseline last transaction ID %d",
			candidate.ID, baseline.BaselineLastTransID)
		return ExecutionEvidence{
			Classification:   EvidenceCorroborationFailed,
			Record:           &candidate,
			CandidateCount:   1,
			CorroborationErr: errDetail,
			Diagnostic:       fmt.Sprintf("corroboration check failed: %s", errDetail),
		}
	}

	// Check B: Temporal Sanity Check (tolerating up to 60s backward clock skew)
	if !baseline.BaselineStartTime.IsZero() {
		skewBoundary := baseline.BaselineStartTime.Add(-60 * time.Second)
		if candidate.StartTime.Before(skewBoundary) {
			errDetail := fmt.Sprintf("candidate start time %s is before baseline start time %s (-60s tolerance)",
				candidate.StartTime.UTC().Format(time.RFC3339), baseline.BaselineStartTime.UTC().Format(time.RFC3339))
			return ExecutionEvidence{
				Classification:   EvidenceCorroborationFailed,
				Record:           &candidate,
				CandidateCount:   1,
				CorroborationErr: errDetail,
				Diagnostic:       fmt.Sprintf("corroboration check failed: %s", errDetail),
			}
		}
	}

	// Check C: Baseline RPMDB Checksum Consistency
	if baseline.BaselineRPMDBVersion != "" && candidate.RPMDBVersionBegin != "" && candidate.RPMDBVersionBegin != baseline.BaselineRPMDBVersion {
		errDetail := fmt.Sprintf("candidate RPMDB begin checksum %q does not match baseline %q",
			candidate.RPMDBVersionBegin, baseline.BaselineRPMDBVersion)
		return ExecutionEvidence{
			Classification:   EvidenceCorroborationFailed,
			Record:           &candidate,
			CandidateCount:   1,
			CorroborationErr: errDetail,
			Diagnostic:       fmt.Sprintf("corroboration check failed: %s", errDetail),
		}
	}

	// 4. Outcome Classification
	// Normalize status
	statusNorm := strings.ToLower(strings.TrimSpace(candidate.State))

	switch {
	case statusNorm == "ok" || candidate.StateID == 2:
		// Success requires non-null, valid end time AND non-empty RPMDB end checksum
		if candidate.EndTime == nil || candidate.EndTime.IsZero() {
			return ExecutionEvidence{
				Classification: EvidenceInconclusive,
				Record:         &candidate,
				CandidateCount: 1,
				Diagnostic:     fmt.Sprintf("candidate transaction %d has 'Ok' status but end timestamp is null or missing", candidate.ID),
			}
		}
		if candidate.EndTime.Before(candidate.StartTime) {
			return ExecutionEvidence{
				Classification: EvidenceInconclusive,
				Record:         &candidate,
				CandidateCount: 1,
				Diagnostic:     fmt.Sprintf("candidate transaction %d has 'Ok' status but end timestamp (%s) is before start timestamp (%s)", candidate.ID, candidate.EndTime.UTC().Format(time.RFC3339), candidate.StartTime.UTC().Format(time.RFC3339)),
			}
		}
		if candidate.RPMDBVersionEnd == "" {
			return ExecutionEvidence{
				Classification: EvidenceInconclusive,
				Record:         &candidate,
				CandidateCount: 1,
				Diagnostic:     fmt.Sprintf("candidate transaction %d has 'Ok' status but end RPMDB checksum is missing", candidate.ID),
			}
		}
		return ExecutionEvidence{
			Classification: EvidenceProvenSuccess,
			Record:         &candidate,
			CandidateCount: 1,
			Diagnostic:     fmt.Sprintf("uniquely correlated transaction %d recorded 'Ok' with complete end state", candidate.ID),
		}

	case statusNorm == "error" || candidate.StateID == 3:
		if candidate.EndTime == nil || candidate.EndTime.IsZero() {
			return ExecutionEvidence{
				Classification: EvidenceInconclusive,
				Record:         &candidate,
				CandidateCount: 1,
				Diagnostic:     fmt.Sprintf("candidate transaction %d has 'Error' status but end timestamp is null or missing", candidate.ID),
			}
		}
		return ExecutionEvidence{
			Classification: EvidenceProvenFailure,
			Record:         &candidate,
			CandidateCount: 1,
			Diagnostic:     fmt.Sprintf("uniquely correlated transaction %d recorded 'Error' outcome", candidate.ID),
		}

	case statusNorm == "started" || candidate.StateID == 1:
		return ExecutionEvidence{
			Classification: EvidenceInconclusive,
			Record:         &candidate,
			CandidateCount: 1,
			Diagnostic:     fmt.Sprintf("candidate transaction %d recorded as 'Started' without completion (interrupted or active)", candidate.ID),
		}

	default:
		return ExecutionEvidence{
			Classification: EvidenceInconclusive,
			Record:         &candidate,
			CandidateCount: 1,
			Diagnostic:     fmt.Sprintf("candidate transaction %d has unrecognized state %q (stateID=%d)", candidate.ID, candidate.State, candidate.StateID),
		}
	}
}
