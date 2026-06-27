// Package version detects deploy type (docker vs ssh) by mirroring Java's
// VersionCheckTask probe of /.dockerenv. See SPEC §17.2.
package version

import "os"

// DetectDeployType returns the override if non-empty, else "docker" when
// /.dockerenv exists, else "ssh".
func DetectDeployType(override string) string {
	if override != "" {
		return override
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	return "ssh"
}
