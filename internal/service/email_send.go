// Package service -- email_send.go: Phase 12.2 SMTP email sending.
// Parallel sending via net/smtp with goroutines. Port of Java
// EmailServiceImpl.send + CompletableFuture pattern.
package service

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"math/rand"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/Muione/oci-start-go/internal/repo"
)

// EmailSendResult holds the per-recipient send result.
type EmailSendResult struct {
	EmailSendRecordID string
	Email             string
	Success           bool
	Message           string
}

// SendEmailInput carries the email send request payload.
type SendEmailInput struct {
	TenantEmailConfigID int64   `json:"tenantEmailConfigId"`
	Title               string  `json:"title"`
	Content             string  `json:"content"`
	EmailReceiveIds     []int64 `json:"emailReceiveIds"`
}

// Send orchestrates the full email sending flow: validate, create records,
// send in parallel, update results.
func (s *EmailService) Send(ctx context.Context, in SendEmailInput) (*EmailBodyResp, error) {
	if in.Title == "" || in.Content == "" {
		return nil, fmt.Errorf("title and content required")
	}
	if len(in.EmailReceiveIds) == 0 {
		return nil, fmt.Errorf("at least one recipient required")
	}

	// 1. Load tenant_email_config.
	cfg, err := repo.New(s.store.Read).FindTenantEmailConfigById(ctx, in.TenantEmailConfigID)
	if err != nil {
		return nil, fmt.Errorf("find email config: %w", err)
	}
	if ni(cfg.Active) == 0 {
		return nil, fmt.Errorf("email service is not active for this config")
	}

	// 2. Load tenant.
	tenantID := ni(cfg.TenantID)
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant: %w", err)
	}
	tenantName := ns(t.UserName)

	// 3. Check daily limit.
	if err := s.CheckAndResetDailyCount(ctx, &cfg); err != nil {
		return nil, err
	}

	// 4. Load email_receive rows.
	receives, err := s.loadReceives(ctx, in.EmailReceiveIds)
	if err != nil {
		return nil, err
	}
	if len(receives) == 0 {
		return nil, fmt.Errorf("no valid recipients found")
	}

	// 5. Generate snowflake-like ID for emailBodyID.
	emailBodyID := generateEmailID()

	// 6. Create email_body record.
	now := time.Now().Format(httpTimeFmt)
	q := repo.New(s.store.Write)
	if err := q.InsertEmailBody(ctx, repo.InsertEmailBodyParams{
		EmailBodyID:         emailBodyID,
		CurrentVersion:      nullInt64(tenantID),
		TenantName:          nullStr(tenantName),
		TenantEmailConfigID: nullInt64(in.TenantEmailConfigID),
		SenderEmail:         cfg.SenderEmail,
		Title:               nullStr(in.Title),
		Content:             nullStr(in.Content),
		ReceiveTotal:        nullInt64(int64(len(receives))),
		ReceiveSuccessTotal: nullInt64(0),
		ReceiveFailTotal:    nullInt64(0),
		CreateTime:          nullStr(now),
	}); err != nil {
		return nil, fmt.Errorf("insert email body: %w", err)
	}

	// 7. Create email_send_record rows (one per recipient, state=0).
	for _, rec := range receives {
		recordID := generateEmailID()
		_ = q.InsertEmailSendRecord(ctx, repo.InsertEmailSendRecordParams{
			EmailSendRecordID:   nullStr(recordID),
			EmailBodyID:         nullStr(emailBodyID),
			EmailSendAddress:    cfg.SenderEmail,
			CurrentVersion:      nullInt64(tenantID),
			TenantName:          nullStr(tenantName),
			EmailReceiveID:      sql.NullInt64{Int64: rec.ID, Valid: true},
			ReceiveEmailAddress: nullStr(rec.Email),
			SendState:           nullInt64(0),
			CreateTime:          nullStr(now),
		})
	}

	// 8. Build send records for parallel sending.
	sendRecords := make([]sendRecord, len(receives))
	for i, rec := range receives {
		sendRecords[i] = sendRecord{
			RecordID:       generateEmailID(),
			ReceiveID:      rec.ID,
			ReceiveEmail:   rec.Email,
			ReceiveName:    rec.Name,
			SenderEmail:    ns(cfg.SenderEmail),
			SmtpHost:       ns(cfg.SmtpHost),
			SmtpPort:       toInt64(ns(cfg.SmtpPort)),
			SmtpUsername:   ns(cfg.SmtpUsername),
			SmtpPassword:   ns(cfg.SmtpPassword),
			Title:          in.Title,
			Content:        in.Content,
			TenantID:       tenantID,
			TenantName:     tenantName,
			EmailBodyID:    emailBodyID,
			ConfigID:       in.TenantEmailConfigID,
		}
	}

	// 9. Send emails in parallel.
	results := s.sendEmailsParallel(ctx, sendRecords)

	// 10. Update records and totals.
	var successCount, failCount int64
	for _, result := range results {
		state := int64(0)
		if result.Success {
			state = 1
			successCount++
		} else {
			failCount++
		}
		// Update the send record state.
		_ = q.UpdateEmailSendRecordState(ctx, repo.UpdateEmailSendRecordStateParams{
			SendState:         nullInt64(state),
			EmailSendRecordID: nullStr(result.EmailSendRecordID),
		})
	}

	// 11. Update email_body totals.
	_ = q.UpdateEmailBodyTotals(ctx, repo.UpdateEmailBodyTotalsParams{
		ReceiveSuccessTotal: nullInt64(successCount),
		ReceiveFailTotal:    nullInt64(failCount),
		EmailBodyID:         emailBodyID,
	})

	// 12. Update tenant_email_config today_sent_count.
	_ = s.IncrementSentCount(ctx, in.TenantEmailConfigID, successCount)

	// Return the email body info.
	body, _ := q.FindEmailBodyById(ctx, emailBodyID)
	return &EmailBodyResp{
		ID:                  body.ID,
		EmailBodyID:         body.EmailBodyID,
		TenantName:          ns(body.TenantName),
		TenantEmailConfigID: ni(body.TenantEmailConfigID),
		SenderEmail:         ns(body.SenderEmail),
		Title:               ns(body.Title),
		Content:             ns(body.Content),
		ReceiveTotal:        ni(body.ReceiveTotal),
		ReceiveSuccessTotal: successCount,
		ReceiveFailTotal:    failCount,
		CreateTime:          ns(body.CreateTime),
	}, nil
}

// sendRecord holds the data needed to send one email.
type sendRecord struct {
	RecordID     string
	ReceiveID    int64
	ReceiveEmail string
	ReceiveName  string
	SenderEmail  string
	SmtpHost     string
	SmtpPort     int64
	SmtpUsername string
	SmtpPassword string
	Title        string
	Content      string
	TenantID     int64
	TenantName   string
	EmailBodyID  string
	ConfigID     int64
}

// sendEmailsParallel sends emails concurrently using goroutines.
func (s *EmailService) sendEmailsParallel(ctx context.Context, records []sendRecord) []EmailSendResult {
	results := make([]EmailSendResult, len(records))
	var wg sync.WaitGroup

	for i, rec := range records {
		wg.Add(1)
		go func(idx int, r sendRecord) {
			defer wg.Done()
			err := sendOneEmail(r.SmtpHost, int(r.SmtpPort), r.SmtpUsername, r.SmtpPassword,
				r.SenderEmail, r.ReceiveEmail, r.Title, r.Content)
			results[idx] = EmailSendRecordResult(r.RecordID, r.ReceiveEmail, err)
		}(i, rec)
	}
	wg.Wait()
	return results
}

// EmailSendRecordResult creates an EmailSendResult from a send attempt.
func EmailSendRecordResult(recordID, email string, err error) EmailSendResult {
	if err != nil {
		return EmailSendResult{
			EmailSendRecordID: recordID,
			Email:             email,
			Success:           false,
			Message:           err.Error(),
		}
	}
	return EmailSendResult{
		EmailSendRecordID: recordID,
		Email:             email,
		Success:           true,
	}
}

// sendOneEmail sends a single email via SMTP with STARTTLS.
func sendOneEmail(host string, port int, username, password, from, to, subject, body string) error {
	if port <= 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	// Build RFC 822 message.
	msg := buildEmailMessage(from, to, subject, body)

	// Use net/smtp with PLAIN auth. Go's SendMail handles STARTTLS automatically.
	auth := smtp.PlainAuth("", username, password, host)

	// Set up TLS config to skip verification for OCI SMTP servers.
	// This matches the Java implementation which also doesn't verify.
	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
	}

	return sendMailWithTLS(addr, auth, from, []string{to}, msg, tlsConfig)
}

// buildEmailMessage constructs a RFC 822 email message.
func buildEmailMessage(from, to, subject, body string) []byte {
	header := make(map[string]string)
	header["From"] = from
	header["To"] = to
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"UTF-8\""
	header["Date"] = time.Now().Format(time.RFC1123Z)

	var msg strings.Builder
	for k, v := range header {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)
	return []byte(msg.String())
}

// sendMailWithTLS is a wrapper around net/smtp that supports STARTTLS with
// custom TLS config. This provides more control than smtp.SendMail.
func sendMailWithTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte, tlsConfig *tls.Config) error {
	// Connect to the SMTP server.
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("split host port: %w", err)
	}

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	// STARTTLS.
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	// Auth.
	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("auth: %w", err)
			}
		}
	}

	// Send.
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("rcpt to %s: %w", addr, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}

	return client.Quit()
}

// loadReceives loads email_receive rows by IDs.
func (s *EmailService) loadReceives(ctx context.Context, ids []int64) ([]repo.EmailReceive, error) {
	var results []repo.EmailReceive
	q := repo.New(s.store.Read)
	for _, id := range ids {
		r, err := q.FindEmailReceiveById(ctx, id)
		if err != nil {
			continue // skip invalid IDs
		}
		results = append(results, r)
	}
	return results, nil
}

// generateEmailID generates a unique ID for email records.
// Uses timestamp + random string as a snowflake-like ID.
func generateEmailID() string {
	now := time.Now().UnixNano() / 1e6 // milliseconds
	return fmt.Sprintf("%d%s", now, randomAlphaNum(6))
}

// randomAlphaNum generates a random alphanumeric string of length n.
func randomAlphaNum(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
