# l4d2-server
求生2服务器快速启动

完整的求生之路2服务端镜像，内置了完整的豆瓣整合包和大量优质插件，开箱即用！
管理界面，不再需要登录服务器传图重启，页面操作即可！支持切图、下载任务与服务器状态显示。

> 新版本的docker修改了sock的api版本，使用docker容器方式重启l4d2需要docker版本<=28，下方部署方式已经全部修改为RCON方式，以兼容所有版本

## 一键部署
需要docker与docker compose环境，需要能够拉取镜像源，国内需要配置镜像源或代理。
```sh
bash <(curl -sL https://raw.githubusercontent.com/LaoYutang/l4d2-server-next/master/manifest/install/install.sh)
```
服务器无法连接github也可以下载脚本，手动上传都服务器运行。或者使用github加速，如
```sh
bash <(curl -sL https://gh.dpik.top/https://raw.githubusercontent.com/LaoYutang/l4d2-server-next/master/manifest/install/install.sh)
```
镜像中带有完整的游戏服务端，需要下载5.XGB的数据，所以安装时间取决于服务器的带宽和cpu性能。

## 手动部署
稳定版使用lastest标签，如果L4D2有更新，可以尝试使用nightly标签，该镜像每晚打包。
```sh
docker volume create l4d2-data
docker run -d \
  --name l4d2 \
  --restart unless-stopped \
  -p 27015:27015 \
  -p 27015:27015/udp \
  -v l4d2-data:/l4d2/left4dead2 \
  -e L4D2_TICK=60 \
  -e L4D2_RCON_PASSWORD=rcon密码 \
  laoyutang/l4d2-pure:latest

# 地图管理器，可选
docker run -d \
  --name l4d2-manager \
  --restart unless-stopped \
  -p 27020:27020 \
  -v l4d2-data:/left4dead2 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e L4D2_RESTART_BY_RCON=true \
  -e L4D2_MANAGER_PASSWORD=设置上传地图的密码 \
  -e L4D2_RCON_URL=localhost:27015 \
  -e L4D2_RCON_PASSWORD=rcon密码 \
  -e L4D2_GAME_PATH=/left4dead2 \
  laoyutang/l4d2-manager-next:latest
```
docker-compose启动
```yaml
# docker-compose.yaml
volumes:
  l4d2-data:

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
    networks:
      - l4d2-network
    environment:
      - L4D2_TICK=60 # 30,60,100
      - L4D2_RCON_PASSWORD=[rcon密码]

  # 地图管理器，可选
  l4d2-manager:
    image: laoyutang/l4d2-manager-next:latest
    container_name: l4d2-manager
    restart: unless-stopped
    ports:
      - "27020:27020"
    volumes:
      - l4d2-data:/left4dead2
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - L4D2_RESTART_BY_RCON=true
      - L4D2_MANAGER_PASSWORD=[web管理密码]
      - L4D2_RCON_URL=l4d2:27015
      - L4D2_RCON_PASSWORD=[rcon密码]
      - L4D2_GAME_PATH=/left4dead2
    networks:
      - l4d2-network
```

## 环境变量
### L4D2
- L4D2_TICK: 游戏 tickrate，可选，默认为 60
- L4D2_RCON_PASSWORD: RCON 密码，必填
- L4D2_PORT: 游戏端口，可选，默认为 27015
### L4D2-MANAGER
- L4D2_MANAGER_PASSWORD: 管理器密码，必填
- L4D2_RCON_URL: RCON 地址，可选，否则不支持状态获取与切图功能
- L4D2_RCON_PASSWORD: RCON 密码，可选，否则不支持状态获取与切图功能
- L4D2_GAME_PATH: left4dead2 游戏目录路径
- L4D2_RESTART_BY_RCON: 是否通过RCON命令重启服务器，默认false
- L4D2_RESTART_CMD: 重启命令，可选，默认使用docker重启
- L4D2_CONTAINER_NAME: 需要重启的docker容器名称，可选，未设置L4D2_RESTART_CMD时有效，默认为"l4d2"
- STEAM_API_KEY：Steam API 密钥，可选，用于查询玩家游戏时长，可以在[steam](https://steamcommunity.com/dev/apikey)获取

## 地图管理
### 手动操作
1. docker volume目录下操作即可  ```/var/lib/docker/volume/l4d2-data/_data/addons``` 
2. 重启服务器```docker restart l4d2```
3. 进入服务器后管理员切图
### 使用地图管理器（推荐）
1. 浏览器登录```ip:27020```
2. 选择地图vpk文件或者zip/rar/7z文件（可多选）
3. 点击上传
4. 上传后点击重启服务器以重新加载地图
5. 点击查看以加载地图，切换对应的地图即可

## 插件修改与替换
插件目录为 ```/var/lib/docker/volume/l4d2-data/_data/addons``` 
配置目录为 ```/var/lib/docker/volume/l4d2-data/_data/cfg```
可以自行按需修改替换

## 管理员设置
进入服务器后，输入```!root 管理员密码```即可在线添加删除管理员
***注意：管理员密码在```addons/sourcemod/configs/l4d2_admins_simple.cfg```中设置，请及时修改默认密码，重启生效***

## windows服务器管理器使用说明
windows服务器可以自行下载服务器启动，使用编译好的l4d2-manager.exe和static文件夹，设置好环境变量启动即可！
***注意***: 非docker启动的l4d2服务器，重启功能需要自行配置环境变量```L4D2_RESTART_BY_RCON``` 或者```L4D2_RESTART_CMD```与```L4D2_GAME_PATH```，重启脚本可参考项目目录下的```restart.dat```。


## 自行打包docker镜像
```docker build -f manifest/docker/l4d2.Dockerfile -t l4d2 .```
```docker build -f manifest/docker/manager.Dockerfile -t l4d2-manager .```
