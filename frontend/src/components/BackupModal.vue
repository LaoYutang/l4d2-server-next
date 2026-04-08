<script setup lang="ts">
  import { ref, watch, computed } from 'vue';
  import {
    message,
    Modal as AModal,
    Table as ATable,
    Button as AButton,
    Input as AInput,
    Popconfirm as APopconfirm,
    Alert as AAlert,
    Descriptions as ADescriptions,
    DescriptionsItem as ADescriptionsItem,
  } from 'ant-design-vue';
  import {
    PlusOutlined,
    UndoOutlined,
    EditOutlined,
    DeleteOutlined,
    SaveOutlined,
    CloseOutlined,
    DownloadOutlined,
    UploadOutlined,
    ExportOutlined,
  } from '@ant-design/icons-vue';
  import { api } from '../services/api';

  interface BackupInfo {
    name: string;
    created_at: number;
    plugin_count: number;
    admin_count: number;
    has_server_info: boolean;
    has_server_config: boolean;
  }

  interface BackupPluginConfig {
    name: string;
    cvars: Record<string, { value: string; default?: string; description?: string }>;
  }

  interface BackupPlugin {
    name: string;
    configs: BackupPluginConfig[];
  }

  interface BackupAdmin {
    steamid: string;
    remark?: string;
  }

  interface BackupServerInfo {
    hostname?: string;
    motd?: string;
    host?: string;
  }

  interface BackupServerConfig {
    hidden: boolean;
    lobby_connect_only: boolean;
    steam_group?: string;
    custom_config?: string[];
  }

  type DetailType = 'plugins' | 'admins' | 'server_info' | 'server_config';

  const props = defineProps<{
    open: boolean;
  }>();

  const emit = defineEmits<{
    (e: 'update:open', value: boolean): void;
    (e: 'restored'): void;
  }>();

  const loading = ref(false);
  const backups = ref<BackupInfo[]>([]);
  const creating = ref(false);
  const newBackupName = ref('');
  const showCreateInput = ref(false);
  const restoringName = ref('');
  const editingName = ref('');
  const editNewName = ref('');
  const renamingName = ref('');
  const getPopupContainer = () => document.body;

  // Responsive modal widths
  const windowWidth = ref(window.innerWidth);
  const onResize = () => {
    windowWidth.value = window.innerWidth;
  };
  watch(
    () => props.open,
    (val) => {
      if (val) window.addEventListener('resize', onResize);
      else window.removeEventListener('resize', onResize);
    }
  );
  const modalWidth = computed(() => (windowWidth.value < 768 ? '95vw' : 900));
  const detailModalWidth = computed(() => (windowWidth.value < 768 ? '95vw' : 600));
  const cfgModalWidth = computed(() => (windowWidth.value < 768 ? '95vw' : 680));

  const detailOpen = ref(false);
  const detailLoading = ref(false);
  const detailType = ref<DetailType>('plugins');
  const detailName = ref('');
  const detailPlugins = ref<BackupPlugin[]>([]);
  const detailAdmins = ref<BackupAdmin[]>([]);
  const detailServerInfo = ref<BackupServerInfo | null>(null);
  const detailServerConfig = ref<BackupServerConfig | null>(null);

  // Plugin config detail (3rd level)
  const pluginCfgOpen = ref(false);
  const pluginCfgName = ref('');
  const pluginCfgRows = ref<{ file: string; cvar: string; current: string; default: string }[]>([]);

  const pluginCfgColumns = [
    { title: '配置文件', dataIndex: 'file', key: 'file', width: 180 },
    { title: 'Cvar 名称', dataIndex: 'cvar', key: 'cvar' },
    { title: '默认值', dataIndex: 'default', key: 'default', width: 100 },
    { title: '当前值', dataIndex: 'current', key: 'current', width: 100 },
  ];

  const pluginListColumns = [
    { title: '插件名称', dataIndex: 'name', key: 'name' },
    { title: '配置修改', key: 'configs', width: 100 },
  ];

  const openPluginCfg = (plugin: BackupPlugin) => {
    pluginCfgName.value = plugin.name;
    const rows: { file: string; cvar: string; current: string; default: string }[] = [];
    for (const cfg of plugin.configs || []) {
      for (const [cvarName, cvar] of Object.entries(cfg.cvars || {})) {
        rows.push({
          file: cfg.name,
          cvar: cvarName,
          current: cvar.value ?? '',
          default: cvar.default ?? '-',
        });
      }
    }
    pluginCfgRows.value = rows;
    pluginCfgOpen.value = true;
  };

  const detailTitle = {
    plugins: '插件列表',
    admins: '管理员列表',
    server_info: '服务器信息',
    server_config: '服务器配置',
  };

  const adminColumns = [
    { title: 'SteamID', dataIndex: 'steamid', key: 'steamid' },
    { title: '备注', dataIndex: 'remark', key: 'remark' },
  ];

  const openDetail = async (record: BackupInfo, type: DetailType) => {
    if (type === 'plugins' && record.plugin_count === 0) return;
    if (type === 'admins' && (record.admin_count ?? 0) === 0) return;
    if (type === 'server_info' && !record.has_server_info) return;
    if (type === 'server_config' && !record.has_server_config) return;
    detailType.value = type;
    detailName.value = record.name;
    detailPlugins.value = [];
    detailAdmins.value = [];
    detailServerInfo.value = null;
    detailServerConfig.value = null;
    detailOpen.value = true;
    detailLoading.value = true;
    try {
      if (type === 'plugins') {
        detailPlugins.value = await api.getBackupPluginsDetail(record.name);
      } else if (type === 'admins') {
        detailAdmins.value = await api.getBackupAdminsDetail(record.name);
      } else if (type === 'server_config') {
        detailServerConfig.value = await api.getBackupServerConfigDetail(record.name);
      } else {
        detailServerInfo.value = await api.getBackupServerInfoDetail(record.name);
      }
    } catch (error: any) {
      message.error('获取备份详情失败: ' + error.message);
      detailOpen.value = false;
    } finally {
      detailLoading.value = false;
    }
  };

  const fetchBackups = async () => {
    loading.value = true;
    try {
      backups.value = await api.listBackups();
    } catch (error: any) {
      message.error('获取备份列表失败: ' + error.message);
    } finally {
      loading.value = false;
    }
  };

  watch(
    () => props.open,
    (val) => {
      if (val) {
        fetchBackups();
        showCreateInput.value = false;
        newBackupName.value = '';
        editingName.value = '';
      }
    }
  );

  const handleCreate = async () => {
    const name = newBackupName.value.trim();
    if (!name) {
      message.warning('请输入备份名称');
      return;
    }
    creating.value = true;
    try {
      await api.createBackup(name);
      message.success('备份创建成功');
      newBackupName.value = '';
      showCreateInput.value = false;
      fetchBackups();
    } catch (error: any) {
      message.error('创建备份失败: ' + error.message);
    } finally {
      creating.value = false;
    }
  };

  const handleRestore = async (name: string) => {
    restoringName.value = name;
    try {
      const result = await api.restoreBackup(name);
      if (result.skipped && result.skipped.length > 0) {
        message.warning(
          `备份还原成功，但以下 ${result.skipped.length} 个插件已不存在被跳过：${result.skipped.join('、')}`,
          8
        );
      } else {
        message.success('备份还原成功');
      }
      emit('restored');
    } catch (error: any) {
      message.error('还原备份失败: ' + error.message);
    } finally {
      restoringName.value = '';
    }
  };

  const startRename = (record: BackupInfo) => {
    editingName.value = record.name;
    editNewName.value = record.name;
  };

  const cancelRename = () => {
    editingName.value = '';
    editNewName.value = '';
  };

  const handleRename = async () => {
    const newName = editNewName.value.trim();
    if (!newName) {
      message.warning('名称不能为空');
      return;
    }
    if (newName === editingName.value) {
      cancelRename();
      return;
    }
    renamingName.value = editingName.value;
    try {
      await api.renameBackup(editingName.value, newName);
      message.success('重命名成功');
      editingName.value = '';
      editNewName.value = '';
      fetchBackups();
    } catch (error: any) {
      message.error('重命名失败: ' + error.message);
    } finally {
      renamingName.value = '';
    }
  };

  const handleDelete = async (name: string) => {
    try {
      await api.deleteBackup(name);
      message.success('备份已删除');
      fetchBackups();
    } catch (error: any) {
      message.error('删除备份失败: ' + error.message);
    }
  };

  const handleExport = async (name: string) => {
    try {
      await api.exportBackup(name);
    } catch (error: any) {
      message.error('导出备份失败: ' + error.message);
    }
  };

  const handleExportAll = async () => {
    try {
      await api.exportAllBackups();
    } catch (error: any) {
      message.error('导出全部备份失败: ' + error.message);
    }
  };

  const importFileInput = ref<HTMLInputElement | null>(null);
  const importing = ref(false);

  const triggerImport = () => {
    importFileInput.value?.click();
  };

  const handleImportFile = async (event: Event) => {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    importing.value = true;
    try {
      const result = await api.importBackup(file);
      message.success(result.message);
      fetchBackups();
    } catch (error: any) {
      message.error('导入备份失败: ' + error.message);
    } finally {
      importing.value = false;
      input.value = '';
    }
  };

  const formatTime = (timestamp: number) => {
    if (!timestamp) return '-';
    const d = new Date(timestamp * 1000);
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  };

  const columns = [
    {
      title: '备份名称',
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 145,
    },
    {
      title: '插件',
      dataIndex: 'plugin_count',
      key: 'plugin_count',
      width: 55,
    },
    {
      title: '管理员',
      dataIndex: 'admin_count',
      key: 'admin_count',
      width: 60,
    },
    {
      title: '服务器',
      key: 'server_info',
      width: 55,
    },
    {
      title: '配置',
      key: 'server_config',
      width: 55,
    },
    {
      title: '操作',
      key: 'actions',
      width: 200,
    },
  ];
</script>

<template>
  <a-modal
    :open="props.open"
    title="插件备份管理"
    :footer="null"
    :width="modalWidth"
    @cancel="emit('update:open', false)"
  >
    <a-alert
      message="此功能会备份：已启用的插件（仅修改过的配置项）、管理员列表、服务器信息（名称/公告/描述）、服务器配置（隐藏/匹配/组ID/自定义配置）。不会备份插件文件本身。若插件被删除，还原时该插件将被跳过。"
      type="info"
      show-icon
    />

    <div class="mt-4 mb-4 flex items-center gap-2 flex-wrap">
      <template v-if="showCreateInput">
        <a-input
          v-model:value="newBackupName"
          placeholder="输入备份名称"
          class="flex-1"
          allow-clear
          @pressEnter="handleCreate"
        />
        <a-button
          type="primary"
          :loading="creating"
          @click="handleCreate"
          class="!inline-flex !items-center !justify-center"
        >
          <template #icon><SaveOutlined /></template>
          保存
        </a-button>
        <a-button
          class="!inline-flex !items-center !justify-center"
          @click="
            showCreateInput = false;
            newBackupName = '';
          "
        >
          <template #icon><CloseOutlined /></template>
        </a-button>
      </template>
      <template v-else>
        <a-button type="primary" @click="showCreateInput = true">
          <template #icon><PlusOutlined /></template>
          新建备份
        </a-button>
        <a-button :loading="importing" @click="triggerImport">
          <template #icon><UploadOutlined /></template>
          导入
        </a-button>
        <a-button :disabled="backups.length === 0" @click="handleExportAll">
          <template #icon><ExportOutlined /></template>
          全部导出
        </a-button>
        <input
          ref="importFileInput"
          type="file"
          accept=".yaml,.yml"
          class="hidden"
          @change="handleImportFile"
        />
      </template>
    </div>

    <a-table
      :columns="columns"
      :data-source="backups"
      :loading="loading"
      row-key="name"
      :pagination="false"
      size="small"
      :scroll="{ x: 640 }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'name'">
          <template v-if="editingName === record.name">
            <div class="flex items-center gap-1">
              <a-input
                v-model:value="editNewName"
                size="small"
                class="flex-1"
                @pressEnter="handleRename"
              />
              <a-button
                size="small"
                type="primary"
                :loading="renamingName === record.name"
                @click="handleRename"
                class="!inline-flex !items-center !justify-center"
              >
                <template #icon><SaveOutlined /></template>
              </a-button>
              <a-button
                size="small"
                @click="cancelRename"
                class="!inline-flex !items-center !justify-center"
              >
                <template #icon><CloseOutlined /></template>
              </a-button>
            </div>
          </template>
          <template v-else>
            <span class="font-medium">{{ record.name }}</span>
          </template>
        </template>

        <template v-else-if="column.key === 'created_at'">
          {{ formatTime(record.created_at) }}
        </template>

        <template v-else-if="column.key === 'plugin_count'">
          <a-button
            type="link"
            size="small"
            :disabled="record.plugin_count === 0"
            class="!p-0 !h-auto"
            @click="openDetail(record as BackupInfo, 'plugins')"
            >{{ record.plugin_count }}</a-button
          >
        </template>

        <template v-else-if="column.key === 'admin_count'">
          <a-button
            type="link"
            size="small"
            :disabled="(record.admin_count ?? 0) === 0"
            class="!p-0 !h-auto"
            @click="openDetail(record as BackupInfo, 'admins')"
            >{{ record.admin_count ?? 0 }}</a-button
          >
        </template>

        <template v-else-if="column.key === 'has_server_info' || column.key === 'server_info'">
          <a-button
            v-if="record.has_server_info"
            type="link"
            size="small"
            class="!p-0 !h-auto"
            @click="openDetail(record as BackupInfo, 'server_info')"
            >✓</a-button
          >
          <span v-else class="text-gray-400">-</span>
        </template>

        <template v-else-if="column.key === 'server_config'">
          <a-button
            v-if="record.has_server_config"
            type="link"
            size="small"
            class="!p-0 !h-auto"
            @click="openDetail(record as BackupInfo, 'server_config')"
            >✓</a-button
          >
          <span v-else class="text-gray-400">-</span>
        </template>

        <template v-else-if="column.key === 'actions'">
          <div class="flex items-center gap-1 whitespace-nowrap">
            <a-popconfirm
              title="还原将重置插件、管理员列表、服务器信息及配置，确定要继续吗？"
              ok-text="确定"
              cancel-text="取消"
              placement="topRight"
              :destroy-tooltip-on-hide="true"
              :get-popup-container="getPopupContainer"
              @confirm="handleRestore(record.name)"
            >
              <a-button
                type="primary"
                size="small"
                :loading="restoringName === record.name"
                class="!inline-flex !items-center !justify-center"
              >
                <template #icon><UndoOutlined /></template>
                还原
              </a-button>
            </a-popconfirm>
            <a-button
              size="small"
              @click="startRename(record as BackupInfo)"
              class="!inline-flex !items-center !justify-center"
            >
              <template #icon><EditOutlined /></template>
              改名
            </a-button>
            <a-popconfirm
              title="确定要删除这个备份吗？"
              ok-text="确定"
              cancel-text="取消"
              placement="topRight"
              :destroy-tooltip-on-hide="true"
              :get-popup-container="getPopupContainer"
              @confirm="handleDelete(record.name)"
            >
              <a-button danger size="small" class="!inline-flex !items-center !justify-center">
                <template #icon><DeleteOutlined /></template>
                删除
              </a-button>
            </a-popconfirm>
            <a-button
              size="small"
              @click="handleExport(record.name)"
              class="!inline-flex !items-center !justify-center"
            >
              <template #icon><DownloadOutlined /></template>
              导出
            </a-button>
          </div>
        </template>
      </template>
    </a-table>
  </a-modal>

  <!-- Detail Modal -->
  <a-modal
    :open="detailOpen"
    :title="`${detailName} — ${detailTitle[detailType]}`"
    :footer="null"
    :width="detailModalWidth"
    @cancel="detailOpen = false"
  >
    <div v-if="detailLoading" class="py-8 text-center text-gray-400">加载中...</div>
    <template v-else>
      <!-- Plugins -->
      <template v-if="detailType === 'plugins'">
        <a-table
          v-if="detailPlugins.length > 0"
          :columns="pluginListColumns"
          :data-source="detailPlugins"
          row-key="name"
          :pagination="false"
          size="small"
          :scroll="{ x: 360 }"
        >
          <template #bodyCell="{ column, record: plugin }">
            <template v-if="column.key === 'configs'">
              <a-button
                v-if="plugin.configs && plugin.configs.length > 0"
                type="link"
                size="small"
                class="!p-0 !h-auto"
                @click="openPluginCfg(plugin as BackupPlugin)"
                >存在修改</a-button
              >
              <span v-else class="text-gray-400">-</span>
            </template>
          </template>
        </a-table>
        <div v-else class="text-gray-400">暂无插件</div>
      </template>

      <!-- Admins -->
      <template v-else-if="detailType === 'admins'">
        <a-table
          v-if="detailAdmins.length > 0"
          :columns="adminColumns"
          :data-source="detailAdmins"
          row-key="steamid"
          :pagination="false"
          size="small"
          :scroll="{ x: 360 }"
        />
        <div v-else class="text-gray-400">暂无管理员</div>
      </template>

      <!-- Server Info -->
      <template v-else-if="detailType === 'server_info'">
        <a-descriptions :column="1" bordered size="small">
          <a-descriptions-item label="服务器名称">
            <span class="whitespace-pre-wrap break-all">{{
              detailServerInfo?.hostname || '-'
            }}</span>
          </a-descriptions-item>
          <a-descriptions-item label="标题 (host.txt)">
            <span class="whitespace-pre-wrap break-all">{{ detailServerInfo?.host || '-' }}</span>
          </a-descriptions-item>
          <a-descriptions-item label="公告 (motd)">
            <span class="whitespace-pre-wrap break-all">{{ detailServerInfo?.motd || '-' }}</span>
          </a-descriptions-item>
        </a-descriptions>
      </template>

      <!-- Server Config -->
      <template v-else-if="detailType === 'server_config'">
        <a-descriptions :column="1" bordered size="small">
          <a-descriptions-item label="隐藏服务器">
            {{ detailServerConfig?.hidden ? '是' : '否' }}
          </a-descriptions-item>
          <a-descriptions-item label="仅匹配连接">
            {{ detailServerConfig?.lobby_connect_only ? '是' : '否' }}
          </a-descriptions-item>
          <a-descriptions-item label="Steam 组 ID">
            {{ detailServerConfig?.steam_group || '-' }}
          </a-descriptions-item>
          <a-descriptions-item label="自定义配置">
            <pre
              v-if="detailServerConfig?.custom_config?.length"
              class="whitespace-pre-wrap break-all text-xs m-0"
              >{{ detailServerConfig.custom_config.join('\n') }}</pre
            >
            <span v-else class="text-gray-400">-</span>
          </a-descriptions-item>
        </a-descriptions>
      </template>
    </template>
  </a-modal>

  <!-- Plugin Config Detail Modal -->
  <a-modal
    :open="pluginCfgOpen"
    :title="`${pluginCfgName} — 配置修改详情`"
    :footer="null"
    :width="cfgModalWidth"
    @cancel="pluginCfgOpen = false"
  >
    <a-table
      :columns="pluginCfgColumns"
      :data-source="pluginCfgRows"
      :row-key="(r, i) => `${r.file}-${r.cvar}-${i}`"
      :pagination="false"
      size="small"
      :scroll="{ x: 480 }"
    />
  </a-modal>
</template>

<style scoped>
  :deep(.ant-popconfirm-buttons) {
    display: flex;
    justify-content: flex-end;
    flex-wrap: nowrap;
    gap: 8px;
    white-space: nowrap;
  }
  :deep(.ant-popconfirm-buttons button) {
    margin-left: 0 !important;
  }
  :deep(.ant-popconfirm-message) {
    white-space: nowrap;
  }
</style>
