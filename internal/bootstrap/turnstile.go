package bootstrap

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/sysconf"
)

// BypassTokenHolder carries the single-use Turnstile emergency bypass token for
// the current boot (parity with Init.checkAndLogTurnstileBypass). NOT phone-home
// (D3) — it is a security bypass, not telemetry.
type BypassTokenHolder struct {
	mu    sync.Mutex
	token string // "" when Turnstile is disabled
}

// CheckAndLog generates + logs the bypass token iff SystemConfig
// turnstile.enabled==true. If localBypass is true, force-disables Turnstile
// instead (parity: disableTurnstileIfLocalBypass).
func (h *BypassTokenHolder) CheckAndLog(ctx context.Context, sc *sysconf.Service, localBypass bool, logger zerolog.Logger) error {
	if localBypass {
		if err := sc.SetEnabled(ctx, "turnstile.enabled", false); err != nil {
			return err
		}
		logger.Info().Msg("turnstile force-disabled (local bypass)")
		return nil
	}
	if !sc.GetBool(ctx, "turnstile.enabled") {
		return nil
	}
	tok, err := randomHex(8) // 16 hex chars
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.token = tok
	h.mu.Unlock()
	logger.Warn().
		Str("url", "/api/disTurnstile?token="+tok).
		Msg("turnstile enabled; emergency bypass token generated (single-use, current boot)")
	return nil
}

// ConsumeAndRotate returns true iff t matches the current token; the token is
// then consumed (single-use).
func (h *BypassTokenHolder) ConsumeAndRotate(t string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.token != "" && t == h.token {
		h.token = ""
		return true
	}
	return false
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
