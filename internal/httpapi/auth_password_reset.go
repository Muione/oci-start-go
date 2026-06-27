package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	bcryptpkg "github.com/Muione/oci-start-go/internal/util/bcrypt"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/response"
)

// sendResetCode generates a one-time password reset code for the given username
// and stores it temporarily. In production, the code would be sent via email/SMS.
// POST /api/send-reset-code  {username: "..."}
func sendResetCode(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Username string `json:"username"`
		}
		if err := c.ShouldBindJSON(&in); err != nil || in.Username == "" {
			response.Fail(c, http.StatusBadRequest, "username required")
			return
		}

		ctx := c.Request.Context()

		// Verify the user exists.
		_, err := repo.New(deps.Store.Read).FindUserByUsername(ctx, in.Username)
		if err != nil {
			// Don't reveal whether user exists.
			response.OK(c, response.SuccessData(gin.H{"message": "if the account exists, a reset code has been generated"}))
			return
		}

		// Generate a 6-character hex code.
		b := make([]byte, 3)
		rand.Read(b)
		code := hex.EncodeToString(b)

		// Store code with 10-minute expiry.
		now := time.Now().Format("2006-01-02 15:04:05")
		expiry := time.Now().Add(10 * time.Minute).Format("2006-01-02 15:04:05")
		codeKey := fmt.Sprintf("reset.code.%s", in.Username)

		q := repo.New(deps.Store.Write)
		_ = q.UpsertConfigValue(ctx, repo.UpsertConfigValueParams{
			ConfigKey:    nullStr(codeKey),
			ConfigValue:  nullStr(code + "|" + expiry),
			ConfigEnabled: nullInt64(0),
			LastModified: nullStr(now),
		})

		response.OK(c, response.SuccessData(gin.H{
			"message": "if the account exists, a reset code has been generated",
			"code":    code, // In production, this is sent via email/SMS, not returned.
		}))
	}
}

// verifyResetCode validates a reset code.
// POST /api/verify-reset-code  {username: "...", code: "..."}
func verifyResetCode(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Username string `json:"username"`
			Code     string `json:"code"`
		}
		if err := c.ShouldBindJSON(&in); err != nil || in.Username == "" || in.Code == "" {
			response.Fail(c, http.StatusBadRequest, "username and code required")
			return
		}

		ctx := c.Request.Context()
		codeKey := fmt.Sprintf("reset.code.%s", in.Username)
		stored := deps.SysConf.GetString(ctx, codeKey)

		if stored == "" {
			response.Fail(c, http.StatusBadRequest, "no reset code found or code expired")
			return
		}

		// Parse "code|expiry".
		var storedCode, expiry string
		if _, err := fmt.Sscanf(stored, "%6s|%s", &storedCode, &expiry); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid stored code format")
			return
		}

		// Check expiry.
		expiryTime, err := time.Parse("2006-01-02 15:04:05", expiry)
		if err != nil || time.Now().After(expiryTime) {
			response.Fail(c, http.StatusBadRequest, "reset code has expired (10-minute window)")
			return
		}

		if storedCode != in.Code {
			response.Fail(c, http.StatusBadRequest, "invalid reset code")
			return
		}

		response.OK(c, response.SuccessData(gin.H{"message": "code verified", "valid": true}))
	}
}

// resetPassword resets the user's password after code verification.
// POST /api/reset-password  {username: "...", code: "...", newPassword: "..."}
func resetPassword(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Username    string `json:"username"`
			Code        string `json:"code"`
			NewPassword string `json:"newPassword"`
		}
		if err := c.ShouldBindJSON(&in); err != nil || in.Username == "" || in.Code == "" || in.NewPassword == "" {
			response.Fail(c, http.StatusBadRequest, "username, code, and newPassword required")
			return
		}

		if len(in.NewPassword) < 6 {
			response.Fail(c, http.StatusBadRequest, "password must be at least 6 characters")
			return
		}

		ctx := c.Request.Context()

		// Verify the code first.
		codeKey := fmt.Sprintf("reset.code.%s", in.Username)
		stored := deps.SysConf.GetString(ctx, codeKey)
		if stored == "" {
			response.Fail(c, http.StatusBadRequest, "no reset code found or code expired")
			return
		}

		var storedCode, expiry string
		if _, err := fmt.Sscanf(stored, "%6s|%s", &storedCode, &expiry); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid stored code format")
			return
		}

		expiryTime, err := time.Parse("2006-01-02 15:04:05", expiry)
		if err != nil || time.Now().After(expiryTime) {
			response.Fail(c, http.StatusBadRequest, "reset code has expired")
			return
		}

		if storedCode != in.Code {
			response.Fail(c, http.StatusBadRequest, "invalid reset code")
			return
		}

		// Hash the new password and update.
		hashed, err := hashPassword(in.NewPassword)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "failed to hash password")
			return
		}

		q := repo.New(deps.Store.Write)
		if err := q.UpdateUserCredentials(ctx, repo.UpdateUserCredentialsParams{
			Username: in.Username,
			Password: hashed,
		}); err != nil {
			response.Fail(c, http.StatusInternalServerError, "failed to update password: "+err.Error())
			return
		}

		// Clean up the reset code.
		_ = q.UpsertConfigValue(ctx, repo.UpsertConfigValueParams{
			ConfigKey:    nullStr(codeKey),
			ConfigValue:  nullStr(""),
			ConfigEnabled: nullInt64(0),
			LastModified: nullStr(time.Now().Format("2006-01-02 15:04:05")),
		})

		response.OK(c, response.SuccessData(gin.H{"message": "password reset successfully"}))
	}
}

// hashPassword creates a bcrypt hash of the password (cost 10).
func hashPassword(password string) (string, error) {
	return bcryptpkg.Hash(password)
}
