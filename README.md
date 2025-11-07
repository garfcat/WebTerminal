# WebTerminal

A web-based terminal that connects your browser to a server-side shell using a PTY bridge and WebSocket, rendered with xterm.js. All assets and the WebSocket endpoint are served under the `/xterm` prefix.

## Demo
Open `http://localhost:8089/xterm/` in your browser.

## Features
- Real-time terminal via PTY (`creack/pty`) and WebSocket (`melody`).
- Embedded static assets using Go `embed` (no external static server needed).
- Optional Basic Auth with sliding window cookie session (30 min by default).
- Configurable server address and shell command via CLI flags (Cobra).
- All resources under `/xterm` prefix; WebSocket at `/xterm/webterminal`.
- Front-end powered by `xterm` and `xterm-addon-fit` with auto-resize.

## Requirements
- Go 1.22+
- A modern browser

## Installation & Run
1) Clone:
```shell
git clone https://github.com/garfcat/webterminal.git
cd webterminal
```
2) Install Go dependencies:
```shell
go mod tidy
```
3) Run server (no auth):
```shell
go run main.go
```
4) Open:
```
http://localhost:8089/xterm/
```

### Run with authentication
```shell
go run main.go \
  --auth \
  --username admin \
  --password secret \
  --addr 0.0.0.0:8089 \
  --shell bash
```

## Configuration (flags)
- `--addr` (default `0.0.0.0:8089`): Server listen address.
- `--auth` (default `false`): Enable Basic Auth + session cookie (`session_id`).
- `--username` (default `admin`): Username for Basic Auth.
- `--password` (default `password`): Password for Basic Auth.
- `--shell` (default `sh`): Shell to launch inside the PTY.

## Paths
- Web page: `/xterm/`
- WebSocket endpoint: `/xterm/webterminal`
- Static assets (example): `/xterm/node_modules/xterm/lib/xterm.js`

## Security
This application exposes a direct shell. For production usages:
- Always enable `--auth` and use a strong password.
- Serve behind TLS (use a reverse proxy) so that WebSocket upgrades use `wss://`.
- Consider restricting the shell to a non-privileged user.
- Review audit, access control, and environment isolation as needed.

## Development notes
- Front-end uses `xterm` and `xterm-addon-fit` from `static/node_modules`. If you need to update front-end dependencies, run `npm install` under `static/`.
- For production, consider bundling/minifying and embedding only built assets.

---

# WebTerminal（中文）

一个基于浏览器的 Web 终端，通过 PTY 与 WebSocket 将浏览器与服务端 Shell 连接，并使用 xterm.js 渲染终端界面。所有资源与接口均位于 `/xterm` 前缀下。

## 演示
浏览器打开：`http://localhost:8089/xterm/`

## 特性
- 通过 `creack/pty` 与 `melody` 实现实时终端与 WebSocket 双向传输。
- 使用 Go `embed` 内嵌静态资源，无需额外静态服务器。
- 可选 Basic Auth 与会话 Cookie（默认 30 分钟滑动过期）。
- 通过 Cobra 提供命令行参数，支持自定义监听地址与 Shell。
- 所有资源挂载到 `/xterm`；WebSocket 端点为 `/xterm/webterminal`。
- 前端基于 `xterm` 与 `xterm-addon-fit`，自动适配窗口大小。

## 环境要求
- Go 1.22+
- 现代浏览器

## 安装与运行
1）克隆项目：
```shell
git clone https://github.com/garfcat/webterminal.git
cd webterminal
```
2）安装 Go 依赖：
```shell
go mod tidy
```
3）启动（无认证）：
```shell
go run main.go
```
4）访问：
```
http://localhost:8089/xterm/
```

### 开启认证示例
```shell
go run main.go \
  --auth \
  --username admin \
  --password secret \
  --addr 0.0.0.0:8089 \
  --shell bash
```

## 配置参数
- `--addr`（默认 `0.0.0.0:8089`）：服务监听地址。
- `--auth`（默认 `false`）：启用 Basic Auth 与会话 Cookie（`session_id`）。
- `--username`（默认 `admin`）：认证用户名。
- `--password`（默认 `password`）：认证密码。
- `--shell`（默认 `sh`）：启动的 Shell 程序。

## 路径说明
- 页面入口：`/xterm/`
- WebSocket：`/xterm/webterminal`
- 静态资源示例：`/xterm/node_modules/xterm/lib/xterm.js`

## 安全建议
- 生产环境务必开启 `--auth` 并设置强口令。
- 通过反向代理启用 TLS（从而使用 `wss://`）。
- 使用受限用户运行 Shell，降低风险面。
- 结合实际需求完善访问控制、审计与隔离策略。

## 开发说明
- 前端依赖位于 `static/node_modules`，如需更新可在 `static/` 目录执行 `npm install`。
- 生产部署建议打包与最小化，仅嵌入构建产物。
