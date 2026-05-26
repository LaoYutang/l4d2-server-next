# PROJECT KNOWLEDGE BASE

**Project**: l4d2-server-next
**Generated**: 2026-05-26

## OVERVIEW

新一代 Left 4 Dead 2 服务器与 Web 管理后台。
Go/Gin 后端 + Vue 3/TypeScript/Vite 前端，支持 Docker 与 Windows 原生部署，内置 SourceMod/Metamod 插件资源。

## STRUCTURE

```
.
├── backend/          # Go 后端管理器 (详见 backend/AGENTS.md)
│   ├── main.go       # Gin 路由入口、后台任务启动
│   ├── controller/   # HTTP 控制器
│   ├── logic/        # 业务逻辑
│   ├── middlewares/  # 鉴权、GeoIP
│   ├── db/           # SQLite/GORM
│   ├── model/        # 数据模型
│   ├── utility/      # GeoIP 工具
│   └── consts/       # 路径与版本常量
├── frontend/         # Vue 3 + TS + Vite 前端 (详见 frontend/AGENTS.md)
│   ├── src/views/    # 页面视图
│   ├── src/components/ # 复用弹窗
│   ├── src/services/ # API 封装
│   ├── src/stores/   # Pinia 状态
│   └── src/router/   # 路由
├── manifest/         # Docker、安装脚本、启动模板 (详见 manifest/AGENTS.md)
│   ├── docker/       # Dockerfile
│   ├── install/      # Linux 安装/管理脚本
│   └── boot/         # Windows 启动脚本
├── plugins/          # 内置 SourceMod/Metamod 插件资源
├── clientvpk/        # 客户端 VPK 资源
└── docs/images/      # README 截图
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| 添加/修改后端接口 | backend/controller/ + backend/logic/ | controller→logic 分层 |
| 前端页面开发 | frontend/src/views/ | Vue SFC + `<script setup>` |
| API 调用封装 | frontend/src/services/api.ts | 单例 ApiService，Bearer 认证 |
| 状态管理 | frontend/src/stores/ | Pinia Composition API 风格 |
| 插件预设配置 | backend/preset.yaml | YAML 格式预设插件组合 |
| Docker 镜像构建 | manifest/docker/ | manager.Dockerfile / l4d2.Dockerfile |
| CI/CD 发布 | .github/workflows/ | v* 标签触发三 workflow |
| 插件资源文件 | plugins/ | 不要批量改动二进制文件 |

## CODE MAP

| Symbol | Type | Location | Role |
|--------|------|----------|------|
| main | func | backend/main.go | Gin 路由入口、后台任务启动 |
| router | var | backend/main.go | 所有 API 路由分组定义 |
| ApiService | class | frontend/src/services/api.ts | 前端统一 API 封装 (1036 行) |
| useAuthStore | store | frontend/src/stores/auth.ts | 登录状态与角色管理 |
| PluginExportTask | struct | backend/logic/plugin_export.go | 插件导出后台任务 |
| BackupInfo | struct | backend/logic/backup.go | 备份数据结构 |

## CONVENTIONS

- Go: controller + logic + model 分层，业务逻辑不塞进路由
- Go: 环境变量驱动配置，默认值在代码中
- 前端: Vue SFC + `<script setup>` + TypeScript
- 前端: Ant Design Vue + Tailwind CSS 混合使用
- 认证: 后端 JWT/Bearer，前端 localStorage 存储密码
- 分片上传: 5MB 分片，并发 3，30 秒超时

## ANTI-PATTERNS (THIS PROJECT)

- 不要把业务逻辑塞进 main.go 或 controller，保持 logic/ 独立
- 不要批量改动 plugins/、clientvpk/ 中的二进制资源 (.smx, .vpk, .dll, .so)
- 不要提交 private.key、数据库文件、构建产物或 node_modules
- 前端不要直接 fetch，统一走 api.ts 的 ApiService
- Go 代码改动后必须运行 `gofmt -w backend`

## UNIQUE STYLES

- 中文文件名支持 UTF-8/GBK，批量脚本需确认 Unicode 处理
- 插件 ZIP 格式有严格规范：单插件根目录为 `left4dead2/`，多插件为 `PluginName/left4dead2/`
- 插件商店通过 GitHub API 拉取远程插件仓库
- 备份系统导出 YAML，不含地图或完整游戏目录
- `__MACOSX/` 和 `.DS_Store` 在上传时自动忽略

## COMMANDS

```bash
# 后端格式化
gofmt -w backend

# 后端编译检查
cd backend && go build .

# 前端构建
cd frontend && npm run build

# 管理器 Docker 镜像
docker build -f manifest/docker/manager.Dockerfile -t l4d2-manager-next:local .

# 游戏服 Docker 镜像
docker build -f manifest/docker/l4d2.Dockerfile -t l4d2-pure:local .
```

## NOTES

- 后端本地运行需以 `backend/` 为工作目录，确保读取到 `ip2region_*.xdb`、`preset.yaml`、`static/`
- Windows 构建使用 `GOOS=windows GOARCH=amd64`，需先构建前端生成 `static/`
- 当前无系统测试文件（仅 logic/ 下 2 个测试），修改后至少运行 `go build` 验证
- Docker 29.3+ 默认 seccomp 与 32 位 srcds 不兼容，需配置 `security_opt: seccomp:unconfined`
- 管理器默认端口 `27020`，环境变量 `L4D2_MANAGER_PORT` 可覆盖
