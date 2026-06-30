// Package oci is the Oracle Cloud integration layer (SPEC §10). It constructs
// per-request OCI ConfigurationProviders from on-disk tenant credentials (no
// ~/.oci/config), builds service clients, and provides a SOCKS proxy pool.
//
// provider.go is the Go port of Java OciUtils.getProvider: it assembles a
// common.ConfigurationProvider from a Tenant's tenancy OCID, user OCID,
// fingerprint, region, and API private key. Deviation from Java (plan D1):
// the private key is stored AES-256-GCM encrypted in the DB (tenant.
// key_file_blob) rather than as a plaintext PEM file on disk; it is decrypted
// here with the Phase 1 master key and passed to the SDK as a PEM string.
package oci

import (
	"fmt"
	"os"

	"github.com/Muione/oci-start-go/internal/oci/region"
	"github.com/Muione/oci-start-go/internal/util/crypto"
	"github.com/oracle/oci-go-sdk/v65/audit"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/email"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/limits"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/oracle/oci-go-sdk/v65/ospgateway"
	"github.com/oracle/oci-go-sdk/v65/usageapi"
)

// Credentials is the subset of a Tenant needed to build an OCI provider.
// Filled by the service layer from repo.Tenant; kept as a local struct so the
// oci package does not import repo for provider construction.
type Credentials struct {
	Tenancy     string // tenancy OCID (tenant.tenancy)
	UserID      string // user OCID (tenant.tenant_id)
	Fingerprint string
	Region      string // friendly name (e.g. "东京"); converted to code internally
	KeyFileBlob string // base64 AES-256-GCM ciphertext of the PEM (preferred)
	KeyFile     string // legacy plaintext PEM file path (fallback for imported data)
}

// NewProvider builds a raw ConfigurationProvider from tenant credentials.
// Parity with OciUtils.getProvider: userId, fingerprint, tenantId, privateKey,
// region(→code). The private key is decrypted from KeyFileBlob with masterKey;
// if absent, KeyFile is read from disk for import compatibility.
func NewProvider(c Credentials, masterKey []byte) (common.ConfigurationProvider, error) {
	pem, err := resolvePrivateKey(c, masterKey)
	if err != nil {
		return nil, fmt.Errorf("oci: resolve private key: %w", err)
	}
	if c.Tenancy == "" || c.UserID == "" || c.Fingerprint == "" {
		return nil, fmt.Errorf("oci: tenancy/user/fingerprint required")
	}
	code := region.CodeByName(c.Region)
	return common.NewRawConfigurationProvider(c.Tenancy, c.UserID, code, c.Fingerprint, pem, nil), nil
}

func resolvePrivateKey(c Credentials, masterKey []byte) (string, error) {
	if c.KeyFileBlob != "" {
		dec, err := crypto.DecryptString(c.KeyFileBlob, masterKey)
		if err != nil {
			return "", fmt.Errorf("decrypt key_file_blob: %w", err)
		}
		return dec, nil
	}
	if c.KeyFile != "" {
		b, err := os.ReadFile(c.KeyFile)
		if err != nil {
			return "", fmt.Errorf("read key_file %s: %w", c.KeyFile, err)
		}
		return string(b), nil
	}
	return "", fmt.Errorf("no private key (key_file_blob/key_file empty)")
}

// Clients groups the OCI service clients used by the basic domains
// (compute/network/storage/identity). Each embeds common.BaseClient, so an
// HTTPClient can be swapped per-client for proxy injection.
type Clients struct {
	Compute       *core.ComputeClient
	Vcn           *core.VirtualNetworkClient
	Identity      *identity.IdentityClient
	ObjectStorage *objectstorage.ObjectStorageClient
	Blockstorage  *core.BlockstorageClient
	Limits        *limits.LimitsClient
	Audit         *audit.AuditClient
	NLB           *networkloadbalancer.NetworkLoadBalancerClient
	Email         *email.EmailClient       // Phase 12.2: Email Delivery
	OspGateway    *ospgateway.SubscriptionServiceClient // Phase B: OSP Gateway
	UsageApi      *usageapi.UsageapiClient              // Phase B: Usage/Cost API
}

// NewClients builds direct (no-proxy) service clients from a provider.
func NewClients(p common.ConfigurationProvider) (Clients, error) {
	compute, err := core.NewComputeClientWithConfigurationProvider(p)
	if err != nil {
		return Clients{}, fmt.Errorf("compute client: %w", err)
	}
	vcn, err := core.NewVirtualNetworkClientWithConfigurationProvider(p)
	if err != nil {
		return Clients{}, fmt.Errorf("vcn client: %w", err)
	}
	idc, err := identity.NewIdentityClientWithConfigurationProvider(p)
	if err != nil {
		return Clients{}, fmt.Errorf("identity client: %w", err)
	}
	os, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(p)
	if err != nil {
		return Clients{}, fmt.Errorf("objectstorage client: %w", err)
	}
	bs, err := core.NewBlockstorageClientWithConfigurationProvider(p)
	if err != nil {
		return Clients{}, fmt.Errorf("blockstorage client: %w", err)
	}
	limClient, err := limits.NewLimitsClientWithConfigurationProvider(p)
	if err != nil {
		return Clients{}, fmt.Errorf("limits client: %w", err)
	}
	audClient, err := audit.NewAuditClientWithConfigurationProvider(p)
	if err != nil {
		return Clients{}, fmt.Errorf("audit client: %w", err)
	}
	nlbClient, err := networkloadbalancer.NewNetworkLoadBalancerClientWithConfigurationProvider(p)
	if err != nil {
		return Clients{}, fmt.Errorf("nlb client: %w", err)
	}
	emailClient, err := email.NewEmailClientWithConfigurationProvider(p)
	if err != nil {
		return Clients{}, fmt.Errorf("email client: %w", err)
	}
	ospClient, err := ospgateway.NewSubscriptionServiceClientWithConfigurationProvider(p)
	if err != nil {
		return Clients{}, fmt.Errorf("ospgateway client: %w", err)
	}
	usageClient, err := usageapi.NewUsageapiClientWithConfigurationProvider(p)
	if err != nil {
		return Clients{}, fmt.Errorf("usageapi client: %w", err)
	}
	return Clients{Compute: &compute, Vcn: &vcn, Identity: &idc, ObjectStorage: &os, Blockstorage: &bs, Limits: &limClient, Audit: &audClient, NLB: &nlbClient, Email: &emailClient, OspGateway: &ospClient, UsageApi: &usageClient}, nil
}

// NewClientsWithHTTPClient lives in proxy.go (needs net/http). It builds the
// Clients via NewClients then overrides each client's embedded BaseClient.
// HTTPClient field with the given *http.Client for proxy routing.
