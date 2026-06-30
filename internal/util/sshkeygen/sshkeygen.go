// Package sshkeygen generates ephemeral RSA-2048 key pairs for OCI console
// connections. The private key is PEM-encoded (PKCS#8) and the public key is
// in OpenSSH authorized_keys format.
package sshkeygen

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// KeyPair holds a PEM private key and its corresponding SSH public key.
type KeyPair struct {
	PrivateKeyPEM string
	PublicKeySSH  string
}

// GenerateRSA2048 creates a new RSA-2048 key pair suitable for OCI console
// connections. The private key is PKCS#8 PEM; the public key is OpenSSH format
// (single line, no trailing newline).
func GenerateRSA2048() (*KeyPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate RSA key: %w", err)
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})

	pubKey, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("create SSH public key: %w", err)
	}

	pubSSH := string(ssh.MarshalAuthorizedKey(pubKey))
	// Remove trailing newline for OCI API compatibility.
	if len(pubSSH) > 0 && pubSSH[len(pubSSH)-1] == '\n' {
		pubSSH = pubSSH[:len(pubSSH)-1]
	}

	return &KeyPair{
		PrivateKeyPEM: string(privPEM),
		PublicKeySSH:  pubSSH,
	}, nil
}
