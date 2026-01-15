<script setup lang="ts">
  import { ref, nextTick, onMounted } from 'vue';
  import { api } from '../services/api';
  import { message } from 'ant-design-vue';
  import { CodeOutlined, ClearOutlined, RightOutlined, SendOutlined } from '@ant-design/icons-vue';

  interface LogEntry {
    type: 'sent' | 'received' | 'error';
    text: string;
    time: string;
  }

  const command = ref('');
  const logs = ref<LogEntry[]>([]);
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

  const addLog = (type: LogEntry['type'], text: string) => {
    const time = new Date().toLocaleTimeString();
    logs.value.push({ type, text, time });
    scrollToBottom();
  };

  const sendCommand = async () => {
    const cmd = command.value.trim();
    if (!cmd) return;

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

  onMounted(() => {
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
        <!-- Quick Commands -->
        <div class="flex gap-2 mb-4 overflow-x-auto pb-2 scrollbar-hide">
          <a-tag color="blue" class="cursor-pointer" @click="fillCommand('status ')">status</a-tag>
          <a-tag color="cyan" class="cursor-pointer" @click="fillCommand('z_difficulty ')"
            >z_difficulty</a-tag
          >
          <a-tag color="green" class="cursor-pointer" @click="fillCommand('sm_cvar mp_gamemode ')"
            >mp_gamemode</a-tag
          >
          <a-tag color="orange" class="cursor-pointer" @click="fillCommand('sv_maxplayers ')"
            >sv_maxplayers</a-tag
          >
          <a-tag color="orange" class="cursor-pointer" @click="fillCommand('sv_visiblemaxplayers ')"
            >sv_visiblemaxplayers</a-tag
          >
          <a-tag color="purple" class="cursor-pointer" @click="fillCommand('sm_timer ')"
            >sm_timer</a-tag
          >
          <a-tag
            color="magenta"
            class="cursor-pointer"
            @click="fillCommand('sm_cvar sar_respawn_survivor_limit ')"
            >survivor_limit</a-tag
          >
          <a-tag
            color="magenta"
            class="cursor-pointer"
            @click="fillCommand('sm_cvar sar_respawn_survivor_time ')"
            >survivor_time</a-tag
          >
          <a-tag
            color="magenta"
            class="cursor-pointer"
            @click="fillCommand('sm_cvar sar_respawn_survivor_time_add ')"
            >survivor_time_add</a-tag
          >
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
