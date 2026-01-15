<script setup lang="ts">
  import { ref, nextTick, onMounted } from 'vue';
  import { api } from '../services/api';
  import { message } from 'ant-design-vue';
  import {
    CodeOutlined,
    ClearOutlined,
    RightOutlined,
    SendOutlined,
    HistoryOutlined,
    QuestionCircleOutlined,
  } from '@ant-design/icons-vue';

  interface LogEntry {
    type: 'sent' | 'received' | 'error';
    text: string;
    time: string;
  }

  const command = ref('');
  const logs = ref<LogEntry[]>([]);
  const history = ref<string[]>([]);
  const sending = ref(false);
  const logContainer = ref<HTMLElement | null>(null);
  const commandInput = ref<any>(null);

  const scrollToBottom = async () => {
    await nextTick();
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight;
    }
  };

  const focusInput = () => {
    commandInput.value?.focus();
  };

  const fillCommand = (cmd: string) => {
    command.value = cmd;
    focusInput();
  };

  const loadHistory = () => {
    try {
      const stored = localStorage.getItem('rcon_history');
      if (stored) {
        history.value = JSON.parse(stored);
      }
    } catch (e) {
      console.error('Failed to load history', e);
    }
  };

  const addToHistory = (cmd: string) => {
    // Remove duplicates and add to front
    const newHistory = [cmd, ...history.value.filter((h) => h !== cmd)].slice(0, 8);
    history.value = newHistory;
    localStorage.setItem('rcon_history', JSON.stringify(newHistory));
  };

  const addLog = (type: LogEntry['type'], text: string) => {
    const time = new Date().toLocaleTimeString();
    logs.value.push({ type, text, time });
    scrollToBottom();
  };

  const sendCommand = async () => {
    const cmd = command.value.trim();
    if (!cmd) return;

    addToHistory(cmd);
    sending.value = true;
    addLog('sent', cmd);
    command.value = '';

    try {
      const response = await api.sendRconCommand(cmd);
      addLog('received', response || '(Empty Response)');
    } catch (e: any) {
      addLog('error', e.message || 'Command failed');
    } finally {
      sending.value = false;
      // Keep focus on input
      nextTick(() => {
        commandInput.value?.focus();
      });
    }
  };

  const clearLog = () => {
    logs.value = [];
    message.success('日志已清空');
  };

  const clearHistory = () => {
    history.value = [];
    localStorage.removeItem('rcon_history');
    message.success('历史记录已清空');
  };

  onMounted(() => {
    loadHistory();
    commandInput.value?.focus();
  });
</script>

<template>
  <div class="h-[calc(100vh-100px)] flex flex-col gap-4">
    <a-card
      class="flex-1 flex flex-col overflow-hidden shadow-xl border-0"
      :bodyStyle="{ padding: 0, display: 'flex', flexDirection: 'column', height: '100%' }"
    >
      <!-- Header -->
      <div
        class="p-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center bg-white dark:bg-gray-800 transition-colors"
      >
        <div class="flex items-center gap-2">
          <div class="w-3 h-3 rounded-full bg-red-500"></div>
          <div class="w-3 h-3 rounded-full bg-yellow-500"></div>
          <div class="w-3 h-3 rounded-full bg-green-500"></div>
          <h2 class="text-lg font-bold ml-2 font-mono flex items-center gap-2 dark:text-gray-100">
            <code-outlined /> RCON 终端
            <a-popover title="常用指令参考" placement="bottomLeft">
              <template #content>
                <div class="space-y-2 text-sm font-mono">
                  <p>
                    <span class="font-bold text-blue-500">status</span> :
                    <span class="text-gray-600 dark:text-gray-400">查看服务器状态</span>
                  </p>
                  <p>
                    <span class="font-bold text-blue-500">z_difficulty [difficulty]</span> :
                    <span class="text-gray-600 dark:text-gray-400"
                      >修改难度 (easy/normal/hard/impossible)</span
                    >
                  </p>
                  <p>
                    <span class="font-bold text-blue-500">sm_cvar mp_gamemode [mode]</span> :
                    <span class="text-gray-600 dark:text-gray-400"
                      >修改模式 (coop/versus/survival)</span
                    >
                  </p>
                  <p>
                    <span class="font-bold text-blue-500">sm_cvar [var] [val]</span> :
                    <span class="text-gray-600 dark:text-gray-400">修改服务器变量</span>
                  </p>
                  <p>
                    <span class="font-bold text-blue-500">kick [user]</span> :
                    <span class="text-gray-600 dark:text-gray-400">踢出玩家</span>
                  </p>
                  <p>
                    <span class="font-bold text-blue-500">changelevel [map]</span> :
                    <span class="text-gray-600 dark:text-gray-400">切换地图</span>
                  </p>
                  <p>
                    <span class="font-bold text-blue-500">banid [minutes] [userid]</span> :
                    <span class="text-gray-600 dark:text-gray-400">封禁玩家(0为永久)</span>
                  </p>
                </div>
              </template>
              <question-circle-outlined
                class="text-gray-400 hover:text-blue-500 cursor-help text-sm transition-colors"
              />
            </a-popover>
          </h2>
        </div>
        <a-button
          size="small"
          type="text"
          @click="clearLog"
          class="!flex !items-center !justify-center"
        >
          <template #icon><clear-outlined /></template>
          清空
        </a-button>
      </div>

      <!-- Log Output (Terminal Style) -->
      <div
        ref="logContainer"
        class="flex-1 overflow-y-auto p-4 font-mono text-sm bg-[#1e1e1e] text-gray-300 space-y-1 shadow-inner"
        @click="focusInput"
      >
        <div
          v-if="logs.length === 0"
          class="flex flex-col items-center justify-center h-full opacity-30 select-none"
        >
          <code-outlined class="text-6xl mb-4" />
          <p>等待命令输入...</p>
        </div>
        <div
          v-for="(log, index) in logs"
          :key="index"
          class="break-words whitespace-pre-wrap leading-relaxed"
        >
          <span class="opacity-40 text-xs mr-2 select-none">[{{ log.time }}]</span>
          <span v-if="log.type === 'sent'" class="text-blue-400 font-bold mr-2">➜ ~</span>
          <span
            :class="{
              'text-blue-300': log.type === 'sent',
              'text-green-400': log.type === 'received',
              'text-red-400': log.type === 'error',
            }"
            >{{ log.text }}</span
          >
        </div>
      </div>

      <!-- Quick Actions & Input -->
      <div
        class="p-4 bg-white dark:bg-gray-800 border-t border-gray-200 dark:border-gray-700 transition-colors"
      >
        <!-- Quick Commands (History) -->
        <div class="flex items-center justify-between mb-2">
          <span class="text-xs text-gray-500 dark:text-gray-400 flex items-center gap-1">
            <history-outlined /> 最近使用的指令
          </span>
          <a-button
            v-if="history.length > 0"
            type="link"
            size="small"
            class="!text-xs !p-0 !h-auto text-gray-400 hover:text-red-500"
            @click="clearHistory"
          >
            清空历史
          </a-button>
        </div>
        <div class="flex gap-2 mb-4 overflow-x-auto pb-2 scrollbar-hide min-h-[32px]">
          <template v-if="history.length > 0">
            <a-tag
              v-for="cmd in history"
              :key="cmd"
              color="blue"
              class="cursor-pointer hover:opacity-80 transition-opacity whitespace-nowrap"
              @click="fillCommand(cmd)"
            >
              {{ cmd }}
            </a-tag>
          </template>
          <div
            v-else
            class="text-gray-400 text-xs italic w-full text-center py-1 bg-gray-50 dark:bg-gray-800/50 rounded border border-dashed border-gray-200 dark:border-gray-700"
          >
            暂无历史指令，发送的指令将显示在这里
          </div>
        </div>

        <div class="flex gap-2">
          <a-input
            v-model:value="command"
            class="font-mono"
            placeholder="输入 RCON 命令..."
            size="large"
            @pressEnter="sendCommand"
            :disabled="sending"
            ref="commandInput"
          >
            <template #prefix>
              <right-outlined class="text-blue-500" />
            </template>
          </a-input>
          <a-button
            type="primary"
            size="large"
            @click="sendCommand"
            :loading="sending"
            :disabled="!command"
            class="!flex !items-center !justify-center"
          >
            <template #icon><send-outlined /></template>
            发送
          </a-button>
        </div>
      </div>
    </a-card>
  </div>
</template>
