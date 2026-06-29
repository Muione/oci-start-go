// Package service -- email.go: Phase 12.2 Email Delivery service.
// Orchestrates OCI email domain/sender/DKIM/SMTP provisioning, Cloudflare DNS
// record creation, email sending, and recipient/body/record management.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/dns"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/oci/region"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/sysconf"
)

const defaultDailyEmailLimit = 200

// EmailService manages the full email lifecycle: OCI provisioning, DNS setup,
// recipient management, email body tracking, and SMTP sending.
type EmailService struct {
	store     *db.Store
	dnsSvc    *dns.DnsService
	sysConf   *sysconf.Service
	pool      *oci.ProxyPool
	masterKey []byte
}

func NewEmailService(store *db.Store, dnsSvc *dns.DnsService, sysConf *sysconf.Service, pool *oci.ProxyPool, masterKey []byte) *EmailService {
	return &EmailService{store: store, dnsSvc: dnsSvc, sysConf: sysConf, pool: pool, masterKey: masterKey}
}

// --- Email Recipient (email_receive) ---

// EmailReceiveResp is the API-facing email recipient.
type EmailReceiveResp struct {
	ID         int64  `json:"id"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	CreateTime string `json:"createTime"`
	UpdateTime string `json:"updateTime"`
}

// ListReceives returns paginated email recipients with optional email/name filters.
func (s *EmailService) ListReceives(ctx context.Context, emailFilter, nameFilter string, page, pageSize int64) ([]EmailReceiveResp, int64, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := page * pageSize
	q := repo.New(s.store.Read)

	total, err := q.CountEmailReceives(ctx, repo.CountEmailReceivesParams{
		NULLIF:   emailFilter,
		NULLIF_2: nameFilter,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count email receives: %w", err)
	}

	rows, err := q.ListEmailReceives(ctx, repo.ListEmailReceivesParams{
		NULLIF:   emailFilter,
		NULLIF_2: nameFilter,
		Limit:    pageSize,
		Offset:   offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list email receives: %w", err)
	}

	out := make([]EmailReceiveResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, EmailReceiveResp{
			ID:         r.ID,
			Email:      r.Email,
			Name:       r.Name,
			CreateTime: ns(r.CreateTime),
			UpdateTime: ns(r.UpdateTime),
		})
	}
	return out, total, nil
}

// GetReceive returns a single email recipient by ID.
func (s *EmailService) GetReceive(ctx context.Context, id int64) (*EmailReceiveResp, error) {
	r, err := repo.New(s.store.Read).FindEmailReceiveById(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find email receive: %w", err)
	}
	return &EmailReceiveResp{
		ID:         r.ID,
		Email:      r.Email,
		Name:       r.Name,
		CreateTime: ns(r.CreateTime),
		UpdateTime: ns(r.UpdateTime),
	}, nil
}

// AddReceive creates a new email recipient.
func (s *EmailService) AddReceive(ctx context.Context, emailAddr, name string) error {
	if emailAddr == "" || name == "" {
		return fmt.Errorf("email and name required")
	}
	now := time.Now().Format(httpTimeFmt)
	return repo.New(s.store.Write).InsertEmailReceive(ctx, repo.InsertEmailReceiveParams{
		Email:      emailAddr,
		Name:       name,
		CreateTime: nullStr(now),
		UpdateTime: nullStr(now),
	})
}

// DeleteReceive removes an email recipient by ID.
func (s *EmailService) DeleteReceive(ctx context.Context, id int64) error {
	return repo.New(s.store.Write).DeleteEmailReceive(ctx, id)
}

// --- Email Body (batch) ---

// EmailBodyResp is the API-facing email body (batch).
type EmailBodyResp struct {
	ID                  int64  `json:"id"`
	EmailBodyID         string `json:"emailBodyId"`
	TenantName          string `json:"tenantName"`
	TenantEmailConfigID int64  `json:"tenantEmailConfigId"`
	SenderEmail         string `json:"senderEmail"`
	Title               string `json:"title"`
	Content             string `json:"content"`
	ReceiveTotal        int64  `json:"receiveTotal"`
	ReceiveSuccessTotal int64  `json:"receiveSuccessTotal"`
	ReceiveFailTotal    int64  `json:"receiveFailTotal"`
	CreateTime          string `json:"createTime"`
}

// ListBodies returns paginated email bodies.
func (s *EmailService) ListBodies(ctx context.Context, emailBodyIDFilter string, page, pageSize int64) ([]EmailBodyResp, int64, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := page * pageSize
	q := repo.New(s.store.Read)

	total, err := q.CountEmailBodies(ctx, emailBodyIDFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("count email bodies: %w", err)
	}

	rows, err := q.ListEmailBodies(ctx, repo.ListEmailBodiesParams{
		NULLIF: emailBodyIDFilter,
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list email bodies: %w", err)
	}

	out := make([]EmailBodyResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, EmailBodyResp{
			ID:                  r.ID,
			EmailBodyID:         r.EmailBodyID,
			TenantName:          ns(r.TenantName),
			TenantEmailConfigID: ni(r.TenantEmailConfigID),
			SenderEmail:         ns(r.SenderEmail),
			Title:               ns(r.Title),
			Content:             ns(r.Content),
			ReceiveTotal:        ni(r.ReceiveTotal),
			ReceiveSuccessTotal: ni(r.ReceiveSuccessTotal),
			ReceiveFailTotal:    ni(r.ReceiveFailTotal),
			CreateTime:          ns(r.CreateTime),
		})
	}
	return out, total, nil
}

// DeleteBody deletes an email body and its associated send records.
func (s *EmailService) DeleteBody(ctx context.Context, emailBodyID string) error {
	return s.store.WithTx(ctx, func(tx *sql.Tx) error {
		q := repo.New(tx)
		if err := q.DeleteEmailSendRecordsByBodyId(ctx, sql.NullString{String: emailBodyID, Valid: true}); err != nil {
			return fmt.Errorf("delete send records: %w", err)
		}
		return q.DeleteEmailBody(ctx, emailBodyID)
	})
}

// BatchDeleteBodies deletes all email bodies and their send records for a config.
func (s *EmailService) BatchDeleteBodies(ctx context.Context, tenantEmailConfigID int64) error {
	return s.store.WithTx(ctx, func(tx *sql.Tx) error {
		q := repo.New(tx)
		configID := sql.NullInt64{Int64: tenantEmailConfigID, Valid: true}
		if err := q.DeleteEmailSendRecordsByConfigId(ctx, configID); err != nil {
			return fmt.Errorf("delete send records: %w", err)
		}
		return q.DeleteEmailBodiesByConfigId(ctx, configID)
	})
}

// --- Email Send Records ---

// EmailSendRecordResp is the API-facing send record.
type EmailSendRecordResp struct {
	ID                  int64  `json:"id"`
	EmailSendRecordID   string `json:"emailSendRecordId"`
	EmailBodyID         string `json:"emailBodyId"`
	EmailSendAddress    string `json:"emailSendAddress"`
	TenantName          string `json:"tenantName"`
	EmailReceiveID      int64  `json:"emailReceiveId"`
	ReceiveEmailAddress string `json:"receiveEmailAddress"`
	SendState           int64  `json:"sendState"`
	CreateTime          string `json:"createTime"`
}

// ListSendRecords returns paginated send records for an email body.
func (s *EmailService) ListSendRecords(ctx context.Context, emailBodyID string, page, pageSize int64) ([]EmailSendRecordResp, int64, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := page * pageSize
	q := repo.New(s.store.Read)
	bodyID := sql.NullString{String: emailBodyID, Valid: emailBodyID != ""}

	total, err := q.CountEmailSendRecords(ctx, bodyID)
	if err != nil {
		return nil, 0, fmt.Errorf("count send records: %w", err)
	}

	rows, err := q.ListEmailSendRecords(ctx, repo.ListEmailSendRecordsParams{
		EmailBodyID: bodyID,
		Limit:       pageSize,
		Offset:      offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list send records: %w", err)
	}

	out := make([]EmailSendRecordResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, EmailSendRecordResp{
			ID:                  r.ID,
			EmailSendRecordID:   ns(r.EmailSendRecordID),
			EmailBodyID:         ns(r.EmailBodyID),
			EmailSendAddress:    ns(r.EmailSendAddress),
			TenantName:          ns(r.TenantName),
			EmailReceiveID:      ni(r.EmailReceiveID),
			ReceiveEmailAddress: ns(r.ReceiveEmailAddress),
			SendState:           ni(r.SendState),
			CreateTime:          ns(r.CreateTime),
		})
	}
	return out, total, nil
}

// --- Tenant Email Config List/Get ---

// TenantEmailConfigListItem is the API-facing tenant email config summary.
type TenantEmailConfigListItem struct {
	ID              int64  `json:"id"`
	TenantID        int64  `json:"tenantId"`
	DomainName      string `json:"domainName"`
	SmtpUsername    string `json:"smtpUsername"`
	SmtpHost        string `json:"smtpHost"`
	SmtpPort        string `json:"smtpPort"`
	SenderEmail     string `json:"senderEmail"`
	Active          bool   `json:"active"`
	CreatedTime     string `json:"createdTime"`
	DailyEmailLimit int64  `json:"dailyEmailLimit"`
	TodaySentCount  int64  `json:"todaySentCount"`
	TenantName      string `json:"tenantName"`
}

// ListTenantConfigs returns paginated active tenant email configs.
func (s *EmailService) ListTenantConfigs(ctx context.Context, page, pageSize int64) ([]TenantEmailConfigListItem, int64, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := page * pageSize
	q := repo.New(s.store.Read)

	total, err := q.CountTenantEmailConfigs(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count tenant email configs: %w", err)
	}

	rows, err := q.ListTenantEmailConfigs(ctx, repo.ListTenantEmailConfigsParams{
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list tenant email configs: %w", err)
	}

	out := make([]TenantEmailConfigListItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, TenantEmailConfigListItem{
			ID:              r.ID,
			TenantID:        ni(r.TenantID),
			DomainName:      ns(r.DomainName),
			SmtpUsername:    ns(r.SmtpUsername),
			SmtpHost:        ns(r.SmtpHost),
			SmtpPort:        ns(r.SmtpPort),
			SenderEmail:     ns(r.SenderEmail),
			Active:          ni(r.Active) != 0,
			CreatedTime:     ns(r.CreatedTime),
			DailyEmailLimit: ni(r.DailyEmailLimit),
			TodaySentCount:  ni(r.TodaySentCount),
			TenantName:      ns(r.TenantName),
		})
	}
	return out, total, nil
}

// GetTenantConfig returns a tenant email config by tenant ID.
func (s *EmailService) GetTenantConfig(ctx context.Context, tenantID int64) (repo.TenantEmailConfig, error) {
	return repo.New(s.store.Read).FindTenantEmailConfigByTenantId(ctx, nullInt64(tenantID))
}

// --- Enable Email Flow ---

// EnableEmailInput carries the enable email request payload.
type EnableEmailInput struct {
	TenantID   int64  `json:"tenantId"`
	DomainName string `json:"domainName"`
}

// EnableEmail provisions OCI email resources and Cloudflare DNS records for a tenant.
func (s *EmailService) EnableEmail(ctx context.Context, in EnableEmailInput) error {
	if in.DomainName == "" {
		return fmt.Errorf("domain name required")
	}

	// 1. Validate Cloudflare config.
	cfToken := s.sysConf.GetString(ctx, "cloudflare_api_token")
	if cfToken == "" {
		return fmt.Errorf("cloudflare API token not configured in system_config")
	}

	// 2. Look up tenant.
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, in.TenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", in.TenantID, err)
	}
	creds := tenantToCredsEmail(t)

	// 3. Build OCI provider and clients.
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return fmt.Errorf("create OCI provider: %w", err)
	}
	clients, err := oci.NewClients(prov)
	if err != nil {
		return fmt.Errorf("create OCI clients: %w", err)
	}

	emailOps := &oci.EmailOps{}
	compartmentID := creds.Tenancy
	userID := creds.UserID

	// 4. Create/find OCI email domain.
	domainID, err := emailOps.CreateEmailDomain(ctx, clients.Email, compartmentID, in.DomainName)
	if err != nil {
		return fmt.Errorf("create email domain: %w", err)
	}

	// 5. Create/find OCI sender.
	senderEmail := "noreply@" + in.DomainName
	senderID, err := emailOps.CreateSender(ctx, clients.Email, compartmentID, senderEmail)
	if err != nil {
		return fmt.Errorf("create sender: %w", err)
	}

	// 6. Create/find OCI DKIM.
	dkimResult, err := emailOps.CreateDkim(ctx, clients.Email, domainID)
	if err != nil {
		return fmt.Errorf("create dkim: %w", err)
	}

	// 7. Generate SMTP credentials.
	smtpCreds, err := emailOps.GenerateSmtpCredentials(ctx, clients.Identity, userID,
		fmt.Sprintf("Email credentials for tenant %d", in.TenantID))
	if err != nil {
		return fmt.Errorf("generate SMTP credentials: %w", err)
	}

	// 8. Construct SMTP host.
	regionCode := region.CodeByName(ns(t.Region))
	smtpHost := fmt.Sprintf("smtp.email.%s.oci.oraclecloud.com", regionCode)
	smtpPort := "587"

	// 9. Create Cloudflare DNS records.
	cfCache := dns.GetOrCreateCache(cfToken)
	dnsRecordIDs, err := s.createCloudflareDNSRecords(ctx, cfCache, in.DomainName, dkimResult.CnameRecordValue)
	if err != nil {
		return fmt.Errorf("create DNS records: %w", err)
	}

	// 10. Save to tenant_email_config.
	now := time.Now().Format(httpTimeFmt)
	err = repo.New(s.store.Write).UpsertTenantEmailConfigFull(ctx, repo.UpsertTenantEmailConfigFullParams{
		TenantID:         nullInt64(in.TenantID),
		DomainID:         nullStr(domainID),
		DomainName:       nullStr(in.DomainName),
		SenderID:         nullStr(senderID),
		CredentialID:     nullStr(smtpCreds.CredentialID),
		SmtpUsername:     nullStr(smtpCreds.Username),
		SmtpPassword:     nullStr(smtpCreds.Password),
		SmtpHost:         nullStr(smtpHost),
		SmtpPort:         nullStr(smtpPort),
		SenderEmail:      nullStr(senderEmail),
		DkimID:           nullStr(dkimResult.DkimID),
		CnameRecordValue: nullStr(dkimResult.CnameRecordValue),
		Active:           nullInt64(1),
		CreatedTime:      nullStr(now),
		DailyEmailLimit:  nullInt64(defaultDailyEmailLimit),
		TodaySentCount:   nullInt64(0),
		LastResetDate:    nullStr(time.Now().Format("2006-01-02")),
		DbsRecordIdsStr:  nullStr(strings.Join(dnsRecordIDs, ",")),
	})
	if err != nil {
		return fmt.Errorf("save email config: %w", err)
	}

	// 11. Set tenant.email_enable = 1.
	return repo.New(s.store.Write).UpdateTenantFields(ctx, repo.UpdateTenantFieldsParams{
		TenancyName:  t.TenancyName,
		TenancyDes:   t.TenancyDes,
		AccountType:  t.AccountType,
		EmailAddress: t.EmailAddress,
		IsActive:     t.IsActive,
		ID:           in.TenantID,
	})
}

// createCloudflareDNSRecords creates SPF TXT and DKIM CNAME records in Cloudflare.
func (s *EmailService) createCloudflareDNSRecords(ctx context.Context, cfCache *dns.CfCache, domainName, cnameRecordValue string) ([]string, error) {
	// Find the zone for this domain.
	zones, err := cfCache.ListZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("list Cloudflare zones: %w", err)
	}

	var zoneID string
	for _, z := range zones {
		if z.Name == domainName || strings.HasSuffix(domainName, "."+z.Name) {
			zoneID = z.ID
			break
		}
	}
	if zoneID == "" {
		return nil, fmt.Errorf("Cloudflare zone not found for domain %s", domainName)
	}

	var recordIDs []string

	// Check if SPF record already exists.
	existingRecords, _ := cfCache.ListRecords(ctx, zoneID, "TXT", domainName)
	spfContent := "v=spf1 include:emaildelivery.oraclecloud.com ~all"
	spfExists := false
	for _, r := range existingRecords {
		if r.Content == spfContent {
			spfExists = true
			recordIDs = append(recordIDs, r.ID)
			break
		}
	}

	// Create SPF TXT record if not exists.
	if !spfExists {
		spfRecord, err := cfCache.CreateRecord(ctx, zoneID, dns.DnsRecord{
			Name:    domainName,
			Type:    "TXT",
			Content: spfContent,
			TTL:     600,
		})
		if err != nil {
			return nil, fmt.Errorf("create SPF record: %w", err)
		}
		recordIDs = append(recordIDs, spfRecord.ID)
	}

	// Parse DKIM CNAME value: "selector._domainkey.domain target"
	if cnameRecordValue != "" {
		parts := strings.SplitN(cnameRecordValue, " ", 2)
		if len(parts) == 2 {
			cnameName := strings.TrimSpace(parts[0])
			cnameTarget := strings.TrimSpace(parts[1])

			// Check if DKIM CNAME already exists.
			dkimRecords, _ := cfCache.ListRecords(ctx, zoneID, "CNAME", cnameName)
			dkimExists := false
			for _, r := range dkimRecords {
				if r.Content == cnameTarget {
					dkimExists = true
					recordIDs = append(recordIDs, r.ID)
					break
				}
			}

			if !dkimExists {
				dkimRecord, err := cfCache.CreateRecord(ctx, zoneID, dns.DnsRecord{
					Name:    cnameName,
					Type:    "CNAME",
					Content: cnameTarget,
					TTL:     600,
				})
				if err != nil {
					return nil, fmt.Errorf("create DKIM CNAME record: %w", err)
				}
				recordIDs = append(recordIDs, dkimRecord.ID)
			}
		}
	}

	return recordIDs, nil
}

// --- Disable Email Flow ---

// DisableEmailInput carries the disable email request payload.
type DisableEmailInput struct {
	TenantEmailConfigID int64 `json:"tenantEmailConfigId"`
}

// DisableEmail tears down OCI email resources and Cloudflare DNS records for a tenant.
func (s *EmailService) DisableEmail(ctx context.Context, in DisableEmailInput) error {
	// 1. Look up tenant_email_config.
	cfg, err := repo.New(s.store.Read).FindTenantEmailConfigById(ctx, in.TenantEmailConfigID)
	if err != nil {
		return fmt.Errorf("find email config %d: %w", in.TenantEmailConfigID, err)
	}

	// 2. Look up tenant.
	tenantID := ni(cfg.TenantID)
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := tenantToCredsEmail(t)

	// 3. Build OCI provider and clients.
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return fmt.Errorf("create OCI provider: %w", err)
	}
	clients, err := oci.NewClients(prov)
	if err != nil {
		return fmt.Errorf("create OCI clients: %w", err)
	}

	emailOps := &oci.EmailOps{}

	// 4. Delete OCI resources (best effort, continue on error).
	if ns(cfg.DkimID) != "" {
		if err := emailOps.DeleteDkim(ctx, clients.Email, ns(cfg.DkimID)); err != nil {
			fmt.Printf("[email] warn: delete dkim %s: %v\n", ns(cfg.DkimID), err)
		}
	}
	if ns(cfg.CredentialID) != "" {
		if err := emailOps.DeleteSmtpCredentials(ctx, clients.Identity, creds.UserID, ns(cfg.CredentialID)); err != nil {
			fmt.Printf("[email] warn: delete smtp creds %s: %v\n", ns(cfg.CredentialID), err)
		}
	}
	if ns(cfg.SenderID) != "" {
		if err := emailOps.DeleteSender(ctx, clients.Email, ns(cfg.SenderID)); err != nil {
			fmt.Printf("[email] warn: delete sender %s: %v\n", ns(cfg.SenderID), err)
		}
	}
	if ns(cfg.DomainID) != "" {
		if err := emailOps.DeleteEmailDomain(ctx, clients.Email, ns(cfg.DomainID)); err != nil {
			fmt.Printf("[email] warn: delete email domain %s: %v\n", ns(cfg.DomainID), err)
		}
	}

	// 5. Delete Cloudflare DNS records.
	cfToken := s.sysConf.GetString(ctx, "cloudflare_api_token")
	if cfToken != "" && ns(cfg.DbsRecordIdsStr) != "" {
		cfCache := dns.GetOrCreateCache(cfToken)
		// We need to find the zone ID for deletion. Try to find from zones.
		zones, _ := cfCache.ListZones(ctx)
		domainName := ns(cfg.DomainName)
		var zoneID string
		for _, z := range zones {
			if z.Name == domainName || strings.HasSuffix(domainName, "."+z.Name) {
				zoneID = z.ID
				break
			}
		}
		if zoneID != "" {
			for _, recordID := range strings.Split(ns(cfg.DbsRecordIdsStr), ",") {
				recordID = strings.TrimSpace(recordID)
				if recordID != "" {
					if err := cfCache.DeleteRecord(ctx, zoneID, recordID); err != nil {
						fmt.Printf("[email] warn: delete DNS record %s: %v\n", recordID, err)
					}
				}
			}
		}
	}

	// 6. Set tenant.email_enable = 0.
	if err := repo.New(s.store.Write).UpdateTenantFields(ctx, repo.UpdateTenantFieldsParams{
		TenancyName:  t.TenancyName,
		TenancyDes:   t.TenancyDes,
		AccountType:  t.AccountType,
		EmailAddress: t.EmailAddress,
		IsActive:     t.IsActive,
		ID:           tenantID,
	}); err != nil {
		return fmt.Errorf("update tenant email_enable: %w", err)
	}

	// 7. Cascade delete email_body + email_send_record.
	configID := sql.NullInt64{Int64: in.TenantEmailConfigID, Valid: true}
	_ = repo.New(s.store.Write).DeleteEmailSendRecordsByConfigId(ctx, configID)
	_ = repo.New(s.store.Write).DeleteEmailBodiesByConfigId(ctx, configID)

	// 8. Delete tenant_email_config row.
	return repo.New(s.store.Write).DeleteTenantEmailConfig(ctx, nullInt64(tenantID))
}

// --- Daily Send Count ---

// CheckAndResetDailyCount checks the daily limit and resets the counter if needed.
// Returns error if the daily limit is reached.
func (s *EmailService) CheckAndResetDailyCount(ctx context.Context, cfg *repo.TenantEmailConfig) error {
	today := time.Now().Format("2006-01-02")
	lastReset := ns(cfg.LastResetDate)

	if lastReset != today {
		// Reset counter for a new day.
		cfg.TodaySentCount = sql.NullInt64{Int64: 0, Valid: true}
		cfg.LastResetDate = sql.NullString{String: today, Valid: true}
		if err := repo.New(s.store.Write).UpdateTenantEmailSentCount(ctx, repo.UpdateTenantEmailSentCountParams{
			TodaySentCount: sql.NullInt64{Int64: 0, Valid: true},
			LastResetDate:  sql.NullString{String: today, Valid: true},
			ID:             cfg.ID,
		}); err != nil {
			return fmt.Errorf("reset daily count: %w", err)
		}
	}

	limit := ni(cfg.DailyEmailLimit)
	if limit <= 0 {
		limit = defaultDailyEmailLimit
	}
	if ni(cfg.TodaySentCount) >= limit {
		return fmt.Errorf("daily email limit reached (%d/%d)", ni(cfg.TodaySentCount), limit)
	}
	return nil
}

// IncrementSentCount increments today_sent_count by n.
func (s *EmailService) IncrementSentCount(ctx context.Context, configID int64, n int64) error {
	cfg, err := repo.New(s.store.Read).FindTenantEmailConfigById(ctx, configID)
	if err != nil {
		return err
	}
	newCount := ni(cfg.TodaySentCount) + n
	return repo.New(s.store.Write).UpdateTenantEmailSentCount(ctx, repo.UpdateTenantEmailSentCountParams{
		TodaySentCount: nullInt64(newCount),
		LastResetDate:  cfg.LastResetDate,
		ID:             configID,
	})
}

// --- Helper ---

// tenantToCredsEmail converts a repo.Tenant to oci.Credentials for email operations.
func tenantToCredsEmail(t repo.Tenant) oci.Credentials {
	return oci.Credentials{
		Tenancy:     ns(t.Tenancy),
		UserID:      ns(t.TenantID),
		Fingerprint: ns(t.Fingerprint),
		Region:      ns(t.Region),
		KeyFileBlob: ns(t.KeyFileBlob),
		KeyFile:     ns(t.KeyFile),
	}
}

// toInt64 converts a string to int64, returning 0 on error.
func toInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
