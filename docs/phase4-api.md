# Phase 4 API 文档 — 抢机引擎

> 所有接口统一返回 `ApiResponse` 信封: `{success: bool, message: string, data: any, code: int}`.
> 除非特别标注，所有接口均需登录 (Cookie `satoken`).

---

## 1. 抢机任务 CRUD

### 1.1 获取任务列表
```
GET /boot/list
```

**Response data**: `BootTask[]`
```json
[{
  "id": 1,
  "bootId": "uuid-string",
  "tenantId": 1,
  "ocpu": 4,
  "memory": 24,
  "disk": 100,
  "loopTime": 6,
  "instanceCount": 1,
  "status": 1,
  "architecture": "ARM",
  "rootPassword": "",
  "publicIp": "",
  "imageId": "",
  "operatingSystem": "Canonical Ubuntu",
  "operatingSystemVersion": "22.04",
  "dataGap": "",
  "notifyFlag": "NO",
  "nextExecutionTime": "2026-06-27 23:59:00",
  "failCount": 0,
  "successCount": 3,
  "totalCount": 0,
  "remark": "",
  "cloudType": 1,
  "createdAt": "2026-06-27 23:00:00",
  "updatedAt": "2026-06-27 23:55:00"
}]
```

**字段说明**:
| 字段 | 类型 | 说明 |
|---|---|---|
| status | int | 0=已停用, 1=运行中, 2=已完成(抢到) |
| architecture | string | ARM 或 AMD |
| loopTime | int | 循环间隔(秒)，默认6 |
| dataGap | string | 时间窗口 "HH:MM-HH:MM"，空=全天 |
| notifyFlag | string | 通知标记，NO=未通知, YES=已通知 |
| cloudType | int | 1=Oracle Cloud, 2=GCP |

### 1.2 新建/更新任务
```
POST /boot/save
Content-Type: application/json
```

**Request body** (`BootSaveInput`):
```json
{
  "bootId": "",
  "tenantId": 1,
  "ocpu": 4,
  "memory": 24,
  "disk": 100,
  "loopTime": 6,
  "instanceCount": 1,
  "architecture": "ARM",
  "rootPassword": "",
  "imageId": "",
  "operatingSystem": "Canonical Ubuntu",
  "operatingSystemVersion": "22.04",
  "dataGap": "00:00-23:59",
  "notifyFlag": "NO",
  "remark": "",
  "cloudType": 1
}
```

- `bootId` 为空 → 新建 (自动生成 UUID)
- `bootId` 非空 → 更新已有任务
- 必填字段: `tenantId`, `ocpu`, `memory`, `disk` (>0)

### 1.3 删除任务 (软删除, status=0)
```
GET /boot/delete?bootId=<uuid>
```

### 1.4 启用/暂停任务
```
GET /boot/toggle?bootId=<uuid>&enable=1|0
```

- `enable=1`: 启用 (status=1, nextExecutionTime 重置)
- `enable=0`: 暂停 (status=0)

---

## 2. 引擎状态

### 2.1 获取引擎状态
```
GET /boot/systemStatus
```

**Response data**:
```json
{
  "totalTasks": 10,
  "runningTasks": 3,
  "activeKeyCount": 2,
  "parentPool": {
    "active": 1,
    "queue": 0,
    "size": 4,
    "completed": 150
  },
  "apiPool": {
    "active": 2,
    "queue": 0,
    "size": 8,
    "completed": 120
  },
  "batchSize": 50,
  "running": true
}
```

**字段说明**:
| 字段 | 说明 |
|---|---|
| totalTasks | boot_instance 表总行数 |
| runningTasks | status=1 的任务数 |
| activeKeyCount | 当前单飞锁定的 key 数 (正在执行的抢机) |
| parentPool.active | 父池当前活跃 goroutine 数 |
| parentPool.size | 父池最大容量 |
| apiPool.active | API 池当前活跃 goroutine 数 |
| apiPool.completed | API 池历史完成数 |
| running | 引擎运行标志 |

### 2.2 获取租户列表 (供下拉框)
```
GET /boot/tenants
```

**Response data**: `[{id, name, region, tenancy}]`
```json
[{
  "id": 1,
  "name": "user1",
  "region": "东京",
  "tenancy": "ocid1.tenancy.oc1..."
}]
```

---

## 3. 调度器 (Cron Jobs)

| Job | Cron | 说明 |
|---|---|---|
| CreateInstanceJob | `*/6 * * * * *` | 每6秒触发抢机调度 |
| InstanceTrafficJob | `0 */5 * * * *` | Phase 5 stub |
| CheckLiveJob | `0 0 * * * *` | Phase 5 stub |
| PingConnTimeJob | `0 */5 * * * *` | Phase 5 stub |
| SslCertJob | `0 0 4 * * *` | Phase 7 stub |
| BootInstanceRefreshJob | `0 0 0 * * *` | 每日00:00重置日计数 |
| MonitorFlashHeartbeatJob | `*/15 * * * * *` | Phase 6 stub |
| CheckOfflineInstanceJob | `0 */1 * * * *` | Phase 5 stub |
| MultipartUploadCleanupJob | `0 0 2 * * *` | Phase 5 stub |

---

## 4. 已有接口 (Phase 1-3 保持不变)

| 方法 | 路径 | 说明 | 认证 |
|---|---|---|---|
| GET | `/healthz` | 健康检查 | 无 |
| GET | `/api/version` | 版本信息 | 需登录 |
| GET | `/api/login/init` | RSA 预登录 | 无 |
| POST | `/api/login` | 登录 | 无 |
| POST | `/api/logout` | 登出 | 需登录 |
| GET | `/api/userInfo` | 当前用户 | 需登录 |
| GET/POST | 租户/代理 CRUD | 见 Phase 3 | 需登录 |
| GET | `/api/stats` | 仪表盘统计 | 需登录 |
