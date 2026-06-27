package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Muione/oci-start-go/internal/auth"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/response"
	iputil "github.com/Muione/oci-start-go/internal/util/ip"
	bcryptpkg "github.com/Muione/oci-start-go/internal/util/bcrypt"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// ---- OAuth state cache (5min TTL) ----

type StateInfo struct {
	ExpireAt   time.Time
	RememberMe bool
}

type StateCache struct {
	m    sync.Map
	stop chan struct{}
	once sync.Once
}

func NewStateCache() *StateCache {
	s := &StateCache{stop: make(chan struct{})}
	go s.sweep()
	return s
}

func (s *StateCache) sweep() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			now := time.Now()
			s.m.Range(func(k, v any) bool {
				if si, ok := v.(*StateInfo); ok && now.After(si.ExpireAt) {
					s.m.Delete(k)
				}
				return true
			})
		case <-s.stop:
			return
		}
	}
}

func (s *StateCache) Stop() { s.once.Do(func() { close(s.stop) }) }

func (s *StateCache) Put(state string, rememberMe bool) {
	s.m.Store(state, &StateInfo{ExpireAt: time.Now().Add(5 * time.Minute), RememberMe: rememberMe})
}

// Take returns (ok, rememberMe); removes the entry (single-use).
func (s *StateCache) Take(state string) (bool, bool) {
	v, ok := s.m.LoadAndDelete(state)
	if !ok {
		return false, false
	}
	si := v.(*StateInfo)
	if time.Now().After(si.ExpireAt) {
		return false, false
	}
	return true, si.RememberMe
}

// ---- GitHub ----

type githubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

func githubLoginURL(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if !deps.SysConf.GetBool(ctx, "github.enabled") {
			response.Fail(c, http.StatusBadRequest, "GitHub登录未启用")
			return
		}
		clientID := deps.SysConf.GetString(ctx, "github.client.id")
		redirectURI := deps.SysConf.GetString(ctx, "github.redirect.uri")
		state := uuid.NewString()
		deps.OAuthState.Put(state, c.Query("remember-me") == "true" || c.Query("remember-me") == "on")
		authURL := fmt.Sprintf(
			"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=read:user&state=%s",
			clientID, url.QueryEscape(redirectURI), url.QueryEscape(state))
		response.OK(c, response.SuccessData(gin.H{"url": authURL}))
	}
}

func githubStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.OK(c, response.SuccessData(gin.H{"enabled": deps.SysConf.GetBool(c.Request.Context(), "github.enabled")}))
	}
}

func githubCallback(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		code := c.Query("code")
		state := c.Query("state")
		ok, _ := deps.OAuthState.Take(state)
		if code == "" || !ok {
			c.Redirect(http.StatusFound, "/login?error=github_state")
			return
		}
		clientID := deps.SysConf.GetString(ctx, "github.client.id")
		clientSecret := deps.SysConf.GetString(ctx, "github.client.secret")
		redirectURI := deps.SysConf.GetString(ctx, "github.redirect.uri")

		accessToken, err := githubExchangeToken(clientID, clientSecret, code, redirectURI)
		if err != nil || accessToken == "" {
			c.Redirect(http.StatusFound, "/login?error=github_token")
			return
		}
		gh, err := githubFetchUser(accessToken)
		if err != nil {
			c.Redirect(http.StatusFound, "/login?error=github_user")
			return
		}
		allowedID := deps.SysConf.GetString(ctx, "github.myself.githubId")
		if allowedID == "" || allowedID != strconv.FormatInt(gh.ID, 10) {
			c.Redirect(http.StatusFound, "/login?error=github_unauthorized")
			return
		}
		user, err := registerThirdPartyUser(ctx, deps, gh.Login, strconv.FormatInt(gh.ID, 10), "GITHUB")
		if err != nil {
			c.Redirect(http.StatusFound, "/login?error=github_register")
			return
		}
		token, err := deps.Session.Create(ctx, user.Username, iputil.ClientIP(c), c.GetHeader("User-Agent"))
		if err != nil {
			c.Redirect(http.StatusFound, "/login?error=github_session")
			return
		}
		auth.SetSessionCookie(c, token)
		c.Redirect(http.StatusFound, "/")
	}
}

func githubExchangeToken(clientID, clientSecret, code, redirectURI string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"code":          code,
		"redirect_uri":  redirectURI,
	})
	req, err := http.NewRequest(http.MethodPost, "https://github.com/login/oauth/access_token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.AccessToken, nil
}

func githubFetchUser(token string) (*githubUser, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var u githubUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

// ---- Google ----

type googleUser struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func googleLoginURL(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if !deps.SysConf.GetBool(ctx, "google.enabled") {
			response.Fail(c, http.StatusBadRequest, "Google登录未启用")
			return
		}
		clientID := deps.SysConf.GetString(ctx, "google.client.id")
		redirectURI := deps.SysConf.GetString(ctx, "google.redirect.uri")
		state := uuid.NewString()
		deps.OAuthState.Put(state, c.Query("remember-me") == "true" || c.Query("remember-me") == "on")
		authURL := fmt.Sprintf(
			"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=openid%%20email%%20profile&state=%s",
			clientID, url.QueryEscape(redirectURI), url.QueryEscape(state))
		response.OK(c, response.SuccessData(gin.H{"url": authURL}))
	}
}

func googleStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.OK(c, response.SuccessData(gin.H{"enabled": deps.SysConf.GetBool(c.Request.Context(), "google.enabled")}))
	}
}

func googleCallback(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		code := c.Query("code")
		state := c.Query("state")
		ok, _ := deps.OAuthState.Take(state)
		if code == "" || !ok {
			c.Redirect(http.StatusFound, "/login?error=google_state")
			return
		}
		clientID := deps.SysConf.GetString(ctx, "google.client.id")
		clientSecret := deps.SysConf.GetString(ctx, "google.client.secret")
		redirectURI := deps.SysConf.GetString(ctx, "google.redirect.uri")

		accessToken, err := googleExchangeToken(clientID, clientSecret, code, redirectURI)
		if err != nil || accessToken == "" {
			c.Redirect(http.StatusFound, "/login?error=google_token")
			return
		}
		g, err := googleFetchUser(accessToken)
		if err != nil {
			c.Redirect(http.StatusFound, "/login?error=google_user")
			return
		}
		allowedEmail := deps.SysConf.GetString(ctx, "google.client.email")
		if allowedEmail == "" || !strings.EqualFold(allowedEmail, g.Email) {
			c.Redirect(http.StatusFound, "/login?error=google_unauthorized")
			return
		}
		usernamePrefix := g.Name
		if usernamePrefix == "" {
			usernamePrefix = strings.Split(g.Email, "@")[0]
		}
		user, err := registerThirdPartyUser(ctx, deps, usernamePrefix, g.Email, "GOOGLE")
		if err != nil {
			c.Redirect(http.StatusFound, "/login?error=google_register")
			return
		}
		token, err := deps.Session.Create(ctx, user.Username, iputil.ClientIP(c), c.GetHeader("User-Agent"))
		if err != nil {
			c.Redirect(http.StatusFound, "/login?error=google_session")
			return
		}
		auth.SetSessionCookie(c, token)
		c.Redirect(http.StatusFound, "/")
	}
}

func googleExchangeToken(clientID, clientSecret, code, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)
	resp, err := httpClient.PostForm("https://oauth2.googleapis.com/token", form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.AccessToken, nil
}

func googleFetchUser(token string) (*googleUser, error) {
	req, err := http.NewRequest(http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var u googleUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

// registerThirdPartyUser finds or creates a LoginUser for an OAuth identity.
// Existing → update last_login_at; new → insert (random password, role USER).
func registerThirdPartyUser(ctx context.Context, deps *Deps, usernamePrefix, externalID, loginType string) (repo.LoginUser, error) {
	qr := repo.New(deps.Store.Read)
	user, err := qr.FindUserByExternalIdAndLoginType(ctx, repo.FindUserByExternalIdAndLoginTypeParams{
		ExternalID: sql.NullString{String: externalID, Valid: true},
		LoginType:  loginType,
	})
	if err == nil {
		_ = repo.New(deps.Store.Write).UpdateLastLoginAt(ctx, repo.UpdateLastLoginAtParams{
			LastLoginAt: sql.NullString{String: nowStr(), Valid: true},
			Username:    user.Username,
		})
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return repo.LoginUser{}, err
	}
	hash, err := bcryptpkg.Hash(uuid.NewString())
	if err != nil {
		return repo.LoginUser{}, err
	}
	un := usernamePrefix + "_" + strings.ToLower(loginType)
	if e := repo.New(deps.Store.Write).InsertLoginUser(ctx, repo.InsertLoginUserParams{
		Username:    un,
		Password:    hash,
		IsFirstUser: sql.NullInt64{Int64: 0, Valid: true},
		LoginType:   loginType,
		ExternalID:  sql.NullString{String: externalID, Valid: true},
		LastLoginAt: sql.NullString{String: nowStr(), Valid: true},
		Role:        "USER",
	}); e != nil {
		return repo.LoginUser{}, e
	}
	return repo.LoginUser{
		Username:   un,
		LoginType:  loginType,
		ExternalID: sql.NullString{String: externalID, Valid: true},
		Role:       "USER",
	}, nil
}
