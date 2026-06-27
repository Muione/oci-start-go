# Phase 6 API 文档 — WebSocket & 终端

---

## WebSocket 端点

所有 WebSocket 端点无认证要求 (升级握手独立于 HTTP 中间件链)。

### 1. SSH 终端 `/ws/ssh`

**协议**: JSON 消息帧，双向

**客户端 → 服务端**:
```json
{"type":"connect","data":{"host":"1.2.3.4","port":22,"username":"root","password":"xxx"}}
{"type":"input","data":"ls -la\n"}
{"type":"resize","data":{"cols":80,"rows":24}}
```

**服务端 → 客户端**: 原始终端输出 (文本帧)

**实现**: `golang.org/x/crypto/ssh` 替换 JSch。支持 PTY (xterm-256color)、窗口大小调整、SSH 密码认证。

### 2. 日志 WebSocket `/log/ws`

**协议**: 纯文本帧 (每行日志)

**行为**:
- 连接时: 发送最近 100 行日志 (`tail -n 100`)
- 持续: 每秒轮询文件新行，广播到所有连接的客户端
- 最后一个客户端断开时停止轮询

### 3. 监控广播 `/ws/monitor`

**协议**: JSON 消息帧

**客户端 → 服务端**: `"ping"` → 服务端响应 `{"type":"pong"}`

**服务端 → 客户端** (广播): 当 agent 通过 `POST /api/monitor/report` 上报时:
```json
{"serverId":"...","serverIp":"...","cpuUsage":45.2,"memoryUsage":62.1,...}
```

### 4. 控制台 `/ws/console` (Stub — Phase 7)

**协议**: JSON 控制消息
- `create_connection` / `input` / `disconnect` / `heartbeat` / `ping`
- VNC websockify 桥接延后至 Phase 7

### 5. 救援 `/ws/rescue` (Stub — Phase 7)

**协议**: JSON 控制消息
- `init{instanceId, rescueType}` / `status`
- 完整救援流程延后至 Phase 7

---

## Monitor Agent HTTP API (公开)

### 下载 agent 脚本
```
GET /api/monitor/download
```

### 上报监控数据
```
POST /api/monitor/report
Content-Type: application/json
```
```json
{
  "serverId": "vps-01",
  "serverIp": "1.2.3.4",
  "cpuUsage": 45.2,
  "memoryUsage": 62.1,
  "diskUsage": 33.0,
  "uploadTraffic": 1.5,
  "downloadTraffic": 10.2,
  "cpuCores": 4,
  "totalMemory": 16.0,
  "totalDisk": 100.0,
  "uptimeHours": 720
}
```

---

## 已有接口 (Phase 1-5 保持不变)

参见 [phase5-api.md](phase5-api.md).
