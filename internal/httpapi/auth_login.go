package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/auth"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/response"
	iputil "github.com/Muione/oci-start-go/internal/util/ip"
	bcryptpkg "github.com/Muione/oci-start-go/internal/util/bcrypt"
	"github.com/Muione/oci-start-go/internal/util/totp"
	"github.com/Muione/oci-start-go/internal/util/turnstile"
)

type turnstileInfo struct {
	Enabled bool   `json:"enabled"`
	SiteKey string `json:"siteKey"`
}

type loginInitResp struct {
	PreLoginToken       string        `json:"preLoginToken"`
	PublicKey           string        `json:"publicKey"`
	Turnstile           turnstileInfo `json:"turnstile"`
	MfaEnabled          bool          `json:"mfaEnabled"`
	GithubEnabled       bool          `json:"githubEnabled"`
	FirstUserRegistered bool          `json:"firstUserRegistered"`
}

// GET /api/login/init — public. Issues a per-session RSA keypair + reports
// Turnstile/MFA config + whether the first user is already registered.
func loginInit(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, pub, err := deps.Keypair.Issue()
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "生成密钥失败")
			return
		}
		ctx := c.Request.Context()
		tsEnabled := deps.SysConf.GetBool(ctx, "turnstile.enabled")
		tsSiteKey := ""
		if tsEnabled {
			tsSiteKey = deps.SysConf.GetString(ctx, "turnstile.site.key")
		}
		n, _ := repo.New(deps.Store.Read).CountByLoginType(ctx, "LOCAL")
		response.OK(c, response.SuccessData(loginInitResp{
			PreLoginToken:       token,
			PublicKey:           pub,
			Turnstile:           turnstileInfo{Enabled: tsEnabled, SiteKey: tsSiteKey},
			MfaEnabled:          deps.SysConf.GetBool(ctx, "mfa.enabled"),
			GithubEnabled:       deps.SysConf.GetBool(ctx, "github.enabled"),
			FirstUserRegistered: n > 0,
		}))
	}
}

type loginReq struct {
	PreLoginToken  string `json:"preLoginToken"`
	Username       string `json:"username"`
	Password       string `json:"password"` // RSA-encrypted base64
	RememberMe     bool   `json:"rememberMe"`
	TurnstileToken string `json:"turnstileToken"`
	MfaCode        string `json:"mfaCode"`
}

// POST /api/login — public. Turnstile → RSA decrypt → BCrypt → MFA → session.
// No plaintext fallback (plan decision #1).
func login(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求参数无效")
			return
		}
		ctx := c.Request.Context()

		if deps.SysConf.GetBool(ctx, "turnstile.enabled") {
			secret := deps.SysConf.GetString(ctx, "turnstile.secret.key")
			ok, err := turnstile.Verify(secret, req.TurnstileToken, iputil.ClientIP(c))
			if err != nil || !ok {
				response.Fail(c, http.StatusUnauthorized, "Turnstile验证失败")
				return
			}
		}

		plain, err := deps.Keypair.Decrypt(req.PreLoginToken, req.Password)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, "用户名或密码错误")
			return
		}

		user, err := repo.New(deps.Store.Read).FindUserByUsername(ctx, req.Username)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, "用户名或密码错误")
			return
		}
		if err := bcryptpkg.Compare(user.Password, plain); err != nil {
			response.Fail(c, http.StatusUnauthorized, "用户名或密码错误")
			return
		}

		if deps.SysConf.GetBool(ctx, "mfa.enabled") {
			if req.MfaCode == "" {
				response.Fail(c, http.StatusUnauthorized, "请提供MFA验证码")
				return
			}
			secret := deps.SysConf.GetString(ctx, "mfa.secret.key")
			if secret == "" || !totp.Validate(secret, req.MfaCode) {
				response.Fail(c, http.StatusUnauthorized, "MFA验证码错误")
				return
			}
		}

		token, err := deps.Session.Create(ctx, user.Username, iputil.ClientIP(c), c.GetHeader("User-Agent"))
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "创建会话失败")
			return
		}
		_ = repo.New(deps.Store.Write).UpdateLastLoginAt(ctx, repo.UpdateLastLoginAtParams{
			LastLoginAt: sql.NullString{String: nowStr(), Valid: true},
			Username:    user.Username,
		})
		auth.SetSessionCookie(c, token)
		response.OK(c, response.SuccessData(gin.H{"redirectUrl": "/"}))
	}
}

// POST /api/logout — public (works with or without a session).
func logout(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token := auth.TokenFromRequest(c); token != "" {
			_ = deps.Session.Delete(c.Request.Context(), token)
		}
		auth.ClearSessionCookie(c)
		response.OK(c, response.Success())
	}
}
