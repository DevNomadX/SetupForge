package domain_test

import (
	"errors"
	"testing"

	"github.com/setupforge/setupforge/internal/domain"
)

func TestValidateTransition_IllegalTransitions(t *testing.T) {
	illegalPairs := []struct {
		from domain.TransactionState
		to   domain.TransactionState
		desc string
	}{
		{domain.TxStatePlanned, domain.TxStateExecuting, "PLANNED -> EXECUTING is forbidden (must be reviewed/confirmed/authorized)"},
		{domain.TxStateReviewed, domain.TxStateExecuting, "REVIEWED -> EXECUTING is forbidden (must be confirmed/authorized)"},
		{domain.TxStateConfirmed, domain.TxStateCompleted, "CONFIRMED -> COMPLETED is forbidden (cannot bypass execution)"},
		{domain.TxStateFailed, domain.TxStateExecuting, "FAILED -> EXECUTING is forbidden"},
		{domain.TxStateInterrupted, domain.TxStateExecuting, "INTERRUPTED -> EXECUTING is forbidden (no automatic replay)"},
		{domain.TxStateInterrupted, domain.TxStateCompleted, "INTERRUPTED -> COMPLETED is forbidden (terminal state)"},
		{domain.TxStateInterrupted, domain.TxStateFailed, "INTERRUPTED -> FAILED is forbidden (terminal state)"},
		{domain.TxStateManualRecovery, domain.TxStateExecuting, "MANUAL_RECOVERY -> EXECUTING is forbidden (no automatic replay)"},
		{domain.TxStateManualRecovery, domain.TxStateCompleted, "MANUAL_RECOVERY -> COMPLETED is forbidden (terminal state)"},
		{domain.TxStateManualRecovery, domain.TxStateFailed, "MANUAL_RECOVERY -> FAILED is forbidden (terminal state)"},
		{domain.TxStateManualRecovery, domain.TxStateRecoveryRequired, "MANUAL_RECOVERY -> RECOVERY_REQUIRED is forbidden (terminal state)"},
		{domain.TxStateManualRecovery, domain.TxStatePlanned, "MANUAL_RECOVERY -> PLANNED is forbidden (terminal state)"},
		{domain.TxStateCompleted, domain.TxStateExecuting, "COMPLETED -> EXECUTING is forbidden"},
		{domain.TxStateCancelled, domain.TxStateExecuting, "CANCELLED -> EXECUTING is forbidden"},
		{domain.TxStateRecoveryRequired, domain.TxStateExecuting, "RECOVERY_REQUIRED -> EXECUTING is forbidden (no automatic replay)"},
		{domain.TxStateRecoveryRequired, domain.TxStateInterrupted, "RECOVERY_REQUIRED -> INTERRUPTED is forbidden (new transitions into legacy INTERRUPTED are rejected)"},
		{"BOGUS_STATE", domain.TxStateExecuting, "unrecognized from state"},
		{domain.TxStatePlanned, "BOGUS_STATE", "unrecognized to state"},
	}

	for _, tc := range illegalPairs {
		t.Run(tc.desc, func(t *testing.T) {
			err := domain.ValidateTransition(tc.from, tc.to)
			if err == nil {
				t.Fatalf("expected illegal transition error for %s -> %s, got nil", tc.from, tc.to)
			}
			if !errors.Is(err, domain.ErrInvalidTransactionState) {
				t.Fatalf("expected ErrInvalidTransactionState, got: %v", err)
			}
		})
	}
}

func TestValidateTransition_LegalTransitions(t *testing.T) {
	legalSequences := []struct {
		from domain.TransactionState
		to   domain.TransactionState
	}{
		{"", domain.TxStatePlanned},
		{domain.TxStatePlanned, domain.TxStateReviewed},
		{domain.TxStatePlanned, domain.TxStateConfirmed},
		{domain.TxStatePlanned, domain.TxStateCancelled},
		{domain.TxStatePlanned, domain.TxStateFailed},
		{domain.TxStateReviewed, domain.TxStateConfirmed},
		{domain.TxStateReviewed, domain.TxStateCancelled},
		{domain.TxStateConfirmed, domain.TxStateAuthorized},
		{domain.TxStateConfirmed, domain.TxStateExecuting},
		{domain.TxStateConfirmed, domain.TxStateCancelled},
		{domain.TxStateAuthorized, domain.TxStateExecuting},
		{domain.TxStateAuthorized, domain.TxStateCancelled},
		{domain.TxStateExecuting, domain.TxStateCompleted},
		{domain.TxStateExecuting, domain.TxStateFailed},
		{domain.TxStateExecuting, domain.TxStateRecoveryRequired},
		{domain.TxStateRecoveryRequired, domain.TxStateManualRecovery},
		{domain.TxStateRecoveryRequired, domain.TxStateCompleted},
		{domain.TxStateRecoveryRequired, domain.TxStateFailed},
	}

	for _, tc := range legalSequences {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			if err := domain.ValidateTransition(tc.from, tc.to); err != nil {
				t.Fatalf("expected legal transition %s -> %s to succeed, got: %v", tc.from, tc.to, err)
			}
		})
	}
}

func TestTransactionState_IsValidAndTerminal(t *testing.T) {
	validStates := []domain.TransactionState{
		domain.TxStatePlanned,
		domain.TxStateReviewed,
		domain.TxStateConfirmed,
		domain.TxStateAuthorized,
		domain.TxStateExecuting,
		domain.TxStateCompleted,
		domain.TxStateFailed,
		domain.TxStateCancelled,
		domain.TxStateRecoveryRequired,
		domain.TxStateManualRecovery,
		domain.TxStateInterrupted,
	}

	for _, s := range validStates {
		if !s.IsValid() {
			t.Errorf("expected state %s to be valid", s)
		}
	}

	if domain.TransactionState("UNKNOWN").IsValid() {
		t.Error("expected UNKNOWN state to be invalid")
	}

	terminalStates := []domain.TransactionState{
		domain.TxStateCompleted,
		domain.TxStateFailed,
		domain.TxStateCancelled,
		domain.TxStateInterrupted,
		domain.TxStateManualRecovery,
	}

	for _, s := range terminalStates {
		if !s.IsTerminal() {
			t.Errorf("expected state %s to be terminal", s)
		}
	}

	nonTerminal := []domain.TransactionState{
		domain.TxStatePlanned,
		domain.TxStateReviewed,
		domain.TxStateConfirmed,
		domain.TxStateAuthorized,
		domain.TxStateExecuting,
		domain.TxStateRecoveryRequired,
	}

	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("expected state %s to be non-terminal", s)
		}
	}
}
