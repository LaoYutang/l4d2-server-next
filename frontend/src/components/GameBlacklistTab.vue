<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue';
  import { message, Modal } from 'ant-design-vue';
  import {
    DeleteOutlined,
    ExclamationCircleOutlined,
    PlusOutlined,
    ReloadOutlined,
  } from '@ant-design/icons-vue';
  import {
    api,
    type GameBanEntry,
    type GameBanKind,
    type GameBanListResponse,
  } from '../services/api';

  const loading = ref(false);
  const adding = ref(false);
  const deletingKey = ref('');
  const loadError = ref('');
  const state = ref<GameBanListResponse | null>(null);

  const addOpen = ref(false);
  const addKind = ref<GameBanKind>('steam_id');
  const addValue = ref('');
  const durationMode = ref<'permanent' | 'timed'>('permanent');
  const durationMinutes = ref<number | undefined>(60);

  const canSubmit = computed(() => {
    if (!addValue.value.trim()) return false;
    return (
      durationMode.value === 'permanent' ||
      (Number.isInteger(durationMinutes.value) && Number(durationMinutes.value) > 0)
    );
  });

  const applyState = (value: GameBanListResponse) => {
    state.value = {
      ...value,
      steam_bans: value.steam_bans || [],
      ip_bans: value.ip_bans || [],
      warnings: value.warnings || [],
    };
  };

  const loadBans = async () => {
    loading.value = true;
    loadError.value = '';
    try {
      applyState(await api.getGameBans());
    } catch (error) {
      loadError.value = error instanceof Error ? error.message : String(error);
      if (state.value) {
        message.error(`刷新游戏黑名单失败：${loadError.value}`);
      }
    } finally {
      loading.value = false;
    }
  };

  const openAddModal = () => {
    addKind.value = 'steam_id';
    addValue.value = '';
    durationMode.value = 'permanent';
    durationMinutes.value = 60;
    addOpen.value = true;
  };

  const addBan = async () => {
    if (!canSubmit.value) return;
    adding.value = true;
    try {
      const result = await api.addGameBan({
        kind: addKind.value,
        value: addValue.value.trim(),
        duration_minutes:
          durationMode.value === 'permanent' ? 0 : Number(durationMinutes.value),
      });
      applyState(result);
      addOpen.value = false;
      message.success('封禁已添加');
    } catch (error) {
      message.error(error instanceof Error ? error.message : String(error));
    } finally {
      adding.value = false;
    }
  };

  const removeBan = async (entry: GameBanEntry) => {
    const key = `${entry.kind}:${entry.value}`;
    deletingKey.value = key;
    try {
      applyState(await api.removeGameBan({ kind: entry.kind, value: entry.value }));
      message.success('封禁已删除');
    } catch (error) {
      message.error(error instanceof Error ? error.message : String(error));
    } finally {
      deletingKey.value = '';
    }
  };

  const confirmRemove = (entry: GameBanEntry) => {
    Modal.confirm({
      title: '删除这条封禁？',
      content: entry.value,
      okText: '删除',
      cancelText: '取消',
      okType: 'danger',
      onOk: () => removeBan(entry),
    });
  };

  const formatDuration = (entry: GameBanEntry) => {
    if (entry.permanent) return '永久';
    if (entry.remaining_minutes === undefined) return '计时';
    return `${entry.remaining_minutes.toLocaleString('zh-CN', {
      maximumFractionDigits: 3,
    })} 分钟`;
  };

  const isDeleting = (entry: GameBanEntry) =>
    deletingKey.value === `${entry.kind}:${entry.value}`;

  onMounted(loadBans);
</script>

<template>
  <div class="space-y-5 pt-1">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <p class="text-sm text-slate-500 dark:text-slate-400">
        计时封禁仅在当前游戏进程中生效，游戏服务端重启后失效。
      </p>
      <div class="flex flex-wrap gap-2 sm:justify-end">
        <a-button
          class="!inline-flex !items-center !justify-center"
          :loading="loading"
          @click="loadBans"
        >
          <template #icon><ReloadOutlined /></template>
          刷新
        </a-button>
        <a-button
          type="primary"
          class="!inline-flex !items-center !justify-center"
          :disabled="!state"
          @click="openAddModal"
        >
          <template #icon><PlusOutlined /></template>
          添加封禁
        </a-button>
      </div>
    </div>

    <section
      v-if="loadError && !state"
      class="rounded-xl border border-slate-200 px-4 py-10 text-center dark:border-slate-700"
    >
      <p class="font-medium text-slate-900 dark:text-slate-100">无法读取游戏黑名单</p>
      <p class="mt-2 break-words text-sm text-slate-500 dark:text-slate-400">{{ loadError }}</p>
      <a-button class="mt-4" :loading="loading" @click="loadBans">重试</a-button>
    </section>

    <a-spin v-else :spinning="loading">
      <div v-if="state" class="space-y-5">
        <div
          v-if="state.warnings.length"
          class="flex gap-3 rounded-xl border border-amber-300/70 bg-amber-50/60 px-4 py-3 text-sm text-amber-900 dark:border-amber-700/70 dark:bg-amber-950/20 dark:text-amber-200"
        >
          <ExclamationCircleOutlined class="mt-0.5 shrink-0" />
          <ul class="space-y-1">
            <li v-for="warning in state.warnings" :key="warning">{{ warning }}</li>
          </ul>
        </div>

        <section class="overflow-hidden rounded-xl border border-slate-200 dark:border-slate-700">
          <header
            class="border-b border-slate-200 bg-slate-50/80 px-4 py-4 dark:border-slate-700 dark:bg-slate-800/50"
          >
            <h3 class="font-semibold text-slate-900 dark:text-slate-100">SteamID 封禁</h3>
          </header>

          <div
            v-if="state.steam_bans.length === 0"
            class="px-4 py-10 text-center text-sm text-slate-500 dark:text-slate-400"
          >
            暂无封禁
          </div>

          <div v-else class="hidden overflow-x-auto md:block">
            <table class="w-full table-fixed text-left text-sm">
              <thead class="bg-slate-50/70 text-slate-500 dark:bg-slate-800/30 dark:text-slate-400">
                <tr>
                  <th class="w-[65%] px-4 py-3 font-medium">SteamID</th>
                  <th class="w-[25%] px-4 py-3 font-medium">时长</th>
                  <th class="w-[10%] px-4 py-3 text-right font-medium">操作</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-200 dark:divide-slate-700">
                <tr v-for="entry in state.steam_bans" :key="entry.value">
                  <td class="px-4 py-3">
                    <div class="break-all font-mono text-slate-900 dark:text-slate-100">
                      {{ entry.value }}
                    </div>
                    <div
                      v-if="entry.steam_id64"
                      class="mt-1 break-all font-mono text-xs text-slate-500 dark:text-slate-400"
                    >
                      {{ entry.steam_id64 }}
                    </div>
                  </td>
                  <td class="px-4 py-3 text-slate-700 dark:text-slate-300">
                    {{ formatDuration(entry) }}
                  </td>
                  <td class="px-4 py-3 text-right">
                    <a-button
                      danger
                      type="text"
                      class="!inline-flex !items-center !justify-center"
                      :loading="isDeleting(entry)"
                      aria-label="删除 SteamID 封禁"
                      @click="confirmRemove(entry)"
                    >
                      <template #icon><DeleteOutlined /></template>
                    </a-button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="divide-y divide-slate-200 md:hidden dark:divide-slate-700">
            <article v-for="entry in state.steam_bans" :key="entry.value" class="space-y-3 p-4">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="break-all font-mono text-sm text-slate-900 dark:text-slate-100">
                    {{ entry.value }}
                  </div>
                  <div
                    v-if="entry.steam_id64"
                    class="mt-1 break-all font-mono text-xs text-slate-500 dark:text-slate-400"
                  >
                    {{ entry.steam_id64 }}
                  </div>
                </div>
                <a-button
                  danger
                  type="text"
                  class="!inline-flex shrink-0 !items-center !justify-center"
                  :loading="isDeleting(entry)"
                  aria-label="删除 SteamID 封禁"
                  @click="confirmRemove(entry)"
                >
                  <template #icon><DeleteOutlined /></template>
                </a-button>
              </div>
              <div class="text-sm text-slate-500 dark:text-slate-400">
                时长：<span class="text-slate-700 dark:text-slate-300">{{ formatDuration(entry) }}</span>
              </div>
            </article>
          </div>
        </section>

        <section class="overflow-hidden rounded-xl border border-slate-200 dark:border-slate-700">
          <header
            class="border-b border-slate-200 bg-slate-50/80 px-4 py-4 dark:border-slate-700 dark:bg-slate-800/50"
          >
            <h3 class="font-semibold text-slate-900 dark:text-slate-100">IP 封禁</h3>
          </header>

          <div
            v-if="state.ip_bans.length === 0"
            class="px-4 py-10 text-center text-sm text-slate-500 dark:text-slate-400"
          >
            暂无记录
          </div>

          <div v-else class="hidden overflow-x-auto md:block">
            <table class="w-full table-fixed text-left text-sm">
              <thead class="bg-slate-50/70 text-slate-500 dark:bg-slate-800/30 dark:text-slate-400">
                <tr>
                  <th class="w-[65%] px-4 py-3 font-medium">IPv4</th>
                  <th class="w-[25%] px-4 py-3 font-medium">时长</th>
                  <th class="w-[10%] px-4 py-3 text-right font-medium">操作</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-200 dark:divide-slate-700">
                <tr v-for="entry in state.ip_bans" :key="entry.value">
                  <td class="break-all px-4 py-3 font-mono text-slate-900 dark:text-slate-100">
                    {{ entry.value }}
                  </td>
                  <td class="px-4 py-3 text-slate-700 dark:text-slate-300">
                    {{ formatDuration(entry) }}
                  </td>
                  <td class="px-4 py-3 text-right">
                    <a-button
                      danger
                      type="text"
                      class="!inline-flex !items-center !justify-center"
                      :loading="isDeleting(entry)"
                      aria-label="删除 IP 封禁"
                      @click="confirmRemove(entry)"
                    >
                      <template #icon><DeleteOutlined /></template>
                    </a-button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="divide-y divide-slate-200 md:hidden dark:divide-slate-700">
            <article v-for="entry in state.ip_bans" :key="entry.value" class="space-y-3 p-4">
              <div class="flex items-start justify-between gap-3">
                <div class="break-all font-mono text-sm text-slate-900 dark:text-slate-100">
                  {{ entry.value }}
                </div>
                <a-button
                  danger
                  type="text"
                  class="!inline-flex shrink-0 !items-center !justify-center"
                  :loading="isDeleting(entry)"
                  aria-label="删除 IP 封禁"
                  @click="confirmRemove(entry)"
                >
                  <template #icon><DeleteOutlined /></template>
                </a-button>
              </div>
              <div class="text-sm text-slate-500 dark:text-slate-400">
                时长：<span class="text-slate-700 dark:text-slate-300">{{ formatDuration(entry) }}</span>
              </div>
            </article>
          </div>
        </section>
      </div>
    </a-spin>

    <a-modal
      v-model:open="addOpen"
      title="添加游戏封禁"
      ok-text="添加"
      cancel-text="取消"
      :width="520"
      :confirm-loading="adding"
      :ok-button-props="{ disabled: !canSubmit }"
      wrap-class-name="game-ban-modal-wrap"
      @ok="addBan"
    >
      <a-form layout="vertical" class="pt-2">
        <a-form-item label="封禁类型" required>
          <a-select v-model:value="addKind">
            <a-select-option value="steam_id">SteamID</a-select-option>
            <a-select-option value="ip">IPv4</a-select-option>
          </a-select>
        </a-form-item>

        <a-form-item :label="addKind === 'steam_id' ? 'SteamID' : 'IPv4'" required>
          <a-input
            v-model:value="addValue"
            class="font-mono"
            :placeholder="
              addKind === 'steam_id'
                ? 'Steam2、Steam3 或 Steam64'
                : '例如：203.0.113.10'
            "
            @press-enter="addBan"
          />
          <p v-if="addKind === 'steam_id'" class="mt-2 text-xs text-slate-500 dark:text-slate-400">
            保存时统一转换为 Steam2；在线玩家会被立即踢出。
          </p>
        </a-form-item>

        <a-form-item label="封禁时长" required>
          <a-radio-group v-model:value="durationMode">
            <a-radio value="permanent">永久</a-radio>
            <a-radio value="timed">计时</a-radio>
          </a-radio-group>
        </a-form-item>

        <a-form-item v-if="durationMode === 'timed'" label="分钟数" required>
          <a-input-number
            v-model:value="durationMinutes"
            class="!w-full"
            :min="1"
            :max="2147483647"
            :precision="0"
          />
          <p class="mt-2 text-xs text-slate-500 dark:text-slate-400">
            计时封禁不会写入永久列表，游戏服务端重启后失效。
          </p>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<style scoped>
  :global(.game-ban-modal-wrap .ant-modal) {
    max-width: calc(100vw - 24px);
  }

  :global(.game-ban-modal-wrap .ant-modal-footer) {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
  }
</style>
