# Phase 6 测试方案 — WebSocket & 终端

---

## 1. 编译验证
| # | 测试项 | 预期 |
|---|---|---|
| 1.1 | `go build ./...` | 无错误 |
| 1.2 | `go build -tags dist -o oci-start ./cmd/oci-start/` | 生成可执行文件 |
| 1.3 | `cd frontend && npm run build` | 生成 dist/ |

## 2. WebSocket 端点
| # | 端点 | 测试方法 | 预期 |
|---|---|---|---|
| 2.1 | `/ws/ssh` | wscat 或浏览器连接 | 升级到 WS |
| 2.2 | `/ws/ssh` | 发送 `{"type":"connect","data":{...}}` 错误凭证 | 返回 "SSH conn error: ..." |
| 2.3 | `/ws/ssh` | 发送 `{"type":"connect","data":{...}}` 正确凭证 | 返回 "SSH conn success"，终端交互 |
| 2.4 | `/ws/ssh` | 发送 `{"type":"input","data":"ls\n"}` | SSH session 执行 ls |
| 2.5 | `/ws/ssh` | 发送 `{"type":"resize","data":{"cols":120,"rows":40}}` | 终端窗口调整 |
| 2.6 | `/log/ws` | 连接 | 收到最近 100 行日志 |
| 2.7 | `/log/ws` | 写入新日志行 | 收到新行 |
| 2.8 | `/ws/monitor` | 连接 | 收到欢迎消息 |
| 2.9 | `/ws/monitor` | POST `/api/monitor/report` | 所有连接收到广播 |
| 2.10 | `/ws/monitor` | 发送 "ping" | 收到 "pong" |
| 2.11 | `/ws/console` | 连接 + create_connection | 收到 stub 响应 |
| 2.12 | `/ws/rescue` | 连接 + init | 收到 stub 响应 |

## 3. Monitor Agent API
| # | 方法 | 路径 | 预期 |
|---|---|---|---|
| 3.1 | GET | `/api/monitor/download` | 返回 agent 脚本 (text/plain) |
| 3.2 | POST | `/api/monitor/report` (有效 JSON) | 200, 广播到 WS 客户端 |
| 3.3 | POST | `/api/monitor/report` (无效 JSON) | 400 |

## 4. 前端页面
| # | 页面 | 测试项 | 预期 |
|---|---|---|---|
| 4.1 | `/terminal` | 页面加载 | 显示连接表单 + 终端容器 |
| 4.2 | `/terminal` | 填写主机/端口/用户名/密码，点击连接 | 建立 WS，终端显示 SSH 输出 |
| 4.3 | `/terminal` | 终端输入 | 字符发送到 SSH |
| 4.4 | `/terminal` | 调整终端窗口 | 发送 resize 消息 |
| 4.5 | `/terminal` | 点击断开 | WS 关闭，终端显示 [Disconnected] |
| 4.6 | 侧边栏 | "SSH终端"菜单项 | 路由到 /terminal |

## 5. 已知限制
| 功能 | 状态 |
|---|---|
| AI Chat WS | 跳过 (用户要求) |
| VNC/websockify 控制台 | Stub (Phase 7) |
| 救援重装流程 | Stub (Phase 7) |
| Monitor agent 脚本嵌入 | inline stub (Phase 7 embed) |
| SSH 密钥认证 | 未实现 (仅密码认证) |
