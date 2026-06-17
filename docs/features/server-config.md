# 服务器配置

“服务器配置”面向 `server.cfg` 中的常用参数，适合维护服务器可见性、匹配连接和自定义配置行。

![服务器配置](https://images.laoyutang.cn/2026/06/c68bf9754dc2ae42d88355499956121d.png)

## 常用开关

页面支持：

- 隐藏服务器：对应 `sv_tags hidden`。
- 仅匹配连接：限制连接来源。
- Steam 组 ID：配置服务器关联的 Steam 组。

## 自定义配置

自定义配置行适合填写面板未内置的 `server.cfg` 参数，例如：

```text
sv_consistency 0
sv_allow_lobby_connect_only 0
```

长期配置建议写在这里或直接维护 `server.cfg`，不要只在 RCON 控制台临时执行。

## 权限与生效

访客登录时只能查看，管理员登录后才能保存。

保存后通常需要重启服务器或换图才会完全生效。
