<script setup lang="ts">
  import { computed, onBeforeUnmount, ref, watch } from 'vue';
  import { message } from 'ant-design-vue';
  import { ClockCircleOutlined, DeleteOutlined } from '@ant-design/icons-vue';
  import { api, type MapCleanupConfig } from '../../services/api';

  const props = withDefaults(defineProps<{ active?: boolean }>(), { active: true });
  const config = ref<MapCleanupConfig | null>(null);
  const enabled = ref(false);
  const retentionDays = ref<number | undefined>(3);
  const loading = ref(false);
  const saving = ref(false);
  const loadError = ref('');
  const saveError = ref('');
  const saved = ref(false);
  let requestId = 0;
  let disposed = false;
  const disabled = computed(() => !config.value || loading.value || saving.value || Boolean(loadError.value));
  const nextCheck = computed(() => {
    const value = config.value?.next_check_at;
    if (!value) return '读取后显示';
    // Keep the server's wall time and UTC offset instead of converting to the browser's timezone.
    return value.replace('T', ' ').replace(/(\.\d+)?(Z|[+-]\d{2}:\d{2})$/, (_match, _fraction, offset: string) => offset === 'Z' ? ' UTC' : ` UTC${offset}`);
  });
  const errorText = (error: unknown) => error instanceof Error ? error.message : String(error);
  const applyConfig = (result: MapCleanupConfig) => {
    config.value = result;
    enabled.value = result.enabled;
    retentionDays.value = result.retention_days;
  };

  const load = async () => {
    const current = ++requestId;
    loading.value = true;
    loadError.value = '';
    try {
      const result = await api.getMapCleanupConfig();
      if (!disposed && current === requestId) {
        applyConfig(result);
        saveError.value = '';
        saved.value = false;
      }
    } catch (error) {
      if (!disposed && current === requestId) loadError.value = `加载地图清理配置失败：${errorText(error)}`;
    } finally {
      if (!disposed && current === requestId) loading.value = false;
    }
  };

  const save = async () => {
    if (disabled.value) return;
    const days = retentionDays.value;
    if (typeof days !== 'number' || !Number.isInteger(days) || days < 1 || days > 3650) {
      saveError.value = '保留天数必须是 1–3650 之间的整数';
      return;
    }
    const target = { enabled: enabled.value, retention_days: days };
    saving.value = true;
    saveError.value = '';
    saved.value = false;
    // A previous read must never undo a successful save.
    requestId += 1;
    try {
      const result = await api.setMapCleanupConfig(target);
      if (disposed) return;
      applyConfig(result);
      saved.value = true;
      message.success(result.enabled ? '已开启地图自动清理，下次检查时生效' : '已关闭地图自动清理');
    } catch (error) {
      if (!disposed) saveError.value = `保存失败：${errorText(error)}。请重新读取配置，核对服务器实际状态。`;
    } finally {
      if (!disposed) saving.value = false;
    }
  };

  watch(() => props.active, (active) => {
    if (active && !config.value && !loading.value) void load();
  }, { immediate: true });
  onBeforeUnmount(() => { disposed = true; requestId += 1; });
</script>

<template>
  <section class="map-cleanup-setting min-w-0 overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-slate-700 dark:bg-slate-900/40">
    <div class="flex flex-wrap items-start justify-between gap-4 p-4 sm:p-5">
      <div class="min-w-0 flex-1">
        <h3 class="flex min-w-0 flex-wrap items-center gap-2 text-base font-semibold text-gray-800 dark:text-gray-100">
          <DeleteOutlined class="shrink-0 text-orange-500" />地图自动清理
          <a-tag v-if="config" :color="config.enabled ? 'green' : 'default'" class="!m-0 !text-xs">
            {{ config.enabled ? '已开启' : '已关闭' }}
          </a-tag>
        </h3>
        <p class="mb-0 mt-1 break-words text-sm leading-6 text-gray-500 dark:text-gray-400">
          默认关闭。开启后只清理有上传时间且已过保留期的未锁定地图。
        </p>
      </div>
      <a-switch v-model:checked="enabled" :disabled="disabled" :loading="loading" class="shrink-0" aria-label="启用地图自动清理" checked-children="开" un-checked-children="关" />
    </div>

    <div class="mx-4 rounded-lg bg-gray-50 p-3 dark:bg-slate-800/60 sm:mx-5 sm:p-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="min-w-0">
          <div class="text-sm font-medium text-gray-800 dark:text-gray-100">地图保留时间</div>
          <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">从成功导入开始计算，默认 3 天</div>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <a-input-number v-model:value="retentionDays" :min="1" :max="3650" :precision="0" :disabled="disabled || !enabled" aria-label="地图保留天数" class="!w-24" />
          <span class="text-sm text-gray-600 dark:text-gray-300">天</span>
        </div>
      </div>
      <div class="mt-4 border-t border-gray-200 pt-3 dark:border-slate-700">
        <div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-600 dark:text-gray-300">
          <ClockCircleOutlined />
          <span>每天 {{ config?.check_time || '03:00' }} 检查</span>
          <span v-if="config" class="break-all text-gray-400 dark:text-gray-500">服务器时区：{{ config.timezone }}</span>
        </div>
        <div v-if="config" class="mt-2 break-words text-xs text-gray-500 dark:text-gray-400">
          下次检查：{{ nextCheck }}<span v-if="!config.enabled">（当前关闭，不执行）</span>
        </div>
      </div>
    </div>

    <div class="map-cleanup-feedback flex min-w-0 flex-col gap-3 p-4 sm:p-5">
      <p class="mb-0 break-words text-xs leading-6 text-gray-500 dark:text-gray-400">
        历史地图没有上传时间，默认保留。锁定地图始终保留。修改设置从下次检查开始生效，重启不会立即补跑；关闭后保留已设置的天数。
      </p>
      <a-alert v-if="loadError" class="map-cleanup-alert" type="error" show-icon :message="loadError">
        <template #action><a-button size="small" @click="load">重试</a-button></template>
      </a-alert>
      <a-alert v-if="saveError" class="map-cleanup-alert" type="error" show-icon :message="saveError">
        <template #action><a-button size="small" :disabled="saving || loading" @click="load">重新读取</a-button></template>
      </a-alert>
      <a-alert v-if="saved && enabled === config?.enabled && retentionDays === config?.retention_days" class="map-cleanup-alert" type="success" show-icon message="配置已保存，下次每日检查按新设置执行。" />
      <div class="flex min-w-0 justify-end">
        <a-button class="!min-h-10 w-full sm:w-auto" type="primary" :disabled="disabled" :loading="saving" @click="save">保存清理设置</a-button>
      </div>
    </div>
  </section>
</template>

<style scoped>
  .map-cleanup-setting {
    overflow-wrap: anywhere;
  }

  .map-cleanup-alert {
    min-width: 0;
    align-items: flex-start;
  }

  .map-cleanup-alert :deep(.ant-alert-content) {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .map-cleanup-alert :deep(.ant-alert-message) {
    white-space: normal;
  }

  .map-cleanup-alert :deep(.ant-alert-action .ant-btn) {
    min-height: 2.5rem;
    white-space: normal;
  }

  @media (max-width: 639px) {
    .map-cleanup-alert {
      flex-wrap: wrap;
      row-gap: 0.75rem;
    }

    .map-cleanup-alert :deep(.ant-alert-action) {
      flex-basis: 100%;
      margin-inline-start: 0;
    }

    .map-cleanup-alert :deep(.ant-alert-action .ant-btn) {
      width: 100%;
    }

    .map-cleanup-setting :deep(.ant-input-number-input) {
      min-height: 2.5rem;
    }
  }
</style>
