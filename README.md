# l4d2-server-next

[![Release](https://img.shields.io/github/v/release/LaoYutang/l4d2-server-next?label=release)](https://github.com/LaoYutang/l4d2-server-next/releases)
[![Manager image size](https://img.shields.io/docker/image-size/laoyutang/l4d2-manager-next/latest?label=manager%20image)](https://hub.docker.com/r/laoyutang/l4d2-manager-next)
[![L4D2 image size](https://img.shields.io/docker/image-size/laoyutang/l4d2-pure/latest?label=L4D2%20image)](https://hub.docker.com/r/laoyutang/l4d2-pure)
[![License](https://img.shields.io/github/license/LaoYutang/l4d2-server-next?label=license)](LICENSE)

新一代 Left 4 Dead 2 服务器与 Web 管理后台。

项目提供 Linux Docker 镜像、Windows 原生管理器、内置 SourceMod/Metamod 插件资源和完整 Web 面板，覆盖地图管理、插件管理、服务器状态、玩家统计、性能监控、RCON 控制台、日志、备份恢复和授权码访问等日常运维场景。

## 文档

[https://l4d2-manage-docs.laoyutang.cn/](https://l4d2-manage-docs.laoyutang.cn/)


## 快速部署

Linux 一键脚本：

```sh
bash <(curl -sL l4d2-manage.laoyutang.cn)
```

官方源：

```sh
bash <(curl -sL https://raw.githubusercontent.com/LaoYutang/l4d2-server-next/master/manifest/install/manage.sh)
```

安装完成后访问：

```text
http://服务器IP:27020
```

## 主要能力

- 多平台部署：Linux Docker、Docker Compose、Windows 原生管理器、接管已有服务器。
- 地图管理：上传、下载、工坊/合集解析、任务队列、详情解析、切图、重命名、删除、资源精简、BSP 字典与全局脚本风险检查。
- 插件管理：上传 ZIP、启用/禁用、立即加载/卸载、批量操作、插件商店、预设、CVAR 配置、导出。
- 服务器运维：状态首页、在线玩家操作、RCON 终端、服务器信息、`server.cfg` 常用配置。
- 统计与监控：CPU、内存、网络、磁盘实时监控，历史趋势，玩家在线统计和 Steam 游戏时长查询。
- 安全与迁移：管理员密码、临时授权码、自助授权、面板黑白名单、可信代理、备份导入导出、插件迁移。

## 预览截图

<div align="center">
  <img src="https://images.laoyutang.cn/2026/06/99465f674d128e3a406d294753147479.png" alt="首页" width="45%" />
  <img src="https://images.laoyutang.cn/2026/06/3dbe0462012ca01b6971e6744655da95.png" alt="插件管理" width="45%" />
  <br/>
  <img src="https://images.laoyutang.cn/2026/06/5a9cc014b0552f975dd39b2b1844d221.png" alt="地图管理" width="45%" />
  <img src="https://images.laoyutang.cn/2026/06/2868050dbad60124dcb313a23d422685.png" alt="性能监控" width="45%" />
</div>
