package auth

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
)

const (
	timeFmt       = "2006-01-02 15:04:05"
	absTimeout    = 30 * 24 * time.Hour // 2592000s
	activeTimeout = 2 * time.Hour       // 7200s
	touchInterval = 60 * time.Second    // throttle last_active_at writes
)

// SessionService implements Sa-Token parity: 30d absolute expiry, 2h active
// timeout, single session (new login deletes prior sessions for the username).
type SessionService struct {
	store *db.Store
}

func NewSessionService(store *db.Store) *SessionService { return &SessionService{store: store} }

// Create deletes prior sessions for the username (single session) and inserts
// a new row, atomically. Returns the new token (UUID).
func (s *SessionService) Create(ctx context.Context, username, ip, ua string) (string, error) {
	token := uuid.NewString()
	now := time.Now()
	nowStr := now.Format(timeFmt)
	expStr := now.Add(absTimeout).Format(timeFmt)
	err := s.store.WithTx(ctx, func(tx *sql.Tx) error {
		q := repo.New(tx)
		if err := q.DeleteSessionsByUsername(ctx, username); err != nil {
			return err
		}
		return q.InsertSession(ctx, repo.InsertSessionParams{
			Token:        token,
			Username:     username,
			Ip:           nullString(ip),
			UserAgent:    nullString(ua),
			CreatedAt:    nowStr,
			ExpiresAt:    expStr,
			LastActiveAt: nowStr,
		})
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// Validate loads the session by token; valid iff expires_at>now AND
// last_active_at > now-2h. Returns the username + lastActive (for touch decision).
func (s *SessionService) Validate(ctx context.Context, token string) (string, time.Time, bool) {
	sess, err := repo.New(s.store.Read).FindSessionByToken(ctx, token)
	if err != nil {
		return "", time.Time{}, false
	}
	now := time.Now()
	expires, err := time.ParseInLocation(timeFmt, sess.ExpiresAt, time.Local)
	if err != nil || !now.Before(expires) {
		return "", time.Time{}, false
	}
	last, err := time.ParseInLocation(timeFmt, sess.LastActiveAt, time.Local)
	if err != nil {
		return "", time.Time{}, false
	}
	if now.Sub(last) > activeTimeout {
		return "", time.Time{}, false
	}
	return sess.Username, last, true
}

// Touch updates last_active_at=now (caller gates on now-lastActive>touchInterval).
func (s *SessionService) Touch(ctx context.Context, token string) error {
	return repo.New(s.store.Write).TouchSessionActive(ctx, repo.TouchSessionActiveParams{
		LastActiveAt: time.Now().Format(timeFmt),
		Token:        token,
	})
}

// Delete removes the session row (logout).
func (s *SessionService) Delete(ctx context.Context, token string) error {
	return repo.New(s.store.Write).DeleteSession(ctx, token)
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
