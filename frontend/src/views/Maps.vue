<script setup lang="ts">
  import { ref, computed, onMounted, onUnmounted, h } from 'vue';
  import { api } from '../services/api';
  import { message, Modal } from 'ant-design-vue';
  import {
    InboxOutlined,
    ReloadOutlined,
    DeleteOutlined,
    FileTextOutlined,
    PlusOutlined,
    ExclamationCircleOutlined,
    CloseCircleOutlined,
  } from '@ant-design/icons-vue';

  const activeTab = ref('local');
  const maps = ref<Array<{ name: string; size: string; info: string }>>([]);
  const downloadTasks = ref<Array<any>>([]);
  const loading = ref(false);
  const searchQuery = ref('');
  const selectedRowKeys = ref<string[]>([]);

  // Upload
  const fileList = ref([]);
  const newTaskUrl = ref('');
  const addingTask = ref(false);
  const addTaskVisible = ref(false);
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
    try {
      await api.uploadMap(file, (percent: number) => {
        onProgress({ percent });
      });
      message.success(`${file.name} 上传成功`);
      onSuccess('Ok');
      loadMaps();
    } catch (e: any) {
      message.error(`上传 ${file.name} 失败: ${e.message}`);
      onError(e);
    }
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

  onMounted(() => {
    loadMaps();
    loadDownloadTasks();
    downloadRefreshInterval = window.setInterval(loadDownloadTasks, 3000);
  });

  onUnmounted(() => {
    if (downloadRefreshInterval) clearInterval(downloadRefreshInterval);
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
                <span class="hidden sm:inline">清空所有</span>
                <span class="sm:hidden">清空</span>
              </a-button>
            </div>
          </div>

          <!-- Maps Table -->
          <a-table
            :columns="mapColumns"
            :dataSource="filteredMaps"
            :loading="loading"
            :pagination="{ pageSize: 10 }"
            rowKey="name"
            :row-selection="{ selectedRowKeys: selectedRowKeys, onChange: onSelectChange }"
            :scroll="{ x: 500 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <div class="flex items-center gap-2 min-w-[160px]">
                  <file-text-outlined class="text-lg text-gray-400 shrink-0" />
                  <span class="font-medium break-all text-sm">{{ record.name }}</span>
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
            accept=".vpk,.zip,.rar,.7z"
          >
            <p class="ant-upload-drag-icon">
              <inbox-outlined />
            </p>
            <p class="ant-upload-text">点击或拖拽上传地图文件</p>
            <p class="ant-upload-hint">支持 .vpk, .zip, .rar, .7z 格式</p>
          </a-upload-dragger>
        </div>
      </a-tab-pane>

      <a-tab-pane key="download" tab="下载任务">
        <div class="space-y-4">
          <!-- Add Task & Actions -->
          <div class="flex justify-between items-center">
            <a-button
              type="primary"
              @click="addTaskVisible = true"
              class="!flex !items-center !justify-center"
            >
              <template #icon><plus-outlined /></template>
              添加任务
            </a-button>

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
            :pagination="{ pageSize: 10 }"
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
                  <span class="text-xs text-gray-500">
                    {{ (record.progress || 0).toFixed(1) }}%
                    <span v-if="record.status === 1" class="ml-1">
                      - {{ record.formattedSpeed }}
                    </span>
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
                    @click="cancelTask(index)"
                    class="!flex !items-center !justify-center"
                    title="取消"
                  >
                    <template #icon><close-circle-outlined /></template>
                  </a-button>
                  <a-button
                    v-if="record.status === 3"
                    type="text"
                    size="small"
                    @click="restartTask(index)"
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
      </a-tab-pane>
    </a-tabs>
  </div>
</template>
