<script setup lang="ts">
  import { ref, watch, onMounted, onUnmounted, onErrorCaptured } from 'vue';
  import { useRoute } from 'vue-router';
  import { message } from 'ant-design-vue';
  import { BulbOutlined, MenuOutlined } from '@ant-design/icons-vue';
  import AppNavigation from '../components/AppNavigation.vue';
  import { useThemeStore } from '../stores/theme';

  const themeStore = useThemeStore();
  const route = useRoute();
  const MENU_COLLAPSED_STORAGE_KEY = 'l4d2_manager_menu_collapsed';
  const storedCollapsed = localStorage.getItem(MENU_COLLAPSED_STORAGE_KEY);
  const desktopMedia = window.matchMedia('(min-width: 1024px)');

  const isDesktop = ref(desktopMedia.matches);
  const mobileOpen = ref(false);
  const mobileMenuTrigger = ref<HTMLButtonElement | null>(null);
  const collapsed = ref(storedCollapsed === null ? true : storedCollapsed === 'true');

  watch(collapsed, (val) => {
    localStorage.setItem(MENU_COLLAPSED_STORAGE_KEY, String(val));
  });

  const handleViewportChange = (event: MediaQueryListEvent) => {
    isDesktop.value = event.matches;
    mobileOpen.value = false;
  };

  const handleDrawerVisibilityChange = (open: boolean) => {
    if (!open && mobileMenuTrigger.value?.offsetParent != null) {
      mobileMenuTrigger.value.focus();
    }
  };

  onMounted(() => desktopMedia.addEventListener('change', handleViewportChange));
  onUnmounted(() => desktopMedia.removeEventListener('change', handleViewportChange));

  onErrorCaptured((err) => {
    console.error('Captured Error:', err);
    message.error('页面加载出现错误，请刷新重试');
    return false;
  });
</script>

<template>
  <a-layout class="app-layout transition-colors duration-300 bg-gray-50 dark:bg-gray-950">
    <a-layout-sider
      v-if="isDesktop"
      :collapsed="collapsed"
      :collapsed-width="80"
      :trigger="null"
      :width="260"
      :theme="themeStore.isDark ? 'dark' : 'light'"
      class="desktop-sider shadow-md z-20 bg-white dark:bg-gray-900"
    >
      <AppNavigation :collapsed="collapsed" @toggle-collapse="collapsed = !collapsed" />
    </a-layout-sider>

    <a-drawer
      v-else
      placement="left"
      :closable="false"
      :open="mobileOpen"
      :body-style="{ padding: 0, height: '100%', overflow: 'hidden' }"
      width="min(300px, calc(100vw - 48px))"
      @close="mobileOpen = false"
      @after-open-change="handleDrawerVisibilityChange"
    >
      <AppNavigation id="mobile-navigation" mobile @close="mobileOpen = false" />
    </a-drawer>

    <a-layout class="min-w-0 min-h-0">
      <a-layout-header
        class="mobile-header lg:hidden !px-3 flex items-center justify-between shadow-sm z-10 transition-colors duration-300 bg-white dark:bg-gray-900"
      >
        <div class="flex min-w-0 items-center gap-2">
          <button
            ref="mobileMenuTrigger"
            type="button"
            class="header-button text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-800"
            aria-label="打开导航菜单"
            aria-controls="mobile-navigation"
            :aria-expanded="mobileOpen"
            @click="mobileOpen = true"
          >
            <MenuOutlined class="text-lg" />
          </button>
          <span class="truncate text-lg font-bold text-blue-600 dark:text-blue-400">L4D2 Manager</span>
        </div>
        <button
          type="button"
          class="header-button text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-800"
          :aria-label="themeStore.isDark ? '切换亮色' : '切换暗色'"
          @click="themeStore.toggleTheme()"
        >
          <BulbOutlined class="text-lg" />
        </button>
      </a-layout-header>

      <a-layout-content
        class="main-content min-h-0 p-4 md:p-6 transition-colors duration-300 bg-gray-50 dark:bg-gray-950"
      >
        <div class="max-w-6xl mx-auto w-full animate-fade-in">
          <router-view v-slot="{ Component }">
            <transition name="fade" mode="out-in">
              <keep-alive :include="['Maps']">
                <component :is="Component" :key="route.fullPath" v-if="Component" />
              </keep-alive>
            </transition>
          </router-view>
        </div>
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<style scoped>
  .app-layout {
    height: 100vh;
    height: 100dvh;
    min-height: 0;
    overflow: hidden;
  }

  .desktop-sider {
    height: 100%;
  }

  .mobile-header {
    flex: none;
    height: 64px;
  }

  .header-button {
    display: flex;
    flex: none;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    border-radius: 8px;
    cursor: pointer;
  }

  .header-button:focus-visible {
    outline: 2px solid #3b82f6;
    outline-offset: -2px;
  }

  .main-content {
    overflow: auto;
    overscroll-behavior: contain;
  }

  .fade-enter-active,
  .fade-leave-active {
    transition: opacity 0.2s ease;
  }

  .fade-enter-from,
  .fade-leave-to {
    opacity: 0;
  }
</style>
