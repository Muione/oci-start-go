// Package dns — service.go: DNS record management service (SPEC S13).
// Provides CRUD for dns_record table and sync with Cloudflare/EdgeOne.
package dns

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
)

// DnsService manages DNS records locally and syncs with providers.
type DnsService struct {
	store *db.Store
}

func NewDnsService(store *db.Store) *DnsService {
	return &DnsService{store: store}
}

// DnsRecordResp is the API representation of a dns_record row.
type DnsRecordResp struct {
	ID             int64  `json:"id"`
	Provider       string `json:"providerType"`
	Domain         string `json:"domainName"`
	RecordName     string `json:"recordName"`
	RecordType     string `json:"recordType"`
	RecordValue    string `json:"recordValue"`
	TTL            int64  `json:"ttl"`
	Proxied        bool   `json:"proxied"`
	Status         string `json:"status"`
	ZoneID         string `json:"zoneId"`
	CreateTime     string `json:"createTime"`
	UpdateTime     string `json:"updateTime"`
}

// List returns all local DNS records from the dns_record table.
func (s *DnsService) List(ctx context.Context) ([]DnsRecordResp, error) {
	rows, err := s.store.Read.QueryContext(ctx,
		`SELECT id, provider_type, domain_name, record_name, record_type,
		        record_value, ttl, proxied, status, zone_id, create_time, update_time
		 FROM dns_record ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query dns_record: %w", err)
	}
	defer rows.Close()

	var result []DnsRecordResp
	for rows.Next() {
		var r DnsRecordResp
		var ttl, proxied sql.NullInt64
		var status, zoneID sql.NullString
		if err := rows.Scan(&r.ID, &r.Provider, &r.Domain, &r.RecordName, &r.RecordType,
			&r.RecordValue, &ttl, &proxied, &status, &zoneID, &r.CreateTime, &r.UpdateTime); err != nil {
			return nil, fmt.Errorf("scan dns_record: %w", err)
		}
		r.TTL = ttl.Int64
		r.Proxied = proxied.Int64 == 1
		r.Status = status.String
		r.ZoneID = zoneID.String
		result = append(result, r)
	}
	if result == nil {
		result = []DnsRecordResp{}
	}
	return result, rows.Err()
}

// Save creates or updates a DNS record locally.
func (s *DnsService) Save(ctx context.Context, in DnsRecordResp) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	var proxied int64
	if in.Proxied {
		proxied = 1
	}

	if in.ID > 0 {
		_, err := s.store.Write.ExecContext(ctx,
			`UPDATE dns_record SET record_value=?, ttl=?, proxied=?, status=?, update_time=?
			 WHERE id=?`,
			in.RecordValue, in.TTL, proxied, in.Status, now, in.ID)
		return err
	}

	_, err := s.store.Write.ExecContext(ctx,
		`INSERT INTO dns_record (provider_type, domain_name, record_name, record_type,
		 record_value, ttl, proxied, status, zone_id, create_time, update_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Provider, in.Domain, in.RecordName, in.RecordType,
		in.RecordValue, in.TTL, proxied, in.Status, in.ZoneID, now, now)
	return err
}

// Delete removes a DNS record locally.
func (s *DnsService) Delete(ctx context.Context, id int64) error {
	_, err := s.store.Write.ExecContext(ctx, `DELETE FROM dns_record WHERE id=?`, id)
	return err
}

// SyncFromCloudflare fetches records from Cloudflare and upserts them locally.
func (s *DnsService) SyncFromCloudflare(ctx context.Context, client *CfClient, zoneID string) (int, error) {
	records, err := client.ListDnsRecords(ctx, zoneID, "", "")
	if err != nil {
		return 0, fmt.Errorf("cf list records: %w", err)
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	count := 0
	for _, r := range records {
		var proxiedInt int64
		if r.Proxied {
			proxiedInt = 1
		}
		// Upsert by provider_record_id
		_, err := s.store.Write.ExecContext(ctx,
			`INSERT INTO dns_record (provider_type, domain_name, record_name, record_type,
			 record_value, ttl, proxied, status, zone_id, provider_record_id,
			 create_time, update_time, last_sync_time)
			 VALUES ('cloudflare', ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?)
			 ON CONFLICT DO NOTHING`,
			r.ZoneID, r.Name, r.Type, r.Content, r.TTL, proxiedInt, zoneID, r.ID,
			now, now, now)
		if err != nil {
			continue
		}
		count++
	}
	return count, nil
}
