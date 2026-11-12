<script setup lang="ts">
  import { computed, ref, onMounted, onUnmounted, nextTick, watch } from 'vue';
  import {
    api,
    type SourceModLogCategory,
    type SourceModLogCleanupPreview,
    type SourceModLogDeleteResult,
    type SourceModLogFile,
  } from '../services/api';
  import { useAuthStore } from '../stores/auth';
  import { message, Modal } from 'ant-design-vue';
  import {
    ArrowLeftOutlined,
    ClearOutlined,
    CompressOutlined,
    DeleteOutlined,
    ExpandOutlined,
    FileTextOutlined,
    LockOutlined,
    PauseCircleOutlined,
    PlayCircleOutlined,
    ReloadOutlined,
  } from '@ant-design/icons-vue';

  interface LogCategories {
    L: SourceModLogFile[];
    errors: SourceModLogFile[];
    other: SourceModLogFile[];
  }

  type RetentionDays = 0 | 7 | 30 | 90;

  const MAX_LOG_LINES = 1000;

  const authStore = useAuthStore();
  const installed = ref(true);
  const categories = ref<LogCategories>({ L: [], errors: [], other: [] });
  const selectedFile = ref('');
  const logLines = ref<string[]>([]);
  const isPaused = ref(false);
  const isLoading = ref(false);
  const isLogExpanded = ref(false);
  const mobileMedia = window.matchMedia('(max-width: 1023px)');
  const isMobileView = ref(mobileMedia.matches);
  const logError = ref('');
  const listError = ref('');
  const logContainer = ref<HTMLElement | null>(null);
  const shouldAutoScroll = ref(true);

  const cleanupModalOpen = ref(false);
  const cleanupCategories = ref<SourceModLogCategory[]>(['L', 'errors', 'other']);
  const cleanupRetentionDays = ref<RetentionDays>(30);
  const cleanupPreview = ref<SourceModLogCleanupPreview | null>(null);
  const cleanupPreviewLoading = ref(false);
  const cleanupPreviewError = ref('');
  const cleanupDeleting = ref(false);
  const cleanupResult = ref<SourceModLogDeleteResult | null>(null);

  let logStream: { close: () => void } | null = null;
  let logStreamSession = 0;
  let fileListRequestId = 0;
  let disposed = false;
  let cleanupPreviewRequestId = 0;

  const isAdmin = computed(() => authStore.isAdmin);
  const isViewerFocused = computed(() => isLogExpanded.value || (isMobileView.value && !!selectedFile.value));
  const allLogFiles = computed(() => [
    ...categories.value.L,
    ...categories.value.errors,
    ...categories.value.other,
  ]);
  const selectedLogFile = computed(
    () => allLogFiles.value.find((file) => file.name === selectedFile.value) || null
  );
  const hasDeletableLogs = computed(() => allLogFiles.value.some((file) => file.deletable));
  const cleanupOkText = computed(() => `删除 ${cleanupPreview.value?.count || 0} 个日志`);
  const cleanupHasIssues = computed(
    () =>
      !!cleanupResult.value &&
      (cleanupResult.value.skipped.length > 0 || cleanupResult.value.failed.length > 0)
  );
  const cleanupIssues = computed(() => [
    ...(cleanupResult.value?.skipped || []),
    ...(cleanupResult.value?.failed || []),
  ]);

  const formatFileSize = (size: number): string => {
    if (size < 1024) return `${size}B`;
    if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)}KB`;
    if (size < 1024 * 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(2)}MB`;
    return `${(size / (1024 * 1024 * 1024)).toFixed(2)}GB`;
  };

  const categoryLabel = (category: SourceModLogCategory): string => {
    if (category === 'L') return '运行日志';
    if (category === 'errors') return '错误日志';
    return '其他日志';
  };

  const closeLogStream = () => {
    logStreamSession += 1;
    if (logStream) {
      logStream.close();
      logStream = null;
    }
  };

  const clearSelectedLog = () => {
    closeLogStream();
    selectedFile.value = '';
    logLines.value = [];
    logError.value = '';
    isPaused.value = false;
    isLogExpanded.value = false;
    shouldAutoScroll.value = true;
  };

  const loadFileList = async () => {
    const requestId = ++fileListRequestId;
    isLoading.value = true;
    listError.value = '';
    try {
      const data = await api.getSourceModLogs();
      if (disposed || requestId !== fileListRequestId) return;
      installed.value = data.installed;
      categories.value = {
        L: data.categories?.L || [],
        errors: data.categories?.errors || [],
        other: data.categories?.other || [],
      };
      if (selectedFile.value && !allLogFiles.value.some((file) => file.name === selectedFile.value)) {
        clearSelectedLog();
      }
    } catch (e: any) {
      if (!disposed && requestId === fileListRequestId) {
        listError.value = e.message || '获取日志列表失败';
        message.error(listError.value);
      }
    } finally {
      if (!disposed && requestId === fileListRequestId) isLoading.value = false;
    }
  };

  const scrollToBottom = () => {
    if (!logContainer.value || !shouldAutoScroll.value) return;
    logContainer.value.scrollTop = logContainer.value.scrollHeight;
  };

  const handleScroll = () => {
    if (!logContainer.value) return;
    const el = logContainer.value;
    const isNearBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - 50;
    shouldAutoScroll.value = isNearBottom;
  };

  const selectFile = (filename: string) => {
    if (disposed || !filename) return;
    closeLogStream();
    const session = logStreamSession;
    selectedFile.value = filename;
    logLines.value = [];
    logError.value = '';
    isPaused.value = false;
    shouldAutoScroll.value = true;

    logStream = api.streamLog(
      filename,
      (line) => {
        if (disposed || session !== logStreamSession || selectedFile.value !== filename) return;
        if (!isPaused.value && line !== '') {
          logLines.value.push(line);
          if (logLines.value.length > MAX_LOG_LINES) {
            logLines.value = logLines.value.slice(-MAX_LOG_LINES);
          }
        }
      },
      (err) => {
        if (disposed || session !== logStreamSession || selectedFile.value !== filename) return;
        logError.value = err;
        message.error(err);
      }
    );
  };

  const reopenLogIfAvailable = (filename: string) => {
    if (filename && allLogFiles.value.some((file) => file.name === filename)) {
      selectFile(filename);
    }
  };

  const togglePause = () => {
    isPaused.value = !isPaused.value;
  };

  const clearDisplay = () => {
    logLines.value = [];
  };

  const deleteSelectedLog = () => {
    const file = selectedLogFile.value;
    if (!file || !file.deletable || !isAdmin.value) return;

    Modal.confirm({
      title: '删除服务器日志？',
      content: `${file.name}（${categoryLabel(file.category)} · ${formatFileSize(file.size)}）。删除后无法恢复。`,
      okText: '删除文件',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        const filename = file.name;
        closeLogStream();
        try {
          const result = await api.deleteSourceModLogs([{ name: file.name, version: file.version }]);
          await loadFileList();
          if (result.deleted.includes(filename)) {
            clearSelectedLog();
            message.success(`已删除 ${filename}，释放 ${formatFileSize(result.freed_bytes)}`);
            return;
          }

          const issue = result.skipped[0] || result.failed[0];
          message.warning(issue?.message || '日志未删除，请刷新后重试');
          reopenLogIfAvailable(filename);
        } catch (e: any) {
          message.error(e.message || '删除日志失败');
          reopenLogIfAvailable(filename);
        }
      },
    });
  };

  const loadCleanupPreview = async () => {
    const requestId = ++cleanupPreviewRequestId;
    cleanupPreviewError.value = '';
    if (cleanupCategories.value.length === 0) {
      cleanupPreview.value = null;
      cleanupPreviewLoading.value = false;
      return;
    }

    cleanupPreviewLoading.value = true;
    try {
      const preview = await api.previewSourceModLogCleanup({
        categories: [...cleanupCategories.value],
        retention_days: cleanupRetentionDays.value,
      });
      if (requestId === cleanupPreviewRequestId && cleanupModalOpen.value) {
        cleanupPreview.value = preview;
      }
    } catch (e: any) {
      if (requestId === cleanupPreviewRequestId && cleanupModalOpen.value) {
        cleanupPreview.value = null;
        cleanupPreviewError.value = e.message || '获取清理预览失败';
      }
    } finally {
      if (requestId === cleanupPreviewRequestId) {
        cleanupPreviewLoading.value = false;
      }
    }
  };

  const openCleanupModal = () => {
    cleanupCategories.value = ['L', 'errors', 'other'];
    cleanupRetentionDays.value = 30;
    cleanupPreview.value = null;
    cleanupResult.value = null;
    cleanupPreviewError.value = '';
    cleanupModalOpen.value = true;
    loadCleanupPreview();
  };

  const closeCleanupModal = () => {
    if (cleanupDeleting.value) return;
    cleanupPreviewRequestId++;
    cleanupModalOpen.value = false;
  };

  const confirmCleanup = async () => {
    const preview = cleanupPreview.value;
    if (!preview || preview.count === 0 || cleanupDeleting.value) return;

    const targets = preview.candidates.map((file) => ({ name: file.name, version: file.version }));
    const targetedNames = new Set(targets.map((file) => file.name));
    const previousSelectedFile = selectedFile.value;
    const selectedIsTargeted = targetedNames.has(previousSelectedFile);
    if (selectedIsTargeted) {
      closeLogStream();
    }

    cleanupDeleting.value = true;
    cleanupResult.value = null;
    try {
      const result = await api.deleteSourceModLogs(targets);
      cleanupResult.value = result;
      await loadFileList();

      if (selectedIsTargeted) {
        if (result.deleted.includes(previousSelectedFile)) {
          clearSelectedLog();
        } else {
          reopenLogIfAvailable(previousSelectedFile);
        }
      }

      if (result.skipped.length === 0 && result.failed.length === 0) {
        cleanupModalOpen.value = false;
        message.success(
          `已删除 ${result.deleted.length} 个日志，释放 ${formatFileSize(result.freed_bytes)}`
        );
      } else {
        message.warning(
          `已删除 ${result.deleted.length} 个，跳过 ${result.skipped.length} 个，失败 ${result.failed.length} 个`
        );
        await loadCleanupPreview();
      }
    } catch (e: any) {
      message.error(e.message || '批量清理日志失败');
      if (selectedIsTargeted) {
        reopenLogIfAvailable(previousSelectedFile);
      }
    } finally {
      cleanupDeleting.value = false;
    }
  };

  const toggleLogExpanded = () => {
    isLogExpanded.value = !isLogExpanded.value;
    nextTick(scrollToBottom);
  };

  const handleViewportChange = (event: MediaQueryListEvent) => {
    isMobileView.value = event.matches;
    nextTick(scrollToBottom);
  };

  watch(
    () => logLines.value.length,
    () => {
      nextTick(scrollToBottom);
    }
  );

  watch(
    () => [cleanupCategories.value.join(','), cleanupRetentionDays.value],
    () => {
      if (!cleanupModalOpen.value) return;
      cleanupResult.value = null;
      loadCleanupPreview();
    }
  );

  onMounted(() => {
    mobileMedia.addEventListener('change', handleViewportChange);
    loadFileList();
  });

  onUnmounted(() => {
    disposed = true;
    fileListRequestId += 1;
    mobileMedia.removeEventListener('change', handleViewportChange);
    closeLogStream();
    cleanupPreviewRequestId++;
  });
</script>

<template>
  <div class="logs-page p-4 md:p-6">
    <!-- Header -->
    <Transition name="log-header">
      <div
        v-if="!isViewerFocused"
        class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 overflow-hidden"
      >
        <div>
          <h1 class="text-2xl font-bold text-gray-800 dark:text-gray-100 flex items-center gap-2">
            <FileTextOutlined /> 日志查看
          </h1>
          <p class="text-gray-500 dark:text-gray-400 mt-1">
            查看 SourceMod 插件日志、错误日志及其他日志文件，支持实时推送
          </p>
        </div>
      </div>
    </Transition>

    <!-- Main Content -->
    <div
      class="logs-workspace flex min-h-0 min-w-0 flex-col lg:flex-row gap-4 transition-all duration-300 ease-out"
      :class="{ 'mt-6': !isViewerFocused }"
      :style="{ height: isViewerFocused ? 'var(--logs-expanded-height)' : 'var(--logs-standard-height)' }"
    >
      <!-- Sidebar -->
      <Transition name="log-sidebar">
        <div v-if="!isViewerFocused" class="log-sidebar min-h-0 min-w-0 w-full lg:w-64 flex-shrink-0">
          <a-card
            class="h-full overflow-hidden shadow-xl border-0"
            :body-style="{
              padding: '16px',
              height: '100%',
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
              gap: '12px',
            }"
          >
            <div class="flex items-center justify-between gap-2">
              <span class="font-bold text-gray-800 dark:text-gray-100">日志文件列表</span>
              <div class="flex items-center flex-shrink-0">
                <a-tooltip v-if="isAdmin" title="按类型和时间批量清理历史日志">
                  <span>
                    <a-button
                      type="text"
                      size="small"
                      danger
                      :disabled="!installed || !hasDeletableLogs"
                      @click="openCleanupModal"
                      class="!flex !items-center !justify-center"
                    >
                      <template #icon><DeleteOutlined /></template>
                      清理
                    </a-button>
                  </span>
                </a-tooltip>
                <a-tooltip title="刷新日志列表">
                  <a-button
                    type="text"
                    size="small"
                    @click="loadFileList"
                    :loading="isLoading"
                    aria-label="刷新列表"
                    class="!flex !items-center !justify-center"
                  >
                    <template #icon><ReloadOutlined /></template>
                  </a-button>
                </a-tooltip>
              </div>
            </div>

            <a-alert v-if="listError" class="log-list-error" type="error" show-icon message="日志列表加载失败" :description="listError" />

            <div v-if="!installed" class="text-center py-8 text-gray-500 dark:text-gray-400">
              <FileTextOutlined class="text-4xl mb-2" />
              <p>请安装 SourceMod</p>
            </div>

            <div v-else class="log-file-list flex-1 min-h-0 min-w-0 overflow-y-auto space-y-3">
              <!-- L Logs -->
              <div v-if="categories.L?.length > 0">
                <div class="text-xs font-semibold text-gray-500 dark:text-gray-400 px-1 py-1">
                  运行日志
                </div>
                <button
                  v-for="file in categories.L"
                  :key="file.name"
                  type="button"
                  class="log-file-item flex w-full min-w-0 items-center justify-between border-0 px-2 py-1.5 rounded cursor-pointer text-left text-sm transition-colors"
                  :aria-pressed="selectedFile === file.name"
                  :class="
                    selectedFile === file.name
                      ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
                      : 'hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-700 dark:text-gray-300'
                  "
                  @click="selectFile(file.name)"
                >
                  <span class="truncate flex-1 mr-2">{{ file.name }}</span>
                  <a-tooltip v-if="!file.deletable" :title="file.protected_reason">
                    <LockOutlined class="mr-1 text-amber-500 flex-shrink-0" />
                  </a-tooltip>
                  <span class="text-xs text-gray-400 dark:text-gray-500 flex-shrink-0">{{
                    formatFileSize(file.size)
                  }}</span>
                </button>
              </div>

              <!-- Error Logs -->
              <div v-if="categories.errors?.length > 0">
                <div class="text-xs font-semibold text-gray-500 dark:text-gray-400 px-1 py-1">
                  错误日志
                </div>
                <button
                  v-for="file in categories.errors"
                  :key="file.name"
                  type="button"
                  class="log-file-item flex w-full min-w-0 items-center justify-between border-0 px-2 py-1.5 rounded cursor-pointer text-left text-sm transition-colors"
                  :aria-pressed="selectedFile === file.name"
                  :class="
                    selectedFile === file.name
                      ? 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300'
                      : 'hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-700 dark:text-gray-300'
                  "
                  @click="selectFile(file.name)"
                >
                  <span class="truncate flex-1 mr-2">{{ file.name }}</span>
                  <a-tooltip v-if="!file.deletable" :title="file.protected_reason">
                    <LockOutlined class="mr-1 text-amber-500 flex-shrink-0" />
                  </a-tooltip>
                  <span class="text-xs text-gray-400 dark:text-gray-500 flex-shrink-0">{{
                    formatFileSize(file.size)
                  }}</span>
                </button>
              </div>

              <!-- Other Logs -->
              <div v-if="categories.other?.length > 0">
                <div class="text-xs font-semibold text-gray-500 dark:text-gray-400 px-1 py-1">
                  其他日志
                </div>
                <button
                  v-for="file in categories.other"
                  :key="file.name"
                  type="button"
                  class="log-file-item flex w-full min-w-0 items-center justify-between border-0 px-2 py-1.5 rounded cursor-pointer text-left text-sm transition-colors"
                  :aria-pressed="selectedFile === file.name"
                  :class="
                    selectedFile === file.name
                      ? 'bg-gray-200 dark:bg-gray-700 text-gray-800 dark:text-gray-200'
                      : 'hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-700 dark:text-gray-300'
                  "
                  @click="selectFile(file.name)"
                >
                  <span class="truncate flex-1 mr-2">{{ file.name }}</span>
                  <a-tooltip v-if="!file.deletable" :title="file.protected_reason">
                    <LockOutlined class="mr-1 text-amber-500 flex-shrink-0" />
                  </a-tooltip>
                  <span class="text-xs text-gray-400 dark:text-gray-500 flex-shrink-0">{{
                    formatFileSize(file.size)
                  }}</span>
                </button>
              </div>

              <div
                v-if="
                  categories.L?.length === 0 &&
                  categories.errors?.length === 0 &&
                  categories.other?.length === 0
                "
                class="text-center py-8 text-gray-400 dark:text-gray-500"
              >
                暂无日志文件
              </div>
            </div>
          </a-card>
        </div>
      </Transition>

      <!-- Content -->
      <div v-show="!isMobileView || !!selectedFile" class="log-viewer flex-1 min-h-0 min-w-0">
        <a-card
          class="h-full overflow-hidden shadow-xl border-0"
          :body-style="{
            padding: 0,
            height: '100%',
            display: 'flex',
            flexDirection: 'column',
            overflow: 'hidden',
          }"
        >
          <!-- Toolbar -->
          <div
            class="log-toolbar flex flex-col xl:flex-row xl:items-center justify-between gap-2 px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 transition-colors"
          >
            <div class="flex items-center gap-2 min-w-0">
              <span class="log-file-name min-w-0 flex-1 font-mono text-sm text-gray-700 dark:text-gray-300 truncate" :title="selectedFile">
                {{ selectedFile || '请选择日志文件' }}
              </span>
              <a-tag v-if="isPaused" color="orange" class="ml-2 flex-shrink-0">已暂停</a-tag>
              <a-tag v-if="selectedFile" color="blue" class="ml-2 flex-shrink-0">
                {{ logLines.length }}/{{ MAX_LOG_LINES }}
              </a-tag>
            </div>
            <div class="flex flex-wrap items-center justify-end gap-2 flex-shrink-0">
              <a-button
                v-if="isMobileView && selectedFile"
                size="small"
                aria-label="返回列表"
                class="!flex !items-center !justify-center"
                @click="clearSelectedLog"
              >
                <template #icon><ArrowLeftOutlined /></template>
                返回列表
              </a-button>
              <a-button
                v-if="!isMobileView"
                size="small"
                @click="toggleLogExpanded"
                :disabled="!selectedFile"
                class="!flex !items-center !justify-center"
              >
                <template #icon>
                  <CompressOutlined v-if="isLogExpanded" />
                  <ExpandOutlined v-else />
                </template>
                {{ isLogExpanded ? '收起' : '展开' }}
              </a-button>
              <a-button
                size="small"
                @click="togglePause"
                :disabled="!selectedFile"
                class="!flex !items-center !justify-center"
              >
                <template #icon>
                  <PauseCircleOutlined v-if="!isPaused" />
                  <PlayCircleOutlined v-else />
                </template>
                {{ isPaused ? '继续' : '暂停' }}
              </a-button>
              <a-button
                size="small"
                @click="clearDisplay"
                :disabled="!selectedFile || logLines.length === 0"
                class="!flex !items-center !justify-center"
              >
                <template #icon><ClearOutlined /></template>
                清空显示
              </a-button>
              <a-tooltip
                v-if="isAdmin"
                :title="selectedLogFile && !selectedLogFile.deletable ? selectedLogFile.protected_reason : ''"
              >
                <span>
                  <a-button
                    size="small"
                    danger
                    @click="deleteSelectedLog"
                    :disabled="!selectedLogFile || !selectedLogFile.deletable"
                    class="!flex !items-center !justify-center"
                  >
                    <template #icon><DeleteOutlined /></template>
                    删除文件
                  </a-button>
                </span>
              </a-tooltip>
            </div>
          </div>

          <a-alert v-if="logError" class="log-stream-error" type="error" show-icon message="日志读取失败" :description="logError">
            <template #action>
              <a-button size="small" @click="selectFile(selectedFile)">重试</a-button>
            </template>
          </a-alert>

          <!-- Log Display -->
          <div
            ref="logContainer"
            class="log-output flex-1 min-h-0 overflow-auto p-4 font-mono text-sm bg-[#1e1e1e] text-gray-300 space-y-0.5 min-w-0"
            @scroll="handleScroll"
          >
            <template v-if="logLines.length > 0">
              <div
                v-for="(line, index) in logLines"
                :key="index"
                class="whitespace-pre leading-snug"
              >
                <span>{{ line }}</span>
              </div>
            </template>
            <div
              v-else-if="selectedFile"
              class="flex flex-col items-center justify-center h-full opacity-30 select-none"
            >
              <FileTextOutlined class="text-6xl mb-4" />
              <p v-if="logError">请选择其他日志或点击重试</p>
              <p v-else-if="isPaused">日志接收已暂停</p>
              <p v-else-if="selectedLogFile?.size === 0">当前日志为空，等待新内容...</p>
              <p v-else>等待日志内容...</p>
            </div>
            <div
              v-else
              class="flex flex-col items-center justify-center h-full opacity-30 select-none"
            >
              <FileTextOutlined class="text-6xl mb-4" />
              <p>请从左侧选择一个日志文件</p>
            </div>
          </div>
        </a-card>
      </div>
    </div>

    <a-modal
      v-model:open="cleanupModalOpen"
      title="批量清理 SourceMod 日志"
      width="680px"
      :confirm-loading="cleanupDeleting"
      :ok-text="cleanupOkText"
      cancel-text="取消"
      :ok-button-props="{
        danger: true,
        disabled:
          cleanupPreviewLoading ||
          cleanupCategories.length === 0 ||
          !cleanupPreview ||
          cleanupPreview.count === 0,
      }"
      :cancel-button-props="{ disabled: cleanupDeleting }"
      :closable="!cleanupDeleting"
      :mask-closable="!cleanupDeleting"
      @ok="confirmCleanup"
      @cancel="closeCleanupModal"
    >
      <div class="space-y-5">
        <a-alert
          type="warning"
          show-icon
          message="日志将被永久删除，无法恢复"
          description="今天、未来日期以及最近 10 分钟内更新的日志会被强制保留。"
        />

        <section>
          <div class="font-medium text-gray-800 dark:text-gray-100 mb-2">日志类型</div>
          <a-checkbox-group v-model:value="cleanupCategories" class="w-full">
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <a-checkbox value="L">运行日志</a-checkbox>
              <a-checkbox value="errors">错误日志</a-checkbox>
              <a-checkbox value="other">其他日志</a-checkbox>
            </div>
          </a-checkbox-group>
        </section>

        <section>
          <div class="font-medium text-gray-800 dark:text-gray-100 mb-2">清理范围</div>
          <a-radio-group v-model:value="cleanupRetentionDays" button-style="solid">
            <a-radio-button :value="7">7 天以前</a-radio-button>
            <a-radio-button :value="30">30 天以前</a-radio-button>
            <a-radio-button :value="90">90 天以前</a-radio-button>
            <a-radio-button :value="0">全部历史</a-radio-button>
          </a-radio-group>
        </section>

        <a-alert
          v-if="cleanupPreviewError"
          type="error"
          show-icon
          message="无法获取清理预览"
          :description="cleanupPreviewError"
        />

        <a-spin :spinning="cleanupPreviewLoading">
          <div
            v-if="cleanupCategories.length === 0"
            class="text-center py-8 text-gray-400 dark:text-gray-500"
          >
            请至少选择一种日志类型
          </div>
          <template v-else-if="cleanupPreview">
            <div class="grid grid-cols-3 gap-3">
              <div class="rounded-lg bg-gray-50 dark:bg-gray-800 p-3 text-center">
                <div class="text-xl font-semibold text-gray-800 dark:text-gray-100">
                  {{ cleanupPreview.count }}
                </div>
                <div class="text-xs text-gray-500 dark:text-gray-400">待删除文件</div>
              </div>
              <div class="rounded-lg bg-gray-50 dark:bg-gray-800 p-3 text-center">
                <div class="text-xl font-semibold text-gray-800 dark:text-gray-100">
                  {{ formatFileSize(cleanupPreview.total_size) }}
                </div>
                <div class="text-xs text-gray-500 dark:text-gray-400">预计释放</div>
              </div>
              <div class="rounded-lg bg-gray-50 dark:bg-gray-800 p-3 text-center">
                <div class="text-xl font-semibold text-amber-600 dark:text-amber-400">
                  {{ cleanupPreview.protected.length }}
                </div>
                <div class="text-xs text-gray-500 dark:text-gray-400">受保护文件</div>
              </div>
            </div>

            <a-empty
              v-if="cleanupPreview.count === 0 && cleanupPreview.protected.length === 0"
              :image="undefined"
              description="当前条件下没有可清理的日志"
              class="!my-5"
            />

            <a-collapse
              v-if="cleanupPreview.count > 0 || cleanupPreview.protected.length > 0"
              ghost
              class="mt-2"
            >
              <a-collapse-panel
                v-if="cleanupPreview.count > 0"
                key="candidates"
                :header="`查看待删除文件（${cleanupPreview.count}）`"
              >
                <div class="max-h-52 overflow-y-auto divide-y divide-gray-100 dark:divide-gray-700">
                  <div
                    v-for="file in cleanupPreview.candidates"
                    :key="file.name"
                    class="flex items-center justify-between gap-3 py-1.5 text-sm"
                  >
                    <span class="font-mono truncate text-gray-700 dark:text-gray-300">{{
                      file.name
                    }}</span>
                    <span class="text-gray-400 flex-shrink-0">{{ formatFileSize(file.size) }}</span>
                  </div>
                </div>
              </a-collapse-panel>
              <a-collapse-panel
                v-if="cleanupPreview.protected.length > 0"
                key="protected"
                :header="`查看受保护文件（${cleanupPreview.protected.length}）`"
              >
                <div class="max-h-44 overflow-y-auto divide-y divide-gray-100 dark:divide-gray-700">
                  <div
                    v-for="file in cleanupPreview.protected"
                    :key="file.name"
                    class="py-1.5 text-sm"
                  >
                    <div class="font-mono truncate text-gray-700 dark:text-gray-300">
                      {{ file.name }}
                    </div>
                    <div class="text-xs text-amber-600 dark:text-amber-400">
                      {{ file.protected_reason }}
                    </div>
                  </div>
                </div>
              </a-collapse-panel>
            </a-collapse>
          </template>
        </a-spin>

        <a-alert
          v-if="cleanupHasIssues && cleanupResult"
          type="warning"
          show-icon
          :message="`已删除 ${cleanupResult.deleted.length} 个，另有 ${cleanupIssues.length} 个未删除`"
        >
          <template #description>
            <div class="max-h-32 overflow-y-auto space-y-1 mt-1">
              <div v-for="issue in cleanupIssues" :key="`${issue.name}-${issue.reason}`">
                <span class="font-mono">{{ issue.name }}</span>：{{ issue.message }}
              </div>
            </div>
          </template>
        </a-alert>
      </div>
    </a-modal>
  </div>
</template>

<style scoped>
  .logs-page {
    --logs-viewport-height: 100vh;
    --logs-expanded-height: calc(var(--logs-viewport-height) - 128px);
    --logs-standard-height: calc(var(--logs-viewport-height) - 220px);
    min-width: 0;
  }

  @supports (height: 100dvh) {
    .logs-page {
      --logs-viewport-height: 100dvh;
    }
  }

  @media (min-width: 768px) and (max-width: 1023px) {
    .logs-page {
      --logs-expanded-height: calc(var(--logs-viewport-height) - 160px);
    }
  }

  @media (min-width: 1024px) {
    .logs-page {
      --logs-expanded-height: calc(var(--logs-viewport-height) - 96px);
    }
  }

  @media (max-width: 1023px) and (max-height: 500px) {
    .logs-page {
      --logs-standard-height: max(16rem, calc(var(--logs-viewport-height) - 220px));
      --logs-expanded-height: max(18rem, calc(var(--logs-viewport-height) - 128px));
    }
  }

  .log-toolbar {
    flex-shrink: 0;
  }

  .log-file-item:focus-visible {
    outline: 2px solid #1677ff;
    outline-offset: -2px;
  }

  .log-stream-error,
  .log-list-error {
    min-width: 0;
    max-height: 45%;
    flex-shrink: 0;
    overflow: auto;
    overflow-wrap: anywhere;
  }

  .log-stream-error :deep(.ant-alert-content),
  .log-list-error :deep(.ant-alert-content) {
    min-width: 0;
  }

  .log-output {
    overscroll-behavior: contain;
  }

  .log-header-enter-active,
  .log-header-leave-active {
    max-height: 140px;
    overflow: hidden;
    transition:
      max-height 0.28s ease,
      opacity 0.22s ease,
      transform 0.28s ease;
  }

  .log-header-enter-from,
  .log-header-leave-to {
    max-height: 0;
    opacity: 0;
    transform: translateY(-8px);
  }

  .log-header-enter-to,
  .log-header-leave-from {
    max-height: 140px;
    opacity: 1;
    transform: translateY(0);
  }

  .log-sidebar-enter-active,
  .log-sidebar-leave-active {
    overflow: hidden;
    transition:
      max-width 0.3s ease,
      max-height 0.3s ease,
      opacity 0.24s ease,
      transform 0.3s ease;
  }

  .log-sidebar-enter-from,
  .log-sidebar-leave-to {
    opacity: 0;
  }

  .log-sidebar-enter-to,
  .log-sidebar-leave-from {
    opacity: 1;
  }

  @media (min-width: 1024px) {
    .log-sidebar-enter-from,
    .log-sidebar-leave-to {
      max-width: 0;
      transform: translateX(-12px);
    }

    .log-sidebar-enter-to,
    .log-sidebar-leave-from {
      max-width: 16rem;
      transform: translateX(0);
    }
  }

  @media (max-width: 1023px) {
    .log-sidebar {
      flex: 1 1 0%;
      min-height: 0;
    }

    .log-file-item,
    .log-toolbar :deep(.ant-btn),
    .log-sidebar :deep(.ant-btn),
    .log-stream-error :deep(.ant-btn) {
      min-width: 2.5rem;
      min-height: 2.5rem;
      height: auto;
    }

    .log-sidebar-enter-from,
    .log-sidebar-leave-to {
      max-height: 0;
      transform: translateY(-8px);
    }

    .log-sidebar-enter-to,
    .log-sidebar-leave-from {
      max-height: var(--logs-standard-height);
      transform: translateY(0);
    }
  }

  @media (max-width: 639px) {
    .log-stream-error {
      flex-wrap: wrap;
      row-gap: 0.5rem;
    }

    .log-stream-error :deep(.ant-alert-action) {
      flex-basis: 100%;
      margin-inline-start: 0;
    }

    .log-stream-error :deep(.ant-alert-action .ant-btn) {
      width: 100%;
    }
  }
</style>
