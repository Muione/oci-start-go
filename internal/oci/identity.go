// Package oci — identity.go: IAM domain operations (compartments, users, groups,
// password policy, MFA, notifications). Ports operations used by OciUtils,
// MFAUtils, NotificationUtils from the Java reference implementation.
//
// Identity Domains (SCIM) API support:
// MFA factor settings, notification settings, and password policy all use
// the OCI Go SDK identitydomains client
// (NewIdentityDomainsClientWithConfigurationProvider), which handles IDCS
// token auth internally — raw OCI-signed HTTP gets 401 on /admin/v1 endpoints.
package oci

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/identitydomains"
)

// ─── Compartment operations ────────────────────────────────────────────────

// ListCompartments returns all active subcompartments reachable from
// tenancyOCID (CompartmentIdInSubtree=true, AccessLevel=Accessible,
// LifecycleState=Active). Note: the tenancy (root compartment) itself is NOT
// included in this list — callers that need the root must add tenancyOCID.
func ListCompartments(ctx context.Context, c Clients, tenancyOCID string) ([]identity.Compartment, error) {
	var out []identity.Compartment
	var page *string
	for {
		resp, err := c.Identity.ListCompartments(ctx, identity.ListCompartmentsRequest{
			CompartmentId:          common.String(tenancyOCID),
			CompartmentIdInSubtree: common.Bool(true),
			AccessLevel:            identity.ListCompartmentsAccessLevelAccessible,
			LifecycleState:         identity.CompartmentLifecycleStateActive,
			Limit:                  common.Int(200),
			Page:                   page,
		})
		if err != nil {
			return nil, fmt.Errorf("list compartments: %w", err)
		}
		out = append(out, resp.Items...)
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return out, nil
}

// PingIdentity tests whether the given credentials are valid by calling
// GetTenancy on the Identity API. Returns nil on success.
func PingIdentity(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) error {
	client, err := identity.NewIdentityClientWithConfigurationProvider(prov)
	if err != nil {
		return fmt.Errorf("create identity client: %w", err)
	}
	_, err = client.GetTenancy(ctx, identity.GetTenancyRequest{
		TenancyId: common.String(tenancyOCID),
	})
	if err != nil {
		return fmt.Errorf("get tenancy: %w", err)
	}
	return nil
}

// ─── Identity Domain Discovery ─────────────────────────────────────────────

// getDomainURL returns the Identity Domain URL for the tenancy by calling
// ListDomains. The domain URL is required to reach SCIM endpoints (password
// policy, MFA settings, notification settings).
func getDomainURL(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) (string, error) {
	client, err := identity.NewIdentityClientWithConfigurationProvider(prov)
	if err != nil {
		return "", fmt.Errorf("create identity client: %w", err)
	}
	resp, err := client.ListDomains(ctx, identity.ListDomainsRequest{
		CompartmentId: common.String(tenancyOCID),
		Limit:         common.Int(10),
	})
	if err != nil {
		return "", fmt.Errorf("list domains: %w", err)
	}
	if len(resp.Items) == 0 {
		return "", fmt.Errorf("no identity domain found for tenancy %s", tenancyOCID)
	}
	// Use the first available domain (typically only one).
	domain := resp.Items[0]
	if domain.Url == nil || *domain.Url == "" {
		return "", fmt.Errorf("identity domain has no URL")
	}
	return *domain.Url, nil
}

// getIdDomainsClient builds an IdentityDomainsClient configured with the
// tenant's domain URL. The SDK client handles IDCS token authentication
// internally (raw HTTP + DefaultRequestSigner gets 401 on /admin/v1 endpoints).
func getIdDomainsClient(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) (identitydomains.IdentityDomainsClient, error) {
	domainURL, err := getDomainURL(ctx, prov, tenancyOCID)
	if err != nil {
		return identitydomains.IdentityDomainsClient{}, fmt.Errorf("get domain URL: %w", err)
	}
	client, err := identitydomains.NewIdentityDomainsClientWithConfigurationProvider(prov, domainURL)
	if err != nil {
		return identitydomains.IdentityDomainsClient{}, fmt.Errorf("create identity domains client: %w", err)
	}
	return client, nil
}

// ─── User operations ───────────────────────────────────────────────────────

// OciUser is a flat representation of an IAM user returned by the list API.
type OciUser struct {
	OCID                    string    `json:"ocid"`
	Name                    string    `json:"name"`
	Email                   string    `json:"email"`
	Domain                  string    `json:"domain"`
	LifecycleState          string    `json:"lifecycleState"`
	TimeCreated             time.Time `json:"timeCreated"`
	LastSuccessfulLoginTime time.Time `json:"lastSuccessfulLoginTime"`
}

// OciGroup is a flat representation of an IAM group.
type OciGroup struct {
	OCID string `json:"ocid"`
	Name string `json:"name"`
}

// ListUsers returns all IAM users in the tenancy (identity domain).
func ListUsers(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) ([]OciUser, error) {
	client, err := identity.NewIdentityClientWithConfigurationProvider(prov)
	if err != nil {
		return nil, fmt.Errorf("create identity client: %w", err)
	}
	var out []OciUser
	var page *string
	for {
		resp, err := client.ListUsers(ctx, identity.ListUsersRequest{
			CompartmentId: common.String(tenancyOCID),
			Limit:         common.Int(200),
			Page:          page,
		})
		if err != nil {
			return nil, fmt.Errorf("list users: %w", err)
		}
		for _, u := range resp.Items {
			ocid := ""
			if u.Id != nil {
				ocid = *u.Id
			}
			name := ""
			if u.Name != nil {
				name = *u.Name
			}
			email := ""
			if u.Email != nil {
				email = *u.Email
			}
			domain := extractDomain(name)
			state := ""
			if u.LifecycleState != "" {
				state = string(u.LifecycleState)
			}
			out = append(out, OciUser{
				OCID:                    ocid,
				Name:                    name,
				Email:                   email,
				Domain:                  domain,
				LifecycleState:          state,
				TimeCreated:             u.TimeCreated.Time,
				LastSuccessfulLoginTime: lastLoginTime(u.LastSuccessfulLoginTime),
			})
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return out, nil
}

// CreateUserRequest holds the parameters for creating an IAM user.
type CreateUserRequest struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	GroupName string `json:"groupName"` // optional: group to add user to
	GroupOCID string `json:"groupOcid"` // optional: group OCID (alternative to GroupName)
}

// CreateUserResult is returned after a successful user creation.
type CreateUserResult struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	UserOCID string `json:"userOcid"`
	Password string `json:"password"` // one-time password
}

// CreateUser creates a new IAM user and optionally adds them to a group.
// Returns the one-time password generated by OCI.
func CreateUser(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string, req CreateUserRequest) (*CreateUserResult, error) {
	client, err := identity.NewIdentityClientWithConfigurationProvider(prov)
	if err != nil {
		return nil, fmt.Errorf("create identity client: %w", err)
	}

	// 1. Create the user
	createResp, err := client.CreateUser(ctx, identity.CreateUserRequest{
		CreateUserDetails: identity.CreateUserDetails{
			CompartmentId: common.String(tenancyOCID),
			Name:          common.String(req.Username),
			Email:         common.String(req.Email),
			Description:   common.String("Created by oci-start-go"),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	userOCID := *createResp.User.Id

	// 2. Add to group if specified (Java reference: createOciAdminUser)
	if req.GroupName != "" || req.GroupOCID != "" {
		var groupOCID string
		if req.GroupOCID != "" {
			groupOCID = req.GroupOCID
		} else {
			groups, err := ListGroups(ctx, prov, tenancyOCID)
			if err != nil {
				return nil, fmt.Errorf("list groups for add-user: %w", err)
			}
			for _, g := range groups {
				if g.Name == req.GroupName {
					groupOCID = g.OCID
					break
				}
			}
			if groupOCID == "" {
				return nil, fmt.Errorf("group %q not found", req.GroupName)
			}
		}
		_, err = client.AddUserToGroup(ctx, identity.AddUserToGroupRequest{
			AddUserToGroupDetails: identity.AddUserToGroupDetails{
				UserId:  common.String(userOCID),
				GroupId: common.String(groupOCID),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("add user to group: %w", err)
		}
	}

	// 3. Create/reset UI password to get a one-time password (Java: SignOnPolicyUtils.resetPass)
	pwResp, err := client.CreateOrResetUIPassword(ctx, identity.CreateOrResetUIPasswordRequest{
		UserId: common.String(userOCID),
	})
	if err != nil {
		return nil, fmt.Errorf("create UI password: %w", err)
	}
	password := ""
	if pwResp.UiPassword.Password != nil {
		password = *pwResp.UiPassword.Password
	}

	return &CreateUserResult{
		Username: req.Username,
		Email:    req.Email,
		UserOCID: userOCID,
		Password: password,
	}, nil
}

// DeleteUser deletes an IAM user by OCID.
// Java reference: SignOnPolicyUtils.deleteUser → identityClient.deleteUser
func DeleteUser(ctx context.Context, prov common.ConfigurationProvider, userOCID string) error {
	client, err := identity.NewIdentityClientWithConfigurationProvider(prov)
	if err != nil {
		return fmt.Errorf("create identity client: %w", err)
	}
	_, err = client.DeleteUser(ctx, identity.DeleteUserRequest{
		UserId: common.String(userOCID),
	})
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// ResetUserPassword resets the console (UI) password for an IAM user.
// Returns the new one-time password.
// Java reference: SignOnPolicyUtils.resetPass → identityClient.CreateOrResetUIPassword
func ResetUserPassword(ctx context.Context, prov common.ConfigurationProvider, userOCID string) (string, error) {
	client, err := identity.NewIdentityClientWithConfigurationProvider(prov)
	if err != nil {
		return "", fmt.Errorf("create identity client: %w", err)
	}
	resp, err := client.CreateOrResetUIPassword(ctx, identity.CreateOrResetUIPasswordRequest{
		UserId: common.String(userOCID),
	})
	if err != nil {
		return "", fmt.Errorf("reset UI password: %w", err)
	}
	if resp.UiPassword.Password != nil {
		return *resp.UiPassword.Password, nil
	}
	return "", nil
}

// ResetMfaForAllUsers resets MFA (TOTP devices) for all users in the tenancy.
// Java reference: TenantServiceImpl.resetAccountFactor → ListMfaTotpDevices + DeleteMfaTotpDevice
func ResetMfaForAllUsers(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) (int, error) {
	client, err := identity.NewIdentityClientWithConfigurationProvider(prov)
	if err != nil {
		return 0, fmt.Errorf("create identity client: %w", err)
	}

	// First, list all users
	users, err := ListUsers(ctx, prov, tenancyOCID)
	if err != nil {
		return 0, fmt.Errorf("list users: %w", err)
	}

	deleted := 0
	for _, u := range users {
		var page *string
		for {
			resp, err := client.ListMfaTotpDevices(ctx, identity.ListMfaTotpDevicesRequest{
				UserId: common.String(u.OCID),
				Page:   page,
			})
			if err != nil {
				// Some users may not support MFA TOTP — skip them
				break
			}
			for _, dev := range resp.Items {
				if dev.Id == nil {
					continue
				}
				_, err := client.DeleteMfaTotpDevice(ctx, identity.DeleteMfaTotpDeviceRequest{
					UserId:      common.String(u.OCID),
					MfaTotpDeviceId: dev.Id,
				})
				if err != nil {
					continue
				}
				deleted++
			}
			if resp.OpcNextPage == nil {
				break
			}
			page = resp.OpcNextPage
		}
	}
	return deleted, nil
}

// ListGroups returns all IAM groups in the tenancy.
// Java reference: OracleInstanceServiceImpl.findGroup → identityClient.listGroups
func ListGroups(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) ([]OciGroup, error) {
	client, err := identity.NewIdentityClientWithConfigurationProvider(prov)
	if err != nil {
		return nil, fmt.Errorf("create identity client: %w", err)
	}
	var out []OciGroup
	var page *string
	for {
		resp, err := client.ListGroups(ctx, identity.ListGroupsRequest{
			CompartmentId: common.String(tenancyOCID),
			Limit:         common.Int(200),
			Page:          page,
		})
		if err != nil {
			return nil, fmt.Errorf("list groups: %w", err)
		}
		for _, g := range resp.Items {
			ocid := ""
			if g.Id != nil {
				ocid = *g.Id
			}
			name := ""
			if g.Name != nil {
				name = *g.Name
			}
			out = append(out, OciGroup{OCID: ocid, Name: name})
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return out, nil
}

// ─── Password Policy operations (Identity Domains SCIM) ────────────────────
//
// Java reference: OciUtils.getCurrentPasswordPolicy, OciUtils.enablePasswordExpirationWithAutoDomain,
// OciUtils.disablePasswordExpirationWithAutoDomain
//
// These use the Identity Domain SCIM API at /admin/v1/PasswordPolicies.

// PasswordPolicyDetail holds the password policy configuration.
type PasswordPolicyDetail struct {
	Name                   string `json:"name"`
	IsPasswordExpiryEnabled bool   `json:"isPasswordExpiryEnabled"`
	PasswordExpiryDays     int    `json:"passwordExpiryDays"`
}

// GetPasswordPolicy retrieves the password expiry policy from the Identity
// Domain. Filters to policies with passwordStrength=Custom (the Java reference
// skips non-Custom policies).
func GetPasswordPolicy(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) (*PasswordPolicyDetail, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	resp, err := client.ListPasswordPolicies(ctx, identitydomains.ListPasswordPoliciesRequest{})
	if err != nil {
		return nil, fmt.Errorf("list password policies: %w", err)
	}
	resources := resp.PasswordPolicies.Resources

	// Find the first Custom-strength policy (Java: filter by PasswordStrength.Custom)
	for _, p := range resources {
		if p.PasswordStrength == identitydomains.PasswordPolicyPasswordStrengthCustom {
			return pwdPolicyToDetail(p), nil
		}
	}
	// Fallback: use the first policy if no Custom one found
	for _, p := range resources {
		return pwdPolicyToDetail(p), nil
	}
	// No policies found — return defaults
	return &PasswordPolicyDetail{IsPasswordExpiryEnabled: false, PasswordExpiryDays: 0}, nil
}

// pwdPolicyToDetail maps an SDK PasswordPolicy to PasswordPolicyDetail.
func pwdPolicyToDetail(p identitydomains.PasswordPolicy) *PasswordPolicyDetail {
	detail := &PasswordPolicyDetail{}
	if p.Name != nil {
		detail.Name = *p.Name
	}
	if p.PasswordExpiresAfter != nil && *p.PasswordExpiresAfter > 0 {
		detail.IsPasswordExpiryEnabled = true
		detail.PasswordExpiryDays = *p.PasswordExpiresAfter
	}
	return detail
}

// UpdatePasswordPolicy enables or disables password expiry by updating the
// Identity Domain password policy.
// Java reference: OciUtils.enablePasswordExpirationWithAutoDomain / disablePasswordExpirationWithAutoDomain
func UpdatePasswordPolicy(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string, enableExpiry bool, expiryDays int) error {
	if enableExpiry && (expiryDays < 0 || expiryDays > 365) {
		return fmt.Errorf("expiry days must be between 0 and 365, got %d", expiryDays)
	}

	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return err
	}

	// 1. List password policies to find a Custom one to update
	listResp, err := client.ListPasswordPolicies(ctx, identitydomains.ListPasswordPoliciesRequest{})
	if err != nil {
		return fmt.Errorf("list password policies: %w", err)
	}
	resources := listResp.PasswordPolicies.Resources
	if len(resources) == 0 {
		return fmt.Errorf("no password policies found in identity domain")
	}

	// Find Custom policy (Java: filter by PasswordStrength.Custom)
	var target *identitydomains.PasswordPolicy
	for i := range resources {
		if resources[i].PasswordStrength == identitydomains.PasswordPolicyPasswordStrengthCustom {
			target = &resources[i]
			break
		}
	}
	if target == nil {
		// Use the first one as fallback
		target = &resources[0]
	}
	if target.Id == nil {
		return fmt.Errorf("target password policy has no ID")
	}

	// 2. Determine expiry settings
	newExpiry := 0
	if enableExpiry {
		if expiryDays > 0 {
			newExpiry = expiryDays
		} else {
			newExpiry = 120 // default
		}
	}

	// 3. Copy the policy, modify expiry fields, PUT full object.
	// Schemas/Name/etc. are preserved from the GET response.
	updated := *target
	updated.PasswordExpiresAfter = common.Int(newExpiry)
	updated.PasswordExpireWarning = common.Int(7) // 7-day warning before expiry
	updated.ForcePasswordReset = common.Bool(false)

	_, err = client.PutPasswordPolicy(ctx, identitydomains.PutPasswordPolicyRequest{
		PasswordPolicyId: target.Id,
		PasswordPolicy:   updated,
	})
	if err != nil {
		return fmt.Errorf("update password policy: %w", err)
	}
	return nil
}

// ─── MFA operations (Identity Domains SCIM) ────────────────────────────────
//
// Java reference: MFAUtils.getMFAConfiguration, MFAUtils.enableEmailMFA
//
// These use the Identity Domain SCIM endpoint at
// /admin/v1/AuthenticationFactorSettings/AuthenticationFactorSettings

// MfaStatus holds the MFA configuration state for a tenancy.
type MfaStatus struct {
	TotpEnabled             bool `json:"totpEnabled"`
	EmailEnabled            bool `json:"emailEnabled"`
	SmsEnabled              bool `json:"smsEnabled"`
	PushEnabled             bool `json:"pushEnabled"`
	SecurityQuestionsEnabled bool `json:"securityQuestionsEnabled"`
	FidoAuthenticatorEnabled bool `json:"fidoAuthenticatorEnabled"`
	PhoneCallEnabled        bool `json:"phoneCallEnabled"`
}

const authFactorSettingsID = "AuthenticationFactorSettings"

// GetMfaStatus retrieves the current MFA configuration.
// Java reference: MFAUtils.getMFAConfiguration
func GetMfaStatus(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) (*MfaStatus, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	resp, err := client.GetAuthenticationFactorSetting(ctx, identitydomains.GetAuthenticationFactorSettingRequest{
		AuthenticationFactorSettingId: common.String(authFactorSettingsID),
	})
	if err != nil {
		return nil, fmt.Errorf("get auth factor settings: %w", err)
	}
	s := resp.AuthenticationFactorSetting
	return &MfaStatus{
		TotpEnabled:              boolPtrVal(s.TotpEnabled),
		EmailEnabled:             boolPtrVal(s.EmailEnabled),
		SmsEnabled:               boolPtrVal(s.SmsEnabled),
		PushEnabled:              boolPtrVal(s.PushEnabled),
		SecurityQuestionsEnabled: boolPtrVal(s.SecurityQuestionsEnabled),
		FidoAuthenticatorEnabled: boolPtrVal(s.FidoAuthenticatorEnabled),
		PhoneCallEnabled:         boolPtrVal(s.PhoneCallEnabled),
	}, nil
}

// ToggleEmailMFA enables or disables email-based MFA for the tenancy.
// Returns the new state (true = enabled, false = disabled).
// Java reference: MFAUtils.enableEmailMFA (GET → modify → PUT full object)
func ToggleEmailMFA(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string, enable bool) (bool, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return false, err
	}
	getResp, err := client.GetAuthenticationFactorSetting(ctx, identitydomains.GetAuthenticationFactorSettingRequest{
		AuthenticationFactorSettingId: common.String(authFactorSettingsID),
	})
	if err != nil {
		return false, fmt.Errorf("get auth factor settings: %w", err)
	}
	settings := getResp.AuthenticationFactorSetting // copy by value
	settings.EmailEnabled = common.Bool(enable)
	_, err = client.PutAuthenticationFactorSetting(ctx, identitydomains.PutAuthenticationFactorSettingRequest{
		AuthenticationFactorSettingId: common.String(authFactorSettingsID),
		AuthenticationFactorSetting:   settings,
	})
	if err != nil {
		return false, fmt.Errorf("update auth factor settings: %w", err)
	}
	return enable, nil
}

// ─── Notification Recipients (Identity Domains SCIM) ───────────────────────
//
// Java reference: NotificationUtils.getCurrentRecipients,
// NotificationUtils.updateNotificationRecipients
//
// Uses the Identity Domain SCIM endpoint at
// /admin/v1/NotificationSettings/NotificationSettings

// NotifRecipient holds a notification email recipient entry.
type NotifRecipient struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	State string `json:"state"`
}

const notificationSettingsID = "NotificationSettings"

// GetNotificationRecipients returns the current notification email recipients.
// Java reference: NotificationUtils.getCurrentRecipients
func GetNotificationRecipients(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) ([]NotifRecipient, error) {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, err
	}
	resp, err := client.GetNotificationSetting(ctx, identitydomains.GetNotificationSettingRequest{
		NotificationSettingId: common.String(notificationSettingsID),
	})
	if err != nil {
		return nil, fmt.Errorf("get notification settings: %w", err)
	}
	recipients := resp.NotificationSetting.TestRecipients
	out := make([]NotifRecipient, 0, len(recipients))
	for i, email := range recipients {
		out = append(out, NotifRecipient{
			ID:    i + 1,
			Email: email,
			State: "VERIFIED",
		})
	}
	return out, nil
}

// UpdateNotificationRecipients replaces the entire notification recipients list.
// Java reference: NotificationUtils.updateNotificationRecipients
func UpdateNotificationRecipients(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string, emails []string) error {
	client, err := getIdDomainsClient(ctx, prov, tenancyOCID)
	if err != nil {
		return err
	}
	getResp, err := client.GetNotificationSetting(ctx, identitydomains.GetNotificationSettingRequest{
		NotificationSettingId: common.String(notificationSettingsID),
	})
	if err != nil {
		return fmt.Errorf("get notification settings: %w", err)
	}
	settings := getResp.NotificationSetting // copy by value
	settings.TestModeEnabled = common.Bool(true)
	settings.TestRecipients = emails
	_, err = client.PutNotificationSetting(ctx, identitydomains.PutNotificationSettingRequest{
		NotificationSettingId: common.String(notificationSettingsID),
		NotificationSetting:   settings,
	})
	if err != nil {
		return fmt.Errorf("update notification settings: %w", err)
	}
	return nil
}

// ─── Tenancy Detail (for auto-fetch) ───────────────────────────────────────
//
// Java reference: OciClassLoader.loadManyRegions / TenancyDetail

// TenancyDetail holds descriptive information about an OCI tenancy,
// fetched from the Identity API for auto-populating local fields.
type TenancyDetail struct {
	TenancyName  string `json:"tenancyName"`  // tenancy display name
	AccountType  string `json:"accountType"`  // "trial" / "paid" / "enterprise" / "free"
	EmailAddress string `json:"emailAddress"` // primary contact email
	Description  string `json:"description"`  // tenancy description
}

// GetTenancyDetail fetches the tenancy metadata from OCI.
// Java reference: OciClassLoader.loadManyRegions uses Identity API + OSP Gateway
// for account type. We use GetTenancy for basic info and infer account type
// from description or subscription metadata.
func GetTenancyDetail(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) (*TenancyDetail, error) {
	client, err := identity.NewIdentityClientWithConfigurationProvider(prov)
	if err != nil {
		return nil, fmt.Errorf("create identity client: %w", err)
	}
	resp, err := client.GetTenancy(ctx, identity.GetTenancyRequest{
		TenancyId: common.String(tenancyOCID),
	})
	if err != nil {
		return nil, fmt.Errorf("get tenancy: %w", err)
	}
	detail := &TenancyDetail{}
	if resp.Tenancy.Name != nil {
		detail.TenancyName = *resp.Tenancy.Name
	}
	if resp.Tenancy.Description != nil {
		detail.Description = *resp.Tenancy.Description
	}

	// Account type is inferred from the tenancy metadata or subscription.
	// The Java reference uses OciGateWayUtils.getAccountTypeInfo which calls
	// the OSP Gateway API for subscription details. The real planType
	// (FREE_TIER/PAYG) is set by UpdateAccountDetail from the subscription API.
	detail.AccountType = ""

	// Email address is not directly available from GetTenancy.
	// The Java reference gets it from subscription details via OSP Gateway.
	// We leave it empty; frontend can populate if needed.
	return detail, nil
}

// --- Subscription Days (BE-001) ---

// SubscriptionDaysInfo holds the subscription duration information for a tenancy.
type SubscriptionDaysInfo struct {
	TimeCreated  time.Time `json:"timeCreated"`
	CurrentTime  time.Time `json:"currentTime"`
	ActiveDays   int64     `json:"activeDays"`
	ActiveMonths float64   `json:"activeMonths"`
	ActiveYears  float64   `json:"activeYears"`
}

// GetTenancyTimeCreated fetches the tenancy creation time from the OCI Identity API.
// Note: Uses GetCompartment instead of GetTenancy because GetTenancy does not return TimeCreated.
func GetTenancyTimeCreated(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) (time.Time, error) {
	client, err := identity.NewIdentityClientWithConfigurationProvider(prov)
	if err != nil {
		return time.Time{}, fmt.Errorf("create identity client: %w", err)
	}
	resp, err := client.GetCompartment(ctx, identity.GetCompartmentRequest{
		CompartmentId: common.String(tenancyOCID),
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("get compartment: %w", err)
	}
	return resp.Compartment.TimeCreated.Time, nil
}

// GetSubscriptionDays calculates the subscription duration from the tenancy's TimeCreated.
func GetSubscriptionDays(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) (*SubscriptionDaysInfo, error) {
	timeCreated, err := GetTenancyTimeCreated(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, fmt.Errorf("get tenancy time created: %w", err)
	}
	now := time.Now().UTC()
	if timeCreated.IsZero() {
		return &SubscriptionDaysInfo{TimeCreated: timeCreated, CurrentTime: now, ActiveDays: 0}, nil
	}
	duration := now.Sub(timeCreated)
	days := int64(duration.Hours() / 24)
	if days < 0 {
		days = 0
	}
	if duration.Nanoseconds()%int64(24*time.Hour) > 0 {
		days++
	}
	return &SubscriptionDaysInfo{
		TimeCreated:  timeCreated,
		CurrentTime:  now,
		ActiveDays:   days,
		ActiveMonths: float64(days) / 30.44,
		ActiveYears:  float64(days) / 365.25,
	}, nil
}

// --- Domain Tenants (BE-003) ---

// DomainInfo holds information about an OCI Identity Domain.
type DomainInfo struct {
	Id             string    `json:"id"`
	DisplayName    string    `json:"displayName"`
	Description    string    `json:"description"`
	Url            string    `json:"url"`
	HomeRegion     string    `json:"homeRegion"`
	Type           string    `json:"type"`
	LicenseType    string    `json:"licenseType"`
	LifecycleState string    `json:"lifecycleState"`
	TimeCreated    time.Time `json:"timeCreated"`
}

// ListDomainTenants lists all active Identity Domains for a tenancy.
func ListDomainTenants(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) ([]DomainInfo, error) {
	client, err := identity.NewIdentityClientWithConfigurationProvider(prov)
	if err != nil {
		return nil, fmt.Errorf("create identity client: %w", err)
	}
	var result []DomainInfo
	var page *string
	for {
		resp, err := client.ListDomains(ctx, identity.ListDomainsRequest{
			CompartmentId: common.String(tenancyOCID),
			Limit:         common.Int(100),
			Page:          page,
		})
		if err != nil {
			return nil, fmt.Errorf("list domains: %w", err)
		}
		for _, d := range resp.Items {
			info := DomainInfo{
				Id:             derefStr(d.Id),
				DisplayName:    derefStr(d.DisplayName),
				Description:    derefStr(d.Description),
				Url:            derefStr(d.Url),
				HomeRegion:     derefStr(d.HomeRegion),
				LicenseType:    derefStr(d.LicenseType),
				LifecycleState: string(d.LifecycleState),
				Type:           string(d.Type),
			}
			if d.TimeCreated != nil {
				info.TimeCreated = d.TimeCreated.Time
			}
			result = append(result, info)
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return result, nil
}

// ─── helpers ───────────────────────────────────────────────────────────────

func extractDomain(userName string) string {
	if idx := strings.Index(userName, "/"); idx >= 0 {
		return userName[:idx]
	}
	return "Default"
}

func lastLoginTime(t *common.SDKTime) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.Time
}

func boolPtrVal(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
