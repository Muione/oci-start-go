package oci

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/Muione/oci-start-go/internal/util/crypto"
)

// genPEM generates a 2048 RSA private key in PKCS#8/SEC1 PEM form, like an OCI
// API key file.
func genPEM(t *testing.T) string {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	b := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
	return string(b)
}

func TestNewProvider_EncryptedKey(t *testing.T) {
	pemStr := genPEM(t)
	gen, err := crypto.GenerateMasterKey()
	if err != nil {
		t.Fatalf("master key: %v", err)
	}
	key, err := base64.StdEncoding.DecodeString(gen)
	if err != nil {
		t.Fatalf("decode master key: %v", err)
	}
	blob, err := crypto.EncryptString(pemStr, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if blob == pemStr || strings.Contains(blob, "PRIVATE KEY") {
		t.Fatal("ciphertext must not contain plaintext PEM")
	}
	creds := Credentials{
		Tenancy:     "ocid1.tenancy.oc1..aaaa",
		UserID:      "ocid1.user.oc1..aaaa",
		Fingerprint: "aa:bb:cc:dd:ee:ff",
		Region:      "东京",
		KeyFileBlob: blob,
	}
	prov, err := NewProvider(creds, key)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if tenancy, _ := prov.TenancyOCID(); tenancy != creds.Tenancy {
		t.Errorf("TenancyOCID=%q want %q", tenancy, creds.Tenancy)
	}
	if u, _ := prov.UserOCID(); u != creds.UserID {
		t.Errorf("UserOCID=%q want %q", u, creds.UserID)
	}
	if r, _ := prov.Region(); r != "ap-tokyo-1" {
		t.Errorf("Region=%q want ap-tokyo-1", r)
	}
}

func TestNewProvider_MissingFields(t *testing.T) {
	_, err := NewProvider(Credentials{Region: "东京", KeyFileBlob: "x"}, []byte("0123456789abcdef"))
	if err == nil {
		t.Fatal("expected error on missing tenancy/user/fingerprint")
	}
}
