// Package openresty provides an HTTP client for the OpenResty management API.
// The API is a sidecar service running on the same host that manages OpenResty
// configuration, SSL certs, and service lifecycle. See Phase 12.1 SPEC section 6.
package openresty

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps net/http.Client for the OpenResty management API.
type Client struct {
	BaseURL    string
	APIToken   string
	HTTPClient *http.Client
}

// New creates an OpenResty API client.
func New(baseURL, apiToken string) *Client {
	return &Client{
		BaseURL:  baseURL,
		APIToken: apiToken,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// StatusResponse is the response from GET /api/test.
type StatusResponse struct {
	Status string `json:"status"`
}

// ConfigTestResponse is the response from POST /api/config/test.
type ConfigTestResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// SSLCertRequest is the request body for POST /api/ssl/certs.
type SSLCertRequest struct {
	Domain       string `json:"domain"`
	Cert         string `json:"cert"`
	Key          string `json:"key"`
	ForceReplace bool   `json:"force_replace"`
}

// SSLCertInfo represents a cert entry from the OpenResty API.
type SSLCertInfo struct {
	Domain     string `json:"domain"`
	CertPath   string `json:"cert_path"`
	KeyPath    string `json:"key_path"`
	ExpiryDate string `json:"expiry_date"`
}

// do executes an HTTP request and returns the response body.
func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIToken != "" {
		req.Header.Set("X-API-Token", c.APIToken)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return respBody, resp.StatusCode, nil
}

// HealthCheck calls GET /api/test to verify the OpenResty API is reachable.
func (c *Client) HealthCheck(ctx context.Context) (bool, error) {
	_, status, err := c.do(ctx, http.MethodGet, "/test", nil)
	if err != nil {
		return false, err
	}
	return status >= 200 && status < 300, nil
}

// TestConfig calls POST /api/config/test to syntax-check nginx config content.
func (c *Client) TestConfig(ctx context.Context, content string) (bool, string, error) {
	body := map[string]string{"content": content}
	respBody, status, err := c.do(ctx, http.MethodPost, "/config/test", body)
	if err != nil {
		return false, "", err
	}
	if status >= 200 && status < 300 {
		var resp ConfigTestResponse
		if err := json.Unmarshal(respBody, &resp); err == nil {
			return resp.Valid, resp.Message, nil
		}
		return true, "", nil
	}
	return false, string(respBody), nil
}

// PushConfig calls PUT /api/config to push nginx config content to OpenResty.
func (c *Client) PushConfig(ctx context.Context, content string) error {
	body := map[string]string{"content": content}
	_, status, err := c.do(ctx, http.MethodPut, "/config", body)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("push config failed: HTTP %d", status)
	}
	return nil
}

// Reload calls POST /api/config/reload to reload OpenResty.
func (c *Client) Reload(ctx context.Context) error {
	_, status, err := c.do(ctx, http.MethodPost, "/config/reload", nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("reload failed: HTTP %d", status)
	}
	return nil
}

// UploadSSLCert calls POST /api/ssl/certs to upload an SSL certificate.
func (c *Client) UploadSSLCert(ctx context.Context, domain, cert, key string, forceReplace bool) error {
	body := SSLCertRequest{
		Domain:       domain,
		Cert:         cert,
		Key:          key,
		ForceReplace: forceReplace,
	}
	_, status, err := c.do(ctx, http.MethodPost, "/ssl/certs", body)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("upload cert failed: HTTP %d", status)
	}
	return nil
}

// ListSSLCerts calls GET /api/ssl/certs to list uploaded certificates.
func (c *Client) ListSSLCerts(ctx context.Context) ([]SSLCertInfo, error) {
	body, status, err := c.do(ctx, http.MethodGet, "/ssl/certs", nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("list certs failed: HTTP %d", status)
	}
	var certs []SSLCertInfo
	if err := json.Unmarshal(body, &certs); err != nil {
		return nil, fmt.Errorf("decode certs: %w", err)
	}
	return certs, nil
}

// GetSSLCert calls GET /api/ssl/certs/{domain} to get a certificate by domain.
func (c *Client) GetSSLCert(ctx context.Context, domain string) (*SSLCertInfo, error) {
	body, status, err := c.do(ctx, http.MethodGet, "/ssl/certs/"+domain, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("get cert failed: HTTP %d", status)
	}
	var cert SSLCertInfo
	if err := json.Unmarshal(body, &cert); err != nil {
		return nil, fmt.Errorf("decode cert: %w", err)
	}
	return &cert, nil
}

// DeleteSSLCert calls DELETE /api/ssl/certs/{domain} to delete a certificate.
func (c *Client) DeleteSSLCert(ctx context.Context, domain string) error {
	_, status, err := c.do(ctx, http.MethodDelete, "/ssl/certs/"+domain, nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("delete cert failed: HTTP %d", status)
	}
	return nil
}
