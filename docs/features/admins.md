# 管理员设置

“管理员设置”维护 SourceMod 的管理员配置文件 `admins_simple.ini`，用于给玩家分配游戏内管理权限。

![管理员](https://images.laoyutang.cn/2026/06/aafad0d923200941d54ab2ac51f41214.png)

## 页面能力

管理员可以：

- 查看现有管理员 SteamID 和备注。
- 新增管理员。
- 删除管理员。
- 点击复制 SteamID。

访客登录时不能新增或删除管理员。

## SteamID 格式

新增管理员时支持常见 SteamID 格式，例如：

```text
STEAM_1:1:123456
[U:1:123456]
```

建议在首页玩家列表中直接复制玩家 SteamID，减少手动输入错误。

## 权限刷新

新增或删除管理员后，面板会尝试刷新游戏内管理员权限。如果刷新失败，通常是以下原因：

- SourceMod 未启用或配置文件不存在。
- 游戏服务器离线。
- RCON 地址或密码配置错误。
- 插件运行状态异常。

可以在游戏服控制台或 RCON 控制台手动执行：

```text
sm_reloadadmins
```

## 和面板管理员密码的区别

这里配置的是游戏内 SourceMod 管理员，不是 Web 面板登录密码。

Web 面板管理员密码来自 `L4D2_MANAGER_PASSWORD`；SourceMod 管理员来自游戏目录中的 `admins_simple.ini`。两者互不替代。
