// Package service holds application services above the repo/db layer. Phase 3
// adds TenantService: tenant CRUD + OCI instance sync. Private keys are
// encrypted at rest (plan D1); Save encrypts the uploaded PEM with the master
// key into tenant.key_file_blob, and SyncOci decrypts it via oci.NewProvider.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/oci/region"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/util/crypto"
)

const httpTimeFmt = "2006-01-02 15:04:05"

type TenantService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool // reserved for Phase 4 (launch/audit); Phase 3 sync is direct
}

func NewTenantService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *TenantService {
	return &TenantService{store: store, masterKey: masterKey, pool: pool}
}

// TenantResp is the masked tenant view returned by List (never the key blob).
type TenantResp struct {
	ID           int64  `json:"id"`
	TenantID     string `json:"tenantId"`
	UserName     string `json:"userName"`
	Fingerprint  string `json:"fingerprint"`
	Tenancy      string `json:"tenancy"`
	Region       string `json:"region"`
	RegionName   string `json:"regionName"`
	CreatedAt    string `json:"createdAt"`
	ApiSynced    bool   `json:"apiSynced"`
	CloudType    int64  `json:"cloudType"`
	IsActive     bool   `json:"isActive"`
	IsHomeRegion bool   `json:"isHomeRegion"`
	AccountType  string `json:"accountType"`
	TenancyName  string `json:"tenancyName"`
	ActiveDays   string `json:"activeDays"`
	HasBootTask   bool   `json:"hasBootTask"`
	HasChildren   bool   `json:"hasChildren"`
	InstanceCount int64  `json:"instanceCount"`
	PlanType      string `json:"planType"`
}

func (s *TenantService) List(ctx context.Context) ([]TenantResp, error) {
	// Single round-trip: ListTenantsWithCounts joins register_time, active boot
	// count and child count as correlated subqueries, replacing the per-tenant
	// fan-out (FindRegisterDetailByTenantId + CountBootInstancesByTenantId +
	// CountTenantChildren) that scaled 1 + 3*N queries with N tenants.
	rows, err := repo.New(s.store.Read).ListTenantsWithCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	out := make([]TenantResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, toTenantRespFromCounts(r))
	}
	return out, nil
}

// SaveInput carries the multipart tenant save payload. KeyPEM is the uploaded
// private key file bytes (plaintext PEM), encrypted here before storage.
type SaveInput struct {
	Tenancy      string
	TenantID     string // user OCID
	UserName     string // optional; generated from region code if empty
	Fingerprint  string
	Region       string
	CloudType    int64
	IsHomeRegion bool
	AccountType  string
	KeyPEM       []byte
}

func (s *TenantService) Save(ctx context.Context, in SaveInput) error {
	if in.Tenancy == "" || in.TenantID == "" || in.Fingerprint == "" || in.Region == "" {
		return errors.New("tenancy/tenantId/fingerprint/region required")
	}
	if len(in.KeyPEM) == 0 {
		return errors.New("key file required")
	}
	blob, err := crypto.EncryptString(string(in.KeyPEM), s.masterKey)
	if err != nil {
		return fmt.Errorf("encrypt private key: %w", err)
	}
	// userName parity: regionCode_random6 (Java saveTenant used RegionEnum code + RandomUtil).
	userName := in.UserName
	if userName == "" {
		userName = region.CodeByName(in.Region) + "_" + randomString(6)
	}
	cloudType := in.CloudType
	if cloudType == 0 {
		cloudType = 1 // ORACLE_CLOUD default
	}
	params := repo.InsertTenantParams{
		TenantID:     nullStr(in.TenantID),
		UserName:     nullStr(userName),
		Fingerprint:  nullStr(in.Fingerprint),
		Tenancy:      nullStr(in.Tenancy),
		Region:       nullStr(in.Region),
		KeyFileBlob:  nullStr(blob),
		CreatedAt:    nullStr(time.Now().Format(httpTimeFmt)),
		CloudType:    nullInt64(cloudType),
		IsHomeRegion: nullInt64(boolToInt(in.IsHomeRegion)),
		AccountType:  nullStr(in.AccountType),
		TenancyName:  nullStr(userName),
		IsActive:     nullInt64(1), // new tenants are active by default
	}
	now := time.Now().Format(httpTimeFmt)
	if err := repo.New(s.store.Write).InsertTenant(ctx, params); err != nil {
		return err
	}
	// Insert register_detail WITHOUT register_time — it's only set on OCI sync
	// (UpdateAccountDetail) from the real subscription timeStart. Leaving it null
	// makes active-days fall back to created_at, and GetSubscriptionDetail fall
	// back to live OCI (correct subscription start) until the first sync.
	_ = repo.New(s.store.Write).UpsertRegisterDetail(ctx, repo.UpsertRegisterDetailParams{
		TenantID:     in.TenantID,
		CloudType:    nullInt64(cloudType),
		CreatedTime:  nullStr(now),
		UpdatedTime:  nullStr(now),
	})
	return nil
}

func (s *TenantService) Delete(ctx context.Context, id int64) error {
	return s.store.WithTx(ctx, func(tx *sql.Tx) error {
		q := repo.New(tx)
		if err := q.DeleteInstanceDetailsByTenantId(ctx, sql.NullInt64{Int64: id, Valid: true}); err != nil {
			return fmt.Errorf("delete instance details: %w", err)
		}
		if err := q.DeleteTenant(ctx, id); err != nil {
			return fmt.Errorf("delete tenant: %w", err)
		}
		return nil
	})
}

// SyncOci re-enumerates the tenant's instances from OCI and replaces the local
// instance_detail rows (delete-by-tenant + insert-all), then marks the tenant
// api-synced. Parity with TenantServiceImpl.syncOci. Direct (non-proxy).
func (s *TenantService) SyncOci(ctx context.Context, id int64) error {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", id, err)
	}
	creds := tenantToCreds(t)
	rows, err := oci.ListInstancesByTenant(ctx, id, creds, s.masterKey)
	if err != nil {
		return fmt.Errorf("list instances: %w", err)
	}
	return s.store.WithTx(ctx, func(tx *sql.Tx) error {
		q := repo.New(tx)
		if err := q.DeleteInstanceDetailsByTenantId(ctx, sql.NullInt64{Int64: id, Valid: true}); err != nil {
			return fmt.Errorf("delete instance details: %w", err)
		}
		for _, r := range rows {
			if err := q.InsertInstanceDetail(ctx, r); err != nil {
				return fmt.Errorf("insert instance detail: %w", err)
			}
		}
		return q.SetTenantApiSynced(ctx, repo.SetTenantApiSyncedParams{
			ApiSynced: sql.NullInt64{Int64: 1, Valid: true},
			ID:        id,
		})
	})
}

// ListInstances returns the locally cached instance_detail rows for a tenant,
// converted to plain types so the JSON serialization produces strings/numbers
// rather than sql.NullString objects.
func (s *TenantService) ListInstances(ctx context.Context, id int64) ([]InstanceDetailResp, error) {
	rows, err := repo.New(s.store.Read).ListInstanceDetailsByTenantId(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		return nil, err
	}
	// Resolve tenant name for the given tenant id
	tenantName := ""
	if t, err := repo.New(s.store.Read).FindTenantByID(ctx, id); err == nil {
		tenantName = ns(t.UserName)
	}
	out := make([]InstanceDetailResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, InstanceDetailResp{
			ID:                  r.ID,
			TenantID:            ni(r.TenantID),
			TenantName:          tenantName,
			InstanceID:          ns(r.InstanceID),
			DisplayName:         ns(r.DisplayName),
			Shape:               ns(r.Shape),
			State:               ns(r.State),
			Ocpus:               ni(r.Ocpus),
			MemoryInGbs:         ni(r.MemoryInGbs),
			BootVolumeSizeInGbs: ni(r.BootVolumeSizeInGbs),
			PublicIps:           ns(r.PublicIps),
			PrivateIps:          ns(r.PrivateIps),
			AvailabilityDomain:  ns(r.AvailabilityDomain),
			CompartmentID:       ns(r.CompartmentID),
			BootVolumeID:        ns(r.BootVolumeID),
			BootVolumeName:      ns(r.BootVolumeName),
			VpusPerGb:           ns(r.VpusPerGb),
			Ipv6Addresses:       ns(r.Ipv6Addresses),
			VnicIds:             ns(r.VnicIds),
			Architecture:        ns(r.Architecture),
			ConnTime:            r.ConnTime,
			EnablePing:          r.EnablePing,
			OnLineEnable:        r.OnLineEnable,
			LastHeartbeat:       ns(r.LastHeartbeat),
			CreateTime:          ns(r.CreateTime),
		})
	}
	return out, nil
}

// TenantFullResp is the detailed tenant view for the detail dialog.
type TenantFullResp struct {
	ID                int64  `json:"id"`
	TenantID          string `json:"tenantId"`
	UserName          string `json:"userName"`
	Fingerprint       string `json:"fingerprint"`
	Tenancy           string `json:"tenancy"`
	Region            string `json:"region"`
	RegionName        string `json:"regionName"`
	CreatedAt         string `json:"createdAt"`
	ApiSynced         bool   `json:"apiSynced"`
	EnableIcmp        bool   `json:"enableIcmp"`
	EnableAllProtocol bool   `json:"enableAllProtocol"`
	IsHomeRegion      bool   `json:"isHomeRegion"`
	ParenID           int64  `json:"parenId"`
	TenancyName       string `json:"tenancyName"`
	TenancyDes        string `json:"tenancyDes"`
	AccountType       string `json:"accountType"`
	CloudType         int64  `json:"cloudType"`
	RegionEn          string `json:"regionEn"`
	IDStr             string `json:"idStr"`
	EmailAddress      string `json:"emailAddress"`
	EmailEnable       bool   `json:"emailEnable"`
	TransferStatus    int64  `json:"transferStatus"`
	TransferAmount    string `json:"transferAmount"`
	IsActive          bool   `json:"isActive"`
	ActiveDays        string `json:"activeDays"`
}

// UpdateInput carries the tenant update payload.
type UpdateInput struct {
	ID           int64  `json:"id"`
	TenancyName  string `json:"tenancyName"`
	TenancyDes   string `json:"tenancyDes"`
	AccountType  string `json:"accountType"`
	EmailAddress string `json:"emailAddress"`
	IsActive     bool   `json:"isActive"`
}

// CheckResult holds the connectivity check result for a tenant.
type CheckResult struct {
	TenantID int64  `json:"tenantId"`
	UserName string `json:"userName"`
	Alive    bool   `json:"alive"`
	Error    string `json:"error,omitempty"`
}

// GetFull returns the full tenant detail (including key_file_blob).
func (s *TenantService) GetFull(ctx context.Context, id int64) (TenantFullResp, error) {
	t, err := repo.New(s.store.Read).FindTenantFullByID(ctx, id)
	if err != nil {
		return TenantFullResp{}, fmt.Errorf("find tenant full: %w", err)
	}
	return TenantFullResp{
		ID:                t.ID,
		TenantID:          ns(t.TenantID),
		UserName:          ns(t.UserName),
		Fingerprint:       ns(t.Fingerprint),
		Tenancy:           ns(t.Tenancy),
		Region:            ns(t.Region),
		RegionName:        region.NameByCode(region.CodeByName(ns(t.Region))),
		CreatedAt:         ns(t.CreatedAt),
		ApiSynced:         ni(t.ApiSynced) == 1,
		EnableIcmp:        ni(t.EnableIcmp) != 0,
		EnableAllProtocol: ni(t.EnableAllProtocol) != 0,
		IsHomeRegion:      ni(t.IsHomeRegion) != 0,
		ParenID:           ni(t.ParenID),
		TenancyName:       ns(t.TenancyName),
		TenancyDes:        ns(t.TenancyDes),
		AccountType:       ns(t.AccountType),
		CloudType:         ni(t.CloudType),
		RegionEn:          ns(t.RegionEn),
		IDStr:             ns(t.IDStr),
		EmailAddress:      ns(t.EmailAddress),
		EmailEnable:       ni(t.EmailEnable) != 0,
		TransferStatus:    ni(t.TransferStatus),
		TransferAmount:    ns(t.TransferAmount),
		IsActive:          isActive(t.IsActive),
		ActiveDays:        s.calculateTenantActiveDays(ctx, t),
	}, nil
}

// calculateTenantActiveDays computes subscription days from
// register_detail.register_time (the OCI subscription timeStart). Returns "—"
// when not synced — no fallback to created_at (avoids showing days since the
// API key was added instead of since the subscription started).
func (s *TenantService) calculateTenantActiveDays(ctx context.Context, t repo.FindTenantFullByIDRow) string {
	tenantID := ns(t.TenantID)
	if tenantID != "" {
		if rd, err := repo.New(s.store.Read).FindRegisterDetailByTenantId(ctx, tenantID); err == nil {
			return calculateActiveDays(ns(rd.RegisterTime))
		}
	}
	return calculateActiveDays("")
}

// Update modifies tenant fields (custom name, account type, email, active).
func (s *TenantService) Update(ctx context.Context, in UpdateInput) error {
	return repo.New(s.store.Write).UpdateTenantFields(ctx, repo.UpdateTenantFieldsParams{
		TenancyName:  nullStr(in.TenancyName),
		TenancyDes:   nullStr(in.TenancyDes),
		AccountType:  nullStr(in.AccountType),
		EmailAddress: nullStr(in.EmailAddress),
		IsActive:     nullInt64(boolToInt(in.IsActive)),
		ID:           in.ID,
	})
}

// UpdateCost updates the account cost for a tenancy via cloud_tenancy table.
func (s *TenantService) UpdateCost(ctx context.Context, tenancyName, accountCost string) error {
	now := time.Now().Format(httpTimeFmt)
	return repo.New(s.store.Write).UpdateCloudTenancyCost(ctx, repo.UpdateCloudTenancyCostParams{
		TenancyName: tenancyName,
		AccountCost: nullStr(accountCost),
		UpdateTime:  nullStr(now),
	})
}

// UpdateCostByID updates the account cost for a tenant by its ID.
// Looks up the tenant's tenancy name, then updates cloud_tenancy.
func (s *TenantService) UpdateCostByID(ctx context.Context, id int64, cost string) error {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", id, err)
	}
	tenancyName := ns(t.Tenancy)
	if tenancyName == "" {
		return fmt.Errorf("tenant %d has no tenancy name", id)
	}
	return s.UpdateCost(ctx, tenancyName, cost)
}

// Check tests whether a tenant's credentials are still valid by attempting
// to list compartments via the OCI Identity API.
func (s *TenantService) Check(ctx context.Context, id int64) CheckResult {
	result := CheckResult{TenantID: id}
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, id)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.UserName = ns(t.UserName)
	creds := tenantToCreds(t)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		result.Error = "无法创建认证提供者: " + err.Error()
		return result
	}
	if err := oci.PingIdentity(ctx, prov, creds.Tenancy); err != nil {
		result.Error = "OCI 认证失败: " + err.Error()
		return result
	}
	result.Alive = true
	return result
}

// Export returns the full tenant data (including instances) as a JSON-serializable map.
func (s *TenantService) Export(ctx context.Context, id int64) (map[string]any, error) {
	full, err := s.GetFull(ctx, id)
	if err != nil {
		return nil, err
	}
	instances, err := s.ListInstances(ctx, id)
	if err != nil {
		instances = []InstanceDetailResp{}
	}
	return map[string]any{
		"tenant":    full,
		"instances": instances,
	}, nil
}

// --- helpers ---

func tenantToCreds(t repo.Tenant) oci.Credentials {
	return oci.Credentials{
		Tenancy:     ns(t.Tenancy),
		UserID:      ns(t.TenantID),
		Fingerprint: ns(t.Fingerprint),
		Region:      ns(t.Region),
		KeyFileBlob: ns(t.KeyFileBlob),
		KeyFile:     ns(t.KeyFile),
	}
}

// toTenantRespFromCounts maps the single-round-trip aggregate row to the API
// response, deriving HasBootTask/HasChildren from BootCount/ChildCount.
func toTenantRespFromCounts(r repo.ListTenantsWithCountsRow) TenantResp {
	createdAt := ns(r.CreatedAt)
	// Subscription days from register_detail.register_time (OCI subscription
	// timeStart). No created_at fallback — "—" when not synced.
	activeDaysInput := ns(r.RegisterTime)
	accountType := ns(r.AccountType)
	return TenantResp{
		ID:            r.ID,
		TenantID:      ns(r.TenantID),
		UserName:      ns(r.UserName),
		Fingerprint:   ns(r.Fingerprint),
		Tenancy:       ns(r.Tenancy),
		Region:        ns(r.Region),
		RegionName:    region.NameByCode(region.CodeByName(ns(r.Region))),
		CreatedAt:     createdAt,
		ApiSynced:     ni(r.ApiSynced) == 1,
		CloudType:     ni(r.CloudType),
		IsActive:      isActive(r.IsActive),
		IsHomeRegion:  ni(r.IsHomeRegion) != 0,
		AccountType:   accountType,
		TenancyName:   ns(r.TenancyName),
		ActiveDays:    calculateActiveDays(activeDaysInput),
		HasBootTask:   r.BootCount > 0,
		HasChildren:   r.ChildCount > 0,
		InstanceCount: r.InstanceCount,
		PlanType:      accountType,
	}
}

func toTenantResp(r repo.ListTenantsRow, registerTime string) TenantResp {
	createdAt := ns(r.CreatedAt)
	// Subscription days from register_detail.register_time. No created_at fallback.
	activeDaysInput := registerTime
	return TenantResp{
		ID:           r.ID,
		TenantID:     ns(r.TenantID),
		UserName:     ns(r.UserName),
		Fingerprint:  ns(r.Fingerprint),
		Tenancy:      ns(r.Tenancy),
		Region:       ns(r.Region),
		RegionName:   region.NameByCode(region.CodeByName(ns(r.Region))),
		CreatedAt:    createdAt,
		ApiSynced:    ni(r.ApiSynced) == 1,
		CloudType:    ni(r.CloudType),
		IsActive:     isActive(r.IsActive),
		IsHomeRegion: ni(r.IsHomeRegion) != 0,
		AccountType:  ns(r.AccountType),
		TenancyName:  ns(r.TenancyName),
		ActiveDays:   calculateActiveDays(activeDaysInput),
	}
}

// calculateActiveDays returns the number of days from a timestamp string to now.
// Returns "—" when there is no timestamp (not synced) — no fallback to created_at.
func calculateActiveDays(timestamp string) string {
	if timestamp == "" {
		return "—"
	}
	// Try local-time parse first (format has no timezone indicator).
	t, err := time.ParseInLocation("2006-01-02 15:04:05", timestamp, time.Local)
	if err != nil {
		// Try alternate format (RFC 3339 / ISO 8601 — has explicit timezone).
		t2, err2 := time.Parse(time.RFC3339, timestamp)
		if err2 != nil {
			return "0"
		}
		t = t2
	}
	d := time.Since(t)
	days := int64(d.Hours() / 24)
	if days < 0 {
		days = 0
	}
	if d.Nanoseconds()%int64(24*time.Hour) > 0 {
		days++ // ceiling: partial day counts as a full day
	}
	return fmt.Sprintf("%d", days)
}

func ns(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func ni(v sql.NullInt64) int64 {
	if v.Valid {
		return v.Int64
	}
	return 0
}

// isActive returns true if the value is non-zero OR NULL (default active per schema).
func isActive(v sql.NullInt64) bool {
	if !v.Valid {
		return true // DEFAULT 1 in schema
	}
	return v.Int64 != 0
}

func nullStr(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }
func nullInt64(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }
func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

const randChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = randChars[rand.Intn(len(randChars))]
	}
	return string(b)
}
