package scheduler

import (
	"bytes"
	"strings"
	"testing"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
)

// TestAddFunc_BadSpecLogged verifies that a malformed cron spec is not silently
// swallowed: the error must be surfaced via the logger so a misconfigured job
// (cert renewal / traffic / cleanup) is visible instead of quietly never running.
// ponytail: tests the shared registration helper directly — registerJobs routes
// every AddFunc through it, so this proves the error path; add a registerJobs
// integration test if a job ever bypasses the helper.
func TestAddFunc_BadSpecLogged(t *testing.T) {
	var buf bytes.Buffer
	s := &Scheduler{
		cron:   cron.New(cron.WithSeconds()),
		logger: zerolog.New(&buf),
	}

	// "* * *" has 3 fields; WithSeconds() requires 6 → Parse error.
	s.addFunc("testjob", "* * *", func() {})

	got := buf.String()
	if !strings.Contains(got, "register cron job") {
		t.Fatalf("expected log to contain %q, got: %s", "register cron job", got)
	}
	if !strings.Contains(got, "testjob") {
		t.Fatalf("expected log to contain job name %q, got: %s", "testjob", got)
	}
}

// TestAddFunc_ValidSpecSilent verifies a good spec registers without logging an
// error, so we don't noise up the logs on the happy path.
func TestAddFunc_ValidSpecSilent(t *testing.T) {
	var buf bytes.Buffer
	s := &Scheduler{
		cron:   cron.New(cron.WithSeconds()),
		logger: zerolog.New(&buf),
	}

	s.addFunc("goodjob", "*/15 * * * * *", func() {})

	if got := buf.String(); strings.Contains(got, "register cron job") {
		t.Fatalf("valid spec must not log an error, got: %s", got)
	}
}
