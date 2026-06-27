// Package gcp — auth.go: GCP service account JWT OAuth2 token exchange.
// Implements the OAuth2 JWT Bearer flow (RFC 7523) using only the standard
// library — no external oauth2 dependencies. Signs a JWT assertion with the
// service account's RSA private key, exchanges it at Google's token endpoint,
// and returns an access token for Compute Engine API calls.
package gcp

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	gcpTokenURL = "https://oauth2.googleapis.com/token"
	gcpScope    = "https://www.googleapis.com/auth/compute"
)

// serviceAccountJSON is the on-disk format of a GCP service account key.
type serviceAccountJSON struct {
	Type        string `json:"type"`
	ProjectID   string `json:"project_id"`
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
	TokenURI    string `json:"token_uri"`
}

// jwtHeader is the fixed JWT RS256 header.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// jwtClaims is the JWT claims set for the OAuth2 token endpoint.
type jwtClaims struct {
	Iss   string `json:"iss"`
	Scope string `json:"scope"`
	Aud   string `json:"aud"`
	Exp   int64  `json:"exp"`
	Iat   int64  `json:"iat"`
}

// tokenResponse is the OAuth2 token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

// GcpAuth holds parsed credentials and provides access tokens.
type GcpAuth struct {
	clientEmail string
	privateKey  *rsa.PrivateKey
	tokenURL    string
	httpClient  *http.Client

	// cached token
	cachedToken string
	expiresAt   time.Time
}

// NewGcpAuth parses a GCP service account JSON blob and returns an
// authenticator that can mint OAuth2 access tokens for the Compute scope.
func NewGcpAuth(credentialsJSON string) (*GcpAuth, error) {
	credsJSON := strings.TrimSpace(credentialsJSON)
	if credsJSON == "" {
		return nil, fmt.Errorf("GCP auth: empty credentials JSON")
	}

	var sa serviceAccountJSON
	if err := json.Unmarshal([]byte(credsJSON), &sa); err != nil {
		return nil, fmt.Errorf("GCP auth: parse service account JSON: %w", err)
	}
	if sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, fmt.Errorf("GCP auth: service account JSON missing client_email or private_key")
	}

	// Parse the PEM-encoded RSA private key.
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return nil, fmt.Errorf("GCP auth: failed to decode PEM private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS1 as fallback (older keys).
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("GCP auth: parse private key: %w", err)
		}
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("GCP auth: private key is not RSA")
	}

	tokenURL := sa.TokenURI
	if tokenURL == "" {
		tokenURL = gcpTokenURL
	}

	return &GcpAuth{
		clientEmail: sa.ClientEmail,
		privateKey:  rsaKey,
		tokenURL:    tokenURL,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// AccessToken returns a valid OAuth2 access token, refreshing if necessary.
func (a *GcpAuth) AccessToken() (string, error) {
	if a.cachedToken != "" && time.Now().Add(30*time.Second).Before(a.expiresAt) {
		return a.cachedToken, nil
	}
	tok, err := a.fetchToken()
	if err != nil {
		return "", err
	}
	a.cachedToken = tok.AccessToken
	a.expiresAt = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	return a.cachedToken, nil
}

// HTTPClient returns an *http.Client that attaches the OAuth2 bearer token
// to every request.
func (a *GcpAuth) HTTPClient() (*http.Client, error) {
	tok, err := a.AccessToken()
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &tokenTransport{
			token:   tok,
			wrapped: http.DefaultTransport,
		},
	}, nil
}

// fetchToken exchanges a signed JWT assertion for an OAuth2 access token.
func (a *GcpAuth) fetchToken() (*tokenResponse, error) {
	jwt, err := a.signJWT()
	if err != nil {
		return nil, fmt.Errorf("GCP auth: sign JWT: %w", err)
	}

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", jwt)

	resp, err := a.httpClient.PostForm(a.tokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("GCP auth: token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("GCP auth: read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GCP auth: token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("GCP auth: parse token response: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("GCP auth: no access_token in response")
	}

	return &tok, nil
}

// signJWT creates a signed JWT assertion per RFC 7523.
func (a *GcpAuth) signJWT() (string, error) {
	now := time.Now().Unix()
	header := jwtHeader{Alg: "RS256", Typ: "JWT"}
	claims := jwtClaims{
		Iss:   a.clientEmail,
		Scope: gcpScope,
		Aud:   a.tokenURL,
		Exp:   now + 3600,
		Iat:   now,
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64

	hashed := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, a.privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}

	sigB64 := base64.RawURLEncoding.EncodeToString(signature)
	return signingInput + "." + sigB64, nil
}

// tokenTransport is an http.RoundTripper that adds an Authorization header.
type tokenTransport struct {
	token   string
	wrapped http.RoundTripper
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.wrapped.RoundTrip(req)
}
