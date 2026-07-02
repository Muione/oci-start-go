package config

import (
	"bytes"
	"os"
	"testing"
)

// captureStderr swaps the package-level stderr sink for the duration of a test
// and returns the buffer it writes to. Config-stage messages (first-run notice,
// DATA_PATH misconfiguration warnings) route here before the logger is up.
func captureStderr(t *testing.T) *bytes.Buffer {
	t.Helper()
	old := stderrOut
	buf := &bytes.Buffer{}
	stderrOut = buf
	t.Cleanup(func() { stderrOut = old })
	return buf
}

// withEnv sets an env var for the duration of a test and restores it after.
func withEnv(t *testing.T, key, val string) {
	t.Helper()
	old, ok := os.LookupEnv(key)
	os.Setenv(key, val)
	t.Cleanup(func() {
		if ok {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

func TestApplyDataPath_Unset_DefaultsToDataDir(t *testing.T) {
	withEnv(t, "DATA_PATH", "")
	cfg := &Config{
		Datasource: DatasourceCfg{URL: "file:./data/vps.db?x"},
		BaseFile:   BaseFileCfg{Path: "./data/upload/"},
	}
	cfg.applyDataPath()
	if cfg.dataDir != "./data" {
		t.Fatalf("dataDir = %q, want %q", cfg.dataDir, "./data")
	}
	if cfg.Datasource.URL != "file:./data/vps.db?x" {
		t.Errorf("URL changed without DATA_PATH: %q", cfg.Datasource.URL)
	}
	if cfg.BaseFile.Path != "./data/upload/" {
		t.Errorf("Path changed without DATA_PATH: %q", cfg.BaseFile.Path)
	}
}

func TestApplyDataPath_RemapsDataPrefix_NoWarning(t *testing.T) {
	withEnv(t, "DATA_PATH", "/opt/data")
	buf := captureStderr(t)
	cfg := &Config{
		Datasource: DatasourceCfg{URL: "file:./data/vps.db?pragma=1"},
		BaseFile:   BaseFileCfg{Path: "./data/upload/"},
	}
	cfg.applyDataPath()

	if cfg.dataDir != "/opt/data" {
		t.Errorf("dataDir = %q, want /opt/data", cfg.dataDir)
	}
	if cfg.Datasource.URL != "file:/opt/data/vps.db?pragma=1" {
		t.Errorf("URL = %q, want file:/opt/data/vps.db?pragma=1", cfg.Datasource.URL)
	}
	if cfg.BaseFile.Path != "/opt/data/upload/" {
		t.Errorf("Path = %q, want /opt/data/upload/", cfg.BaseFile.Path)
	}
	// Clean remap must be silent (no misconfiguration warning).
	if buf.Len() != 0 {
		t.Errorf("expected no warning on clean remap, got: %s", buf.String())
	}
}

func TestApplyDataPath_AbsolutePath_WarnsNotSilent(t *testing.T) {
	withEnv(t, "DATA_PATH", "/opt/data")
	buf := captureStderr(t)
	cfg := &Config{
		Datasource: DatasourceCfg{URL: "file:/var/lib/vps.db?x"},
		BaseFile:   BaseFileCfg{Path: "/var/lib/upload/"},
	}
	cfg.applyDataPath()

	// Values left as-is: no ./data prefix to remap.
	if cfg.Datasource.URL != "file:/var/lib/vps.db?x" {
		t.Errorf("URL = %q, want unchanged file:/var/lib/vps.db?x", cfg.Datasource.URL)
	}
	if cfg.BaseFile.Path != "/var/lib/upload/" {
		t.Errorf("Path = %q, want unchanged /var/lib/upload/", cfg.BaseFile.Path)
	}
	// MUST NOT be a silent no-op: DATA_PATH was set but matched nothing.
	if buf.Len() == 0 {
		t.Errorf("expected DATA_PATH misconfiguration warning on stderr, got none (silent no-op)")
	}
}

func TestApplyDataPath_AlreadyReplacedValue_WarnsNotSilent(t *testing.T) {
	withEnv(t, "DATA_PATH", "/opt/data")
	buf := captureStderr(t)
	cfg := &Config{
		Datasource: DatasourceCfg{URL: "file:/opt/data/vps.db?x"},
		BaseFile:   BaseFileCfg{Path: "/opt/data/upload/"},
	}
	cfg.applyDataPath()

	// No double-replace, no munging of the already-resolved value.
	if cfg.Datasource.URL != "file:/opt/data/vps.db?x" {
		t.Errorf("URL = %q, want unchanged (no double-replace)", cfg.Datasource.URL)
	}
	if cfg.BaseFile.Path != "/opt/data/upload/" {
		t.Errorf("Path = %q, want unchanged (no double-replace)", cfg.BaseFile.Path)
	}
	if buf.Len() == 0 {
		t.Errorf("expected DATA_PATH misconfiguration warning on already-replaced value, got none (silent no-op)")
	}
}
