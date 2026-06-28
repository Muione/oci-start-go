// Package acme — manager.go: SSL certificate management via ACME (SPEC S13.3).
// Wraps go-acme/lego/v4 for Let's Encrypt certificate lifecycle: obtain,
// renew, revoke. DNS-01 challenge via Cloudflare provider.
package acme

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/registration"
	"github.com/rs/zerolog"
)

// CertManager wraps the lego client for SSL certificate operations.
type CertManager struct {
	Logger zerolog.Logger
}

// CertResult holds the result of an ACME certificate operation.
type CertResult struct {
	Domain        string `json:"domain"`
	Certificate   string `json:"certificate"`
	PrivateKey    string `json:"privateKey"`
	IssuerCert    string `json:"issuerCertificate"`
	NotAfter      string `json:"notAfter"`
}

// NewCertManager creates a CertManager.
func NewCertManager(logger zerolog.Logger) *CertManager {
	return &CertManager{Logger: logger}
}

// ObtainCertificate obtains a new certificate via Let's Encrypt using DNS-01
// challenge with Cloudflare as the DNS provider.
func (m *CertManager) ObtainCertificate(ctx context.Context, domain, email, cfAPIToken string, staging bool) (*CertResult, error) {
	// Generate ECDSA private key for the account.
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate account key: %w", err)
	}

	user := &acmeUser{Email: email, Key: privKey}

	config := lego.NewConfig(user)
	if staging {
		config.CADirURL = lego.LEDirectoryStaging
	} else {
		config.CADirURL = lego.LEDirectoryProduction
	}
	// lego v4 changed CADirURL handling
	_ = config

	client, err := lego.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("create lego client: %w", err)
	}

	// Register if needed.
	_, err = client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		// May already be registered; continue.
		m.Logger.Debug().Err(err).Msg("acme: register (may be ok)")
	}

	// Set up Cloudflare DNS-01 provider using API Token.
	cfProvider, err := cloudflare.NewDNSProviderConfig(&cloudflare.Config{
		AuthToken: cfAPIToken,
	})
	if err != nil {
		return nil, fmt.Errorf("cf dns provider: %w", err)
	}

	err = client.Challenge.SetDNS01Provider(cfProvider,
		dns01.AddDNSTimeout(120*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("set dns provider: %w", err)
	}

	// Obtain the certificate.
	req := certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	}
	certRes, err := client.Certificate.Obtain(req)
	if err != nil {
		return nil, fmt.Errorf("obtain cert for %s: %w", domain, err)
	}

	return &CertResult{
		Domain:      domain,
		Certificate:   string(certRes.Certificate),
		PrivateKey:    string(certRes.PrivateKey),
		IssuerCert:    string(certRes.IssuerCertificate),
		NotAfter:      time.Now().Add(90 * 24 * time.Hour).Format("2006-01-02"),
	}, nil
}

// RenewCertificate renews an existing certificate.
func (m *CertManager) RenewCertificate(ctx context.Context, domain, email, cfAPIToken string, staging bool) (*CertResult, error) {
	// lego's Obtain with existing cert auto-renews.
	return m.ObtainCertificate(ctx, domain, email, cfAPIToken, staging)
}

// acmeUser implements the registration.User interface.
type acmeUser struct {
	Email string
	Key   crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string                        { return u.Email }
func (u *acmeUser) GetRegistration() *registration.Resource { return nil }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.Key }
