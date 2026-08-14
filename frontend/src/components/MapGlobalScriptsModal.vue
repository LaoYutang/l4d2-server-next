<script setup lang="ts">
  import { computed, ref, watch } from 'vue';
  import { CloseOutlined, EditOutlined, SaveOutlined } from '@ant-design/icons-vue';
  import { message } from 'ant-design-vue';
  import {
    api,
    ApiRequestError,
    type MapGlobalScriptContent,
  } from '../services/api';
  import { useAuthStore } from '../stores/auth';

  defineOptions({ name: 'MapGlobalScriptsModal' });

  const props = defineProps<{
    open: boolean;
    mapName: string;
  }>();

  const emit = defineEmits<{
    'update:open': [value: boolean];
    updated: [mapName: string];
  }>();

  const authStore = useAuthStore();
  const isAdmin = computed(() => authStore.isAdmin);
  const modalOpen = computed({
    get: () => props.open,
    set: (value: boolean) => emit('update:open', value),
  });

  const loading = ref(false);
  const errorMessage = ref('');
  const revision = ref('');
  const scripts = ref<MapGlobalScriptContent[]>([]);
  const activeKeys = ref<string[]>([]);
  const editingPath = ref('');
  const draftContent = ref('');
  const saving = ref(false);
  let requestId = 0;

  const getRequestErrorMessage = (error: unknown, fallback: string) => {
    if (error instanceof Error && error.message.trim()) return error.message.trim();
    return fallback;
  };

  const formatScriptSize = (size: number) => {
    if (!Number.isFinite(size) || size < 0) return '未知大小';
    if (size < 1024) return `${size} B`;
    if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KiB`;
    return `${(size / (1024 * 1024)).toFixed(1)} MiB`;
  };

  const clearEditing = () => {
    editingPath.value = '';
    draftContent.value = '';
  };

  const resetEditor = () => {
    clearEditing();
    saving.value = false;
  };

  const resetState = () => {
    requestId++;
    loading.value = false;
    errorMessage.value = '';
    revision.value = '';
    scripts.value = [];
    activeKeys.value = [];
    resetEditor();
  };

  const loadScripts = async () => {
    const mapName = props.mapName;
    if (!props.open || !mapName) return;

    const currentRequestId = ++requestId;
    loading.value = true;
    errorMessage.value = '';
    revision.value = '';
    scripts.value = [];
    activeKeys.value = [];
    resetEditor();

    try {
      const result = await api.getMapGlobalScripts(mapName);
      if (currentRequestId !== requestId) return;
      revision.value = result.revision;
      scripts.value = result.scripts || [];
      const firstScript = scripts.value[0];
      if (firstScript) activeKeys.value = [firstScript.path];
    } catch (error) {
      if (currentRequestId !== requestId) return;
      errorMessage.value = getRequestErrorMessage(error, '读取全局脚本失败');
    } finally {
      if (currentRequestId === requestId) loading.value = false;
    }
  };

  const getEditDisabledReason = (script: MapGlobalScriptContent) => {
    if (!isAdmin.value) return '只有管理员可以编辑';
    if (script.error) return script.error;
    if (script.truncated) return '脚本超过 512 KiB，不能在面板中安全编辑';
    if (script.encoding === 'unknown') return '无法确认脚本编码，不能安全写回';
    return '';
  };

  const startEditing = (script: MapGlobalScriptContent) => {
    if (getEditDisabledReason(script)) return;
    editingPath.value = script.path;
    draftContent.value = script.content || '';
    if (!activeKeys.value.includes(script.path)) {
      activeKeys.value = [...activeKeys.value, script.path];
    }
  };

  const cancelEditing = () => {
    if (saving.value) return;
    resetEditor();
  };

  const saveScript = async () => {
    const script = scripts.value.find((item) => item.path === editingPath.value);
    if (!script || !revision.value) return;

    saving.value = true;
    try {
      const result = await api.updateMapGlobalScript({
        map: props.mapName,
        path: script.path,
        content: draftContent.value,
        encoding: script.encoding,
        expected_revision: revision.value,
      });

      revision.value = result.revision;
      scripts.value = scripts.value.map((item) =>
        item.path === result.script.path ? result.script : item
      );
      clearEditing();
      emit('updated', result.map);
      message.success('脚本已保存并重新打包；请使用页面顶部的“热重载地图”让服务器重新挂载 VPK');
    } catch (error) {
      if (error instanceof ApiRequestError && error.status === 409) {
        message.warning('地图文件已发生变化，已为你重新载入最新脚本');
        await loadScripts();
      } else {
        message.error(getRequestErrorMessage(error, '保存全局脚本失败'));
      }
    } finally {
      saving.value = false;
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
    :title="`全局脚本 - ${mapName}`"
    :footer="null"
    :closable="!saving"
    :keyboard="!saving"
    :mask-closable="!saving"
    width="960px"
    wrap-class-name="global-scripts-modal"
    @after-close="resetState"
  >
    <div v-if="loading" class="py-12 text-center">
      <a-spin tip="正在读取脚本内容..." />
    </div>
    <a-alert v-else-if="errorMessage" type="error" show-icon :message="errorMessage" />
    <template v-else>
      <a-alert
        v-if="!isAdmin"
        type="info"
        show-icon
        class="mb-3"
        message="当前为只读模式，只有管理员可以编辑并重打包地图脚本。"
      />
      <a-empty v-if="scripts.length === 0" description="未读取到全局脚本" />
      <a-collapse v-else v-model:activeKey="activeKeys" class="script-collapse">
        <a-collapse-panel v-for="script in scripts" :key="script.path" :force-render="false">
          <template #header>
            <div class="script-panel-header">
              <span class="script-panel-path" :title="script.path">{{ script.path }}</span>
              <span class="script-panel-meta">
                {{ formatScriptSize(script.size) }} · {{ script.encoding }}
              </span>
            </div>
          </template>
          <div class="script-panel-body">
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
              message="脚本超过 512 KiB，仅显示前 512 KiB，不能直接编辑。"
              class="mb-3"
            />

            <template v-if="editingPath === script.path">
              <a-textarea
                v-model:value="draftContent"
                class="script-editor-textarea"
                :auto-size="{ minRows: 16, maxRows: 30 }"
                :disabled="saving"
                spellcheck="false"
              />
              <div class="script-editor-footer">
                <span class="script-editor-count">{{ draftContent.length }} 个字符</span>
                <a-space wrap>
                  <a-button
                    class="script-action-button"
                    :disabled="saving"
                    @click="cancelEditing"
                  >
                    <template #icon><close-outlined /></template>
                    取消
                  </a-button>
                  <a-button
                    class="script-action-button"
                    type="primary"
                    :loading="saving"
                    @click="saveScript"
                  >
                    <template #icon><save-outlined /></template>
                    保存
                  </a-button>
                </a-space>
              </div>
            </template>
            <template v-else>
              <div v-if="isAdmin" class="script-panel-actions">
                <a-button
                  class="script-action-button"
                  size="small"
                  :disabled="Boolean(getEditDisabledReason(script)) || saving"
                  :title="getEditDisabledReason(script) || '编辑脚本'"
                  @click="startEditing(script)"
                >
                  <template #icon><edit-outlined /></template>
                  编辑
                </a-button>
              </div>
              <pre v-if="!script.error" class="script-content">{{ script.content || '' }}</pre>
            </template>
          </div>
        </a-collapse-panel>
      </a-collapse>
    </template>
  </a-modal>
</template>

<style scoped>
  :global(.global-scripts-modal .ant-modal) {
    max-width: calc(100vw - 24px);
  }

  :global(.global-scripts-modal .ant-modal-body) {
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

  .script-panel-meta,
  .script-editor-count {
    flex: 0 0 auto;
    color: #6b7280;
    font-size: 12px;
    font-weight: 400;
  }

  .script-panel-actions {
    display: flex;
    justify-content: flex-end;
    margin-bottom: 8px;
  }

  .script-action-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  :global(.global-scripts-modal .script-action-button .anticon) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    line-height: 1;
  }

  :global(.global-scripts-modal .script-action-button .anticon svg) {
    display: block;
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

  .script-editor-textarea {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 12px;
    line-height: 1.55;
    tab-size: 4;
    white-space: pre;
  }

  .script-editor-footer {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin-top: 10px;
  }

  :global(.global-scripts-modal .script-collapse .ant-collapse-content-box) {
    padding: 12px;
  }

  :global(.dark .global-scripts-modal .script-panel-path) {
    color: #e2e8f0;
  }

  :global(.dark .global-scripts-modal .script-panel-meta),
  :global(.dark .global-scripts-modal .script-editor-count) {
    color: #94a3b8;
  }

  :global(.dark .global-scripts-modal .script-content) {
    border-color: #334155;
    background: #0f172a;
    color: #e2e8f0;
  }

  :global(.dark .global-scripts-modal .ant-collapse),
  :global(.dark .global-scripts-modal .ant-collapse-item) {
    border-color: #334155;
  }

  @media (max-width: 640px) {
    :global(.global-scripts-modal .ant-modal) {
      top: 12px;
      margin: 0 auto;
      padding-bottom: 12px;
    }

    :global(.global-scripts-modal .ant-modal-body) {
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

    .script-editor-footer {
      align-items: stretch;
      flex-direction: column;
    }
  }
</style>
