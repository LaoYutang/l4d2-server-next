<script setup lang="ts">
  import { CloudUploadOutlined, LogoutOutlined } from '@ant-design/icons-vue';
  import MapHotReloadButton from '../components/MapHotReloadButton.vue';
  import MapUploadPanel from '../components/MapUploadPanel.vue';
  import { useAuthStore } from '../stores/auth';

  const authStore = useAuthStore();

  const logout = () => {
    authStore.logout();
  };
</script>

<template>
  <div class="min-h-screen bg-gray-50 px-4 py-6 transition-colors dark:bg-gray-950 sm:px-6 lg:px-8">
    <div class="mx-auto flex w-full max-w-4xl flex-col gap-6">
      <header
        class="flex flex-col gap-4 rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900 sm:flex-row sm:items-center sm:justify-between"
      >
        <div class="flex items-center gap-3">
          <img src="/logo.png" alt="Logo" class="h-12 w-12 rounded-lg object-cover" />
          <div>
            <h1 class="m-0 text-xl font-bold text-gray-900 dark:text-gray-100">地图上传</h1>
            <p class="m-0 mt-1 text-sm text-gray-500 dark:text-gray-400">
              当前授权码仅允许上传地图和执行地图热重载
            </p>
          </div>
        </div>
        <a-button danger class="!flex !items-center !justify-center" @click="logout">
          <template #icon><LogoutOutlined /></template>
          退出登录
        </a-button>
      </header>

      <a-card :bordered="false" class="shadow-sm dark:bg-gray-900">
        <template #title>
          <span class="flex items-center gap-2">
            <CloudUploadOutlined class="text-blue-500" />
            上传地图文件
          </span>
        </template>
        <MapUploadPanel />
      </a-card>

      <a-card :bordered="false" class="shadow-sm dark:bg-gray-900">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div class="font-medium text-gray-900 dark:text-gray-100">地图热重载</div>
            <div class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              上传完成后执行，让游戏服务器重新扫描地图资源。
            </div>
          </div>
          <MapHotReloadButton />
        </div>
      </a-card>
    </div>
  </div>
</template>
