<script setup lang="ts">
  import { onMounted, reactive, ref } from 'vue';
  import { message } from 'ant-design-vue';
  import { AuditOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons-vue';
  import {
    api,
    type AuditListParams,
    type AuditLogItem,
  } from '../services/api';

  type SuccessFilter = 'all' | 'success' | 'failed';

  const loading = ref(false);
  const records = ref<AuditLogItem[]>([]);
  const total = ref(0);
  const currentPage = ref(1);
  const pageSize = ref(20);

  const filters = reactive<{
    timeRange?: [string, string];
    role: '' | 'admin' | 'guest';
    ip: string;
    path: string;
    success: SuccessFilter;
    keyword: string;
  }>({
    timeRange: undefined,
    role: '',
    ip: '',
    path: '',
    success: 'all',
    keyword: '',
  });

  const toUnixSeconds = (value?: string) => {
    if (!value) return 0;
    const timestamp = new Date(value).getTime();
    return Number.isNaN(timestamp) ? 0 : Math.floor(timestamp / 1000);
  };

  const buildParams = (): AuditListParams => ({
    page: currentPage.value,
    page_size: pageSize.value,
    start_time: toUnixSeconds(filters.timeRange?.[0]),
    end_time: toUnixSeconds(filters.timeRange?.[1]),
    role: filters.role,
    ip: filters.ip.trim(),
    path: filters.path.trim(),
    success:
      filters.success === 'all' ? null : filters.success === 'success',
    keyword: filters.keyword.trim(),
  });

  const loadAuditLogs = async () => {
    loading.value = true;
    try {
      const response = await api.getAuditLogs(buildParams());
      records.value = response.items || [];
      total.value = response.total;
      currentPage.value = response.page;
      pageSize.value = response.page_size;
    } catch (error: any) {
      message.error(error?.message || '获取审计记录失败');
    } finally {
      loading.value = false;
    }
  };

  const handleSearch = () => {
    currentPage.value = 1;
    loadAuditLogs();
  };

  const handleReset = () => {
    filters.timeRange = undefined;
    filters.role = '';
    filters.ip = '';
    filters.path = '';
    filters.success = 'all';
    filters.keyword = '';
    currentPage.value = 1;
    loadAuditLogs();
  };

  const handlePageChange = (page: number, size: number) => {
    currentPage.value = page;
    pageSize.value = size;
    loadAuditLogs();
  };

  const handleMobilePageSizeChange = () => {
    currentPage.value = 1;
    loadAuditLogs();
  };

  const formatTime = (timestamp: number) => {
    if (!timestamp) return '-';
    return new Date(timestamp * 1000).toLocaleString('zh-CN', {
      hour12: false,
    });
  };

  onMounted(loadAuditLogs);
</script>

<template>
  <div class="space-y-6 p-4 md:p-6">
    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
      <div>
        <h1 class="text-2xl font-bold text-gray-800 dark:text-gray-100 flex items-center gap-2">
          <AuditOutlined /> 操作审计
        </h1>
        <p class="text-gray-500 dark:text-gray-400 mt-1">
          查看管理器关键操作的成功状态、来源和说明
        </p>
      </div>
      <a-button
        class="!flex !items-center !justify-center"
        :loading="loading"
        @click="loadAuditLogs"
      >
        <template #icon><ReloadOutlined /></template>
        刷新
      </a-button>
    </div>

    <a-card
      :bordered="false"
      class="shadow-sm dark:bg-gray-800 rounded-xl overflow-hidden"
      :body-style="{ padding: '0' }"
    >
      <div
        class="p-4 md:p-5 border-b border-gray-100 bg-gray-50/60 dark:border-gray-700 dark:bg-gray-900/30"
      >
        <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3">
          <a-range-picker
            v-model:value="filters.timeRange"
            show-time
            value-format="YYYY-MM-DDTHH:mm:ss"
            :placeholder="['开始时间', '结束时间']"
            class="w-full"
          />
          <a-select v-model:value="filters.role" class="w-full" placeholder="全部角色">
            <a-select-option value="">全部角色</a-select-option>
            <a-select-option value="admin">admin</a-select-option>
            <a-select-option value="guest">guest</a-select-option>
          </a-select>
          <a-select v-model:value="filters.success" class="w-full" placeholder="全部状态">
            <a-select-option value="all">全部状态</a-select-option>
            <a-select-option value="success">成功</a-select-option>
            <a-select-option value="failed">失败</a-select-option>
          </a-select>
          <a-input
            v-model:value="filters.ip"
            allow-clear
            placeholder="筛选 IP"
            @press-enter="handleSearch"
          />
          <a-input
            v-model:value="filters.path"
            allow-clear
            placeholder="筛选 Path"
            @press-enter="handleSearch"
          />
          <a-input
            v-model:value="filters.keyword"
            allow-clear
            placeholder="搜索 Detail"
            @press-enter="handleSearch"
          />
        </div>
        <div class="grid grid-cols-2 sm:flex sm:justify-end gap-2 mt-4">
          <a-button class="w-full sm:w-auto" @click="handleReset">重置</a-button>
          <a-button
            type="primary"
            class="w-full sm:w-auto !flex !items-center !justify-center"
            :loading="loading"
            @click="handleSearch"
          >
            <template #icon><SearchOutlined /></template>
            查询
          </a-button>
        </div>
      </div>

      <div class="hidden md:block overflow-x-auto">
        <table class="w-full min-w-[960px] text-left text-sm">
          <thead
            class="bg-gray-50 dark:bg-gray-900/50 text-gray-500 dark:text-gray-400 border-b border-gray-100 dark:border-gray-700"
          >
            <tr>
              <th class="px-5 py-4 font-medium whitespace-nowrap">时间</th>
              <th class="px-4 py-4 font-medium">角色</th>
              <th class="px-4 py-4 font-medium">IP</th>
              <th class="px-4 py-4 font-medium">Path</th>
              <th class="px-4 py-4 font-medium whitespace-nowrap">状态</th>
              <th class="px-5 py-4 font-medium">Detail</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-gray-700">
            <tr v-if="loading">
              <td colspan="6" class="py-12 text-center text-gray-500"><a-spin /> 加载中...</td>
            </tr>
            <tr v-else-if="records.length === 0">
              <td colspan="6" class="py-12 text-center text-gray-500 dark:text-gray-400">
                暂无审计记录
              </td>
            </tr>
            <tr
              v-for="(record, index) in records"
              v-else
              :key="`${record.time}-${record.ip}-${record.path}-${index}`"
              class="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
            >
              <td class="px-5 py-4 text-gray-600 dark:text-gray-400 whitespace-nowrap">
                {{ formatTime(record.time) }}
              </td>
              <td class="px-4 py-4">
                <a-tag :color="record.role === 'admin' ? 'blue' : 'default'">{{ record.role }}</a-tag>
              </td>
              <td class="px-4 py-4 text-gray-700 dark:text-gray-300">
                <div class="font-mono whitespace-nowrap">{{ record.ip }}</div>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap">
                  {{ record.location || '未知' }}
                </div>
              </td>
              <td class="px-4 py-4 font-mono text-xs text-gray-700 dark:text-gray-300 break-all">
                {{ record.path }}
              </td>
              <td class="px-4 py-4">
                <a-tag :color="record.success ? 'success' : 'error'">
                  {{ record.success ? '成功' : '失败' }}
                </a-tag>
              </td>
              <td class="px-5 py-4 text-gray-700 dark:text-gray-300 whitespace-pre-wrap break-words max-w-md">
                {{ record.detail || '-' }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="md:hidden">
        <div v-if="loading" class="p-10 text-center text-gray-500"><a-spin /> 加载中...</div>
        <div
          v-else-if="records.length === 0"
          class="p-10 text-center text-gray-500 dark:text-gray-400"
        >
          暂无审计记录
        </div>
        <div v-else class="divide-y divide-gray-100 dark:divide-gray-700">
          <div
            v-for="(record, index) in records"
            :key="`${record.time}-${record.ip}-${record.path}-${index}`"
            class="p-4 space-y-3 bg-white dark:bg-gray-800"
          >
            <div class="flex justify-between items-start gap-3">
              <span class="text-sm text-gray-600 dark:text-gray-400">
                {{ formatTime(record.time) }}
              </span>
              <a-tag :color="record.success ? 'success' : 'error'" class="shrink-0 !mr-0">
                {{ record.success ? '成功' : '失败' }}
              </a-tag>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <a-tag :color="record.role === 'admin' ? 'blue' : 'default'" class="!mr-0">
                {{ record.role }}
              </a-tag>
              <div class="min-w-0">
                <div class="font-mono text-sm text-gray-700 dark:text-gray-300 break-all">
                  {{ record.ip }}
                </div>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400 break-words">
                  {{ record.location || '未知' }}
                </div>
              </div>
            </div>
            <div
              class="font-mono text-xs rounded-md px-3 py-2 break-all bg-gray-50 text-gray-700 border border-gray-100 dark:bg-gray-900/60 dark:text-gray-300 dark:border-gray-700"
            >
              {{ record.path }}
            </div>
            <div class="text-sm text-gray-700 dark:text-gray-300 whitespace-pre-wrap break-words">
              {{ record.detail || '-' }}
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="total > 0"
        class="px-4 py-4 border-t border-gray-100 dark:border-gray-700"
      >
        <div class="hidden md:flex justify-end">
          <a-pagination
            :current="currentPage"
            :page-size="pageSize"
            :total="total"
            :page-size-options="['20', '50', '100']"
            show-size-changer
            show-less-items
            :show-total="(count: number) => `共 ${count} 条`"
            @change="handlePageChange"
          />
        </div>
        <div class="md:hidden flex items-center justify-between gap-3">
          <a-select
            v-model:value="pageSize"
            size="small"
            class="w-20 shrink-0"
            @change="handleMobilePageSizeChange"
          >
            <a-select-option :value="20">20 条</a-select-option>
            <a-select-option :value="50">50 条</a-select-option>
            <a-select-option :value="100">100 条</a-select-option>
          </a-select>
          <a-pagination
            simple
            size="small"
            :current="currentPage"
            :page-size="pageSize"
            :total="total"
            @change="handlePageChange"
          />
        </div>
      </div>
    </a-card>
  </div>
</template>
