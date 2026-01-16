import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import { api } from '../services/api';
import { message } from 'ant-design-vue';

export const useMonitorStore = defineStore('monitor', () => {
  const isMonitoring = ref(false);
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
    } catch (error) {
      console.error(error);
      message.error('获取监控数据失败');
      stopMonitor();
    }
  };

  const startMonitor = () => {
    if (isMonitoring.value) return;
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

  return {
    isMonitoring,
    timestamps,
    cpuData,
    cpuMaxCoreData,
    memUsedData,
    memTotal,
    swapUsedData,
    swapTotal,
    netUpData,
    netDownData,
    diskTotal,
    diskUsed,
    diskPercent,
    startMonitor,
    stopMonitor,
    toggleMonitor,
  };
});
