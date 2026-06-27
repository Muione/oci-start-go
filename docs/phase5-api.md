# Phase 5 API 文档 — 网络 / 流量 / 备份 / 实例管理

> 所有接口统一返回 `ApiResponse` 信封: `{success, message, data, code}`.
> 除非特别标注，所有接口均需登录 (Cookie `satoken`).

---

## 1. 实例管理

### 1.1 实例列表 (分页)
```
GET /instances/list?limit=20&offset=0
```
**Response data**: `{items: InstanceDetailResp[], total: int64}`

### 1.2 实例详情
```
GET /instances/:id
```
**Response data**: `InstanceDetailResp`

### 1.3 更新备注
```
POST /instances/:id/remark
{"remark": "my note"}
```

### 1.4 实例流量统计 (按租户)
```
GET /instances/traffic?tenantId=1
```
**Response data**: `TenantTrafficStats` (月出站流量、每实例明细)

---

## 2. 备份管理

### 2.1 备份列表
```
GET /backup/list?tenantId=1
```
**Response data**: `BackupListResp[]`

### 2.2 删除备份记录
```
GET /backup/delete?id=1
```

---

## 3. 流量告警配置

### 3.1 告警列表
```
GET /traffic/alert/list
```

### 3.2 获取单条告警
```
GET /traffic/alert/get?tenantId=1
```

### 3.3 保存告警配置
```
POST /traffic/alert/save
{
  "tenantId": 1,
  "threshold": 10240,
  "autoShutdown": false,
  "enabled": true,
  "statisticsEnabled": true
}
```

---

## 4. 调度器 Jobs (Phase 5 实现)

| Job | Cron | 实现状态 |
|---|---|---|
| CreateInstanceJob | `*/6 * * * * *` | Phase 4 (完整) |
| InstanceTrafficJob | `0 */30 * * * *` | Phase 5 (完整 — 实时 OCI Monitoring 查询) |
| CheckLiveJob | `0 0 * * * *` | Phase 5 (完整 — 账号存活检查) |
| PingConnTimeJob | `0 */5 * * * *` | Phase 5 (完整 — TCP 22 连通性) |
| BootInstanceRefreshJob | `0 0 0 * * *` | Phase 4 (完整) |
| CheckOfflineInstanceJob | `0 */1 * * * *` | Phase 5 (完整 — 心跳超时检测) |

---

## 5. 已有接口 (Phase 1-4 保持不变)

参见 [phase4-api.md](phase4-api.md) 第4节.
