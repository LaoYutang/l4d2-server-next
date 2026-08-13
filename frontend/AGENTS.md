# FRONTEND KNOWLEDGE BASE

**Scope**: Vue 3 + TypeScript + Vite frontend for l4d2-server-next
**Last verified**: 2026-08-13

## OVERVIEW

Vue 3.5 管理面板，使用 Rolldown Vite、TypeScript 5.9、Pinia 3、Vue Router 4、Ant Design Vue 4.2、Tailwind CSS 4.1 和 ECharts 6。构建包含 `vue-tsc` 类型检查，产物直接写入 `backend/static/`。

## STRUCTURE

```text
frontend/src/
├── main.ts
├── App.vue                  # ConfigProvider 与全局主题
├── layouts/
│   └── MainLayout.vue       # 桌面/移动导航和角色可见性
├── router/
│   └── index.ts             # Hash 路由、懒加载、auth/admin 守卫
├── services/
│   └── api.ts               # ApiService、XHR 分片上传、SSE 日志
├── stores/
│   ├── auth.ts              # admin/guest 登录与 localStorage
│   ├── monitor.ts           # 实时/历史监控
│   └── theme.ts             # 亮暗主题
├── views/
│   ├── Home.vue             # 服务器状态与快捷 RCON
│   ├── Maps.vue             # 地图、上传下载、解析、检查、精简、热重载
│   ├── Plugins.vue          # 插件、商店、预设、导出、热加载
│   ├── Monitor.vue          # 系统监控
│   ├── PlayerStats.vue      # 玩家排名、趋势和详情
│   ├── Logs.vue             # SourceMod 日志与 SSE
│   ├── Audit.vue            # 管理员操作审计
│   ├── AccessControl.vue    # 管理员访问控制与游戏黑名单
│   ├── Backup.vue           # 备份导入导出与恢复
│   └── ...                  # Rcon/Admins/System/ServerInfo/ServerConfig/Login
├── components/
│   ├── AccessRuleList.vue
│   ├── GameBlacklistTab.vue
│   ├── MapSelectorModal.vue
│   ├── PluginConfigModal.vue
│   ├── PluginDetailModal.vue
│   ├── DifficultyModal.vue
│   └── GameModeModal.vue
├── utils/                   # 状态解析、游戏常量、剪贴板
├── data/officialMaps.ts
└── components.d.ts          # 自动导入声明（生成文件，已跟踪）
```

## ROUTING AND ROLES

- 使用 `createWebHashHistory`，`MainLayout` 下的页面均懒加载。
- `/login` 公开；其他页面要求已登录。
- `/audit` 和 `/access-control` 带 `requiresAdmin`，路由守卫会把 guest 重定向到首页。
- 菜单和按钮也按 `authStore.isAdmin` 隐藏，但这只是交互层；真正权限必须由后端保证。
- 新增页面通常要同时修改 `src/views/`、`src/router/index.ts` 和 `src/layouts/MainLayout.vue`。

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| API/类型/长任务 | `src/services/api.ts` | FormData/JSON、上传、下载、导出、SSE 都在这里 |
| 登录与角色 | `src/stores/auth.ts` + `src/router/index.ts` | `server_password` localStorage，admin/guest |
| 地图功能 | `src/views/Maps.vue` | 当前最大的页面；优先抽取新组件/组合逻辑 |
| 插件功能 | `src/views/Plugins.vue` + `src/components/Plugin*.vue` | 商店与后台任务状态较多 |
| 访问控制 | `src/views/AccessControl.vue` + `src/components/AccessRuleList.vue` | revision、草稿预览、锁定保护 |
| 游戏黑名单 | `src/components/GameBlacklistTab.vue` | 永久/计时封禁与 RCON 错误状态 |
| 操作审计 | `src/views/Audit.vue` | 管理员分页、筛选和 GeoIP 展示 |
| 玩家统计 | `src/views/PlayerStats.vue` | 时间范围、趋势 bucket、玩家详情 |
| 日志流 | `src/views/Logs.vue` + `api.streamLog` | SSE 需要显式 close/AbortController |
| 主题 | `src/stores/theme.ts` + `src/App.vue` | localStorage、系统偏好、Ant token |
| 全局组件声明 | `src/components.d.ts` + `vite.config.ts` | 由 `unplugin-vue-components` 生成 |

## API CONVENTIONS

- 业务请求经单例 `api` 发起；视图/组件不要新增裸 `fetch`。
- 允许 `ApiService` 内部直接使用 `fetch`/`XMLHttpRequest`：Bearer、上传进度、30 秒超时、SSE 和公开自助授权需要底层控制。
- `src/stores/auth.ts` 的登录请求是刻意保留的例外，因为成功前 Pinia 尚未建立认证状态。
- `handleResponseError` 对 `401/429` 注销，对 `403` 抛出权限错误；新增底层请求也要保持一致。
- 后端错误主要是文本响应，调用方通常捕获并以 Ant `message` 显示；不要静默吞掉状态码。
- 对后台任务保留任务 ID，并在页面卸载、取消或完成时清理轮询/流/AbortController。

## CHUNK UPLOAD INVARIANTS

`Maps.vue` 和 `ApiService` 共同维护地图分片上传：

- 5 MiB 分片、并发 3、单分片 30 秒超时。
- `/upload/init` 成功后立即保存 `uploadId`。
- 网络/超时中断保留服务端分片和 `uploadId`，页面显示“继续上传”。
- 用户主动移除、取消或用同名文件重新开始时，先 abort 客户端请求，再调用 `api.cancelUpload(uploadId)`。
- 初始化期间取消时，`ApiService` 会在拿到 ID 后自行清理，防止孤儿临时目录。
- 成功、不可续传错误和主动取消都要按当前 `uploadId` 清理本地状态，避免旧请求覆盖新任务。

修改上传流程时必须分别验证成功、断网续传、主动取消、初始化时取消和同名文件重试。

## UI CONVENTIONS

- Vue SFC 使用 `<script setup lang="ts">`；保持现有两空格缩进和 Composition API 风格。
- 弹窗采用 `v-model:open`；窄屏宽度和表格/卡片双布局沿用现有响应式模式。
- Ant Design 组件通过自动导入 resolver；图标通常显式从 `@ant-design/icons-vue` 导入。
- 样式使用 Tailwind 工具类配合 Ant 主题 token；不要硬编码只适用于亮色模式的颜色。
- 插件 README 使用 `marked` 后必须经过 `DOMPurify.sanitize`；禁止直接渲染第三方 HTML。
- 对长列表/轮询/请求使用稳定 ID 防竞态；仅以文件名或数组下标作为任务身份时要特别谨慎。
- `src/components.d.ts` 不手工维护组件清单；运行构建让生成器同步新增/删除。

## KNOWN PRESSURE POINTS

- `src/views/Maps.vue` 和 `src/views/Plugins.vue` 已很大，包含大量 UI 与请求状态；新增独立功能时优先拆组件或 composable。
- 当前没有 ESLint/Prettier 配置，也没有前端单元测试框架；`vue-tsc` + 生产构建是最低验证门槛。
- API 类型集中在单个 `src/services/api.ts`；删除导出类型或方法前先用 `rg` 确认没有视图/组件引用。
- `fileList`、任务进度、轮询器和 AbortController 都是响应式状态，旧异步回调不得删除新任务的状态。
- guest 页面可能看到只读数据，但保存/删除/启停等操作必须同时有 UI 限制和后端管理员校验。

## ANTI-PATTERNS

- 不要在视图中直接复制认证头、错误状态处理或 FormData 组装。
- 不要只修改菜单而漏掉路由，或只加 `requiresAdmin` 而漏掉后端权限。
- 不要在 `v-html` 中渲染未经 DOMPurify 清洗的内容。
- 不要在组件卸载后继续 SSE、计时器、轮询或上传请求。
- 不要手工加入已经不存在的自动导入组件声明。
- 不要把新的大块功能继续塞入 `src/views/Maps.vue`/`src/views/Plugins.vue`，除非它与现有状态强耦合。

## VALIDATION

```bash
# 从仓库根目录执行
npm --prefix frontend run build

# 首次安装
npm --prefix frontend install
npm --prefix frontend run build
```

构建运行 `vue-tsc -b && vite build`，会清空并重建被 Git 忽略的 `backend/static/`。构建完成后检查 `git status`，确认 `src/components.d.ts` 仅有预期变化。
