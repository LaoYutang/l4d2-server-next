<script setup lang="ts">
  import { ref, onMounted, onUnmounted, computed } from 'vue';
  import { message } from 'ant-design-vue';
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
    ReloadOutlined,
    SaveOutlined as SaveFilled,
  } from '@ant-design/icons-vue';
  import { api } from '../services/api';
  import { useAuthStore } from '../stores/auth';

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

  const authStore = useAuthStore();
  const isAdmin = computed(() => authStore.isAdmin);

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

  const isMobile = ref(window.innerWidth < 768);
  const handleResize = () => {
    isMobile.value = window.innerWidth < 768;
  };
  onMounted(() => {
    fetchBackups();
    window.addEventListener('resize', handleResize);
  });
  onUnmounted(() => {
    window.removeEventListener('resize', handleResize);
  });

  const detailModalWidth = computed(() => (isMobile.value ? '95vw' : 600));
  const cfgModalWidth = computed(() => (isMobile.value ? '95vw' : 680));

  const detailOpen = ref(false);
  const detailLoading = ref(false);
  const detailType = ref<DetailType>('plugins');
  const detailName = ref('');
  const detailPlugins = ref<BackupPlugin[]>([]);
  const detailAdmins = ref<BackupAdmin[]>([]);
  const detailServerInfo = ref<BackupServerInfo | null>(null);
  const detailServerConfig = ref<BackupServerConfig | null>(null);

  // Plugin config detail (2nd level modal)
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

  const detailTitle: Record<DetailType, string> = {
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
          `备份还原成功，但以下 ${result.skipped.length} 个插件已不存在被跳过：${result.skipped.join('、')}。还原后请重启服务器以使配置生效。`,
          8
        );
      } else {
        message.success('备份还原成功，请重启服务器以使配置生效', 5);
      }
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
</script>

<template>
  <div class="space-y-6 p-4 md:p-6">
    <!-- Header -->
    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
      <div>
        <h1 class="text-2xl font-bold text-gray-800 dark:text-gray-100 flex items-center gap-2">
          <SaveFilled /> 备份管理
        </h1>
        <p class="text-gray-500 dark:text-gray-400 mt-1">
          备份和还原插件配置、管理员列表、服务器信息及服务器配置（仅备份插件名与配置，不备份插件本身）
        </p>
      </div>
      <div class="flex gap-2">
        <a-button
          @click="fetchBackups"
          :loading="loading"
          class="!flex !items-center !justify-center"
        >
          <template #icon><ReloadOutlined /></template>
          刷新
        </a-button>
      </div>
    </div>

    <!-- Action Bar -->
    <div v-if="isAdmin" class="flex items-center gap-2 flex-wrap">
      <template v-if="showCreateInput">
        <a-input
          v-model:value="newBackupName"
          placeholder="输入备份名称"
          style="width: 240px"
          allow-clear
          @pressEnter="handleCreate"
        />
        <a-button
          type="primary"
          :loading="creating"
          @click="handleCreate"
          class="!flex !items-center !justify-center"
        >
          <template #icon><SaveOutlined /></template>
          保存
        </a-button>
        <a-button
          @click="
            showCreateInput = false;
            newBackupName = '';
          "
          class="!flex !items-center !justify-center"
        >
          <template #icon><CloseOutlined /></template>
        </a-button>
      </template>
      <template v-else>
        <a-button
          type="primary"
          @click="showCreateInput = true"
          class="!flex !items-center !justify-center"
        >
          <template #icon><PlusOutlined /></template>
          新建备份
        </a-button>
        <a-button
          :loading="importing"
          @click="triggerImport"
          class="!flex !items-center !justify-center"
        >
          <template #icon><DownloadOutlined /></template>
          导入
        </a-button>
        <a-button
          v-if="isAdmin"
          :disabled="backups.length === 0"
          @click="handleExportAll"
          class="!flex !items-center !justify-center"
        >
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

    <!-- Backup List Card -->
    <a-card
      :bordered="false"
      class="shadow-sm dark:bg-gray-800 rounded-xl"
      :bodyStyle="{ padding: '0' }"
    >
      <!-- Desktop Table -->
      <div class="hidden md:block overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead
            class="bg-gray-50 dark:bg-gray-900/50 text-gray-500 dark:text-gray-400 border-b border-gray-100 dark:border-gray-700"
          >
            <tr>
              <th class="px-6 py-4 font-medium">备份名称</th>
              <th class="px-4 py-4 font-medium whitespace-nowrap">创建时间</th>
              <th class="px-4 py-4 font-medium whitespace-nowrap text-center">插件</th>
              <th class="px-4 py-4 font-medium whitespace-nowrap text-center">管理员</th>
              <th class="px-4 py-4 font-medium whitespace-nowrap text-center">设置</th>
              <th class="px-4 py-4 font-medium whitespace-nowrap text-center">配置</th>
              <th class="px-4 py-4 font-medium text-right whitespace-nowrap" v-if="isAdmin">
                操作
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-gray-700">
            <tr v-if="loading" class="text-center">
              <td :colspan="isAdmin ? 7 : 6" class="py-8 text-gray-500"><a-spin /> 加载中...</td>
            </tr>
            <tr v-else-if="backups.length === 0" class="text-center">
              <td :colspan="isAdmin ? 7 : 6" class="py-8 text-gray-500 dark:text-gray-400">
                暂无备份
              </td>
            </tr>
            <tr
              v-for="backup in backups"
              :key="backup.name"
              class="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
            >
              <!-- Name -->
              <td class="px-6 py-4 max-w-[260px]">
                <template v-if="editingName === backup.name">
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
                      :loading="renamingName === backup.name"
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
                  <span class="font-medium text-gray-800 dark:text-gray-200 break-all">{{
                    backup.name
                  }}</span>
                </template>
              </td>
              <!-- Time -->
              <td class="px-4 py-4 text-gray-600 dark:text-gray-400 whitespace-nowrap">
                {{ formatTime(backup.created_at) }}
              </td>
              <!-- Plugin Count -->
              <td class="px-4 py-4 text-center">
                <a-button
                  type="link"
                  size="small"
                  :disabled="backup.plugin_count === 0"
                  class="!p-0 !h-auto"
                  @click="openDetail(backup, 'plugins')"
                  >{{ backup.plugin_count }}</a-button
                >
              </td>
              <!-- Admin Count -->
              <td class="px-4 py-4 text-center">
                <a-button
                  type="link"
                  size="small"
                  :disabled="(backup.admin_count ?? 0) === 0"
                  class="!p-0 !h-auto"
                  @click="openDetail(backup, 'admins')"
                  >{{ backup.admin_count ?? 0 }}</a-button
                >
              </td>
              <!-- Server Info -->
              <td class="px-4 py-4 text-center">
                <a-button
                  v-if="backup.has_server_info"
                  type="link"
                  size="small"
                  class="!p-0 !h-auto"
                  @click="openDetail(backup, 'server_info')"
                  >✓</a-button
                >
                <span v-else class="text-gray-400">-</span>
              </td>
              <!-- Server Config -->
              <td class="px-4 py-4 text-center">
                <a-button
                  v-if="backup.has_server_config"
                  type="link"
                  size="small"
                  class="!p-0 !h-auto"
                  @click="openDetail(backup, 'server_config')"
                  >✓</a-button
                >
                <span v-else class="text-gray-400">-</span>
              </td>
              <!-- Actions -->
              <td class="px-4 py-4 text-right" v-if="isAdmin">
                <div class="flex items-center justify-end gap-1 flex-wrap">
                  <a-popconfirm
                    title="还原将重置插件、管理员列表、服务器信息及配置，还原后需重启服务器。确定要继续吗？"
                    ok-text="确定"
                    cancel-text="取消"
                    placement="topRight"
                    :destroy-tooltip-on-hide="true"
                    :get-popup-container="getPopupContainer"
                    @confirm="handleRestore(backup.name)"
                  >
                    <a-button
                      type="primary"
                      size="small"
                      :loading="restoringName === backup.name"
                      class="!inline-flex !items-center !justify-center"
                    >
                      <template #icon><UndoOutlined /></template>
                      还原
                    </a-button>
                  </a-popconfirm>
                  <a-button
                    size="small"
                    @click="startRename(backup)"
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
                    @confirm="handleDelete(backup.name)"
                  >
                    <a-button
                      danger
                      size="small"
                      class="!inline-flex !items-center !justify-center"
                    >
                      <template #icon><DeleteOutlined /></template>
                      删除
                    </a-button>
                  </a-popconfirm>
                  <a-button
                    size="small"
                    @click="handleExport(backup.name)"
                    class="!inline-flex !items-center !justify-center"
                  >
                    <template #icon><UploadOutlined /></template>
                    导出
                  </a-button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </a-card>

    <!-- Mobile Card List -->
    <div class="md:hidden flex flex-col gap-3">
      <div v-if="loading" class="p-8 text-center text-gray-500"><a-spin /> 加载中...</div>
      <div
        v-else-if="backups.length === 0"
        class="p-8 text-center text-gray-500 dark:text-gray-400"
      >
        暂无备份
      </div>
      <a-card v-for="backup in backups" :key="backup.name" v-else size="small">
        <div class="flex flex-col gap-3">
          <!-- Name + Time -->
          <div>
            <template v-if="editingName === backup.name">
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
                  :loading="renamingName === backup.name"
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
              <div class="font-medium text-gray-800 dark:text-gray-200 break-all">
                {{ backup.name }}
              </div>
            </template>
            <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
              {{ formatTime(backup.created_at) }}
            </div>
          </div>
          <!-- Stats -->
          <div class="flex items-center gap-4 text-sm text-gray-600 dark:text-gray-400">
            <a-button
              type="link"
              size="small"
              :disabled="backup.plugin_count === 0"
              class="!p-0 !h-auto"
              @click="openDetail(backup, 'plugins')"
            >
              插件: {{ backup.plugin_count }}
            </a-button>
            <a-button
              type="link"
              size="small"
              :disabled="(backup.admin_count ?? 0) === 0"
              class="!p-0 !h-auto"
              @click="openDetail(backup, 'admins')"
            >
              管理员: {{ backup.admin_count ?? 0 }}
            </a-button>
            <a-button
              v-if="backup.has_server_info"
              type="link"
              size="small"
              class="!p-0 !h-auto"
              @click="openDetail(backup, 'server_info')"
              >设置</a-button
            >
            <a-button
              v-if="backup.has_server_config"
              type="link"
              size="small"
              class="!p-0 !h-auto"
              @click="openDetail(backup, 'server_config')"
              >配置</a-button
            >
          </div>
          <!-- Actions -->
          <div v-if="isAdmin" class="flex items-center gap-1 flex-wrap">
            <a-popconfirm
              title="还原将重置插件、管理员列表、服务器信息及配置，还原后需重启服务器。确定要继续吗？"
              ok-text="确定"
              cancel-text="取消"
              :destroy-tooltip-on-hide="true"
              :get-popup-container="getPopupContainer"
              @confirm="handleRestore(backup.name)"
            >
              <a-button
                type="primary"
                size="small"
                :loading="restoringName === backup.name"
                class="!inline-flex !items-center !justify-center"
              >
                <template #icon><UndoOutlined /></template>
                还原
              </a-button>
            </a-popconfirm>
            <a-button
              size="small"
              @click="startRename(backup)"
              class="!inline-flex !items-center !justify-center"
            >
              <template #icon><EditOutlined /></template>
              改名
            </a-button>
            <a-popconfirm
              title="确定要删除这个备份吗？"
              ok-text="确定"
              cancel-text="取消"
              :destroy-tooltip-on-hide="true"
              :get-popup-container="getPopupContainer"
              @confirm="handleDelete(backup.name)"
            >
              <a-button danger size="small" class="!inline-flex !items-center !justify-center">
                <template #icon><DeleteOutlined /></template>
                删除
              </a-button>
            </a-popconfirm>
            <a-button
              size="small"
              @click="handleExport(backup.name)"
              class="!inline-flex !items-center !justify-center"
            >
              <template #icon><UploadOutlined /></template>
              导出
            </a-button>
          </div>
        </div>
      </a-card>
    </div>

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
            <a-descriptions-item label="开启匹配">
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
        :row-key="(r: any, i: any) => `${r.file}-${r.cvar}-${i}`"
        :pagination="false"
        size="small"
        :scroll="{ x: 480 }"
      />
    </a-modal>
  </div>
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
