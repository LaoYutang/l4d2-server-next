## 简介

多特感插件

## 可用指令

- `sm_weight <类型> <比重>` 设置特感生成比重（管理员指令）
  - 类型：`reset`（重置默认）/ `all`（全部）/ `smoker` / `boomer` / `hunter` / `spitter` / `jockey` / `charger`
  - 比重：`>= 0` 的整数
- `sm_limit <类型> <数量>` 设置特感生成数量（管理员指令）
  - 类型：`reset`（重置默认）/ `all`（全部种类）/ `max`（最大总数）/ `group`/`wave`（每波数量）/ `smoker` / `boomer` / `hunter` / `spitter` / `jockey` / `charger`
  - 数量：`>= 0` 的整数
- `sm_timer <固定时间>` 或 `sm_timer <最小时间> <最大时间>` 设置特感生成时间（管理员指令）
  - 固定时间：设为固定秒数（最小 0.1）
  - 最小/最大时间：设置随机范围（最小 >= 0.1，最大 >= 1.0 且大于最小值）
- `sm_resetspawn` 处死所有特感并重新开始生成计时（管理员指令）
- `sm_forcetimer` 或 `sm_forcetimer <时间>` 手动开始生成计时（管理员指令）
  - 不填时间则立即开始，填写时间则指定下次生成等待秒数
- `sm_type <类型>` 切换特感轮换模式（管理员指令）
  - `off` 关闭单一特感模式，恢复默认
  - `random` 随机轮换一种特感模式
  - `smoker` / `boomer` / `hunter` / `spitter` / `jockey` / `charger` 只刷指定特感
