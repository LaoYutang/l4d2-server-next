# MANIFEST KNOWLEDGE BASE

**Scope**: Docker、Linux 安装/多实例管理、Windows 启动和 GitHub Actions
**Last verified**: 2026-08-13

## OVERVIEW

部署层包含两个容器镜像、Linux 单实例与多实例脚本、Windows 原生启动模板，以及三个发布工作流。管理器容器需要读写游戏卷和独立 `/data`，并通过 Docker socket 或 RCON 重启游戏服。

## STRUCTURE

```text
manifest/
├── docker/
│   ├── manager.Dockerfile       # Node 24 → Go 1.25 → Docker CLI
│   ├── l4d2.Dockerfile          # Ubuntu 22.04 + SteamCMD + L4D2
│   ├── start.sh                 # Tick/VAC/端口/RCON 后启动 srcds
│   └── cfg/
│       ├── server.cfg.30tick
│       ├── server.cfg.60tick
│       ├── server.cfg.100tick
│       └── server.cfg.128tick
├── install/
│   ├── install.sh               # 旧单实例一键安装
│   └── manage.sh                # /data/l4d2 下的交互式多实例管理器
└── boot/
    └── start_manager.bat        # Windows 环境变量模板 + 崩溃重启循环

.github/workflows/
├── manager-image.yml            # v* 标签：管理器镜像
├── windows-release.yml          # v* 标签：Windows ZIP/Release
└── l4d2-image.yml               # workflow_dispatch：游戏服镜像
```

## IMAGE TOPOLOGY

### Manager image

`docker/manager.Dockerfile`：

1. `node:24-alpine` 安装前端依赖并构建到 `backend/static/`。
2. `golang:1.25-alpine` 构建管理器，并通过 ldflags 注入版本。
3. `docker:29.1.1-cli-alpine3.22` 作为最终镜像，包含管理器、静态文件、插件预设和 IPv4/IPv6 xdb。

运行时通常挂载：

- 游戏数据卷到 `/left4dead2`；
- 插件库到 `/plugins`；
- 管理器持久化卷到 `/data`；
- `/var/run/docker.sock` 用于容器重启；
- `/proc` 只读挂载用于宿主监控。

### Game image

`docker/l4d2.Dockerfile` 基于 Ubuntu 22.04，安装 i386 运行库，通过 SteamCMD 依次获取 Windows/Linux 的 app 222860 文件。`docker/start.sh` 根据环境变量选择 `server.cfg` 并启动 `srcds_run`。

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| 管理器基础镜像/工具链 | `docker/manager.Dockerfile` | 改版本时同时检查 workflow 与 Go/Node 项目版本 |
| 游戏服依赖/SteamCMD | `docker/l4d2.Dockerfile` | 32 位库、双平台 app_update、重试次数 |
| Tick/VAC/RCON 启动 | `docker/start.sh` + `docker/cfg/` | RCON 行不存在时会追加，不能只做 sed 替换 |
| Linux 单实例模板 | `install/install.sh` | 兼容路径，功能较少 |
| Linux 多实例 | `install/manage.sh` | 配置、端口检测、Compose 生成、镜像更新、实例管理 |
| Windows 启动 | `boot/start_manager.bat` | 发布 ZIP 中的用户可编辑模板 |
| 镜像发布 | `.github/workflows/manager-image.yml`、`.github/workflows/l4d2-image.yml` | Docker Hub 元数据、Buildx cache |
| Windows Release | `.github/workflows/windows-release.yml` | 前端 → Windows Go 二进制 → 资源打包 |

## MULTI-INSTANCE MANAGER

`manage.sh` 的稳定路径和约束：

- 根目录：`/data/l4d2`。
- 实例配置：`/data/l4d2/.config/instances/*.conf`。
- 生成文件：`/data/l4d2/docker-compose.yaml`。
- 默认镜像：`laoyutang/l4d2-pure:latest` 与 `laoyutang/l4d2-manager-next:latest`。
- 默认首实例端口：游戏 `27015`、面板 `27020`；新增实例会检测游戏/面板端口冲突。
- 每个实例包含游戏容器和 `<name>-manager` 容器，共享游戏数据卷，管理器另有 plugins/data 卷。
- 支持 Docker 安装/状态、镜像源、镜像拉取与重建、实例增删改查和导入已有 Compose。

修改 Compose 生成逻辑时，要同步检查保存/导入配置、端口冲突、符号链接、卷名和已有实例升级。

## ENVIRONMENT VARIABLES

### Manager

| Variable | Purpose |
|----------|---------|
| `L4D2_MANAGER_PORT` | 面板监听端口，默认 27020 |
| `L4D2_MANAGER_PASSWORD` | 管理员密码，生产环境必须修改 |
| `L4D2_GAME_PATH` | `left4dead2` 目录，容器中为 `/left4dead2` |
| `L4D2_RCON_URL` / `L4D2_RCON_PASSWORD` | RCON 地址和密码 |
| `L4D2_RESTART_BY_RCON` | 优先通过 RCON 退出以触发自动重启 |
| `L4D2_RESTART_CMD` / `L4D2_CONTAINER_NAME` | 非 RCON 重启后备路径 |
| `L4D2_PLUGIN_STORE_PATH` | 插件商店资源路径 |
| `STEAM_API_KEY` | 玩家游戏时长查询，可选 |

### Game server

| Variable | Purpose |
|----------|---------|
| `L4D2_TICK` | 30/60/100/128，默认 30 |
| `L4D2_VAC` | `true` 不加 `-insecure`；默认 false |
| `L4D2_PORT` | 游戏端口，默认 27015 |
| `L4D2_RCON_PASSWORD` | 写入或追加到 `server.cfg` |

## CI/CD

- 推送 `v*` 标签同时触发 `manager-image.yml` 和 `windows-release.yml`。
- 管理器镜像发布 `latest` 和标签版本，并把标签通过 `-X l4d2-manager-next/consts.Version=...` 注入。
- Windows workflow 使用 Node 20、稳定 Go，先生成 `backend/static/`，再交叉编译 amd64，并打包二进制、xdb、`preset.yaml`、`plugins/`、`static/` 和启动脚本。
- `l4d2-image.yml` 只允许手动触发，发布 `latest` 和时间戳标签。
- 管理器 workflow 在构建前把 `plugins/` 时间戳归一为 2020-01-01，以稳定 Docker 缓存。

## CONVENTIONS

- 管理器最终镜像必须保留 Docker CLI；`restart.go` 会在非 RCON 路径使用 Docker。
- 版本只通过 linker flags 注入，不要改写源码常量生成发布版本。
- `start.sh` 对 `rcon_password` 既支持替换已有行，也支持缺失时追加。
- 生成 Compose 时沿用 `list_env`/打印函数；若扩展可接受字符集，要同时验证 YAML 序列化和已有配置导入。
- 游戏卷、plugins 卷和 manager data 卷职责不同，不要合并。
- Docker 29.3+ 运行 32 位 `srcds` 需要 `security_opt: seccomp:unconfined`；单实例与多实例模板都要保持一致。
- Shell 文件保持 LF 和可执行位；`.bat` 保持 Windows 可用的 CRLF/命令语法。

## ANTI-PATTERNS

- 不要只修改 Dockerfile 而不检查对应 workflow、安装脚本和用户文档。
- 不要移除插件时间戳归一步骤，除非同时重新设计全局镜像缓存。
- 不要把管理员数据库/JSON 写进 `/plugins` 或游戏卷；管理器状态必须挂载 `/data`。
- 不要改变命名卷或服务命名而不提供已有多实例迁移路径。
- 不要在日志中输出管理员密码、RCON 密码、Steam API key 或 Docker 登录凭据。
- 不要让 `v*` 标签自动构建游戏服镜像；当前大镜像仍是手动发布策略。
- 不要删除 `seccomp:unconfined` 注释/配置而只在当前 Docker 版本验证。

## VALIDATION

```bash
# Shell 语法
bash -n manifest/docker/start.sh
bash -n manifest/install/install.sh
bash -n manifest/install/manage.sh

# 本地镜像
docker build -f manifest/docker/manager.Dockerfile -t l4d2-manager-next:local .
docker build -f manifest/docker/l4d2.Dockerfile -t l4d2-pure:local .

# 与发布相同的应用构建
npm --prefix frontend run build
go -C backend build .
```

修改 workflow 后还要人工核对 trigger、secrets 名称、tag/metadata 和发布包文件清单；本地 Docker 构建不能覆盖这些错误。
