# BACKEND KNOWLEDGE BASE

**Scope**: Go/Gin backend for l4d2-server-next
**Last verified**: 2026-08-13

## OVERVIEW

Go 1.25.5 管理器，负责 Bearer 鉴权、访问控制、RCON、地图与插件文件、后台下载/导出、备份、监控、玩家统计和操作审计。SQLite 使用纯 Go 驱动；运行配置和数据库统一位于工作目录下的 `data/`。

## STRUCTURE

```text
backend/
├── main.go                 # 启动顺序、后台任务、中间件和全部路由
├── controller/
│   ├── chunk_upload.go     # 分片上传、续传、取消与过期清理
│   ├── download*.go        # 下载任务、链接解析入口、Steam CDN 配置
│   ├── map_*.go            # 地图详情/汇总/检查/精简/热重载
│   ├── access_control.go   # 管理员访问控制 API
│   ├── game_bans.go        # 游戏服原生封禁 API
│   ├── audit.go            # 审计查询
│   └── log_helper.go       # LogOp / FailWithError
├── logic/
│   ├── access_control.go   # 规则校验、revision、快照、原子保存
│   ├── audit.go            # 异步审计 writer 和查询
│   ├── game_bans*.go       # listid/listip 与持久化 exec 区块
│   ├── manager_config.go   # 面板功能开关和运行配置
│   ├── map_*.go            # 地图解析、检查和热重载
│   ├── vpk_trim.go         # VPK 精简任务
│   ├── plugins.go          # 插件启停、热加载、文件引用
│   └── plugin_store.go     # GitHub 插件商店与后台下载
├── middlewares/
│   ├── auth.go             # admin/guest Bearer 鉴权与限流
│   └── access_control.go   # 可信代理和最终客户端 IP
├── db/ + model/            # monitor/player_stats/audit SQLite
├── pkg/
│   ├── valve/              # 内置 Valve 文件格式实现
│   └── vpkmission/         # 宽容 mission 解析
├── utility/                # ip2region GeoIP
├── consts/                 # 路径、旧数据迁移、版本
├── preset.yaml             # 插件预设
└── ip2region_*.xdb         # IPv4/IPv6 数据库
```

## STARTUP FLOW

`main.go` 当前按以下职责初始化：

1. 初始化监控、玩家统计、审计三个数据库并启动异步审计 writer。
2. 启动性能监控、玩家统计采集、上传过期清理和各类临时目录清理。
3. 加载面板访问控制，并确保游戏封禁文件会被 `server.cfg`/tick 模板执行。
4. 初始化 GeoIP；即使失败，IP/CIDR 访问规则仍可工作。
5. 安装访问控制中间件、静态资源缓存策略、Bearer 鉴权和业务路由。

新增需要常驻的后台任务时，必须明确启动顺序、停止/清理策略和并发保护。

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| 注册路由 | `main.go` | 公开、自助授权、普通鉴权、管理员专属接口要区分 |
| 管理员/访客鉴权 | `middlewares/auth.go` + `controller/auth.go` | 管理员密码、临时码、自助 1 小时授权 |
| 最终客户端 IP | `middlewares/access_control.go` + `logic/access_control.go` | 可信代理链、IPv4-mapped IPv6、地区/IP/CIDR |
| 操作审计 | `controller/log_helper.go` + `logic/audit.go` + `model/audit.go` | 异步 SQLite，记录 role/IP/path/result/detail |
| 面板持久化配置 | `logic/manager_config.go` + `consts/consts.go` | JSON 原子写入，旧根目录文件自动迁移 |
| 游戏封禁 | `logic/game_bans.go` + `logic/game_ban_persistence.go` | RCON `listid/listip`，永久封禁写原生 cfg |
| 地图上传 | `controller/upload.go` + `controller/chunk_upload.go` + `controller/file_processor.go` | 2 GiB 上限、5 MiB 分片、路径边界、压缩包 |
| 地图元数据 | `logic/missions.go` + `pkg/vpkmission/` | 战役/章节解析与宽容恢复 |
| 地图检查 | `logic/map_vpk_inspection.go` + `controller/map_inspection.go` | 字典缺失、全局脚本、JSON 缓存 |
| 地图精简 | `logic/vpk_trim.go` + `controller/vpk_trim.go` + `controller/map_trim.go` | 功能开关、临时文件、手动/自动入口 |
| 地图热重载 | `logic/map_hot_reload.go` + `controller/map_hot_reload.go` | 可配置 RCON 命令和状态 |
| 插件核心 | `logic/plugins.go` + `logic/plugin_config.go` | 文件引用、热加载/卸载、配置读写 |
| 插件商店/导出 | `logic/plugin_store.go` + `logic/plugin_export.go` | 取消、进度和临时目录生命周期 |
| 监控与统计 | `controller/monitor.go`、`controller/player_stats.go` + `db/db.go` | 监控 3 天；玩家统计 30 天 |
| 备份 | `logic/backup.go` + `controller/backup.go` | YAML 导入导出与详情 |

## PERSISTENCE

`consts/consts.go` 以 `./data` 为根目录：

| File | Owner |
|------|-------|
| `private.key` | 临时授权 JWT 的 HS256 密钥 |
| `manager_config.json` | 自助授权、统计、监控历史、VPK 精简、热重载命令、Steam CDN IP |
| `access_control.json` | 可信代理与面板黑白名单，带 revision |
| `map_vpk_inspections.json` | 地图 VPK 检查缓存 |
| `monitor.db` | 性能历史 |
| `player_stats.db` | 玩家采样、别名和在线时长 |
| `audit.db` | 操作审计 |

插件状态、备份和 SourceMod 配置位于游戏目录或 addons 目录，不属于 `data/`。修改持久化格式时同时考虑旧数据迁移、原子替换、损坏恢复和并发读写。

## CONVENTIONS

- Controller 负责绑定/校验输入、角色检查、调用逻辑层和形成 HTTP 响应；可复用业务逻辑放 `logic/`。
- 历史文件上传/解压编排仍位于 `controller/`；在这些文件中修改时保持现有清理和安全边界，不要继续复制到新控制器。
- 写操作通常以 `defer LogOp(c, "详情")()` 收尾；错误使用 `FailWithError`，避免返回失败却被审计成成功。
- admin/guest 授权必须在后端执行。访问控制配置与游戏黑名单接口还需显式管理员检查。
- 获取 IP 使用 `middlewares.GetClientIP/GetClientIPInfo`；不要直接读转发头或只用 `c.ClientIP()`。
- 路径输入必须先规范化和限定根目录。上传 ID 只接受规范 RFC 4122 v4 UUID；删除优先使用 `os.OpenRoot`。
- 配置文件优先采用临时文件 + `Sync` + `Rename` 原子替换；保留原文件权限。
- 共享运行状态使用 mutex、不可变快照或 channel；不要返回仍会被后台修改的 map/slice。
- RCON 地址/密码统一读取 `L4D2_RCON_URL`、`L4D2_RCON_PASSWORD`。
- 修改 Go 文件后运行 `go fmt ./...`；针对性测试通过后再运行 `go test ./...` 和 `go build .`。

## SECURITY-SENSITIVE AREAS

- `middlewares/access_control.go`：可信代理链决定所有限流、审计和规则判断使用的 IP。
- `logic/access_control.go`：保存时会防止管理员把自己锁在面板外，并通过 revision 防并发覆盖。
- `controller/chunk_upload.go`、`controller/remove.go`、`controller/file_processor.go`：曾修复路径穿越，不能退回基于未经验证的 `filepath.Join/RemoveAll`。
- `../frontend/src/components/PluginDetailModal.vue` 的后端输入可能包含第三方 README；后端不要假设 Markdown 安全。
- `logic/game_bans.go`：永久与计时封禁语义不同；永久项需要写 `banned_user.cfg/banned_ip.cfg`。
- `controller/log_helper.go`：审计详情会归一化空白并限制长度，不要记录密码、token 或完整私钥。

## TESTING

测试已覆盖 controller、logic、middlewares、utility、`pkg/vpkmission` 和 `pkg/valve`。常用命令：

```bash
# 从仓库根目录执行全量检查
go -C backend fmt ./...
go -C backend test ./...
go -C backend build .

# 示例：按领域聚焦
go -C backend test ./controller -run 'Upload|Map|Access|Audit'
go -C backend test ./logic -run 'Access|GameBan|VPK|Plugin'
go -C backend test ./middlewares -run AccessControl
```

涉及路径、配置或数据库的测试使用 `t.TempDir()`/临时路径，并在 `t.Cleanup` 中恢复全局变量；不要触碰真实游戏目录或 `data/`。

## ANTI-PATTERNS

- 不要在 `main.go` 中实现业务，只注册路由和启动依赖。
- 不要信任文件名、`uploadId`、地图名、压缩包内路径或代理头。
- 不要用前端 `requiresAdmin` 代替控制器管理员校验。
- 不要在持锁期间执行 RCON、网络请求、慢磁盘 IO 或向无缓冲 channel 阻塞发送。
- 不要直接覆盖持久化 JSON/cfg；避免部分写入造成恢复模式或配置丢失。
- 不要忽略后台任务的取消、临时目录清理和进度状态竞态。
- 不要批量格式化或改写 `pkg/valve`；它是较大的内置上游实现，修改应聚焦并运行相关测试。
