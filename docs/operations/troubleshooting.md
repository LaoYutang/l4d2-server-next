# 常见问题排查

这页按症状列出常见排查路径。

## 管理后台打不开

检查：

- 管理器进程或容器是否运行。
- `L4D2_MANAGER_PORT` 是否和访问端口一致。
- 防火墙或云安全组是否放行 `27020/tcp`。
- “访问控制”中的面板黑白名单是否允许当前 IP。
- 使用反向代理时，代理 IP/CIDR 是否已加入可信代理，页面诊断的最终客户端 IP 是否正确。

Docker 查看日志：

```sh
docker logs l4d2-manager
```

如果 `data/access_control.json` 损坏，管理器会进入恢复模式，只允许直接来自 `127.0.0.0/8` 或 `::1` 的请求。可在服务器本机登录后保存有效配置，或停止管理器后修复该文件。

## 登录提示密码错误或被锁定

管理器使用 Bearer 认证，密码来自 `L4D2_MANAGER_PASSWORD`。连续输错过多会临时锁定当前 IP。

处理方式：

- 确认环境变量或 `start_manager.bat` 中的密码。
- 等待锁定时间结束。
- 重启管理器会清空内存中的错误计数。

## 首页没有服务器状态

优先检查 RCON：

- 游戏服是否启动。
- `server.cfg` 是否设置了 `rcon_password`。
- `L4D2_RCON_URL` 是否可从管理器访问。
- `L4D2_RCON_PASSWORD` 是否和游戏服一致。

可以在 RCON 控制台执行：

```text
status
```

## Docker 游戏服启动失败

Docker 29.3+ 下，如果日志中出现 32 位程序相关的系统调用问题，请确认游戏服容器保留了：

```yaml
security_opt:
  - seccomp:unconfined
```

同时确认数据卷没有被错误地挂载为空目录，导致 `srcds_run` 或游戏文件缺失。

## 插件启用后没有生效

检查：

- SourceMod 和 Metamod 是否正确启用。
- 插件 ZIP 目录是否符合 [插件 ZIP 规范](/operations/plugin-package)。
- 插件是否依赖额外扩展、gamedata 或 translations。
- SourceMod 错误日志中是否有加载失败记录。
- 是否需要换图或重启服务器。

常用 RCON：

```text
sm plugins list
sm exts list
```

## 地图上传失败

检查：

- 文件格式是否为 `.vpk`、`.zip`、`.rar` 或 `.7z`。
- 浏览器到服务器的网络是否稳定。
- 管理器容器或进程是否有写入 `L4D2_GAME_PATH` 的权限。
- 磁盘空间是否足够。

大文件上传会分片进行，失败后可以尝试重新上传，已上传分片会尽量复用。

## 首页玩家游戏时长无法查询

首页玩家列表中的 L4D2 游戏时长来自 Steam API，和“玩家统计”页面的服务器在线时长不同。无法查询时检查：

- 设置 `STEAM_API_KEY`。
- Steam API 可访问。
- 玩家 SteamID 可转换为 Steam64。
- 玩家资料或游戏时长隐私允许查询。

“玩家统计”页面的服务器在线时长来自本地采样数据库，不需要 `STEAM_API_KEY`。
