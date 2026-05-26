# MANIFEST KNOWLEDGE BASE

**Scope**: Docker、CI/CD、安装脚本、启动模板

## OVERVIEW
构建与部署资源配置。包含多阶段 Dockerfile、GitHub Actions 工作流、Linux 一键安装/多实例管理脚本、Windows 启动脚本。

## STRUCTURE
```
manifest/
├── docker/
│   ├── manager.Dockerfile     # 管理器镜像：Node 24 → Go 1.25 → Docker CLI
│   ├── l4d2.Dockerfile        # 游戏服镜像：Ubuntu 22.04 + SteamCMD
│   └── start.sh               # 游戏服启动脚本
├── install/
│   ├── install.sh             # 旧版一键安装（单实例）
│   └── manage.sh              # 新版多实例管理器（1400+ 行）
└── boot/
    └── start_manager.bat      # Windows 启动脚本（自动重启循环）
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| 管理器镜像构建 | docker/manager.Dockerfile | 三阶段构建，最终基于 docker:29-cli-alpine |
| 游戏服镜像构建 | docker/l4d2.Dockerfile | 需 security_opt: seccomp:unconfined (Docker 29.3+) |
| CI/CD 配置 | .github/workflows/ | v* 标签触发 manager/windows，手动触发 l4d2 |
| Linux 多实例管理 | install/manage.sh | 动态生成 docker-compose，端口冲突检测 |
| Windows 启动 | boot/start_manager.bat | 环境变量 + 自动重启 loop |
| 游戏服启动参数 | docker/start.sh | 根据 L4D2_TICK 选择 server.cfg 模板 |

## CONVENTIONS
- manager.Dockerfile 最终镜像基于 `docker:29.1.1-cli-alpine3.22`（支持 Docker API 重启容器）
- 构建时执行 `find ./plugins -exec touch -t 202001010000 {} +` 保证缓存确定性
- Go 版本号通过 `-ldflags "-X l4d2-manager-next/consts.Version=${VERSION}"` 注入
- Windows 发布工作流先构建前端 static/，再交叉编译后端

## ANTI-PATTERNS
- 不要修改插件目录时间戳相关的构建步骤，避免破坏缓存优化
- 修改 Dockerfile 后需同步检查三个 workflow 文件
- l4d2-image.yml 为手动触发，推送 v* 标签不会自动构建游戏服镜像
