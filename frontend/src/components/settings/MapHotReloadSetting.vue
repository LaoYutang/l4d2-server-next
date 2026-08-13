<script setup lang="ts">
  import { ref, watch } from 'vue';
  import { message } from 'ant-design-vue';
  import { ReloadOutlined } from '@ant-design/icons-vue';
  import { api } from '../../services/api';

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
    saved: [command: string];
    cancel: [];
  }>();

  const loading = ref(false);
  const saving = ref(false);
  const loaded = ref(false);
  const loadError = ref('');
  const command = ref('');
  const defaultCommand = ref('update_addon_paths; mission_reload');
  let loadRequestId = 0;

  const getErrorMessage = (error: unknown) =>
    error instanceof Error ? error.message : String(error);

  const loadConfig = async () => {
    const requestId = ++loadRequestId;
    loading.value = true;
    loadError.value = '';

    try {
      const config = await api.getMapHotReloadConfig();
      if (requestId !== loadRequestId) return;

      defaultCommand.value = config.default_command;
      command.value = config.command === config.default_command ? '' : config.command;
      loaded.value = true;
    } catch (error: unknown) {
      if (requestId !== loadRequestId) return;

      loadError.value = `获取热重载配置失败：${getErrorMessage(error)}`;
      message.error(loadError.value);
    } finally {
      if (requestId === loadRequestId) {
        loading.value = false;
      }
    }
  };

  const saveConfig = async () => {
    if (loading.value || loadError.value) return;

    saving.value = true;
    try {
      const result = await api.setMapHotReloadConfig(command.value);
      command.value = result.command === defaultCommand.value ? '' : result.command;
      message.success('热重载指令已保存');
      emit('saved', result.command);
    } catch (error: unknown) {
      message.error(`保存热重载配置失败：${getErrorMessage(error)}`);
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
        <ReloadOutlined class="text-blue-500" />
        地图热重载指令
      </h3>
      <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-gray-400">
        配置“热重载地图”操作通过 RCON 执行的指令，可同时刷新地图插件缓存。
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
          <a-form-item label="热重载指令" class="!mb-3">
            <a-input
              v-model:value="command"
              :disabled="loading || Boolean(loadError)"
              :placeholder="defaultCommand"
              @pressEnter="saveConfig"
            />
          </a-form-item>

          <div
            class="rounded-lg border border-gray-200 bg-gray-50/80 p-3 text-xs leading-5 text-gray-600 dark:border-slate-700 dark:bg-slate-950/40 dark:text-gray-300"
          >
            <div>留空保存将恢复默认指令：</div>
            <code class="mt-1 block break-all text-blue-600 dark:text-blue-400">{{
              defaultCommand
            }}</code>
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
