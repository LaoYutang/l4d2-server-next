<script setup lang="ts">
  import { ref, onMounted, onUnmounted, h, computed } from 'vue';
  import { api } from '../services/api';
  import { useAuthStore } from '../stores/auth';
  import { parseStatus, type ParsedServerStatus } from '../utils/statusParser';
  import {
    ReloadOutlined,
    SyncOutlined,
    ThunderboltOutlined,
    GlobalOutlined,
    EnvironmentOutlined,
    DashboardOutlined,
    ExclamationCircleOutlined,
    UserOutlined,
    WifiOutlined,
    ClockCircleOutlined,
    CopyOutlined,
    StopOutlined,
    PlusOutlined,
    SettingOutlined,
  } from '@ant-design/icons-vue';
  import { Modal, message } from 'ant-design-vue';
  import { copyToClipboard as copyText } from '../utils/clipboard';
  import MapSelectorModal from '../components/MapSelectorModal.vue';
  import DifficultyModal from '../components/DifficultyModal.vue';
  import GameModeModal from '../components/GameModeModal.vue';

  // Custom Kick Icon (Kicking Person)
  const KickIcon = {
    render: () =>
      h(
        'svg',
        {
          viewBox: '0 0 24 24',
          width: '1em',
          height: '1em',
          fill: 'currentColor',
          'aria-hidden': 'true',
        },
        [
          h('path', {
            d: 'M13.5 5.5c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zM9.8 8.9L7 23h2.1l1.8-8 2.1 2v6h2v-7.5l-2.1-2 .6-3C14.8 12 16.8 13 19 13v-2c-1.9 0-3.5-1-4.3-2.4l-1-1.6c-.4-.6-1-1-1.7-1-.3 0-.5.1-.8.1L6 8.3V13h2V9.6l1.8-.7',
          }),
        ]
      ),
  };

  const authStore = useAuthStore();
  const isAdmin = computed(() => authStore.isAdmin);

  const status = ref<ParsedServerStatus | null>(null);
  const loading = ref(false);
  const autoRefresh = ref(false);
  let refreshInterval: number | null = null;

  const showMapModal = ref(false);
  const showDifficultyModal = ref(false);
  const showGameModeModal = ref(false);
  const showMaxPlayersModal = ref(false);
  const tempMaxPlayers = ref<number>();
  const savingMaxPlayers = ref(false);

  const loadStatus = async () => {
    loading.value = true;
    try {
      const rawStatus = await api.getStatus();
      status.value = parseStatus(rawStatus);

      // 更新页面标题
      if (status.value?.Hostname?.value) {
        document.title = status.value.Hostname.value;
      }

      // 如果有地图信息，尝试获取地图真实名称
      if (status.value?.Map?.value) {
        const mapCode = status.value.Map.value;
        api.fetchMapName(mapCode).then((mapName) => {
          if (status.value?.Map && status.value.Map.value === mapCode) {
            status.value.Map.value = mapName;
          }
        });
      }
    } catch (e: any) {
      message.error(e.message || '获取状态失败');
      if (autoRefresh.value) {
        toggleAutoRefresh();
      }
    } finally {
      loading.value = false;
    }
  };

  const toggleAutoRefresh = () => {
    autoRefresh.value = !autoRefresh.value;
    if (autoRefresh.value) {
      refreshInterval = setInterval(loadStatus, 5000);
      message.success('已开启自动刷新');
    } else if (refreshInterval) {
      clearInterval(refreshInterval);
      refreshInterval = null;
      message.info('已关闭自动刷新');
    }
  };

  const restartServer = async () => {
    Modal.confirm({
      title: '确定要重启服务器吗？',
      icon: () => h(ExclamationCircleOutlined),
      content: '所有玩家将断开连接。',
      onOk: async () => {
        try {
          if (autoRefresh.value) {
            toggleAutoRefresh();
          }
          await api.restartServer();
          message.success('服务器重启指令已发送，请稍后手动刷新查看状态');
        } catch (e: any) {
          message.error('重启失败: ' + e.message);
        }
      },
    });
  };

  const kickUser = async (userName: string, userId: string | number) => {
    Modal.confirm({
      title: `确定要踢出玩家 ${userName} 吗？`,
      icon: () => h(ExclamationCircleOutlined),
      content: '该玩家将被断开连接。',
      onOk: async () => {
        try {
          await api.kickUser(userName, String(userId));
          message.success(`玩家 ${userName} 已被踢出`);
          loadStatus();
        } catch (e: any) {
          message.error('踢出失败: ' + e.message);
        }
      },
    });
  };

  const banUser = async (userName: string, steamId: string) => {
    Modal.confirm({
      title: `确定要封禁玩家 ${userName} 吗？`,
      icon: () => h(StopOutlined, { style: { color: 'red' } }),
      content: '该玩家将被永久封禁并踢出服务器 (banid 0)。',
      okText: '确认封禁',
      okType: 'danger',
      onOk: async () => {
        try {
          await api.banUser(steamId, true);
          message.success(`玩家 ${userName} 已被永久封禁`);
          loadStatus();
        } catch (e: any) {
          message.error('封禁失败: ' + e.message);
        }
      },
    });
  };

  const addAdmin = async (userName: string, steamId: string) => {
    Modal.confirm({
      title: `确定要将 ${userName} 设为管理员吗？`,
      content: `SteamID: ${steamId}`,
      onOk: async () => {
        try {
          const result = await api.addAdmin(steamId, userName);
          if (result.reload) {
            message.success(`已添加 ${userName} 为管理员`);
          } else {
            message.warning(`已添加 ${userName} 为管理员，但游戏内权限刷新失败，请检查服务器状态或手动执行 sm_reloadadmins`);
          }
        } catch (e: any) {
          message.error('添加失败: ' + e.message);
        }
      },
    });
  };

  const getUserPlaytime = async (userName: string, steamId: string) => {
    if (!steamId) {
      message.warning('无法获取该玩家的 SteamID');
      return;
    }
    try {
      const data = await api.getUserPlaytime(steamId);
      const totalHours = data.total_playtime ? Math.round(data.total_playtime * 10) / 10 : 0;
      const realHours = data.real_playtime ? Math.round(data.real_playtime * 10) / 10 : 0;

      const totalTimeText = totalHours > 0 ? `${totalHours} 小时` : '未公开';
      const realTimeText = realHours > 0 ? `${realHours} 小时` : '未公开';

      message.info(`${userName} 的游戏时长: 总时长 ${totalTimeText} / 实战时长 ${realTimeText}`);
    } catch (e: any) {
      message.error('获取时长失败: ' + (e.message || '未知错误'));
    }
  };

  const copyToClipboard = async (text: string) => {
    const success = await copyText(text);
    if (success) {
      message.success('已复制 SteamID');
    } else {
      message.error('复制失败，请手动复制');
    }
  };

  const openIpQuery = (ip: string) => {
    if (!ip) return;
    const cleanIp = ip.split(':')[0];
    window.open(`https://www.ip138.com/iplookup.php?ip=${cleanIp}`, '_blank');
  };

  const openMaxPlayersModal = () => {
    tempMaxPlayers.value = undefined;
    showMaxPlayersModal.value = true;
  };

  const saveMaxPlayers = async () => {
    if (!tempMaxPlayers.value || tempMaxPlayers.value < 4 || tempMaxPlayers.value > 30) {
      message.warning('人数必须在 4-30 之间');
      return;
    }

    try {
      savingMaxPlayers.value = true;
      await api.setMaxPlayers(tempMaxPlayers.value);
      message.success('最大人数设置成功');
      showMaxPlayersModal.value = false;
      loadStatus();
    } catch (e: any) {
      message.error('设置失败: ' + e.message);
    } finally {
      savingMaxPlayers.value = false;
    }
  };

  onMounted(() => {
    loadStatus();
  });

  onUnmounted(() => {
    if (refreshInterval) clearInterval(refreshInterval);
  });
</script>

<template>
  <div class="space-y-8 p-4 md:p-6">
    <!-- Header & Status -->
    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-6">
      <div>
        <h2
          class="text-3xl font-extrabold text-gray-900 dark:text-gray-100 flex items-center gap-3"
        >
          <span
            class="text-blue-600 bg-blue-50 dark:bg-blue-900/30 p-2 rounded-xl flex items-center justify-center"
            ><DashboardOutlined class="text-xl"
          /></span>
          服务器状态
        </h2>
        <p class="text-gray-500 dark:text-gray-400 mt-2 text-base ml-1">
          实时监控与管理您的 L4D2 服务器状态
        </p>
      </div>
      <div class="flex gap-4">
        <a-button
          @click="loadStatus"
          :loading="loading"
          size="large"
          class="!rounded-lg !flex !items-center !justify-center"
        >
          <template #icon><ReloadOutlined /></template>
          刷新
        </a-button>
        <a-button
          :type="autoRefresh ? 'primary' : 'default'"
          @click="toggleAutoRefresh"
          size="large"
          class="!rounded-lg !flex !items-center !justify-center"
        >
          <template #icon><SyncOutlined :spin="autoRefresh" /></template>
          {{ autoRefresh ? '自动刷新中' : '自动刷新' }}
        </a-button>
      </div>
    </div>

    <!-- Status Stats - Compact Grid -->
    <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
      <div>
        <a-tooltip :title="status?.Hostname?.value || 'Unknown'">
          <a-card
            hoverable
            class="!cursor-default shadow-sm rounded-xl border-t-4 border-t-indigo-500 transition-all duration-300 hover:-translate-y-1 dark:bg-gray-800 dark:border-gray-700"
            :bodyStyle="{ padding: '16px' }"
          >
            <div class="flex flex-col w-full">
              <div class="text-gray-500 dark:text-gray-400 text-sm mb-1">服务器名称</div>
              <div class="flex items-center text-lg font-bold text-gray-900 dark:text-gray-100">
                <GlobalOutlined class="text-indigo-400 mr-2 shrink-0" />
                <div class="truncate min-w-0 flex-1">
                  {{ status?.Hostname?.value || 'Unknown' }}
                </div>
              </div>
            </div>
          </a-card>
        </a-tooltip>
      </div>

      <div>
        <a-tooltip :title="status?.Map?.value || 'Unknown'">
          <a-card
            hoverable
            class="!cursor-default shadow-sm rounded-xl border-t-4 border-t-pink-500 transition-all duration-300 hover:-translate-y-1 dark:bg-gray-800 dark:border-gray-700"
            :bodyStyle="{ padding: '16px' }"
          >
            <div class="flex flex-col w-full">
              <div class="text-gray-500 dark:text-gray-400 text-sm mb-1">当前地图</div>
              <div class="flex items-center text-lg font-bold text-gray-900 dark:text-gray-100">
                <EnvironmentOutlined class="text-pink-400 mr-2 shrink-0" />
                <div class="truncate min-w-0 flex-1">
                  {{ status?.Map?.value || 'Unknown' }}
                </div>
              </div>
            </div>
          </a-card>
        </a-tooltip>
      </div>

      <!-- Difficulty Card -->
      <a-card
        hoverable
        class="!cursor-default shadow-sm rounded-xl border-t-4 border-t-blue-500 transition-all duration-300 hover:-translate-y-1 dark:bg-gray-800 dark:border-gray-700"
        :bodyStyle="{ padding: '16px', display: 'flex', alignItems: 'center' }"
      >
        <a-statistic
          title="游戏难度"
          :value="status?.Difficulty?.value || 'Unknown'"
          class="w-full"
          :value-style="{
            fontWeight: '700',
            fontSize: '1.125rem',
            lineHeight: '1.5rem',
          }"
        >
          <template #prefix><DashboardOutlined class="text-blue-400 mr-2" /></template>
        </a-statistic>
      </a-card>

      <!-- Game Mode Card -->
      <a-card
        hoverable
        class="!cursor-default shadow-sm rounded-xl border-t-4 border-t-green-500 transition-all duration-300 hover:-translate-y-1 dark:bg-gray-800 dark:border-gray-700"
        :bodyStyle="{ padding: '16px', display: 'flex', alignItems: 'center' }"
      >
        <a-statistic
          title="游戏模式"
          :value="status?.GameMode?.value || 'Unknown'"
          class="w-full"
          :value-style="{
            fontWeight: '700',
            fontSize: '1.125rem',
            lineHeight: '1.5rem',
          }"
        >
          <template #prefix><ThunderboltOutlined class="text-green-400 mr-2" /></template>
        </a-statistic>
      </a-card>
    </div>

    <!-- Player List Section -->
    <div>
      <h3 class="text-xl font-bold mb-4 flex items-center gap-2 text-gray-900 dark:text-gray-100">
        <span
          class="text-indigo-500 bg-indigo-50 dark:bg-indigo-900/30 p-1.5 rounded-lg text-lg flex items-center justify-center"
          ><UserOutlined
        /></span>
        在线玩家列表
        <span class="text-sm font-normal text-gray-500 dark:text-gray-400 ml-2">
          ({{ status?.Players?.value || '0/0' }})
        </span>
        <a-tooltip title="设置临时最大人数">
          <a-button
            type="text"
            size="small"
            class="!text-gray-400 hover:!text-blue-500 !flex !items-center !justify-center"
            @click="openMaxPlayersModal"
          >
            <template #icon><SettingOutlined /></template>
          </a-button>
        </a-tooltip>
      </h3>

      <div v-if="status?.Users?.users?.length" class="flex flex-col gap-3">
        <div
          v-for="user in status.Users.users"
          :key="user.id"
          class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700 p-3 hover:shadow-md transition-shadow duration-300 flex flex-col md:flex-row items-stretch md:items-center gap-3 md:gap-4 leading-normal"
        >
          <!-- User Info (Left) -->
          <div class="flex justify-between items-center w-full md:w-[200px] md:min-w-[200px] gap-3">
            <div class="flex items-center gap-3 min-w-0">
              <div
                class="w-10 h-10 rounded-full bg-gradient-to-br from-blue-100 to-blue-200 dark:from-blue-900 dark:to-blue-800 text-blue-600 dark:text-blue-200 flex items-center justify-center font-bold text-lg shrink-0"
              >
                {{ user.name ? user.name.charAt(0).toUpperCase() : '?' }}
              </div>
              <div class="min-w-0 flex-1">
                <a-tooltip :title="user.name">
                  <div class="font-bold text-gray-900 dark:text-gray-100 truncate w-[110px]">
                    {{ user.name }}
                  </div>
                </a-tooltip>
                <div class="text-xs text-gray-500 dark:text-gray-400">ID: #{{ user.id }}</div>
              </div>
            </div>

            <!-- Mobile Actions -->
            <div class="flex md:hidden gap-2 shrink-0">
              <a-button
                v-if="isAdmin"
                type="primary"
                ghost
                shape="circle"
                size="small"
                class="!flex !items-center !justify-center"
                :disabled="!user.steamid"
                @click="addAdmin(user.name, user.steamid)"
              >
                <PlusOutlined />
              </a-button>
              <a-button
                type="primary"
                ghost
                shape="circle"
                size="small"
                class="!flex !items-center !justify-center"
                :disabled="!user.steamid"
                @click="getUserPlaytime(user.name, user.steamid)"
              >
                <ClockCircleOutlined />
              </a-button>
              <a-button
                type="primary"
                danger
                ghost
                shape="circle"
                size="small"
                class="!flex !items-center !justify-center"
                @click="kickUser(user.name, user.id)"
              >
                <KickIcon />
              </a-button>
              <a-button
                type="primary"
                danger
                ghost
                shape="circle"
                size="small"
                class="!flex !items-center !justify-center"
                :disabled="!user.steamid"
                @click="banUser(user.name, user.steamid)"
              >
                <StopOutlined />
              </a-button>
            </div>
          </div>

          <!-- Details (Middle) -->
          <div
            class="flex-1 w-full md:w-auto flex flex-wrap items-center gap-x-6 gap-y-2 text-xs text-gray-600 dark:text-gray-400 bg-gray-50 dark:bg-transparent md:bg-transparent p-2 md:p-0 rounded-lg md:rounded-none"
          >
            <!-- ID & Location Stack -->
            <div
              class="flex flex-col justify-center gap-1 w-full md:w-[240px] shrink-0 h-full pt-1"
            >
              <!-- Location (Primary) -->
              <div>
                <a-tooltip :title="user.location || (user.ip ? user.ip.split(':')[0] : '未知')">
                  <div
                    v-if="user.location || user.ip"
                    class="flex items-center gap-2 group cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-700/50 rounded px-1 -ml-1 border border-transparent hover:border-gray-200 dark:hover:border-gray-600 transition-all box-border h-[22px]"
                    @click="openIpQuery(user.ip)"
                  >
                    <div class="flex justify-end w-12 shrink-0">
                      <span class="text-xs font-bold text-gray-500 dark:text-gray-400">ASN</span>
                    </div>
                    <span
                      class="truncate text-xs font-medium text-gray-900 dark:text-gray-100 group-hover:text-blue-600 dark:group-hover:text-blue-400 transition-colors"
                      >{{ user.location || (user.ip ? user.ip.split(':')[0] : '未知') }}</span
                    >
                  </div>
                </a-tooltip>
              </div>

              <!-- SteamID (Secondary) -->
              <div
                v-if="user.steamid"
                class="flex items-center gap-2 group cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-700/50 rounded px-1 -ml-1 border border-transparent hover:border-gray-200 dark:hover:border-gray-600 transition-all box-border h-[22px]"
                @click="copyToClipboard(user.steamid)"
                title="点击复制 SteamID"
              >
                <div class="flex justify-end w-12 shrink-0">
                  <span class="text-[10px] text-gray-400 dark:text-gray-500 uppercase tracking-wide"
                    >SteamID</span
                  >
                </div>
                <span
                  class="font-mono text-xs text-gray-500 dark:text-gray-400 truncate min-w-0 group-hover:text-blue-600 dark:group-hover:text-blue-400 transition-colors"
                  >{{ user.steamid }}</span
                >
                <CopyOutlined
                  class="text-[10px] text-gray-400 opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
                />
              </div>
            </div>

            <!-- Network Stats Group -->
            <div class="flex items-center gap-4 whitespace-nowrap">
              <div class="flex items-center gap-1.5 min-w-[60px]" title="Latency">
                <WifiOutlined class="text-gray-400 dark:text-gray-500" />
                <span>{{ user.delay || 0 }}ms</span>
              </div>
              <div class="flex items-center gap-1.5 min-w-[70px]" title="Packet Loss">
                <span
                  class="w-1.5 h-1.5 rounded-full"
                  :class="(user.loss || 0) > 0 ? 'bg-red-400' : 'bg-green-400'"
                ></span>
                <span>{{ user.loss || 0 }}% Loss</span>
              </div>
              <div
                v-if="user.duration"
                class="flex items-center gap-1.5 min-w-[60px]"
                title="Duration"
              >
                <ClockCircleOutlined class="text-gray-400 dark:text-gray-500" />
                <span>{{ user.duration }}</span>
              </div>
            </div>
          </div>

          <!-- Actions (Desktop Right) -->
          <div class="hidden md:flex gap-2 shrink-0 w-full md:w-auto justify-end">
            <div v-if="isAdmin">
              <a-tooltip title="设为管理员">
                <a-button
                  type="primary"
                  ghost
                  shape="circle"
                  class="!flex !items-center !justify-center"
                  :disabled="!user.steamid"
                  @click="addAdmin(user.name, user.steamid)"
                >
                  <PlusOutlined />
                </a-button>
              </a-tooltip>
            </div>
            <div>
              <a-tooltip title="获取游戏时长">
                <a-button
                  type="primary"
                  ghost
                  shape="circle"
                  class="!flex !items-center !justify-center"
                  :disabled="!user.steamid"
                  @click="getUserPlaytime(user.name, user.steamid)"
                >
                  <ClockCircleOutlined />
                </a-button>
              </a-tooltip>
            </div>
            <div>
              <a-tooltip title="踢出玩家">
                <a-button
                  type="primary"
                  danger
                  ghost
                  shape="circle"
                  class="!flex !items-center !justify-center"
                  @click="kickUser(user.name, user.id)"
                >
                  <KickIcon />
                </a-button>
              </a-tooltip>
            </div>
            <div>
              <a-tooltip title="永久封禁">
                <a-button
                  type="primary"
                  danger
                  ghost
                  shape="circle"
                  class="!flex !items-center !justify-center"
                  :disabled="!user.steamid"
                  @click="banUser(user.name, user.steamid)"
                >
                  <StopOutlined />
                </a-button>
              </a-tooltip>
            </div>
          </div>
        </div>
      </div>

      <div
        v-else
        class="text-center py-12 bg-gray-50 dark:bg-gray-800 rounded-xl border border-dashed border-gray-200 dark:border-gray-700"
      >
        <UserOutlined class="text-4xl text-gray-300 dark:text-gray-600 mb-3" />
        <p class="text-gray-500 dark:text-gray-400">暂无在线玩家</p>
      </div>
    </div>

    <!-- Operation Zone -->
    <div class="pt-6 border-t border-gray-100 dark:border-gray-700">
      <h3 class="text-xl font-bold mb-6 flex items-center gap-2 text-gray-900 dark:text-gray-100">
        <span
          class="text-orange-600 bg-orange-50 dark:bg-orange-900/30 p-1.5 rounded-lg text-lg flex items-center justify-center"
          >⚡</span
        >
        操作区
      </h3>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        <a-card
          hoverable
          class="shadow-sm rounded-xl border-none cursor-pointer group bg-gradient-to-br from-red-50 to-white dark:from-red-900/20 dark:to-gray-800 hover:from-red-100 dark:hover:from-red-900/30 transition-all duration-300"
          @click="restartServer"
          :bodyStyle="{ padding: '12px' }"
        >
          <div class="flex flex-col items-center justify-center gap-2 text-center">
            <div
              class="w-10 h-10 flex items-center justify-center bg-white dark:bg-gray-700 rounded-full shadow-sm text-red-500 group-hover:scale-110 transition-transform duration-300"
            >
              <ReloadOutlined class="text-lg" />
            </div>
            <div>
              <div class="font-bold text-gray-800 dark:text-gray-100 text-sm">重启服务器</div>
              <div class="text-[10px] text-gray-500 dark:text-gray-400 mt-0.5">
                断开所有连接并重启
              </div>
            </div>
          </div>
        </a-card>

        <a-card
          hoverable
          class="shadow-sm rounded-xl border-none cursor-pointer group bg-gradient-to-br from-purple-50 to-white dark:from-purple-900/20 dark:to-gray-800 hover:from-purple-100 dark:hover:from-purple-900/30 transition-all duration-300"
          @click="showMapModal = true"
          :bodyStyle="{ padding: '12px' }"
        >
          <div class="flex flex-col items-center justify-center gap-2 text-center">
            <div
              class="w-10 h-10 flex items-center justify-center bg-white dark:bg-gray-700 rounded-full shadow-sm text-purple-500 group-hover:scale-110 transition-transform duration-300"
            >
              <EnvironmentOutlined class="text-lg" />
            </div>
            <div>
              <div class="font-bold text-gray-800 dark:text-gray-100 text-sm">切换地图</div>
              <div class="text-[10px] text-gray-500 dark:text-gray-400 mt-0.5">
                选择官方或第三方地图
              </div>
            </div>
          </div>
        </a-card>

        <a-card
          hoverable
          class="shadow-sm rounded-xl border-none cursor-pointer group bg-gradient-to-br from-blue-50 to-white dark:from-blue-900/20 dark:to-gray-800 hover:from-blue-100 dark:hover:from-blue-900/30 transition-all duration-300"
          @click="showDifficultyModal = true"
          :bodyStyle="{ padding: '12px' }"
        >
          <div class="flex flex-col items-center justify-center gap-2 text-center">
            <div
              class="w-10 h-10 flex items-center justify-center bg-white dark:bg-gray-700 rounded-full shadow-sm text-blue-500 group-hover:scale-110 transition-transform duration-300"
            >
              <DashboardOutlined class="text-lg" />
            </div>
            <div>
              <div class="font-bold text-gray-800 dark:text-gray-100 text-sm">设置难度</div>
              <div class="text-[10px] text-gray-500 dark:text-gray-400 mt-0.5">
                简单/普通/高级/专家
              </div>
            </div>
          </div>
        </a-card>

        <a-card
          hoverable
          class="shadow-sm rounded-xl border-none cursor-pointer group bg-gradient-to-br from-green-50 to-white dark:from-green-900/20 dark:to-gray-800 hover:from-green-100 dark:hover:from-green-900/30 transition-all duration-300"
          @click="showGameModeModal = true"
          :bodyStyle="{ padding: '12px' }"
        >
          <div class="flex flex-col items-center justify-center gap-2 text-center">
            <div
              class="w-10 h-10 flex items-center justify-center bg-white dark:bg-gray-700 rounded-full shadow-sm text-green-500 group-hover:scale-110 transition-transform duration-300"
            >
              <ThunderboltOutlined class="text-lg" />
            </div>
            <div>
              <div class="font-bold text-gray-800 dark:text-gray-100 text-sm">更改模式</div>
              <div class="text-[10px] text-gray-500 dark:text-gray-400 mt-0.5">
                战役/对抗/写实/生存
              </div>
            </div>
          </div>
        </a-card>
      </div>
    </div>

    <!-- Modals -->
    <MapSelectorModal v-model:open="showMapModal" />
    <DifficultyModal v-model:open="showDifficultyModal" />
    <GameModeModal v-model:open="showGameModeModal" @success="loadStatus" />

    <a-modal
      v-model:open="showMaxPlayersModal"
      title="设置临时最大人数"
      :confirm-loading="savingMaxPlayers"
      @ok="saveMaxPlayers"
    >
      <div class="flex flex-col gap-4 py-4">
        <a-alert
          type="warning"
          show-icon
          message="临时设置"
          description="此设置仅临时生效，重启服务器后将失效。若需要持久化人数设置，请启用 sset 插件或添加到服务器配置(server.cfg)中。注意，需要人数破解插件支持!"
        />
        <div class="flex items-center gap-4">
          <span class="whitespace-nowrap">最大玩家数:</span>
          <a-input-number
            v-model:value="tempMaxPlayers"
            :min="4"
            :max="30"
            class="w-full"
            placeholder="4-30"
            addon-after="人"
          />
        </div>
        <div class="text-gray-500 text-xs">
          范围: 4 - 30 人。修改后将执行 sv_maxplayers 和 sv_visiblemaxplayers 指令。
        </div>
      </div>
    </a-modal>
  </div>
</template>

<style scoped>
  @reference "../style.css";

  /* Ensure statistic title and content are visible in dark mode */
  :deep(.ant-statistic-title) {
    @apply dark:text-gray-400;
  }
  :deep(.ant-statistic-content) {
    @apply dark:text-gray-100;
  }

  /* Force dark mode colors for statistics */
  :global(.dark) :deep(.ant-statistic-title) {
    color: #9ca3af !important;
  }
  :global(.dark) :deep(.ant-statistic-content) {
    color: #f3f4f6 !important;
  }
</style>
