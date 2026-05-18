<script setup lang="ts">
  import { ref, computed, onMounted, onUnmounted, h, watch, reactive } from 'vue';
  defineOptions({ name: 'Maps' });
  import { api, type WorkshopParseResult } from '../services/api';
  import { message, Modal } from 'ant-design-vue';
  import type { TablePaginationConfig } from 'ant-design-vue';
  import {
    InboxOutlined,
    ReloadOutlined,
    DeleteOutlined,
    FileTextOutlined,
    PlusOutlined,
    ExclamationCircleOutlined,
    CloseCircleOutlined,
    PlayCircleOutlined,
    SearchOutlined,
    DownloadOutlined,
  } from '@ant-design/icons-vue';

  const activeTab = ref('local');
  const maps = ref<Array<{ name: string; size: string; info: string }>>([]);
  const downloadTasks = ref<Array<any>>([]);
  const loading = ref(false);
  const searchQuery = ref('');
  const selectedRowKeys = ref<string[]>([]);

  // Upload
  const fileList = ref<any[]>([]);
  const uploadSpeeds = ref<Record<string, string>>({});
  const uploadStates = ref<Record<string, { uploadId: string }>>({});
  const uploadControllers = ref<Record<string, AbortController>>({});
  const uploadPercents = ref<Record<string, number>>({});
  const newTaskUrl = ref('');
  const addingTask = ref(false);
  const addTaskVisible = ref(false);
  const workshopParseVisible = ref(false);
  const workshopUrl = ref('');
  const workshopParsing = ref(false);
  const workshopAddingAll = ref(false);
  const workshopResult = ref<WorkshopParseResult | null>(null);
  const addingWorkshopItems = ref<Record<string, boolean>>({});
  const addedWorkshopItems = ref<Record<string, boolean>>({});
  let downloadRefreshInterval: number | null = null;

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
    return maps.value.filter((m) => m.name.toLowerCase().includes(q));
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

  const customRequest = async (options: any) => {
    const { file, onSuccess, onError, onProgress } = options;
    delete uploadStates.value[file.name];
    // 取消同文件的上一次上传（如果有）
    const existingController = uploadControllers.value[file.name];
    if (existingController) {
      existingController.abort();
    }
    const controller = new AbortController();
    uploadControllers.value[file.name] = controller;

    try {
      const result = await api.uploadMap(
        file,
        ({ percent, speed }: { percent: number; speed: string }) => {
          uploadSpeeds.value[file.name] = speed;
          uploadPercents.value[file.name] = percent;
          onProgress({ percent });
        },
        controller.signal
      );
      delete uploadSpeeds.value[file.name];
      delete uploadControllers.value[file.name];
      if ('success' in result && result.success) {
        delete uploadPercents.value[file.name];
        message.success(`${file.name} 上传成功`);
        onSuccess('Ok');
        loadMaps();
      } else {
        const failed = result as { success: false; uploadId: string; uploadedChunks: number[] };
        uploadStates.value[file.name] = { uploadId: failed.uploadId };
        const currentPercent = uploadPercents.value[file.name] || file.percent || 0;
        message.warning(`${file.name} 上传中断，可点击继续上传恢复`);
        onProgress({ percent: currentPercent });
        onError(new Error('Upload interrupted'));
      }
    } catch (e: any) {
      delete uploadSpeeds.value[file.name];
      delete uploadControllers.value[file.name];
      const currentPercent = uploadPercents.value[file.name] || file.percent || 0;
      if (e.message === '上传已取消') {
        onProgress({ percent: currentPercent });
        onError(e);
        return;
      }
      message.error(`上传 ${file.name} 失败: ${e.message}`);
      onProgress({ percent: currentPercent });
      onError(e);
    }
  };

  const resumeUpload = async (fileItem: any) => {
    const state = uploadStates.value[fileItem.name];
    if (!state) return;

    const targetFile = fileList.value.find((f: any) => f.uid === fileItem.uid);
    // 保持当前进度，不要清零
    const currentPercent = targetFile?.percent || 0;
    if (targetFile) {
      targetFile.status = 'uploading';
    }

    const controller = new AbortController();
    uploadControllers.value[fileItem.name] = controller;

    try {
      const fileObj = fileItem.originFileObj || fileItem;
      await api.resumeUpload(
        state.uploadId,
        fileObj,
        ({ percent, speed }: { percent: number; speed: string }) => {
          uploadSpeeds.value[fileItem.name] = speed;
          uploadPercents.value[fileItem.name] = percent;
          if (targetFile) {
            targetFile.percent = percent;
          }
        },
        controller.signal
      );
      delete uploadSpeeds.value[fileItem.name];
      delete uploadStates.value[fileItem.name];
      delete uploadControllers.value[fileItem.name];
      delete uploadPercents.value[fileItem.name];
      message.success(`${fileItem.name} 上传成功`);
      if (targetFile) {
        targetFile.status = 'done';
        targetFile.percent = 100;
      }
      loadMaps();
    } catch (e: any) {
      delete uploadSpeeds.value[fileItem.name];
      delete uploadControllers.value[fileItem.name];
      const failedPercent = uploadPercents.value[fileItem.name] || currentPercent;
      if (e.message === '上传已取消') {
        if (targetFile) {
          targetFile.status = 'error';
          targetFile.percent = failedPercent;
        }
        return;
      }
      message.error(`续传 ${fileItem.name} 失败: ${e.message}`);
      if (targetFile) {
        targetFile.status = 'error';
        targetFile.percent = failedPercent;
      }
    }
  };

  const removeUploadFile = async (uid: string) => {
    const file = fileList.value.find((f: any) => f.uid === uid);
    if (!file) return;

    // 如果有正在进行的上传，先取消
    const controller = uploadControllers.value[file.name];
    if (controller) {
      controller.abort();
      delete uploadControllers.value[file.name];
    }

    // 清理本地状态
    delete uploadStates.value[file.name];
    delete uploadSpeeds.value[file.name];
    delete uploadPercents.value[file.name];

    fileList.value = fileList.value.filter((f: any) => f.uid !== uid);
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

  // Download Tasks Logic
  const loadDownloadTasks = async () => {
    try {
      downloadTasks.value = await api.getDownloadTasks();
    } catch (e) {
      console.error(e);
    }
  };

  const addDownloadTask = async () => {
    if (!newTaskUrl.value) return;
    addingTask.value = true;
    try {
      await api.addDownloadTask(newTaskUrl.value);
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

  const workshopItems = computed(() => workshopResult.value?.items || []);

  const getWorkshopDownloadFilename = (item: any) => {
    const title = String(item.title || '').trim();
    if (title) {
      return /\.(vpk|zip|rar|7z)$/i.test(title) ? title : `${title}.vpk`;
    }
    return `${item.publishedfileid}.vpk`;
  };

  const isWorkshopItemAdded = (item: any) => {
    return !!addedWorkshopItems.value[item.publishedfileid];
  };

  const allWorkshopItemsAdded = computed(() => {
    return workshopItems.value.length > 0 && workshopItems.value.every(isWorkshopItemAdded);
  });

  const formatWorkshopFileSize = (size: string | number) => {
    const bytes = Number(size);
    if (!Number.isFinite(bytes) || bytes <= 0) return '未知大小';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  };

  const parseWorkshopLink = async () => {
    const link = workshopUrl.value.trim();
    if (!link) {
      message.error('请输入工坊链接或 ID');
      return;
    }

    workshopParsing.value = true;
    workshopResult.value = null;
    addedWorkshopItems.value = {};
    addingWorkshopItems.value = {};
    try {
      const result = await api.parseWorkshopLink(link);
      workshopResult.value = result;
      message.success(`解析成功，共 ${result.items.length} 个可下载文件`);
    } catch (e: any) {
      message.error('解析失败: ' + e.message);
    } finally {
      workshopParsing.value = false;
    }
  };

  const openWorkshopParseModal = () => {
    workshopParseVisible.value = true;
  };

  const addWorkshopDownload = async (item: any, refresh = true) => {
    if (!item.file_url) {
      message.error('该工坊条目没有可用下载链接');
      return false;
    }

    const itemID = item.publishedfileid;
    addingWorkshopItems.value = { ...addingWorkshopItems.value, [itemID]: true };
    try {
      await api.addDownloadTask(item.file_url, getWorkshopDownloadFilename(item));
      addedWorkshopItems.value = { ...addedWorkshopItems.value, [itemID]: true };
      if (refresh) {
        message.success(`${getWorkshopDownloadFilename(item)} 已添加到下载任务`);
        loadDownloadTasks();
      }
      return true;
    } catch (e: any) {
      if (refresh) {
        message.error('添加下载任务失败: ' + e.message);
      }
      return false;
    } finally {
      addingWorkshopItems.value = { ...addingWorkshopItems.value, [itemID]: false };
    }
  };

  const downloadAllWorkshopItems = async () => {
    const pendingItems = workshopItems.value.filter((item) => !isWorkshopItemAdded(item));
    if (pendingItems.length === 0) return;

    workshopAddingAll.value = true;
    let successCount = 0;
    let failCount = 0;
    try {
      for (const item of pendingItems) {
        const ok = await addWorkshopDownload(item, false);
        if (ok) {
          successCount++;
        } else {
          failCount++;
        }
      }

      if (successCount > 0) {
        message.success(`已添加 ${successCount} 个工坊下载任务`);
        loadDownloadTasks();
      }
      if (failCount > 0) {
        message.warning(`${failCount} 个任务添加失败`);
      }
      if (successCount > 0 && failCount === 0) {
        workshopParseVisible.value = false;
      }
    } finally {
      workshopAddingAll.value = false;
    }
  };

  const cancelTask = async (index: number) => {
    try {
      await api.cancelDownloadTask(index);
      message.success('任务已取消');
      loadDownloadTasks();
    } catch (e: any) {
      message.error('取消任务失败: ' + e.message);
    }
  };

  const restartTask = async (index: number) => {
    try {
      await api.restartDownloadTask(index);
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

  const mapColumns = [
    { title: '地图名称', dataIndex: 'name', key: 'name' },
    { title: '大小', dataIndex: 'size', key: 'size', width: 120 },
    { title: '操作', key: 'action', width: 100, align: 'right' as const },
  ];

  const taskColumns = [
    { title: '文件/URL', dataIndex: 'filename', key: 'filename' },
    { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
    { title: '进度', dataIndex: 'progress', key: 'progress', width: 200 },
    { title: '操作', key: 'action', width: 80, align: 'right' as const },
  ];

  const workshopColumns = [
    { title: '预览', key: 'preview', width: 84 },
    { title: '工坊地图', key: 'info' },
    { title: '大小', key: 'size', width: 120 },
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

  const startPolling = () => {
    if (downloadRefreshInterval) return;
    loadDownloadTasks(); // Immediate load
    downloadRefreshInterval = window.setInterval(loadDownloadTasks, 3000);
  };

  const stopPolling = () => {
    if (downloadRefreshInterval) {
      clearInterval(downloadRefreshInterval);
      downloadRefreshInterval = null;
    }
  };

  watch(activeTab, (newTab) => {
    if (newTab === 'download') {
      startPolling();
    } else {
      stopPolling();
    }
  });

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

  onMounted(() => {
    loadMaps();
    if (activeTab.value === 'download') {
      startPolling();
    }
  });

  onUnmounted(() => {
    stopPolling();
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
            :scroll="{ x: 500 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <div class="flex items-center gap-2 min-w-[160px]">
                  <file-text-outlined class="text-lg text-gray-400 dark:text-gray-500 shrink-0" />
                  <span class="font-medium break-all text-sm dark:text-gray-200">{{
                    record.name
                  }}</span>
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
      </a-tab-pane>

      <a-tab-pane key="upload" tab="上传地图">
        <div class="space-y-4">
          <a-upload-dragger
            v-model:fileList="fileList"
            name="file"
            :multiple="true"
            :customRequest="customRequest"
            :showUploadList="false"
            accept=".vpk,.zip,.rar,.7z"
          >
            <p class="ant-upload-drag-icon">
              <inbox-outlined />
            </p>
            <p class="ant-upload-text">点击或拖拽上传地图文件</p>
            <p class="ant-upload-hint">支持 .vpk, .zip, .rar, .7z 格式</p>
          </a-upload-dragger>

          <!-- 自定义上传列表 -->
          <div v-if="fileList.length > 0" class="space-y-2 mt-4">
            <div
              v-for="file in fileList"
              :key="file.uid"
              class="flex flex-col gap-1 p-2 rounded border border-gray-200 dark:border-gray-700"
            >
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2 min-w-0">
                  <span class="text-sm truncate" :title="file.name">{{ file.name }}</span>
                  <span
                    v-if="file.status === 'uploading' && uploadSpeeds[file.name]"
                    class="text-xs text-gray-500 whitespace-nowrap"
                  >
                    {{ uploadSpeeds[file.name] }}
                  </span>
                  <span
                    v-if="file.status === 'error'"
                    class="text-xs text-red-500 whitespace-nowrap"
                  >
                    上传中断
                  </span>
                </div>
                <a-space>
                  <a-button
                    v-if="file.status === 'error' && uploadStates[file.name]"
                    type="text"
                    size="small"
                    class="!flex !items-center"
                    @click="resumeUpload(file)"
                  >
                    <template #icon><play-circle-outlined /></template>
                    继续上传
                  </a-button>
                  <a-button
                    v-if="file.status === 'uploading'"
                    type="text"
                    size="small"
                    danger
                    @click="removeUploadFile(file.uid)"
                  >
                    取消
                  </a-button>
                  <a-button
                    v-else
                    type="text"
                    size="small"
                    danger
                    @click="removeUploadFile(file.uid)"
                  >
                    <template #icon><close-circle-outlined /></template>
                  </a-button>
                </a-space>
              </div>
              <a-progress
                v-if="file.status === 'uploading' || file.status === 'done' || file.status === 'error'"
                :percent="Number((uploadPercents[file.name] || file.percent || 0).toFixed(1))"
                size="small"
                :show-info="false"
                :status="
                  file.status === 'error' ? 'exception' : file.status === 'done' ? 'success' : 'active'
                "
              />
            </div>
          </div>
        </div>
      </a-tab-pane>

      <a-tab-pane key="download" tab="下载任务">
        <div class="space-y-4">
          <!-- Add Task & Actions -->
          <div class="flex justify-between items-center">
            <div class="flex gap-2 flex-wrap">
              <a-button
                type="primary"
                @click="addTaskVisible = true"
                class="!flex !items-center !justify-center"
              >
                <template #icon><plus-outlined /></template>
                添加任务
              </a-button>
              <a-button
                @click="openWorkshopParseModal"
                class="!flex !items-center !justify-center"
              >
                <template #icon><search-outlined /></template>
                解析工坊链接
              </a-button>
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

          <!-- Task List -->
          <a-table
            :columns="taskColumns"
            :dataSource="downloadTasks"
            :pagination="taskPaginationConfig"
            @change="handleTaskTableChange"
            :rowKey="(_: any, index?: number) => index || 0"
            :scroll="{ x: 500 }"
          >
            <template #bodyCell="{ column, record, index }">
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
                    @click="index !== undefined && cancelTask(index)"
                    class="!flex !items-center !justify-center"
                    title="取消"
                  >
                    <template #icon><close-circle-outlined /></template>
                  </a-button>
                  <a-button
                    v-if="record.status === 3"
                    type="text"
                    size="small"
                    @click="index !== undefined && restartTask(index)"
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
          v-model:open="workshopParseVisible"
          title="解析工坊链接"
          width="920px"
          :footer="null"
        >
          <div class="space-y-4">
            <a-input-search
              v-model:value="workshopUrl"
              placeholder="粘贴 Steam 工坊链接、合集链接或输入工坊 ID"
              enter-button="解析"
              :loading="workshopParsing"
              @search="parseWorkshopLink"
            />

            <div v-if="workshopResult" class="space-y-3">
              <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
                <div class="text-sm text-gray-600 dark:text-gray-400">
                  解析到 {{ workshopItems.length }} 个可下载文件
                  <span class="ml-2 text-xs text-gray-400">源 ID: {{ workshopResult.source_id }}</span>
                </div>
                <a-button
                  type="primary"
                  :loading="workshopAddingAll"
                  :disabled="workshopItems.length === 0 || allWorkshopItemsAdded"
                  @click="downloadAllWorkshopItems"
                  class="!flex !items-center !justify-center"
                >
                  <template #icon><download-outlined /></template>
                  全部下载
                </a-button>
              </div>

              <a-table
                :columns="workshopColumns"
                :dataSource="workshopItems"
                :pagination="{ pageSize: 8, showSizeChanger: false }"
                :rowKey="(record: any) => record.publishedfileid"
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
                        {{ record.title || `工坊 ${record.publishedfileid}` }}
                      </div>
                      <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                        ID: {{ record.publishedfileid }}
                      </div>
                    </div>
                  </template>
                  <template v-else-if="column.key === 'size'">
                    <a-tag>{{ formatWorkshopFileSize(record.file_size) }}</a-tag>
                  </template>
                  <template v-else-if="column.key === 'action'">
                    <a-button
                      size="small"
                      type="primary"
                      :loading="addingWorkshopItems[record.publishedfileid]"
                      :disabled="isWorkshopItemAdded(record)"
                      @click="addWorkshopDownload(record)"
                      class="!flex !items-center !justify-center"
                    >
                      <template #icon><download-outlined /></template>
                      {{ isWorkshopItemAdded(record) ? '已添加' : '下载' }}
                    </a-button>
                  </template>
                </template>
              </a-table>
            </div>
          </div>
        </a-modal>
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<style scoped>
  /* Dark mode overrides for Upload Dragger */
  :global(.dark) :deep(.ant-upload.ant-upload-drag) {
    background-color: #1f2937 !important;
    border-color: #374151 !important;
  }

  :global(.dark) :deep(.ant-upload.ant-upload-drag .ant-upload-text) {
    color: #e5e7eb !important;
  }

  :global(.dark) :deep(.ant-upload.ant-upload-drag .ant-upload-hint) {
    color: #9ca3af !important;
  }

  :global(.dark) :deep(.ant-upload.ant-upload-drag:hover) {
    border-color: #3b82f6 !important;
  }

  /* Target the icon specifically */
  :global(.dark) :deep(.ant-upload.ant-upload-drag .anticon) {
    color: #3b82f6 !important;
  }
</style>
