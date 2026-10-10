package domain_test

import (
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
)

func TestCorrelateHistory_EmptyOperationID(t *testing.T) {
	baseline := domain.ExecutionBaseline{
		OperationID: "",
	}
	records := []domain.HistoryRecord{
		{ID: 1, Comment: "setupforge:"},
	}
	evidence := domain.CorrelateHistory(baseline, records)
	if evidence.Classification != domain.EvidenceNotFound {
		t.Fatalf("expected EvidenceNotFound for empty opID, got %s", evidence.Classification)
	}
	if evidence.Record != nil {
		t.Fatalf("expected nil record, got %+v", evidence.Record)
	}
}

func TestCorrelateHistory_MissingRecords(t *testing.T) {
	baseline := domain.ExecutionBaseline{
		OperationID: "op-1234",
	}

	// Case 1: Empty records slice
	ev1 := domain.CorrelateHistory(baseline, nil)
	if ev1.Classification != domain.EvidenceNotFound {
		t.Fatalf("expected EvidenceNotFound for nil records, got %s", ev1.Classification)
	}

	// Case 2: Records present but none match comment
	records := []domain.HistoryRecord{
		{ID: 1, Comment: "dnf up"},
		{ID: 2, Comment: "setupforge:other-op"},
		{ID: 3, Comment: ""},
	}
	ev2 := domain.CorrelateHistory(baseline, records)
	if ev2.Classification != domain.EvidenceNotFound {
		t.Fatalf("expected EvidenceNotFound for non-matching records, got %s", ev2.Classification)
	}
	if ev2.CandidateCount != 0 {
		t.Fatalf("expected 0 candidate count, got %d", ev2.CandidateCount)
	}
}

func TestCorrelateHistory_ExactOperationIDMatching(t *testing.T) {
	baseline := domain.ExecutionBaseline{
		OperationID: "op-test-555",
	}
	now := time.Now()
	later := now.Add(5 * time.Second)

	records := []domain.HistoryRecord{
		{
			ID:                10,
			State:             "Ok",
			StartTime:         now,
			EndTime:           &later,
			RPMDBVersionBegin: "rpmdb-begin-hash",
			RPMDBVersionEnd:   "rpmdb-end-hash",
			Comment:           "  setupforge:op-test-555  ", // verifies whitespace trimming
		},
	}

	ev := domain.CorrelateHistory(baseline, records)
	if ev.Classification != domain.EvidenceProvenSuccess {
		t.Fatalf("expected EvidenceProvenSuccess, got %s (%s)", ev.Classification, ev.Diagnostic)
	}
	if ev.Record == nil || ev.Record.ID != 10 {
		t.Fatalf("expected record ID 10, got %+v", ev.Record)
	}
}

func TestCorrelateHistory_MalformedComments(t *testing.T) {
	baseline := domain.ExecutionBaseline{
		OperationID: "op-target",
	}
	records := []domain.HistoryRecord{
		{ID: 1, Comment: "setupforge:"},                // prefix with empty opID
		{ID: 2, Comment: "setupforge:op-target-extra"}, // prefix with appended chars
		{ID: 3, Comment: "pre-setupforge:op-target"},   // extra prefix
		{ID: 4, Comment: "setupforge:op-targe"},        // truncated
		{ID: 5, Comment: "setupforge: op-target"},      // space after colon
	}

	ev := domain.CorrelateHistory(baseline, records)
	if ev.Classification != domain.EvidenceNotFound {
		t.Fatalf("expected EvidenceNotFound for malformed comments, got %s", ev.Classification)
	}
}

func TestCorrelateHistory_MultipleCandidates_AmbiguousConflict(t *testing.T) {
	baseline := domain.ExecutionBaseline{
		OperationID: "op-duplicate",
	}
	now := time.Now()
	later := now.Add(5 * time.Second)

	records := []domain.HistoryRecord{
		{
			ID:              10,
			State:           "Ok",
			StartTime:       now,
			EndTime:         &later,
			RPMDBVersionEnd: "hash-1",
			Comment:         "setupforge:op-duplicate",
		},
		{
			ID:        11,
			State:     "Error",
			StartTime: now.Add(10 * time.Second),
			EndTime:   &later,
			Comment:   "setupforge:op-duplicate",
		},
	}

	ev := domain.CorrelateHistory(baseline, records)
	if ev.Classification != domain.EvidenceAmbiguousConflict {
		t.Fatalf("expected EvidenceAmbiguousConflict for multiple matches, got %s", ev.Classification)
	}
	if ev.CandidateCount != 2 {
		t.Fatalf("expected CandidateCount 2, got %d", ev.CandidateCount)
	}
	if ev.Record != nil {
		t.Fatalf("expected nil record on ambiguous conflict to prevent silent selection, got %+v", ev.Record)
	}
}

func TestCorrelateHistory_CorroborationChecks(t *testing.T) {
	now := time.Now()
	later := now.Add(5 * time.Second)

	t.Run("StaleTransactionID", func(t *testing.T) {
		baseline := domain.ExecutionBaseline{
			OperationID:         "op-stale",
			BaselineLastTransID: 50,
		}
		records := []domain.HistoryRecord{
			{
				ID:              50, // equal to baseline -> stale
				State:           "Ok",
				StartTime:       now,
				EndTime:         &later,
				RPMDBVersionEnd: "hash-end",
				Comment:         "setupforge:op-stale",
			},
		}
		ev := domain.CorrelateHistory(baseline, records)
		if ev.Classification != domain.EvidenceCorroborationFailed {
			t.Fatalf("expected EvidenceCorroborationFailed for candidate ID <= BaselineLastTransID, got %s", ev.Classification)
		}
		if ev.CorroborationErr == "" {
			t.Fatal("expected non-empty CorroborationErr")
		}
	})

	t.Run("PrecedingStartTimeBeyondTolerance", func(t *testing.T) {
		baseline := domain.ExecutionBaseline{
			OperationID:       "op-clock",
			BaselineStartTime: now,
		}
		records := []domain.HistoryRecord{
			{
				ID:              51,
				State:           "Ok",
				StartTime:       now.Add(-65 * time.Second), // 65s before baseline -> exceeds 60s tolerance
				EndTime:         &later,
				RPMDBVersionEnd: "hash-end",
				Comment:         "setupforge:op-clock",
			},
		}
		ev := domain.CorrelateHistory(baseline, records)
		if ev.Classification != domain.EvidenceCorroborationFailed {
			t.Fatalf("expected EvidenceCorroborationFailed for start time preceding baseline > 60s, got %s", ev.Classification)
		}
	})

	t.Run("StartTimeWithinTolerancePasses", func(t *testing.T) {
		baseline := domain.ExecutionBaseline{
			OperationID:       "op-clock-ok",
			BaselineStartTime: now,
		}
		records := []domain.HistoryRecord{
			{
				ID:              52,
				State:           "Ok",
				StartTime:       now.Add(-30 * time.Second), // 30s before baseline -> within 60s tolerance
				EndTime:         &later,
				RPMDBVersionEnd: "hash-end",
				Comment:         "setupforge:op-clock-ok",
			},
		}
		ev := domain.CorrelateHistory(baseline, records)
		if ev.Classification != domain.EvidenceProvenSuccess {
			t.Fatalf("expected EvidenceProvenSuccess for start time within tolerance, got %s", ev.Classification)
		}
	})

	t.Run("BaselineRPMDBMismatch", func(t *testing.T) {
		baseline := domain.ExecutionBaseline{
			OperationID:          "op-rpmdb",
			BaselineRPMDBVersion: "expected-baseline-hash",
		}
		records := []domain.HistoryRecord{
			{
				ID:                53,
				State:             "Ok",
				StartTime:         now,
				EndTime:           &later,
				RPMDBVersionBegin: "unexpected-begin-hash",
				RPMDBVersionEnd:   "hash-end",
				Comment:           "setupforge:op-rpmdb",
			},
		}
		ev := domain.CorrelateHistory(baseline, records)
		if ev.Classification != domain.EvidenceCorroborationFailed {
			t.Fatalf("expected EvidenceCorroborationFailed for RPMDB mismatch, got %s", ev.Classification)
		}
	})
}

func TestCorrelateHistory_Outcomes(t *testing.T) {
	now := time.Now()
	later := now.Add(5 * time.Second)
	earlier := now.Add(-5 * time.Second)

	tests := []struct {
		name           string
		record         domain.HistoryRecord
		expectedClass  domain.EvidenceClassification
		expectDiagPart string
	}{
		{
			name: "ProvenSuccess_OkWithCompleteEndState",
			record: domain.HistoryRecord{
				ID:              1,
				State:           "Ok",
				StateID:         2,
				StartTime:       now,
				EndTime:         &later,
				RPMDBVersionEnd: "valid-end-rpmdb",
				Comment:         "setupforge:test-op",
			},
			expectedClass: domain.EvidenceProvenSuccess,
		},
		{
			name: "Inconclusive_OkWithMissingEndTime",
			record: domain.HistoryRecord{
				ID:              2,
				State:           "Ok",
				StateID:         2,
				StartTime:       now,
				EndTime:         nil, // missing end time
				RPMDBVersionEnd: "valid-end-rpmdb",
				Comment:         "setupforge:test-op",
			},
			expectedClass: domain.EvidenceInconclusive,
		},
		{
			name: "Inconclusive_OkWithZeroEndTime",
			record: domain.HistoryRecord{
				ID:              3,
				State:           "Ok",
				StateID:         2,
				StartTime:       now,
				EndTime:         &time.Time{}, // zero end time
				RPMDBVersionEnd: "valid-end-rpmdb",
				Comment:         "setupforge:test-op",
			},
			expectedClass: domain.EvidenceInconclusive,
		},
		{
			name: "Inconclusive_OkWithEndTimeBeforeStartTime",
			record: domain.HistoryRecord{
				ID:              4,
				State:           "Ok",
				StateID:         2,
				StartTime:       now,
				EndTime:         &earlier, // end before start
				RPMDBVersionEnd: "valid-end-rpmdb",
				Comment:         "setupforge:test-op",
			},
			expectedClass: domain.EvidenceInconclusive,
		},
		{
			name: "Inconclusive_OkWithMissingRPMDBEnd",
			record: domain.HistoryRecord{
				ID:              5,
				State:           "Ok",
				StateID:         2,
				StartTime:       now,
				EndTime:         &later,
				RPMDBVersionEnd: "", // missing RPMDB end
				Comment:         "setupforge:test-op",
			},
			expectedClass: domain.EvidenceInconclusive,
		},
		{
			name: "ProvenFailure_ErrorWithValidEndTime",
			record: domain.HistoryRecord{
				ID:        6,
				State:     "Error",
				StateID:   3,
				StartTime: now,
				EndTime:   &later,
				Comment:   "setupforge:test-op",
			},
			expectedClass: domain.EvidenceProvenFailure,
		},
		{
			name: "Inconclusive_ErrorWithMissingEndTime",
			record: domain.HistoryRecord{
				ID:        7,
				State:     "Error",
				StateID:   3,
				StartTime: now,
				EndTime:   nil, // error without clean end
				Comment:   "setupforge:test-op",
			},
			expectedClass: domain.EvidenceInconclusive,
		},
		{
			name: "Inconclusive_StartedState",
			record: domain.HistoryRecord{
				ID:        8,
				State:     "Started",
				StateID:   1,
				StartTime: now,
				EndTime:   nil,
				Comment:   "setupforge:test-op",
			},
			expectedClass: domain.EvidenceInconclusive,
		},
		{
			name: "Inconclusive_UnrecognizedState",
			record: domain.HistoryRecord{
				ID:        9,
				State:     "SomethingWeird",
				StateID:   99,
				StartTime: now,
				EndTime:   &later,
				Comment:   "setupforge:test-op",
			},
			expectedClass: domain.EvidenceInconclusive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseline := domain.ExecutionBaseline{
				OperationID: "test-op",
			}
			ev := domain.CorrelateHistory(baseline, []domain.HistoryRecord{tt.record})
			if ev.Classification != tt.expectedClass {
				t.Fatalf("expected classification %s, got %s (diagnostic: %s)",
					tt.expectedClass, ev.Classification, ev.Diagnostic)
			}
			if ev.Record == nil {
				t.Fatal("expected non-nil record for single candidate")
			}
		})
	}
}
