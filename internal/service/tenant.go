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
}

func (s *TenantService) List(ctx context.Context) ([]TenantResp, error) {
	rows, err := repo.New(s.store.Read).ListTenants(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	out := make([]TenantResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, toTenantResp(r))
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
	}
	return repo.New(s.store.Write).InsertTenant(ctx, params)
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

func toTenantResp(r repo.ListTenantsRow) TenantResp {
	return TenantResp{
		ID:           r.ID,
		TenantID:     ns(r.TenantID),
		UserName:     ns(r.UserName),
		Fingerprint:  ns(r.Fingerprint),
		Tenancy:      ns(r.Tenancy),
		Region:       ns(r.Region),
		RegionName:   region.NameByCode(region.CodeByName(ns(r.Region))),
		CreatedAt:    ns(r.CreatedAt),
		ApiSynced:    ni(r.ApiSynced) == 1,
		CloudType:    ni(r.CloudType),
		IsActive:     ni(r.IsActive) != 0,
		IsHomeRegion: ni(r.IsHomeRegion) != 0,
		AccountType:  ns(r.AccountType),
		TenancyName:  ns(r.TenancyName),
	}
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

func nullStr(s string) sql.NullString  { return sql.NullString{String: s, Valid: s != ""} }
func nullInt64(v int64) sql.NullInt64  { return sql.NullInt64{Int64: v, Valid: true} }
func boolToInt(b bool) int64           { if b { return 1 }; return 0 }

const randChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = randChars[rand.Intn(len(randChars))]
	}
	return string(b)
}
