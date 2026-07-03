// Package httpapi -- security.go: Login history API.
// GET /api/security/login-history — protected. Returns paginated login history.
// recordLoginAttempt() inserts a login attempt row (called from login handler).
package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// loginHistoryResp represents a single login history entry.
type loginHistoryResp struct {
	ID            int64  `json:"id"`
	Username      string `json:"username"`
	IPAddress     string `json:"ip_address"`
	UserAgent     string `json:"user_agent"`
	LoginType     string `json:"login_type"`
	Success       bool   `json:"success"`
	FailureReason string `json:"failure_reason,omitempty"`
	CreatedAt     string `json:"created_at"`
}

// loginHistory — GET /api/security/login-history — protected.
// Returns paginated login history with optional username/success filters.
func loginHistory(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Parse query parameters
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		username := c.Query("username")
		successStr := c.Query("success")

		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 20
		}
		offset := (page - 1) * limit

		// Build query
		query := "SELECT id, username, ip_address, user_agent, login_type, success, failure_reason, created_at FROM login_history WHERE 1=1"
		countQuery := "SELECT COUNT(*) FROM login_history WHERE 1=1"
		args := []interface{}{}

		if username != "" {
			query += " AND username = ?"
			countQuery += " AND username = ?"
			args = append(args, username)
		}
		if successStr != "" {
			query += " AND success = ?"
			countQuery += " AND success = ?"
			args = append(args, successStr == "1")
		}

		// Get total count
		var total int
		if err := deps.Store.Read.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
			response.Fail(c, http.StatusInternalServerError, "查询失败")
			return
		}

		// Get items
		query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
		args = append(args, limit, offset)

		rows, err := deps.Store.Read.QueryContext(ctx, query, args...)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "查询失败")
			return
		}
		defer rows.Close()

		items := []loginHistoryResp{}
		for rows.Next() {
			var item loginHistoryResp
			var success int
			if err := rows.Scan(&item.ID, &item.Username, &item.IPAddress, &item.UserAgent, &item.LoginType, &success, &item.FailureReason, &item.CreatedAt); err != nil {
				continue
			}
			item.Success = success == 1
			items = append(items, item)
		}

		response.OK(c, response.SuccessData(gin.H{
			"items": items,
			"total": total,
			"page":  page,
			"limit": limit,
		}))
	}
}

// recordLoginAttempt inserts a login attempt into the login_history table.
// Errors are silently ignored — login recording is best-effort and must not
// block the authentication flow.
func recordLoginAttempt(deps *Deps, ctx context.Context, username, ip, userAgent, loginType string, success bool, failureReason string) {
	successInt := 0
	if success {
		successInt = 1
	}
	_, _ = deps.Store.Write.ExecContext(ctx,
		`INSERT INTO login_history (username, ip_address, user_agent, login_type, success, failure_reason) VALUES (?, ?, ?, ?, ?, ?)`,
		username, ip, userAgent, loginType, successInt, failureReason)
}
