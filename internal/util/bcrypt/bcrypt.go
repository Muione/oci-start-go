// Package bcrypt wraps golang.org/x/crypto/bcrypt at Spring's default strength
// (cost 10) for password hash/verify parity. See SPEC §7.
package bcrypt

import "golang.org/x/crypto/bcrypt"

const cost = 10 // BCryptPasswordEncoder default strength

// Hash returns a bcrypt hash of the plaintext (cost 10).
func Hash(plaintext string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plaintext), cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Compare returns nil on match.
func Compare(hash, plaintext string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
}
