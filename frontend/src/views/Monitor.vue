<template>
  <div class="p-4 md:p-6 space-y-6">
    <!-- Header -->
    <div
      class="flex flex-col md:flex-row justify-between items-center gap-4 bg-white dark:bg-gray-900 p-4 rounded-lg shadow-sm transition-colors duration-300"
    >
      <div>
        <h1 class="text-2xl font-bold text-gray-800 dark:text-gray-100 flex items-center gap-2">
          <LineChartOutlined /> 性能监控
        </h1>
        <p class="text-gray-500 dark:text-gray-400 mt-1">实时监控服务器资源使用情况</p>
      </div>
      <div class="flex gap-3">
        <a-button
          :type="isMonitoring ? 'primary' : 'default'"
          :danger="isMonitoring"
          @click="toggleMonitor"
          :loading="loading"
          class="!flex !items-center !justify-center"
        >
          <template #icon>
            <StopOutlined v-if="isMonitoring" />
            <PlayCircleOutlined v-else />
          </template>
          <span>{{ isMonitoring ? '停止监控' : '开始监控' }}</span>
        </a-button>
      </div>
    </div>

    <!-- Charts Grid -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- CPU -->
      <div
        class="bg-white dark:bg-gray-900 p-4 rounded-lg shadow-sm transition-colors duration-300"
      >
        <div
          class="flex justify-between items-center mb-4 border-b border-gray-100 dark:border-gray-800 pb-2"
        >
          <h3 class="font-bold text-gray-700 dark:text-gray-200">CPU</h3>
          <div class="text-xs text-gray-500 dark:text-gray-400 font-mono">
            <span v-if="cpuData.length > 0">
              Total: {{ cpuData[cpuData.length - 1] }}% | Core Max:
              {{ cpuMaxCoreData[cpuMaxCoreData.length - 1] }}%
            </span>
            <span v-else>Waiting for data...</span>
          </div>
        </div>
        <div class="relative w-full h-64">
          <div
            v-if="!isMonitoring && cpuData.length === 0"
            class="absolute inset-0 flex flex-col items-center justify-center text-gray-300 dark:text-gray-600"
          >
            <LineChartOutlined class="text-6xl mb-2 opacity-50" />
            <span class="text-sm">点击开始监控以查看数据</span>
          </div>
          <div
            ref="cpuChartRef"
            class="w-full h-full"
            :class="{ 'opacity-0': !isMonitoring && cpuData.length === 0 }"
          ></div>
        </div>
      </div>

      <!-- Memory -->
      <div
        class="bg-white dark:bg-gray-900 p-4 rounded-lg shadow-sm transition-colors duration-300"
      >
        <div
          class="flex justify-between items-center mb-4 border-b border-gray-100 dark:border-gray-800 pb-2"
        >
          <h3 class="font-bold text-gray-700 dark:text-gray-200">内存</h3>
          <div class="text-xs text-gray-500 dark:text-gray-400 font-mono flex flex-col items-end">
            <span v-if="memUsedData.length > 0">
              Phy: {{ memUsedData[memUsedData.length - 1] }} GB /
              {{ (memTotal / 1024 / 1024 / 1024).toFixed(1) }} GB
            </span>
            <span v-if="swapUsedData.length > 0">
              Swap: {{ swapUsedData[swapUsedData.length - 1] }} GB /
              {{ (swapTotal / 1024 / 1024 / 1024).toFixed(1) }} GB
            </span>
            <span v-if="memUsedData.length === 0">Waiting for data...</span>
          </div>
        </div>
        <div class="relative w-full h-64">
          <div
            v-if="!isMonitoring && memUsedData.length === 0"
            class="absolute inset-0 flex flex-col items-center justify-center text-gray-300 dark:text-gray-600"
          >
            <AppstoreAddOutlined class="text-6xl mb-2 opacity-50" />
            <span class="text-sm">点击开始监控以查看数据</span>
          </div>
          <div
            ref="memChartRef"
            class="w-full h-full"
            :class="{ 'opacity-0': !isMonitoring && memUsedData.length === 0 }"
          ></div>
        </div>
      </div>

      <!-- Network -->
      <div
        class="bg-white dark:bg-gray-900 p-4 rounded-lg shadow-sm transition-colors duration-300"
      >
        <div
          class="flex justify-between items-center mb-4 border-b border-gray-100 dark:border-gray-800 pb-2"
        >
          <h3 class="font-bold text-gray-700 dark:text-gray-200">网络</h3>
          <div class="text-xs text-gray-500 dark:text-gray-400 font-mono">
            <span v-if="netUpData.length > 0">
              ↑ {{ netUpData[netUpData.length - 1] }} KB/s | ↓
              {{ netDownData[netDownData.length - 1] }} KB/s
            </span>
            <span v-else>Waiting for data...</span>
          </div>
        </div>
        <div class="relative w-full h-64">
          <div
            v-if="!isMonitoring && netUpData.length === 0"
            class="absolute inset-0 flex flex-col items-center justify-center text-gray-300 dark:text-gray-600"
          >
            <GlobalOutlined class="text-6xl mb-2 opacity-50" />
            <span class="text-sm">点击开始监控以查看数据</span>
          </div>
          <div
            ref="netChartRef"
            class="w-full h-full"
            :class="{ 'opacity-0': !isMonitoring && netUpData.length === 0 }"
          ></div>
        </div>
      </div>

      <!-- Disk (Progress Circle) -->
      <div
        class="bg-white dark:bg-gray-900 p-4 rounded-lg shadow-sm transition-colors duration-300 flex flex-col"
      >
        <div
          class="flex justify-between items-center mb-4 border-b border-gray-100 dark:border-gray-800 pb-2"
        >
          <h3 class="font-bold text-gray-700 dark:text-gray-200">硬盘</h3>
          <div class="text-xs text-gray-500 dark:text-gray-400 font-mono">
            <span v-if="diskTotal > 0">
              {{ (diskUsed / 1024 / 1024 / 1024).toFixed(1) }} GB /
              {{ (diskTotal / 1024 / 1024 / 1024).toFixed(1) }} GB
            </span>
            <span v-else>Waiting for data...</span>
          </div>
        </div>
        <div class="relative flex-1 flex flex-col justify-center items-center py-4">
          <div
            v-if="!isMonitoring && diskTotal === 0"
            class="absolute inset-0 flex flex-col items-center justify-center text-gray-300 dark:text-gray-600 bg-white dark:bg-gray-900 z-10"
          >
            <DatabaseOutlined class="text-6xl mb-2 opacity-50" />
            <span class="text-sm">点击开始监控以查看数据</span>
          </div>
          <a-progress
            type="circle"
            :percent="diskPercent"
            :stroke-color="{ '0%': '#108ee9', '100%': '#87d068' }"
            :format="(percent) => `${percent?.toFixed(1)}%`"
            :width="200"
            :stroke-width="10"
          />
          <div class="mt-6 text-center text-gray-500 dark:text-gray-400">
            <p class="text-lg font-medium">总空间使用率</p>
            <p class="text-xs mt-1 text-gray-400">当前程序所在磁盘分区</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { ref, onMounted, onUnmounted, watch, nextTick, computed } from 'vue';
  import {
    LineChartOutlined,
    PlayCircleOutlined,
    StopOutlined,
    AppstoreAddOutlined,
    GlobalOutlined,
    DatabaseOutlined,
  } from '@ant-design/icons-vue';
  import { message } from 'ant-design-vue';

  // ECharts Core
  import * as echarts from 'echarts/core';
  import { LineChart } from 'echarts/charts';
  import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components';
  import { CanvasRenderer } from 'echarts/renderers';

  echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer]);

  import { api } from '../services/api';
  import { useThemeStore } from '../stores/theme';

  const themeStore = useThemeStore();
  const isMonitoring = ref(false);
  const loading = ref(false);
  const timer = ref<any>(null);

  // Data Storage (Max 600)
  const maxPoints = 600;
  const timestamps = ref<string[]>([]);
  const cpuData = ref<number[]>([]);
  const cpuMaxCoreData = ref<number[]>([]);
  const memUsedData = ref<number[]>([]); // GB
  const memTotal = ref(0);
  const swapUsedData = ref<number[]>([]); // GB
  const swapTotal = ref(0);
  const netUpData = ref<number[]>([]); // KB/s
  const netDownData = ref<number[]>([]); // KB/s

  // Disk Data (Current Only)
  const diskTotal = ref(0);
  const diskUsed = ref(0);
  const diskPercent = computed(() => {
    if (diskTotal.value === 0) return 0;
    return (diskUsed.value / diskTotal.value) * 100;
  });

  // Chart Refs
  const cpuChartRef = ref<HTMLElement | null>(null);
  const memChartRef = ref<HTMLElement | null>(null);
  const netChartRef = ref<HTMLElement | null>(null);

  let cpuChart: echarts.ECharts | null = null;
  let memChart: echarts.ECharts | null = null;
  let netChart: echarts.ECharts | null = null;

  const getChartThemeColor = () => {
    return themeStore.isDark ? '#e5e7eb' : '#374151';
  };

  const getSplitLineColor = () => {
    return themeStore.isDark ? '#374151' : '#e5e7eb';
  };

  const initCharts = () => {
    const commonOption = {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'cross' },
      },
      grid: {
        left: '3%',
        right: '4%',
        bottom: '3%',
        top: '10%', // Reduced top padding since title is removed
        containLabel: true,
      },
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: timestamps.value,
        axisLabel: {
          color: getChartThemeColor(),
          rotate: 45,
        },
        axisLine: { lineStyle: { color: getChartThemeColor() } },
      },
      yAxis: {
        type: 'value',
        splitLine: {
          lineStyle: {
            color: getSplitLineColor(),
          },
        },
        axisLabel: { color: getChartThemeColor() },
      },
      legend: {
        show: false, // Hidden legend
      },
    };

    if (cpuChartRef.value) {
      cpuChart = echarts.init(cpuChartRef.value);
      cpuChart.setOption({
        ...commonOption,
        series: [
          {
            name: '总占用',
            type: 'line',
            smooth: true,
            data: cpuData.value,
            showSymbol: false,
            areaStyle: { opacity: 0.3 },
          },
          {
            name: '最高核心',
            type: 'line',
            smooth: true,
            data: cpuMaxCoreData.value,
            showSymbol: false,
          },
        ],
      });
    }

    if (memChartRef.value) {
      memChart = echarts.init(memChartRef.value);
      memChart.setOption({
        ...commonOption,
        yAxis: {
          ...commonOption.yAxis,
          max: memTotal.value > 0 ? (memTotal.value / 1024 / 1024 / 1024).toFixed(1) : undefined,
        },
        series: [
          {
            name: '已用内存',
            type: 'line',
            smooth: true,
            data: memUsedData.value,
            showSymbol: false,
            areaStyle: { opacity: 0.3 },
          },
          {
            name: 'Swap已用',
            type: 'line',
            smooth: true,
            data: swapUsedData.value,
            showSymbol: false,
            lineStyle: { type: 'dashed' },
          },
        ],
      });
    }

    if (netChartRef.value) {
      netChart = echarts.init(netChartRef.value);
      netChart.setOption({
        ...commonOption,
        series: [
          { name: '上行', type: 'line', smooth: true, data: netUpData.value, showSymbol: false },
          { name: '下行', type: 'line', smooth: true, data: netDownData.value, showSymbol: false },
        ],
      });
    }
  };

  const updateCharts = () => {
    const commonUpdate = {
      xAxis: { data: timestamps.value },
    };

    cpuChart?.setOption({
      ...commonUpdate,
      series: [{ data: cpuData.value }, { data: cpuMaxCoreData.value }],
    });

    memChart?.setOption({
      ...commonUpdate,
      yAxis: {
        max: memTotal.value > 0 ? (memTotal.value / 1024 / 1024 / 1024).toFixed(1) : undefined,
      },
      series: [{ data: memUsedData.value }, { data: swapUsedData.value }],
    });

    netChart?.setOption({
      ...commonUpdate,
      series: [{ data: netUpData.value }, { data: netDownData.value }],
    });
  };

  const fetchData = async () => {
    try {
      const response = await api.post('/monitor/status');
      const data = await response.json();

      const now = new Date(data.timestamp * 1000).toLocaleTimeString();

      memTotal.value = data.mem_total;
      swapTotal.value = data.swap_total;
      diskTotal.value = data.disk_total;
      diskUsed.value = data.disk_used;

      timestamps.value.push(now);
      cpuData.value.push(parseFloat(data.cpu_percent.toFixed(1)));
      cpuMaxCoreData.value.push(parseFloat(data.cpu_max_core_percent.toFixed(1)));
      memUsedData.value.push(parseFloat((data.mem_used / 1024 / 1024 / 1024).toFixed(2)));
      swapUsedData.value.push(parseFloat((data.swap_used / 1024 / 1024 / 1024).toFixed(2)));
      netUpData.value.push(parseFloat((data.net_up_speed / 1024).toFixed(1)));
      netDownData.value.push(parseFloat((data.net_down_speed / 1024).toFixed(1)));

      if (timestamps.value.length > maxPoints) {
        timestamps.value.shift();
        cpuData.value.shift();
        cpuMaxCoreData.value.shift();
        memUsedData.value.shift();
        swapUsedData.value.shift();
        netUpData.value.shift();
        netDownData.value.shift();
      }

      updateCharts();
    } catch (error) {
      console.error(error);
      message.error('获取监控数据失败');
      stopMonitor();
    }
  };

  const startMonitor = () => {
    isMonitoring.value = true;
    fetchData();
    timer.value = setInterval(fetchData, 1000);
  };

  const stopMonitor = () => {
    isMonitoring.value = false;
    if (timer.value) {
      clearInterval(timer.value);
      timer.value = null;
    }
  };

  const toggleMonitor = () => {
    if (isMonitoring.value) {
      stopMonitor();
    } else {
      startMonitor();
    }
  };

  watch(
    () => themeStore.isDark,
    () => {
      cpuChart?.dispose();
      memChart?.dispose();
      netChart?.dispose();
      nextTick(() => {
        initCharts();
      });
    }
  );

  onMounted(() => {
    initCharts();
    window.addEventListener('resize', handleResize);
  });

  const handleResize = () => {
    cpuChart?.resize();
    memChart?.resize();
    netChart?.resize();
  };

  onUnmounted(() => {
    stopMonitor();
    window.removeEventListener('resize', handleResize);
    cpuChart?.dispose();
    memChart?.dispose();
    netChart?.dispose();
  });
</script>
