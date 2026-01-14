<script setup lang="ts">
  import { ref, watch, onErrorCaptured } from 'vue';
  import { useAuthStore } from '../stores/auth';
  import { useRouter, useRoute } from 'vue-router';
  import { message } from 'ant-design-vue';
  import {
    DashboardOutlined,
    CodeOutlined,
    SettingOutlined,
    LogoutOutlined,
    MenuOutlined,
    ReadOutlined,
    AppstoreAddOutlined,
  } from '@ant-design/icons-vue';

  const authStore = useAuthStore();
  const router = useRouter();
  const route = useRoute();

  const mobileOpen = ref(false);
  const collapsed = ref(true);
  const selectedKeys = ref<string[]>([]);

  // Sync selected keys with route
  watch(
    () => route.path,
    (path) => {
      selectedKeys.value = [path];
    },
    { immediate: true }
  );

  const handleLogout = () => {
    authStore.logout();
    router.push('/login');
  };

  const handleMenuClick = ({ key }: { key: string | number }) => {
    const keyStr = String(key);
    if (keyStr === 'logout') {
      handleLogout();
    } else if (keyStr === 'toggle-collapse') {
      collapsed.value = !collapsed.value;
      selectedKeys.value = [route.path];
    } else {
      router.push(keyStr);
      mobileOpen.value = false;
    }
  };

  onErrorCaptured((err) => {
    console.error('Captured Error:', err);
    message.error('页面加载出现错误，请刷新重试');
    return false; // Prevent error from propagating further
  });
</script>

<template>
  <a-layout class="min-h-screen">
    <!-- Desktop Sider -->
    <a-layout-sider
      collapsed-width="80"
      v-model:collapsed="collapsed"
      collapsible
      :trigger="null"
      class="hidden lg:block !bg-white shadow-md z-20"
      theme="light"
      width="260"
    >
      <div
        class="flex items-center overflow-hidden whitespace-nowrap transition-all duration-300 h-20"
        :class="collapsed ? 'justify-center w-full' : 'px-6 gap-3'"
      >
        <img src="/logo.png" alt="Logo" class="w-10 h-10 min-w-[2.5rem] rounded-lg object-cover" />
        <div
          :class="{ 'opacity-0 w-0': collapsed, 'opacity-100 w-auto': !collapsed }"
          class="transition-all duration-300 overflow-hidden"
        >
          <div class="font-bold text-lg text-gray-800">L4D2 Manager</div>
          <div class="text-xs text-gray-500">Server Admin Panel</div>
        </div>
      </div>

      <a-menu
        v-model:selectedKeys="selectedKeys"
        mode="inline"
        :style="{ borderRight: 0 }"
        @click="handleMenuClick"
        class="flex flex-col h-[calc(100vh-80px)]"
      >
        <a-menu-item key="/">
          <template #icon><DashboardOutlined /></template>
          <span>服务器状态</span>
        </a-menu-item>
        <a-menu-item key="/maps">
          <template #icon><ReadOutlined /></template>
          <span>地图管理</span>
        </a-menu-item>
        <a-menu-item key="/rcon">
          <template #icon><CodeOutlined /></template>
          <span>RCON 控制台</span>
        </a-menu-item>
        <a-menu-item key="/plugins">
          <template #icon><AppstoreAddOutlined /></template>
          <span>插件管理</span>
        </a-menu-item>
        <a-menu-item key="/system">
          <template #icon><SettingOutlined /></template>
          <span>系统管理</span>
        </a-menu-item>

        <a-menu-divider />

        <a-menu-item key="logout" class="!text-red-500 hover:!text-red-600">
          <template #icon><LogoutOutlined /></template>
          <span>退出登录</span>
        </a-menu-item>

        <a-menu-divider class="!mt-auto !mb-0" />

        <a-menu-item key="toggle-collapse">
          <template #icon><MenuOutlined /></template>
          <span>{{ collapsed ? '展开' : '收起菜单' }}</span>
        </a-menu-item>
      </a-menu>
    </a-layout-sider>

    <!-- Mobile Drawer -->
    <a-drawer
      placement="left"
      :closable="false"
      :open="mobileOpen"
      @close="mobileOpen = false"
      class="lg:hidden"
      :body-style="{ padding: 0 }"
      width="260"
    >
      <div class="p-6 flex items-center gap-3 bg-white">
        <img src="/logo.png" alt="Logo" class="w-10 h-10 rounded-lg object-cover" />
        <div>
          <div class="font-bold text-lg text-gray-800">L4D2 Manager</div>
          <div class="text-xs text-gray-500">Server Admin Panel</div>
        </div>
      </div>

      <a-menu
        v-model:selectedKeys="selectedKeys"
        mode="inline"
        :style="{ borderRight: 0 }"
        @click="handleMenuClick"
      >
        <a-menu-item key="/">
          <template #icon><DashboardOutlined /></template>
          服务器状态
        </a-menu-item>
        <a-menu-item key="/maps">
          <template #icon><ReadOutlined /></template>
          地图管理
        </a-menu-item>
        <a-menu-item key="/rcon">
          <template #icon><CodeOutlined /></template>
          RCON 控制台
        </a-menu-item>
        <a-menu-item key="/plugins">
          <template #icon><AppstoreAddOutlined /></template>
          插件管理
        </a-menu-item>
        <a-menu-item key="/system">
          <template #icon><SettingOutlined /></template>
          系统管理
        </a-menu-item>

        <a-menu-divider />

        <a-menu-item key="logout" class="!text-red-500 hover:!text-red-600">
          <template #icon><LogoutOutlined /></template>
          退出登录
        </a-menu-item>
      </a-menu>
    </a-drawer>

    <a-layout>
      <!-- Mobile Header -->
      <a-layout-header class="lg:hidden !bg-white !px-4 flex items-center shadow-sm z-10 h-16">
        <MenuOutlined class="text-lg cursor-pointer" @click="mobileOpen = true" />
        <span class="ml-3 text-lg font-bold text-blue-600">L4D2 Manager</span>
      </a-layout-header>

      <a-layout-content
        class="p-4 md:p-6 overflow-y-auto bg-gray-50 h-[calc(100vh-64px)] lg:h-screen"
      >
        <div class="max-w-6xl mx-auto w-full animate-fade-in">
          <router-view v-slot="{ Component }">
            <transition name="fade" mode="out-in">
              <component :is="Component" :key="route.fullPath" v-if="Component" />
            </transition>
          </router-view>
        </div>
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<style scoped>
  .fade-enter-active,
  .fade-leave-active {
    transition: opacity 0.2s ease;
  }

  .fade-enter-from,
  .fade-leave-to {
    opacity: 0;
  }
</style>
