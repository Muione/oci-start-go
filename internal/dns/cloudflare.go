// Package dns — cloudflare.go: Cloudflare DNS REST client (SPEC S13.1).
// Port of CloudflareService.java core. Uses Cloudflare API v4 with
// X-Auth-Email + X-Auth-Key authentication.
package dns

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CfClient wraps the Cloudflare API v4.
type CfClient struct {
	APIKey    string
	Email     string
	BaseURL   string
	Client    *http.Client
}

// NewCfClient creates a Cloudflare API client.
func NewCfClient(email, apiKey string) *CfClient {
	return &CfClient{
		APIKey:  apiKey,
		Email:   email,
		BaseURL: "https://api.cloudflare.com/client/v4",
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// CfResponse is the standard Cloudflare API response envelope.
type CfResponse struct {
	Success  bool             `json:"success"`
	Errors   []CfError        `json:"errors"`
	Messages []string         `json:"messages"`
	Result   json.RawMessage  `json:"result"`
}

// CfError represents a Cloudflare API error.
type CfError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Zone represents a Cloudflare zone (domain).
type Zone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// DnsRecord represents a Cloudflare DNS record.
type DnsRecord struct {
	ID       string `json:"id"`
	ZoneID   string `json:"zone_id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Proxied  bool   `json:"proxied"`
	Priority int    `json:"priority"`
}

// CfListRecordsResponse is the Cloudflare API response for listing DNS records
// with pagination info.
type CfListRecordsResponse struct {
	Success  bool        `json:"success"`
	Errors   []CfError   `json:"errors"`
	Messages []string    `json:"messages"`
	Result   []DnsRecord `json:"result"`
	ResultInfo struct {
		Page       int `json:"page"`
		PerPage    int `json:"per_page"`
		TotalPages int `json:"total_pages"`
		Count      int `json:"count"`
		TotalCount int `json:"total_count"`
	} `json:"result_info"`
}

func (c *CfClient) do(ctx context.Context, method, path string, body any) (*CfResponse, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Email", c.Email)
	req.Header.Set("X-Auth-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cf request: %w", err)
	}
	defer resp.Body.Close()

	var cfResp CfResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return nil, fmt.Errorf("cf decode: %w", err)
	}
	if !cfResp.Success {
		errMsgs := ""
		for _, e := range cfResp.Errors {
			errMsgs += fmt.Sprintf("[%d] %s; ", e.Code, e.Message)
		}
		return &cfResp, fmt.Errorf("cf error: %s", errMsgs)
	}
	return &cfResp, nil
}

// ListZones returns all zones accessible with the credentials.
func (c *CfClient) ListZones(ctx context.Context) ([]Zone, error) {
	resp, err := c.do(ctx, http.MethodGet, "/zones", nil)
	if err != nil {
		return nil, err
	}
	var zones []Zone
	if err := json.Unmarshal(resp.Result, &zones); err != nil {
		return nil, fmt.Errorf("unmarshal zones: %w", err)
	}
	return zones, nil
}

// ListDnsRecords returns DNS records for a zone.
func (c *CfClient) ListDnsRecords(ctx context.Context, zoneID, recordType, name string) ([]DnsRecord, error) {
	path := fmt.Sprintf("/zones/%s/dns_records?per_page=100", zoneID)
	if recordType != "" {
		path += "&type=" + recordType
	}
	if name != "" {
		path += "&name=" + name
	}
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var records []DnsRecord
	if err := json.Unmarshal(resp.Result, &records); err != nil {
		return nil, fmt.Errorf("unmarshal records: %w", err)
	}
	return records, nil
}

// ListDnsRecordsPage returns paginated DNS records for a zone.
func (c *CfClient) ListDnsRecordsPage(ctx context.Context, zoneID string, page, perPage int, recordType, name, content string) ([]DnsRecord, *CfListRecordsResponse, error) {
	path := fmt.Sprintf("/zones/%s/dns_records?page=%d&per_page=%d", zoneID, page, perPage)
	if recordType != "" {
		path += "&type=" + recordType
	}
	if name != "" {
		path += "&name=" + name
	}
	if content != "" {
		path += "&content=" + content
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("X-Auth-Email", c.Email)
	req.Header.Set("X-Auth-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("cf request: %w", err)
	}
	defer resp.Body.Close()

	var cfResp CfListRecordsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return nil, nil, fmt.Errorf("cf decode: %w", err)
	}
	if !cfResp.Success {
		errMsgs := ""
		for _, e := range cfResp.Errors {
			errMsgs += fmt.Sprintf("[%d] %s; ", e.Code, e.Message)
		}
		return nil, &cfResp, fmt.Errorf("cf error: %s", errMsgs)
	}
	return cfResp.Result, &cfResp, nil
}

// CreateDnsRecord creates a new DNS record.
func (c *CfClient) CreateDnsRecord(ctx context.Context, zoneID string, record DnsRecord) (*DnsRecord, error) {
	resp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/zones/%s/dns_records", zoneID), record)
	if err != nil {
		return nil, err
	}
	var created DnsRecord
	if err := json.Unmarshal(resp.Result, &created); err != nil {
		return nil, fmt.Errorf("unmarshal created record: %w", err)
	}
	return &created, nil
}

// UpdateDnsRecord updates an existing DNS record.
func (c *CfClient) UpdateDnsRecord(ctx context.Context, zoneID, recordID string, record DnsRecord) (*DnsRecord, error) {
	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID), record)
	if err != nil {
		return nil, err
	}
	var updated DnsRecord
	if err := json.Unmarshal(resp.Result, &updated); err != nil {
		return nil, fmt.Errorf("unmarshal updated record: %w", err)
	}
	return &updated, nil
}

// DeleteDnsRecord deletes a DNS record.
func (c *CfClient) DeleteDnsRecord(ctx context.Context, zoneID, recordID string) error {
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID), nil)
	return err
}
