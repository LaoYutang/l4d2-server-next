# RCON 控制台

“RCON 控制台”用于从 Web 面板直接向 L4D2 服务端发送 RCON 指令，适合临时排查、查看状态和执行一次性管理操作。

![rcon](https://images.laoyutang.cn/2026/06/b52312a76778ddce4fc57a4d09622e88.png)

## 使用前提

RCON 控制台依赖管理器环境变量：

- `L4D2_RCON_URL`：游戏服 RCON 地址，例如 `127.0.0.1:27015` 或 Docker 网络中的 `l4d2:27015`。
- `L4D2_RCON_PASSWORD`：游戏服 RCON 密码。

游戏服 `server.cfg` 中也需要设置相同密码：

```text
rcon_password "你的RCON密码"
```

如果首页状态、切图、踢人等功能也无法使用，通常优先检查这两项配置。

## 常用指令

页面内置常用指令参考，常见命令包括：

```text
status
sm plugins list
sm exts list
sm_reloadadmins
sm_cvar z_difficulty Hard
changelevel c1m1_hotel
kickid 3
banid 0 STEAM_1:1:123456 kick
```

## 适合的场景

RCON 控制台适合：

- 查看服务器当前状态。
- 查询 SourceMod 插件和扩展加载状态。
- 临时修改 CVAR。
- 手动刷新管理员权限。
- 临时切换地图。
- 踢出或封禁玩家。

对需要长期保留的配置，不建议只在 RCON 控制台临时执行；请写入“服务器配置”、插件配置文件或 `server.cfg`。

## 输出日志

RCON 控制台会保留当前页面会话中的命令输出，刷新页面后清空。需要排查持续运行问题时，可以配合 [日志查看](/features/logs) 观察 SourceMod 日志。
