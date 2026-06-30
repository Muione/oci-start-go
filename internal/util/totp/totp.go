// Package totp wraps pquerna/otp to mirror Java GoogleAuthenticatorDef.verify
// (RFC6238, 6 digits, 30s step, ±30s = prev/curr/next window). See SPEC §7.3.
package totp

import (
	"encoding/base64"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	qrcode "github.com/skip2/go-qrcode"
)

// Validate verifies a 6-digit code against a Base32 secret, allowing the
// previous/current/next 30s windows (Skew=1).
func Validate(secretBase32, code string) bool {
	ok, err := totp.ValidateCustom(code, secretBase32, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return err == nil && ok
}

// GenerateSecret creates a new TOTP key and returns the Base32 secret and
// the otpauth:// URL (for QR code generation).
func GenerateSecret(issuer, account string) (secretBase32 string, otpURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
		Algorithm:   otp.AlgorithmSHA1,
		Digits:      otp.DigitsSix,
		Period:      30,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// QRCodeBase64 renders an otpauth:// URL as a PNG QR code and returns it
// as a base64-encoded string (data:image/png;base64,...).
func QRCodeBase64(otpURL string) (string, error) {
	png, err := qrcode.Encode(otpURL, qrcode.Medium, 256)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}
