# Backend Support for Frontend Features Design

## 概述

为前端重构后新增的功能提供后端 API 支持，包括登录历史、会话管理、通知测试和历史、SSL 证书状态增强。

## 项目背景

- **技术栈：** Go + Gin + SQLite (modernc.org/sqlite)
- **存储：** WAL 模式，读/写双连接池
- **模块：** `github.com/Muione/oci-start-go`
- **约束：** 不改变现有 API 签名，向后兼容

## 需求分析

### 前端功能需求

| 功能模块 | 需要的 API | 优先级 |
|----------|-----------|--------|
| 用户标签 | 登录统计、会话信息 | 高 |
| SSL 标签 | 证书状态、到期时间 | 高 |
| 安全标签 | 登录历史、会话管理 | 高 |
| 通知标签 | 测试发送、通知历史 | 高 |

### 设计决策

1. **数据存储**：使用 SQLite 数据库存储
2. **登录历史**：记录所有登录尝试（成功和失败）
3. **IP 白名单**：暂不实现
4. **通知历史**：保留最近 1000 条记录
5. **会话管理**：查看所有会话 + 退出指定/所有会话

---

## 数据库设计

### 1. 登录历史表 (login_history)

```sql
CREATE TABLE login_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    user_agent TEXT,
    login_type TEXT NOT NULL,        -- 'password', 'mfa', 'oauth_github', 'oauth_google'
    success INTEGER NOT NULL,        -- 0=失败, 1=成功
    failure_reason TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_login_history_username ON login_history(username);
CREATE INDEX idx_login_history_created_at ON login_history(created_at);
CREATE INDEX idx_login_history_success ON login_history(success);
```

### 2. 会话表 (sessions)

```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,              -- session token hash
    username TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    user_agent TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_active_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL
);

CREATE INDEX idx_sessions_username ON sessions(username);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
```

### 3. 通知历史表 (notification_history)

```sql
CREATE TABLE notification_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel TEXT NOT NULL,            -- 'telegram', 'dingtalk', 'bark', 'feishu'
    message TEXT NOT NULL,
    success INTEGER NOT NULL,
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_notification_history_channel ON notification_history(channel);
CREATE INDEX idx_notification_history_created_at ON notification_history(created_at);
```

---

## API 设计

### 1. 登录历史 API

#### GET /api/security/login-history

获取登录历史记录，支持分页和筛选。

**请求参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认 1 |
| limit | int | 否 | 每页数量，默认 20，最大 100 |
| username | string | 否 | 按用户名筛选 |
| success | int | 否 | 0=失败，1=成功，不传=全部 |
| start_date | string | 否 | 开始日期 (YYYY-MM-DD) |
| end_date | string | 否 | 结束日期 (YYYY-MM-DD) |

**响应：**
```json
{
    "items": [
        {
            "id": 1,
            "username": "admin",
            "ip_address": "192.168.1.100",
            "user_agent": "Mozilla/5.0...",
            "login_type": "password",
            "success": 1,
            "failure_reason": null,
            "created_at": "2026-07-03T10:30:00Z"
        }
    ],
    "total": 150,
    "page": 1,
    "limit": 20
}
```

### 2. 会话管理 API

#### GET /api/security/sessions

获取当前活跃会话列表。

**响应：**
```json
{
    "sessions": [
        {
            "id": "abc123...",
            "username": "admin",
            "ip_address": "192.168.1.100",
            "user_agent": "Mozilla/5.0...",
            "created_at": "2026-07-03T10:30:00Z",
            "last_active_at": "2026-07-03T11:45:00Z",
            "is_current": true
        }
    ]
}
```

#### DELETE /api/security/sessions/:id

删除指定会话（强制登出）。

**响应：**
```json
{
    "message": "Session deleted"
}
```

#### POST /api/security/logout-all

退出所有其他会话（保留当前会话）。

**响应：**
```json
{
    "message": "All other sessions terminated",
    "terminated_count": 3
}
```

### 3. 通知 API

#### POST /system/notification/test

测试发送通知到指定渠道。

**请求体：**
```json
{
    "channel": "telegram"
}
```

**响应：**
```json
{
    "success": true,
    "message": "Test notification sent"
}
```

#### GET /system/notification/history

获取通知发送历史。

**请求参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| channel | string | 否 | 按渠道筛选 |
| limit | int | 否 | 返回数量，默认 50 |

**响应：**
```json
{
    "history": [
        {
            "id": 1,
            "channel": "telegram",
            "message": "Test notification",
            "success": 1,
            "error_message": null,
            "created_at": "2026-07-03T10:30:00Z"
        }
    ]
}
```

### 4. SSL 证书状态增强

#### GET /system/config

增强现有 API，返回 SSL 证书到期时间。

**新增字段：**
```json
{
    "strings": {
        "ssl.domain": "example.com",
        "ssl.email": "admin@example.com",
        "ssl.notAfter": "2026-10-01T00:00:00Z"
    },
    "bools": {
        "ssl.staging": false
    }
}
```

---

## 实现细节

### 1. 登录历史记录

在 `internal/httpapi/auth_login.go` 的 `login()` 函数中添加记录逻辑：

```go
// 记录登录尝试
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

### 2. 会话管理

扩展现有 session 存储，增加 IP 和 User-Agent 字段：

```go
type Session struct {
    ID           string
    Username     string
    IPAddress    string
    UserAgent    string
    CreatedAt    time.Time
    LastActiveAt time.Time
    ExpiresAt    time.Time
}
```

### 3. 通知历史

在发送通知时记录到数据库：

```go
func recordNotification(db *sql.DB, channel, message string, success bool, errMsg string) {
    successInt := 0
    if success {
        successInt = 1
    }
    _, _ = db.Exec(`INSERT INTO notification_history (channel, message, success, error_message) VALUES (?, ?, ?, ?)`,
        channel, message, successInt, errMsg)
}
```

**定时清理任务：**

在 `internal/scheduler/` 中添加定时任务，每天凌晨清理超过 1000 条的旧记录：

```go
// CleanupNotificationHistory 清理超过 1000 条的通知历史记录
func CleanupNotificationHistory(db *sql.DB) error {
    _, err := db.Exec(`DELETE FROM notification_history WHERE id NOT IN (
        SELECT id FROM notification_history ORDER BY id DESC LIMIT 1000
    )`)
    return err
}
```

在 `scheduler.go` 中注册：

```go
// 每天凌晨 3 点清理通知历史
c.AddFunc("0 3 * * *", func() {
    if err := CleanupNotificationHistory(deps.Store.Write); err != nil {
        log.Printf("cleanup notification history: %v", err)
    }
})
```

### 4. SSL 证书状态

在 `systemConfigGet` 中返回 `ssl.notAfter`：

```go
// 已有代码中添加
if notAfter := deps.SysConf.GetString(ctx, "ssl.notAfter"); notAfter != "" {
    strings["ssl.notAfter"] = notAfter
}
```

---

## 文件变更

### 新建文件

| 文件 | 说明 |
|------|------|
| `internal/httpapi/auth_session.go` | 会话管理 API |
| `internal/httpapi/notification.go` | 通知测试和历史 API |
| `internal/db/migrations/006_add_security_tables.sql` | 数据库迁移 |

### 修改文件

| 文件 | 变更内容 |
|------|----------|
| `internal/httpapi/server.go` | 注册新路由 |
| `internal/httpapi/auth_login.go` | 添加登录历史记录 |
| `internal/httpapi/dns.go` | SSL 状态增强 |
| `internal/db/migrate.go` | 添加新迁移 |
| `internal/scheduler/scheduler.go` | 注册清理任务 |

---

## 测试策略

1. **单元测试**
   - 登录历史记录函数
   - 会话管理函数
   - 通知记录函数

2. **集成测试**
   - 完整登录流程 + 历史记录
   - 会话创建/删除/退出
   - 通知发送 + 历史查询

3. **数据库测试**
   - 迁移脚本执行
   - 数据清理逻辑

4. **定时任务测试**
   - 通知历史清理任务

---

## 实施计划

### 阶段 1：数据库迁移（0.5 天）
1. 创建数据库迁移脚本
2. 更新 migrate.go

### 阶段 2：登录历史（1 天）
1. 实现登录历史记录
2. 实现查询 API
3. 编写测试

### 阶段 3：会话管理（1 天）
1. 扩展会话存储
2. 实现会话 API
3. 编写测试

### 阶段 4：通知功能（1 天）
1. 实现通知测试
2. 实现通知历史
3. 添加定时清理任务
4. 编写测试

### 阶段 5：SSL 增强（0.5 天）
1. 增强 system/config API
2. 前端适配

**总计：4 天**
