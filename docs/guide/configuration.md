# 配置项说明

管理器主要通过环境变量配置。Docker 部署写在 `docker-compose.yaml` 或 `docker run -e` 中，Windows 部署写在 `start_manager.bat` 中。

## 管理器变量

| 变量名 | 说明 | 默认值 |
| --- | --- | --- |
| `L4D2_MANAGER_PASSWORD` | Web 管理后台管理员密码。生产环境必须设置强密码。 | `laoyutangnb` |
| `L4D2_MANAGER_PORT` | 管理器监听端口。 | `27020` |
| `L4D2_GAME_PATH` | L4D2 的 `left4dead2` 游戏目录。 | `/left4dead2` |
| `L4D2_RCON_URL` | RCON 地址，格式为 `IP:端口` 或 `容器名:端口`。 | 空 |
| `L4D2_RCON_PASSWORD` | RCON 密码，需要和 `server.cfg` 一致。 | 空 |
| `L4D2_RESTART_BY_RCON` | 重启服务器时是否优先使用 RCON。推荐设为 `true`。 | `false` |
| `L4D2_RESTART_CMD` | 不使用 RCON 重启时执行的自定义命令。 | 空 |
| `L4D2_CONTAINER_NAME` | 未配置重启命令时用于重启的 Docker 容器名。 | `l4d2` |
| `L4D2_PLUGIN_STORE_PATH` | 插件库数据目录。 | `./plugins` |
| `STEAM_API_KEY` | 首页玩家列表中查询 Steam 账号的 L4D2 游戏时长。 | 空 |
| `REGION_WHITE_LIST` | GeoIP 区域白名单，例如 `中国`。为空时不拦截地区。 | 空 |

## 游戏服镜像变量

| 变量名 | 说明 | 默认值 |
| --- | --- | --- |
| `L4D2_TICK` | 游戏服 Tickrate，支持 `30`、`60`、`100`、`128`。 | `30` |
| `L4D2_VAC` | 是否启用 VAC。`false` 会添加 `-insecure`。 | `false` |
| `L4D2_PORT` | 游戏服监听端口。 | `27015` |
| `L4D2_RCON_PASSWORD` | 写入游戏服 `server.cfg` 的 RCON 密码。 | `laoyutangnb!` |

## 数据目录

管理器默认把运行数据放在 `./data`，Docker 部署中通常映射为 `/data` 卷。该目录包含：

- `private.key`：临时授权码和自助授权码的签名密钥。
- `manager_config.json`：自助授权、玩家统计、监控历史、地图精简等开关。
- `monitor.db`：性能监控历史数据。
- `player_stats.db`：玩家在线统计数据。

这些数据不属于游戏目录。迁移管理器时，如果希望保留统计、授权码状态和历史监控，请保留该目录。

## 权限模式

后台有两类登录身份：

| 身份 | 来源 | 权限 |
| --- | --- | --- |
| 管理员 | `L4D2_MANAGER_PASSWORD` | 可进行上传、删除、启停、保存配置、生成授权码等写操作。 |
| 访客 | 临时授权码或自助授权码 | 以查看为主，部分敏感页面和写操作会被隐藏或禁用。 |

临时授权码由管理员在“系统管理”中生成；自助授权码需要管理员先开启自助服务，开启后访客可按冷却限制自行申请短期访问码。

## GeoIP 白名单

设置 `REGION_WHITE_LIST` 后，管理器会按 IP 归属地限制访问后台。例如：

```sh
REGION_WHITE_LIST=中国
```

该能力依赖后端随包提供的 `ip2region_*.xdb` 数据文件。误配白名单可能导致自己无法访问后台，首次启用建议保留 SSH 或服务器控制台入口。
