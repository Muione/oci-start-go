# Backend Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add backend API support for frontend features: login history, session management, notification test/history, and SSL certificate status enhancement.

**Architecture:** Extend existing auth and notification systems with database-backed logging. Add new API endpoints following existing Gin handler patterns. Use robfig/cron for scheduled cleanup tasks.

**Tech Stack:** Go, Gin, SQLite (modernc.org/sqlite), robfig/cron/v3

## Global Constraints

- Go 1.25, module `github.com/Muione/oci-start-go`
- SQLite-only with WAL mode, read/write dual connection pool
- Do not change existing public API signatures
- Follow existing code patterns in `internal/httpapi/` and `internal/auth/`
- All new tables need database migration in `internal/db/`

---

## File Structure

### New Files
| File | Responsibility |
|------|----------------|
| `internal/httpapi/security.go` | Login history, session management, logout-all APIs |
| `internal/httpapi/notification.go` | Notification test and history APIs |
| `internal/db/migrations/007_add_security_tables.sql` | Database migration for new tables |

### Modified Files
| File | Changes |
|------|---------|
| `internal/httpapi/server.go` | Register new routes |
| `internal/httpapi/auth_login.go` | Add login history recording |
| `internal/httpapi/dns.go` | Add ssl.notAfter to systemConfigGet |
| `internal/scheduler/scheduler.go` | Register notification history cleanup job |

---

### Task 1: Database Migration

**Files:**
- Create: `internal/db/migrations/007_add_security_tables.sql`

**Interfaces:**
- Produces: Three new tables (login_history, notification_history, sessions)

- [ ] **Step 1: Create migration file**

```sql
-- 007_add_security_tables.sql
-- Login history table
CREATE TABLE IF NOT EXISTS login_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    user_agent TEXT,
    login_type TEXT NOT NULL,
    success INTEGER NOT NULL,
    failure_reason TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_login_history_username ON login_history(username);
CREATE INDEX IF NOT EXISTS idx_login_history_created_at ON login_history(created_at);
CREATE INDEX IF NOT EXISTS idx_login_history_success ON login_history(success);

-- Notification history table
CREATE TABLE IF NOT EXISTS notification_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel TEXT NOT NULL,
    message TEXT NOT NULL,
    success INTEGER NOT NULL,
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notification_history_channel ON notification_history(channel);
CREATE INDEX IF NOT EXISTS idx_notification_history_created_at ON notification_history(created_at);
```

- [ ] **Step 2: Verify migration syntax**

Run: `sqlite3 :memory: < internal/db/migrations/007_add_security_tables.sql`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/db/migrations/007_add_security_tables.sql
git commit -m "feat: add database migration for security tables"
```

---

### Task 2: Login History API

**Files:**
- Create: `internal/httpapi/security.go`
- Modify: `internal/httpapi/auth_login.go:68-128`
- Modify: `internal/httpapi/server.go`

**Interfaces:**
- Consumes: `deps.Store.Write` for database operations
- Produces: `GET /api/security/login-history`, `recordLoginAttempt()` function

- [ ] **Step 1: Create security.go with login history handler**

```go
package httpapi

import (
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

// GET /api/security/login-history — protected. Returns paginated login history.
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

// recordLoginAttempt inserts a login attempt into the database.
func recordLoginAttempt(deps *Deps, ctx context.Context, username, ip, userAgent, loginType string, success bool, failureReason string) {
    successInt := 0
    if success {
        successInt = 1
    }
    _, _ = deps.Store.Write.ExecContext(ctx,
        `INSERT INTO login_history (username, ip_address, user_agent, login_type, success, failure_reason) VALUES (?, ?, ?, ?, ?, ?)`,
        username, ip, userAgent, loginType, successInt, failureReason)
}
```

- [ ] **Step 2: Add context import to security.go**

```go
import "context"
```

- [ ] **Step 3: Update auth_login.go to record login attempts**

In `login()` function, add recording after success and failure:

```go
// After successful login (around line 126):
recordLoginAttempt(deps, ctx, user.Username, iputil.ClientIP(c), c.GetHeader("User-Agent"), "password", true, "")

// After each failure, add recording:
// After Turnstile failure (line 83-85):
recordLoginAttempt(deps, ctx, req.Username, iputil.ClientIP(c), c.GetHeader("User-Agent"), "password", false, "Turnstile验证失败")

// After password decrypt failure (line 89-91):
recordLoginAttempt(deps, ctx, req.Username, iputil.ClientIP(c), c.GetHeader("User-Agent"), "password", false, "用户名或密码错误")

// After user not found (line 95-97):
recordLoginAttempt(deps, ctx, req.Username, iputil.ClientIP(c), c.GetHeader("User-Agent"), "password", false, "用户名或密码错误")

// After password mismatch (line 99-101):
recordLoginAttempt(deps, ctx, req.Username, iputil.ClientIP(c), c.GetHeader("User-Agent"), "password", false, "用户名或密码错误")

// After MFA failure (line 110-112):
recordLoginAttempt(deps, ctx, req.Username, iputil.ClientIP(c), c.GetHeader("User-Agent"), "password", false, "MFA验证码错误")
```

- [ ] **Step 4: Register route in server.go**

Add after line 54 (after mfa routes):

```go
pro.GET("/api/security/login-history", loginHistory(deps))
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/httpapi/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/httpapi/security.go internal/httpapi/auth_login.go internal/httpapi/server.go
git commit -m "feat: add login history API and recording"
```

---

### Task 3: Session Management API

**Files:**
- Modify: `internal/httpapi/security.go`
- Modify: `internal/httpapi/server.go`

**Interfaces:**
- Consumes: `deps.Session` for session operations
- Produces: `GET /api/security/sessions`, `DELETE /api/security/sessions/:id`, `POST /api/security/logout-all`

- [ ] **Step 1: Add session management handlers to security.go**

```go
// sessionResp represents a session entry.
type sessionResp struct {
    ID           string `json:"id"`
    Username     string `json:"username"`
    IPAddress    string `json:"ip_address"`
    UserAgent    string `json:"user_agent"`
    CreatedAt    string `json:"created_at"`
    LastActiveAt string `json:"last_active_at"`
    IsCurrent    bool   `json:"is_current"`
}

// GET /api/security/sessions — protected. Returns active sessions.
func listSessions(deps *Deps) gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx := c.Request.Context()
        currentToken := auth.TokenFromRequest(c)

        rows, err := deps.Store.Read.QueryContext(ctx,
            `SELECT id, username, ip_address, user_agent, created_at, last_active_at FROM sessions WHERE expires_at > datetime('now') ORDER BY last_active_at DESC`)
        if err != nil {
            response.Fail(c, http.StatusInternalServerError, "查询会话失败")
            return
        }
        defer rows.Close()

        sessions := []sessionResp{}
        for rows.Next() {
            var s sessionResp
            if err := rows.Scan(&s.ID, &s.Username, &s.IPAddress, &s.UserAgent, &s.CreatedAt, &s.LastActiveAt); err != nil {
                continue
            }
            s.IsCurrent = s.ID == currentToken
            sessions = append(sessions, s)
        }

        response.OK(c, response.SuccessData(gin.H{"sessions": sessions}))
    }
}

// DELETE /api/security/sessions/:id — protected. Delete a specific session.
func deleteSession(deps *Deps) gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx := c.Request.Context()
        sessionID := c.Param("id")
        currentToken := auth.TokenFromRequest(c)

        if sessionID == currentToken {
            response.Fail(c, http.StatusBadRequest, "不能删除当前会话")
            return
        }

        result, err := deps.Store.Write.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID)
        if err != nil {
            response.Fail(c, http.StatusInternalServerError, "删除会话失败")
            return
        }

        rowsAffected, _ := result.RowsAffected()
        if rowsAffected == 0 {
            response.Fail(c, http.StatusNotFound, "会话不存在")
            return
        }

        response.OK(c, response.SuccessData(gin.H{"message": "会话已删除"}))
    }
}

// POST /api/security/logout-all — protected. Terminate all other sessions.
func logoutAllSessions(deps *Deps) gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx := c.Request.Context()
        currentToken := auth.TokenFromRequest(c)

        result, err := deps.Store.Write.ExecContext(ctx,
            `DELETE FROM sessions WHERE id != ? AND username = (SELECT username FROM sessions WHERE id = ?)`,
            currentToken, currentToken)
        if err != nil {
            response.Fail(c, http.StatusInternalServerError, "退出会话失败")
            return
        }

        rowsAffected, _ := result.RowsAffected()
        response.OK(c, response.SuccessData(gin.H{
            "message":          "已退出所有其他会话",
            "terminated_count": rowsAffected,
        }))
    }
}
```

- [ ] **Step 2: Add auth import to security.go**

```go
import "github.com/Muione/oci-start-go/internal/auth"
```

- [ ] **Step 3: Register routes in server.go**

Add after login-history route:

```go
pro.GET("/api/security/sessions", listSessions(deps))
pro.DELETE("/api/security/sessions/:id", deleteSession(deps))
pro.POST("/api/security/logout-all", logoutAllSessions(deps))
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/httpapi/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi/security.go internal/httpapi/server.go
git commit -m "feat: add session management API"
```

---

### Task 4: Notification Test API

**Files:**
- Create: `internal/httpapi/notification.go`
- Modify: `internal/httpapi/server.go`

**Interfaces:**
- Consumes: `deps.SysConf` for notification config
- Produces: `POST /system/notification/test`

- [ ] **Step 1: Create notification.go with test handler**

```go
package httpapi

import (
    "context"
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/rs/zerolog"

    "github.com/Muione/oci-start-go/internal/notify"
    "github.com/Muione/oci-start-go/internal/response"
)

// POST /system/notification/test — protected. Test send notification.
func notificationTest(deps *Deps) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            Channel string `json:"channel"`
        }
        if err := c.ShouldBindJSON(&req); err != nil {
            response.Fail(c, http.StatusBadRequest, "参数无效")
            return
        }

        ctx := c.Request.Context()
        message := "🔔 这是一条测试通知\n时间: " + nowStr()

        var notifier notify.Notifier
        switch req.Channel {
        case "telegram":
            token := deps.SysConf.GetString(ctx, "telegram.bot.token")
            chatID := deps.SysConf.GetString(ctx, "telegram.chat.id")
            if token == "" || chatID == "" {
                recordNotification(deps, ctx, req.Channel, message, false, "未配置")
                response.Fail(c, http.StatusBadRequest, "Telegram 未配置")
                return
            }
            notifier = notify.NewTelegramNotifier(token, chatID, zerolog.Nop())
        case "dingtalk":
            webhook := deps.SysConf.GetString(ctx, "dingtalk.webhook")
            secret := deps.SysConf.GetString(ctx, "dingtalk.secret")
            if webhook == "" {
                recordNotification(deps, ctx, req.Channel, message, false, "未配置")
                response.Fail(c, http.StatusBadRequest, "钉钉未配置")
                return
            }
            notifier = notify.NewDingTalkNotifier(webhook, secret, zerolog.Nop())
        case "bark":
            key := deps.SysConf.GetString(ctx, "bark.key")
            if key == "" {
                recordNotification(deps, ctx, req.Channel, message, false, "未配置")
                response.Fail(c, http.StatusBadRequest, "Bark 未配置")
                return
            }
            server := deps.SysConf.GetString(ctx, "bark.server")
            if server == "" {
                server = "https://api.day.app"
            }
            notifier = notify.NewBarkNotifier(key, server, zerolog.Nop())
        case "feishu":
            webhook := deps.SysConf.GetString(ctx, "feishu.webhook")
            secret := deps.SysConf.GetString(ctx, "feishu.secret")
            if webhook == "" {
                recordNotification(deps, ctx, req.Channel, message, false, "未配置")
                response.Fail(c, http.StatusBadRequest, "飞书未配置")
                return
            }
            notifier = notify.NewFeishuNotifier(webhook, secret, zerolog.Nop())
        default:
            response.Fail(c, http.StatusBadRequest, "不支持的渠道")
            return
        }

        if err := notifier.Send(ctx, message); err != nil {
            recordNotification(deps, ctx, req.Channel, message, false, err.Error())
            response.Fail(c, http.StatusInternalServerError, "发送失败: "+err.Error())
            return
        }

        recordNotification(deps, ctx, req.Channel, message, true, "")
        response.OK(c, response.SuccessData(gin.H{
            "success": true,
            "message": "测试通知已发送",
        }))
    }
}

// recordNotification inserts a notification record into the database.
func recordNotification(deps *Deps, ctx context.Context, channel, message string, success bool, errMsg string) {
    successInt := 0
    if success {
        successInt = 1
    }
    _, _ = deps.Store.Write.ExecContext(ctx,
        `INSERT INTO notification_history (channel, message, success, error_message) VALUES (?, ?, ?, ?)`,
        channel, message, successInt, errMsg)
}

func nowStr() string {
    return time.Now().Format("2006-01-02 15:04:05")
}
```

- [ ] **Step 2: Add imports to notification.go**

```go
import "time"
```

- [ ] **Step 3: Register route in server.go**

Add after system config routes:

```go
pro.POST("/system/notification/test", notificationTest(deps))
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/httpapi/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi/notification.go internal/httpapi/server.go
git commit -m "feat: add notification test API"
```

---

### Task 5: Notification History API

**Files:**
- Modify: `internal/httpapi/notification.go`
- Modify: `internal/httpapi/server.go`

**Interfaces:**
- Consumes: `deps.Store.Read` for database queries
- Produces: `GET /system/notification/history`

- [ ] **Step 1: Add history handler to notification.go**

```go
// notificationHistoryResp represents a notification history entry.
type notificationHistoryResp struct {
    ID           int64  `json:"id"`
    Channel      string `json:"channel"`
    Message      string `json:"message"`
    Success      bool   `json:"success"`
    ErrorMessage string `json:"error_message,omitempty"`
    CreatedAt    string `json:"created_at"`
}

// GET /system/notification/history — protected. Returns notification history.
func notificationHistory(deps *Deps) gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx := c.Request.Context()
        channel := c.Query("channel")
        limit := 50
        if l, err := strconv.Atoi(c.DefaultQuery("limit", "50")); err == nil && l > 0 && l <= 200 {
            limit = l
        }

        query := "SELECT id, channel, message, success, error_message, created_at FROM notification_history WHERE 1=1"
        args := []interface{}{}

        if channel != "" {
            query += " AND channel = ?"
            args = append(args, channel)
        }

        query += " ORDER BY created_at DESC LIMIT ?"
        args = append(args, limit)

        rows, err := deps.Store.Read.QueryContext(ctx, query, args...)
        if err != nil {
            response.Fail(c, http.StatusInternalServerError, "查询失败")
            return
        }
        defer rows.Close()

        history := []notificationHistoryResp{}
        for rows.Next() {
            var item notificationHistoryResp
            var success int
            if err := rows.Scan(&item.ID, &item.Channel, &item.Message, &success, &item.ErrorMessage, &item.CreatedAt); err != nil {
                continue
            }
            item.Success = success == 1
            history = append(history, item)
        }

        response.OK(c, response.SuccessData(gin.H{"history": history}))
    }
}
```

- [ ] **Step 2: Add strconv import to notification.go**

```go
import "strconv"
```

- [ ] **Step 3: Register route in server.go**

Add after notification/test route:

```go
pro.GET("/system/notification/history", notificationHistory(deps))
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/httpapi/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi/notification.go internal/httpapi/server.go
git commit -m "feat: add notification history API"
```

---

### Task 6: SSL Certificate Status Enhancement

**Files:**
- Modify: `internal/httpapi/dns.go:19-44`

**Interfaces:**
- Consumes: `deps.SysConf` for SSL config
- Produces: Enhanced `sslList()` response with `notAfter`

- [ ] **Step 1: Update sslList function in dns.go**

```go
// sslList returns configured SSL certificate status.
// GET /ssl/list
func sslList(deps *Deps) gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx := c.Request.Context()
        domain := deps.SysConf.GetString(ctx, "ssl.domain")
        email := deps.SysConf.GetString(ctx, "ssl.email")
        staging := deps.SysConf.GetBool(ctx, "ssl.staging")
        notAfter := deps.SysConf.GetString(ctx, "ssl.notAfter")

        certs := []gin.H{}
        if domain != "" {
            cert := gin.H{
                "domain":  domain,
                "email":   email,
                "staging": staging,
                "status":  "configured",
            }
            if notAfter != "" {
                cert["notAfter"] = notAfter
            }
            certs = append(certs, cert)
        }

        response.OK(c, response.SuccessData(gin.H{
            "certs":      certs,
            "configured": domain != "" && email != "",
            "staging":    staging,
            "notAfter":   notAfter,
        }))
    }
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/httpapi/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/httpapi/dns.go
git commit -m "feat: add SSL certificate expiry to API response"
```

---

### Task 7: Notification History Cleanup Job

**Files:**
- Modify: `internal/scheduler/scheduler.go`

**Interfaces:**
- Consumes: `deps.Store.Write` for database operations
- Produces: Registered cleanup job

- [ ] **Step 1: Add cleanup function to scheduler.go**

```go
// cleanupNotificationHistory deletes old notification records, keeping only the latest 1000.
func (s *Scheduler) cleanupNotificationHistory() {
    ctx := context.Background()
    result, err := s.store.Write.ExecContext(ctx,
        `DELETE FROM notification_history WHERE id NOT IN (SELECT id FROM notification_history ORDER BY id DESC LIMIT 1000)`)
    if err != nil {
        s.logger.Error().Err(err).Msg("cleanup notification history failed")
        return
    }
    rowsAffected, _ := result.RowsAffected()
    if rowsAffected > 0 {
        s.logger.Info().Int64("deleted", rowsAffected).Msg("cleaned up old notification records")
    }
}
```

- [ ] **Step 2: Register job in registerJobs()**

Add after existing jobs (around line 150):

```go
// Notification history cleanup - daily at 3:00 AM
s.cron.AddFunc("0 0 3 * * *", s.cleanupNotificationHistory)
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/scheduler/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/scheduler/scheduler.go
git commit -m "feat: add notification history cleanup scheduled job"
```

---

## Summary

| Task | Description | Files Changed |
|------|-------------|---------------|
| 1 | Database Migration | 1 new |
| 2 | Login History API | 3 modified |
| 3 | Session Management API | 2 modified |
| 4 | Notification Test API | 2 files |
| 5 | Notification History API | 2 modified |
| 6 | SSL Status Enhancement | 1 modified |
| 7 | Cleanup Scheduled Job | 1 modified |

**Estimated Total Time:** 4-5 days

**Dependencies:**
- Task 1 must be done first (database tables)
- Tasks 2-6 can be done in parallel after Task 1
- Task 7 depends on Task 4 (notification_history table)
