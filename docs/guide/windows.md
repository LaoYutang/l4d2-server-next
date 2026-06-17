# Windows 部署

Windows 部署适合本机或 Windows 服务器上已有 L4D2 Dedicated Server 的用户。管理器以原生 `.exe` 运行，通过文件路径和 RCON 接管服务端。

## 下载与解压

1. 打开 [Releases](https://github.com/LaoYutang/l4d2-server-next/releases)。
2. 下载 `l4d2-manager-windows-amd64-vX.X.X.zip`。
3. 解压到一个固定目录，建议路径只包含英文、数字和常见符号。

## 编辑启动脚本

右键编辑 `start_manager.bat`，至少修改这些变量：

```bat
set L4D2_MANAGER_PORT=27020
set L4D2_MANAGER_PASSWORD=请修改为强密码
set L4D2_GAME_PATH=D:\SteamCMD\steamapps\common\Left 4 Dead 2 Dedicated Server\left4dead2
set L4D2_RCON_URL=127.0.0.1:27015
set L4D2_RCON_PASSWORD=游戏服RCON密码
set L4D2_RESTART_BY_RCON=true
set STEAM_API_KEY=
```

`L4D2_GAME_PATH` 必须指向 `left4dead2` 游戏目录本身，而不是上一级安装目录。

## 游戏服准备

在服务端 `left4dead2/cfg/server.cfg` 中设置同样的 RCON 密码：

```text
rcon_password "游戏服RCON密码"
```

启动游戏服务器后，再双击 `start_manager.bat` 启动管理器。浏览器访问：

```text
http://localhost:27020
```

如果从另一台电脑访问，把 `localhost` 换成 Windows 服务器的局域网或公网 IP，并确认防火墙放行了 `27020/tcp`。

## 开放防火墙

如果 Windows 服务器需要被其他电脑访问，请在 Windows 防火墙中放行实际使用的端口：

- 游戏端口：TCP 和 UDP，例如 `27015/tcp`、`27015/udp`。
- 管理后台端口：TCP，例如 `27020/tcp`。

如果你修改了 `L4D2_MANAGER_PORT` 或游戏服端口，请按修改后的端口放行。云服务器还需要在云厂商安全组中放行同样端口。

## 运行数据

Windows 包会在管理器运行目录下使用 `data/` 保存登录密钥、配置、监控数据库和玩家统计数据库。迁移管理器时，建议连同该目录一起保留。
