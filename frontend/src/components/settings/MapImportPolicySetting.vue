<script setup lang="ts">
  import { onBeforeUnmount, ref, watch } from 'vue';
  import { message } from 'ant-design-vue';
  import { SafetyCertificateOutlined } from '@ant-design/icons-vue';
  import { api, type MapImportPolicy } from '../../services/api';

  const props = withDefaults(defineProps<{ active?: boolean }>(), { active: true });
  const policy = ref<MapImportPolicy>({
    reject_missing_dictionary: false,
    reject_global_scripts: false,
    reject_script_overrides: false,
    reject_inspection_error: false,
  });
  const rules: { key: keyof MapImportPolicy; title: string; description: string }[] = [
    { key: 'reject_missing_dictionary', title: '拒绝缺失字典的地图', description: '任一 BSP 的 Pakfile 中缺少 stringtable_dictionary.dct 时拒绝导入。' },
    { key: 'reject_global_scripts', title: '拒绝包含全局脚本的地图', description: '包含 mapspawn_addon.nut、scriptedmode_addon.nut 或 director_base_addon.nut 时拒绝导入。' },
    { key: 'reject_script_overrides', title: '拒绝包含脚本覆盖的地图', description: '包含 scripts/gamemodes.txt 或 scripts/weapon_*.txt 时拒绝导入。' },
    { key: 'reject_inspection_error', title: '拒绝无法完成风险检查的地图', description: 'VPK 目录无法解析、BSP 或 Pakfile 无法检查时拒绝导入。这表示检查失败，并非已确认存在上述风险。' },
  ];
  const loading = ref(false);
  const saving = ref(false);
  const loaded = ref(false);
  const loadError = ref('');
  let disposed = false;
  const errorText = (error: unknown) => error instanceof Error ? error.message : String(error);

  const load = async () => {
    loading.value = true;
    loadError.value = '';
    try {
      const result = await api.getMapImportPolicy();
      if (disposed) return;
      policy.value = result;
      loaded.value = true;
    } catch (error) {
      if (!disposed) loadError.value = `加载导入策略失败：${errorText(error)}`;
    } finally {
      if (!disposed) loading.value = false;
    }
  };
  const save = async () => {
    if (!loaded.value || loading.value || saving.value || loadError.value) return;
    saving.value = true;
    try {
      const result = await api.setMapImportPolicy({ ...policy.value });
      if (!disposed) {
        policy.value = result;
        message.success('地图导入风险策略已保存');
      }
    } catch (error) {
      if (!disposed) message.error(`保存失败：${errorText(error)}`);
    } finally {
      if (!disposed) saving.value = false;
    }
  };
  watch(() => props.active, (active) => {
    if (active && !loaded.value && !loading.value) void load();
  }, { immediate: true });
  onBeforeUnmount(() => { disposed = true; });
</script>

<template>
  <section class="rounded-xl border border-gray-200 bg-white p-4 sm:p-5 dark:border-slate-700 dark:bg-slate-900/40">
    <h3 class="flex items-center gap-2 text-base font-semibold text-gray-800 dark:text-gray-100">
      <SafetyCertificateOutlined class="text-orange-500" />地图导入风险拦截
    </h3>
    <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-gray-400">
      默认全部关闭，关闭时仍保留风险提示。规则同时适用于上传、链接和工坊下载；开启后拒绝相应地图导入。
    </p>
    <a-alert v-if="loadError" class="mt-3" type="error" show-icon :message="loadError">
      <template #action><a-button size="small" @click="load">重试</a-button></template>
    </a-alert>
    <div class="mt-3 divide-y divide-gray-100 dark:divide-slate-800">
      <div v-for="rule in rules" :key="rule.key" class="flex items-start justify-between gap-4 py-3">
        <div class="min-w-0">
          <div class="text-sm font-medium text-gray-800 dark:text-gray-100">{{ rule.title }}</div>
          <div class="mt-1 break-words text-xs leading-5 text-gray-500 dark:text-gray-400">{{ rule.description }}</div>
        </div>
        <a-switch v-model:checked="policy[rule.key]" :aria-label="rule.title" :disabled="!loaded || loading || saving || Boolean(loadError)" checked-children="开" un-checked-children="关" />
      </div>
    </div>
    <p class="mt-2 text-xs leading-5 text-gray-500 dark:text-gray-400">
      对之后开始检查的导入批次生效，不会删除已安装地图。压缩包内任一 VPK 被拒，整包不安装；工坊合集的下载任务分别判断。
    </p>
    <div class="mt-4 flex justify-end">
      <a-button type="primary" :loading="saving" :disabled="!loaded || loading || Boolean(loadError)" @click="save">保存</a-button>
    </div>
  </section>
</template>
