package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// GET /api/disTurnstile?token= — public. Single-use bypass token (current boot).
// Match → disable turnstile.enabled; else error.
func disTurnstile(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tok := c.Query("token")
		if tok == "" || !deps.Bypass.ConsumeAndRotate(tok) {
			response.Fail(c, http.StatusBadRequest, "Token无效、已过期或系统未开启Turnstile验证，禁用失败！")
			return
		}
		if err := deps.SysConf.SetEnabled(c.Request.Context(), "turnstile.enabled", false); err != nil {
			response.Fail(c, http.StatusInternalServerError, "禁用Turnstile失败")
			return
		}
		response.OK(c, response.Success())
	}
}
