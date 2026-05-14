# AGENTS.md

本文件是本仓库的 agent 入口说明。开始任何改动前，请先阅读本文件，再按需阅读 `README.md`、相关模块代码和对应构建脚本。

## 项目概览

`l4d2-server-next` 是新一代 Left 4 Dead 2 服务器与 Web 管理后台。项目提供：

- Go/Gin 后端管理器，负责登录鉴权、RCON、地图/插件/备份/日志/监控等接口。
- Vue 3 + TypeScript + Vite 前端管理面板。
- Docker 镜像构建脚本，用于游戏服务器镜像和管理器镜像。
- Windows 原生发布包构建脚本。
- 内置 SourceMod/Metamod 插件资源、插件预设和客户端 VPK 文件。

主要运行形态：

- Linux Docker：`l4d2-pure` 游戏服镜像 + `l4d2-manager-next` 管理器镜像。
- Linux 宿主机：只运行管理器并连接已有 L4D2 服务端。
- Windows：发布包内运行 `l4d2-manager.exe` 并通过 `start_manager.bat` 配置环境变量。

## 目录结构

```text
.
├── backend/                  # Go 后端管理器
│   ├── main.go               # Gin 路由、静态文件服务、后台任务入口
│   ├── controller/           # HTTP 控制器
│   ├── logic/                # 业务逻辑：插件、预设、地图、备份等
│   ├── middlewares/          # 鉴权、GeoIP 访问控制
│   ├── db/                   # SQLite/GORM 初始化与历史监控数据
│   ├── model/                # 数据模型
│   ├── utility/              # GeoIP 等工具
│   ├── consts/               # 运行路径、版本等常量
│   ├── preset.yaml           # 插件预设配置
│   ├── ip2region_v4.xdb      # IPv4 地理位置数据库
│   └── ip2region_v6.xdb      # IPv6 地理位置数据库
├── frontend/                 # Vue 3 + TypeScript + Vite 前端
│   ├── src/views/            # 页面视图
│   ├── src/components/       # 复用组件与弹窗
│   ├── src/services/         # API 封装
│   ├── src/stores/           # Pinia 状态
│   ├── src/router/           # 路由
│   ├── src/utils/            # 前端工具函数
│   └── package.json          # 前端脚本和依赖
├── manifest/
│   ├── docker/               # Dockerfile、启动脚本、服务器 cfg 模板
│   ├── install/              # Linux 一键安装/管理脚本
│   └── boot/                 # Windows 启动脚本模板
├── plugins/                  # 内置 SourceMod/Metamod 插件资源
├── clientvpk/                # 客户端 VPK 资源
├── docs/images/              # README 截图
├── .github/workflows/        # 镜像与 Windows 发布 workflow
├── go.work                   # Go workspace，当前使用 ./backend
└── README.md                 # 用户侧部署和功能说明
```

## 技术栈

- 后端：Go `1.25.5`、Gin、GORM、SQLite、Viper、RCON、gopsutil。
- 前端：Node.js、Vue `3.5`、TypeScript `5.9`、Vite/Rolldown Vite、Pinia、Vue Router、Ant Design Vue、Tailwind CSS、ECharts。
- 构建/发布：Docker Buildx、GitHub Actions、Windows `GOOS=windows GOARCH=amd64` 构建。

## 常用命令

在仓库根目录执行：

```sh
# 后端格式化
gofmt -w backend
```

后端命令需要进入 `backend/`：

```sh
cd backend
go build .
```

前端命令需要进入 `frontend/`：

```sh
cd frontend
npm run build
```

管理器 Docker 镜像：

```sh
docker build -f manifest/docker/manager.Dockerfile -t l4d2-manager-next:local .
```

游戏服 Docker 镜像：

```sh
docker build -f manifest/docker/l4d2.Dockerfile -t l4d2-pure:local .
```

Windows 管理器构建参考：

```sh
cd frontend
npm install
npm run build

cd ../backend
GOOS=windows GOARCH=amd64 go build -ldflags "-X l4d2-manager-next/consts.Version=Dev" -o l4d2-manager.exe main.go
```

注意：当前仓库没有发现 `*_test.go` 测试文件。修改 Go 代码时仍应在 `backend/` 运行 `go build .` 作为编译和依赖检查；修改前端时至少运行 `npm run build`。

## 本地运行

后端管理器需要关键环境变量，环境变量由用户在系统中配置好。
最小示例：

```sh
cd backend
go run .
```

Windows PowerShell 示例：

```powershell
Set-Location backend
go run .
```

以上命令应在 `backend/` 目录执行。默认管理器端口为 `27020`，可通过 `L4D2_MANAGER_PORT` 覆盖。后端会从当前工作目录读取 `ip2region_v4.xdb`、`ip2region_v6.xdb`、`preset.yaml` 和 `static/`，因此本地运行通常应以 `backend/` 为工作目录；打包运行时要确保这些文件被复制到可执行文件同级或 Docker 镜像根路径。

前端开发服务，后端接口包括了前端打包后的静态文件：

```sh
cd frontend
npm run build
```

如需联调后端，请检查 `frontend/src/services/api.ts` 中的 API 基地址和认证逻辑。

## 关键环境变量

- `L4D2_MANAGER_PASSWORD`：Web 管理后台密码，生产环境必填。
- `L4D2_GAME_PATH`：L4D2 `left4dead2` 游戏目录，生产环境必填。
- `L4D2_RCON_URL`：游戏服 RCON 地址，如 `127.0.0.1:27015`。
- `L4D2_RCON_PASSWORD`：RCON 密码。
- `L4D2_MANAGER_PORT`：管理器监听端口，默认 `27020`。
- `L4D2_RESTART_BY_RCON`：为 `true` 时通过 RCON 重启服务器。
- `L4D2_RESTART_CMD`：自定义重启命令。
- `L4D2_CONTAINER_NAME`：Docker 重启场景下的容器名。
- `L4D2_HISTORY_METRICS`：为 `true` 时启用历史性能监控数据库。
- `L4D2_PLUGIN_STORE_PATH`：覆盖插件商店数据路径。
- `STEAM_API_KEY`：用于查询玩家游戏时长。
- `REGION_WHITE_LIST`：GeoIP 区域白名单，不填则不启用拦截。
- `L4D2_TICK`、`L4D2_VAC`、`L4D2_PORT`：游戏服 Docker 镜像启动参数。

## 开发约定

- 保护用户数据和资源文件：`plugins/`、`clientvpk/`、`backend/ip2region_*.xdb`、`.vpk`、`.smx`、`.dll`、`.so` 等通常是二进制或外部资源，除非任务明确要求，不要批量改动或重新格式化。
- 保持中文文件名与现有编码。仓库包含大量中文路径，执行批量脚本时要先确认命令能正确处理 Unicode 路径。
- Go 代码改动后运行 `gofmt`。优先沿用现有 `controller` + `logic` + `model` 分层，不要把业务逻辑塞进路由入口。
- 前端改动优先沿用 Vue SFC、Pinia、Ant Design Vue 与现有组件风格。修改页面后运行 `npm run build`。
- 修改 Docker 构建链路时同步检查 `.github/workflows/manager-image.yml`、`.github/workflows/l4d2-image.yml` 和对应 Dockerfile。
- 修改 Windows 发布内容时同步检查 `.github/workflows/windows-release.yml` 与 `manifest/boot/start_manager.bat`。
- 修改插件预设时优先编辑 `backend/preset.yaml`，并确认相关插件目录仍存在。
- 不要提交本地运行产生的 `private.key`、数据库文件、临时上传分片、构建产物或依赖目录。

## 发布流程参考

GitHub Actions 在推送 `v*` 标签时触发：

- `manager-image.yml`：构建并推送 `laoyutang/l4d2-manager-next`，使用 `manifest/docker/manager.Dockerfile`。
- `l4d2-image.yml`：构建并推送游戏服镜像，使用 `manifest/docker/l4d2.Dockerfile`。
- `windows-release.yml`：构建前端、构建 Windows 后端、复制 `ip2region`、`preset.yaml`、`static/`、`plugins/` 和 `start_manager.bat`，生成 release zip。

发布相关改动至少检查：

```sh
cd frontend
npm run build

cd ../backend
go test ./...
go build .
```

## 变更前检查清单

- 这次改动影响后端、前端、Docker、发布脚本、插件资源中的哪一块？
- 是否需要同步更新 `README.md` 或本文件？
- 是否引入了新的环境变量、端口、挂载路径或持久化文件？
- 是否会影响已有用户的 `left4dead2` 游戏目录、插件目录或备份数据？
- 是否已经运行与改动范围匹配的最小验证命令？
