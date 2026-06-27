# Phase 7 API 文档 — 通知 / DNS / SSL / 系统设置

---

## 1. 通知系统

Telegram Bot API 通知已集成到所有 Phase 4-6 的 stub 中：

| 事件 | 通知内容 | 触发位置 |
|---|---|---|
| 抢机成功 | 任务ID + 实例ID + 公网IP | `grabber/success.go` |
| 抢机失败 | 任务ID + 用户 + 区域 + 错误 | `grabber/failure.go` |
| 流量超限 | 实例名 + IP + 流量(GB) | `service/traffic.go` |
| 账号失效 | 租户 + 区域 + 原因 | `service/check_live.go` |
| 实例离线 | 实例名 + IP | `service/offline.go` |

**通知配置**: SystemConfig KV:
- `telegram.bot.token` — Bot API token
- `telegram.chat.id` — Target chat ID

---

## 2. DNS 管理

### 2.1 DNS 记录列表
```
GET /dns/list
```

### 2.2 从 Cloudflare 同步
```
POST /dns/sync  {"zoneId": "..."}
```
前置: 需配置 `cloudflare.email` + `cloudflare.api.key`

---

## 3. SSL 证书

### 3.1 证书列表
```
GET /ssl/list
```

### 3.2 签发证书 (Let's Encrypt DNS-01)
```
POST /ssl/issue  {"domain": "example.com", "email": "admin@example.com"}
```

---

## 4. 系统设置
```
GET /system/config
```

---

## 5. 已有接口 (Phase 1-6 保持不变)

参见 [phase6-api.md](phase6-api.md).
