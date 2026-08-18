<script setup lang="ts">
  import { computed, ref, watch } from 'vue';
  import { api, type MapScriptOverrideContent } from '../services/api';

  defineOptions({ name: 'MapScriptOverridesModal' });

  const props = defineProps<{
    open: boolean;
    mapName: string;
  }>();

  const emit = defineEmits<{
    'update:open': [value: boolean];
  }>();

  const modalOpen = computed({
    get: () => props.open,
    set: (value: boolean) => emit('update:open', value),
  });
  const loading = ref(false);
  const errorMessage = ref('');
  const scripts = ref<MapScriptOverrideContent[]>([]);
  const activeKeys = ref<string[]>([]);
  let requestId = 0;

  const formatScriptSize = (size: number) => {
    if (!Number.isFinite(size) || size < 0) return '未知大小';
    if (size < 1024) return `${size} B`;
    if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KiB`;
    return `${(size / (1024 * 1024)).toFixed(1)} MiB`;
  };

  const getRequestErrorMessage = (error: unknown) => {
    if (error instanceof Error && error.message.trim()) return error.message.trim();
    return '读取覆盖脚本失败';
  };

  const resetState = () => {
    requestId++;
    loading.value = false;
    errorMessage.value = '';
    scripts.value = [];
    activeKeys.value = [];
  };

  const loadScripts = async () => {
    const mapName = props.mapName;
    if (!props.open || !mapName) return;

    const currentRequestId = ++requestId;
    loading.value = true;
    errorMessage.value = '';
    scripts.value = [];
    activeKeys.value = [];

    try {
      const result = await api.getMapScriptOverrides(mapName);
      if (currentRequestId !== requestId) return;
      scripts.value = result.scripts || [];
      const firstScript = scripts.value[0];
      if (firstScript) activeKeys.value = [firstScript.path];
    } catch (error) {
      if (currentRequestId !== requestId) return;
      errorMessage.value = getRequestErrorMessage(error);
    } finally {
      if (currentRequestId === requestId) loading.value = false;
    }
  };

  watch(
    () => props.open,
    (open) => {
      if (open) void loadScripts();
    }
  );

  watch(
    () => props.mapName,
    () => {
      if (props.open) void loadScripts();
    }
  );
</script>

<template>
  <a-modal
    v-model:open="modalOpen"
    :title="`脚本覆盖 - ${mapName}`"
    :footer="null"
    width="960px"
    wrap-class-name="script-overrides-modal"
    @after-close="resetState"
  >
    <div v-if="loading" class="py-12 text-center">
      <a-spin tip="正在读取脚本内容..." />
    </div>
    <a-alert v-else-if="errorMessage" type="error" show-icon :message="errorMessage" />
    <template v-else>
      <a-empty v-if="scripts.length === 0" description="未读取到覆盖脚本" />
      <a-collapse v-else v-model:activeKey="activeKeys" class="script-overrides-collapse">
        <a-collapse-panel v-for="script in scripts" :key="script.path" :force-render="false">
          <template #header>
            <div class="script-panel-header">
              <span class="script-panel-path" :title="script.path">{{ script.path }}</span>
              <span class="script-panel-meta">
                {{ formatScriptSize(script.size) }} · {{ script.encoding }}
              </span>
            </div>
          </template>
          <a-alert
            v-if="script.error"
            type="error"
            show-icon
            :message="script.error"
            class="mb-3"
          />
          <a-alert
            v-if="script.truncated"
            type="warning"
            show-icon
            message="脚本超过 512 KiB，仅显示前 512 KiB。"
            class="mb-3"
          />
          <pre v-if="!script.error" class="script-content">{{ script.content || '' }}</pre>
        </a-collapse-panel>
      </a-collapse>
    </template>
  </a-modal>
</template>

<style scoped>
  :global(.script-overrides-modal .ant-modal) {
    max-width: calc(100vw - 24px);
  }

  :global(.script-overrides-modal .ant-modal-body) {
    max-height: calc(100vh - 170px);
    overflow-y: auto;
  }

  .script-panel-header {
    display: flex;
    min-width: 0;
    width: 100%;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 4px 12px;
    padding-right: 8px;
  }

  .script-panel-path {
    min-width: 0;
    flex: 1 1 460px;
    color: #1f2937;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 12px;
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .script-panel-meta {
    flex: 0 0 auto;
    color: #6b7280;
    font-size: 12px;
    font-weight: 400;
  }

  .script-content {
    max-height: 52vh;
    margin: 0;
    overflow: auto;
    border: 1px solid #e5e7eb;
    border-radius: 6px;
    background: #f8fafc;
    padding: 12px;
    color: #1f2937;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 12px;
    line-height: 1.55;
    tab-size: 4;
    white-space: pre;
  }

  :global(.script-overrides-modal .script-overrides-collapse .ant-collapse-content-box) {
    padding: 12px;
  }

  :global(.dark .script-overrides-modal .script-panel-path) {
    color: #e2e8f0;
  }

  :global(.dark .script-overrides-modal .script-panel-meta) {
    color: #94a3b8;
  }

  :global(.dark .script-overrides-modal .script-content) {
    border-color: #334155;
    background: #0f172a;
    color: #e2e8f0;
  }

  :global(.dark .script-overrides-modal .ant-collapse),
  :global(.dark .script-overrides-modal .ant-collapse-item) {
    border-color: #334155;
  }

  @media (max-width: 640px) {
    :global(.script-overrides-modal .ant-modal) {
      top: 12px;
      margin: 0 auto;
      padding-bottom: 12px;
    }

    :global(.script-overrides-modal .ant-modal-body) {
      max-height: calc(100vh - 145px);
    }

    .script-panel-header {
      gap: 3px;
    }

    .script-panel-path,
    .script-panel-meta {
      flex-basis: 100%;
    }

    .script-content {
      max-height: 45vh;
      padding: 10px;
      font-size: 11px;
    }
  }
</style>
