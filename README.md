# l4d2-server-next

新一代求生之路 2 (L4D2) 服务器与 Web 管理后台。
提供 Docker 镜像与 Windows 原生程序，内置完整的整合包和大量优质插件，开箱即用！
支持地图管理、插件管理、服务器状态监控、RCON 控制台等功能，让服务器管理变得简单高效。

## ✨ 功能特性

*   **🖥️ 多平台支持**
    *   **Docker**: 提供完整的游戏服镜像（`l4d2-pure`）与管理器镜像（`l4d2-manager-next`），一键部署。
    *   **Windows**: 提供原生 `.exe` 管理器，无需 Docker 即可管理现有的 Windows 服务器。
    *   **Linux**: 支持管理宿主机直接运行的 L4D2 服务端。
*   **🗺️ 地图管理**
    *   支持 `.vpk` 及 `.zip/.rar/.7z` 压缩包拖拽上传。
    *   自动解压安装地图文件到正确目录。
    *   地图下载任务：支持后台下载地图文件到服务器。
    *   可视化地图列表，支持一键切换地图、修改游戏模式、修改难度。
*   **🔌 插件管理**
    *   Web 端查看已安装的所有 SourceMod 插件。
    *   支持在线上传插件文件。
    *   支持在线启用/禁用插件。
    *   **内置整合包**: 镜像中已包含 SourceMod、Metamod 以及大量热门实用插件，开箱即玩。
    *   在线插件商店：直接在 Web 端浏览、搜索、安装 SourceMod 插件。[插件商店](https://github.com/LaoYutang/l4d2-plugins-store)
*   **📊 服务器监控**
    *   实时仪表盘：显示 CPU、内存占用率。
    *   网络状态：监控服务器网络延迟与丢包率。
    *   游戏状态：显示当前地图、模式、难度、玩家数等。
    *   玩家列表：查看当前在线玩家、SteamID、连接时长、Ping 值。
*   **💻 RCON 控制台**
    *   Web 端直接发送 RCON 指令，无需登录游戏。
    *   支持快捷指令菜单。
    *   快捷操作：踢出玩家、封禁 SteamID、修改服务器参数。
*   **🛡️ 安全与权限**
    *   Web 后台密码保护，防止未授权访问。
    *   支持可视化配置服务器管理员（无需手动编辑 admins_simple.ini）。
    *   **GeoIP 访问控制**: 支持按地区（如“中国”）拦截非法 IP 访问管理后台，保障面板安全。
*   **💾 备份与恢复**
    *   支持一键备份当前服务器的插件配置、管理员列表、服务器信息与服务器设置。
    *   支持导入备份压缩包，批量恢复配置。
    *   查看备份详情：包括插件配置列表、管理员列表、服务器信息快照、服务器设置快照。
*   **🔑 授权码与自助服务**
    *   支持生成临时授权码，可配置过期时间，方便分享面板访问权限。
    *   支持自助服务模式：授权码持有者可在有限权限下使用面板。
*   **🎮 玩家游戏时长查询**
    *   通过 Steam API 查询玩家在 L4D2 的总游戏时长与实际游玩时长。
*   **📦 插件预设**
    *   支持预设插件方案，一键批量应用预配置的插件组合。
    *   通过 `preset.yaml` 管理预设配置。
*   **📝 插件配置管理**
    *   在线查看和编辑插件的 CVAR 配置项。
    *   显示每个 CVAR 的当前值、默认值和说明。

## 📸 预览截图

<div align="center">
  <img src="docs/images/首页.png" alt="首页" width="45%" />
  <img src="docs/images/系统信息与授权码.png" alt="系统信息与授权码" width="45%" />
  <br/>
  <img src="docs/images/插件管理.png" alt="插件管理" width="45%" />
  <img src="docs/images/插件配置.png" alt="插件配置" width="45%" />
  <br/>
  <img src="docs/images/rcon控制台.png" alt="rcon控制台" width="45%" />
  <img src="docs/images/性能监控.png" alt="性能监控" width="45%" />
  <br/>
  <img src="docs/images/地图管理.png" alt="地图管理" width="45%" />
  <img src="docs/images/地图下载.png" alt="地图下载" width="45%" />
</div>

---

## 🚀 Linux 部署

### 1. 完整部署 (游戏服务器 + 管理器)

适合从零开始搭建服务器的用户。

#### 方式一：一键脚本 (推荐) 【支持安装与管理】【多开/安装/删除/更新】
Cloudflare加速：
```sh
bash <(curl -sL l4d2-manage.laoyutang.cn)
```
官方源：
```sh
bash <(curl -sL https://raw.githubusercontent.com/LaoYutang/l4d2-server-next/master/manifest/install/manage.sh)
```

#### 方式二：Docker Compose

创建 `docker-compose.yaml`：

```yaml
volumes:
  l4d2-data:
  l4d2-plugins:
  l4d2-manager-data:

networks:
  l4d2-network:

services:
  # 游戏服务器
  l4d2:
    image: laoyutang/l4d2-pure:latest
    container_name: l4d2
    restart: unless-stopped
    ports:
      - "27015:27015"
      - "27015:27015/udp"
    volumes:
      - l4d2-data:/l4d2/left4dead2
      - /etc/localtime:/etc/localtime:ro # 同步宿主机时区
      - /etc/timezone:/etc/timezone:ro
    networks:
      - l4d2-network
    security_opt:
      - seccomp:unconfined # Docker 29.3+ 默认 seccomp 与 32 位 srcds 不兼容，低版本可省略此项
    environment:
      - L4D2_TICK=100 # 30,60,100,128
      - L4D2_VAC=false # false: 添加-insecure, true: 不添加
      - L4D2_PORT=27015
      - L4D2_RCON_PASSWORD=[rcon密码] # 请修改此处

  # 管理器
  l4d2-manager:
    image: laoyutang/l4d2-manager-next:latest
    container_name: l4d2-manager
    restart: unless-stopped
    ports:
      - "27020:27020"
    volumes:
      - l4d2-data:/left4dead2 # 与游戏服务器共享数据卷
      - l4d2-plugins:/plugins # 插件数据持久化
      - l4d2-manager-data:/data # 管理器运行数据持久化（key/监控/配置）
      - /proc:/host/proc:ro # 挂载宿主机进程信息用于监控
      - /etc/localtime:/etc/localtime:ro # 同步宿主机时区
      - /etc/timezone:/etc/timezone:ro
      - /var/run/docker.sock:/var/run/docker.sock # 允许管理器访问 Docker API（可选，若使用docker方式重启L4D2）
    environment:
      - L4D2_RESTART_BY_RCON=true
      - L4D2_MANAGER_PASSWORD=[web管理密码] # 请修改此处
      - L4D2_RCON_URL=l4d2:27015
      - L4D2_RCON_PASSWORD=[rcon密码] # 与上方保持一致
      - L4D2_GAME_PATH=/left4dead2
    networks:
      - l4d2-network
```
启动：
```sh
docker-compose up -d
```

> **关于 `security_opt` 配置**：Docker 29.3+ 版本的默认 seccomp profile 收紧了对部分 32 位 syscalls 的放行规则，导致 `srcds_linux` 无法正常启动。如果你使用的是 **Docker 29.3 以下版本**，可以删除上述 `security_opt` 字段。

### 2. 仅部署管理器 (Linux)

适合已有 L4D2 服务器（Docker 或宿主机部署），只需添加 Web 管理功能的用户。

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
  -e L4D2_MANAGER_PASSWORD=[web管理密码] \
  -e L4D2_GAME_PATH=/left4dead2 \
  -e L4D2_RCON_URL=127.0.0.1:27015 \
  -e L4D2_RCON_PASSWORD=[游戏服RCON密码] \
  -e L4D2_RESTART_BY_RCON=true \
  laoyutang/l4d2-manager-next:latest
```
注意修改 `/path/to/your/l4d2/left4dead2` 为实际的游戏目录。

---

## 💻 Windows 部署

适合在 Windows 机器上运行 L4D2 服务器的用户。

1. **下载管理器**：
   前往 [Releases](https://github.com/LaoYutang/l4d2-server-next/releases) 页面，下载最新的 `l4d2-manager-windows-amd64-vX.X.X.zip`。

2. **解压**：
   解压压缩包到任意目录（建议不要包含中文路径）。

3. **配置**：
   右键编辑 `start_manager.bat` 文件，修改以下配置：
   *   `L4D2_MANAGER_PASSWORD`: 设置 Web 管理后台密码。
   *   `L4D2_GAME_PATH`: 设置 L4D2 游戏目录（例如 `D:\SteamCMD\steamapps\common\Left 4 Dead 2 Dedicated Server\left4dead2`）。
   *   `L4D2_RCON_URL`: 游戏服务器 IP:端口（通常是 `127.0.0.1:27015`）。
   *   `L4D2_RCON_PASSWORD`: 游戏服务器的 RCON 密码（需在 `server.cfg` 中配置 `rcon_password`）。

4. **启动**：
   双击运行 `start_manager.bat`。

5. **访问**：
   打开浏览器访问 `http://localhost:27020`（或服务器 IP:27020）。

---

## ⚙️ 环境变量说明

| 变量名                    | 描述                                  | 默认值/必填                     |
| :------------------------ | :------------------------------------ | :------------------------------ |
| **L4D2_MANAGER_PASSWORD** | Web 管理后台登录密码                  | **必填**                        |
| **L4D2_GAME_PATH**        | L4D2 游戏目录路径 (left4dead2 文件夹) | **必填**                        |
| **L4D2_RCON_URL**         | RCON 地址 (IP:Port)                   | 推荐配置，否则无法切图/看状态   |
| **L4D2_RCON_PASSWORD**    | RCON 密码                             | 推荐配置                        |
| **L4D2_VAC**              | 游戏服是否启用 VAC                    | `false`（默认添加 `-insecure`） |
| **L4D2_RESTART_BY_RCON**  | 是否通过 RCON 命令重启服务器          | `false` (推荐 `true`)           |
| **L4D2_HISTORY_METRICS**  | 是否开启历史性能监控 (需持久化数据)   | `false`                         |
| **STEAM_API_KEY**         | Steam API Key (用于查询玩家时长)      | 可选                            |
| **REGION_WHITE_LIST**     | GeoIP 区域白名单 (如: 中国)           | 可选 (不填则不拦截)             |
| **L4D2_MANAGER_PORT**     | 管理器监听端口                        | `27020`                         |

---

## 🔌 插件 ZIP 格式规范

Web 面板的“上传插件”功能只接受 `.zip` 文件。ZIP 内部必须按 L4D2 的 `left4dead2` 游戏目录组织文件，支持单插件包和多插件包两种格式。

### 单插件 ZIP

如果压缩包内的根目录就是 `left4dead2/`，会被识别为一个插件。插件名称取自 ZIP 文件名（去掉 `.zip` 后缀）。

```text
MyPlugin.zip
├── README.md
└── left4dead2/
    ├── addons/sourcemod/plugins/my_plugin.smx
    ├── addons/sourcemod/gamedata/my_plugin.txt
    └── cfg/sourcemod/my_plugin.cfg
```

### 多插件 ZIP

如果一个 ZIP 内包含多个插件，每个一级目录就是一个插件名称，并且每个插件目录下应包含 `left4dead2/` 目录。

```text
plugins-bundle.zip
├── PluginA/
│   ├── README.md
│   └── left4dead2/
│       └── addons/sourcemod/plugins/plugin_a.smx
└── PluginB/
    ├── readme.md
    └── left4dead2/
        ├── addons/sourcemod/plugins/plugin_b.smx
        └── cfg/sourcemod/plugin_b.cfg
```

插件根目录可以放 `.md` 说明文件，推荐命名为 `README.md` 或 `readme.md`，面板会将它作为插件详情显示；如果存在多个 `.md` 文件，会优先使用 `README.md`。

常见文件路径示例：

```text
left4dead2/addons/sourcemod/plugins/*.smx
left4dead2/addons/sourcemod/configs/
left4dead2/addons/sourcemod/gamedata/
left4dead2/addons/sourcemod/translations/
left4dead2/cfg/sourcemod/*.cfg
```

放在 `left4dead2/cfg/sourcemod/` 下的 `.cfg` 文件会随插件启用复制到服务器，并在“插件配置”中作为可编辑配置项显示（可识别其中的 SourceMod CVAR 配置）。

注意事项：

*   不要在 ZIP 根目录放 `readme.txt` 等零散文件；单插件 ZIP 根目录只放 `left4dead2/` 和可选 `.md` 说明文件，多插件 ZIP 根目录只放插件文件夹。
*   单插件 ZIP 不要再套一层插件名目录，除非你要使用多插件 ZIP 格式。
*   插件名称不能和现有插件重复：单插件取 ZIP 文件名，多插件取一级目录名。
*   `__MACOSX/` 和 `.DS_Store` 会被自动忽略，中文文件名支持 UTF-8/GBK 编码。
