// Package turnstile verifies Cloudflare Turnstile tokens via the siteverify
// endpoint. Mirrors Java RsaDecryptionFilter.callTurnstileApi. See SPEC §7.2.
package turnstile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const verifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type siteverifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify POSTs {secret, response, remoteip} to Cloudflare and returns success.
func Verify(secret, responseToken, remoteIP string) (bool, error) {
	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", responseToken)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	var out siteverifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, err
	}
	return out.Success, nil
}
