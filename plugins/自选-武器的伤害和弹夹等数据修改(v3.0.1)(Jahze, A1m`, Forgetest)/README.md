## 简介

武器的伤害和弹夹等数据修改

## 可用指令

- `sm_weaponstats [武器名]` 显示武器属性数据，不填武器名则查看当前手持武器
- `sm_weapon_attributes [武器名]` 与上条相同

## 服务器控制台指令
建议写到server.cfg中，方便管理员管理
- `sm_weapon <武器名> <属性> <数值>` 修改指定武器的属性
- `sm_weapon_attributes_reset` 重置所有已修改的武器和近战属性为默认值

## 常用武器设置用例

**指令格式**：`sm_weapon <武器名> <属性名> <数值>`

武器名不需要带 `weapon_` 前缀，例如 `weapon_rifle` 直接写 `rifle`。

### 常用武器名对照

- 步枪：`rifle`（M16）、`rifle_ak47`（AK47）、`rifle_desert`（SCAR）、`rifle_sg552`（SG552）
- 冲锋枪：`smg`（UZI）、`smg_silenced`（MAC）、`smg_mp5`（MP5）
- 霰弹枪：`pumpshotgun`（木喷）、`shotgun_chrome`（铁喷）、`autoshotgun`（二代连喷）、`shotgun_spas`（一代连喷）
- 狙击枪：`hunting_rifle`（猎枪）、`sniper_military`（军狙）、`sniper_awp`（AWP）、`sniper_scout`（Scout）
- 手枪：`pistol`（小手枪）、`pistol_magnum`（马格南）
- 其他：`grenade_launcher`（榴弹）、`rifle_m60`（M60）
- 近战：`knife`、`baseball_bat`、`chainsaw`、`cricket_bat`、`crowbar`、`fireaxe`、`frying_pan`、`golfclub`、`katana`、`machete`、`tonfa`、`shovel`、`pitchfork`

### 枪械属性

| 属性名               | 说明                         | 示例                                           |
| -------------------- | ---------------------------- | ---------------------------------------------- |
| `damage`             | 单发伤害                     | `sm_weapon rifle damage 50`                    |
| `bullets`            | 每发子弹数（霰弹枪有效）     | `sm_weapon pumpshotgun bullets 20`             |
| `clipsize`           | 弹夹容量                     | `sm_weapon rifle clipsize 60`                  |
| `cycletime`          | 射击间隔（秒，越小射速越快） | `sm_weapon rifle cycletime 0.05`               |
| `reloadduration`     | 换弹时间（秒）               | `sm_weapon rifle reloadduration 1.5`           |
| `reloaddurationmult` | 换弹时间倍率（仅霰弹枪）     | `sm_weapon pumpshotgun reloaddurationmult 0.5` |
| `spreadpershot`      | 每次射击扩散                 | `sm_weapon rifle spreadpershot 0.1`            |
| `maxspread`          | 最大扩散                     | `sm_weapon rifle maxspread 5.0`                |
| `speed`              | 持枪移动速度                 | `sm_weapon rifle speed 250`                    |
| `range`              | 射程                         | `sm_weapon rifle range 5000`                   |
| `penlayers`          | 穿透层数                     | `sm_weapon rifle penlayers 5`                  |
| `tankdamagemult`     | 对坦克伤害倍率               | `sm_weapon rifle tankdamagemult 2.0`           |
| `verticalpunch`      | 垂直后坐力                   | `sm_weapon rifle verticalpunch 1.0`            |
| `horizpunch`         | 水平后坐力                   | `sm_weapon rifle horizpunch 0.5`               |

### 近战属性

| 属性名           | 说明            | 示例                                  |
| ---------------- | --------------- | ------------------------------------- |
| `damage`         | 伤害            | `sm_weapon katana damage 200`         |
| `refiredelay`    | 攻击间隔（秒）  | `sm_weapon katana refiredelay 0.3`    |
| `weaponidletime` | 武器闲置时间    | `sm_weapon katana weaponidletime 1.0` |
| `decapitates`    | 是否斩首（0/1） | `sm_weapon katana decapitates 1`      |
| `tankdamagemult` | 对坦克伤害倍率  | `sm_weapon katana tankdamagemult 3.0` |
