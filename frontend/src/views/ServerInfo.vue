<template>
  <div>
    <div class="flex flex-col">
      <div class="flex justify-between items-center mb-6">
        <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">服务器信息编辑</h1>
        <a-button type="primary" size="large" @click="save" :loading="saving">保存修改</a-button>
      </div>

      <div v-if="loading" class="flex justify-center items-center h-64">
        <a-spin size="large" />
      </div>

      <div v-else class="flex flex-col gap-6">
        <!-- Hostname Column -->
        <div class="flex flex-col bg-white dark:bg-gray-800 p-4 rounded-lg shadow shrink-0">
          <h2 class="text-lg font-semibold mb-4 text-gray-800 dark:text-gray-200 shrink-0">
            服务器名
            <span class="text-xs font-normal text-gray-500 ml-2 block sm:inline"
              >l4d2_hostname.txt</span
            >
          </h2>
          <div
            v-if="hostnameError"
            class="p-4 bg-red-50 dark:bg-red-900/20 rounded border border-red-200 dark:border-red-800 text-red-600 dark:text-red-400"
          >
            {{ hostnameError }}
          </div>
          <textarea
            v-else
            v-model="form.hostname"
            rows="1"
            class="w-full p-3 bg-gray-50 dark:bg-gray-900 text-gray-900 dark:text-gray-100 border border-gray-300 dark:border-gray-700 rounded focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none resize-none font-mono text-sm"
          ></textarea>
        </div>

        <!-- Title Column -->
        <div class="flex flex-col bg-white dark:bg-gray-800 p-4 rounded-lg shadow shrink-0">
          <h2 class="text-lg font-semibold mb-4 text-gray-800 dark:text-gray-200 shrink-0">
            服务器信息
            <span class="text-xs font-normal text-gray-500 ml-2 block sm:inline">host.txt</span>
          </h2>
          <textarea
            v-model="form.host"
            rows="3"
            class="w-full p-3 bg-gray-50 dark:bg-gray-900 text-gray-900 dark:text-gray-100 border border-gray-300 dark:border-gray-700 rounded focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none resize-none font-mono text-sm"
          ></textarea>
        </div>

        <!-- Motd Column -->
        <div class="flex flex-col bg-white dark:bg-gray-800 p-4 rounded-lg shadow shrink-0">
          <h2 class="text-lg font-semibold mb-4 text-gray-800 dark:text-gray-200 shrink-0">
            服务器公告
            <span class="text-xs font-normal text-gray-500 ml-2 block sm:inline">motd.txt</span>
          </h2>
          <textarea
            v-model="form.motd"
            rows="15"
            class="w-full p-3 bg-gray-50 dark:bg-gray-900 text-gray-900 dark:text-gray-100 border border-gray-300 dark:border-gray-700 rounded focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none resize-none font-mono text-sm"
          ></textarea>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { ref, onMounted, reactive } from 'vue';
  import { api } from '../services/api';
  import { message } from 'ant-design-vue';

  const loading = ref(true);
  const saving = ref(false);
  const hostnameError = ref('');

  const form = reactive({
    hostname: '',
    host: '',
    motd: '',
  });

  const fetchData = async () => {
    try {
      loading.value = true;
      const data = await api.getServerInfo();
      form.hostname = data.hostname;
      form.host = data.host;
      form.motd = data.motd;
      hostnameError.value = data.hostname_error || '';
    } catch (e: any) {
      message.error('获取服务器信息失败: ' + e.message);
    } finally {
      loading.value = false;
    }
  };

  const save = async () => {
    try {
      saving.value = true;
      await api.updateServerInfo(form);
      message.success('保存成功');
    } catch (e: any) {
      message.error('保存失败: ' + e.message);
    } finally {
      saving.value = false;
    }
  };

  onMounted(() => {
    fetchData();
  });
</script>
