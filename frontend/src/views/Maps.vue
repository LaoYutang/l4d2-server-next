<script setup lang="ts">
  import { ref, computed, onMounted, h, watch, reactive } from 'vue';
  defineOptions({ name: 'Maps' });
  import {
    api,
    type MapDictionaryChapterInspection,
    type MapMissionCampaign,
    type MapSummaryItem,
  } from '../services/api';
  import { useAuthStore } from '../stores/auth';
  import MapGlobalScriptsModal from '../components/MapGlobalScriptsModal.vue';
  import MapScriptOverridesModal from '../components/MapScriptOverridesModal.vue';
  import MapHotReloadSetting from '../components/settings/MapHotReloadSetting.vue';
  import MapHotReloadButton from '../components/MapHotReloadButton.vue';
  import MapQueueButton from '../components/MapQueueButton.vue';
  import MapUploadPanel from '../components/MapUploadPanel.vue';
  import MapDownloadPanel from '../components/MapDownloadPanel.vue';
  import { message, Modal } from 'ant-design-vue';
  import type { TablePaginationConfig } from 'ant-design-vue';
  import {
    ReloadOutlined,
    DeleteOutlined,
    FileTextOutlined,
    ExclamationCircleOutlined,
    EditOutlined,
    CompressOutlined,
    SettingOutlined,
  } from '@ant-design/icons-vue';

  const authStore = useAuthStore();
  const isAdmin = computed(() => authStore.isAdmin);
  const activeTab = ref('local');
  const maps = ref<Array<{ name: string; size: string; info: string }>>([]);
  const mapSummaries = ref<Record<string, MapSummaryItem>>({});
  const mapSummaryLoading = ref<Record<string, boolean>>({});
  const loading = ref(false);
  const searchQuery = ref('');
  const selectedRowKeys = ref<string[]>([]);
  const trimmingMaps = ref<Record<string, boolean>>({});
  const renameVisible = ref(false);
  const renamingMap = ref(false);
  const renameOldName = ref('');
  const renameNewName = ref('');
  const detailVisible = ref(false);
  const detailLoading = ref(false);
  const detailMapName = ref('');
  const detailCampaigns = ref<MapMissionCampaign[]>([]);
  const globalScriptsVisible = ref(false);
  const globalScriptsMapName = ref('');
  const scriptOverridesVisible = ref(false);
  const scriptOverridesMapName = ref('');
  const hotReloadConfigVisible = ref(false);
  const detailCampaignTitle = computed(() =>
    detailCampaigns.value.map((campaign) => campaign.Title || '未命名战役').join(' / ')
  );
  const detailModalTitle = computed(
    () => `地图详情 - ${detailCampaignTitle.value || detailMapName.value}`
  );

  // Local Maps Logic
  const loadMaps = async () => {
    loading.value = true;
    try {
      maps.value = await api.getMapList();
    } catch (e) {
      console.error(e);
      message.error('加载地图列表失败');
    } finally {
      loading.value = false;
    }
  };

  const filteredMaps = computed(() => {
    if (!searchQuery.value) return maps.value;
    const q = searchQuery.value.toLowerCase();
    return maps.value.filter((m) => {
      if (m.name.toLowerCase().includes(q)) return true;

      const summary = mapSummaries.value[m.name];
      if (!summary) return false;
      if (summary.title?.toLowerCase().includes(q)) return true;
      return summary.campaigns?.some((campaign) => campaign.toLowerCase().includes(q));
    });
  });

  const getMapSizeColor = (sizeStr: string) => {
    // Expecting size format like "123 MB", "1.5 GB", "500 KB"
    if (!sizeStr) return 'default';

    const size = parseFloat(sizeStr);
    const unit = sizeStr
      .replace(/[0-9.]/g, '')
      .trim()
      .toUpperCase();

    let sizeInMB = 0;
    if (unit.includes('G')) {
      sizeInMB = size * 1024;
    } else if (unit.includes('K')) {
      sizeInMB = size / 1024;
    } else {
      sizeInMB = size;
    }

    if (sizeInMB < 200) return 'green';
    if (sizeInMB < 500) return 'orange';
    return 'red';
  };

  const onSelectChange = (keys: any[]) => {
    selectedRowKeys.value = keys;
  };

  const batchDeleteMaps = async () => {
    if (selectedRowKeys.value.length === 0) return;

    Modal.confirm({
      title: `确定要删除选中的 ${selectedRowKeys.value.length} 个地图吗？`,
      icon: () => h(ExclamationCircleOutlined),
      content: '此操作不可逆。',
      onOk: async () => {
        let successCount = 0;
        let failCount = 0;

        for (const name of selectedRowKeys.value) {
          try {
            await api.deleteMap(name);
            successCount++;
          } catch (e) {
            console.error(`Failed to delete ${name}`, e);
            failCount++;
          }
        }

        if (failCount > 0) {
          message.warning(`删除完成: ${successCount} 个成功, ${failCount} 个失败`);
        } else {
          message.success(`成功删除 ${successCount} 个地图`);
        }

        selectedRowKeys.value = [];
        loadMaps();
      },
    });
  };

  const deleteMap = async (name: string) => {
    Modal.confirm({
      title: `确定要删除地图 ${name} 吗？`,
      icon: () => h(ExclamationCircleOutlined),
      content: '此操作不可逆。',
      onOk: async () => {
        try {
          await api.deleteMap(name);
          message.success('删除成功');
          loadMaps();
        } catch (e: any) {
          message.error('删除失败: ' + e.message);
        }
      },
    });
  };

  const trimMap = async (name: string) => {
    Modal.confirm({
      title: `确定要精简地图 ${name} 吗？`,
      icon: () => h(ExclamationCircleOutlined),
      content: '精简成功后会替换当前VPK文件。精简后的地图仅适合服务端使用，不适合客户端本地使用。',
      okText: '精简',
      onOk: async () => {
        trimmingMaps.value = { ...trimmingMaps.value, [name]: true };
        try {
          const result = await api.trimMap(name);
          if (result.trimmed) {
            message.success(result.message || `精简成功，节省 ${result.saved_size_label}`);
          } else {
            message.info(result.message || '当前地图无需精简');
          }
          loadMaps();
          loadMapSummaries([name]);
        } catch (e: any) {
          message.error('精简失败: ' + e.message);
        } finally {
          trimmingMaps.value = { ...trimmingMaps.value, [name]: false };
        }
      },
    });
  };

  const openRenameModal = (name: string) => {
    renameOldName.value = name;
    renameNewName.value = name;
    renameVisible.value = true;
  };

  const openMapDetail = async (name: string) => {
    if (!canOpenMapDetail(name)) return;

    detailMapName.value = name;
    detailCampaigns.value = [];
    detailVisible.value = true;
    detailLoading.value = true;
    try {
      const detail = await api.getMapMissionDetail(name);
      detailMapName.value = detail.name || name;
      detailCampaigns.value = detail.campaigns || [];
    } catch (e: any) {
      message.error('获取地图详情失败: ' + e.message);
      detailVisible.value = false;
    } finally {
      detailLoading.value = false;
    }
  };

  const submitRenameMap = async () => {
    const newName = renameNewName.value.trim();
    if (!renameOldName.value || !newName) {
      message.error('请输入新的地图名称');
      return;
    }

    renamingMap.value = true;
    try {
      const result = await api.renameMap(renameOldName.value, newName);
      selectedRowKeys.value = selectedRowKeys.value.map((key) =>
        key === renameOldName.value ? result.name : key
      );
      message.success(result.message || '重命名成功');
      renameVisible.value = false;
      loadMaps();
    } catch (e: any) {
      message.error('重命名失败: ' + e.message);
    } finally {
      renamingMap.value = false;
    }
  };

  const confirmClearMaps = async () => {
    Modal.confirm({
      title: '警告：这将删除所有第三方地图文件！',
      icon: () => h(ExclamationCircleOutlined),
      content: '确定继续吗？',
      okType: 'danger',
      onOk: async () => {
        try {
          await api.clearMaps();
          message.success('所有地图已清空');
          loadMaps();
        } catch (e: any) {
          message.error('清空失败: ' + e.message);
        }
      },
    });
  };

  const openHotReloadConfig = () => {
    if (!isAdmin.value) return;
    hotReloadConfigVisible.value = true;
  };

  const mapColumns = [
    { title: '地图名称', dataIndex: 'name', key: 'name' },
    { title: '大小', dataIndex: 'size', key: 'size', width: 120 },
    { title: '操作', key: 'action', width: 330, align: 'right' as const },
  ];

  const mapDetailChapterColumns = [
    { title: '章节名', dataIndex: 'Title', key: 'title', width: 180 },
    { title: '地图代码', dataIndex: 'Code', key: 'code', width: 150 },
    { title: '模式', key: 'modes', width: 360 },
  ];

  const paginationConfig = reactive<TablePaginationConfig>({
    current: 1,
    pageSize: 10,
    showSizeChanger: true,
    pageSizeOptions: ['10', '20', '50', '100'],
    showTotal: (total: number) => `共 ${total} 条`,
  });

  const handleTableChange = (pag: TablePaginationConfig) => {
    paginationConfig.current = pag.current;
    paginationConfig.pageSize = pag.pageSize;
  };

  const currentPageMaps = computed(() => {
    const current = paginationConfig.current || 1;
    const pageSize = paginationConfig.pageSize || 10;
    const start = (current - 1) * pageSize;
    return filteredMaps.value.slice(start, start + pageSize);
  });

  const currentPageMapNames = computed(() => currentPageMaps.value.map((map) => map.name));
  const currentPageMapNamesKey = computed(() => currentPageMapNames.value.join('\0'));

  const loadMapSummaries = async (names: string[]) => {
    const uniqueNames = Array.from(new Set(names.filter(Boolean)));

    if (uniqueNames.length === 0) {
      return;
    }

    const loadingPatch = uniqueNames.reduce<Record<string, boolean>>((acc, name) => {
      acc[name] = true;
      return acc;
    }, {});
    mapSummaryLoading.value = { ...mapSummaryLoading.value, ...loadingPatch };

    try {
      const items = await api.getMapSummaries(uniqueNames);
      mapSummaries.value = { ...mapSummaries.value, ...items };
    } catch (e) {
      console.error('Failed to load map summaries', e);
    } finally {
      const nextLoading = { ...mapSummaryLoading.value };
      uniqueNames.forEach((name) => {
        nextLoading[name] = false;
      });
      mapSummaryLoading.value = nextLoading;
    }
  };

  const loadCurrentPageMapSummaries = () => loadMapSummaries(currentPageMapNames.value);

  const loadSearchMapSummaries = () => {
    if (!searchQuery.value.trim()) return;
    loadMapSummaries(maps.value.map((map) => map.name));
  };

  const getMapSummaryTitle = (name: string) => mapSummaries.value[name]?.title || '';
  const getMapSummaryChapterCount = (name: string) => mapSummaries.value[name]?.chapter_count || 0;
  const canOpenMapDetail = (name: string) => getMapSummaryChapterCount(name) > 0;
  const getMapSummaryError = (name: string) => mapSummaries.value[name]?.error || '';
  const isMapSummaryLoading = (name: string) => !!mapSummaryLoading.value[name];

  const getMapInspection = (name: string) => mapSummaries.value[name]?.inspection;

  const getMissingDictionaryChapters = (name: string) =>
    (getMapInspection(name)?.dictionary.chapters || []).filter(
      (chapter) => chapter.status === 'missing'
    );

  const getUnreadableDictionaryChapters = (name: string) =>
    (getMapInspection(name)?.dictionary.chapters || []).filter(
      (chapter) => chapter.status === 'unreadable'
    );

  const hasDictionaryInspectionError = (name: string) =>
    getMapInspection(name)?.dictionary.status === 'unreadable' ||
    getUnreadableDictionaryChapters(name).length > 0;

  const getGlobalScriptCount = (name: string) => {
    const globalScriptsInspection = getMapInspection(name)?.global_scripts;
    if (globalScriptsInspection?.status !== 'detected') return 0;
    return globalScriptsInspection.files?.length || 0;
  };

  const getScriptOverrideCount = (name: string) => {
    const scriptOverridesInspection = getMapInspection(name)?.script_overrides;
    if (scriptOverridesInspection?.status !== 'detected') return 0;
    return scriptOverridesInspection.files?.length || 0;
  };

  const getChapterDisplayName = (chapter: MapDictionaryChapterInspection) => {
    const campaignTitle = chapter.campaign_title?.trim();
    const chapterTitle = chapter.chapter_title?.trim();
    if (campaignTitle && chapterTitle) return `${campaignTitle} / ${chapterTitle}`;
    if (chapterTitle) return chapterTitle;
    if (campaignTitle) return campaignTitle;
    return chapter.chapter_code?.trim() || chapter.bsp_path?.trim() || '未知章节';
  };

  const getChapterDisplayCode = (chapter: MapDictionaryChapterInspection) => {
    if (!chapter.campaign_title?.trim() && !chapter.chapter_title?.trim()) return '';
    return chapter.chapter_code?.trim() || chapter.bsp_path?.trim() || '';
  };

  const activatePopoverTag = (event: KeyboardEvent) => {
    (event.currentTarget as HTMLElement | null)?.click();
  };

  // 表格单元格的浮层挂到 body：App 级 getPopupContainer 返回触发元素的父节点，
  // 而该节点未定位、最近的定位祖先却是 position: relative 的 td，
  // 会让气泡偏移到表头位置。
  const getBody = () => document.body;

  const openGlobalScripts = (mapName: string) => {
    globalScriptsMapName.value = mapName;
    globalScriptsVisible.value = true;
  };

  const openScriptOverrides = (mapName: string) => {
    scriptOverridesMapName.value = mapName;
    scriptOverridesVisible.value = true;
  };

  const handleGlobalScriptsUpdated = (mapName: string) => {
    void Promise.all([loadMaps(), loadMapSummaries([mapName])]);
  };

  watch(searchQuery, () => {
    paginationConfig.current = 1;
    loadSearchMapSummaries();
  });

  watch(maps, () => {
    loadSearchMapSummaries();
  });

  watch(currentPageMapNamesKey, () => {
    loadCurrentPageMapSummaries();
  });

  onMounted(() => {
    loadMaps();
  });

</script>

<template>
  <div class="h-full">
    <a-tabs v-model:activeKey="activeTab" type="card">
      <a-tab-pane key="local" tab="地图管理">
        <div class="space-y-4">
          <!-- Actions Bar -->
          <div class="flex flex-col md:flex-row justify-between gap-4">
            <div class="w-full md:w-1/3">
              <a-input-search v-model:value="searchQuery" placeholder="搜索地图..." allow-clear />
            </div>
            <div class="flex gap-2 flex-wrap">
              <a-button
                v-if="selectedRowKeys.length > 0"
                danger
                @click="batchDeleteMaps"
                class="!flex !items-center !justify-center"
              >
                <template #icon><delete-outlined /></template>
                <span class="hidden sm:inline">删除选中</span>
                <span class="sm:hidden">删除</span>
                ({{ selectedRowKeys.length }})
              </a-button>
              <a-button
                @click="loadMaps"
                :loading="loading"
                class="!flex !items-center !justify-center"
              >
                <template #icon><reload-outlined /></template>
                刷新
              </a-button>
              <a-button-group class="!flex">
                <MapHotReloadButton />
                <a-button
                  v-if="isAdmin"
                  @click="openHotReloadConfig"
                  class="!flex !items-center !justify-center"
                  title="设置热重载指令"
                >
                  <template #icon><setting-outlined /></template>
                </a-button>
              </a-button-group>
              <MapQueueButton />
              <a-button
                danger
                @click="confirmClearMaps"
                class="!flex !items-center !justify-center"
              >
                <template #icon><delete-outlined /></template>
                清空地图
              </a-button>
            </div>
          </div>

          <!-- Maps Table -->
          <a-table
            :columns="mapColumns"
            :dataSource="filteredMaps"
            :loading="loading"
            :pagination="paginationConfig"
            @change="handleTableChange"
            rowKey="name"
            :row-selection="{ selectedRowKeys: selectedRowKeys, onChange: onSelectChange }"
            :scroll="{ x: 940 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <div class="min-w-[180px]">
                  <div class="min-w-0 flex flex-col gap-0.5">
                    <span class="font-medium break-all text-sm dark:text-gray-200">{{
                      record.name
                    }}</span>
                    <span
                      v-if="isMapSummaryLoading(record.name)"
                      class="text-xs text-gray-400 dark:text-gray-500"
                    >
                      读取中...
                    </span>
                    <span
                      v-else-if="getMapSummaryTitle(record.name)"
                      class="block max-w-[480px] truncate text-xs text-gray-500 dark:text-gray-400"
                      :title="getMapSummaryTitle(record.name)"
                    >
                      {{ getMapSummaryTitle(record.name) }}
                      <template v-if="getMapSummaryChapterCount(record.name) > 0">
                        · {{ getMapSummaryChapterCount(record.name) }} 章节
                      </template>
                    </span>
                    <span
                      v-else-if="getMapSummaryError(record.name)"
                      class="text-xs text-gray-400 dark:text-gray-500"
                      :title="getMapSummaryError(record.name)"
                    >
                      未识别
                    </span>
                    <div
                      v-if="!isMapSummaryLoading(record.name)"
                      class="mt-1 flex max-w-full flex-wrap gap-1"
                    >
                      <a-popover
                        v-if="getMissingDictionaryChapters(record.name).length > 0"
                        title="缺失字典的章节"
                        trigger="click"
                        placement="bottomLeft"
                        overlayClassName="map-inspection-popover"
                        :getPopupContainer="getBody"
                      >
                        <template #content>
                          <div class="map-inspection-list" role="list">
                            <div
                              v-for="chapter in getMissingDictionaryChapters(record.name)"
                              :key="chapter.bsp_path"
                              class="map-inspection-item"
                              role="listitem"
                            >
                              <div class="map-inspection-primary">
                                {{ getChapterDisplayName(chapter) }}
                              </div>
                              <div
                                v-if="getChapterDisplayCode(chapter)"
                                class="map-inspection-secondary"
                              >
                                {{ getChapterDisplayCode(chapter) }}
                              </div>
                            </div>
                          </div>
                        </template>
                        <a-tag
                          color="red"
                          class="clickable-risk-tag"
                          role="button"
                          tabindex="0"
                          @keydown.enter.prevent.stop="activatePopoverTag"
                          @keydown.space.prevent.stop="activatePopoverTag"
                        >
                          字典缺失 {{ getMissingDictionaryChapters(record.name).length }}
                        </a-tag>
                      </a-popover>

                      <a-popover
                        v-if="hasDictionaryInspectionError(record.name)"
                        title="字典检测异常"
                        trigger="click"
                        placement="bottomLeft"
                        overlayClassName="map-inspection-popover"
                        :getPopupContainer="getBody"
                      >
                        <template #content>
                          <div
                            v-if="getUnreadableDictionaryChapters(record.name).length > 0"
                            class="map-inspection-list"
                            role="list"
                          >
                            <div
                              v-for="chapter in getUnreadableDictionaryChapters(record.name)"
                              :key="chapter.bsp_path"
                              class="map-inspection-item"
                              role="listitem"
                            >
                              <div class="map-inspection-primary">{{ chapter.bsp_path }}</div>
                              <div class="map-inspection-secondary">
                                {{ chapter.message || '无法解析 BSP' }}
                              </div>
                            </div>
                          </div>
                          <div v-else class="map-inspection-empty">
                            无法解析 VPK 目录或 BSP 内容。
                          </div>
                        </template>
                        <a-tag
                          color="orange"
                          class="clickable-risk-tag"
                          role="button"
                          tabindex="0"
                          @keydown.enter.prevent.stop="activatePopoverTag"
                          @keydown.space.prevent.stop="activatePopoverTag"
                        >
                          字典检测异常
                        </a-tag>
                      </a-popover>

                      <a-tag
                        v-if="getGlobalScriptCount(record.name) > 0"
                        color="orange"
                        class="clickable-risk-tag"
                        role="button"
                        tabindex="0"
                        @click="openGlobalScripts(record.name)"
                        @keydown.enter.prevent.stop="openGlobalScripts(record.name)"
                        @keydown.space.prevent.stop="openGlobalScripts(record.name)"
                      >
                        存在全局脚本 {{ getGlobalScriptCount(record.name) }}
                      </a-tag>

                      <a-tag
                        v-if="getScriptOverrideCount(record.name) > 0"
                        color="red"
                        class="clickable-risk-tag"
                        role="button"
                        tabindex="0"
                        @click="openScriptOverrides(record.name)"
                        @keydown.enter.prevent.stop="openScriptOverrides(record.name)"
                        @keydown.space.prevent.stop="openScriptOverrides(record.name)"
                      >
                        脚本覆盖 {{ getScriptOverrideCount(record.name) }}
                      </a-tag>
                    </div>
                  </div>
                </div>
              </template>
              <template v-else-if="column.key === 'size'">
                <a-tag :color="getMapSizeColor(record.size)">
                  {{ record.size }}
                </a-tag>
              </template>
              <template v-else-if="column.key === 'action'">
                <a-space>
                  <a-button
                    size="small"
                    type="text"
                    :disabled="record.size === 'unknown' || !canOpenMapDetail(record.name)"
                    @click="openMapDetail(record.name)"
                    class="!flex !items-center !justify-center"
                    :title="canOpenMapDetail(record.name) ? '详情' : '未识别到章节，详情不可用'"
                  >
                    <template #icon><file-text-outlined /></template>
                    <span class="hidden sm:inline">详情</span>
                  </a-button>
                  <a-button
                    v-if="record.size !== 'unknown'"
                    size="small"
                    type="text"
                    @click="openRenameModal(record.name)"
                    class="!flex !items-center !justify-center"
                    title="重命名"
                  >
                    <template #icon><edit-outlined /></template>
                    <span class="hidden sm:inline">重命名</span>
                  </a-button>
                  <a-button
                    v-if="record.size !== 'unknown'"
                    size="small"
                    type="text"
                    :loading="trimmingMaps[record.name]"
                    :disabled="trimmingMaps[record.name]"
                    @click="trimMap(record.name)"
                    class="!flex !items-center !justify-center"
                    title="精简"
                  >
                    <template #icon><compress-outlined /></template>
                    <span class="hidden sm:inline">精简</span>
                  </a-button>
                  <a-button
                    size="small"
                    danger
                    type="text"
                    @click="deleteMap(record.name)"
                    class="!flex !items-center !justify-center"
                    title="删除"
                  >
                    <template #icon><delete-outlined /></template>
                    <span class="hidden sm:inline">删除</span>
                  </a-button>
                </a-space>
              </template>
            </template>
          </a-table>
        </div>

        <a-modal
          v-model:open="renameVisible"
          title="重命名地图"
          @ok="submitRenameMap"
          :confirmLoading="renamingMap"
        >
          <div class="space-y-3">
            <div class="text-sm text-gray-500 dark:text-gray-400 break-all">
              当前名称: {{ renameOldName }}
            </div>
            <a-input
              v-model:value="renameNewName"
              placeholder="请输入新的地图名称"
              @pressEnter="submitRenameMap"
            />
            <div class="text-xs text-gray-400 dark:text-gray-500">
              保存时会自动清理特殊字符，未填写 .vpk 时会自动补齐。
            </div>
          </div>
        </a-modal>

        <a-modal
          v-model:open="hotReloadConfigVisible"
          title="热重载地图设置"
          :footer="null"
        >
          <MapHotReloadSetting
            :active="hotReloadConfigVisible"
            context="modal"
            @saved="hotReloadConfigVisible = false"
            @cancel="hotReloadConfigVisible = false"
          />
        </a-modal>

        <a-modal
          v-model:open="detailVisible"
          :title="detailModalTitle"
          :footer="null"
          width="820px"
          wrap-class-name="map-detail-modal"
        >
          <div v-if="detailLoading" class="py-10 text-center text-gray-400">加载中...</div>
          <a-empty v-else-if="detailCampaigns.length === 0" description="未解析到战役信息" />
          <div v-else class="space-y-5">
            <div
              class="rounded-lg border border-gray-200 bg-gray-50/80 p-4 dark:border-slate-700 dark:bg-slate-900/30"
            >
              <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div class="flex min-w-0 items-start gap-3">
                  <div
                    class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-500/10 dark:text-blue-300"
                  >
                    <file-text-outlined />
                  </div>
                  <div class="min-w-0">
                    <div class="text-xs font-medium text-gray-500 dark:text-gray-400">战役名</div>
                    <div
                      class="mt-1 break-words text-lg font-semibold text-gray-900 dark:text-gray-100"
                    >
                      {{ detailCampaignTitle }}
                    </div>
                  </div>
                </div>
                <div class="min-w-0 sm:max-w-[45%] sm:text-right">
                  <div class="text-xs font-medium text-gray-500 dark:text-gray-400">VPK 文件</div>
                  <div
                    class="mt-1 truncate text-sm font-medium text-gray-700 dark:text-gray-200"
                    :title="detailMapName"
                  >
                    {{ detailMapName }}
                  </div>
                </div>
              </div>
            </div>

            <div
              v-for="campaign in detailCampaigns"
              :key="`${campaign.VpkName || detailMapName}-${campaign.Title}`"
              class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-slate-700 dark:bg-slate-900/40"
            >
              <div
                v-if="detailCampaigns.length > 1"
                class="border-b border-gray-100 bg-gray-50/70 px-4 py-3 dark:border-slate-700 dark:bg-slate-800/40"
              >
                <div class="break-words text-base font-semibold text-gray-900 dark:text-gray-100">
                  {{ campaign.Title || '未命名战役' }}
                </div>
                <div class="mt-1 break-all text-xs text-gray-500 dark:text-gray-400">
                  {{ campaign.VpkName || detailMapName }}
                </div>
              </div>

              <a-table
                class="map-detail-chapter-table"
                :columns="mapDetailChapterColumns"
                :data-source="campaign.Chapters || []"
                :pagination="false"
                :row-key="(chapter) => chapter.Code || chapter.Title"
                size="small"
                table-layout="fixed"
                :scroll="{ x: 690 }"
              >
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'title'">
                    <span class="block truncate pr-3" :title="record.Title || '-'">
                      {{ record.Title || '-' }}
                    </span>
                  </template>
                  <template v-else-if="column.key === 'code'">
                    <code
                      class="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-700 dark:bg-slate-800 dark:text-gray-200"
                      >{{ record.Code || '-' }}</code
                    >
                  </template>
                  <template v-else-if="column.key === 'modes'">
                    <div v-if="record.Modes?.length" class="flex min-w-[260px] flex-wrap gap-1">
                      <a-tag
                        v-for="mode in record.Modes"
                        :key="mode"
                        color="default"
                        class="mr-0 map-detail-mode-tag"
                      >
                        {{ mode }}
                      </a-tag>
                    </div>
                    <span v-else class="text-gray-400">-</span>
                  </template>
                </template>
              </a-table>
            </div>
          </div>
        </a-modal>

        <MapGlobalScriptsModal
          v-model:open="globalScriptsVisible"
          :map-name="globalScriptsMapName"
          @updated="handleGlobalScriptsUpdated"
        />

        <MapScriptOverridesModal
          v-model:open="scriptOverridesVisible"
          :map-name="scriptOverridesMapName"
        />
      </a-tab-pane>

      <a-tab-pane key="upload" tab="上传地图">
        <MapUploadPanel @uploaded="loadMaps" />
      </a-tab-pane>

      <a-tab-pane key="download" tab="下载任务">
        <MapDownloadPanel :active="activeTab === 'download'" />
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<style scoped>
  :deep(.clickable-risk-tag) {
    margin-inline-end: 0;
    cursor: pointer;
    font-size: 12px;
    user-select: none;
  }

  :deep(.clickable-risk-tag:focus-visible) {
    outline: 2px solid #1677ff;
    outline-offset: 2px;
  }

  :global(.map-inspection-popover) {
    max-width: min(420px, calc(100vw - 24px));
  }

  :global(.map-inspection-popover .ant-popover-inner) {
    max-width: min(420px, calc(100vw - 24px));
  }

  :global(.map-inspection-list) {
    display: flex;
    max-height: min(420px, 55vh);
    min-width: min(300px, calc(100vw - 64px));
    flex-direction: column;
    gap: 8px;
    overflow-y: auto;
  }

  :global(.map-inspection-item) {
    border-bottom: 1px solid #f0f0f0;
    padding-bottom: 8px;
    overflow-wrap: anywhere;
  }

  :global(.map-inspection-item:last-child) {
    border-bottom: 0;
    padding-bottom: 0;
  }

  :global(.map-inspection-primary) {
    color: #1f2937;
    font-size: 13px;
    font-weight: 600;
    line-height: 1.45;
  }

  :global(.map-inspection-secondary),
  :global(.map-inspection-empty) {
    margin-top: 2px;
    color: #6b7280;
    font-size: 12px;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }

  :global(.dark .map-inspection-item) {
    border-bottom-color: #334155;
  }

  :global(.dark .map-inspection-primary) {
    color: #e2e8f0;
  }

  :global(.dark .map-inspection-secondary),
  :global(.dark .map-inspection-empty) {
    color: #94a3b8;
  }

  :global(.map-detail-modal .ant-modal-body) {
    padding-top: 14px;
  }

  :global(.dark .map-detail-modal) {
    --ant-color-split: #334155;
    --ant-color-border-secondary: #334155;
  }

  @media (max-width: 640px) {
    :global(.map-inspection-list) {
      min-width: 0;
      width: calc(100vw - 64px);
    }
  }

  :deep(.map-detail-chapter-table .ant-table) {
    border-radius: 0;
    background: transparent;
  }

  :deep(.map-detail-chapter-table .ant-table-thead > tr > th) {
    background: transparent;
    color: #4b5563;
    font-size: 12px;
    font-weight: 600;
    padding: 10px 16px;
    border-bottom: 0 !important;
  }

  :deep(.map-detail-chapter-table .ant-table-thead),
  :deep(.map-detail-chapter-table .ant-table-thead > tr) {
    border-bottom: 0 !important;
  }

  :deep(.map-detail-chapter-table .ant-table-thead > tr > th::before) {
    display: none !important;
  }

  :deep(.map-detail-chapter-table .ant-table-tbody > tr > td) {
    padding: 10px 16px;
    border-bottom-color: #f1f5f9;
  }

  :deep(.map-detail-chapter-table .ant-table-tbody > tr:last-child > td) {
    border-bottom: 0;
  }

  :deep(.map-detail-mode-tag) {
    line-height: 20px;
    border-radius: 5px;
  }

  :global(.dark .map-detail-chapter-table .ant-table-thead > tr > th) {
    color: #cbd5e1;
    border-bottom: 0 !important;
  }

  :global(.dark .map-detail-chapter-table .ant-table-thead),
  :global(.dark .map-detail-chapter-table .ant-table-thead > tr) {
    border-bottom: 0 !important;
  }

  :global(.dark .map-detail-chapter-table .ant-table-tbody > tr > td) {
    border-bottom: 1px solid #1e293b !important;
  }

  :global(.dark .map-detail-chapter-table .ant-table-header),
  :global(.dark .map-detail-chapter-table .ant-table-container),
  :global(.dark .map-detail-chapter-table .ant-table-content),
  :global(.dark .map-detail-chapter-table table) {
    border-color: #334155 !important;
  }

</style>
