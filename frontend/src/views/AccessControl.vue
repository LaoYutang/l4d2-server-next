<script setup lang="ts">
  import { computed, nextTick, onMounted, ref, watch } from 'vue';
  import { message } from 'ant-design-vue';
  import {
    DeleteOutlined,
    PlusOutlined,
    ReloadOutlined,
    SafetyCertificateOutlined,
    SaveOutlined,
    SecurityScanOutlined,
    ExperimentOutlined,
    StopOutlined,
  } from '@ant-design/icons-vue';
  import AccessRuleList from '../components/AccessRuleList.vue';
  import GameBlacklistTab from '../components/GameBlacklistTab.vue';
  import {
    api,
    ApiRequestError,
    type AccessControlPreviewResponse,
    type AccessControlRule,
    type AccessControlStateResponse,
    type AccessDecision,
  } from '../services/api';

  const activeTab = ref('panel-rules');
  const tabsRef = ref<{ $el?: HTMLElement } | null>(null);
  const gameBlacklistActivated = ref(false);
  const loading = ref(true);
  const savingRules = ref(false);
  const savingProxies = ref(false);
  const previewingRules = ref(false);
  const revisionConflict = ref(false);

  const revision = ref(0);
  const enabled = ref(false);
  const panelBlacklist = ref<AccessControlRule[]>([]);
  const panelWhitelist = ref<AccessControlRule[]>([]);
  const trustedProxies = ref<string[]>([]);
  const recoveryMode = ref(false);
  const loadError = ref('');
  const panelPreview = ref<AccessControlPreviewResponse | null>(null);
  const testIP = ref('');

  const rulesBaseline = ref('');
  const proxiesBaseline = ref('');

  const cloneRules = (rules: AccessControlRule[]) => rules.map((rule) => ({ ...rule }));

  const serializeRules = () =>
    JSON.stringify({
      enabled: enabled.value,
      panel_blacklist: panelBlacklist.value,
      panel_whitelist: panelWhitelist.value,
    });

  const serializeProxies = () => JSON.stringify(trustedProxies.value);

  const rulesDirty = computed(() => rulesBaseline.value !== '' && serializeRules() !== rulesBaseline.value);
  const proxiesDirty = computed(
    () => proxiesBaseline.value !== '' && serializeProxies() !== proxiesBaseline.value
  );

  const applyState = (
    data: AccessControlStateResponse,
    preserveDraft?: 'panel_rules' | 'trusted_proxies'
  ) => {
    revision.value = data.config.revision;
    recoveryMode.value = data.recovery_mode;
    loadError.value = data.load_error || '';
    revisionConflict.value = false;

    if (preserveDraft !== 'panel_rules') {
      enabled.value = data.config.enabled;
      panelBlacklist.value = cloneRules(data.config.panel_blacklist || []);
      panelWhitelist.value = cloneRules(data.config.panel_whitelist || []);
      rulesBaseline.value = serializeRules();
      panelPreview.value = null;
    }
    if (preserveDraft !== 'trusted_proxies') {
      trustedProxies.value = [...(data.config.trusted_proxies || [])];
      proxiesBaseline.value = serializeProxies();
    }
  };

  const loadConfig = async () => {
    loading.value = true;
    try {
      applyState(await api.getAccessControlConfig());
    } catch (error: any) {
      message.error(`加载访问控制配置失败：${error.message || error}`);
    } finally {
      loading.value = false;
    }
  };

  const handleRequestError = (error: unknown) => {
    if (error instanceof ApiRequestError && error.code === 'revision_conflict') {
      revisionConflict.value = true;
      message.warning(error.message);
      return;
    }
    message.error(error instanceof Error ? error.message : String(error));
  };

  const savePanelRules = async () => {
    savingRules.value = true;
    try {
      const data = await api.updatePanelAccessRules({
        expected_revision: revision.value,
        enabled: enabled.value,
        panel_blacklist: panelBlacklist.value,
        panel_whitelist: panelWhitelist.value,
      });
      applyState(data, proxiesDirty.value ? 'trusted_proxies' : undefined);
      message.success('面板黑白名单已保存并生效');
    } catch (error) {
      handleRequestError(error);
    } finally {
      savingRules.value = false;
    }
  };

  const saveTrustedProxies = async () => {
    savingProxies.value = true;
    try {
      const data = await api.updateAccessControlTrustedProxies({
        expected_revision: revision.value,
        trusted_proxies: trustedProxies.value,
      });
      applyState(data, rulesDirty.value ? 'panel_rules' : undefined);
      message.success('可信代理已保存并生效');
    } catch (error) {
      handleRequestError(error);
    } finally {
      savingProxies.value = false;
    }
  };

  const previewPanelRules = async () => {
    previewingRules.value = true;
    try {
      panelPreview.value = await api.previewAccessControl({
        section: 'panel_rules',
        enabled: enabled.value,
        panel_blacklist: panelBlacklist.value,
        panel_whitelist: panelWhitelist.value,
        test_ip: testIP.value.trim(),
      });
    } catch (error) {
      handleRequestError(error);
    } finally {
      previewingRules.value = false;
    }
  };

  const addTrustedProxy = () => {
    trustedProxies.value = [...trustedProxies.value, ''];
  };

  const updateTrustedProxy = (index: number, value: string) => {
    trustedProxies.value = trustedProxies.value.map((entry, currentIndex) =>
      currentIndex === index ? value : entry
    );
  };

  const removeTrustedProxy = (index: number) => {
    trustedProxies.value = trustedProxies.value.filter((_, currentIndex) => currentIndex !== index);
  };

  const resetRules = async () => {
    loading.value = true;
    try {
      const data = await api.getAccessControlConfig();
      applyState(data, proxiesDirty.value ? 'trusted_proxies' : undefined);
    } catch (error: any) {
      message.error(`刷新面板规则失败：${error.message || error}`);
    } finally {
      loading.value = false;
    }
  };

  const resetProxies = async () => {
    loading.value = true;
    try {
      const data = await api.getAccessControlConfig();
      applyState(data, rulesDirty.value ? 'panel_rules' : undefined);
    } catch (error: any) {
      message.error(`刷新可信代理失败：${error.message || error}`);
    } finally {
      loading.value = false;
    }
  };

  const reasonText = (decision?: AccessDecision | null) => {
    if (!decision) return '尚未判断';
    const labels: Record<string, string> = {
      loopback: '本机回环地址，始终允许',
      disabled: '面板黑白名单未启用',
      blacklisted: '命中面板黑名单',
      whitelisted: '命中面板白名单',
      no_whitelist: '面板白名单为空',
      not_whitelisted: '未命中面板白名单',
      geoip_unavailable: 'GeoIP 服务不可用',
      recovery_mode: '配置损坏恢复模式',
      invalid_client_ip: '客户端 IP 无效',
    };
    return labels[decision.reason] || decision.reason;
  };

  const decisionAlertType = (decision?: AccessDecision | null) => {
    if (!decision) return 'info';
    return decision.allowed ? 'success' : decision.reason === 'geoip_unavailable' ? 'warning' : 'error';
  };

  watch(activeTab, (tab) => {
    if (tab === 'game-blacklist') {
      gameBlacklistActivated.value = true;
    }
    nextTick(() => {
      const activeElement = tabsRef.value?.$el?.querySelector<HTMLElement>('.ant-tabs-tab-active');
      activeElement?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    });
  });

  onMounted(loadConfig);
</script>

<template>
  <div class="space-y-5">
    <header>
      <h1 class="flex items-center gap-2 text-2xl font-bold text-slate-900 dark:text-slate-100">
        <SecurityScanOutlined />
        访问控制
      </h1>
    </header>

    <a-alert
      v-if="recoveryMode"
      type="error"
      show-icon
      message="访问控制处于本机恢复模式"
      :description="loadError || '配置文件无法读取，仅允许本机回环地址访问。保存有效配置后可恢复。'"
    />

    <a-alert v-if="revisionConflict" type="warning" show-icon>
      <template #message>配置已由其他管理员修改</template>
      <template #description>
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <span>请刷新服务端配置后重新编辑，避免覆盖其他管理员的改动。</span>
          <a-button size="small" @click="loadConfig">
            <template #icon><ReloadOutlined /></template>
            刷新配置
          </a-button>
        </div>
      </template>
    </a-alert>

    <a-spin :spinning="loading">
      <a-card :bordered="false" class="access-control-card shadow-lg dark:shadow-slate-950/40">
        <a-tabs ref="tabsRef" v-model:activeKey="activeTab" class="access-control-tabs">
          <a-tab-pane key="panel-rules">
            <template #tab>
              <span class="whitespace-nowrap"><SafetyCertificateOutlined /> 面板黑白名单</span>
            </template>

            <div class="space-y-5 pt-1">
              <section
                class="flex items-center justify-between gap-4 rounded-xl border border-slate-200 px-4 py-3 dark:border-slate-700"
              >
                <div class="font-semibold text-slate-900 dark:text-slate-100">启用面板黑白名单</div>
                <a-switch
                  :checked="enabled"
                  :disabled="savingRules"
                  checked-children="启用"
                  un-checked-children="关闭"
                  @update:checked="enabled = Boolean($event)"
                />
              </section>

              <AccessRuleList
                v-model="panelBlacklist"
                title="面板黑名单"
                description="任意规则命中后立即拒绝，优先级高于面板白名单。"
                :disabled="savingRules"
              />

              <AccessRuleList
                v-model="panelWhitelist"
                title="面板白名单"
                description="存在启用规则时，客户端必须至少命中一条才能访问。"
                :disabled="savingRules"
              />

              <section class="rounded-xl border border-slate-200 p-4 dark:border-slate-700">
                <div class="mb-3 flex items-center gap-2 font-semibold text-slate-900 dark:text-slate-100">
                  <ExperimentOutlined class="text-slate-500 dark:text-slate-400" />
                  规则预览
                </div>
                <div class="flex flex-col gap-3 sm:flex-row">
                  <a-input
                    v-model:value="testIP"
                    class="font-mono"
                    placeholder="可选：输入要测试的 IPv4 或 IPv6"
                    @pressEnter="previewPanelRules"
                  />
                  <a-button :loading="previewingRules" @click="previewPanelRules">预览规则</a-button>
                </div>

                <div v-if="panelPreview" class="mt-4 grid gap-3 lg:grid-cols-2">
                  <a-alert
                    show-icon
                    :type="decisionAlertType(panelPreview.current_decision)"
                    :message="`当前管理员：${panelPreview.current_decision.allowed ? '允许' : '拒绝'}`"
                    :description="reasonText(panelPreview.current_decision)"
                  />
                  <a-alert
                    v-if="panelPreview.test_decision"
                    show-icon
                    :type="decisionAlertType(panelPreview.test_decision)"
                    :message="`测试 IP：${panelPreview.test_decision.allowed ? '允许' : '拒绝'}`"
                    :description="reasonText(panelPreview.test_decision)"
                  />
                </div>
              </section>

              <div
                class="flex flex-col-reverse gap-3 border-t border-slate-200 pt-4 dark:border-slate-700 sm:flex-row sm:justify-end"
              >
                <a-button :disabled="!rulesDirty || savingRules" @click="resetRules">放弃修改</a-button>
                <a-button
                  type="primary"
                  class="!inline-flex !items-center !justify-center"
                  :loading="savingRules"
                  :disabled="!rulesDirty || revisionConflict"
                  @click="savePanelRules"
                >
                  <template #icon><SaveOutlined /></template>
                  保存
                </a-button>
              </div>
            </div>
          </a-tab-pane>

          <a-tab-pane key="trusted-proxies">
            <template #tab>
              <span class="whitespace-nowrap"><SecurityScanOutlined /> 可信代理</span>
            </template>

            <div class="space-y-5 pt-1">
              <section class="overflow-hidden rounded-xl border border-slate-200 dark:border-slate-700">
                <header
                  class="flex flex-col gap-3 border-b border-slate-200 bg-slate-50/80 px-4 py-4 dark:border-slate-700 dark:bg-slate-800/50 sm:flex-row sm:items-center sm:justify-between"
                >
                  <div>
                    <h3 class="font-semibold text-slate-900 dark:text-slate-100">可信代理 IP / CIDR</h3>
                  </div>
                  <a-button
                    class="!inline-flex !items-center !justify-center self-start sm:self-auto"
                    :disabled="savingProxies"
                    @click="addTrustedProxy"
                  >
                    <template #icon><PlusOutlined /></template>
                    添加代理
                  </a-button>
                </header>

                <div
                  v-if="trustedProxies.length === 0"
                  class="px-4 py-10 text-center text-sm text-slate-500 dark:text-slate-400"
                >
                  当前不信任任何代理，所有转发头都会被忽略。
                </div>
                <div v-else class="space-y-3 p-4">
                  <div
                    v-for="(proxy, index) in trustedProxies"
                    :key="index"
                    class="flex items-center gap-2"
                  >
                    <a-input
                      :value="proxy"
                      class="font-mono"
                      placeholder="例如：127.0.0.0/8 或 172.18.0.5"
                      :disabled="savingProxies"
                      @update:value="updateTrustedProxy(index, String($event))"
                    />
                    <a-button
                      danger
                      type="text"
                      :disabled="savingProxies"
                      aria-label="删除可信代理"
                      @click="removeTrustedProxy(index)"
                    >
                      <template #icon><DeleteOutlined /></template>
                    </a-button>
                  </div>
                </div>
              </section>

              <div
                class="flex flex-col-reverse gap-3 border-t border-slate-200 pt-4 dark:border-slate-700 sm:flex-row sm:justify-end"
              >
                <a-button :disabled="!proxiesDirty || savingProxies" @click="resetProxies">放弃修改</a-button>
                <a-button
                  type="primary"
                  class="!inline-flex !items-center !justify-center"
                  :loading="savingProxies"
                  :disabled="!proxiesDirty || revisionConflict"
                  @click="saveTrustedProxies"
                >
                  <template #icon><SaveOutlined /></template>
                  保存
                </a-button>
              </div>
            </div>
          </a-tab-pane>

          <a-tab-pane key="game-blacklist">
            <template #tab>
              <span class="whitespace-nowrap"><StopOutlined /> 游戏黑名单</span>
            </template>

            <GameBlacklistTab v-if="gameBlacklistActivated" />
          </a-tab-pane>
        </a-tabs>
      </a-card>
    </a-spin>
  </div>
</template>

<style scoped>
  .access-control-tabs :deep(.ant-tabs-nav-wrap) {
    overflow-x: auto;
    scrollbar-width: thin;
  }

  .access-control-tabs :deep(.ant-tabs-tab) {
    flex-shrink: 0;
  }

  @media (max-width: 640px) {
    .access-control-card :deep(.ant-card-body) {
      padding: 16px;
    }

    .access-control-tabs :deep(.ant-tabs-nav-wrap) {
      overflow-x: auto !important;
      scrollbar-width: none;
    }

    .access-control-tabs :deep(.ant-tabs-nav-wrap::-webkit-scrollbar) {
      display: none;
    }

    .access-control-tabs :deep(.ant-tabs-nav-list) {
      width: max-content;
      transform: none !important;
    }

    .access-control-tabs :deep(.ant-tabs-nav-operations) {
      display: none !important;
    }
  }
</style>
