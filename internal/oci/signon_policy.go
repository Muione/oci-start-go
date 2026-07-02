// Package oci — signon_policy.go: domain-level sign-on policies and account
// recovery settings via the Identity Domains (IDCS SCIM) API. Sign-on policies
// are read-only; account recovery settings follow the GET→modify→PUT singleton
// pattern used for MFA in identity.go.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identitydomains"
)

// accountRecoverySettingsID is the singleton resource id for the domain's
// account recovery settings (IDCS singleton convention: id == collection name,
// same as AuthenticationFactorSettings / NotificationSettings in identity.go).
const accountRecoverySettingsID = "AccountRecoverySettings"

// ─── Sign-on Policies ───────────────────────────────────────────────────────

// SignOnPolicyInfo is a flat view of an IDCS sign-on policy.
type SignOnPolicyInfo struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
	Ocid        string `json:"ocid"`
}

// ListSignOnPolicies lists all sign-on policies in the identity domain.
func ListSignOnPolicies(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) ([]SignOnPolicyInfo, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	resp, err := client.ListPolicies(ctx, identitydomains.ListPoliciesRequest{
		Count: common.Int(100),
	})
	if err != nil {
		return nil, fmt.Errorf("list sign-on policies: %w", err)
	}
	out := make([]SignOnPolicyInfo, 0, len(resp.Policies.Resources))
	for _, p := range resp.Policies.Resources {
		out = append(out, SignOnPolicyInfo{
			Id:          derefStr(p.Id),
			Name:        derefStr(p.Name),
			Description: derefStr(p.Description),
			Active:      derefBool(p.Active),
			Ocid:        derefStr(p.Ocid),
		})
	}
	return out, nil
}

// ─── Account Recovery Settings ──────────────────────────────────────────────

// AccountRecoveryInfo is a flat view of the domain's account recovery settings.
// Factors are the enabled recovery factors (email, sms, secquestions, push, totp).
type AccountRecoveryInfo struct {
	Factors               []string `json:"factors"`
	MaxIncorrectAttempts  int      `json:"maxIncorrectAttempts"`
	LockoutDuration       int      `json:"lockoutDuration"`
}

// toRecoveryInfo maps the SDK struct to the flat info. Factors are converted
// from enum values to plain strings.
func toRecoveryInfo(s identitydomains.AccountRecoverySetting) AccountRecoveryInfo {
	info := AccountRecoveryInfo{
		MaxIncorrectAttempts: derefInt(s.MaxIncorrectAttempts),
		LockoutDuration:      derefInt(s.LockoutDuration),
	}
	info.Factors = make([]string, 0, len(s.Factors))
	for _, f := range s.Factors {
		info.Factors = append(info.Factors, string(f))
	}
	return info
}

// GetAccountRecoverySetting retrieves the domain's account recovery settings.
func GetAccountRecoverySetting(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) (*AccountRecoveryInfo, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	resp, err := client.GetAccountRecoverySetting(ctx, identitydomains.GetAccountRecoverySettingRequest{
		AccountRecoverySettingId: common.String(accountRecoverySettingsID),
	})
	if err != nil {
		return nil, fmt.Errorf("get account recovery setting: %w", err)
	}
	info := toRecoveryInfo(resp.AccountRecoverySetting)
	return &info, nil
}

// UpdateAccountRecoverySetting replaces the enabled recovery factors. It GETs
// the current settings (preserving MaxIncorrectAttempts/LockoutDuration, which
// are mandatory), sets the new factors, and PUTs the full object — the same
// pattern as ToggleEmailMFA in identity.go.
func UpdateAccountRecoverySetting(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string, factors []string) (*AccountRecoveryInfo, error) {
	if len(factors) == 0 {
		return nil, fmt.Errorf("at least one recovery factor is required")
	}
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	getResp, err := client.GetAccountRecoverySetting(ctx, identitydomains.GetAccountRecoverySettingRequest{
		AccountRecoverySettingId: common.String(accountRecoverySettingsID),
	})
	if err != nil {
		return nil, fmt.Errorf("get account recovery setting: %w", err)
	}
	updated := getResp.AccountRecoverySetting // copy by value
	updated.Factors = make([]identitydomains.AccountRecoverySettingFactorsEnum, 0, len(factors))
	for _, f := range factors {
		updated.Factors = append(updated.Factors, identitydomains.AccountRecoverySettingFactorsEnum(f))
	}
	putResp, err := client.PutAccountRecoverySetting(ctx, identitydomains.PutAccountRecoverySettingRequest{
		AccountRecoverySettingId: common.String(accountRecoverySettingsID),
		AccountRecoverySetting:   updated,
	})
	if err != nil {
		return nil, fmt.Errorf("update account recovery setting: %w", err)
	}
	info := toRecoveryInfo(putResp.AccountRecoverySetting)
	return &info, nil
}
