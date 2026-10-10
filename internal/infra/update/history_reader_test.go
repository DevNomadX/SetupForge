package update_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/update"
)

func TestParseDNF5HistoryJSON_ValidArray(t *testing.T) {
	fixture := `[
  {
    "id": 56,
    "start_time": 1791213555,
    "end_time": 1791213569,
    "rpmdb_version_begin": "ba862e08811af5801be449dec1ff67c51f3dc4be531e28da1d0538f90f386a28",
    "rpmdb_version_end": "235d6530cdb822d4bc02408ca45d2778dbf43c0884f5bd32848ee9615c232325",
    "user_id": 1000,
    "user_name": "1000 SetupForge Test <testuser>",
    "status": "Ok",
    "releasever": "44",
    "description": "dnf up --refresh",
    "comment": "setupforge:test-op-1",
    "packages": [
      {
        "nevra": "pkg-a-1.0-1.fc44.x86_64",
        "action": "Upgrade",
        "reason": "User",
        "repository": "updates"
      },
      {
        "nevra": "pkg-b-2.0-1.fc44.x86_64",
        "action": "Upgrade",
        "reason": "Dependency",
        "repository": "updates"
      }
    ]
  },
  {
    "id": 55,
    "start_time": 1791126015,
    "end_time": null,
    "rpmdb_version_begin": "begin-hash",
    "rpmdb_version_end": null,
    "user_id": 0,
    "user_name": "root",
    "status": "Started",
    "releasever": "44",
    "description": "dnf install test",
    "comment": "setupforge:test-op-2",
    "packages": []
  }
]`

	records, err := update.ParseDNF5HistoryJSON(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error parsing valid JSON: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// Verify Record 1
	r1 := records[0]
	if r1.ID != 56 {
		t.Errorf("expected ID 56, got %d", r1.ID)
	}
	if r1.State != "Ok" || r1.StateID != 2 {
		t.Errorf("expected Ok/2, got state=%q, stateID=%d", r1.State, r1.StateID)
	}
	if r1.StartTime != time.Unix(1791213555, 0) {
		t.Errorf("expected StartTime %v, got %v", time.Unix(1791213555, 0), r1.StartTime)
	}
	if r1.EndTime == nil || *r1.EndTime != time.Unix(1791213569, 0) {
		t.Errorf("expected EndTime %v, got %v", time.Unix(1791213569, 0), r1.EndTime)
	}
	if r1.RPMDBVersionBegin != "ba862e08811af5801be449dec1ff67c51f3dc4be531e28da1d0538f90f386a28" {
		t.Errorf("unexpected RPMDBVersionBegin: %q", r1.RPMDBVersionBegin)
	}
	if r1.RPMDBVersionEnd != "235d6530cdb822d4bc02408ca45d2778dbf43c0884f5bd32848ee9615c232325" {
		t.Errorf("unexpected RPMDBVersionEnd: %q", r1.RPMDBVersionEnd)
	}
	if r1.Comment != "setupforge:test-op-1" {
		t.Errorf("unexpected Comment: %q", r1.Comment)
	}
	if r1.AlteredCount != 2 {
		t.Errorf("expected AlteredCount 2, got %d", r1.AlteredCount)
	}

	// Verify Record 2 (inconclusive / null end time)
	r2 := records[1]
	if r2.ID != 55 {
		t.Errorf("expected ID 55, got %d", r2.ID)
	}
	if r2.State != "Started" || r2.StateID != 1 {
		t.Errorf("expected Started/1, got state=%q, stateID=%d", r2.State, r2.StateID)
	}
	if r2.EndTime != nil {
		t.Errorf("expected nil EndTime for null end_time, got %v", r2.EndTime)
	}
	if r2.RPMDBVersionEnd != "" {
		t.Errorf("expected empty RPMDBVersionEnd for null, got %q", r2.RPMDBVersionEnd)
	}
}

func TestParseDNF5HistoryJSON_SingleObject(t *testing.T) {
	fixture := `{
    "id": 10,
    "start_time": 1700000000,
    "end_time": 1700000010,
    "status": "Ok",
    "comment": "setupforge:single-object"
  }`

	records, err := update.ParseDNF5HistoryJSON(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error parsing single object JSON: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ID != 10 {
		t.Errorf("expected ID 10, got %d", records[0].ID)
	}
}

func TestParseDNF5HistoryJSON_EmptyAndWhitespace(t *testing.T) {
	cases := []string{"", "   \n\t  ", "[]"}
	for _, c := range cases {
		records, err := update.ParseDNF5HistoryJSON(strings.NewReader(c))
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", c, err)
		}
		if len(records) != 0 {
			t.Fatalf("expected 0 records for %q, got %d", c, len(records))
		}
	}
}

func TestParseDNF5HistoryJSON_MalformedAndInvalid(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"MalformedJSON", `[{"id": 1, "start_time": 100, broken`},
		{"NonJSONToken", `Error: command failed`},
		{"InvalidID", `[{"id": 0, "start_time": 100, "status": "Ok"}]`},
		{"NegativeID", `[{"id": -5, "start_time": 100, "status": "Ok"}]`},
		{"InvalidStartTime", `[{"id": 1, "start_time": 0, "status": "Ok"}]`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := update.ParseDNF5HistoryJSON(strings.NewReader(tc.content))
			if err == nil {
				t.Fatalf("expected error for case %q, got nil", tc.name)
			}
			if !errors.Is(err, domain.ErrCorruptedHistory) {
				t.Fatalf("expected error wrapping ErrCorruptedHistory, got %v", err)
			}
		})
	}
}

func TestParseDNF5HistoryJSON_ExceedsMaxLimit(t *testing.T) {
	// Construct data slightly exceeding 10MB
	oversized := bytes.Repeat([]byte(" "), update.MaxHistoryReadBytes+10)
	_, err := update.ParseDNF5HistoryJSON(bytes.NewReader(oversized))
	if err == nil {
		t.Fatal("expected error for oversized input, got nil")
	}
	if !errors.Is(err, domain.ErrCorruptedHistory) {
		t.Fatalf("expected error wrapping ErrCorruptedHistory, got %v", err)
	}
}

func TestDNF5HistoryReader_QueryHistory(t *testing.T) {
	fixtureJSON := `[
  {"id": 30, "start_time": 1700000000, "status": "Ok", "comment": "setupforge:op-30"},
  {"id": 20, "start_time": 1690000000, "status": "Ok", "comment": "setupforge:op-20"},
  {"id": 10, "start_time": 1680000000, "status": "Error", "comment": "setupforge:op-10"}
]`

	t.Run("QueryAll", func(t *testing.T) {
		mockExec := func(ctx context.Context, executable string, args ...string) ([]byte, error) {
			if executable != "/usr/bin/dnf5" {
				t.Errorf("unexpected executable: %s", executable)
			}
			if len(args) != 3 || args[0] != "history" || args[1] != "info" || args[2] != "--json" {
				t.Errorf("unexpected args: %v", args)
			}
			return []byte(fixtureJSON), nil
		}

		reader := update.NewDNF5HistoryReader(
			update.WithHistoryCommandExecutor(mockExec),
		)

		records, err := reader.QueryHistory(context.Background(), domain.HistoryCriteria{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(records) != 3 {
			t.Fatalf("expected 3 records, got %d", len(records))
		}
	})

	t.Run("QuerySpecificTransactionID", func(t *testing.T) {
		mockExec := func(ctx context.Context, executable string, args ...string) ([]byte, error) {
			expectedArgs := []string{"history", "info", "--json", "20"}
			if strings.Join(args, " ") != strings.Join(expectedArgs, " ") {
				t.Errorf("unexpected args: got %v, expected %v", args, expectedArgs)
			}
			return []byte(`[{"id": 20, "start_time": 1690000000, "status": "Ok", "comment": "setupforge:op-20"}]`), nil
		}

		reader := update.NewDNF5HistoryReader(
			update.WithHistoryCommandExecutor(mockExec),
		)

		records, err := reader.QueryHistory(context.Background(), domain.HistoryCriteria{TransactionID: 20})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(records) != 1 || records[0].ID != 20 {
			t.Fatalf("expected 1 record with ID 20, got %+v", records)
		}
	})

	t.Run("QuerySinceTransIDFilter", func(t *testing.T) {
		mockExec := func(ctx context.Context, executable string, args ...string) ([]byte, error) {
			return []byte(fixtureJSON), nil
		}

		reader := update.NewDNF5HistoryReader(
			update.WithHistoryCommandExecutor(mockExec),
		)

		records, err := reader.QueryHistory(context.Background(), domain.HistoryCriteria{SinceTransID: 15})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Records 30 and 20 are > 15, record 10 is not
		if len(records) != 2 {
			t.Fatalf("expected 2 records > 15, got %d", len(records))
		}
		if records[0].ID != 30 || records[1].ID != 20 {
			t.Fatalf("unexpected records: %+v", records)
		}
	})

	t.Run("QueryWithLimit", func(t *testing.T) {
		mockExec := func(ctx context.Context, executable string, args ...string) ([]byte, error) {
			return []byte(fixtureJSON), nil
		}

		reader := update.NewDNF5HistoryReader(
			update.WithHistoryCommandExecutor(mockExec),
		)

		records, err := reader.QueryHistory(context.Background(), domain.HistoryCriteria{Limit: 1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(records) != 1 || records[0].ID != 30 {
			t.Fatalf("expected 1 record with ID 30, got %+v", records)
		}
	})

	t.Run("ExecutionError_ReturnsHistoryUnavailable", func(t *testing.T) {
		mockExec := func(ctx context.Context, executable string, args ...string) ([]byte, error) {
			return nil, errors.New("exit status 1")
		}

		reader := update.NewDNF5HistoryReader(
			update.WithHistoryCommandExecutor(mockExec),
		)

		_, err := reader.QueryHistory(context.Background(), domain.HistoryCriteria{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, domain.ErrHistoryUnavailable) {
			t.Fatalf("expected error wrapping ErrHistoryUnavailable, got %v", err)
		}
	})
}
