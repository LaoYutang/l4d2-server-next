<script setup lang="ts">
  import { ref, computed, onMounted } from 'vue';
  import { message } from 'ant-design-vue';
  import {
    UploadOutlined,
    DeleteOutlined,
    PoweroffOutlined,
    SearchOutlined,
    ReloadOutlined,
  } from '@ant-design/icons-vue';
  import { api } from '../services/api';
  import type { UploadProps } from 'ant-design-vue';

  interface Plugin {
    name: string;
    status: 'enabled' | 'disabled';
    description?: string;
  }

  const plugins = ref<Plugin[]>([]);
  const loading = ref(false);
  const uploading = ref(false);
  const fileList = ref<UploadProps['fileList']>([]);
  const activeTab = ref('enabled');
  const selectedRowKeys = ref<string[]>([]);
  const searchText = ref('');
  const filterText = ref('');

  const enabledPlugins = computed(() =>
    plugins.value.filter((p) => p.status === 'enabled').sort((a, b) => a.name.localeCompare(b.name))
  );
  const disabledPlugins = computed(() =>
    plugins.value
      .filter((p) => p.status === 'disabled')
      .sort((a, b) => a.name.localeCompare(b.name))
  );

  const filteredDisabledPlugins = computed(() => {
    if (!filterText.value) return disabledPlugins.value;
    const lower = filterText.value.toLowerCase();
    return disabledPlugins.value.filter(
      (p) =>
        p.name.toLowerCase().includes(lower) ||
        (p.description && p.description.toLowerCase().includes(lower))
    );
  });

  const filteredEnabledPlugins = computed(() => {
    if (!filterText.value) return enabledPlugins.value;
    const lower = filterText.value.toLowerCase();
    return enabledPlugins.value.filter(
      (p) =>
        p.name.toLowerCase().includes(lower) ||
        (p.description && p.description.toLowerCase().includes(lower))
    );
  });

  const fetchPlugins = async () => {
    loading.value = true;
    try {
      plugins.value = await api.getPlugins();
    } catch (error: any) {
      message.error('加载插件失败: ' + error.message);
    } finally {
      loading.value = false;
    }
  };

  const handleUpload = async (file: File) => {
    if (!file.name.endsWith('.zip')) {
      message.error('只允许上传 .zip 格式的文件');
      return false;
    }

    const hide = message.loading('正在上传插件...', 0);
    uploading.value = true;
    try {
      await api.uploadPlugin(file);
      message.success('插件上传成功');
      fileList.value = [];
      fetchPlugins();
    } catch (error: any) {
      message.error('上传失败: ' + error.message);
    } finally {
      uploading.value = false;
      hide();
    }
    return false; // Prevent default upload behavior
  };

  const togglePlugin = async (plugin: Plugin) => {
    const actionText = plugin.status === 'enabled' ? '禁用' : '启用';
    const hide = message.loading(`正在${actionText}插件...`, 0);

    try {
      if (plugin.status === 'enabled') {
        await api.disablePlugin(plugin.name);
      } else {
        await api.enablePlugin(plugin.name);
      }
      message.success(`插件${actionText}成功`);
      fetchPlugins();
    } catch (error: any) {
      message.error(`${actionText}插件失败: ` + error.message);
    } finally {
      hide();
    }
  };

  const deletePlugin = async (plugin: Plugin) => {
    if (plugin.status === 'enabled') {
      message.warning('请先禁用插件');
      return;
    }

    const hide = message.loading('正在删除插件...', 0);
    try {
      await api.deletePlugin(plugin.name);
      message.success('插件删除成功');
      fetchPlugins();
    } catch (error: any) {
      message.error('删除插件失败: ' + error.message);
    } finally {
      hide();
    }
  };

  const handleSearch = () => {
    filterText.value = searchText.value;
  };

  const handleReset = () => {
    searchText.value = '';
    filterText.value = '';
  };

  const handleBatchDelete = async () => {
    if (selectedRowKeys.value.length === 0) return;

    const hide = message.loading('正在批量删除插件...', 0);
    try {
      // Execute deletions sequentially to avoid potential conflicts or backend overload
      // Or concurrent if backend supports it. Here sequential for safety.
      for (const name of selectedRowKeys.value) {
        await api.deletePlugin(name);
      }
      message.success(`成功删除 ${selectedRowKeys.value.length} 个插件`);
      selectedRowKeys.value = [];
      fetchPlugins();
    } catch (error: any) {
      message.error('批量删除部分失败: ' + error.message);
      // Refresh to see what was actually deleted
      fetchPlugins();
    } finally {
      hide();
    }
  };

  const onSelectChange = (keys: any[]) => {
    selectedRowKeys.value = keys;
  };

  onMounted(() => {
    fetchPlugins();
  });

  const enabledColumns = [
    {
      title: '插件名称',
      dataIndex: 'name',
      key: 'name',
      sorter: (a: Plugin, b: Plugin) => a.name.localeCompare(b.name),
    },
    {
      title: '操作',
      key: 'actions',
      width: 200,
    },
  ];

  const disabledColumns = [
    {
      title: '插件名称',
      dataIndex: 'name',
      key: 'name',
      sorter: (a: Plugin, b: Plugin) => a.name.localeCompare(b.name),
    },
    {
      title: '操作',
      key: 'actions',
      width: 200,
    },
  ];
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
      <div>
        <h1 class="text-2xl font-bold text-gray-800">插件管理</h1>
        <p class="text-gray-500 mt-1">管理服务器插件和模组</p>
      </div>
    </div>

    <a-card :bordered="false" class="shadow-sm">
      <a-tabs v-model:activeKey="activeTab">
        <!-- Enabled Plugins Tab -->
        <a-tab-pane key="enabled" tab="已启用插件">
          <div
            class="mb-4 flex flex-col lg:flex-row justify-between items-start lg:items-center gap-4"
          >
            <div class="flex flex-col sm:flex-row gap-2 w-full lg:w-auto">
              <a-input
                v-model:value="searchText"
                placeholder="搜索插件..."
                class="w-full sm:w-[200px]"
                allow-clear
                @pressEnter="handleSearch"
              />
              <div class="flex gap-2">
                <a-button
                  type="primary"
                  @click="handleSearch"
                  class="!flex !items-center !justify-center flex-1 sm:flex-none"
                >
                  <template #icon><SearchOutlined /></template>
                  搜索
                </a-button>
                <a-button
                  @click="handleReset"
                  class="!flex !items-center !justify-center flex-1 sm:flex-none"
                >
                  <template #icon><ReloadOutlined /></template>
                  重置
                </a-button>
              </div>
            </div>
          </div>

          <a-table
            :columns="enabledColumns"
            :data-source="filteredEnabledPlugins"
            :loading="loading"
            :pagination="false"
            row-key="name"
            :scroll="{ x: 'max-content' }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <div class="font-medium text-gray-700">{{ record.name }}</div>
                <div v-if="record.description" class="text-xs text-gray-400">
                  {{ record.description }}
                </div>
              </template>

              <template v-else-if="column.key === 'actions'">
                <a-popconfirm
                  title="确定要禁用这个插件吗？"
                  ok-text="确定"
                  cancel-text="取消"
                  @confirm="togglePlugin(record as Plugin)"
                >
                  <a-button
                    type="default"
                    danger
                    size="small"
                    class="!flex !items-center !justify-center"
                  >
                    <template #icon><PoweroffOutlined /></template>
                    禁用
                  </a-button>
                </a-popconfirm>
              </template>
            </template>
          </a-table>
        </a-tab-pane>

        <!-- Disabled Plugins Tab -->
        <a-tab-pane key="disabled" tab="未启用插件">
          <div
            class="mb-4 flex flex-col lg:flex-row justify-between items-start lg:items-center gap-4"
          >
            <div class="flex flex-col sm:flex-row gap-2 w-full lg:w-auto">
              <a-input
                v-model:value="searchText"
                placeholder="搜索插件..."
                class="w-full sm:w-[200px]"
                allow-clear
                @pressEnter="handleSearch"
              />
              <div class="flex gap-2">
                <a-button
                  type="primary"
                  @click="handleSearch"
                  class="!flex !items-center !justify-center flex-1 sm:flex-none"
                >
                  <template #icon><SearchOutlined /></template>
                  搜索
                </a-button>
                <a-button
                  @click="handleReset"
                  class="!flex !items-center !justify-center flex-1 sm:flex-none"
                >
                  <template #icon><ReloadOutlined /></template>
                  重置
                </a-button>
              </div>
            </div>

            <div class="flex flex-wrap gap-2 w-full lg:w-auto">
              <a-popconfirm
                title="确定要删除选中的插件吗？"
                ok-text="确定"
                cancel-text="取消"
                @confirm="handleBatchDelete"
                :disabled="selectedRowKeys.length === 0"
              >
                <a-button
                  danger
                  :disabled="selectedRowKeys.length === 0"
                  class="!flex !items-center !justify-center flex-1 sm:flex-none"
                >
                  <template #icon><DeleteOutlined /></template>
                  批量删除
                </a-button>
              </a-popconfirm>

              <a-upload
                v-model:file-list="fileList"
                :before-upload="handleUpload"
                accept=".zip"
                :show-upload-list="false"
                :disabled="uploading"
                class="flex-1 sm:flex-none"
              >
                <a-button
                  type="primary"
                  :loading="uploading"
                  class="!flex !items-center !justify-center w-full"
                >
                  <template #icon><UploadOutlined /></template>
                  上传插件 (.zip)
                </a-button>
              </a-upload>
            </div>
          </div>

          <a-table
            :columns="disabledColumns"
            :data-source="filteredDisabledPlugins"
            :loading="loading"
            :pagination="false"
            row-key="name"
            :row-selection="{ selectedRowKeys: selectedRowKeys, onChange: onSelectChange }"
            :scroll="{ x: 'max-content' }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <div class="font-medium text-gray-700">{{ record.name }}</div>
                <div v-if="record.description" class="text-xs text-gray-400">
                  {{ record.description }}
                </div>
              </template>

              <template v-else-if="column.key === 'actions'">
                <div class="flex items-center gap-2">
                  <a-popconfirm
                    title="确定要启用这个插件吗？"
                    ok-text="确定"
                    cancel-text="取消"
                    @confirm="togglePlugin(record as Plugin)"
                  >
                    <a-button
                      type="primary"
                      size="small"
                      class="!flex !items-center !justify-center"
                    >
                      <template #icon><PoweroffOutlined /></template>
                      启用
                    </a-button>
                  </a-popconfirm>

                  <a-popconfirm
                    title="确定要删除这个插件吗？"
                    ok-text="确定"
                    cancel-text="取消"
                    @confirm="deletePlugin(record as Plugin)"
                  >
                    <a-button
                      type="text"
                      danger
                      size="small"
                      class="!flex !items-center !justify-center"
                    >
                      <template #icon><DeleteOutlined /></template>
                      删除
                    </a-button>
                  </a-popconfirm>
                </div>
              </template>
            </template>
          </a-table>
        </a-tab-pane>
      </a-tabs>
    </a-card>
  </div>
</template>
