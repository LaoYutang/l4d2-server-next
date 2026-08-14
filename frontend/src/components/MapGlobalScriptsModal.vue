<script setup lang="ts">
  import { computed, ref, watch } from 'vue';
  import {
    CloseOutlined,
    EditOutlined,
    EnvironmentOutlined,
    SaveOutlined,
    UndoOutlined,
  } from '@ant-design/icons-vue';
  import { message } from 'ant-design-vue';
  import {
    api,
    ApiRequestError,
    type MapGlobalScriptContent,
    type MapMissionCampaign,
  } from '../services/api';
  import { useAuthStore } from '../stores/auth';
  import {
    applyCampaignGuard,
    inspectCampaignGuard,
    prepareCampaignCodes,
    removeCampaignGuard,
  } from '../utils/mapCampaignGuard';

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
  const campaignSelectorOpen = ref(false);
  const campaignLoading = ref(false);
  const campaignError = ref('');
  const campaignLoaded = ref(false);
  const campaigns = ref<MapMissionCampaign[]>([]);
  const selectedCampaignKey = ref('');
  let requestId = 0;
  let campaignRequestId = 0;

  interface CampaignChoice {
    key: string;
    campaign: MapMissionCampaign;
    codes: string[];
    invalidCount: number;
  }

  const campaignChoices = computed<CampaignChoice[]>(() =>
    campaigns.value.map((campaign, index) => {
      const chapters = Array.isArray(campaign.Chapters) ? campaign.Chapters : [];
      const prepared = prepareCampaignCodes(chapters.map((chapter) => chapter?.Code));
      return {
        key: `campaign-${index}`,
        campaign,
        codes: prepared.codes,
        invalidCount: prepared.invalidCount,
      };
    })
  );
  const campaignSelectOptions = computed(() =>
    campaignChoices.value.map((choice) => ({
      value: choice.key,
      label: `${choice.campaign.Title?.trim() || '未命名战役'} · ${
        choice.campaign.VpkName?.trim() || '未知 VPK'
      } · ${choice.codes.length} 个有效章节`,
      disabled: choice.codes.length === 0,
    }))
  );
  const selectedCampaignChoice = computed(() =>
    campaignChoices.value.find((choice) => choice.key === selectedCampaignKey.value)
  );
  const draftGuardState = computed(() => inspectCampaignGuard(draftContent.value));
  const codeSignature = (codes: string[]) => [...codes].sort().join('\u0000');
  const currentLimitedCampaign = computed(() => {
    if (draftGuardState.value.status !== 'valid') return undefined;
    const currentSignature = codeSignature(draftGuardState.value.codes);
    return campaignChoices.value.find(
      (choice) => codeSignature(choice.codes) === currentSignature
    );
  });
  const currentCampaignLimitLabel = computed(() => {
    if (draftGuardState.value.status !== 'valid') return '';
    return (
      currentLimitedCampaign.value?.campaign.Title?.trim() ||
      `${draftGuardState.value.codes.length} 个章节`
    );
  });

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

  const closeCampaignSelector = () => {
    campaignSelectorOpen.value = false;
    selectedCampaignKey.value = '';
  };

  const resetCampaigns = () => {
    campaignRequestId++;
    closeCampaignSelector();
    campaignLoading.value = false;
    campaignError.value = '';
    campaignLoaded.value = false;
    campaigns.value = [];
  };

  const clearEditing = () => {
    closeCampaignSelector();
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
    resetCampaigns();
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

  const loadCampaigns = async (force = false) => {
    if (campaignLoaded.value && !force) return;

    const currentRequestId = ++campaignRequestId;
    campaignLoading.value = true;
    campaignError.value = '';
    if (force) {
      campaignLoaded.value = false;
      campaigns.value = [];
      selectedCampaignKey.value = '';
    }

    try {
      const result = await api.getRconMapList();
      if (currentRequestId !== campaignRequestId) return;
      campaigns.value = Array.isArray(result)
        ? result.filter((campaign) => campaign && typeof campaign === 'object')
        : [];
      campaignLoaded.value = true;
    } catch (error) {
      if (currentRequestId !== campaignRequestId) return;
      campaignError.value = getRequestErrorMessage(error, '获取三方战役列表失败');
    } finally {
      if (currentRequestId === campaignRequestId) campaignLoading.value = false;
    }
  };

  const openCampaignSelector = async () => {
    const guardState = inspectCampaignGuard(draftContent.value);
    if (guardState.status === 'malformed') {
      message.warning(guardState.error);
      return;
    }

    selectedCampaignKey.value = '';
    campaignSelectorOpen.value = true;
    await loadCampaigns();
    if (!campaignSelectorOpen.value) return;
    selectedCampaignKey.value = currentLimitedCampaign.value?.key || '';
  };

  const retryLoadCampaigns = async () => {
    await loadCampaigns(true);
    if (!campaignSelectorOpen.value) return;
    selectedCampaignKey.value = currentLimitedCampaign.value?.key || '';
  };

  const applySelectedCampaign = () => {
    const choice = selectedCampaignChoice.value;
    if (!choice) {
      message.warning('请选择一个包含有效章节 Code 的三方战役');
      return;
    }

    const result = applyCampaignGuard(draftContent.value, choice.codes);
    if (!result.ok) {
      message.error(result.error);
      return;
    }

    draftContent.value = result.content;
    closeCampaignSelector();
    const ignoredMessage = choice.invalidCount > 0
      ? `，已忽略 ${choice.invalidCount} 个无效 Code`
      : '';
    message.success(`已限定为“${choice.campaign.Title || '未命名战役'}”${ignoredMessage}`);
  };

  const removeCampaignLimit = () => {
    const result = removeCampaignGuard(draftContent.value);
    if (!result.ok) {
      message.error(result.error);
      return;
    }
    draftContent.value = result.content;
    message.success('已移除战役限定');
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
              <a-alert
                v-if="draftGuardState.status === 'malformed'"
                type="warning"
                show-icon
                :message="draftGuardState.error"
                class="mb-3"
              />
              <a-textarea
                v-model:value="draftContent"
                class="script-editor-textarea"
                :auto-size="{ minRows: 16, maxRows: 30 }"
                :disabled="saving"
                spellcheck="false"
              />
              <div class="script-editor-footer">
                <div class="script-editor-status">
                  <span class="script-editor-count">{{ draftContent.length }} 个字符</span>
                  <a-tag v-if="draftGuardState.status === 'valid'" color="blue" class="!m-0">
                    已限定：{{ currentCampaignLimitLabel }}
                  </a-tag>
                </div>
                <a-space wrap>
                  <a-button
                    class="script-action-button"
                    :disabled="saving"
                    @click="openCampaignSelector"
                  >
                    <template #icon><environment-outlined /></template>
                    {{ draftGuardState.status === 'valid' ? '更改限定' : '限制战役' }}
                  </a-button>
                  <a-button
                    v-if="draftGuardState.status === 'valid'"
                    class="script-action-button"
                    :disabled="saving"
                    @click="removeCampaignLimit"
                  >
                    <template #icon><undo-outlined /></template>
                    移除限定
                  </a-button>
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

  <a-modal
    v-model:open="campaignSelectorOpen"
    title="限制脚本执行战役"
    width="680px"
    wrap-class-name="campaign-limit-modal"
    :mask-closable="!campaignLoading"
    :keyboard="!campaignLoading"
    @after-close="selectedCampaignKey = ''"
  >
    <div v-if="campaignLoading" class="py-10 text-center">
      <a-spin tip="正在读取三方战役..." />
    </div>
    <a-alert
      v-else-if="campaignError"
      type="error"
      show-icon
      :message="campaignError"
    >
      <template #action>
        <a-button size="small" @click="retryLoadCampaigns">重试</a-button>
      </template>
    </a-alert>
    <a-empty
      v-else-if="campaignChoices.length === 0"
      description="未读取到三方战役"
    />
    <div v-else class="campaign-selector-content">
      <a-select
        v-model:value="selectedCampaignKey"
        class="w-full"
        show-search
        option-filter-prop="label"
        placeholder="请选择三方战役"
        :options="campaignSelectOptions"
      />

      <a-alert
        v-if="draftGuardState.status === 'valid' && !currentLimitedCampaign"
        type="info"
        show-icon
        message="当前脚本已有战役限定，但对应战役已不在三方战役列表中；可选择新战役替换。"
      />

      <div v-if="selectedCampaignChoice" class="campaign-choice-detail">
        <div class="campaign-choice-title">
          {{ selectedCampaignChoice.campaign.Title || '未命名战役' }}
        </div>
        <div class="campaign-choice-vpk">
          {{ selectedCampaignChoice.campaign.VpkName || '未知 VPK' }}
        </div>
        <div class="campaign-code-list">
          <a-tag
            v-for="code in selectedCampaignChoice.codes"
            :key="code"
            class="!m-0 font-mono"
          >
            {{ code }}
          </a-tag>
        </div>
        <a-alert
          v-if="selectedCampaignChoice.invalidCount > 0"
          type="warning"
          show-icon
          :message="`有 ${selectedCampaignChoice.invalidCount} 个章节 Code 格式无效，应用时会忽略。`"
        />
      </div>
    </div>

    <template #footer>
      <a-button :disabled="campaignLoading" @click="closeCampaignSelector">取消</a-button>
      <a-button
        type="primary"
        :disabled="campaignLoading || !selectedCampaignChoice"
        @click="applySelectedCampaign"
      >
        应用限定
      </a-button>
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

  .script-editor-status {
    display: flex;
    min-width: 0;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }

  :global(.campaign-limit-modal .ant-modal) {
    max-width: calc(100vw - 24px);
  }

  .campaign-selector-content {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .campaign-choice-detail {
    display: flex;
    flex-direction: column;
    gap: 8px;
    border: 1px solid #e5e7eb;
    border-radius: 8px;
    background: #f8fafc;
    padding: 12px;
  }

  .campaign-choice-title {
    color: #1f2937;
    font-size: 14px;
    font-weight: 600;
  }

  .campaign-choice-vpk {
    color: #6b7280;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 12px;
    overflow-wrap: anywhere;
  }

  .campaign-code-list {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
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

  :global(.dark .campaign-limit-modal .campaign-choice-detail) {
    border-color: #334155;
    background: #0f172a;
  }

  :global(.dark .campaign-limit-modal .campaign-choice-title) {
    color: #e2e8f0;
  }

  :global(.dark .campaign-limit-modal .campaign-choice-vpk) {
    color: #94a3b8;
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

    .script-editor-status {
      align-items: flex-start;
      flex-direction: column;
    }
  }
</style>
