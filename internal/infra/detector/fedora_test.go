package detector_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/infra/detector"
)

const sampleFedoraOSRelease = `NAME="Fedora Linux"
VERSION="44 (Workstation Edition)"
ID=fedora
VERSION_ID=44
VERSION_CODENAME=""
PLATFORM_ID="platform:f44"
PRETTY_NAME="Fedora Linux 44 (Workstation Edition)"
ANSI_COLOR="0;38;2;60;110;180"
LOGO=fedora-logo-icon
CPE_NAME="cpe:/o:fedoraproject:fedora:44"
HOME_URL="https://fedoraproject.org/"
DOCUMENTATION_URL="https://docs.fedoraproject.org/en-US/fedora/f44/"
SUPPORT_URL="https://ask.fedoraproject.org/"
BUG_REPORT_URL="https://bugzilla.redhat.com/"
REDHAT_BUGZILLA_PRODUCT="Fedora"
REDHAT_BUGZILLA_PRODUCT_VERSION=44
REDHAT_SUPPORT_PRODUCT="Fedora"
REDHAT_SUPPORT_PRODUCT_VERSION=44
SUPPORT_END=2027-05-19
VARIANT="Workstation Edition"
VARIANT_ID=workstation
`

const sampleUbuntuOSRelease = `NAME="Ubuntu"
VERSION="24.04 LTS (Noble Numbat)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 24.04 LTS"
VERSION_ID="24.04"
`

func TestParseOSRelease_Fedora(t *testing.T) {
	rel, err := detector.ParseOSRelease(strings.NewReader(sampleFedoraOSRelease))
	if err != nil {
		t.Fatalf("unexpected error parsing Fedora os-release: %v", err)
	}

	if rel.ID != "fedora" {
		t.Errorf("expected ID 'fedora', got %q", rel.ID)
	}
	if rel.VersionID != "44" {
		t.Errorf("expected VersionID '44', got %q", rel.VersionID)
	}
	if rel.PrettyName != "Fedora Linux 44 (Workstation Edition)" {
		t.Errorf("expected PrettyName 'Fedora Linux 44 (Workstation Edition)', got %q", rel.PrettyName)
	}
	if rel.VariantID != "workstation" {
		t.Errorf("expected VariantID 'workstation', got %q", rel.VariantID)
	}
	if !rel.IsFedora() {
		t.Error("expected IsFedora() to be true")
	}
}

func TestParseOSRelease_NonFedora(t *testing.T) {
	rel, err := detector.ParseOSRelease(strings.NewReader(sampleUbuntuOSRelease))
	if err != nil {
		t.Fatalf("unexpected error parsing Ubuntu os-release: %v", err)
	}

	if rel.ID != "ubuntu" {
		t.Errorf("expected ID 'ubuntu', got %q", rel.ID)
	}
	if rel.IsFedora() {
		t.Error("expected IsFedora() to be false for Ubuntu")
	}
}

func TestFedoraDetector_ContextCancellation(t *testing.T) {
	d := detector.NewFedoraDetector()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := d.Detect(ctx)
	if err == nil {
		t.Error("expected error on cancelled context, got nil")
	}
}

func TestFedoraDetector_HostDetection(t *testing.T) {
	// On this Fedora host, NewFedoraDetector should successfully detect the system
	d := detector.NewFedoraDetector()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	info, err := d.Detect(ctx)
	if err != nil {
		t.Fatalf("unexpected error on host detection: %v", err)
	}

	if info.OS.ID == "" {
		t.Error("expected non-empty OS ID from host")
	}
	if info.Architecture == "" {
		t.Error("expected non-empty architecture")
	}
}
