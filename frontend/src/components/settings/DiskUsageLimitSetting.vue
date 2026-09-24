<script setup lang="ts">
  import { computed, ref, watch } from 'vue';
  import { message } from 'ant-design-vue';
  import { DatabaseOutlined } from '@ant-design/icons-vue';
  import { api } from '../../services/api';
  import { formatBytes, formatPercent } from '../../utils/format';

  type SettingContext = 'page' | 'modal';

  const props = withDefaults(
    defineProps<{
      active?: boolean;
      context?: SettingContext;
    }>(),
    {
      active: true,
      context: 'page',
    }
  );

  const emit = defineEmits<{
    saved: [limitPercent: number];
    cancel: [];
  }>();

  const loading = ref(false);
  const saving = ref(false);
  const loaded = ref(false);
  const loadError = ref('');
  const limitPercent = ref<number>(90);
  const defaultPercent = ref(90);
  const minPercent = ref(80);
  const maxPercent = ref(99);
  const currentUsedPercent = ref<number | null>(null);
  const currentFreeBytes = ref<number | null>(null);
  let loadRequestId = 0;

  const usageSummary = computed(() => {
    if (currentUsedPercent.value === null) return '';
    const parts = [`当前使用率 ${formatPercent(currentUsedPercent.value)}`];
    if (currentFreeBytes.value !== null) {
      parts.push(`剩余 ${formatBytes(currentFreeBytes.value)}`);
    }
    return parts.join(' · ');
  });

  const getErrorMessage = (error: unknown) =>
    error instanceof Error ? error.message : String(error);

  const loadConfig = async () => {
    const requestId = ++loadRequestId;
    loading.value = true;
    loadError.value = '';

    try {
      const config = await api.getDiskUsageLimitConfig();
      if (requestId !== loadRequestId) return;

      limitPercent.value = config.limit_percent;
      defaultPercent.value = config.default_percent;
      minPercent.value = config.min_percent;
      maxPercent.value = config.max_percent;
      currentUsedPercent.value = config.current?.used_percent ?? null;
      currentFreeBytes.value = config.current?.free_bytes ?? null;
      loaded.value = true;
    } catch (error: unknown) {
      if (requestId !== loadRequestId) return;

      loadError.value = `获取磁盘配置失败：${getErrorMessage(error)}`;
      message.error(loadError.value);
    } finally {
      if (requestId === loadRequestId) {
        loading.value = false;
      }
    }
  };

  const saveConfig = async () => {
    if (loading.value || saving.value || loadError.value) return;

    const target = Number(limitPercent.value);
    if (!Number.isInteger(target) || target < minPercent.value || target > maxPercent.value) {
      message.error(`磁盘使用率上限必须是 ${minPercent.value} - ${maxPercent.value} 之间的整数`);
      return;
    }

    saving.value = true;
    try {
      const result = await api.setDiskUsageLimitConfig(target);
      limitPercent.value = result.limit_percent;
      message.success(`磁盘使用率上限已设置为 ${result.limit_percent}%`);
      emit('saved', result.limit_percent);
    } catch (error: unknown) {
      message.error(`保存磁盘配置失败：${getErrorMessage(error)}`);
    } finally {
      saving.value = false;
    }
  };

  watch(
    () => props.active,
    (active) => {
      if (!active) {
        if (props.context === 'modal') {
          loadRequestId += 1;
        }
        return;
      }

      if (props.context === 'modal' || !loaded.value) {
        void loadConfig();
      }
    },
    { immediate: true }
  );
</script>

<template>
  <section
    :class="[
      context === 'page'
        ? 'rounded-xl border border-gray-200 bg-white p-4 sm:p-5 dark:border-slate-700 dark:bg-slate-900/40'
        : '',
    ]"
  >
    <div v-if="context === 'page'" class="mb-4">
      <h3 class="flex items-center gap-2 text-base font-semibold text-gray-800 dark:text-gray-100">
        <DatabaseOutlined class="text-rose-500" />
        磁盘使用率上限
      </h3>
      <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-gray-400">
        地图目录所在分区的使用率超过该上限时，将阻止新的上传和下载任务，避免把磁盘写满。
      </p>
    </div>

    <a-spin :spinning="loading">
      <div class="space-y-4">
        <a-alert v-if="loadError" type="error" show-icon :message="loadError">
          <template #action>
            <a-button size="small" @click="loadConfig">重试</a-button>
          </template>
        </a-alert>

        <a-form layout="vertical">
          <a-form-item :label="`使用率上限（${minPercent}% - ${maxPercent}%）`" class="!mb-3">
            <a-input-number
              v-model:value="limitPercent"
              class="w-full"
              :min="minPercent"
              :max="maxPercent"
              :step="1"
              :precision="0"
              :disabled="loading || Boolean(loadError)"
              addon-after="%"
            />
          </a-form-item>

          <a-alert
            v-if="usageSummary"
            type="info"
            show-icon
            :message="usageSummary"
            class="mb-3"
          />

          <div
            class="rounded-lg border border-gray-200 bg-gray-50/80 p-3 text-xs leading-5 text-gray-600 dark:border-slate-700 dark:bg-slate-950/40 dark:text-gray-300"
          >
            <div>默认值 {{ defaultPercent }}%，可直接输入 {{ minPercent }} - {{ maxPercent }} 之间的整数。</div>
            <div>超过上限时，管理员会看到二次确认提示，确认后仍可继续上传；游客和“仅地图上传”授权码会被直接拒绝。</div>
            <div>添加下载任务超过上限时一律拒绝，不提供二次确认。</div>
            <div>剩余空间不足以下载或解压文件时（低于文件大小的 2 倍），任何角色都无法继续。</div>
          </div>
        </a-form>

        <div class="mt-4 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <a-button v-if="context === 'modal'" class="w-full sm:w-auto" @click="emit('cancel')">
            取消
          </a-button>
          <a-button
            type="primary"
            class="w-full sm:w-auto"
            :loading="saving"
            :disabled="loading || Boolean(loadError)"
            @click="saveConfig"
          >
            保存
          </a-button>
        </div>
      </div>
    </a-spin>
  </section>
</template>
