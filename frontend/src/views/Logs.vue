<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick, watch } from 'vue';
import { api } from '../services/api';
import { message } from 'ant-design-vue';
import {
  FileTextOutlined,
  ReloadOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  DeleteOutlined,
} from '@ant-design/icons-vue';

interface LogFileInfo {
  name: string;
  date: string;
  size: number;
}

interface LogCategories {
  L: LogFileInfo[];
  errors: LogFileInfo[];
  other: LogFileInfo[];
}

const installed = ref(true);
const categories = ref<LogCategories>({ L: [], errors: [], other: [] });
const selectedFile = ref('');
const logLines = ref<string[]>([]);
const isPaused = ref(false);
const isLoading = ref(false);
const logContainer = ref<HTMLElement | null>(null);
const shouldAutoScroll = ref(true);
let eventSource: EventSource | null = null;

const formatFileSize = (size: number): string => {
  if (size < 1024) return `${size}B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)}KB`;
  return `${(size / (1024 * 1024)).toFixed(2)}MB`;
};

const loadFileList = async () => {
  isLoading.value = true;
  try {
    const data = await api.getSourceModLogs();
    installed.value = data.installed;
    if (data.installed) {
      categories.value = data.categories || { L: [], errors: [], other: [] };
    }
  } catch (e: any) {
    message.error(e.message || '获取日志列表失败');
  } finally {
    isLoading.value = false;
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
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }

  selectedFile.value = filename;
  logLines.value = [];
  isPaused.value = false;
  shouldAutoScroll.value = true;

  eventSource = api.streamLog(
    filename,
    (line) => {
      if (!isPaused.value && line !== '') {
        logLines.value.push(line);
      }
    },
    (err) => {
      message.error(err);
    }
  );
};

const togglePause = () => {
  isPaused.value = !isPaused.value;
};

const clearDisplay = () => {
  logLines.value = [];
};

watch(
  logLines,
  () => {
    nextTick(scrollToBottom);
  },
  { deep: true }
);

onMounted(() => {
  loadFileList();
});

onUnmounted(() => {
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
});
</script>

<template>
  <div class="space-y-6 p-4 md:p-6">
    <!-- Header -->
    <div
      class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4"
    >
      <div>
        <h1
          class="text-2xl font-bold text-gray-800 dark:text-gray-100 flex items-center gap-2"
        >
          <FileTextOutlined /> 日志查看
        </h1>
        <p class="text-gray-500 dark:text-gray-400 mt-1">
          查看 SourceMod 插件日志、错误日志及其他日志文件，支持实时推送
        </p>
      </div>
    </div>

    <!-- Main Content -->
    <div class="h-[calc(100vh-220px)] flex flex-col lg:flex-row gap-4">
      <!-- Sidebar -->
      <div class="w-full lg:w-64 flex-shrink-0">
        <a-card
          class="h-full overflow-hidden shadow-xl border-0"
          :body-style="{ padding: '16px', height: '100%', overflow: 'hidden', display: 'flex', flexDirection: 'column' }"
        >
          <div class="flex items-center justify-between mb-3">
            <span class="font-bold text-gray-800 dark:text-gray-100"
              >日志文件列表</span
            >
            <a-button
              type="text"
              size="small"
              @click="loadFileList"
              :loading="isLoading"
              class="!flex !items-center !justify-center"
            >
              <template #icon><ReloadOutlined /></template>
            </a-button>
          </div>

          <div
            v-if="!installed"
            class="text-center py-8 text-gray-500 dark:text-gray-400"
          >
            <FileTextOutlined class="text-4xl mb-2" />
            <p>请安装 SourceMod</p>
          </div>

          <div v-else class="flex-1 overflow-y-auto space-y-3">
            <!-- L Logs -->
            <div v-if="categories.L?.length > 0">
              <div
                class="text-xs font-semibold text-gray-500 dark:text-gray-400 px-1 py-1"
              >
                L 日志
              </div>
              <div
                v-for="file in categories.L"
                :key="file.name"
                class="flex items-center justify-between px-2 py-1.5 rounded cursor-pointer text-sm transition-colors"
                :class="
                  selectedFile === file.name
                    ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
                    : 'hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-700 dark:text-gray-300'
                "
                @click="selectFile(file.name)"
              >
                <span class="truncate flex-1 mr-2">{{ file.name }}</span>
                <span
                  class="text-xs text-gray-400 dark:text-gray-500 flex-shrink-0"
                  >{{ formatFileSize(file.size) }}</span
                >
              </div>
            </div>

            <!-- Error Logs -->
            <div v-if="categories.errors?.length > 0">
              <div
                class="text-xs font-semibold text-gray-500 dark:text-gray-400 px-1 py-1"
              >
                错误日志
              </div>
              <div
                v-for="file in categories.errors"
                :key="file.name"
                class="flex items-center justify-between px-2 py-1.5 rounded cursor-pointer text-sm transition-colors"
                :class="
                  selectedFile === file.name
                    ? 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300'
                    : 'hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-700 dark:text-gray-300'
                "
                @click="selectFile(file.name)"
              >
                <span class="truncate flex-1 mr-2">{{ file.name }}</span>
                <span
                  class="text-xs text-gray-400 dark:text-gray-500 flex-shrink-0"
                  >{{ formatFileSize(file.size) }}</span
                >
              </div>
            </div>

            <!-- Other Logs -->
            <div v-if="categories.other?.length > 0">
              <div
                class="text-xs font-semibold text-gray-500 dark:text-gray-400 px-1 py-1"
              >
                其他日志
              </div>
              <div
                v-for="file in categories.other"
                :key="file.name"
                class="flex items-center justify-between px-2 py-1.5 rounded cursor-pointer text-sm transition-colors"
                :class="
                  selectedFile === file.name
                    ? 'bg-gray-200 dark:bg-gray-700 text-gray-800 dark:text-gray-200'
                    : 'hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-700 dark:text-gray-300'
                "
                @click="selectFile(file.name)"
              >
                <span class="truncate flex-1 mr-2">{{ file.name }}</span>
                <span
                  class="text-xs text-gray-400 dark:text-gray-500 flex-shrink-0"
                  >{{ formatFileSize(file.size) }}</span
                >
              </div>
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

      <!-- Content -->
      <div class="flex-1 min-h-0">
        <a-card
          class="h-full overflow-hidden shadow-xl border-0"
          :body-style="{ padding: 0, height: '100%', display: 'flex', flexDirection: 'column', overflow: 'hidden' }"
        >
          <!-- Toolbar -->
          <div
            class="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 transition-colors"
          >
            <div class="flex items-center gap-2 min-w-0">
              <span
                class="font-mono text-sm text-gray-700 dark:text-gray-300 truncate"
              >
                {{ selectedFile || '请选择日志文件' }}
              </span>
              <a-tag
                v-if="isPaused"
                color="orange"
                class="ml-2 flex-shrink-0"
                >已暂停</a-tag
              >
            </div>
            <div class="flex items-center gap-2 flex-shrink-0">
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
                danger
                @click="clearDisplay"
                :disabled="!selectedFile || logLines.length === 0"
                class="!flex !items-center !justify-center"
              >
                <template #icon><DeleteOutlined /></template>
                清空
              </a-button>
            </div>
          </div>

          <!-- Log Display -->
          <div
            ref="logContainer"
            class="flex-1 overflow-y-auto p-4 font-mono text-sm bg-[#1e1e1e] text-gray-300 space-y-0.5"
            @scroll="handleScroll"
          >
            <template v-if="logLines.length > 0">
              <div
                v-for="(line, index) in logLines"
                :key="index"
                class="break-words whitespace-pre-wrap leading-snug"
              >
                <span>{{ line }}</span>
              </div>
            </template>
            <div
              v-else-if="selectedFile"
              class="flex flex-col items-center justify-center h-full opacity-30 select-none"
            >
              <FileTextOutlined class="text-6xl mb-4" />
              <p>等待日志内容...</p>
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
  </div>
</template>
