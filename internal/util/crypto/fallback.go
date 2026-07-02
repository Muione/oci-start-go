package crypto

// DecryptStringWithFallback decrypts an AES-256-GCM envelope produced by
// EncryptString. If raw is not a valid envelope (plain-text legacy data, empty
// string, wrong key, or tampered blob), the input is returned verbatim — a
// plaintext-degrade path that keeps older rows readable during migration.
//
// ponytail: no logging of failure reason here — callers may pipe raw into
// audit paths; logging ciphertext/decode errors risks leaking shape info.
func DecryptStringWithFallback(raw string, masterKey []byte) string {
	if raw == "" {
		return ""
	}
	pt, err := DecryptString(raw, masterKey)
	if err != nil {
		return raw
	}
	return pt
}
