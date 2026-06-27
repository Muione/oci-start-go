package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/response"
	bcryptpkg "github.com/Muione/oci-start-go/internal/util/bcrypt"
)

type registerReq struct {
	PreLoginToken string `json:"preLoginToken"`
	Username      string `json:"username"`
	Password      string `json:"password"` // RSA-encrypted base64
}

// POST /api/register-first-user — public. Only allowed when no LOCAL user
// exists yet. Password is RSA-decrypted (plan decision #7).
func registerFirstUser(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req registerReq
		if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" {
			response.Fail(c, http.StatusBadRequest, "请求参数无效")
			return
		}
		ctx := c.Request.Context()
		n, _ := repo.New(deps.Store.Read).CountByLoginType(ctx, "LOCAL")
		if n > 0 {
			response.Fail(c, http.StatusBadRequest, "系统已初始化")
			return
		}
		plain, err := deps.Keypair.Decrypt(req.PreLoginToken, req.Password)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "密码解密失败")
			return
		}
		hash, err := bcryptpkg.Hash(plain)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "密码哈希失败")
			return
		}
		err = repo.New(deps.Store.Write).InsertLoginUser(ctx, repo.InsertLoginUserParams{
			Username:    req.Username,
			Password:    hash,
			IsFirstUser: sql.NullInt64{Int64: 1, Valid: true},
			LoginType:   "LOCAL",
			ExternalID:  sql.NullString{},
			LastLoginAt: sql.NullString{},
			Role:        "USER",
		})
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "注册失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}
