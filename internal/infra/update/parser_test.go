package update_test

import (
	"errors"
	"testing"

	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/update"
)

func TestParseDNF5CheckUpgradeJSON_UpToDate(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty bytes", data: []byte("")},
		{name: "whitespace bytes", data: []byte("   \n\t  ")},
		{name: "empty json object", data: []byte("{}")},
		{name: "empty upgrades array", data: []byte(`{"upgrades":[]}`)},
		{name: "empty upgrades array with whitespace", data: []byte("  \n {\"upgrades\": []} \n ")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, err := update.ParseDNF5CheckUpgradeJSON(tc.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status.State != domain.UpdateStateUpToDate {
				t.Errorf("expected state %q, got %q", domain.UpdateStateUpToDate, status.State)
			}
			if status.UpdateCount != 0 {
				t.Errorf("expected count 0, got %d", status.UpdateCount)
			}
			if len(status.Updates) != 0 {
				t.Errorf("expected empty updates slice, got %v", status.Updates)
			}
		})
	}
}

func TestParseDNF5CheckUpgradeJSON_Available(t *testing.T) {
	jsonFixture := []byte(`{
		"upgrades": [
			{
				"name": "google-noto-color-emoji-fonts",
				"arch": "noarch",
				"evr": "20260921-1.fc44",
				"repository": "updates"
			},
			{
				"name": "vlc",
				"arch": "x86_64",
				"evr": "1:3.0.24-1.fc44",
				"repository": "updates"
			},
			{
				"name": "vlc-plugins-freeworld",
				"arch": "x86_64",
				"evr": "3.0.24-1.fc44",
				"repository": "rpmfusion-free-updates"
			}
		]
	}`)

	status, err := update.ParseDNF5CheckUpgradeJSON(jsonFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.State != domain.UpdateStateAvailable {
		t.Errorf("expected state %q, got %q", domain.UpdateStateAvailable, status.State)
	}
	if status.UpdateCount != 3 {
		t.Errorf("expected count 3, got %d", status.UpdateCount)
	}
	if len(status.Updates) != 3 {
		t.Fatalf("expected 3 updates, got %d", len(status.Updates))
	}

	first := status.Updates[0]
	if first.Name != "google-noto-color-emoji-fonts" || first.Arch != "noarch" || first.EVR != "20260921-1.fc44" || first.Repository != "updates" {
		t.Errorf("first update fields mismatch: %+v", first)
	}

	third := status.Updates[2]
	if third.Name != "vlc-plugins-freeworld" || third.Repository != "rpmfusion-free-updates" {
		t.Errorf("third update fields mismatch: %+v", third)
	}
}

func TestParseDNF5CheckUpgradeJSON_Malformed(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{name: "invalid json syntax", data: []byte(`{"upgrades": [`)},
		{name: "string instead of object", data: []byte(`"not-an-object"`)},
		{name: "wrong upgrades field type", data: []byte(`{"upgrades": "not-a-list"}`)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, err := update.ParseDNF5CheckUpgradeJSON(tc.data)
			if err == nil {
				t.Fatalf("expected error for malformed json, got nil")
			}
			if !errors.Is(err, domain.ErrMalformedUpdateOutput) {
				t.Errorf("expected ErrMalformedUpdateOutput, got %v", err)
			}
			if status.State != domain.UpdateStateFailed {
				t.Errorf("expected state %q, got %q", domain.UpdateStateFailed, status.State)
			}
			if status.ErrorMsg == "" {
				t.Errorf("expected non-empty ErrorMsg on failure")
			}
		})
	}
}
