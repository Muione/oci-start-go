package httpapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/util/totp"
)

// ---- TOTP setup cache (5min TTL, single-user) ----

type totpSetupEntry struct {
	Secret    string
	ExpireAt  time.Time
}

type totpSetupCache struct {
	m    sync.Map
	stop chan struct{}
	once sync.Once
}

func NewTotpSetupCache() *totpSetupCache {
	c := &totpSetupCache{stop: make(chan struct{})}
	go c.sweep()
	return c
}

func (c *totpSetupCache) sweep() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			now := time.Now()
			c.m.Range(func(k, v any) bool {
				if e, ok := v.(*totpSetupEntry); ok && now.After(e.ExpireAt) {
					c.m.Delete(k)
				}
				return true
			})
		case <-c.stop:
			return
		}
	}
}

func (c *totpSetupCache) Stop() { c.once.Do(func() { close(c.stop) }) }

func (c *totpSetupCache) Put(secret string) {
	c.m.Store("setup", &totpSetupEntry{Secret: secret, ExpireAt: time.Now().Add(5 * time.Minute)})
}

func (c *totpSetupCache) Take() (string, bool) {
	v, ok := c.m.LoadAndDelete("setup")
	if !ok {
		return "", false
	}
	e := v.(*totpSetupEntry)
	if time.Now().After(e.ExpireAt) {
		return "", false
	}
	return e.Secret, true
}

// ---- Handlers ----

// GET /api/mfa/status — returns current MFA status for the system.
func mfaStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		enabled := deps.SysConf.GetBool(ctx, "mfa.enabled")
		configured := deps.SysConf.GetString(ctx, "mfa.secret.key") != ""
		response.OK(c, response.SuccessData(gin.H{
			"enabled":   enabled,
			"configured": configured,
		}))
	}
}

// POST /api/mfa/totp/setup — generates a new TOTP secret, stores it
// temporarily, and returns the QR code for scanning.
func mfaTotpSetup(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		secret, otpURL, err := totp.GenerateSecret("oci-start", "admin")
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "生成TOTP密钥失败")
			return
		}
		qrBase64, err := totp.QRCodeBase64(otpURL)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "生成二维码失败")
			return
		}
		deps.TotpSetup.Put(secret)
		response.OK(c, response.SuccessData(gin.H{
			"secret":       secret,
			"otpauthUrl":   otpURL,
			"qrCodeBase64": qrBase64,
		}))
	}
}

// POST /api/mfa/totp/verify — verifies a TOTP code against the temporarily
// stored secret. On success, persists the secret and enables MFA.
func mfaTotpVerify(deps *Deps) gin.HandlerFunc {
	type req struct {
		Code string `json:"code" binding:"required"`
	}
	return func(c *gin.Context) {
		var body req
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "请输入验证码")
			return
		}
		secret, ok := deps.TotpSetup.Take()
		if !ok {
			response.Fail(c, http.StatusBadRequest, "设置已过期，请重新生成")
			return
		}
		if !totp.Validate(secret, body.Code) {
			response.Fail(c, http.StatusBadRequest, "验证码错误")
			return
		}
		ctx := c.Request.Context()
		deps.SysConf.SetString(ctx, "mfa.secret.key", secret)
		deps.SysConf.SetEnabled(ctx, "mfa.enabled", true)
		response.OK(c, response.SuccessData(gin.H{"enabled": true}))
	}
}

// POST /api/mfa/disable — disables MFA and clears the stored secret.
func mfaDisable(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		deps.SysConf.SetEnabled(ctx, "mfa.enabled", false)
		deps.SysConf.SetString(ctx, "mfa.secret.key", "")
		response.OK(c, response.SuccessData(gin.H{"enabled": false}))
	}
}
