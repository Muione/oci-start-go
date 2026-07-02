// Package oci — credentials.go: per-user IAM credential management via the
// Identity Domains (IDCS SCIM) API. Covers API keys, auth tokens, SMTP
// credentials, and customer secret keys — list (filtered by user), create
// (returning one-time secrets where applicable), and delete.
//
// Mirrors the IDCS helper pattern in identity.go: getIdDomainsClient → SDK
// call → flat response struct. derefStr/derefBool live in helpers.go /
// osp_gateway.go.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identitydomains"
)

// SCIM schema URIs for the credential resource types (IDCS convention).
const (
	schemaAPIKey            = "urn:ietf:params:scim:schemas:oracle:idcs:ApiKey"
	schemaAuthToken         = "urn:ietf:params:scim:schemas:oracle:idcs:AuthToken"
	schemaSmtpCredential    = "urn:ietf:params:scim:schemas:oracle:idcs:SmtpCredential"
	schemaCustomerSecretKey = "urn:ietf:params:scim:schemas:oracle:idcs:CustomerSecretKey"
)

// userFilter returns the SCIM filter expression selecting resources owned by
// userOCID. IDCS exposes user as a complex ref with a "value" sub-attribute.
func userFilter(userOCID string) *string {
	return common.String(fmt.Sprintf(`user.value eq "%s"`, userOCID))
}

// ─── API Keys ───────────────────────────────────────────────────────────────

// ApiKeyInfo is a flat view of an IAM user API key.
type ApiKeyInfo struct {
	Id          string `json:"id"`
	Fingerprint string `json:"fingerprint"`
	Key         string `json:"key"`
}

// ListApiKeys lists API keys for a user.
func ListApiKeys(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID, userOCID string) ([]ApiKeyInfo, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	resp, err := client.ListApiKeys(ctx, identitydomains.ListApiKeysRequest{
		Filter: userFilter(userOCID),
		Count:  common.Int(100),
	})
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	out := make([]ApiKeyInfo, 0, len(resp.ApiKeys.Resources))
	for _, k := range resp.ApiKeys.Resources {
		out = append(out, ApiKeyInfo{Id: derefStr(k.Id), Fingerprint: derefStr(k.Fingerprint), Key: derefStr(k.Key)})
	}
	return out, nil
}

// CreateApiKeyResult holds the created API key (fingerprint is computed by OCI).
type CreateApiKeyResult struct {
	Id          string `json:"id"`
	Fingerprint string `json:"fingerprint"`
	Key         string `json:"key"`
}

// CreateApiKey uploads a public key PEM for a user. OCI computes the fingerprint.
func CreateApiKey(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID, userOCID, pem string) (*CreateApiKeyResult, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	resp, err := client.CreateApiKey(ctx, identitydomains.CreateApiKeyRequest{
		ApiKey: identitydomains.ApiKey{
			Schemas: []string{schemaAPIKey},
			Key:     common.String(pem),
			User:    &identitydomains.ApiKeyUser{Value: common.String(userOCID)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}
	return &CreateApiKeyResult{
		Id:          derefStr(resp.ApiKey.Id),
		Fingerprint: derefStr(resp.ApiKey.Fingerprint),
		Key:         derefStr(resp.ApiKey.Key),
	}, nil
}

// DeleteApiKey deletes an API key by its IDCS resource id.
func DeleteApiKey(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID, keyID string) error {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return err
	}
	_, err = client.DeleteApiKey(ctx, identitydomains.DeleteApiKeyRequest{
		ApiKeyId: common.String(keyID),
	})
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	return nil
}

// ─── Auth Tokens ────────────────────────────────────────────────────────────

// AuthTokenInfo is a flat view of an IAM user auth token (the one-time Token
// value is only present right after create; list returns it empty).
type AuthTokenInfo struct {
	Id          string `json:"id"`
	Description string `json:"description"`
	Token       string `json:"token,omitempty"` // one-time, only on create
}

// ListAuthTokens lists auth tokens for a user.
func ListAuthTokens(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID, userOCID string) ([]AuthTokenInfo, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	resp, err := client.ListAuthTokens(ctx, identitydomains.ListAuthTokensRequest{
		Filter: userFilter(userOCID),
		Count:  common.Int(100),
	})
	if err != nil {
		return nil, fmt.Errorf("list auth tokens: %w", err)
	}
	out := make([]AuthTokenInfo, 0, len(resp.AuthTokens.Resources))
	for _, t := range resp.AuthTokens.Resources {
		out = append(out, AuthTokenInfo{Id: derefStr(t.Id), Description: derefStr(t.Description)})
	}
	return out, nil
}

// CreateAuthToken creates an auth token for a user and returns the one-time token value.
func CreateAuthToken(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID, userOCID, description string) (*AuthTokenInfo, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	resp, err := client.CreateAuthToken(ctx, identitydomains.CreateAuthTokenRequest{
		AuthToken: identitydomains.AuthToken{
			Schemas:     []string{schemaAuthToken},
			Description: common.String(description),
			User:        &identitydomains.AuthTokenUser{Value: common.String(userOCID)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create auth token: %w", err)
	}
	return &AuthTokenInfo{
		Id:          derefStr(resp.AuthToken.Id),
		Description: derefStr(resp.AuthToken.Description),
		Token:       derefStr(resp.AuthToken.Token),
	}, nil
}

// DeleteAuthToken deletes an auth token by id.
func DeleteAuthToken(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID, tokenID string) error {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return err
	}
	_, err = client.DeleteAuthToken(ctx, identitydomains.DeleteAuthTokenRequest{
		AuthTokenId: common.String(tokenID),
	})
	if err != nil {
		return fmt.Errorf("delete auth token: %w", err)
	}
	return nil
}

// ─── SMTP Credentials ───────────────────────────────────────────────────────

// SmtpCredentialInfo is a flat view of an IAM SMTP credential. UserName and
// Password are one-time values only present right after create; list returns
// the UserName but not the Password.
type SmtpCredentialInfo struct {
	Id          string `json:"id"`
	Description string `json:"description"`
	UserName    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"` // one-time, only on create
}

// ListSmtpCredentials lists SMTP credentials for a user.
func ListSmtpCredentials(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID, userOCID string) ([]SmtpCredentialInfo, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	resp, err := client.ListSmtpCredentials(ctx, identitydomains.ListSmtpCredentialsRequest{
		Filter: userFilter(userOCID),
		Count:  common.Int(100),
	})
	if err != nil {
		return nil, fmt.Errorf("list smtp credentials: %w", err)
	}
	out := make([]SmtpCredentialInfo, 0, len(resp.SmtpCredentials.Resources))
	for _, c := range resp.SmtpCredentials.Resources {
		out = append(out, SmtpCredentialInfo{Id: derefStr(c.Id), Description: derefStr(c.Description), UserName: derefStr(c.UserName)})
	}
	return out, nil
}

// CreateSmtpCredential creates an SMTP credential for a user. OCI generates the
// UserName and the one-time Password (UserName is readOnly on the struct).
func CreateSmtpCredential(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID, userOCID, description string) (*SmtpCredentialInfo, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	resp, err := client.CreateSmtpCredential(ctx, identitydomains.CreateSmtpCredentialRequest{
		SmtpCredential: identitydomains.SmtpCredential{
			Schemas:     []string{schemaSmtpCredential},
			Description: common.String(description),
			User:        &identitydomains.SmtpCredentialUser{Value: common.String(userOCID)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create smtp credential: %w", err)
	}
	return &SmtpCredentialInfo{
		Id:          derefStr(resp.SmtpCredential.Id),
		Description: derefStr(resp.SmtpCredential.Description),
		UserName:    derefStr(resp.SmtpCredential.UserName),
		Password:    derefStr(resp.SmtpCredential.Password),
	}, nil
}

// DeleteSmtpCredential deletes an SMTP credential by id.
func DeleteSmtpCredential(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID, credID string) error {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return err
	}
	_, err = client.DeleteSmtpCredential(ctx, identitydomains.DeleteSmtpCredentialRequest{
		SmtpCredentialId: common.String(credID),
	})
	if err != nil {
		return fmt.Errorf("delete smtp credential: %w", err)
	}
	return nil
}

// ─── Customer Secret Keys ───────────────────────────────────────────────────

// CustomerSecretKeyInfo is a flat view of an IAM customer secret key. The
// SecretKey (access key) is a one-time value only present right after create.
type CustomerSecretKeyInfo struct {
	Id          string `json:"id"`
	DisplayName string `json:"displayName"`
	SecretKey   string `json:"secretKey,omitempty"` // one-time, only on create
}

// ListCustomerSecretKeys lists customer secret keys for a user.
func ListCustomerSecretKeys(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID, userOCID string) ([]CustomerSecretKeyInfo, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	resp, err := client.ListCustomerSecretKeys(ctx, identitydomains.ListCustomerSecretKeysRequest{
		Filter: userFilter(userOCID),
		Count:  common.Int(100),
	})
	if err != nil {
		return nil, fmt.Errorf("list customer secret keys: %w", err)
	}
	out := make([]CustomerSecretKeyInfo, 0, len(resp.CustomerSecretKeys.Resources))
	for _, k := range resp.CustomerSecretKeys.Resources {
		out = append(out, CustomerSecretKeyInfo{Id: derefStr(k.Id), DisplayName: derefStr(k.DisplayName)})
	}
	return out, nil
}

// CreateCustomerSecretKey creates a customer secret key for a user and returns
// the one-time secret access key.
func CreateCustomerSecretKey(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID, userOCID, displayName string) (*CustomerSecretKeyInfo, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	resp, err := client.CreateCustomerSecretKey(ctx, identitydomains.CreateCustomerSecretKeyRequest{
		CustomerSecretKey: identitydomains.CustomerSecretKey{
			Schemas:     []string{schemaCustomerSecretKey},
			DisplayName: common.String(displayName),
			User:        &identitydomains.CustomerSecretKeyUser{Value: common.String(userOCID)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create customer secret key: %w", err)
	}
	return &CustomerSecretKeyInfo{
		Id:          derefStr(resp.CustomerSecretKey.Id),
		DisplayName: derefStr(resp.CustomerSecretKey.DisplayName),
		SecretKey:   derefStr(resp.CustomerSecretKey.SecretKey),
	}, nil
}

// DeleteCustomerSecretKey deletes a customer secret key by id.
func DeleteCustomerSecretKey(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID, keyID string) error {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return err
	}
	_, err = client.DeleteCustomerSecretKey(ctx, identitydomains.DeleteCustomerSecretKeyRequest{
		CustomerSecretKeyId: common.String(keyID),
	})
	if err != nil {
		return fmt.Errorf("delete customer secret key: %w", err)
	}
	return nil
}
