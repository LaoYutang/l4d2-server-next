<script setup lang="ts">
  import { computed, onMounted, onUnmounted, reactive, ref } from 'vue';
  import { message, Modal } from 'ant-design-vue';
  import { CopyOutlined, DeleteOutlined, LockOutlined, PlusOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons-vue';
  import { api, type AuthCodeCreated, type AuthCodeItem, type AuthCodeListParams, type AuthCodeStatus, type SelfServiceStatus, type TempAccessType } from '../services/api';
  import { copyToClipboard } from '../utils/clipboard';

  const records = ref<AuthCodeItem[]>([]);
  const loading = ref(false);
  const loadError = ref('');
  const total = ref(0);
  const cleanupCount = ref(0);
  const counts = ref({ all: 0, active: 0, expired: 0, revoked: 0 });
  const filters = reactive<AuthCodeListParams>({ page: 1, page_size: 20, status: '', source: '', keyword: '' });
  const selfService = ref<SelfServiceStatus | null>(null);
  const settingSelfService = ref(false);
  const clock = ref(Date.now());
  const serverOffset = ref(0);
  const cooldownUntil = ref(0);
  const remainingSeconds = computed(() => Math.max(0, Math.ceil((cooldownUntil.value - clock.value) / 1000)));
  const cooldownLabel = computed(() => `${Math.floor(remainingSeconds.value / 60)} 分 ${remainingSeconds.value % 60} 秒`);
  const statusLabels: Record<AuthCodeStatus, string> = { active: '有效', expired: '已过期', revoked: '已撤销' };
  const statusColors = { active: 'success', expired: 'default', revoked: 'error' };
  const summary = computed(() => [
    { label: '全部授权', value: counts.value.all },
    { label: '有效', value: counts.value.active },
    { label: '已过期', value: counts.value.expired },
    { label: '已撤销', value: counts.value.revoked },
  ]);
  const accessLabel = (type: TempAccessType) => type === 'map_upload_only' ? '仅地图上传' : '临时权限';
  const formatTime = (value: string | null) => value ? new Date(value).toLocaleString() : '—';
  const statusOf = (record: AuthCodeItem): AuthCodeStatus => record.revoked_at ? 'revoked' : new Date(record.expires_at).getTime() <= clock.value + serverOffset.value ? 'expired' : 'active';
  const localTime = (date: Date) => new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 19);
  const toUTC = (value: string) => {
    const date = new Date(value);
    if (!value || Number.isNaN(date.getTime())) throw new Error('请选择有效的到期时间');
    return date.toISOString();
  };
  const errorMessage = (error: unknown) => error instanceof Error ? error.message : String(error);

  let disposed = false;
  let listSequence = 0;
  let selfSequence = 0;
  let tickTimer: number | undefined;
  let refreshTimer: number | undefined;
  const loadRecords = async () => {
    const sequence = ++listSequence;
    loading.value = true;
    try {
      const result = await api.getAuthCodes({ ...filters, keyword: filters.keyword.trim() });
      if (disposed || sequence !== listSequence) return;
      const lastPage = Math.max(1, Math.ceil(result.total / result.page_size));
      if (result.page > lastPage) {
        filters.page = lastPage;
        await loadRecords();
        return;
      }
      records.value = result.items;
      total.value = result.total;
      counts.value = result.counts;
      cleanupCount.value = result.cleanup_count;
      serverOffset.value = new Date(result.server_time).getTime() - Date.now();
      loadError.value = '';
    } catch (error) {
      if (!disposed && sequence === listSequence) loadError.value = errorMessage(error);
    } finally {
      if (!disposed && sequence === listSequence) loading.value = false;
    }
  };
  const loadSelfService = async () => {
    const sequence = ++selfSequence;
    try {
      const status = await api.getSelfServiceStatus();
      if (disposed || sequence !== selfSequence) return;
      selfService.value = status;
      cooldownUntil.value = Date.now() + status.remaining_seconds * 1000;
    } catch {
      if (!disposed && sequence === selfSequence) selfService.value = null;
    }
  };
  const refresh = () => Promise.all([loadRecords(), loadSelfService()]);
  const search = () => { filters.page = 1; void loadRecords(); };
  const changePage = (page: number, size: number) => {
    filters.page = page;
    filters.page_size = size;
    void loadRecords();
  };
  const toggleSelfService = async (checked: boolean | string | number) => {
    settingSelfService.value = true;
    ++selfSequence;
    try {
      await api.setSelfServiceConfig(Boolean(checked));
      message.success(checked ? '已开启自助授权' : '已关闭自助授权');
    } catch (error) {
      message.error(errorMessage(error));
    } finally {
      await loadSelfService();
      settingSelfService.value = false;
    }
  };

  const createOpen = ref(false);
  const creating = ref(false);
  const createForm = reactive({ mode: 'random', code: '', remark: '', access_type: 'temporary' as TempAccessType, expires_at: '' });
  const durationOptions = [{ label: '1 小时', hours: 1 }, { label: '1 天', hours: 24 }, { label: '7 天', hours: 168 }, { label: '30 天', hours: 720 }];
  const expiryAfter = (hours: number) => localTime(new Date(Date.now() + serverOffset.value + hours * 3600000));
  const openCreate = () => {
    Object.assign(createForm, { mode: 'random', code: '', remark: '', access_type: 'temporary', expires_at: expiryAfter(1) });
    createOpen.value = true;
  };
  const result = ref<AuthCodeCreated | null>(null);
  const resultOpen = ref(false);
  const createCode = async () => {
    if (creating.value) return;
    try {
      if (createForm.mode === 'custom' && !/^[\x21-\x7e]{8,32}$/.test(createForm.code)) throw new Error('自定义授权码需为 8–32 位可见英文字符，不能包含空格');
      const expires_at = toUTC(createForm.expires_at);
      if (new Date(expires_at).getTime() <= Date.now() + serverOffset.value) throw new Error('新授权码的到期时间必须晚于当前时间');
      creating.value = true;
      result.value = await api.createAuthCode({ code: createForm.mode === 'custom' ? createForm.code : undefined, remark: createForm.remark, access_type: createForm.access_type, expires_at });
      createForm.code = '';
      createOpen.value = false;
      resultOpen.value = true;
      await loadRecords();
    } catch (error) {
      message.error(errorMessage(error));
    } finally {
      creating.value = false;
    }
  };
  const copyCode = async () => {
    if (!result.value) return;
    if (await copyToClipboard(result.value.code)) message.success('授权码已复制');
    else message.warning('无法自动复制，请选中授权码手动复制');
  };

  const editOpen = ref(false);
  const saving = ref(false);
  const selected = ref<AuthCodeItem | null>(null);
  const editForm = reactive({ remark: '', expires_at: '' });
  const openEdit = (record: AuthCodeItem) => {
    selected.value = record;
    editForm.remark = record.remark;
    editForm.expires_at = localTime(new Date(record.expires_at));
    editOpen.value = true;
  };
  const saveEdit = async () => {
    if (!selected.value || saving.value) return;
    try {
      const expires_at = toUTC(editForm.expires_at);
      saving.value = true;
      await api.updateAuthCode({ id: selected.value.id, remark: editForm.remark, expires_at });
      editOpen.value = false;
      message.success('授权信息已更新');
      await loadRecords();
    } catch (error) {
      message.error(errorMessage(error));
    } finally {
      saving.value = false;
    }
  };
  const busy = ref(false);
  const confirmMutation = (title: string, content: string, action: () => Promise<string>) => {
    Modal.confirm({
      title, content, wrapClassName: 'auth-codes-modal', okText: '确认', cancelText: '取消', okButtonProps: { danger: true },
      async onOk() {
        busy.value = true;
        try {
          message.success(await action());
          await loadRecords();
        } catch (error) {
          message.error(errorMessage(error));
          throw error;
        } finally { busy.value = false; }
      },
    });
  };
  const revoke = (record: AuthCodeItem) => confirmMutation('撤销授权', `撤销「${record.remark || record.masked_code}」后将立即失效，记录会保留。撤销后调整到期时间也不会恢复授权。`, async () => { await api.revokeAuthCode(record.id); return '授权已撤销'; });
  const remove = (record: AuthCodeItem) => confirmMutation('删除授权码', `删除「${record.remark || record.masked_code}」后将立即失效，授权记录会移除，操作审计会保留。`, async () => { await api.deleteAuthCode(record.id); return '授权码已删除'; });
  const cleanup = () => confirmMutation('清理全部过期授权码', `将删除服务器当前已过期的授权记录（当前约 ${cleanupCount.value} 条），包含已撤销且过期的记录。此操作不受列表筛选限制，审计会保留。`, async () => { const response = await api.cleanupExpiredAuthCodes(); return `已清理 ${response.deleted_count} 条过期授权码`; });

  onMounted(() => {
    void refresh();
    tickTimer = window.setInterval(() => { clock.value = Date.now(); }, 1000);
    refreshTimer = window.setInterval(() => { if (!busy.value) void loadRecords(); if (!settingSelfService.value) void loadSelfService(); }, 30000);
  });
  onUnmounted(() => {
    disposed = true;
    window.clearInterval(tickTimer);
    window.clearInterval(refreshTimer);
    createForm.code = '';
    result.value = null;
    Modal.destroyAll();
  });
</script>

<template>
  <div class="auth-codes-page flex flex-col gap-5">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 class="flex items-center gap-2 text-xl font-semibold"><LockOutlined class="text-blue-500" /> 授权码管理</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">创建和管理访问授权，记录授权时间与登录情况。</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <a-button :loading="loading" @click="refresh"><template #icon><ReloadOutlined /></template>刷新</a-button>
        <a-button danger :disabled="!cleanupCount || busy || !!loadError" @click="cleanup"><template #icon><DeleteOutlined /></template>清理过期</a-button>
        <a-button type="primary" @click="openCreate"><template #icon><PlusOutlined /></template>创建授权码</a-button>
      </div>
    </div>

    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <a-card v-for="item in summary" :key="item.label" size="small" :bordered="false">
        <div class="text-sm text-gray-500 dark:text-gray-400">{{ item.label }}</div>
        <div class="mt-1 text-2xl font-semibold">{{ item.value }}</div>
      </a-card>
    </div>

    <a-card :bordered="false" size="small">
      <div class="flex items-start justify-between gap-4">
        <div>
          <div class="font-medium">登录页自助授权</div>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">开启后，访客可领取有效期为 1 小时的临时授权码，每次领取后全局冷却 1 小时。</p>
          <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
            <template v-if="!selfService">状态暂不可用</template>
            <template v-else-if="remainingSeconds">冷却剩余 {{ cooldownLabel }}</template>
            <template v-else>{{ selfService.enabled ? '当前可以领取' : '自助授权已关闭' }}</template>
            <span v-if="selfService && selfService.last_generated_time && !selfService.last_generated_time.startsWith('0001')"> · 上次领取：{{ formatTime(selfService.last_generated_time) }}</span>
          </p>
        </div>
        <a-switch :checked="selfService?.enabled || false" :disabled="!selfService" :loading="settingSelfService" checked-children="开" un-checked-children="关" @update:checked="toggleSelfService" />
      </div>
    </a-card>

    <a-card :bordered="false">
      <div class="mb-5 grid gap-3 sm:grid-cols-[minmax(0,1fr)_140px_140px_auto]">
        <a-input v-model:value="filters.keyword" placeholder="搜索备注或授权 ID" allow-clear @press-enter="search" />
        <a-select v-model:value="filters.status" aria-label="状态" @change="search">
          <a-select-option value="">全部状态</a-select-option>
          <a-select-option v-for="(label, value) in statusLabels" :key="value" :value="value">{{ label }}</a-select-option>
        </a-select>
        <a-select v-model:value="filters.source" aria-label="来源" @change="search">
          <a-select-option value="">全部来源</a-select-option>
          <a-select-option value="manual">管理员创建</a-select-option>
          <a-select-option value="self_service">自助领取</a-select-option>
        </a-select>
        <a-button :loading="loading" @click="search"><template #icon><SearchOutlined /></template>搜索</a-button>
      </div>
      <a-alert v-if="loadError" type="error" show-icon :message="loadError" class="mb-4" />
      <a-spin :spinning="loading">
        <div v-if="records.length" class="hidden overflow-x-auto lg:block">
          <table class="w-full text-left text-sm">
            <thead class="text-gray-500 dark:text-gray-400"><tr class="border-b border-gray-100 dark:border-gray-700"><th class="p-3">授权码 / 备注</th><th class="p-3">权限 / 来源</th><th class="p-3">状态</th><th class="p-3">授权时间 / 到期时间</th><th class="p-3">登录情况</th><th class="p-3">操作</th></tr></thead>
            <tbody>
              <tr v-for="record in records" :key="record.id" class="border-b border-gray-100 align-top dark:border-gray-700">
                <td class="p-3"><div class="font-mono">{{ record.masked_code }}</div><div class="mt-1 max-w-56 break-words">{{ record.remark || '无备注' }}</div><div class="mt-1 text-xs text-gray-400 font-mono break-all" :title="record.id">{{ record.id }}</div></td>
                <td class="p-3 whitespace-nowrap"><div>{{ accessLabel(record.access_type) }}</div><div class="mt-1 text-xs text-gray-500">{{ record.source === 'self_service' ? '自助领取' : '管理员创建' }}</div></td>
                <td class="p-3 whitespace-nowrap"><a-tag :color="statusColors[statusOf(record)]">{{ statusLabels[statusOf(record)] }}</a-tag><div v-if="record.revoked_at" class="mt-1 text-xs text-gray-500">{{ formatTime(record.revoked_at) }}</div></td>
                <td class="p-3 whitespace-nowrap"><div>{{ formatTime(record.created_at) }}</div><div class="mt-1 text-gray-500">至 {{ formatTime(record.expires_at) }}</div></td>
                <td class="p-3 whitespace-nowrap text-xs"><div>登录 {{ record.login_count }} 次</div><div class="mt-1">首次：{{ formatTime(record.first_login_at) }}</div><div class="mt-1">最近：{{ formatTime(record.last_login_at) }}</div><div class="mt-1 font-mono text-gray-500">{{ record.last_login_ip || '—' }}</div></td>
                <td class="p-3"><div class="flex flex-wrap gap-2 min-w-40"><a-button size="small" @click="openEdit(record)">编辑</a-button><a-button size="small" danger :disabled="busy || !!record.revoked_at" @click="revoke(record)">撤销</a-button><a-button size="small" danger :disabled="busy" @click="remove(record)">删除</a-button></div></td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="records.length" class="space-y-3 lg:hidden">
          <div v-for="record in records" :key="record.id" class="rounded-lg border border-gray-200 p-4 dark:border-gray-700">
            <div class="flex items-start justify-between gap-3"><div class="min-w-0"><div class="font-mono">{{ record.masked_code }}</div><div class="mt-1 break-words">{{ record.remark || '无备注' }}</div></div><a-tag :color="statusColors[statusOf(record)]">{{ statusLabels[statusOf(record)] }}</a-tag></div>
            <div class="mt-2 text-xs text-gray-400 font-mono break-all">{{ record.id }}</div>
            <div class="mt-3 space-y-1 text-xs text-gray-500 dark:text-gray-400">
              <div>{{ accessLabel(record.access_type) }} · {{ record.source === 'self_service' ? '自助领取' : '管理员创建' }}</div>
              <div>授权：{{ formatTime(record.created_at) }}</div><div>到期：{{ formatTime(record.expires_at) }}</div>
              <div v-if="record.revoked_at">撤销：{{ formatTime(record.revoked_at) }}</div>
              <div>登录 {{ record.login_count }} 次 · IP：{{ record.last_login_ip || '—' }}</div>
              <div>首次：{{ formatTime(record.first_login_at) }}</div><div>最近：{{ formatTime(record.last_login_at) }}</div>
            </div>
            <div class="mt-4 flex gap-2"><a-button size="small" @click="openEdit(record)">编辑</a-button><a-button size="small" danger :disabled="busy || !!record.revoked_at" @click="revoke(record)">撤销</a-button><a-button size="small" danger :disabled="busy" @click="remove(record)">删除</a-button></div>
          </div>
        </div>
        <a-empty v-if="!records.length" description="暂无授权记录" />
      </a-spin>
      <div class="mt-5 flex justify-end"><a-pagination :current="filters.page" :page-size="filters.page_size" :total="total" :show-size-changer="true" :page-size-options="['10', '20', '50', '100']" :show-total="(n: number) => `共 ${n} 条`" @change="changePage" /></div>
    </a-card>

    <a-modal v-model:open="createOpen" wrap-class-name="auth-codes-modal" title="创建授权码" :confirm-loading="creating" :cancel-button-props="{ disabled: creating }" :closable="!creating" :mask-closable="false" ok-text="创建" cancel-text="取消" @ok="createCode" @after-close="createForm.code = ''">
      <div class="space-y-4 py-2">
        <div><label class="field-label">生成方式</label><a-radio-group v-model:value="createForm.mode"><a-radio value="random">随机生成 32 位</a-radio><a-radio value="custom">自定义</a-radio></a-radio-group></div>
        <div v-if="createForm.mode === 'custom'"><label for="custom-auth-code" class="field-label">自定义授权码</label><a-input id="custom-auth-code" v-model:value="createForm.code" :maxlength="32" autocomplete="off" placeholder="8–32 位，区分大小写" /><p class="field-help">支持英文字母、数字和可见英文符号，不含空格，不能与现有授权码或管理员密码重复。</p></div>
        <div><label for="auth-remark" class="field-label">备注</label><a-input id="auth-remark" v-model:value="createForm.remark" :maxlength="200" placeholder="可选，如使用人或用途" /></div>
        <div><label class="field-label">权限类型</label><a-select v-model:value="createForm.access_type" class="w-full"><a-select-option value="temporary">临时权限</a-select-option><a-select-option value="map_upload_only">仅地图上传</a-select-option></a-select><p class="field-help">临时权限沿用访客权限；仅地图上传只开放上传页面。</p></div>
        <div><label for="auth-expiry" class="field-label">到期时间（本地时间）</label><a-input id="auth-expiry" v-model:value="createForm.expires_at" type="datetime-local" step="1" /><div class="mt-2 flex flex-wrap gap-2"><a-button v-for="duration in durationOptions" :key="duration.hours" size="small" @click="createForm.expires_at = expiryAfter(duration.hours)">{{ duration.label }}</a-button></div><p class="field-help">可指定任意未来日期。</p></div>
      </div>
    </a-modal>

    <a-modal v-model:open="resultOpen" wrap-class-name="auth-codes-modal" title="授权码创建成功" :footer="null" :mask-closable="false" @after-close="result = null">
      <div v-if="result" class="space-y-4 py-2">
        <a-alert type="warning" show-icon message="完整授权码仅在此显示，请复制保存。关闭后无法再次查看。" />
        <a-input :value="result.code" readonly class="font-mono" aria-label="新授权码" @focus="($event.target as HTMLInputElement).select()" />
        <div class="text-sm text-gray-500">{{ accessLabel(result.access_type) }} · 到期：{{ formatTime(result.expires_at) }}</div>
        <div class="flex justify-end gap-2"><a-button @click="resultOpen = false">关闭</a-button><a-button type="primary" @click="copyCode"><template #icon><CopyOutlined /></template>复制授权码</a-button></div>
      </div>
    </a-modal>

    <a-modal v-model:open="editOpen" wrap-class-name="auth-codes-modal" title="编辑授权信息" :confirm-loading="saving" :cancel-button-props="{ disabled: saving }" :closable="!saving" :mask-closable="false" ok-text="保存" cancel-text="取消" @ok="saveEdit" @after-close="selected = null">
      <div class="space-y-4 py-2">
        <div v-if="selected" class="font-mono text-gray-500">{{ selected.masked_code }}</div>
        <a-alert v-if="selected?.revoked_at" type="warning" show-icon message="此授权已撤销。更新到期时间不会恢复授权。" />
        <div><label for="edit-auth-remark" class="field-label">备注</label><a-input id="edit-auth-remark" v-model:value="editForm.remark" :maxlength="200" /></div>
        <div><label for="edit-auth-expiry" class="field-label">到期时间（本地时间）</label><a-input id="edit-auth-expiry" v-model:value="editForm.expires_at" type="datetime-local" step="1" /><div class="mt-2 flex flex-wrap gap-2"><a-button v-for="duration in durationOptions" :key="duration.hours" size="small" @click="editForm.expires_at = expiryAfter(duration.hours)">从现在起 {{ duration.label }}</a-button></div><p class="field-help">可延长或缩短期限。已过期且未撤销的授权码，延长到未来后可继续使用原码。</p></div>
      </div>
    </a-modal>
  </div>
</template>

<style scoped>
  .auth-codes-page :deep(.ant-btn),
  :global(.auth-codes-modal .ant-btn) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .auth-codes-page :deep(.ant-btn .anticon),
  .auth-codes-page :deep(.ant-btn .ant-btn-loading-icon),
  :global(.auth-codes-modal .ant-btn .anticon),
  :global(.auth-codes-modal .ant-btn .ant-btn-loading-icon) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    vertical-align: 0;
  }

  .auth-codes-page :deep(.ant-btn .anticon > svg),
  :global(.auth-codes-modal .ant-btn .anticon > svg) {
    display: block;
  }

  .field-label { display: block; margin-bottom: 6px; font-size: 14px; font-weight: 500; }
  .field-help { margin-top: 6px; font-size: 12px; color: #6b7280; }
</style>
