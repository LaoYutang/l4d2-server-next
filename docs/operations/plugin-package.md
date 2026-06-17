# 插件 ZIP 规范

Web 面板的“上传插件”功能只接受 `.zip` 文件。ZIP 内部必须按 L4D2 的 `left4dead2` 游戏目录组织文件，支持单插件包和多插件包两种格式。

## 单插件 ZIP

如果压缩包根目录就是 `left4dead2/`，会被识别为一个插件。插件名称取自 ZIP 文件名去掉 `.zip` 后缀。

```text
MyPlugin.zip
├── README.md
└── left4dead2/
    ├── addons/sourcemod/plugins/my_plugin.smx
    ├── addons/sourcemod/gamedata/my_plugin.txt
    └── cfg/sourcemod/my_plugin.cfg
```

## 多插件 ZIP

如果一个 ZIP 内包含多个插件，每个一级目录就是一个插件名称，并且每个插件目录下应包含 `left4dead2/`。

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

## 推荐目录

常见路径：

```text
left4dead2/addons/sourcemod/plugins/*.smx
left4dead2/addons/sourcemod/configs/
left4dead2/addons/sourcemod/gamedata/
left4dead2/addons/sourcemod/translations/
left4dead2/cfg/sourcemod/*.cfg
```

放在 `left4dead2/cfg/sourcemod/` 下的 `.cfg` 文件会随插件启用复制到服务器，并在“插件配置”中作为可编辑配置项显示。

## README

插件根目录可以放 `.md` 说明文件，推荐命名为 `README.md` 或 `readme.md`。面板会把它作为插件详情显示；如果存在多个 `.md` 文件，会优先使用 `README.md`。

## 注意事项

- 不要在 ZIP 根目录放 `readme.txt` 等零散文件。
- 单插件 ZIP 根目录只放 `left4dead2/` 和可选 `.md` 说明文件。
- 多插件 ZIP 根目录只放插件文件夹。
- 单插件 ZIP 不要再套一层插件名目录，除非你要使用多插件 ZIP 格式。
- 插件名称不能和现有插件重复。
- `__MACOSX/` 和 `.DS_Store` 会被自动忽略。
- 中文文件名支持 UTF-8/GBK 编码。
