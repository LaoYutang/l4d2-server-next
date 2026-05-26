# FRONTEND KNOWLEDGE BASE

**Scope**: Vue 3 + TypeScript + Vite frontend for l4d2-server-next

## OVERVIEW
Vue 3.5 前端管理面板，使用 Vite(Rolldown)、Pinia、Vue Router、Ant Design Vue 4.2、Tailwind CSS 4.1、ECharts 6.0。

## STRUCTURE
```
frontend/src/
├── main.ts              # 应用入口
├── App.vue              # 根组件 + ConfigProvider
├── router/index.ts      # 11 条路由，hash 模式，懒加载
├── services/api.ts      # 统一 ApiService (1036 行)
├── stores/              # 3 个 Pinia store
│   ├── auth.ts          # 认证状态
│   ├── monitor.ts       # 监控数据 + ECharts
│   └── theme.ts         # 暗黑/亮色主题
├── views/               # 11 个页面组件
│   ├── Home.vue         # 仪表盘
│   ├── Plugins.vue      # 插件管理 (1666 行)
│   ├── Maps.vue         # 地图管理 (1026 行)
│   ├── Monitor.vue      # 性能监控
│   ├── Backup.vue       # 备份恢复
│   └── ...              # 其他页面
├── components/          # 6 个复用弹窗
│   ├── BackupModal.vue
│   ├── MapSelectorModal.vue
│   ├── PluginConfigModal.vue
│   └── ...
├── utils/
│   ├── statusParser.ts  # RCON 状态解析
│   ├── gameConstants.ts # 难度/模式映射
│   └── clipboard.ts     # 剪贴板工具
└── data/
    └── officialMaps.ts  # 14 个战役、53 个章节
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| 新增页面 | views/ + router/index.ts | 懒加载除 Login/Home 外所有页面 |
| API 调用 | services/api.ts | 统一封装，401/403/429 自动登出 |
| 认证状态 | stores/auth.ts | admin/guest 双角色 |
| 监控图表 | stores/monitor.ts + views/Monitor.vue | ECharts 实时/历史数据 |
| 主题切换 | stores/theme.ts | localStorage + 系统偏好 |
| 弹窗组件 | components/ | v-model:open 模式 |

## CONVENTIONS
- Vue SFC + `<script setup>` + TypeScript
- 状态管理用 Pinia Composition API 风格
- API 统一走 ApiService，禁止视图内直接 fetch（除特殊需求）
- Ant Design 组件通过 `unplugin-vue-components` 自动导入
- 样式：Tailwind 工具类 + Ant Design 主题 token

## ANTI-PATTERNS
- 视图文件过大：Plugins.vue (1666 行)、Maps.vue (1026 行) 考虑进一步拆分
- 视图混合 UI 逻辑与 API 调用，无独立 composables
- 无 ESLint/Prettier 配置，需手动保持代码风格一致
- 无前端测试框架
