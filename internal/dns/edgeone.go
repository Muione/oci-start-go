// Package dns — edgeone.go: Tencent Cloud EdgeOne DNS provider (Phase 8).
// Implements the DNS provider interface for EdgeOne, allowing DNS-01 ACME
// challenges and zone/record management alongside Cloudflare.
package dns

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// EdgeOneClient manages DNS records via Tencent Cloud EdgeOne API.
// API docs: https://cloud.tencent.com/document/product/1552
type EdgeOneClient struct {
	SecretID  string
	SecretKey string
	ZoneID    string
	Client    *http.Client
	Logger    zerolog.Logger
}

// NewEdgeOneClient creates an EdgeOne DNS client.
func NewEdgeOneClient(secretID, secretKey, zoneID string, logger zerolog.Logger) *EdgeOneClient {
	return &EdgeOneClient{
		SecretID:  secretID,
		SecretKey: secretKey,
		ZoneID:    zoneID,
		Client:    &http.Client{Timeout: 15 * time.Second},
		Logger:    logger,
	}
}

// Enabled returns true if credentials are configured.
func (eo *EdgeOneClient) Enabled() bool {
	return eo.SecretID != "" && eo.SecretKey != "" && eo.ZoneID != ""
}

// EdgeOneRecord represents a DNS record in EdgeOne.
type EdgeOneRecord struct {
	RecordID   string `json:"RecordId"`
	Name       string `json:"Name"`
	Type       string `json:"Type"`
	Content    string `json:"Content"`
	TTL        int    `json:"TTL"`
	Priority   int    `json:"Priority"`
	Enable     bool   `json:"Enable"`
	Remark     string `json:"Remark"`
	CreateTime string `json:"CreatedOn"`
	UpdateTime string `json:"ModifiedOn"`
}

// ListRecords returns all DNS records for the configured zone.
func (eo *EdgeOneClient) ListRecords() ([]EdgeOneRecord, error) {
	if !eo.Enabled() {
		return nil, fmt.Errorf("edgeone: not configured")
	}

	params := map[string]interface{}{
		"ZoneId": eo.ZoneID,
		"Limit":  3000,
	}

	resp, err := eo.callAPI("DescribeDnsRecords", params)
	if err != nil {
		return nil, fmt.Errorf("edgeone list records: %w", err)
	}

	var result struct {
		Response struct {
			Records    []EdgeOneRecord `json:"RecordList"`
			TotalCount int             `json:"TotalCount"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("edgeone parse records: %w", err)
	}

	return result.Response.Records, nil
}

// CreateRecord creates a DNS record in EdgeOne.
func (eo *EdgeOneClient) CreateRecord(name, recordType, content string, ttl int, priority int) (*EdgeOneRecord, error) {
	if !eo.Enabled() {
		return nil, fmt.Errorf("edgeone: not configured")
	}

	params := map[string]interface{}{
		"ZoneId":   eo.ZoneID,
		"Type":     recordType,
		"Name":     name,
		"Content":  content,
		"TTL":      ttl,
		"Mode":     "proxied", // proxied or dns_only
	}

	if recordType == "MX" || recordType == "SRV" {
		params["Priority"] = priority
	}

	resp, err := eo.callAPI("CreateDnsRecord", params)
	if err != nil {
		return nil, fmt.Errorf("edgeone create record: %w", err)
	}

	var result struct {
		Response struct {
			RecordID string `json:"RecordId"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("edgeone parse create: %w", err)
	}

	return &EdgeOneRecord{
		RecordID: result.Response.RecordID,
		Name:     name,
		Type:     recordType,
		Content:  content,
		TTL:      ttl,
	}, nil
}

// UpdateRecord updates an existing DNS record.
func (eo *EdgeOneClient) UpdateRecord(recordID, name, recordType, content string, ttl int, priority int) error {
	if !eo.Enabled() {
		return fmt.Errorf("edgeone: not configured")
	}

	params := map[string]interface{}{
		"ZoneId":   eo.ZoneID,
		"RecordId": recordID,
		"Type":     recordType,
		"Name":     name,
		"Content":  content,
		"TTL":      ttl,
	}

	if recordType == "MX" || recordType == "SRV" {
		params["Priority"] = priority
	}

	_, err := eo.callAPI("ModifyDnsRecord", params)
	if err != nil {
		return fmt.Errorf("edgeone update record: %w", err)
	}
	return nil
}

// DeleteRecord deletes a DNS record.
func (eo *EdgeOneClient) DeleteRecord(recordID string) error {
	if !eo.Enabled() {
		return fmt.Errorf("edgeone: not configured")
	}

	params := map[string]interface{}{
		"ZoneId":   eo.ZoneID,
		"RecordId": recordID,
	}

	_, err := eo.callAPI("DeleteDnsRecords", params)
	if err != nil {
		return fmt.Errorf("edgeone delete record: %w", err)
	}
	return nil
}

// callAPI makes a signed request to the Tencent Cloud API.
func (eo *EdgeOneClient) callAPI(action string, params map[string]interface{}) ([]byte, error) {
	body := map[string]interface{}{
		"Action":    action,
		"Version":   "2022-09-01",
		"Timestamp": time.Now().Unix(),
		"Nonce":     time.Now().UnixNano() % 100000,
		"SecretId":  eo.SecretID,
	}
	for k, v := range params {
		body[k] = v
	}

	// Flatten and sign
	sortedKeys := make([]string, 0, len(body))
	for k := range body {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var signParts []string
	var queryParts []string
	for _, k := range sortedKeys {
		val := fmt.Sprint(body[k])
		signParts = append(signParts, k+"="+val)
		queryParts = append(queryParts, k+"="+val)
	}

	signStr := "POSTteo.tencentcloudapi.com/?" + strings.Join(signParts, "&")
	sig := eo.hmacSHA256(eo.SecretKey, signStr)

	queryParts = append(queryParts, "Signature="+sig)
	reqBody := strings.Join(queryParts, "&")

	req, err := http.NewRequest("POST", "https://teo.tencentcloudapi.com/", strings.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := eo.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (eo *EdgeOneClient) hmacSHA256(key, data string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}
