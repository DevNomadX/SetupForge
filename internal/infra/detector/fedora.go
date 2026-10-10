package detector

import (
	"bufio"
	"context"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/setupforge/setupforge/internal/domain"
)

// FedoraDetector discovers host platform properties by safely inspecting
// standardized release files without spawning external shell processes.
type FedoraDetector struct {
	osReleasePaths []string
}

// NewFedoraDetector creates a detector with standard Linux os-release search paths.
func NewFedoraDetector() *FedoraDetector {
	return &FedoraDetector{
		osReleasePaths: []string{
			"/etc/os-release",
			"/usr/lib/os-release",
		},
	}
}

// NewCustomFedoraDetector creates a detector with explicit paths, primarily for tests.
func NewCustomFedoraDetector(paths ...string) *FedoraDetector {
	return &FedoraDetector{
		osReleasePaths: paths,
	}
}

// Detect reads release information and returns structured domain.SystemInfo.
func (d *FedoraDetector) Detect(ctx context.Context) (domain.SystemInfo, error) {
	select {
	case <-ctx.Done():
		return domain.SystemInfo{}, ctx.Err()
	default:
	}

	release, err := d.readOSRelease()
	if err != nil {
		return domain.SystemInfo{}, err
	}

	hostname, _ := os.Hostname()
	kernel := d.readKernel()

	info := domain.SystemInfo{
		OS:           release,
		Kernel:       kernel,
		Architecture: runtime.GOARCH,
		Hostname:     hostname,
		IsSupported:  release.IsFedora(),
	}

	return info, nil
}

func (d *FedoraDetector) readOSRelease() (domain.OSRelease, error) {
	var file *os.File
	var err error

	for _, path := range d.osReleasePaths {
		file, err = os.Open(path)
		if err == nil {
			break
		}
	}

	if file == nil {
		return domain.OSRelease{}, domain.ErrOSReleaseNotFound
	}
	defer file.Close()

	return ParseOSRelease(file)
}

// ParseOSRelease parses key-value pairs formatted according to the freedesktop os-release specification.
func ParseOSRelease(r io.Reader) (domain.OSRelease, error) {
	scanner := bufio.NewScanner(r)
	fields := make(map[string]string)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Strip surrounding single or double quotes
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		fields[key] = val
	}

	if err := scanner.Err(); err != nil {
		return domain.OSRelease{}, err
	}

	release := domain.OSRelease{
		ID:           fields["ID"],
		IDLike:       fields["ID_LIKE"],
		Name:         fields["NAME"],
		Version:      fields["VERSION"],
		VersionID:    fields["VERSION_ID"],
		Variant:      fields["VARIANT"],
		VariantID:    fields["VARIANT_ID"],
		PrettyName:   fields["PRETTY_NAME"],
		HomeURL:      fields["HOME_URL"],
		BugReportURL: fields["BUG_REPORT_URL"],
		SupportEnd:   fields["SUPPORT_END"],
	}

	if release.PrettyName == "" {
		if release.Name != "" && release.Version != "" {
			release.PrettyName = release.Name + " " + release.Version
		} else if release.Name != "" {
			release.PrettyName = release.Name
		} else {
			release.PrettyName = "Linux"
		}
	}

	return release, nil
}

func (d *FedoraDetector) readKernel() string {
	// Attempt to read from /proc/sys/kernel/osrelease first (clean, no proc fork)
	if data, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		return strings.TrimSpace(string(data))
	}
	return "Linux"
}
