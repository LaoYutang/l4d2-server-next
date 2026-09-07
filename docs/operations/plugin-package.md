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

## NUT 插件包

纯 NUT 允许携带 CFG、EMS 配置和其他资源，例如：

```text
Demo.zip
├── README.md
└── left4dead2/
    ├── scripts/vscripts/mapspawn_addon.nut
    ├── scripts/vscripts/demo/main.nut
    ├── cfg/demo.cfg
    └── ems/demo/Settings.cfg
```

上传或下载后自动记录 `nut` 类型，启用时把非配置内容打包为 `addons/manager_nut_<ID>.vpk`，保留游戏目录内的相对路径。所有 `cfg/`、`ems/` 文件和其他路径中的 `.cfg` 均不打入 VPK，直接部署到对应位置；已有配置会保留，禁用也不会删除。

`FileToString("demo/Settings.cfg")` 对应外部 `left4dead2/ems/demo/Settings.cfg`，不是 `cfg/demo.cfg`。请按插件实际读取方式提供默认文件，不要为了打包移动配置或修改读取路径。将 EMS 配置放入独立的插件子目录，可让面板发现该目录内后续生成的文本配置。

如果同时存在 `.smx`，类型为 `mix`，整个插件沿用散装部署。包内已经包含 VPK 或地图时，不进行二次 VPK 封装。非入口脚本建议使用独立子目录，避免普通同名虚拟文件发生覆盖。

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
