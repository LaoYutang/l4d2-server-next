<script setup lang="ts">
  import { computed, h, onBeforeUnmount, reactive, ref, watch } from 'vue';
  import { message, Modal } from 'ant-design-vue';
  import type { TablePaginationConfig } from 'ant-design-vue';
  import {
    CloseCircleOutlined,
    DownloadOutlined,
    ExclamationCircleOutlined,
    FileTextOutlined,
    PlusOutlined,
    ReloadOutlined,
    SearchOutlined,
    SettingOutlined,
  } from '@ant-design/icons-vue';
  import {
    api,
    type DownloadTask,
    type DownloadLinkParseResult,
    type ParsedDownloadItem,
  } from '../services/api';
  import { useAuthStore } from '../stores/auth';
  import SteamCDNSetting from './settings/SteamCDNSetting.vue';

  const props = withDefaults(defineProps<{ active?: boolean }>(), { active: true });
  const authStore = useAuthStore();
  const isAdmin = computed(() => authStore.isAdmin);
  const downloadTasks = ref<DownloadTask[]>([]);
  const tasksLoading = ref(false);
  const tasksError = ref('');
  let tasksRequestId = 0;
  let tasksRequestPending = false;
  let tasksRefreshPending = false;
  let parseRequestId = 0;
  let disposed = false;

  const newTaskUrl = ref('');
  const addingTask = ref(false);
  const addTaskVisible = ref(false);
  const linkParseVisible = ref(false);
  const linkUrl = ref('');
  const linkParsing = ref(false);
  const linkAddingSelected = ref(false);
  const linkResult = ref<DownloadLinkParseResult | null>(null);
  const addingParsedItems = ref<Record<string, boolean>>({});
  const addedParsedItems = ref<Record<string, boolean>>({});
  const selectedParsedItemKeys = ref<string[]>([]);
  const downloadConfigVisible = ref(false);
  let downloadRefreshInterval: number | null = null;

  const hasAddingParsedItems = computed(() => Object.values(addingParsedItems.value).some(Boolean));

  const loadDownloadTasks = async () => {
    if (tasksRequestPending) {
      tasksRefreshPending = true;
      return;
    }
    tasksRequestPending = true;
    const requestId = ++tasksRequestId;
    tasksLoading.value = downloadTasks.value.length === 0;
    try {
      const tasks = await api.getDownloadTasks();
      if (disposed || requestId !== tasksRequestId || !props.active) return;
      downloadTasks.value = tasks;
      tasksError.value = '';
    } catch (e: any) {
      if (disposed || requestId !== tasksRequestId || !props.active) return;
      tasksError.value = '加载下载任务失败: ' + e.message;
    } finally {
      tasksRequestPending = false;
      if (!disposed && requestId === tasksRequestId) tasksLoading.value = false;
      if (tasksRefreshPending && !disposed && props.active) {
        tasksRefreshPending = false;
        void loadDownloadTasks();
      }
    }
  };

  const addDownloadTask = async () => {
    if (!newTaskUrl.value.trim() || addingTask.value) return;
    addingTask.value = true;
    try {
      await api.addDownloadTask(newTaskUrl.value.trim());
      newTaskUrl.value = '';
      message.success('下载任务已添加');
      loadDownloadTasks();
      addTaskVisible.value = false;
    } catch (e: any) {
      message.error('添加任务失败: ' + e.message);
    } finally {
      addingTask.value = false;
    }
  };

  const parsedItems = computed(() => linkResult.value?.items || []);

  const getParsedItemKey = (item: ParsedDownloadItem | Record<string, any>) => {
    return String(item.id || item.file_url || item.filename || item.title || '');
  };

  const getParsedDownloadFilename = (item: ParsedDownloadItem | Record<string, any>) => {
    return String(item.filename || item.title || item.id || 'downloaded_file').trim();
  };

  const isParsedItemAdded = (item: ParsedDownloadItem | Record<string, any>) => {
    return !!addedParsedItems.value[getParsedItemKey(item)];
  };

  const sourceTypeLabel = computed(() => {
    if (!linkResult.value) return '';
    if (linkResult.value.source_type === 'workshop') return 'Steam 工坊';
    if (linkResult.value.source_type === 'qq_flash_transfer') return 'QQ 闪传';
    return linkResult.value.source_type;
  });

  const selectedPendingParsedItems = computed(() => {
    const selected = new Set(selectedParsedItemKeys.value);
    return parsedItems.value.filter(
      (item) => selected.has(getParsedItemKey(item)) && item.supported && !isParsedItemAdded(item)
    );
  });

  const parsedRowSelection = computed(() => ({
    selectedRowKeys: selectedParsedItemKeys.value,
    onChange: (keys: any[]) => {
      selectedParsedItemKeys.value = keys.map(String);
    },
    getCheckboxProps: (record: ParsedDownloadItem | Record<string, any>) => ({
      disabled: !record.supported || isParsedItemAdded(record),
    }),
  }));

  const formatParsedFileSize = (size: string | number) => {
    if (typeof size === 'string' && size.trim() && !/^\d+(\.\d+)?$/.test(size.trim())) {
      return size.trim();
    }
    const bytes = Number(size);
    if (!Number.isFinite(bytes) || bytes <= 0) return '未知大小';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  };

  const parseDownloadLink = async () => {
    if (linkParsing.value || linkAddingSelected.value || hasAddingParsedItems.value) return;
    const link = linkUrl.value.trim();
    if (!link) {
      message.error('请输入要解析的链接或工坊 ID');
      return;
    }

    linkParsing.value = true;
    const requestId = ++parseRequestId;
    linkResult.value = null;
    selectedParsedItemKeys.value = [];
    addedParsedItems.value = {};
    addingParsedItems.value = {};
    try {
      const result = await api.parseDownloadLink(link);
      if (disposed || requestId !== parseRequestId) return;
      linkResult.value = result;
      selectedParsedItemKeys.value = result.items
        .filter((item) => item.supported)
        .map((item) => getParsedItemKey(item));
      message.success(`解析成功，共 ${result.items.length} 个文件`);
    } catch (e: any) {
      if (!disposed && requestId === parseRequestId) message.error('解析失败: ' + e.message);
    } finally {
      if (!disposed && requestId === parseRequestId) linkParsing.value = false;
    }
  };

  const openLinkParseModal = () => {
    linkParseVisible.value = true;
  };

  const openDownloadConfig = () => {
    if (!isAdmin.value) return;
    downloadConfigVisible.value = true;
  };

  const addParsedDownload = async (
    item: ParsedDownloadItem | Record<string, any>,
    refresh = true
  ) => {
    if (!item.supported) {
      message.error(item.disabled_reason || '该文件类型暂不支持加入地图下载任务');
      return false;
    }
    if (!item.file_url) {
      message.error('该条目没有可用下载链接');
      return false;
    }

    const itemID = getParsedItemKey(item);
    if (
      isParsedItemAdded(item) ||
      addingParsedItems.value[itemID] ||
      (refresh && linkAddingSelected.value)
    ) return false;
    addingParsedItems.value = { ...addingParsedItems.value, [itemID]: true };
    try {
      await api.addDownloadTask(item.file_url, getParsedDownloadFilename(item), item.referer);
      addedParsedItems.value = { ...addedParsedItems.value, [itemID]: true };
      if (refresh) {
        message.success(`${getParsedDownloadFilename(item)} 已添加到下载任务`);
        loadDownloadTasks();
      }
      return true;
    } catch (e: any) {
      if (refresh) {
        message.error('添加下载任务失败: ' + e.message);
      }
      return false;
    } finally {
      addingParsedItems.value = { ...addingParsedItems.value, [itemID]: false };
    }
  };

  const downloadSelectedParsedItems = async () => {
    if (linkAddingSelected.value || hasAddingParsedItems.value || linkParsing.value) return;
    const pendingItems = selectedPendingParsedItems.value;
    if (pendingItems.length === 0) {
      message.warning('没有可添加的选中文件');
      return;
    }

    linkAddingSelected.value = true;
    let successCount = 0;
    let failCount = 0;
    try {
      for (const item of pendingItems) {
        const ok = await addParsedDownload(item, false);
        if (ok) {
          successCount++;
        } else {
          failCount++;
        }
      }

      if (successCount > 0) {
        message.success(`已添加 ${successCount} 个下载任务`);
        loadDownloadTasks();
      }
      if (failCount > 0) {
        message.warning(`${failCount} 个任务添加失败`);
      }
      if (successCount > 0 && failCount === 0) {
        linkParseVisible.value = false;
      }
    } finally {
      linkAddingSelected.value = false;
    }
  };

  const cancelTask = async (id: string) => {
    try {
      await api.cancelDownloadTask(id);
      message.success('任务已取消');
      loadDownloadTasks();
    } catch (e: any) {
      message.error('取消任务失败: ' + e.message);
    }
  };

  const restartTask = async (id: string) => {
    try {
      await api.restartDownloadTask(id);
      message.success('任务已重启');
      loadDownloadTasks();
    } catch (e: any) {
      message.error('重启任务失败: ' + e.message);
    }
  };

  const clearDownloadTasks = async () => {
    Modal.confirm({
      title: '确定要清空所有下载记录吗？',
      icon: () => h(ExclamationCircleOutlined),
      onOk: async () => {
        try {
          await api.clearDownloadTasks();
          message.success('记录已清空');
          loadDownloadTasks();
        } catch (e: any) {
          message.error('清空任务失败: ' + e.message);
        }
      },
    });
  };

  const taskColumns = [
    { title: '文件/URL', dataIndex: 'filename', key: 'filename' },
    { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
    { title: '大小', dataIndex: 'formattedSize', key: 'size', width: 120 },
    { title: '速度', dataIndex: 'formattedSpeed', key: 'speed', width: 130 },
    { title: '进度', dataIndex: 'progress', key: 'progress', width: 200 },
    { title: '操作', key: 'action', width: 80, align: 'right' as const },
  ];

  const parsedLinkColumns = [
    { title: '预览', key: 'preview', width: 84 },
    { title: '文件', key: 'info' },
    { title: '大小', key: 'size', width: 120 },
    { title: '状态', key: 'support', width: 150 },
    { title: '操作', key: 'action', width: 110 },
  ];

  const getFileNameFromUrl = (url: string) => {
    if (!url) return 'Unknown';
    try {
      const parts = url.split('/');
      const lastPart = parts[parts.length - 1];
      if (!lastPart) return 'Unknown';
      const filename = lastPart.split('?')[0];
      return filename ? decodeURIComponent(filename) : filename;
    } catch {
      return 'Unknown';
    }
  };

  const taskPaginationConfig = reactive<TablePaginationConfig>({
    current: 1,
    pageSize: 10,
    showSizeChanger: true,
    pageSizeOptions: ['10', '20', '50', '100'],
    showTotal: (total: number) => `共 ${total} 条`,
  });

  const handleTaskTableChange = (pag: TablePaginationConfig) => {
    taskPaginationConfig.current = pag.current;
    taskPaginationConfig.pageSize = pag.pageSize;
  };

  const startPolling = () => {
    if (downloadRefreshInterval !== null) return;
    void loadDownloadTasks();
    downloadRefreshInterval = window.setInterval(() => void loadDownloadTasks(), 3000);
  };

  const stopPolling = () => {
    if (downloadRefreshInterval !== null) {
      clearInterval(downloadRefreshInterval);
      downloadRefreshInterval = null;
    }
    tasksRequestId++;
    tasksRefreshPending = false;
    tasksLoading.value = false;
  };

  watch(() => props.active, (active) => active ? startPolling() : stopPolling(), { immediate: true });

  onBeforeUnmount(() => {
    disposed = true;
    parseRequestId++;
    stopPolling();
  });
</script>

<template>
  <div>
    <div class="space-y-4">
      <!-- Add Task & Actions -->
      <div class="flex flex-wrap justify-between items-center gap-2">
        <div class="flex gap-2 flex-wrap">
          <a-button
            type="primary"
            @click="addTaskVisible = true"
            class="!flex !items-center !justify-center"
          >
            <template #icon><plus-outlined /></template>
            添加任务
          </a-button>
          <a-button-group class="!flex">
            <a-button
              @click="openLinkParseModal"
              class="!flex !items-center !justify-center"
            >
              <template #icon><search-outlined /></template>
              解析链接
            </a-button>
            <a-button
              v-if="isAdmin"
              @click="openDownloadConfig"
              class="!flex !items-center !justify-center"
              title="下载设置"
              aria-label="下载设置"
            >
              <template #icon><setting-outlined /></template>
            </a-button>
          </a-button-group>
        </div>

        <a-button
          v-if="downloadTasks.length > 0"
          size="small"
          danger
          type="text"
          @click="clearDownloadTasks"
          class="!flex !items-center !justify-center"
        >
          清空记录
        </a-button>
      </div>

      <a-alert v-if="tasksError" :message="tasksError" type="error" show-icon />

      <!-- Task List -->
      <a-table
        :columns="taskColumns"
        :dataSource="downloadTasks"
        :loading="tasksLoading"
        :pagination="taskPaginationConfig"
        @change="handleTaskTableChange"
        row-key="id"
        :scroll="{ x: 900 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'filename'">
            <div class="flex flex-col gap-1 min-w-[120px]">
              <div
                class="font-bold text-sm truncate"
                :title="record.filename || getFileNameFromUrl(record.url)"
              >
                {{ record.filename || getFileNameFromUrl(record.url) }}
              </div>
              <div
                class="text-xs text-gray-400 truncate max-w-[150px] md:max-w-md"
                :title="record.url"
              >
                {{ record.url }}
              </div>
              <div v-if="record.status === 3" class="text-xs text-red-500 break-words">
                失败原因: {{ record.message }}
              </div>
            </div>
          </template>
          <template v-else-if="column.key === 'status'">
            <div class="min-w-[80px]">
              <a-tag
                class="mr-0"
                :color="
                  record.status === 2
                    ? 'success'
                    : record.status === 1
                      ? 'processing'
                      : record.status === 3
                        ? 'error'
                        : 'default'
                "
              >
                {{
                  record.status === 2
                    ? '已完成'
                    : record.status === 1
                      ? '下载中'
                      : record.status === 3
                        ? '失败'
                        : '等待中'
                }}
              </a-tag>
            </div>
          </template>
          <template v-else-if="column.key === 'size'">
            <a-tag class="mr-0">
              {{ record.formattedSize || '未知大小' }}
            </a-tag>
          </template>
          <template v-else-if="column.key === 'speed'">
            <span class="text-xs text-gray-600 dark:text-gray-300 whitespace-nowrap">
              {{ record.status === 1 ? record.formattedSpeed || '0 B/s' : '-' }}
            </span>
          </template>
          <template v-else-if="column.key === 'progress'">
            <div class="flex flex-col items-start gap-1 min-w-[100px]">
              <a-progress
                :percent="Number((record.progress || 0).toFixed(1))"
                size="small"
                :show-info="false"
                :status="
                  record.status === 3 ? 'exception' : record.status === 2 ? 'success' : 'active'
                "
              />
              <span class="text-xs text-gray-500 dark:text-gray-400">
                {{ (record.progress || 0).toFixed(1) }}%
              </span>
            </div>
          </template>
          <template v-else-if="column.key === 'action'">
            <a-space>
              <a-button
                v-if="record.status === 0 || record.status === 1"
                type="text"
                size="small"
                danger
                @click="cancelTask(record.id)"
                class="!flex !items-center !justify-center"
                title="取消"
              >
                <template #icon><close-circle-outlined /></template>
              </a-button>
              <a-button
                v-if="record.status === 3"
                type="text"
                size="small"
                @click="restartTask(record.id)"
                class="!flex !items-center !justify-center"
                title="重试"
              >
                <template #icon><reload-outlined /></template>
              </a-button>
            </a-space>
          </template>
        </template>
      </a-table>
    </div>

    <a-modal
      v-model:open="addTaskVisible"
      title="添加下载任务"
      width="min(520px, 95vw)"
      @ok="addDownloadTask"
      :confirmLoading="addingTask"
    >
      <a-textarea
        v-model:value="newTaskUrl"
        placeholder="请输入下载链接，支持多个链接（每行一个或空格分隔）
支持 .vpk, .zip, .rar, .7z 格式"
        :rows="6"
      />
    </a-modal>

    <a-modal
      v-model:open="downloadConfigVisible"
      title="下载设置"
      :width="520"
      :footer="null"
      wrap-class-name="download-config-modal"
    >
      <SteamCDNSetting
        :active="downloadConfigVisible"
        context="modal"
        @saved="downloadConfigVisible = false"
        @cancel="downloadConfigVisible = false"
      />
    </a-modal>

    <a-modal v-model:open="linkParseVisible" title="解析链接" width="min(920px, 95vw)" :footer="null">
      <div class="space-y-4">
        <a-input-search
          v-model:value="linkUrl"
          placeholder="粘贴 Steam 工坊链接、合集链接、工坊 ID 或 QQ 闪传链接"
          enter-button="解析"
          :loading="linkParsing"
          :disabled="linkParsing || linkAddingSelected || hasAddingParsedItems"
          @search="parseDownloadLink"
        />

        <div v-if="linkResult" class="space-y-3">
          <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
            <div class="text-sm text-gray-600 dark:text-gray-400">
              解析到 {{ parsedItems.length }} 个文件
              <span class="ml-2 text-xs text-gray-400">
                来源: {{ sourceTypeLabel }} · ID: {{ linkResult.source_id }}
              </span>
            </div>
            <a-button
              type="primary"
              :loading="linkAddingSelected"
              :disabled="selectedPendingParsedItems.length === 0 || hasAddingParsedItems || linkParsing"
              @click="downloadSelectedParsedItems"
              class="!flex !items-center !justify-center"
            >
              <template #icon><download-outlined /></template>
              添加选中
            </a-button>
          </div>

          <a-table
            :columns="parsedLinkColumns"
            :dataSource="parsedItems"
            :pagination="{ pageSize: 8, showSizeChanger: false }"
            :rowKey="(record: any) => getParsedItemKey(record)"
            :row-selection="parsedRowSelection"
            size="small"
            :scroll="{ x: 720 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'preview'">
                <div
                  class="w-14 h-14 rounded border border-gray-200 dark:border-gray-700 overflow-hidden bg-gray-50 dark:bg-gray-800 flex items-center justify-center"
                >
                  <img
                    v-if="record.preview_url"
                    :src="record.preview_url"
                    :alt="record.title"
                    class="w-full h-full object-cover"
                    loading="lazy"
                  />
                  <file-text-outlined v-else class="text-xl text-gray-400" />
                </div>
              </template>
              <template v-else-if="column.key === 'info'">
                <div class="min-w-[260px]">
                  <div class="font-medium text-sm break-words dark:text-gray-100">
                    {{ record.title || record.filename || record.id }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    {{ record.filename }}
                  </div>
                </div>
              </template>
              <template v-else-if="column.key === 'size'">
                <a-tag>{{ formatParsedFileSize(record.file_size) }}</a-tag>
              </template>
              <template v-else-if="column.key === 'support'">
                <a-tag v-if="record.supported" color="success" class="mr-0">可添加</a-tag>
                <a-tag v-else color="default" class="mr-0">
                  {{ record.disabled_reason || '不支持' }}
                </a-tag>
              </template>
              <template v-else-if="column.key === 'action'">
                <a-button
                  size="small"
                  type="primary"
                  :loading="addingParsedItems[getParsedItemKey(record)]"
                  :disabled="!record.supported || isParsedItemAdded(record) || linkAddingSelected || linkParsing"
                  @click="addParsedDownload(record)"
                  class="!flex !items-center !justify-center"
                >
                  <template #icon><download-outlined /></template>
                  {{ isParsedItemAdded(record) ? '已添加' : '添加' }}
                </a-button>
              </template>
            </template>
          </a-table>
        </div>
      </div>
    </a-modal>
  </div>
</template>

<style scoped>
  :global(.download-config-modal .ant-modal) {
    max-width: calc(100vw - 24px);
  }

  :global(.download-config-modal .ant-modal-body) {
    overflow-wrap: anywhere;
  }

  @media (max-width: 640px) {
    :global(.download-config-modal .ant-modal) {
      top: 12px;
      margin: 0 auto;
      padding-bottom: 12px;
    }

    :global(.download-config-modal .ant-modal-body) {
      max-height: calc(100vh - 170px);
      overflow-y: auto;
    }
  }
</style>
