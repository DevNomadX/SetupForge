package update

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
)

// dnf5UpgradePayload matches the structured JSON produced by `dnf5 check-upgrade --json`.
type dnf5UpgradePayload struct {
	Upgrades  []dnf5PackageEntry `json:"upgrades"`
	Obsoletes []dnf5PackageEntry `json:"obsoletes"`
}

type dnf5PackageEntry struct {
	Name       string `json:"name"`
	Arch       string `json:"arch"`
	EVR        string `json:"evr"`
	Repository string `json:"repository"`
}

// ParseDNF5CheckUpgradeJSON parses JSON output from `dnf5 check-upgrade --json` into a domain.UpdateStatus.
func ParseDNF5CheckUpgradeJSON(data []byte) (domain.UpdateStatus, error) {
	trimmed := bytes.TrimSpace(data)

	// An empty response or empty JSON object signifies that the system is fully up to date.
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("{}")) {
		return domain.UpdateStatus{
			State:       domain.UpdateStateUpToDate,
			UpdateCount: 0,
			Updates:     nil,
			LastChecked: time.Now(),
		}, nil
	}

	var payload dnf5UpgradePayload
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		return domain.UpdateStatus{
			State:       domain.UpdateStateFailed,
			LastChecked: time.Now(),
			ErrorMsg:    fmt.Sprintf("parsing update JSON: %v", err),
		}, fmt.Errorf("%w: %v", domain.ErrMalformedUpdateOutput, err)
	}

	if len(payload.Upgrades) == 0 {
		return domain.UpdateStatus{
			State:       domain.UpdateStateUpToDate,
			UpdateCount: 0,
			Updates:     nil,
			LastChecked: time.Now(),
		}, nil
	}

	updates := make([]domain.PackageUpdate, len(payload.Upgrades))
	for i, u := range payload.Upgrades {
		updates[i] = domain.PackageUpdate{
			Name:       u.Name,
			Arch:       u.Arch,
			EVR:        u.EVR,
			Repository: u.Repository,
		}
	}

	return domain.UpdateStatus{
		State:       domain.UpdateStateAvailable,
		UpdateCount: len(updates),
		Updates:     updates,
		LastChecked: time.Now(),
	}, nil
}
