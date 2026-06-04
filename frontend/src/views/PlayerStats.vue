<template>
  <div class="p-4 md:p-6 space-y-6">
    <a-result v-if="!isAdmin" status="403" title="需要管理员权限" sub-title="玩家统计包含玩家标识和连接信息。" />

    <template v-else>
      <div
        class="flex flex-col xl:flex-row xl:items-center justify-between gap-4 bg-white dark:bg-gray-900 p-4 rounded-lg shadow-sm transition-colors duration-300"
      >
        <div>
          <h1 class="text-2xl font-bold text-gray-800 dark:text-gray-100 flex items-center gap-2">
            <TeamOutlined /> 玩家统计
          </h1>
          <p class="text-gray-500 dark:text-gray-400 mt-1">
            每10分钟采集一次玩家在线数据，保留最近30天
          </p>
        </div>

        <div class="flex flex-col sm:flex-row gap-3">
          <a-button
            @click="refresh"
            :loading="loading || configLoading"
            class="!flex !items-center !justify-center"
          >
            <template #icon><SyncOutlined /></template>
            刷新
          </a-button>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div class="bg-white dark:bg-gray-900 p-4 rounded-lg shadow-sm">
          <div class="text-xs text-gray-500 dark:text-gray-400 mb-1">功能状态</div>
          <div class="flex items-center gap-2 text-lg font-bold">
            <CheckCircleOutlined v-if="config?.enabled" class="text-emerald-500" />
            <WarningOutlined v-else class="text-amber-500" />
            <span class="text-gray-800 dark:text-gray-100">
              {{ config?.enabled ? '已启用' : '未启用' }}
            </span>
          </div>
        </div>

        <div class="bg-white dark:bg-gray-900 p-4 rounded-lg shadow-sm">
          <div class="text-xs text-gray-500 dark:text-gray-400 mb-1">最近采集</div>
          <div class="text-lg font-bold text-gray-800 dark:text-gray-100">
            {{ lastSnapshotTime }}
          </div>
        </div>

        <div class="bg-white dark:bg-gray-900 p-4 rounded-lg shadow-sm">
          <div class="text-xs text-gray-500 dark:text-gray-400 mb-1">服务器状态</div>
          <div class="text-lg font-bold" :class="lastSnapshotClass">
            {{ lastSnapshotStatus }}
          </div>
        </div>
      </div>

      <a-alert
        v-if="config && !config.enabled"
        type="info"
        show-icon
        message="玩家在线统计未启用"
        description="可在系统管理中开启该功能。关闭期间不会新增采集数据，已有数据仍按30天滚动保留。"
      />

      <div
        v-if="config?.enabled"
        class="bg-white dark:bg-gray-900 p-4 rounded-lg shadow-sm transition-colors duration-300"
      >
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-4">
          <h3 class="font-bold text-gray-700 dark:text-gray-200 flex items-center gap-2">
            <LineChartOutlined /> {{ chartTitle }}
          </h3>
          <div class="flex min-h-6 items-center justify-end gap-2">
            <span
              v-if="!selectedTrendDay"
              class="text-xs leading-6 text-gray-500 dark:text-gray-400"
            >
              点击柱状图查看小时视图
            </span>
            <a-button
              v-else
              size="small"
              @click="clearTrendDay"
              class="!flex !h-6 !items-center !justify-center"
            >
              <template #icon><ArrowLeftOutlined /></template>
              返回
            </a-button>
          </div>
        </div>
        <div class="relative w-full h-80">
          <a-empty
            v-if="!loading && hourlyData.length === 0"
            class="absolute inset-0 flex flex-col items-center justify-center"
            description="暂无统计数据"
          />
          <div
            ref="chartRef"
            class="w-full h-full"
            :class="{ 'opacity-0': !loading && hourlyData.length === 0 }"
          ></div>
        </div>
      </div>

      <div v-if="config?.enabled" class="bg-white dark:bg-gray-900 p-4 rounded-lg shadow-sm">
        <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-3 mb-4">
          <h3 class="font-bold text-gray-700 dark:text-gray-200 flex items-center gap-2">
            <TeamOutlined /> 玩家列表
          </h3>
          <div class="flex flex-col sm:flex-row gap-2 lg:w-[520px]">
            <a-input
              v-model:value="keyword"
              placeholder="搜索昵称或 SteamID"
              allow-clear
              @pressEnter="searchPlayers"
            />
            <a-button
              type="primary"
              @click="searchPlayers"
              :loading="searchLoading"
              class="!flex !items-center !justify-center"
            >
              <template #icon><SearchOutlined /></template>
              搜索
            </a-button>
          </div>
        </div>

        <a-table
          size="middle"
          :columns="playerColumns"
          :data-source="players"
          :pagination="{ pageSize: 10 }"
          row-key="steam_id"
          :loading="searchLoading"
          :scroll="{ x: 840 }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'rank'">
              <span class="font-mono text-sm text-gray-600 dark:text-gray-300">
                {{ record.rank ? `#${record.rank}` : '-' }}
              </span>
            </template>
            <template v-else-if="column.key === 'player'">
              <div>
                <div class="font-medium text-gray-800 dark:text-gray-100">
                  {{ record.name || 'Unknown' }}
                </div>
                <div class="text-xs text-gray-500 font-mono">{{ record.steam_id }}</div>
              </div>
            </template>
            <template v-else-if="column.key === 'last_seen'">
              {{ formatTime(record.last_seen) }}
            </template>
            <template v-else-if="column.key === 'estimated_minutes'">
              {{ formatMinutes(record.estimated_minutes || 0) }}
            </template>
            <template v-else-if="column.key === 'location'">
              {{ record.location || '-' }}
            </template>
            <template v-else-if="column.key === 'action'">
              <a-button size="small" type="primary" ghost @click="selectPlayer(record)">
                详情
              </a-button>
            </template>
          </template>
        </a-table>
      </div>

      <a-modal
        v-model:open="detailOpen"
        :title="selectedPlayer?.name || '玩家详情'"
        width="min(1040px, calc(100vw - 24px))"
        wrap-class-name="player-stats-modal"
        :footer="null"
        destroy-on-close
      >
        <a-spin :spinning="detailLoading">
          <template v-if="selectedPlayer">
            <div class="space-y-5">
              <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-3">
                <div class="bg-gray-50 dark:bg-gray-900/70 dark:border dark:border-gray-700 p-3 rounded">
                  <div class="text-xs text-gray-500 mb-1">SteamID</div>
                  <div class="font-mono text-sm break-all">{{ selectedPlayer.steam_id }}</div>
                </div>
                <div class="bg-gray-50 dark:bg-gray-900/70 dark:border dark:border-gray-700 p-3 rounded">
                  <div class="text-xs text-gray-500 mb-1">最近昵称</div>
                  <div class="font-medium break-words">{{ selectedPlayer.name || 'Unknown' }}</div>
                </div>
                <div class="bg-gray-50 dark:bg-gray-900/70 dark:border dark:border-gray-700 p-3 rounded">
                  <div class="text-xs text-gray-500 mb-1">地区</div>
                  <div class="font-medium break-words">{{ selectedPlayer.location || '-' }}</div>
                </div>
                <div class="bg-gray-50 dark:bg-gray-900/70 dark:border dark:border-gray-700 p-3 rounded">
                  <div class="text-xs text-gray-500 mb-1">估算在线</div>
                  <div class="font-medium">{{ detailTotalMinutes }}</div>
                </div>
              </div>

              <div v-if="playerAliases.length" class="space-y-2">
                <h4 class="font-bold text-gray-700 dark:text-gray-200">昵称记录</h4>
                <div class="flex flex-wrap gap-2">
                  <a-tag
                    v-for="alias in playerAliases"
                    :key="alias.name"
                    color="blue"
                    class="!m-0 !py-1 max-w-full whitespace-normal break-all leading-relaxed"
                  >
                    {{ alias.name }} · {{ formatMinutes(alias.estimated_minutes) }}
                  </a-tag>
                </div>
              </div>

              <div>
                <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-1 mb-3">
                  <h4 class="font-bold text-gray-700 dark:text-gray-200 flex items-center gap-2">
                    <ClockCircleOutlined /> 在线日历
                  </h4>
                  <span class="text-xs text-gray-500 dark:text-gray-400">高亮日期表示该玩家有在线采样</span>
                </div>

                <div class="space-y-4">
                  <div
                    v-for="month in calendarMonths"
                    :key="month.key"
                    class="w-full border border-gray-100 dark:border-gray-700 rounded-lg p-3 sm:p-4 bg-white dark:bg-gray-900/40"
                  >
                    <div class="font-bold text-gray-700 dark:text-gray-200 mb-3">
                      {{ month.title }}
                    </div>
                    <div class="grid grid-cols-7 gap-1 sm:gap-1.5 text-center text-xs text-gray-400 dark:text-gray-500 mb-2">
                      <div v-for="day in weekDays" :key="day">{{ day }}</div>
                    </div>
                    <div class="grid grid-cols-7 gap-1 sm:gap-1.5">
                      <div
                        v-for="day in month.days"
                        :key="day.key"
                        class="h-11 sm:h-12 rounded-md border text-[11px] sm:text-xs px-1.5 sm:px-2 py-1.5 transition-colors overflow-hidden"
                        :class="calendarDayClass(day)"
                      >
                        <div class="font-medium">{{ day.day }}</div>
                        <div v-if="day.stat" class="mt-0.5 leading-tight truncate">
                          {{ formatCalendarMinutes(day.stat.online_minutes) }}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <a-empty
                  v-if="calendarMonths.length === 0"
                  description="当前时间范围内暂无在线日期"
                />
              </div>
            </div>
          </template>
        </a-spin>
      </a-modal>
    </template>
  </div>
</template>

<script setup lang="ts">
  import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue';
  import { message } from 'ant-design-vue';
  import {
    ArrowLeftOutlined,
    CheckCircleOutlined,
    ClockCircleOutlined,
    LineChartOutlined,
    SearchOutlined,
    SyncOutlined,
    TeamOutlined,
    WarningOutlined,
  } from '@ant-design/icons-vue';
  import * as echarts from 'echarts/core';
  import { BarChart } from 'echarts/charts';
  import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components';
  import { CanvasRenderer } from 'echarts/renderers';
  import {
    api,
    type PlayerStatsAlias,
    type PlayerStatsDay,
    type PlayerStatsHourlyItem,
    type PlayerStatsPlayer,
  } from '../services/api';
  import { useAuthStore } from '../stores/auth';
  import { useThemeStore } from '../stores/theme';

  echarts.use([BarChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer]);

  const authStore = useAuthStore();
  const themeStore = useThemeStore();
  const isAdmin = computed(() => authStore.isAdmin);

  const config = ref<any>(null);
  const configLoading = ref(false);
  const loading = ref(false);
  const searchLoading = ref(false);
  const detailLoading = ref(false);
  const selectedTrendDay = ref<number | null>(null);
  const hourlyData = ref<PlayerStatsHourlyItem[]>([]);
  const keyword = ref('');
  const players = ref<PlayerStatsPlayer[]>([]);
  const selectedPlayer = ref<PlayerStatsPlayer | null>(null);
  const detailOpen = ref(false);
  const playerDays = ref<PlayerStatsDay[]>([]);
  const playerAliases = ref<PlayerStatsAlias[]>([]);
  const chartRef = ref<HTMLDivElement | null>(null);
  let chart: echarts.ECharts | null = null;

  const playerColumns = [
    { title: '排名', key: 'rank', width: 80 },
    { title: '玩家', key: 'player' },
    { title: '地区', key: 'location' },
    { title: '最近出现', key: 'last_seen' },
    { title: '估算在线', key: 'estimated_minutes' },
    { title: '', key: 'action', width: 72 },
  ];
  const weekDays = ['日', '一', '二', '三', '四', '五', '六'];

  const lastSnapshotTime = computed(() => {
    const snapshot = config.value?.last_snapshot;
    if (!snapshot?.timestamp) return '-';
    return formatTime(snapshot.timestamp);
  });

  const lastSnapshotStatus = computed(() => {
    const snapshot = config.value?.last_snapshot;
    if (!snapshot) return '-';
    if (snapshot.collect_ok && snapshot.server_online) {
      return `${snapshot.player_count}/${snapshot.max_players || '-'} 在线`;
    }
    return '服务器离线';
  });

  const lastSnapshotClass = computed(() => {
    const snapshot = config.value?.last_snapshot;
    if (!snapshot) return 'text-gray-800 dark:text-gray-100';
    return snapshot.collect_ok && snapshot.server_online
      ? 'text-emerald-600 dark:text-emerald-400'
      : 'text-amber-600 dark:text-amber-400';
  });

  const detailTotalMinutes = computed(() => {
    const total = playerDays.value.reduce((sum, day) => sum + day.online_minutes, 0);
    return formatMinutes(total);
  });

  const trendBucket = computed<'hour' | 'day'>(() =>
    selectedTrendDay.value ? 'hour' : 'day'
  );

  const chartTitle = computed(() => {
    if (selectedTrendDay.value) return `${formatDayTitle(selectedTrendDay.value)} 24小时在线人数`;
    return '最近30天每日在线人数';
  });

  const chartData = computed<PlayerStatsHourlyItem[]>(() => {
    if (trendBucket.value !== 'hour' || !selectedTrendDay.value) {
      return hourlyData.value;
    }

    const dataByHour = new Map(hourlyData.value.map((item) => [item.timestamp, item]));
    return Array.from({ length: 24 }, (_, hour) => {
      const timestamp = selectedTrendDay.value! + hour * 3600;
      return dataByHour.get(timestamp) || {
        timestamp,
        avg_players: null,
        peak_players: null,
        unique_players: 0,
        offline_samples: 0,
        sample_count: 0,
      };
    });
  });

  const dayStatMap = computed(() => {
    const map = new Map<string, PlayerStatsDay>();
    playerDays.value.forEach((day) => map.set(day.date, day));
    return map;
  });

  const calendarMonths = computed(() => {
    if (playerDays.value.length === 0) return [];

    const dates = playerDays.value.map((day) => new Date(`${day.date}T00:00:00`));
    const minDate = new Date(Math.min(...dates.map((date) => date.getTime())));
    const maxDate = new Date(Math.max(...dates.map((date) => date.getTime())));
    const current = new Date(minDate.getFullYear(), minDate.getMonth(), 1);
    const end = new Date(maxDate.getFullYear(), maxDate.getMonth(), 1);
    const months: Array<{
      key: string;
      title: string;
      days: Array<{
        key: string;
        day: number;
        inMonth: boolean;
        stat?: PlayerStatsDay;
      }>;
    }> = [];

    while (current <= end) {
      const year = current.getFullYear();
      const month = current.getMonth();
      const firstDay = new Date(year, month, 1);
      const gridStart = new Date(year, month, 1 - firstDay.getDay());
      const days = [];

      for (let i = 0; i < 42; i++) {
        const date = new Date(gridStart);
        date.setDate(gridStart.getDate() + i);
        const dateKey = formatDateKey(date);
        days.push({
          key: dateKey,
          day: date.getDate(),
          inMonth: date.getMonth() === month,
          stat: dayStatMap.value.get(dateKey),
        });
      }

      months.push({
        key: `${year}-${month}`,
        title: `${year}年${month + 1}月`,
        days,
      });
      current.setMonth(current.getMonth() + 1);
    }

    return months;
  });

  const getRange = () => {
    if (selectedTrendDay.value) {
      return { start: selectedTrendDay.value, end: selectedTrendDay.value + 86400 - 1 };
    }
    return getBaseRange();
  };

  const getBaseRange = () => {
    const end = Math.floor(Date.now() / 1000);
    return { start: end - 86400 * 30, end };
  };

  const fetchConfig = async () => {
    configLoading.value = true;
    try {
      config.value = await api.getPlayerStatsConfig();
    } catch (e: any) {
      message.error(`获取玩家统计配置失败: ${e.message}`);
    } finally {
      configLoading.value = false;
    }
  };

  const fetchHourly = async () => {
    if (!config.value?.enabled) return;
    loading.value = true;
    try {
      const { start, end } = getRange();
      hourlyData.value = await api.getPlayerStatsHourly(start, end, trendBucket.value);
      await nextTick();
      updateChart();
    } catch (e: any) {
      message.error(`获取玩家统计失败: ${e.message}`);
    } finally {
      loading.value = false;
    }
  };

  const searchPlayers = async () => {
    if (!config.value?.enabled) return;
    searchLoading.value = true;
    try {
      const { start } = getBaseRange();
      players.value = await api.searchPlayerStatsPlayers(keyword.value, start);
    } catch (e: any) {
      message.error(`搜索玩家失败: ${e.message}`);
    } finally {
      searchLoading.value = false;
    }
  };

  const selectPlayer = async (player: PlayerStatsPlayer | Record<string, any>) => {
    const selected = player as PlayerStatsPlayer;
    selectedPlayer.value = selected;
    detailOpen.value = true;
    await fetchPlayerDetail(selected);
  };

  const fetchPlayerDetail = async (player: PlayerStatsPlayer) => {
    detailLoading.value = true;
    try {
      const { start, end } = getBaseRange();
      const detail = await api.getPlayerStatsPlayerDays(player.steam_id, start, end);
      playerDays.value = detail.days || [];
      playerAliases.value = detail.aliases || [];
    } catch (e: any) {
      message.error(`查询玩家在线日期失败: ${e.message}`);
    } finally {
      detailLoading.value = false;
    }
  };

  const refresh = async () => {
    if (!isAdmin.value) return;
    selectedTrendDay.value = null;
    await fetchConfig();
    if (config.value?.enabled) {
      await fetchHourly();
      await searchPlayers();
      if (detailOpen.value && selectedPlayer.value) {
        await fetchPlayerDetail(selectedPlayer.value);
      }
    }
  };

  const formatTime = (timestamp?: number) => {
    if (!timestamp) return '-';
    return new Date(timestamp * 1000).toLocaleString();
  };

  const formatHour = (timestamp: number) => {
    const date = new Date(timestamp * 1000);
    if (trendBucket.value === 'hour') {
      return `${date.getHours()}`;
    }
    return `${date.getMonth() + 1}-${date.getDate()}`;
  };

  const formatDayTitle = (timestamp: number) => {
    const date = new Date(timestamp * 1000);
    return `${date.getFullYear()}-${date.getMonth() + 1}-${date.getDate()}`;
  };

  const formatMinutes = (minutes: number) => {
    if (minutes < 60) return `${minutes} 分钟`;
    const hours = Math.floor(minutes / 60);
    const rest = minutes % 60;
    return rest ? `${hours} 小时 ${rest} 分钟` : `${hours} 小时`;
  };

  const formatCalendarMinutes = (minutes: number) => {
    if (minutes < 60) return `${minutes}分`;
    const hours = Math.floor(minutes / 60);
    const rest = minutes % 60;
    return rest ? `${hours}时${rest}分` : `${hours}时`;
  };

  const initChart = () => {
    if (!chartRef.value) return;
    chart?.dispose();
    chart = echarts.init(chartRef.value, themeStore.isDark ? 'dark' : undefined);
    chart.on('click', handleChartClick);
    updateChart();
  };

  const updateChart = () => {
    if (!chartRef.value) return;
    if (!chart) {
      initChart();
      return;
    }

    const displayData = chartData.value;
    const labels = displayData.map((item) => formatHour(item.timestamp));
    const avgData = displayData.map((item) =>
      item.avg_players === null ? null : Number(item.avg_players.toFixed(2))
    );
    const peakData = displayData.map((item) => item.peak_players);

    chart.setOption({
      backgroundColor: 'transparent',
      color: ['#1677ff', '#fa8c16'],
      tooltip: {
        trigger: 'axis',
        formatter: (params: any) => {
          const list = Array.isArray(params) ? params : [params];
          const index = list[0]?.dataIndex ?? 0;
          const item = displayData[index];
          if (!item) return '';
          const avg = item.sample_count === 0
            ? '暂无采样'
            : item.avg_players === null ? '离线/无在线采样' : item.avg_players;
          const peak = item.peak_players === null ? '-' : item.peak_players;
          return [
            `<strong>${formatHour(item.timestamp)}</strong>`,
            `平均人数: ${avg}`,
            `峰值人数: ${peak}`,
            `独立玩家: ${item.unique_players}`,
            `离线采样: ${item.offline_samples}`,
          ].join('<br/>');
        },
      },
      legend: { top: 0 },
      grid: { top: 48, left: 40, right: 20, bottom: 36, containLabel: true },
      xAxis: {
        type: 'category',
        data: labels,
        axisTick: { alignWithLabel: true },
      },
      yAxis: {
        type: 'value',
        minInterval: 1,
      },
      series: [
        {
          name: '平均人数',
          type: 'bar',
          data: avgData,
          barMaxWidth: 34,
          itemStyle: { borderRadius: [4, 4, 0, 0] },
        },
        {
          name: '峰值人数',
          type: 'bar',
          data: peakData,
          barMaxWidth: 34,
          itemStyle: { borderRadius: [4, 4, 0, 0] },
        },
      ],
    });
  };

  const handleChartClick = (params: any) => {
    if (trendBucket.value !== 'day') return;
    const item = hourlyData.value[params?.dataIndex];
    if (!item) return;
    selectedTrendDay.value = item.timestamp;
    fetchHourly();
  };

  const clearTrendDay = async () => {
    selectedTrendDay.value = null;
    await fetchHourly();
  };

  const handleResize = () => {
    chart?.resize();
  };

  const formatDateKey = (date: Date) => {
    const year = date.getFullYear();
    const month = `${date.getMonth() + 1}`.padStart(2, '0');
    const day = `${date.getDate()}`.padStart(2, '0');
    return `${year}-${month}-${day}`;
  };

  const calendarDayClass = (day: { inMonth: boolean; stat?: PlayerStatsDay }) => {
    if (!day.inMonth) {
      return 'border-transparent text-gray-300 dark:text-gray-700 bg-transparent';
    }
    if (day.stat) {
      return 'border-blue-300 bg-blue-50 text-blue-700 shadow-sm dark:border-blue-500/70 dark:bg-blue-500/15 dark:text-blue-100';
    }
    return 'border-gray-100 bg-gray-50 text-gray-500 dark:border-gray-700 dark:bg-gray-800/60 dark:text-gray-400';
  };

  watch(
    () => themeStore.isDark,
    () => {
      nextTick(initChart);
    }
  );

  onMounted(async () => {
    if (!isAdmin.value) return;
    await fetchConfig();
    await nextTick();
    initChart();
    if (config.value?.enabled) {
      await fetchHourly();
      await searchPlayers();
    }
    window.addEventListener('resize', handleResize);
  });

  onUnmounted(() => {
    window.removeEventListener('resize', handleResize);
    chart?.dispose();
  });
</script>

<style scoped>
  :global(.player-stats-modal .ant-modal) {
    max-width: calc(100vw - 24px);
    padding-bottom: 24px;
  }

  :global(.player-stats-modal .ant-modal-content) {
    border-radius: 8px;
    overflow: hidden;
  }

  :global(.player-stats-modal .ant-modal-body) {
    max-height: calc(100vh - 132px);
    overflow-y: auto;
  }

  :global(.dark .player-stats-modal .ant-modal-content),
  :global(.dark .player-stats-modal .ant-modal-header) {
    background: #111827;
  }

  :global(.dark .player-stats-modal .ant-modal-header) {
    border-bottom: 1px solid #374151;
  }

  :global(.dark .player-stats-modal .ant-modal-title) {
    color: #e5e7eb;
  }

  :global(.dark .player-stats-modal .ant-modal-close) {
    color: #9ca3af;
  }

  :global(.dark .player-stats-modal .ant-modal-close:hover) {
    color: #f3f4f6;
    background: #1f2937;
  }

  @media (max-width: 640px) {
    :global(.player-stats-modal .ant-modal) {
      max-width: calc(100vw - 16px);
      padding-bottom: 12px;
    }

    :global(.player-stats-modal .ant-modal-body) {
      max-height: calc(100vh - 108px);
      padding: 16px;
    }
  }
</style>
