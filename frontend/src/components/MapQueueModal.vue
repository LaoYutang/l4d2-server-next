<script setup lang="ts">
  import { computed, ref, watch } from 'vue';
  import { message, Modal } from 'ant-design-vue';
  import {
    ClearOutlined,
    ClockCircleOutlined,
    DeleteOutlined,
    PauseCircleOutlined,
    PlayCircleOutlined,
    PlusOutlined,
    ReloadOutlined,
    StepForwardOutlined,
  } from '@ant-design/icons-vue';
  import {
    api,
    type MapQueueActionResponse,
    type MapQueueItem,
    type MapQueueRunState,
    type MapQueueSnapshot,
  } from '../services/api';
  import { buildMapCatalog, type MapCatalogCampaign } from '../utils/mapCatalog';

  const props = defineProps<{
    open: boolean;
  }>();

  const emit = defineEmits<{
    (event: 'update:open', value: boolean): void;
  }>();

  const snapshot = ref<MapQueueSnapshot | null>(null);
  const snapshotLoading = ref(false);
  const snapshotError = ref('');
  const stale = ref(false);
  const activeTab = ref('queue');
  const actionKey = ref('');
  let snapshotRequestId = 0;

  const allMaps = ref<MapCatalogCampaign[]>([]);
  const catalogLoading = ref(false);
  const catalogLoaded = ref(false);
  const catalogError = ref('');
  const searchText = ref('');
  const showOfficial = ref(true);
  const activeCampaignKeys = ref<string[]>([]);
  const addPosition = ref<'front' | 'back'>('back');

  const stateLabels: Record<MapQueueRunState, string> = {
    stopped: '已停止',
    armed: '等待通关',
    running: '执行中',
    delay: '等待切换',
    paused: '已暂停',
  };

  const stateColors: Record<MapQueueRunState, string> = {
    stopped: 'default',
    armed: 'blue',
    running: 'green',
    delay: 'orange',
    paused: 'orange',
  };

  const currentStateLabel = computed(() => {
    const state = snapshot.value?.state;
    return state ? stateLabels[state] : '未知';
  });

  const currentStateColor = computed(() => {
    const state = snapshot.value?.state;
    return state ? stateColors[state] : 'default';
  });

  const pendingCount = computed(() => snapshot.value?.pending.length ?? 0);
  const queueHasContent = computed(() => Boolean(snapshot.value?.active) || pendingCount.value > 0);
  const actionsAvailable = computed(
    () =>
      Boolean(snapshot.value?.installed && snapshot.value.enabled) &&
      !stale.value &&
      !snapshotLoading.value &&
      !actionKey.value
  );
  const canStartNow = computed(() => {
    const state = snapshot.value?.state;
    return (
      actionsAvailable.value &&
      Boolean(snapshot.value?.supported) &&
      ((pendingCount.value > 0 && (state === 'stopped' || state === 'armed')) ||
        (state === 'paused' && Boolean(snapshot.value?.active)))
    );
  });
  const canStartAfterCampaign = computed(
    () =>
      actionsAvailable.value &&
      Boolean(snapshot.value?.supported) &&
      pendingCount.value > 0 &&
      snapshot.value?.state === 'stopped'
  );
  const canPause = computed(() => {
    const state = snapshot.value?.state;
    return (
      actionsAvailable.value &&
      Boolean(state && (state === 'armed' || state === 'running' || state === 'delay'))
    );
  });
  const canClear = computed(
    () => actionsAvailable.value && (queueHasContent.value || snapshot.value?.state !== 'stopped')
  );
  const canAdd = computed(() => actionsAvailable.value);

  const filteredMaps = computed(() => {
    let result = allMaps.value;
    if (!showOfficial.value) {
      result = result.filter((campaign) => campaign.IsCustom);
    }

    const keyword = searchText.value.trim().toLowerCase();
    if (!keyword) return result;
    return result.filter(
      (campaign) =>
        campaign.Title.toLowerCase().includes(keyword) ||
        campaign.VpkName?.toLowerCase().includes(keyword) ||
        campaign.Chapters.some(
          (chapter) =>
            chapter.Code.toLowerCase().includes(keyword) ||
            chapter.Title?.toLowerCase().includes(keyword)
        )
    );
  });

  const normalizeError = (error: unknown, fallback = '操作失败') => {
    const detail = error instanceof Error ? error.message.trim() : String(error || '').trim();
    if (/RCON|服务端未配置|连接失败|命令执行失败/i.test(detail)) return 'RCON 连接失败';
    return detail || fallback;
  };

  const fetchSnapshot = async () => {
    const requestId = ++snapshotRequestId;
    snapshotLoading.value = true;
    snapshotError.value = '';
    try {
      const result = await api.getMapQueueSnapshot();
      if (requestId !== snapshotRequestId) return;
      snapshot.value = result;
      stale.value = false;
    } catch (error) {
      if (requestId !== snapshotRequestId) return;
      snapshotError.value = normalizeError(error, '获取地图待办队列失败');
      stale.value = snapshot.value !== null;
    } finally {
      if (requestId === snapshotRequestId) snapshotLoading.value = false;
    }
  };

  const fetchMapCatalog = async () => {
    if (catalogLoading.value) return;
    catalogLoading.value = true;
    catalogError.value = '';
    try {
      allMaps.value = buildMapCatalog(await api.getRconMapList());
      catalogLoaded.value = true;
    } catch (error) {
      catalogError.value = normalizeError(error, '获取地图章节缓存失败');
    } finally {
      catalogLoading.value = false;
    }
  };

  const applyActionResponse = (response: MapQueueActionResponse) => {
    if (response.map_change_expected) {
      stale.value = true;
      message.success('指令已发送，服务器正在换图，请稍后点击刷新');
      return;
    }

    message.success(response.message);
    if (response.snapshot) {
      snapshot.value = response.snapshot;
      snapshotError.value = '';
      stale.value = false;
    } else {
      stale.value = true;
    }

    if (response.refresh_error) {
      message.warning(`操作已成功，但状态刷新失败：${normalizeError(response.refresh_error)}`);
    }
  };

  const runAction = async (
    key: string,
    request: () => Promise<MapQueueActionResponse>,
    addingMap = false
  ) => {
    if (actionKey.value) return false;
    actionKey.value = key;
    try {
      applyActionResponse(await request());
      return true;
    } catch (error) {
      const detail = normalizeError(error);
      if (addingMap) {
        message.error(`添加失败：${detail} 如果是刚上传的地图，请先执行“热重载地图”后再试。`);
      } else {
        message.error(detail);
      }
      return false;
    } finally {
      actionKey.value = '';
    }
  };

  const addMap = async (mapCode: string) => {
    if (!canAdd.value) return;
    const code = mapCode.trim();
    await runAction(`add:${code}`, () => api.addMapQueueItem(code, addPosition.value), true);
  };

  const sameMapCount = (mapName: string) =>
    snapshot.value?.pending.filter((item) => item.map.toLowerCase() === mapName.toLowerCase())
      .length ?? 0;

  const confirmRemove = (item: MapQueueItem) => {
    const count = sameMapCount(item.map);
    Modal.confirm({
      title: `是否删除所有 ${item.map} 地图？`,
      content: `当前待执行列表中共有 ${count} 个同名项，确认后会全部删除。`,
      okText: '全部删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: () => runAction(`remove:${item.map}`, () => api.removeMapQueueItems(item.map)),
    });
  };

  const confirmStartNow = () => {
    if (snapshot.value?.state === 'paused') {
      Modal.confirm({
        title: '恢复地图待办队列？',
        content: '当前战役仍作为执行中项目，通关后将继续进入下一项。',
        okText: '恢复队列',
        cancelText: '取消',
        onOk: () => runAction('start:now', () => api.startMapQueue('now')),
      });
      return;
    }

    Modal.confirm({
      title: '立即启动队列？',
      content: '服务器将立即切换至队首地图，切图期间 RCON 会短暂断开。',
      okText: '立即启动',
      cancelText: '取消',
      onOk: () => runAction('start:now', () => api.startMapQueue('now')),
    });
  };

  const startAfterCampaign = () =>
    runAction('start:after_campaign', () => api.startMapQueue('after_campaign'));

  const pauseQueue = () => runAction('pause', () => api.pauseMapQueue());

  const confirmClear = () => {
    Modal.confirm({
      title: '清空并停止地图待办队列？',
      content: '这会清空执行中和全部待执行地图，并停止队列。',
      okText: '清空并停止',
      okType: 'danger',
      cancelText: '取消',
      onOk: () => runAction('clear', () => api.clearMapQueue()),
    });
  };

  const confirmSkip = () => {
    const activeMap = snapshot.value?.active?.map;
    if (!activeMap) return;
    Modal.confirm({
      title: `跳过当前地图 ${activeMap}？`,
      content: '如果队列中还有有效地图，服务器可能立即切换至下一张地图。',
      okText: '跳过当前',
      cancelText: '取消',
      onOk: () => runAction('skip', () => api.skipMapQueueItem()),
    });
  };

  const formatModes = (modes: string[]) => {
    if (!modes?.length) return 'Unknown';
    const labels: Record<string, string> = {
      coop: '战役',
      realism: '写实',
      versus: '对抗',
      survival: '生存',
      scavenge: '清道夫',
    };
    return modes.map((mode) => labels[mode] || mode).join(', ');
  };

  const getModeColor = (mode: string) => {
    const colors: Record<string, string> = {
      coop: 'blue',
      realism: 'purple',
      versus: 'red',
      survival: 'orange',
      scavenge: 'green',
    };
    return colors[mode] || 'default';
  };

  watch(
    () => props.open,
    (open) => {
      if (!open) {
        snapshotRequestId++;
        return;
      }
      activeTab.value = 'queue';
      snapshot.value = null;
      snapshotError.value = '';
      stale.value = false;
      fetchSnapshot();
    }
  );

  watch(activeTab, (tab) => {
    if (tab === 'add' && !catalogLoaded.value) fetchMapCatalog();
  });
</script>

<template>
  <a-modal
    :open="open"
    title="地图待办队列"
    :footer="null"
    width="min(94vw, 920px)"
    wrap-class-name="map-queue-modal"
    centered
    @update:open="emit('update:open', $event)"
  >
    <div class="queue-modal-body">
      <a-alert
        v-if="snapshot && !snapshot.installed"
        type="warning"
        show-icon
        message="需要安装地图待办队列插件"
        description="服务器未识别 sm_mq 指令，请安装并加载[地图待办队列]插件，然后重新打开此弹框。"
      />

      <template v-else>
        <section
          class="status-panel rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-slate-900"
        >
          <div class="flex flex-wrap items-center justify-between gap-2">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-semibold text-gray-900 dark:text-gray-100">运行状态</span>
              <template v-if="snapshot">
                <a-tag :color="snapshot.installed ? 'green' : 'red'">
                  {{ snapshot.installed ? '插件已安装' : '插件未安装' }}
                </a-tag>
                <template v-if="snapshot.installed">
                  <a-tag :color="snapshot.enabled ? 'green' : 'default'">
                    {{ snapshot.enabled ? '已启用' : '未启用' }}
                  </a-tag>
                  <a-tag :color="snapshot.supported ? 'blue' : 'orange'">
                    {{ snapshot.supported ? '当前模式支持' : '当前模式不支持' }}
                  </a-tag>
                  <a-tag :color="currentStateColor">{{ currentStateLabel }}</a-tag>
                  <span class="text-xs text-gray-500 dark:text-gray-400">
                    待执行 {{ pendingCount }} 项
                  </span>
                </template>
              </template>
            </div>
            <a-button
              :loading="snapshotLoading"
              :disabled="Boolean(actionKey)"
              @click="fetchSnapshot"
            >
              <template #icon><reload-outlined /></template>
              刷新
            </a-button>
          </div>
        </section>

        <a-alert
          v-if="snapshotError"
          class="mt-3"
          type="error"
          show-icon
          :message="snapshotError"
          description="请检查 RCON 连接或插件版本后点击刷新。"
        />
        <a-alert
          v-else-if="stale"
          class="mt-3"
          type="warning"
          show-icon
          message="当前数据可能已过期"
          description="服务器可能正在换图，或操作后的状态读取失败。请稍后点击刷新。"
        />
        <a-alert
          v-else-if="snapshot && !snapshot.enabled"
          class="mt-3"
          type="warning"
          show-icon
          message="地图待办队列插件未启用"
          description="请在插件管理中启用该插件；本弹框不修改插件 CVar。"
        />
        <a-alert
          v-else-if="snapshot && !snapshot.supported"
          class="mt-3"
          type="warning"
          show-icon
          message="当前游戏模式不支持启动队列"
          description="仍可编辑列表；切换到合作、写实或合作类突变模式后再启动。"
        />

        <a-tabs v-model:activeKey="activeTab" class="queue-tabs">
          <a-tab-pane key="queue" tab="队列列表">
            <div
              v-if="snapshotLoading && !snapshot"
              class="flex min-h-48 items-center justify-center gap-2 text-gray-500 dark:text-gray-400"
            >
              <a-spin /> 正在读取队列...
            </div>
            <a-empty
              v-else-if="!snapshot || !snapshot.installed"
              :description="snapshotError ? '读取失败，请点击刷新' : '插件安装后可查看队列'"
            />
            <div v-else class="space-y-4">
              <section v-if="snapshot.active" class="active-section">
                <div class="mb-2 flex items-center justify-between gap-2">
                  <h3 class="m-0 text-sm font-semibold text-gray-900 dark:text-gray-100">
                    {{ snapshot.state === 'paused' ? '暂停中的当前地图' : '执行中' }}
                  </h3>
                  <a-tag :color="snapshot.state === 'paused' ? 'orange' : 'green'">
                    {{ snapshot.state === 'paused' ? '队列已暂停' : '当前地图' }}
                  </a-tag>
                </div>
                <div
                  class="queue-card"
                  :class="
                    snapshot.state === 'paused'
                      ? 'border-orange-200 bg-orange-50/70 dark:border-orange-800 dark:bg-orange-950/20'
                      : 'border-green-200 bg-green-50/70 dark:border-green-800 dark:bg-green-950/20'
                  "
                >
                  <div class="min-w-0 flex-1">
                    <div class="truncate font-semibold text-gray-900 dark:text-gray-100">
                      {{ snapshot.active.mission_name || snapshot.active.map }}
                      <span v-if="snapshot.active.chapter_name" class="font-normal">
                        · {{ snapshot.active.chapter_name }}
                      </span>
                    </div>
                    <div class="mt-1 break-all font-mono text-xs text-gray-500 dark:text-gray-400">
                      {{ snapshot.active.map }}
                    </div>
                  </div>
                  <a-button
                    danger
                    class="shrink-0"
                    :disabled="!actionsAvailable"
                    :loading="actionKey === 'skip'"
                    @click="confirmSkip"
                  >
                    <template #icon><step-forward-outlined /></template>
                    跳过当前
                  </a-button>
                </div>
              </section>

              <section>
                <div class="mb-2 flex items-center justify-between gap-2">
                  <h3 class="m-0 text-sm font-semibold text-gray-900 dark:text-gray-100">
                    待执行列表
                  </h3>
                  <span class="text-xs text-gray-500 dark:text-gray-400">
                    共 {{ snapshot.pending.length }} 项
                  </span>
                </div>
                <div
                  v-if="snapshot.pending.length"
                  class="pending-list custom-scrollbar space-y-2 pr-1"
                >
                  <div
                    v-for="(item, position) in snapshot.pending"
                    :key="`${item.index}:${item.map}`"
                    class="queue-card border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900"
                  >
                    <div class="flex min-w-0 flex-1 items-start gap-3">
                      <span class="queue-index">{{ position + 1 }}</span>
                      <div class="min-w-0 flex-1">
                        <div class="flex flex-wrap items-center gap-1.5">
                          <span class="font-semibold text-gray-900 dark:text-gray-100">
                            {{ item.mission_name || item.map }}
                          </span>
                          <span
                            v-if="item.chapter_name"
                            class="text-sm text-gray-600 dark:text-gray-300"
                          >
                            · {{ item.chapter_name }}
                          </span>
                          <a-tag :color="item.official ? 'blue' : 'purple'" class="!m-0">
                            {{ item.official ? '官方' : '三方' }}
                          </a-tag>
                          <a-tag v-if="sameMapCount(item.map) > 1" color="orange" class="!m-0">
                            同名 ×{{ sameMapCount(item.map) }}
                          </a-tag>
                        </div>
                        <div
                          class="mt-1 break-all font-mono text-xs text-gray-500 dark:text-gray-400"
                        >
                          {{ item.map }}
                        </div>
                      </div>
                    </div>
                    <a-button
                      danger
                      class="shrink-0"
                      :disabled="!actionsAvailable"
                      :loading="actionKey === `remove:${item.map}`"
                      @click="confirmRemove(item)"
                    >
                      <template #icon><delete-outlined /></template>
                      删除
                    </a-button>
                  </div>
                </div>
                <a-empty v-else description="暂无待执行地图" />
              </section>
            </div>
          </a-tab-pane>

          <a-tab-pane key="add" tab="添加地图" :disabled="!snapshot?.installed">
            <div class="space-y-4">
              <div
                class="flex flex-col gap-3 rounded-lg border border-gray-200 p-3 dark:border-gray-700 sm:flex-row sm:items-center sm:justify-between"
              >
                <div>
                  <div class="font-semibold text-gray-900 dark:text-gray-100">加入位置</div>
                  <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                    每次仅添加一张地图
                  </div>
                </div>
                <a-radio-group v-model:value="addPosition" button-style="solid" :disabled="!canAdd">
                  <a-radio-button value="back">加入队尾</a-radio-button>
                  <a-radio-button value="front">加入队首</a-radio-button>
                </a-radio-group>
              </div>

              <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
                <a-switch
                  v-model:checked="showOfficial"
                  checked-children="显示官图"
                  un-checked-children="隐藏官图"
                  class="self-start sm:self-center"
                />
                <a-input-search
                  v-model:value="searchText"
                  placeholder="搜索战役、章节或地图代码..."
                  allow-clear
                  class="w-full sm:flex-1"
                />
                <a-button
                  :loading="catalogLoading"
                  @click="fetchMapCatalog"
                >
                  <template #icon><reload-outlined /></template>
                  刷新目录
                </a-button>
              </div>

              <div class="map-catalog-list custom-scrollbar space-y-2 pr-1">
                <a-alert v-if="catalogError" type="error" show-icon :message="catalogError" />
                <div
                  v-if="catalogLoading && !catalogLoaded"
                  class="flex min-h-48 items-center justify-center gap-2 text-gray-500 dark:text-gray-400"
                >
                  <a-spin /> 正在读取地图章节缓存...
                </div>
                <a-empty v-else-if="filteredMaps.length === 0" description="未找到匹配的地图" />
                <a-collapse v-else v-model:activeKey="activeCampaignKeys" ghost accordion>
                  <a-collapse-panel
                    v-for="(campaign, campaignIndex) in filteredMaps"
                    :key="`${campaign.IsCustom ? 'custom' : 'official'}:${campaign.Title}:${campaignIndex}`"
                    class="map-collapse-panel mb-2 overflow-hidden rounded-lg bg-gray-50 dark:bg-slate-900/20"
                  >
                    <template #header>
                      <div class="flex w-full items-center gap-2 py-1">
                        <span class="mr-1 text-xl">{{ campaign.IsCustom ? '🗺️' : '🏛️' }}</span>
                        <div class="min-w-0 flex-1">
                          <div class="flex flex-wrap items-center gap-1.5">
                            <span class="font-bold text-gray-900 dark:text-gray-100">
                              {{ campaign.Title }}
                            </span>
                            <a-tag :color="campaign.IsCustom ? 'purple' : 'blue'" class="!m-0">
                              {{ campaign.IsCustom ? '三方' : '官方' }}
                            </a-tag>
                            <a-tag class="!m-0">{{ campaign.Chapters.length }} 章</a-tag>
                          </div>
                          <div
                            v-if="campaign.IsCustom && campaign.VpkName"
                            class="mt-0.5 truncate text-xs text-gray-400 dark:text-gray-500"
                          >
                            {{ campaign.VpkName }}
                          </div>
                        </div>
                      </div>
                    </template>

                    <div class="chapter-grid">
                      <article
                        v-for="chapter in campaign.Chapters"
                        :key="chapter.Code"
                        class="chapter-card border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900"
                      >
                        <div class="min-w-0 flex-1">
                          <div
                            class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100"
                          >
                            {{ chapter.Title || chapter.Code }}
                          </div>
                          <div
                            class="mt-1 break-all font-mono text-xs text-gray-500 dark:text-gray-400"
                          >
                            {{ chapter.Code }}
                          </div>
                          <div class="mt-2 flex flex-wrap gap-1">
                            <a-tag
                              v-for="mode in chapter.Modes || []"
                              :key="mode"
                              :color="getModeColor(mode)"
                              class="!m-0 !text-xs"
                            >
                              {{ formatModes([mode]) }}
                            </a-tag>
                          </div>
                        </div>
                        <a-button
                          type="primary"
                          class="shrink-0"
                          :disabled="!canAdd"
                          :loading="actionKey === `add:${chapter.Code}`"
                          @click="addMap(chapter.Code)"
                        >
                          <template #icon><plus-outlined /></template>
                          添加
                        </a-button>
                      </article>
                    </div>
                  </a-collapse-panel>
                </a-collapse>
              </div>
            </div>
          </a-tab-pane>
        </a-tabs>

        <div
          class="queue-footer-actions mt-3 flex flex-wrap items-center justify-end gap-2 border-t border-gray-200 pt-3 dark:border-gray-700"
        >
          <a-button-group>
            <a-button
              type="primary"
              :disabled="!canStartNow"
              :loading="actionKey === 'start:now'"
              @click="confirmStartNow"
            >
              <template #icon><play-circle-outlined /></template>
              立即启动队列
            </a-button>
            <a-tooltip title="通关后启动" placement="top">
              <a-button
                type="primary"
                class="queue-start-after-button"
                aria-label="通关后启动"
                :disabled="!canStartAfterCampaign"
                :loading="actionKey === 'start:after_campaign'"
                @click="startAfterCampaign"
              >
                <template #icon><clock-circle-outlined /></template>
              </a-button>
            </a-tooltip>
          </a-button-group>

          <a-button
            :disabled="!canPause"
            :loading="actionKey === 'pause'"
            @click="pauseQueue"
          >
            <template #icon><pause-circle-outlined /></template>
            暂停队列
          </a-button>

          <a-button
            danger
            :disabled="!canClear"
            :loading="actionKey === 'clear'"
            @click="confirmClear"
          >
            <template #icon><clear-outlined /></template>
            清空并停止
          </a-button>
        </div>
      </template>
    </div>
  </a-modal>
</template>

<style scoped>
  .pending-list {
    max-height: min(50vh, 480px);
    overflow-y: auto;
  }

  .map-catalog-list {
    max-height: clamp(160px, calc(100dvh - 420px), 480px);
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  .queue-card {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 68px;
    border-width: 1px;
    border-radius: 8px;
    padding: 12px;
  }

  .queue-index {
    display: inline-flex;
    width: 28px;
    height: 28px;
    flex: 0 0 auto;
    align-items: center;
    justify-content: center;
    border-radius: 9999px;
    background: rgb(243 244 246);
    color: rgb(75 85 99);
    font-size: 12px;
    font-weight: 600;
  }

  :global(.dark) .queue-index {
    background: rgb(31 41 55);
    color: rgb(209 213 219);
  }

  .chapter-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: 10px;
    padding-top: 8px;
  }

  .chapter-card {
    display: flex;
    min-height: 112px;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    border-width: 1px;
    border-radius: 8px;
    padding: 12px;
  }

  .queue-modal-body :deep(.ant-btn) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .queue-modal-body :deep(.ant-btn .anticon) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    line-height: 1;
  }

  .queue-modal-body :deep(.ant-btn .anticon > svg) {
    display: block;
  }

  .queue-footer-actions :deep(.queue-start-after-button.ant-btn) {
    border-start-end-radius: 6px !important;
    border-end-end-radius: 6px !important;
  }

  .queue-footer-actions :deep(.queue-start-after-button.ant-btn-icon-only > .anticon) {
    transform: none;
  }

  :deep(.ant-collapse-header) {
    align-items: center !important;
  }

  :deep(.ant-collapse-header-text) {
    min-width: 0;
    flex-grow: 1;
  }

  :deep(.ant-collapse-ghost > .ant-collapse-item) {
    border-bottom: 0 !important;
  }

  :deep(.ant-collapse-content) {
    border-top: 0 !important;
    background: transparent !important;
  }

  :deep(.ant-collapse-content-box) {
    padding: 0 16px 18px !important;
  }

  @media (max-width: 640px) {
    .queue-modal-body {
      max-height: calc(100dvh - 116px);
      overflow-y: auto;
      padding-right: 2px;
    }

    .status-panel {
      position: sticky;
      top: 0;
      z-index: 4;
    }

    .pending-list {
      max-height: none;
      overflow: visible;
      padding-right: 0;
    }

    .map-catalog-list {
      max-height: none;
      overflow: visible;
      padding-right: 0;
    }

    .queue-card,
    .chapter-card {
      align-items: stretch;
      flex-direction: column;
    }

    .queue-card > .ant-btn,
    .chapter-card > .ant-btn {
      width: auto;
      align-self: flex-end;
    }

    .chapter-grid {
      grid-template-columns: 1fr;
    }

    :deep(.queue-modal-body .ant-input-affix-wrapper),
    :deep(.queue-modal-body .ant-input-search-button) {
      min-height: 44px;
    }
  }

  :global(.map-queue-modal .ant-modal-body) {
    padding-top: 12px;
  }

  @media (max-width: 640px) {
    :global(.map-queue-modal .ant-modal) {
      top: 8px;
      max-width: calc(100vw - 16px);
      margin: 0 8px;
      padding-bottom: 8px;
    }

    :global(.map-queue-modal .ant-modal-content) {
      min-height: calc(100dvh - 16px);
      padding: 16px 12px;
    }
  }
</style>
