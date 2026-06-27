// Package totp wraps pquerna/otp to mirror Java GoogleAuthenticatorDef.verify
// (RFC6238, 6 digits, 30s step, ±30s = prev/curr/next window). See SPEC §7.3.
package totp

import (
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
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
