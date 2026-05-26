# BACKEND KNOWLEDGE BASE

**Scope**: Go/Gin backend for l4d2-manager-next

## OVERVIEW
Go 1.25.5 后端，提供 Web 管理 API、RCON 代理、插件管理、地图管理、备份恢复、系统监控。使用 Gin + GORM + SQLite。

## STRUCTURE
```
backend/
├── main.go              # 路由入口、后台任务、中间件初始化
├── controller/          # HTTP 控制器 (22 文件，~78 个路由处理函数)
├── logic/               # 业务逻辑 (15 文件，~151 个导出函数)
├── middlewares/         # JWT 鉴权、GeoIP 拦截
├── db/                  # SQLite/GORM 初始化、历史监控数据
├── model/               # 数据模型 (SystemMetric)
├── utility/             # GeoIP 查询
├── consts/              # 路径常量、版本、旧数据迁移
├── preset.yaml          # 插件预设配置
├── ip2region_v4.xdb     # IPv4 地理位置库
└── ip2region_v6.xdb     # IPv6 地理位置库
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| 添加新接口 | controller/ + logic/ | controller 处理 HTTP，logic 处理业务 |
| 插件业务逻辑 | logic/plugins.go | 启用/禁用、文件引用追踪、并发复制 |
| 插件商店 | logic/plugin_store.go | GitHub API、Git LFS、并发下载 |
| 备份系统 | logic/backup.go | YAML 备份/恢复 (744 行) |
| 监控数据 | controller/monitor.go + db/db.go | 1 秒轮询、SQLite 3 天保留 |
| 分片上传 | controller/chunk_upload.go | 2GB 上限、磁盘 90% 检查 |
| 文件处理 | controller/file_processor.go | 解压、文件名清理、GBK 编码 |
| 下载任务 | controller/download.go + logic/ | 后台下载、Workshop 解析 |
| RCON 操作 | controller/rcon.go | 连接池、状态解析、切图/改难度 |

## CONVENTIONS
- Controller → Logic 严格分层，logic/ 不依赖 gin
- 并发操作使用 `ants` 协程池，`runtime.NumCPU()` workers
- 文件引用追踪：`fileRefs` map 防止禁用插件时误删共享文件
- 插件状态持久化：`plugins.yaml`，由 Viper 管理
- 操作审计：`LogOp()` 输出 `[OPT] Time | IP | Role | Path | Params`

## ANTI-PATTERNS
- 不要把业务逻辑塞进 main.go 路由注册处
- Controller 禁止直接操作文件系统，必须通过 logic/
- 不要忽略 `sync.Mutex`/`sync.RWMutex` 的竞态保护
- 环境变量缺失时默认密码为 `laoyutangnb`（仅限开发）
