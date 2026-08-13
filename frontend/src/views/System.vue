<script setup lang="ts">
  import { computed, nextTick, onErrorCaptured, onMounted, onUnmounted, ref, watch } from 'vue';
  import {
    message,
    Card as ACard,
    Select as ASelect,
    SelectOption as ASelectOption,
    Button as AButton,
    Input as AInput,
    Divider as ADivider,
    Switch as ASwitch,
  } from 'ant-design-vue';
  import {
    KeyOutlined,
    InfoCircleOutlined,
    CheckCircleOutlined,
    CopyOutlined,
    CheckOutlined,
    SafetyCertificateOutlined,
    LineChartOutlined,
    DatabaseOutlined,
    CloudUploadOutlined,
    ReloadOutlined,
  } from '@ant-design/icons-vue';
  import { api } from '../services/api';
  import { useAuthStore } from '../stores/auth';
  import { useMonitorStore } from '../stores/monitor';
  import { copyToClipboard } from '../utils/clipboard';
  import MapHotReloadSetting from '../components/settings/MapHotReloadSetting.vue';
  import SteamCDNSetting from '../components/settings/SteamCDNSetting.vue';

  type SettingsSection = 'authorization' | 'statistics' | 'map-management' | 'about';

  const authStore = useAuthStore();
  const monitorStore = useMonitorStore();
  const isAdmin = computed(() => authStore.isAdmin);
  const activeSection = ref<SettingsSection>(isAdmin.value ? 'authorization' : 'about');
  const authorizationSection = ref<HTMLElement | null>(null);
  const statisticsSection = ref<HTMLElement | null>(null);
  const mapManagementSection = ref<HTMLElement | null>(null);
  const aboutSection = ref<HTMLElement | null>(null);

  const expiredHours = ref(1);
  const generating = ref(false);
  const generatedCode = ref('');
  const expirationTime = ref('');
  const copied = ref(false);
  const codeInput = ref<any>(null);
  const version = ref('');
  const enableSelfService = ref(false);
  const settingSelfService = ref(false);
  const enablePlayerStats = ref(false);
  const settingPlayerStats = ref(false);
  const enableMonitorHistory = ref(false);
  const settingMonitorHistory = ref(false);
  const enableVpkTrim = ref(false);
  const settingVpkTrim = ref(false);
  let sectionUpdateFrame: number | null = null;
  let pendingSection: SettingsSection | null = null;
  let sectionNavigationTimer: number | null = null;

  onErrorCaptured((err) => {
    console.error('System.vue Error:', err);
    message.error('系统管理页面发生错误');
    return false;
  });

  const fetchVersion = async () => {
    try {
      const data = await api.getVersion();
      version.value = data.version;
    } catch (error) {
      console.error('Failed to fetch version:', error);
    }
  };

  const fetchSelfServiceStatus = async () => {
    if (!isAdmin.value) return;
    try {
      const status = await api.getSelfServiceStatus();
      enableSelfService.value = status.enabled;
    } catch (error) {
      console.error('Failed to fetch self service status:', error);
    }
  };

  const fetchPlayerStatsConfig = async () => {
    if (!isAdmin.value) return;
    try {
      const config = await api.getPlayerStatsConfig();
      enablePlayerStats.value = config.enabled;
    } catch (error) {
      console.error('Failed to fetch player stats config:', error);
    }
  };

  const fetchMonitorConfig = async () => {
    if (!isAdmin.value) return;
    try {
      const config = await api.getMonitorConfig();
      enableMonitorHistory.value = config.history_enabled;
      monitorStore.setHistoryEnabled(config.history_enabled);
    } catch (error) {
      console.error('Failed to fetch monitor config:', error);
    }
  };

  const fetchVpkTrimConfig = async () => {
    if (!isAdmin.value) return;
    try {
      const config = await api.getVpkTrimConfig();
      enableVpkTrim.value = config.enabled;
    } catch (error) {
      console.error('Failed to fetch VPK trim config:', error);
    }
  };

  const loadSettings = async () => {
    const tasks: Array<Promise<void>> = [fetchVersion()];
    if (isAdmin.value) {
      tasks.push(
        fetchSelfServiceStatus(),
        fetchPlayerStatsConfig(),
        fetchMonitorConfig(),
        fetchVpkTrimConfig()
      );
    }
    await Promise.all(tasks);
  };

  const toggleSelfService = async (checked: boolean | string | number) => {
    const isChecked = Boolean(checked);
    settingSelfService.value = true;
    try {
      await api.setSelfServiceConfig(isChecked);
      enableSelfService.value = isChecked;
      message.success(isChecked ? '已开启自助授权功能' : '已关闭自助授权功能');
    } catch (error: any) {
      message.error(`设置失败: ${error.message}`);
      enableSelfService.value = !isChecked;
    } finally {
      settingSelfService.value = false;
    }
  };

  const togglePlayerStats = async (checked: boolean | string | number) => {
    const isChecked = Boolean(checked);
    settingPlayerStats.value = true;
    try {
      await api.setPlayerStatsConfig(isChecked);
      enablePlayerStats.value = isChecked;
      message.success(isChecked ? '已开启玩家在线统计' : '已关闭玩家在线统计');
    } catch (error: any) {
      message.error(`设置失败: ${error.message}`);
      enablePlayerStats.value = !isChecked;
    } finally {
      settingPlayerStats.value = false;
    }
  };

  const toggleMonitorHistory = async (checked: boolean | string | number) => {
    const isChecked = Boolean(checked);
    settingMonitorHistory.value = true;
    try {
      await api.setMonitorHistoryConfig(isChecked);
      enableMonitorHistory.value = isChecked;
      monitorStore.setHistoryEnabled(isChecked);
      message.success(isChecked ? '已开启性能监控历史记录' : '已关闭性能监控历史记录');
    } catch (error: any) {
      message.error(`设置失败: ${error.message}`);
      enableMonitorHistory.value = !isChecked;
      monitorStore.setHistoryEnabled(!isChecked);
    } finally {
      settingMonitorHistory.value = false;
    }
  };

  const toggleVpkTrim = async (checked: boolean | string | number) => {
    const isChecked = Boolean(checked);
    settingVpkTrim.value = true;
    try {
      await api.setVpkTrimConfig(isChecked);
      enableVpkTrim.value = isChecked;
      message.success(isChecked ? '已开启地图自动精简' : '已关闭地图自动精简');
    } catch (error: any) {
      message.error(`设置失败: ${error.message}`);
      enableVpkTrim.value = !isChecked;
    } finally {
      settingVpkTrim.value = false;
    }
  };

  const generateCode = async () => {
    generating.value = true;
    generatedCode.value = '';
    copied.value = false;

    try {
      generatedCode.value = await api.generateTempAuthCode(expiredHours.value);

      const date = new Date();
      date.setHours(date.getHours() + Number(expiredHours.value));
      expirationTime.value = date.toLocaleString();
      message.success('授权码生成成功');
    } catch (error: any) {
      message.error(`生成失败: ${error.message}`);
    } finally {
      generating.value = false;
    }
  };

  const copyCode = async () => {
    if (!generatedCode.value) return;

    const success = await copyToClipboard(generatedCode.value);
    if (success) {
      copied.value = true;
      message.success('已复制到剪贴板');
      window.setTimeout(() => {
        copied.value = false;
      }, 2000);
      return;
    }

    codeInput.value?.focus();
    message.warning('无法自动复制，请手动复制');
  };

  const getSectionElement = (section: SettingsSection) => {
    switch (section) {
      case 'authorization':
        return authorizationSection.value;
      case 'statistics':
        return statisticsSection.value;
      case 'map-management':
        return mapManagementSection.value;
      case 'about':
        return aboutSection.value;
    }
  };

  const getVisibleSections = (): SettingsSection[] =>
    isAdmin.value
      ? ['authorization', 'statistics', 'map-management', 'about']
      : ['about'];

  const scrollToSection = (section: SettingsSection) => {
    const element = getSectionElement(section);
    if (!element) return;

    if (sectionNavigationTimer !== null) {
      window.clearTimeout(sectionNavigationTimer);
    }
    pendingSection = section;
    activeSection.value = section;
    element.scrollIntoView({ behavior: 'smooth', block: 'start' });
    sectionNavigationTimer = window.setTimeout(() => {
      pendingSection = null;
      sectionNavigationTimer = null;
      updateActiveSection();
    }, 700);
  };

  const updateActiveSection = () => {
    if (pendingSection) {
      activeSection.value = pendingSection;
      return;
    }

    const visibleSections = getVisibleSections();
    const [firstVisibleSection] = visibleSections;
    const lastVisibleSection = visibleSections[visibleSections.length - 1];
    const scrollContainer = firstVisibleSection
      ? getSectionElement(firstVisibleSection)?.closest<HTMLElement>('main')
      : null;
    if (
      scrollContainer &&
      lastVisibleSection &&
      scrollContainer.scrollHeight -
        scrollContainer.scrollTop -
        scrollContainer.clientHeight <=
        2
    ) {
      activeSection.value = lastVisibleSection;
      return;
    }

    const mobileNav = window.matchMedia('(max-width: 767px)').matches
      ? document.querySelector<HTMLElement>('.settings-nav')
      : null;
    const markerOffset = mobileNav
      ? mobileNav.getBoundingClientRect().bottom + 8
      : 96;
    const sections = visibleSections
      .map((section) => ({
        section,
        element: getSectionElement(section),
      }))
      .filter(
        (item): item is { section: SettingsSection; element: HTMLElement } =>
          Boolean(item.element)
      )
      .map((item) => ({
        ...item,
        rect: item.element.getBoundingClientRect(),
      }));

    const current =
      sections.find(
        ({ rect }) => rect.top <= markerOffset && rect.bottom > markerOffset
      ) ||
      sections
        .filter(({ rect }) => rect.top > markerOffset)
        .sort((left, right) => left.rect.top - right.rect.top)[0] ||
      sections[sections.length - 1];

    if (current) {
      activeSection.value = current.section;
    }
  };

  const scheduleSectionUpdate = () => {
    if (sectionUpdateFrame !== null) return;

    sectionUpdateFrame = window.requestAnimationFrame(() => {
      sectionUpdateFrame = null;
      updateActiveSection();
    });
  };

  const teardownSectionTracking = () => {
    document.removeEventListener('scroll', scheduleSectionUpdate, true);
    window.removeEventListener('resize', scheduleSectionUpdate);
    if (sectionUpdateFrame !== null) {
      window.cancelAnimationFrame(sectionUpdateFrame);
      sectionUpdateFrame = null;
    }
    if (sectionNavigationTimer !== null) {
      window.clearTimeout(sectionNavigationTimer);
      sectionNavigationTimer = null;
    }
    pendingSection = null;
  };

  const setupSectionTracking = () => {
    teardownSectionTracking();
    const [firstVisibleSection] = getVisibleSections();
    if (!firstVisibleSection) return;
    document.addEventListener('scroll', scheduleSectionUpdate, {
      capture: true,
      passive: true,
    });
    window.addEventListener('resize', scheduleSectionUpdate, { passive: true });
    updateActiveSection();
  };

  watch(isAdmin, async (admin) => {
    if (!admin) {
      activeSection.value = 'about';
    } else if (activeSection.value === 'about') {
      activeSection.value = 'authorization';
    }
    await nextTick();
    setupSectionTracking();
    void loadSettings();
  });

  onMounted(() => {
    void loadSettings();
    void nextTick(setupSectionTracking);
  });

  onUnmounted(() => {
    teardownSectionTracking();
  });
</script>

<template>
  <div class="system-settings">
    <a-card
      class="settings-shell shadow-xl dark:shadow-slate-950/50"
      :bordered="false"
    >
      <div class="settings-layout">
        <nav class="settings-nav" aria-label="设置分类">
          <button
            v-if="isAdmin"
            type="button"
            class="settings-nav-item"
            :class="{ active: activeSection === 'authorization' }"
            :aria-current="activeSection === 'authorization' ? 'location' : undefined"
            @click="scrollToSection('authorization')"
          >
            <KeyOutlined />
            <span>授权管理</span>
          </button>
          <button
            v-if="isAdmin"
            type="button"
            class="settings-nav-item"
            :class="{ active: activeSection === 'statistics' }"
            :aria-current="activeSection === 'statistics' ? 'location' : undefined"
            @click="scrollToSection('statistics')"
          >
            <LineChartOutlined />
            <span>数据统计</span>
          </button>
          <button
            v-if="isAdmin"
            type="button"
            class="settings-nav-item"
            :class="{ active: activeSection === 'map-management' }"
            :aria-current="activeSection === 'map-management' ? 'location' : undefined"
            @click="scrollToSection('map-management')"
          >
            <CloudUploadOutlined />
            <span>地图管理设置</span>
          </button>
          <button
            type="button"
            class="settings-nav-item"
            :class="{ active: activeSection === 'about' }"
            :aria-current="activeSection === 'about' ? 'location' : undefined"
            @click="scrollToSection('about')"
          >
            <InfoCircleOutlined />
            <span>关于系统</span>
          </button>
        </nav>

        <div class="settings-content">
          <section
            v-if="isAdmin"
            ref="authorizationSection"
            class="settings-category"
            data-settings-section="authorization"
          >
            <div class="settings-heading">
              <h2><KeyOutlined class="text-blue-500" /> 临时授权管理</h2>
              <p>管理访客自助授权，并生成指定有效期的临时访问授权码。</p>
            </div>

            <div class="space-y-4">
              <section class="setting-section">
                <div class="setting-row">
                  <div class="min-w-0">
                    <div class="setting-title">
                      <SafetyCertificateOutlined />
                      开启自助获取通道
                    </div>
                    <div class="setting-description">
                      开启后，访客可在登录页自助获取 1 小时有效期的授权码，获取后有 1 小时全局冷却时间。
                    </div>
                  </div>
                  <a-switch
                    class="shrink-0"
                    :checked="enableSelfService"
                    :loading="settingSelfService"
                    checked-children="开"
                    un-checked-children="关"
                    @update:checked="toggleSelfService"
                  />
                </div>
              </section>

              <section class="setting-section">
                <div class="mb-4">
                  <div class="setting-title">手动生成（管理员专用）</div>
                  <div class="setting-description">
                    选择有效期并直接生成授权码，不受自助授权冷却时间限制。
                  </div>
                </div>

                <div class="flex flex-col gap-3 sm:flex-row">
                  <a-select v-model:value="expiredHours" class="min-w-0 flex-1">
                    <a-select-option :value="1">1 小时</a-select-option>
                    <a-select-option :value="6">6 小时</a-select-option>
                    <a-select-option :value="12">12 小时</a-select-option>
                    <a-select-option :value="24">24 小时（1 天）</a-select-option>
                    <a-select-option :value="72">72 小时（3 天）</a-select-option>
                    <a-select-option :value="168">168 小时（7 天）</a-select-option>
                  </a-select>
                  <a-button type="primary" :loading="generating" @click="generateCode">
                    {{ generating ? '生成中' : '生成授权码' }}
                  </a-button>
                </div>

                <div
                  v-if="generatedCode"
                  class="animate-fade-in mt-4 rounded-lg border border-green-200 bg-green-50 p-4 dark:border-green-800 dark:bg-green-900/20"
                >
                  <div
                    class="mb-1 flex items-center gap-2 font-bold text-green-600 dark:text-green-400"
                  >
                    <CheckCircleOutlined />
                    生成成功
                  </div>
                  <div class="mb-3 text-xs text-gray-500 dark:text-gray-400">
                    有效期至：{{ expirationTime }}
                  </div>
                  <div class="flex flex-col gap-2 sm:flex-row">
                    <a-input
                      ref="codeInput"
                      v-model:value="generatedCode"
                      readonly
                      class="min-w-0 flex-1 text-center font-mono text-lg !text-blue-600 dark:!text-blue-400"
                    />
                    <a-button class="!flex !items-center !justify-center" @click="copyCode">
                      <template #icon>
                        <CheckOutlined v-if="copied" />
                        <CopyOutlined v-else />
                      </template>
                      {{ copied ? '已复制' : '复制' }}
                    </a-button>
                  </div>
                </div>
              </section>
            </div>
          </section>

          <section
            v-if="isAdmin"
            ref="statisticsSection"
            class="settings-category"
            data-settings-section="statistics"
          >
            <div class="settings-heading">
              <h2><LineChartOutlined class="text-emerald-500" /> 数据统计设置</h2>
              <p>控制玩家在线统计和性能监控历史数据的采集。</p>
            </div>

            <div class="space-y-4">
              <section class="setting-section">
                <div class="setting-row">
                  <div class="min-w-0">
                    <div class="setting-title">
                      <LineChartOutlined />
                      开启玩家在线统计
                    </div>
                    <div class="setting-description">
                      开启后每 10 分钟采集一次玩家列表，数据保留 30 天，服务器离线时会记录离线状态。
                    </div>
                  </div>
                  <a-switch
                    class="shrink-0"
                    :checked="enablePlayerStats"
                    :loading="settingPlayerStats"
                    checked-children="开"
                    un-checked-children="关"
                    @update:checked="togglePlayerStats"
                  />
                </div>
              </section>

              <section class="setting-section">
                <div class="setting-row">
                  <div class="min-w-0">
                    <div class="setting-title">
                      <DatabaseOutlined />
                      开启性能监控历史记录
                    </div>
                    <div class="setting-description">
                      开启后每秒保存性能采样，历史数据保留 3 天，用于性能监控页面的历史区间查询。
                    </div>
                  </div>
                  <a-switch
                    class="shrink-0"
                    :checked="enableMonitorHistory"
                    :loading="settingMonitorHistory"
                    checked-children="开"
                    un-checked-children="关"
                    @update:checked="toggleMonitorHistory"
                  />
                </div>
              </section>
            </div>
          </section>

          <section
            v-if="isAdmin"
            ref="mapManagementSection"
            class="settings-category"
            data-settings-section="map-management"
          >
            <div class="settings-heading">
              <h2><CloudUploadOutlined class="text-orange-500" /> 地图管理设置</h2>
              <p>统一管理地图资源精简、热重载指令和 Steam CDN 下载配置。</p>
            </div>

            <div class="space-y-4">
              <section class="setting-section">
                <div class="setting-row">
                  <div class="min-w-0">
                    <div class="setting-title">
                      <CloudUploadOutlined />
                      启用地图自动精简
                    </div>
                    <div class="setting-description">
                      仅精简启用后上传和下载的地图资源，可以极大程度减轻服务器存储压力。精简后的地图仅适合服务端使用，不适合客户端本地使用。
                    </div>
                  </div>
                  <a-switch
                    class="shrink-0"
                    :checked="enableVpkTrim"
                    :loading="settingVpkTrim"
                    checked-children="开"
                    un-checked-children="关"
                    @update:checked="toggleVpkTrim"
                  />
                </div>
              </section>

              <MapHotReloadSetting :active="isAdmin" context="page" />
              <SteamCDNSetting :active="isAdmin" context="page" />
            </div>
          </section>

          <section
            ref="aboutSection"
            class="settings-category"
            data-settings-section="about"
          >
            <div class="settings-heading">
              <h2><InfoCircleOutlined class="text-blue-500" /> 关于系统</h2>
              <p>查看管理器版本、项目信息和开源协议。</p>
            </div>

            <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
              <section class="setting-section">
                <h3 class="setting-title mb-3"><InfoCircleOutlined /> 项目信息</h3>
                <p class="break-words text-sm leading-7 text-gray-600 dark:text-gray-300">
                  L4D2 服务器管理工具<br />
                  版本：{{ version || '加载中...' }}<br />
                  作者：LaoYutang<br />
                  GitHub：
                  <a
                    href="https://github.com/LaoYutang/l4d2-server-next"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="break-all text-blue-500 hover:underline dark:text-blue-400"
                  >
                    https://github.com/LaoYutang/l4d2-server-next
                  </a>
                  <br />
                  © 2026 开源社区贡献
                </p>
              </section>

              <section class="setting-section">
                <h3 class="setting-title mb-3"><ReloadOutlined /> 开源协议</h3>
                <p class="text-sm leading-7 text-gray-600 dark:text-gray-300">
                  本项目基于 Apache License 2.0 开源协议发布。<br />
                  欢迎贡献代码和提出建议。
                </p>
                <a
                  href="https://github.com/LaoYutang/l4d2-server-next/blob/master/LICENSE"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="mt-3 inline-flex text-blue-500 hover:underline dark:text-blue-400"
                >
                  查看许可证
                </a>
              </section>
            </div>

            <a-divider />
            <div class="text-center text-sm text-gray-500 dark:text-gray-400">
              Made with ❤️ by the community
            </div>
          </section>
        </div>
      </div>
    </a-card>
  </div>
</template>

<style scoped>
  .system-settings {
    min-width: 0;
  }

  .settings-layout {
    display: grid;
    grid-template-columns: 180px minmax(0, 1fr);
    align-items: start;
    gap: 1.5rem;
    min-width: 0;
  }

  .settings-nav {
    position: sticky;
    top: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    min-width: 0;
    border-right: 1px solid rgb(229 231 235);
    padding-right: 1rem;
  }

  :global(.dark .system-settings .settings-nav) {
    border-color: rgb(51 65 85);
  }

  .settings-nav-item {
    position: relative;
    display: flex;
    align-items: center;
    gap: 0.625rem;
    width: 100%;
    border: 0;
    border-radius: 0.5rem;
    background: transparent;
    padding: 0.75rem 1rem;
    color: rgb(55 65 81);
    font-size: 0.875rem;
    line-height: 1.25rem;
    text-align: left;
    white-space: nowrap;
    cursor: pointer;
    outline: none;
    transition:
      color 0.2s ease,
      background-color 0.2s ease;
  }

  .settings-nav-item:hover {
    background: rgb(249 250 251);
    color: rgb(22 119 255);
  }

  .settings-nav-item:focus-visible {
    outline: 2px solid rgb(22 119 255);
    outline-offset: 2px;
  }

  .settings-nav-item.active {
    background: rgb(230 244 255);
    color: rgb(22 119 255);
    font-weight: 500;
  }

  .settings-nav-item.active::after {
    position: absolute;
    top: 0.5rem;
    right: -1.0625rem;
    bottom: 0.5rem;
    width: 2px;
    border-radius: 9999px;
    background: rgb(22 119 255);
    content: '';
  }

  :global(.dark .system-settings .settings-nav-item) {
    color: rgb(209 213 219);
  }

  :global(.dark .system-settings .settings-nav-item:hover) {
    background: rgb(30 41 59 / 0.6);
    color: rgb(96 165 250);
  }

  :global(.dark .system-settings .settings-nav-item.active) {
    background: rgb(30 58 138 / 0.3);
    color: rgb(96 165 250);
  }

  .settings-content,
  .settings-category {
    min-width: 0;
  }

  .settings-category {
    scroll-margin-top: 1rem;
  }

  .settings-category + .settings-category {
    margin-top: 2rem;
    border-top: 1px solid rgb(229 231 235);
    padding-top: 2rem;
  }

  :global(.dark .system-settings .settings-category + .settings-category) {
    border-color: rgb(51 65 85);
  }

  .settings-heading {
    margin-bottom: 1.5rem;
  }

  .settings-heading h2 {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    color: rgb(31 41 55);
    font-size: 1.25rem;
    font-weight: 700;
    line-height: 1.75rem;
  }

  :global(.dark .system-settings .settings-heading h2) {
    color: rgb(243 244 246);
  }

  .settings-heading p {
    margin-top: 0.375rem;
    color: rgb(107 114 128);
    font-size: 0.875rem;
    line-height: 1.5rem;
  }

  :global(.dark .system-settings .settings-heading p) {
    color: rgb(156 163 175);
  }

  .setting-section {
    min-width: 0;
    border: 1px solid rgb(229 231 235);
    border-radius: 0.75rem;
    background: rgb(255 255 255);
    padding: 1rem;
  }

  :global(.dark .system-settings .setting-section) {
    border-color: rgb(51 65 85);
    background: rgb(15 23 42 / 0.4);
  }

  .setting-row {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .setting-title {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    color: rgb(31 41 55);
    font-size: 0.875rem;
    font-weight: 700;
    line-height: 1.25rem;
  }

  :global(.dark .system-settings .setting-title) {
    color: rgb(229 231 235);
  }

  .setting-description {
    margin-top: 0.375rem;
    color: rgb(107 114 128);
    font-size: 0.75rem;
    line-height: 1.25rem;
  }

  :global(.dark .system-settings .setting-description) {
    color: rgb(156 163 175);
  }

  .animate-fade-in {
    animation: fadeIn 0.3s ease-in-out;
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: translateY(-10px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  @media (min-width: 640px) {
    .setting-section {
      padding: 1.25rem;
    }

    .setting-row {
      flex-direction: row;
      align-items: center;
      justify-content: space-between;
    }
  }

  @media (max-width: 767px) {
    .settings-shell :deep(.ant-card-body) {
      padding: 1rem;
    }

    .settings-layout {
      display: block;
    }

    .settings-nav {
      position: sticky;
      top: -1rem;
      z-index: 10;
      flex-direction: row;
      gap: 0;
      margin: -0.25rem -0.25rem 1.25rem;
      border-right: 0;
      border-bottom: 1px solid rgb(229 231 235);
      background: rgb(255 255 255);
      padding: 0.25rem 0.25rem 0.5rem;
      overflow-x: auto;
      scrollbar-width: none;
    }

    :global(.dark .system-settings .settings-nav) {
      background: rgb(15 23 42);
    }

    .settings-nav::-webkit-scrollbar {
      display: none;
    }

    .settings-nav-item {
      flex: 0 0 auto;
      width: auto;
      padding: 0.75rem;
    }

    .settings-nav-item.active::after {
      top: auto;
      right: 0.75rem;
      bottom: -0.5625rem;
      left: 0.75rem;
      width: auto;
      height: 2px;
    }

    .settings-category {
      scroll-margin-top: 4.5rem;
    }

    .settings-category + .settings-category {
      margin-top: 1.5rem;
      padding-top: 1.5rem;
    }

    .settings-heading {
      margin-bottom: 1rem;
    }
  }
</style>
