import { defineConfig } from 'vitepress';

export default defineConfig({
  lang: 'zh-CN',
  title: 'L4D2 Server Next',
  description: 'Left 4 Dead 2 服务器与 Web 管理后台使用文档',
  cleanUrls: true,
  lastUpdated: true,
  head: [['meta', { name: 'theme-color', content: '#2563eb' }]],
  themeConfig: {
    logo: '/logo.png',
    nav: [
      { text: '快速开始', link: '/guide/quick-start' },
      { text: '部署配置', link: '/guide/linux' },
      { text: '功能指南', link: '/features/dashboard' },
      { text: '运维手册', link: '/operations/plugin-package' },
      { text: 'GitHub', link: 'https://github.com/LaoYutang/l4d2-server-next' }
    ],
    sidebar: [
      {
        text: '开始使用',
        items: [
          { text: '项目介绍', link: '/' },
          { text: '快速开始', link: '/guide/quick-start' },
          { text: 'Linux 部署', link: '/guide/linux' },
          { text: 'Windows 部署', link: '/guide/windows' },
          { text: '配置项说明', link: '/guide/configuration' }
        ]
      },
      {
        text: '功能指南',
        items: [
          { text: '服务器状态', link: '/features/dashboard' },
          { text: '地图管理', link: '/features/maps' },
          { text: '插件管理', link: '/features/plugins' },
          { text: '性能监控', link: '/features/monitor' },
          { text: '玩家统计', link: '/features/player-stats' },
          { text: '管理员设置', link: '/features/admins' },
          { text: 'RCON 控制台', link: '/features/rcon' },
          { text: '服务器信息', link: '/features/server-info' },
          { text: '服务器配置', link: '/features/server-config' },
          { text: '备份管理', link: '/features/backup' },
          { text: '日志查看', link: '/features/logs' },
          { text: '系统管理', link: '/features/system' }
        ]
      },
      {
        text: '运维手册',
        items: [
          { text: '插件 ZIP 规范', link: '/operations/plugin-package' },
          { text: '地图资源精简', link: '/operations/map-trim' },
          { text: '备份迁移流程', link: '/operations/migration' },
          { text: '常见问题排查', link: '/operations/troubleshooting' },
          { text: '构建与开发', link: '/development/build' }
        ]
      }
    ],
    outline: {
      label: '本页目录',
      level: [2, 3]
    },
    docFooter: {
      prev: '上一页',
      next: '下一页'
    },
    lastUpdated: {
      text: '最后更新',
      formatOptions: {
        dateStyle: 'medium',
        timeStyle: 'short'
      }
    },
    search: {
      provider: 'local',
      options: {
        translations: {
          button: {
            buttonText: '搜索文档',
            buttonAriaLabel: '搜索文档'
          },
          modal: {
            noResultsText: '没有找到结果',
            resetButtonTitle: '清空搜索',
            footer: {
              selectText: '选择',
              navigateText: '切换',
              closeText: '关闭'
            }
          }
        }
      }
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/LaoYutang/l4d2-server-next' }
    ]
  }
});
