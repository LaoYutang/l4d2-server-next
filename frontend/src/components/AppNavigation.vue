<script setup lang="ts">
  import { computed, nextTick, onMounted, ref, watch, type Component } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import {
    AppstoreAddOutlined,
    AuditOutlined,
    BulbOutlined,
    CloseOutlined,
    CodeOutlined,
    DashboardOutlined,
    EditOutlined,
    FileTextOutlined,
    LineChartOutlined,
    LockOutlined,
    LogoutOutlined,
    MenuFoldOutlined,
    MenuUnfoldOutlined,
    ReadOutlined,
    SafetyCertificateOutlined,
    SaveOutlined,
    SecurityScanOutlined,
    SettingOutlined,
    TeamOutlined,
    ToolOutlined,
  } from '@ant-design/icons-vue';
  import { useAuthStore } from '../stores/auth';
  import { useThemeStore } from '../stores/theme';

  const props = withDefaults(defineProps<{ collapsed?: boolean; mobile?: boolean }>(), {
    collapsed: false,
    mobile: false,
  });
  const emit = defineEmits<{ close: []; 'toggle-collapse': [] }>();

  interface NavigationLink {
    key: string;
    label: string;
    icon: Component;
    adminOnly?: boolean;
  }

  interface NavigationGroup {
    key: string;
    label: string;
    icon: Component;
    children: NavigationLink[];
  }

  const commonLinks: NavigationLink[] = [
    { key: '/', label: '服务器状态', icon: DashboardOutlined },
    { key: '/maps', label: '地图管理', icon: ReadOutlined },
    { key: '/plugins', label: '插件管理', icon: AppstoreAddOutlined },
    { key: '/rcon', label: 'RCON 控制台', icon: CodeOutlined },
  ];
  const navigationGroups: NavigationGroup[] = [
    {
      key: 'activity',
      label: '运行数据',
      icon: LineChartOutlined,
      children: [
        { key: '/monitor', label: '性能监控', icon: LineChartOutlined },
        { key: '/player-stats', label: '玩家统计', icon: TeamOutlined },
        { key: '/logs', label: '日志查看', icon: FileTextOutlined },
      ],
    },
    {
      key: 'server',
      label: '服务器设置',
      icon: ToolOutlined,
      children: [
        { key: '/server-info', label: '服务器信息', icon: EditOutlined },
        { key: '/server-config', label: '服务器配置', icon: ToolOutlined },
        { key: '/backup', label: '备份管理', icon: SaveOutlined },
      ],
    },
    {
      key: 'system',
      label: '系统与安全',
      icon: SecurityScanOutlined,
      children: [
        { key: '/admins', label: '管理员设置', icon: SafetyCertificateOutlined },
        { key: '/auth-codes', label: '授权码管理', icon: LockOutlined, adminOnly: true },
        { key: '/access-control', label: '访问控制', icon: SecurityScanOutlined, adminOnly: true },
        { key: '/audit', label: '操作审计', icon: AuditOutlined, adminOnly: true },
        { key: '/system', label: '系统管理', icon: SettingOutlined },
      ],
    },
  ];

  const authStore = useAuthStore();
  const themeStore = useThemeStore();
  const route = useRoute();
  const router = useRouter();
  const scrollViewport = ref<HTMLElement | null>(null);
  const selectedKeys = computed(() => [route.path]);
  const themeLabel = computed(() => (themeStore.isDark ? '切换亮色' : '切换暗色'));
  const visibleGroups = computed(() =>
    navigationGroups.map((group) => ({
      ...group,
      children: group.children.filter((item) => !item.adminOnly || authStore.isAdmin),
    }))
  );

  const OPEN_GROUPS_STORAGE_KEY = 'l4d2_manager_menu_open_groups';
  const readOpenGroups = (): string[] => {
    try {
      const stored: unknown = JSON.parse(localStorage.getItem(OPEN_GROUPS_STORAGE_KEY) || '[]');
      return Array.isArray(stored)
        ? [...new Set(stored.filter((key) => navigationGroups.some((group) => group.key === key)))]
        : [];
    } catch {
      return [];
    }
  };

  const expandedKeys = ref<string[]>(readOpenGroups());
  const popupKeys = ref<string[]>([]);
  const openKeys = computed(() => (props.collapsed ? popupKeys.value : expandedKeys.value));

  watch(expandedKeys, (keys) => {
    localStorage.setItem(OPEN_GROUPS_STORAGE_KEY, JSON.stringify(keys));
  });

  const scrollToSelected = () => {
    const viewport = scrollViewport.value;
    const selected = viewport?.querySelector<HTMLElement>(
      props.collapsed ? '.ant-menu-submenu-selected, .ant-menu-item-selected' : '.ant-menu-item-selected'
    );
    if (!viewport || !selected) return;

    const viewportRect = viewport.getBoundingClientRect();
    const selectedRect = selected.getBoundingClientRect();
    if (selectedRect.bottom > viewportRect.bottom) {
      viewport.scrollTop += selectedRect.bottom - viewportRect.bottom + 8;
    } else if (selectedRect.top < viewportRect.top) {
      viewport.scrollTop -= viewportRect.top - selectedRect.top + 8;
    }
  };

  watch(
    () => route.path,
    () => {
      const activeGroup = visibleGroups.value.find((group) =>
        group.children.some((item) => item.key === route.path)
      );
      if (activeGroup && !expandedKeys.value.includes(activeGroup.key)) {
        expandedKeys.value = [...expandedKeys.value, activeGroup.key];
      }
      popupKeys.value = [];
      nextTick(scrollToSelected);
    },
    { immediate: true }
  );

  watch(
    () => props.collapsed,
    () => {
      popupKeys.value = [];
      nextTick(scrollToSelected);
    }
  );
  onMounted(scrollToSelected);

  const handleOpenChange = (keys: (string | number)[]) => {
    if (props.collapsed) {
      popupKeys.value = keys.map(String);
    } else {
      expandedKeys.value = keys.map(String);
    }
  };

  const handleNavigate = ({ key }: { key: string | number }) => {
    router.push(String(key));
    emit('close');
  };

  const handleLogout = () => {
    emit('close');
    authStore.logout();
    router.push('/login');
  };

  // Menu popups and collapsed tooltips must escape the scroll viewport.
  const getNavigationPopupContainer = () => document.body;
</script>

<template>
  <section
    class="navigation-panel bg-white text-gray-700 dark:bg-gray-900 dark:text-gray-300"
    :class="{ 'navigation-panel--collapsed': collapsed, 'navigation-panel--mobile': mobile }"
  >
    <header class="navigation-brand border-b border-gray-100 dark:border-gray-800">
      <img src="/logo.png" alt="Logo" class="h-10 w-10 shrink-0 rounded-lg object-cover" />
      <div v-if="!collapsed" class="min-w-0 flex-1 whitespace-nowrap">
        <div class="truncate text-base font-bold text-gray-800 dark:text-gray-100">L4D2 Manager</div>
        <div class="text-xs text-gray-500 dark:text-gray-400">Server Admin Panel</div>
      </div>
      <button
        v-if="mobile"
        type="button"
        class="navigation-close hover:bg-gray-100 dark:hover:bg-gray-800"
        aria-label="关闭导航菜单"
        @click="emit('close')"
      >
        <CloseOutlined />
      </button>
    </header>

    <nav ref="scrollViewport" class="navigation-scroll" aria-label="主导航">
      <a-config-provider :get-popup-container="getNavigationPopupContainer">
        <a-menu
          :selected-keys="selectedKeys"
          :open-keys="openKeys"
          :theme="themeStore.isDark ? 'dark' : 'light'"
          :get-popup-container="getNavigationPopupContainer"
          mode="inline"
          class="navigation-menu"
          @open-change="handleOpenChange"
          @click="handleNavigate"
        >
          <a-menu-item v-for="item in commonLinks" :key="item.key" :title="item.label">
            <template #icon><component :is="item.icon" /></template>
            <span>{{ item.label }}</span>
          </a-menu-item>

          <a-menu-divider />

          <a-sub-menu
            v-for="group in visibleGroups"
            :key="group.key"
            :title="group.label"
            popup-class-name="navigation-submenu-popup"
          >
            <template #icon><component :is="group.icon" /></template>
            <a-menu-item v-for="item in group.children" :key="item.key" :title="item.label">
              <template #icon><component :is="item.icon" /></template>
              <span>{{ item.label }}</span>
            </a-menu-item>
          </a-sub-menu>
        </a-menu>
      </a-config-provider>
    </nav>

    <footer class="navigation-footer border-t border-gray-200 dark:border-gray-700">
      <div class="navigation-account-actions">
        <a-tooltip
          :title="collapsed ? themeLabel : undefined"
          placement="right"
          :get-popup-container="getNavigationPopupContainer"
        >
          <button
            type="button"
            class="navigation-action hover:bg-gray-100 dark:hover:bg-gray-800"
            :aria-label="themeLabel"
            @click="themeStore.toggleTheme()"
          >
            <BulbOutlined />
            <span v-if="!collapsed">{{ themeLabel }}</span>
          </button>
        </a-tooltip>
        <a-tooltip
          :title="collapsed ? '退出登录' : undefined"
          placement="right"
          :get-popup-container="getNavigationPopupContainer"
        >
          <button
            type="button"
            class="navigation-action text-red-500 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/40"
            aria-label="退出登录"
            @click="handleLogout"
          >
            <LogoutOutlined />
            <span v-if="!collapsed">退出登录</span>
          </button>
        </a-tooltip>
      </div>
      <button
        v-if="!mobile"
        type="button"
        class="navigation-action navigation-collapse border-t border-gray-100 hover:bg-gray-100 dark:border-gray-800 dark:hover:bg-gray-800"
        :aria-label="collapsed ? '展开菜单' : '收起菜单'"
        :aria-expanded="!collapsed"
        @click="emit('toggle-collapse')"
      >
        <MenuUnfoldOutlined v-if="collapsed" />
        <MenuFoldOutlined v-else />
        <span v-if="!collapsed">收起菜单</span>
      </button>
    </footer>
  </section>
</template>

<style scoped>
  .navigation-panel {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }

  .navigation-brand {
    display: flex;
    flex: none;
    align-items: center;
    gap: 12px;
    height: 72px;
    padding: 0 20px;
  }

  .navigation-scroll {
    flex: 1;
    min-height: 0;
    overflow-x: hidden;
    overflow-y: auto;
    overscroll-behavior: contain;
    scroll-padding-block: 8px;
    padding-block: 8px;
  }

  .navigation-menu {
    border-inline-end: 0 !important;
  }

  .navigation-footer {
    flex: none;
    padding: 8px;
    box-shadow: 0 -4px 12px rgb(15 23 42 / 4%);
  }

  .navigation-action,
  .navigation-close {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 44px;
    border-radius: 8px;
    cursor: pointer;
    transition: background-color 0.2s;
  }

  .navigation-action {
    width: 100%;
    padding: 0 16px;
    text-align: start;
  }

  .navigation-action:focus-visible,
  .navigation-close:focus-visible {
    outline: 2px solid #3b82f6;
    outline-offset: -2px;
  }

  .navigation-collapse {
    margin-top: 4px;
    border-radius: 0 0 8px 8px;
  }

  .navigation-panel--collapsed .navigation-brand,
  .navigation-panel--collapsed .navigation-action {
    justify-content: center;
    padding-inline: 0;
  }

  .navigation-panel--mobile .navigation-brand {
    gap: 8px;
    padding-inline: 12px;
  }

  .navigation-close {
    flex: none;
    justify-content: center;
    width: 44px;
  }

  .navigation-panel--mobile .navigation-account-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .navigation-panel--mobile .navigation-action {
    justify-content: center;
    gap: 8px;
    padding-inline: 8px;
  }

  .navigation-panel--mobile .navigation-footer {
    padding-bottom: max(8px, env(safe-area-inset-bottom));
  }

  .navigation-panel--mobile :deep(.ant-menu-item),
  .navigation-panel--mobile :deep(.ant-menu-submenu-title) {
    height: 44px;
    line-height: 44px;
  }

  :global(.navigation-submenu-popup .ant-menu) {
    max-height: calc(100vh - 24px);
    max-height: calc(100dvh - 24px);
    overflow-y: auto;
    overscroll-behavior: contain;
  }
</style>
