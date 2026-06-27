package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/auth"
	bcryptpkg "github.com/Muione/oci-start-go/internal/util/bcrypt"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/response"
)

// GET /api/userInfo — protected. Returns the current session username and role.
func userInfo(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		username, _ := auth.UsernameFromContext(c.Request.Context())
		role := ""
		if username != "" {
			u, err := repo.New(deps.Store.Read).FindUserByUsername(c.Request.Context(), username)
			if err == nil {
				role = u.Role
			}
		}
		response.OK(c, response.SuccessData(gin.H{"username": username, "role": role}))
	}
}

// POST /api/change-password — protected. Change password for the logged-in user.
// Requires the current password for verification.
func changePassword(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		username, ok := auth.UsernameFromContext(c.Request.Context())
		if !ok || username == "" {
			response.Fail(c, http.StatusUnauthorized, "not authenticated")
			return
		}

		var in struct {
			CurrentPassword string `json:"currentPassword"`
			NewPassword     string `json:"newPassword"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body")
			return
		}
		if in.CurrentPassword == "" || in.NewPassword == "" {
			response.Fail(c, http.StatusBadRequest, "currentPassword and newPassword are required")
			return
		}
		if len(in.NewPassword) < 6 {
			response.Fail(c, http.StatusBadRequest, "new password must be at least 6 characters")
			return
		}
		if in.CurrentPassword == in.NewPassword {
			response.Fail(c, http.StatusBadRequest, "new password must differ from current password")
			return
		}

		ctx := c.Request.Context()
		q := repo.New(deps.Store.Read)

		// Verify current password.
		user, err := q.FindUserByUsername(ctx, username)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "failed to look up user")
			return
		}
		if err := bcryptpkg.Compare(user.Password, in.CurrentPassword); err != nil {
			response.Fail(c, http.StatusForbidden, "current password is incorrect")
			return
		}

		// Hash and update the new password.
		hashed, err := bcryptpkg.Hash(in.NewPassword)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "failed to hash password")
			return
		}

		w := repo.New(deps.Store.Write)
		if err := w.UpdateUserCredentials(ctx, repo.UpdateUserCredentialsParams{
			Username:   username,
			Password:   hashed,
			Username_2: username,
		}); err != nil {
			response.Fail(c, http.StatusInternalServerError, "failed to update password: "+err.Error())
			return
		}

		response.OK(c, response.SuccessData(gin.H{"message": "password changed successfully"}))
	}
}
