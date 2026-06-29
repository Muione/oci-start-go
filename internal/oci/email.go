// Package oci -- email.go: OCI Email Delivery SDK operations.
// Port of Java OciEmailUtils: email domain, sender, DKIM, and SMTP credential
// management via the OCI Email and Identity APIs.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/email"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

// EmailOps groups all OCI Email Delivery SDK operations.
type EmailOps struct{}

// --- Email Domain ---

// CreateEmailDomain creates an OCI email domain. If the domain already exists,
// it returns the existing domain's ID (idempotent).
func (e *EmailOps) CreateEmailDomain(ctx context.Context, client *email.EmailClient, compartmentID, domainName string) (string, error) {
	// Check if domain already exists.
	existing, err := e.FindEmailDomainByName(ctx, client, compartmentID, domainName)
	if err == nil && existing != "" {
		return existing, nil
	}

	req := email.CreateEmailDomainRequest{
		CreateEmailDomainDetails: email.CreateEmailDomainDetails{
			CompartmentId: &compartmentID,
			Name:          &domainName,
		},
	}
	resp, err := client.CreateEmailDomain(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create email domain: %w", err)
	}
	if resp.Id == nil {
		return "", fmt.Errorf("create email domain: nil ID in response")
	}
	return *resp.Id, nil
}

// FindEmailDomainByName looks up an email domain by name in the compartment.
// Returns the domain OCID or empty string if not found.
func (e *EmailOps) FindEmailDomainByName(ctx context.Context, client *email.EmailClient, compartmentID, domainName string) (string, error) {
	req := email.ListEmailDomainsRequest{
		CompartmentId: &compartmentID,
		Name:          &domainName,
		Limit:         common.Int(10),
	}
	resp, err := client.ListEmailDomains(ctx, req)
	if err != nil {
		return "", fmt.Errorf("list email domains: %w", err)
	}
	for _, d := range resp.Items {
		if d.Name != nil && *d.Name == domainName && d.Id != nil {
			return *d.Id, nil
		}
	}
	return "", nil
}

// DeleteEmailDomain deletes an OCI email domain by OCID.
func (e *EmailOps) DeleteEmailDomain(ctx context.Context, client *email.EmailClient, domainID string) error {
	req := email.DeleteEmailDomainRequest{
		EmailDomainId: &domainID,
	}
	_, err := client.DeleteEmailDomain(ctx, req)
	if err != nil {
		return fmt.Errorf("delete email domain %s: %w", domainID, err)
	}
	return nil
}

// --- Sender ---

// CreateSender creates an OCI approved sender. If the sender already exists,
// it returns the existing sender's ID (idempotent).
func (e *EmailOps) CreateSender(ctx context.Context, client *email.EmailClient, compartmentID, emailAddress string) (string, error) {
	existing, err := e.FindSenderByEmail(ctx, client, compartmentID, emailAddress)
	if err == nil && existing != "" {
		return existing, nil
	}

	req := email.CreateSenderRequest{
		CreateSenderDetails: email.CreateSenderDetails{
			CompartmentId: &compartmentID,
			EmailAddress:  &emailAddress,
		},
	}
	resp, err := client.CreateSender(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create sender: %w", err)
	}
	if resp.Id == nil {
		return "", fmt.Errorf("create sender: nil ID in response")
	}
	return *resp.Id, nil
}

// FindSenderByEmail looks up an approved sender by email address.
// Returns the sender OCID or empty string if not found.
func (e *EmailOps) FindSenderByEmail(ctx context.Context, client *email.EmailClient, compartmentID, emailAddress string) (string, error) {
	req := email.ListSendersRequest{
		CompartmentId: &compartmentID,
		EmailAddress:  &emailAddress,
		Limit:         common.Int(10),
	}
	resp, err := client.ListSenders(ctx, req)
	if err != nil {
		return "", fmt.Errorf("list senders: %w", err)
	}
	for _, s := range resp.Items {
		if s.EmailAddress != nil && *s.EmailAddress == emailAddress && s.Id != nil {
			return *s.Id, nil
		}
	}
	return "", nil
}

// DeleteSender deletes an OCI approved sender by OCID.
func (e *EmailOps) DeleteSender(ctx context.Context, client *email.EmailClient, senderID string) error {
	req := email.DeleteSenderRequest{
		SenderId: &senderID,
	}
	_, err := client.DeleteSender(ctx, req)
	if err != nil {
		return fmt.Errorf("delete sender %s: %w", senderID, err)
	}
	return nil
}

// --- DKIM ---

// DkimResult holds the result of a DKIM creation.
type DkimResult struct {
	DkimID           string
	CnameRecordValue string
}

// CreateDkim creates an OCI DKIM for an email domain. If a DKIM already exists
// for the domain, it returns the existing one (idempotent).
func (e *EmailOps) CreateDkim(ctx context.Context, client *email.EmailClient, domainID string) (*DkimResult, error) {
	existing, err := e.FindDkimByDomainId(ctx, client, domainID)
	if err == nil && existing != nil {
		return existing, nil
	}

	req := email.CreateDkimRequest{
		CreateDkimDetails: email.CreateDkimDetails{
			EmailDomainId: &domainID,
		},
	}
	resp, err := client.CreateDkim(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create dkim: %w", err)
	}
	result := &DkimResult{}
	if resp.Id != nil {
		result.DkimID = *resp.Id
	}
	if resp.CnameRecordValue != nil {
		result.CnameRecordValue = *resp.CnameRecordValue
	}
	return result, nil
}

// FindDkimByDomainId finds an existing DKIM for the given email domain.
func (e *EmailOps) FindDkimByDomainId(ctx context.Context, client *email.EmailClient, domainID string) (*DkimResult, error) {
	req := email.ListDkimsRequest{
		EmailDomainId: &domainID,
		Limit:         common.Int(10),
	}
	resp, err := client.ListDkims(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list dkims: %w", err)
	}
	for _, d := range resp.Items {
		// Return the first active or creating DKIM.
		state := string(d.LifecycleState)
		if (state == "ACTIVE" || state == "CREATING") && d.Id != nil {
			// Fetch full DKIM details to get CnameRecordValue.
			getReq := email.GetDkimRequest{DkimId: d.Id}
			getResp, err := client.GetDkim(ctx, getReq)
			if err != nil {
				continue
			}
			result := &DkimResult{DkimID: *d.Id}
			if getResp.CnameRecordValue != nil {
				result.CnameRecordValue = *getResp.CnameRecordValue
			}
			return result, nil
		}
	}
	return nil, nil
}

// DeleteDkim deletes an OCI DKIM by OCID.
func (e *EmailOps) DeleteDkim(ctx context.Context, client *email.EmailClient, dkimID string) error {
	req := email.DeleteDkimRequest{
		DkimId: &dkimID,
	}
	_, err := client.DeleteDkim(ctx, req)
	if err != nil {
		return fmt.Errorf("delete dkim %s: %w", dkimID, err)
	}
	return nil
}

// --- SMTP Credentials ---

// SmtpCredsResult holds the result of SMTP credential generation.
type SmtpCredsResult struct {
	CredentialID string
	Username     string
	Password     string
}

// GenerateSmtpCredentials creates SMTP credentials for the given OCI user.
// The password is only available at creation time.
func (e *EmailOps) GenerateSmtpCredentials(ctx context.Context, client *identity.IdentityClient, userID, description string) (*SmtpCredsResult, error) {
	req := identity.CreateSmtpCredentialRequest{
		CreateSmtpCredentialDetails: identity.CreateSmtpCredentialDetails{
			Description: &description,
		},
		UserId: &userID,
	}
	resp, err := client.CreateSmtpCredential(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create smtp credential: %w", err)
	}
	result := &SmtpCredsResult{}
	if resp.Id != nil {
		result.CredentialID = *resp.Id
	}
	if resp.Username != nil {
		result.Username = *resp.Username
	}
	if resp.Password != nil {
		result.Password = *resp.Password
	}
	return result, nil
}

// DeleteSmtpCredentials deletes SMTP credentials for a user.
func (e *EmailOps) DeleteSmtpCredentials(ctx context.Context, client *identity.IdentityClient, userID, credentialID string) error {
	req := identity.DeleteSmtpCredentialRequest{
		UserId:           &userID,
		SmtpCredentialId: &credentialID,
	}
	_, err := client.DeleteSmtpCredential(ctx, req)
	if err != nil {
		return fmt.Errorf("delete smtp credential %s: %w", credentialID, err)
	}
	return nil
}
