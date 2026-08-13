<script setup lang="ts">
  import { computed, ref, watch } from 'vue';
  import { message } from 'ant-design-vue';
  import { CloudDownloadOutlined } from '@ant-design/icons-vue';
  import { api, type SteamCDNIPEntry } from '../../services/api';

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
    saved: [steamCDNIP: string];
    cancel: [];
  }>();

  const loading = ref(false);
  const saving = ref(false);
  const loaded = ref(false);
  const loadError = ref('');
  const steamCDNIP = ref('');
  const entries = ref<SteamCDNIPEntry[]>([]);
  const entriesError = ref('');
  let loadRequestId = 0;

  const options = computed(() =>
    entries.value.map((entry) => ({
      value: entry.ip,
      label: entry.category ? `${entry.category} · ${entry.ip}` : entry.ip,
    }))
  );

  const getErrorMessage = (error: unknown) =>
    error instanceof Error ? error.message : String(error);

  const filterOption = (input: string, option?: { label?: string; value?: string }) => {
    const keyword = input.trim().toLowerCase();
    if (!keyword) return true;
    return `${option?.label || ''} ${option?.value || ''}`.toLowerCase().includes(keyword);
  };

  const loadConfig = async () => {
    const requestId = ++loadRequestId;
    loading.value = true;
    loadError.value = '';
    entriesError.value = '';

    const [configResult, entriesResult] = await Promise.allSettled([
      api.getDownloadConfig(),
      api.getSteamCDNIPEntries(),
    ]);

    if (requestId !== loadRequestId) return;

    if (configResult.status === 'fulfilled') {
      steamCDNIP.value = configResult.value.steam_cdn_ip || '';
      loaded.value = true;
    } else {
      loadError.value = `获取下载配置失败：${getErrorMessage(configResult.reason)}`;
      message.error(loadError.value);
    }

    if (entriesResult.status === 'fulfilled') {
      entries.value = entriesResult.value;
      if (entriesResult.value.length === 0) {
        entriesError.value = '候选 IP 服务暂未返回可用地址，可直接输入 IP。';
      }
    } else {
      console.warn('Failed to load Steam CDN IP entries', entriesResult.reason);
      entries.value = [];
      entriesError.value = '候选 IP 列表加载失败，可直接输入 IP。';
    }

    loading.value = false;
  };

  const saveConfig = async () => {
    if (loading.value || loadError.value) return;

    saving.value = true;
    try {
      const result = await api.setDownloadConfig(steamCDNIP.value.trim());
      steamCDNIP.value = result.steam_cdn_ip || '';
      message.success(result.steam_cdn_ip ? 'Steam CDN 指定 IP 已保存' : '已恢复使用 DNS');
      emit('saved', steamCDNIP.value);
    } catch (error: unknown) {
      message.error(`保存下载配置失败：${getErrorMessage(error)}`);
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
        <CloudDownloadOutlined class="text-cyan-500" />
        Steam CDN 下载设置
      </h3>
      <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-gray-400">
        为 Steam 创意工坊下载指定 CDN 地址，适用于 DNS 解析结果不稳定的网络环境。
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
          <a-form-item label="Steam CDN 指定 IP" class="!mb-3">
            <a-auto-complete
              v-model:value="steamCDNIP"
              :options="options"
              :filter-option="filterOption"
              :disabled="loading || Boolean(loadError)"
              allow-clear
              placeholder="留空则使用 DNS，也可以直接输入 IP"
            />
          </a-form-item>

          <a-alert
            v-if="entriesError"
            type="warning"
            show-icon
            :message="entriesError"
            class="mb-3"
          />

          <div
            class="rounded-lg border border-gray-200 bg-gray-50/80 p-3 text-xs leading-5 text-gray-600 dark:border-slate-700 dark:bg-slate-950/40 dark:text-gray-300"
          >
            <div>仅 cdn.steamusercontent.com 的下载连接会使用此 IP。</div>
            <div>请求域名与 HTTPS 证书校验保持不变；指定 IP 不可用时不会回退 DNS。</div>
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
