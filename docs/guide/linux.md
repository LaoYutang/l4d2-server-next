# Linux 部署

Linux 下推荐优先使用一键脚本。需要完全掌控配置时，可以使用 Docker Compose；已有 L4D2 服务端时，只运行管理器容器即可。

## 完整部署：一键脚本

适合新机器或多实例场景。脚本位于 `manifest/install/manage.sh`，支持安装、管理、更新和删除实例。

```sh
bash <(curl -sL l4d2-manage.laoyutang.cn)
```

也可以使用 GitHub 官方源：

```sh
bash <(curl -sL https://raw.githubusercontent.com/LaoYutang/l4d2-server-next/master/manifest/install/manage.sh)
```

脚本会生成 Docker Compose 配置，并持久化游戏数据、插件目录和管理器数据。

脚本默认从 `docker.cnb.cool` 拉取项目镜像；已有镜像源配置仍会继续使用。如需改用 Docker Hub 官方源，可以在“镜像管理 → 设置镜像加速源”中留空并保存。

安装完成后，请在系统防火墙和云服务器安全组中放行你在脚本中填写的游戏端口和管理后台端口。游戏端口需要 TCP/UDP，管理后台端口需要 TCP。

## 完整部署：Docker Compose

创建 `docker-compose.yaml`：

```yaml
volumes:
  l4d2-data:
  l4d2-plugins:
  l4d2-manager-data:

networks:
  l4d2-network:

services:
  l4d2:
    image: laoyutang/l4d2-pure:latest
    container_name: l4d2
    restart: unless-stopped
    ports:
      - "27015:27015"
      - "27015:27015/udp"
    volumes:
      - l4d2-data:/l4d2/left4dead2
      - /etc/localtime:/etc/localtime:ro
      - /etc/timezone:/etc/timezone:ro
    networks:
      - l4d2-network
    security_opt:
      - seccomp:unconfined
    environment:
      - L4D2_TICK=100
      - L4D2_VAC=false
      - L4D2_PORT=27015
      - L4D2_RCON_PASSWORD=请修改为强密码

  l4d2-manager:
    image: laoyutang/l4d2-manager-next:latest
    container_name: l4d2-manager
    restart: unless-stopped
    ports:
      - "27020:27020"
    volumes:
      - l4d2-data:/left4dead2
      - l4d2-plugins:/plugins
      - l4d2-manager-data:/data
      - /proc:/host/proc:ro
      - /etc/localtime:/etc/localtime:ro
      - /etc/timezone:/etc/timezone:ro
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - L4D2_RESTART_BY_RCON=true
      - L4D2_MANAGER_PASSWORD=请修改为强密码
      - L4D2_RCON_URL=l4d2:27015
      - L4D2_RCON_PASSWORD=请与游戏服保持一致
      - L4D2_GAME_PATH=/left4dead2
    networks:
      - l4d2-network
```

启动：

```sh
docker compose up -d
```

Docker 29.3+ 默认 seccomp 配置会影响 32 位 `srcds_linux`，因此上面的游戏服容器保留了 `security_opt: seccomp:unconfined`。如果你确定使用的是更早版本 Docker，可以按实际情况移除。

## 更新游戏服务端

Docker 部署可以直接在游戏服容器中使用 SteamCMD 更新服务端文件：

```sh
docker exec l4d2 bash -lc 'cd /root/steamcmd && ./steamcmd.sh +@sSteamCmdForcePlatformType linux +force_install_dir /l4d2 +login anonymous +app_update 222860 validate +quit'
```

如果你的游戏服容器名不是 `l4d2`，请把命令中的 `l4d2` 替换为实际容器名。更新完成后建议重启游戏服容器：

```sh
docker restart l4d2
```

## 防火墙和安全组

公网服务器需要同时检查系统防火墙和云厂商安全组。以默认建议端口为例：

```sh
# ufw 示例
ufw allow 27015/tcp
ufw allow 27015/udp
ufw allow 27020/tcp
```

如果你使用 `firewalld`：

```sh
firewall-cmd --permanent --add-port=27015/tcp
firewall-cmd --permanent --add-port=27015/udp
firewall-cmd --permanent --add-port=27020/tcp
firewall-cmd --reload
```

如果脚本或 Compose 中使用了其他端口，请把示例中的 `27015` 和 `27020` 换成你的实际端口。多实例部署时，每个实例的游戏端口和管理端口都需要分别放行。

## 仅部署管理器 Linux

适合已经有游戏服务器，只需要 Web 面板接管地图、插件、RCON 和配置文件的情况。

```sh
docker run -d \
  --name l4d2-manager \
  --restart unless-stopped \
  --net host \
  -v /path/to/your/l4d2/left4dead2:/left4dead2 \
  -v l4d2-plugins:/plugins \
  -v l4d2-manager-data:/data \
  -v /proc:/host/proc:ro \
  -v /etc/localtime:/etc/localtime:ro \
  -v /etc/timezone:/etc/timezone:ro \
  -e L4D2_MANAGER_PORT=27020 \
  -e L4D2_MANAGER_PASSWORD=请修改为强密码 \
  -e L4D2_GAME_PATH=/left4dead2 \
  -e L4D2_RCON_URL=127.0.0.1:27015 \
  -e L4D2_RCON_PASSWORD=游戏服RCON密码 \
  -e L4D2_RESTART_BY_RCON=true \
  laoyutang/l4d2-manager-next:latest
```

把 `/path/to/your/l4d2/left4dead2` 替换成真实的 `left4dead2` 目录。该目录下应能看到 `addons/`、`cfg/`、`maps/` 等游戏目录。

## RCON 配置检查

管理器依赖 RCON 实现状态读取、切图、踢人、封禁、难度和模式切换。请确保游戏服 `server.cfg` 中存在：

```text
rcon_password "你的RCON密码"
```

并且管理器的 `L4D2_RCON_URL` 和 `L4D2_RCON_PASSWORD` 与游戏服一致。
