// Package service -- email_send_test.go: E1 regression. Send() must surface DB
// write failures (UpdateEmailBodyTotals/IncrementSentCount/per-recipient record
// inserts) instead of returning nil with a half-persisted, zero-state record.
package service

import (
	"context"
	"testing"
	"time"
)

// TestEmailSend_DBWriteFailureSurfacesError forces the swallowed
// UpdateEmailBodyTotals write to fail (an ABORT trigger on email_body UPDATE)
// and asserts Send returns a non-nil error. Before the fix, Send swallowed the
// error and returned nil, so the caller could not tell a fully-persisted send
// from a totally-failed one.
func TestEmailSend_DBWriteFailureSurfacesError(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	// Seed tenant + active email config (SMTP points at a refused port so the
	// real net/smtp dial fails fast without network egress).
	mustExec(t, store, `INSERT INTO tenant (id, tenant_id, user_name, tenancy, region, created_at, is_active)
		VALUES (1, 'ocid1.tenancy.oc1..aaa', 'test-tenant', 'tenancy-aaa', 'us-phoenix-1', ?, 1)`, today+" 00:00:00")
	mustExec(t, store, `INSERT INTO tenant_email_config
		(id, tenant_id, active, smtp_host, smtp_port, smtp_username, smtp_password, sender_email,
		 daily_email_limit, today_sent_count, last_reset_date)
		VALUES (1, 1, 1, '127.0.0.1', '1', 'u', 'p', 'noreply@x.test', 200, 0, ?)`, today)
	mustExec(t, store, `INSERT INTO email_receive (id, email, name) VALUES (1, 'a@b.test', 'A'), (2, 'c@d.test', 'C')`)

	// Make every UPDATE on email_body abort. INSERT (InsertEmailBody) still
	// succeeds, so Send reaches the swallowed UpdateEmailBodyTotals call.
	mustExec(t, store, `CREATE TRIGGER fail_email_body_update BEFORE UPDATE ON email_body
		BEGIN SELECT RAISE(ABORT, 'simulated write failure'); END`)

	svc := NewEmailService(store, nil, nil, nil, nil)
	_, err := svc.Send(ctx, SendEmailInput{
		TenantEmailConfigID: 1,
		Title:               "t",
		Content:             "c",
		EmailReceiveIds:     []int64{1, 2},
	})
	if err == nil {
		t.Fatal("Send returned nil error after a failed DB write; expected the persistence failure to surface")
	}
}
