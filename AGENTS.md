# PROJECT KNOWLEDGE BASE

**Project**: l4d2-server-next
**Last verified**: 2026-08-13

## OVERVIEW

Left 4 Dead 2 游戏服务器与 Web 管理后台。Go/Gin 后端管理游戏文件、RCON、插件、地图、监控和持久化数据；Vue 3/TypeScript 前端提供管理界面；VitePress 维护用户文档。支持 Docker 多实例部署和 Windows 原生部署，仓库内置 SourceMod/Metamod、客户端 VPK 及 Valve/VPK 解析代码。

## STRUCTURE

```text
.
├── backend/             # Go 管理器，详见 backend/AGENTS.md
│   ├── main.go          # 启动流程、中间件、全部 HTTP 路由
│   ├── controller/      # Gin 控制器、上传/下载/RCON 等 HTTP 编排
│   ├── logic/           # 可复用业务逻辑与持久化
│   ├── middlewares/     # Bearer 鉴权、访问控制与客户端 IP
│   ├── db/ + model/     # 监控、玩家统计、审计 SQLite 模型
│   ├── pkg/valve/       # 内置 Valve BSP/VPK/VDF 等解析库
│   ├── pkg/vpkmission/  # 宽容的 VPK mission 解析
│   ├── utility/         # GeoIP
│   └── consts/          # 路径、数据迁移与版本常量
├── frontend/            # Vue 3 管理面板，详见 frontend/AGENTS.md
│   └── src/
│       ├── views/       # 页面
│       ├── components/  # 复用组件和弹窗
│       ├── services/    # ApiService
│       ├── stores/      # Pinia
│       └── router/      # Hash 路由与角色守卫
├── docs/                # VitePress 用户与开发文档
├── manifest/            # Docker/安装/启动资源，详见 manifest/AGENTS.md
├── .github/workflows/   # 镜像与 Windows 发布流程
├── plugins/             # 内置 SourceMod/Metamod 资源
└── clientvpk/           # 客户端 VPK 资源
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| 添加或修改后端接口 | `backend/main.go` + `backend/controller/` + `backend/logic/` | 路由只在 `main.go` 注册；可复用业务放 `logic/` |
| 前端页面与导航 | `frontend/src/views/` + `frontend/src/router/index.ts` + `frontend/src/layouts/MainLayout.vue` | 管理员页面同时需要路由和后端权限 |
| API 调用与长任务 | `frontend/src/services/api.ts` | Bearer、分片上传、SSE、下载/导出任务统一封装 |
| 访问控制与游戏封禁 | `backend/logic/access_control.go`、`backend/middlewares/access_control.go`、`backend/logic/game_bans.go` | 最终客户端 IP、可信代理、黑白名单、RCON 封禁 |
| 操作审计 | `backend/controller/log_helper.go` + `backend/logic/audit.go` | `defer LogOp(...)()` 记录结果并异步写库 |
| 地图解析/检查/精简 | `backend/logic/missions.go`、`backend/logic/map_vpk_inspection.go`、`backend/logic/vpk_trim.go`、`backend/pkg/` | 上传、下载、重命名、删除需同步维护检查数据 |
| 插件管理与商店 | `backend/logic/plugins.go`、`backend/logic/plugin_store.go`、`backend/logic/plugin_export.go` | 热加载/卸载、GitHub 商店、后台导出任务 |
| 面板级持久化配置 | `backend/logic/manager_config.go` + `backend/consts/consts.go` | 运行数据统一写到工作目录下的 `data/` |
| 用户文档 | `docs/guide/`、`docs/features/`、`docs/operations/` | 用户可见行为或配置变化应同步文档 |
| 部署与发布 | `manifest/` + `.github/workflows/` | Docker、Linux 多实例、Windows 发布 |

## CODE MAP

| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `main` | func | `backend/main.go` | 初始化 DB/后台任务/访问控制并注册路由 |
| `ApiService` | class | `frontend/src/services/api.ts` | 前端统一 API、上传下载和流式日志入口 |
| `AccessControlSnapshot` | struct | `backend/logic/access_control.go` | 无锁读取的访问控制运行快照 |
| `LogOp` | func | `backend/controller/log_helper.go` | 控制台操作日志和异步审计入库 |
| `InspectMapVPK` | func | `backend/logic/map_vpk_inspection.go` | 字典缺失和全局脚本检查 |
| `ParseDownloadLink` | func | `backend/logic/link_parser.go` | 统一解析 Workshop/QQ 闪传链接 |
| `ManagerConfig` | struct | `backend/logic/manager_config.go` | 自助授权、统计、监控、精简、热重载、CDN 配置 |
| `PluginExportProgress` | struct | `backend/logic/plugin_export.go` | 插件全量导出后台任务状态 |

## CONVENTIONS

- Go 控制器负责 HTTP 参数、权限和响应；跨接口可复用的规则、状态和持久化优先放 `logic/`。
- 运行数据位于相对工作目录 `./data/`：`private.key`、三个 SQLite 数据库和 JSON 配置均由 `consts` 管理；不要恢复到仓库根目录的旧路径。
- 认证使用 Bearer：管理员密码来自 `L4D2_MANAGER_PASSWORD`，临时授权码登录为 guest；敏感接口必须由后端再次校验管理员角色。
- 写操作使用 `defer LogOp(c, detail)()` 记录成功/失败；统一错误路径优先使用 `FailWithError`。
- 前端使用 Vue SFC + `<script setup>` + TypeScript、Pinia、Ant Design Vue 和 Tailwind CSS。
- 应用 API 统一放在 `ApiService`；允许在该服务内部为 XHR 上传、SSE 和公开接口直接使用 `fetch`。
- 分片上传为 5 MiB、并发 3、单分片 30 秒超时；网络中断保留 `uploadId` 续传，主动取消必须调用 `/upload/cancel`。
- `frontend/src/components.d.ts` 由 `unplugin-vue-components` 在构建时生成并纳入版本控制。
- 用户可见功能、环境变量、部署步骤变化要同步 `docs/`。

## ANTI-PATTERNS

- 不要把新业务逻辑直接堆进 `main.go`；它只负责初始化和路由。
- 不要批量改动 `plugins/`、`clientvpk/` 或 `backend/pkg/valve/` 中的大量资源/上游代码。
- 不要提交 `data/`、`private.key`、`*.db*`、`backend/static/`、`node_modules/` 或 VitePress 构建产物。
- 不要绕过 `validateUploadId`、`NormalizeMapVPKName`、`os.OpenRoot` 等路径边界；上传、删除、解压代码属于高风险区域。
- 不要直接信任 `X-Forwarded-For`；客户端 IP 必须经过可信代理逻辑。
- 不要只靠前端隐藏按钮做授权；admin/guest 权限以后端为准。
- 不要渲染未清洗的插件 Markdown/HTML；沿用 `marked + DOMPurify`。
- Go 代码修改后必须格式化相关文件，并运行与改动相称的测试。

## UNIQUE STYLES

- 中文文件名和插件配置可能包含 UTF-8/GBK，批处理前确认编码。
- 插件 ZIP：单插件根目录为 `left4dead2/`，多插件为 `PluginName/left4dead2/`；`__MACOSX/` 和 `.DS_Store` 会被忽略。
- 备份导入导出使用 YAML，覆盖插件、管理员、服务器信息/配置，不包含地图或完整游戏目录。
- 地图管线会解析战役/章节、保存 VPK 检查结果，并可选择自动或手动精简客户端资源。
- 监控、玩家统计、审计分别使用独立 SQLite 数据库；审计写入异步执行。
- 访问控制使用配置 revision、原子写入和不可变快照；GeoIP 不可用时 IP/CIDR 规则仍应工作。

## COMMANDS

```bash
# 后端（从仓库根目录执行）
go -C backend fmt ./...
go -C backend test ./...
go -C backend build .

# 前端（构建产物写入 backend/static/）
npm --prefix frontend run build

# VitePress 文档
npm --prefix docs run docs:build

# Docker 镜像
docker build -f manifest/docker/manager.Dockerfile -t l4d2-manager-next:local .
docker build -f manifest/docker/l4d2.Dockerfile -t l4d2-pure:local .
```

## NOTES

- 后端本地运行建议以 `backend/` 为工作目录，确保读取 `ip2region_*.xdb`、`preset.yaml`、`static/`，并把运行数据写入 `backend/data/`。
- `npm run build` 会清空并重建被忽略的 `backend/static/`；不要把它当作源码目录。
- 后端目前在 controller、logic、middlewares、utility 和 pkg 下均有测试；后端改动优先运行 `go test ./...`，不要再假设只有少量测试。
- Windows 发布需先构建前端，再使用 `GOOS=windows GOARCH=amd64` 编译。
- Docker 29.3+ 默认 seccomp 与 32 位 `srcds` 不兼容，游戏服 Compose 需要 `security_opt: seccomp:unconfined`。
- 管理器默认端口为 `27020`，可由 `L4D2_MANAGER_PORT` 覆盖。
- 推送 `v*` 标签触发管理器镜像和 Windows 发布；游戏服镜像工作流仍为手动触发。
