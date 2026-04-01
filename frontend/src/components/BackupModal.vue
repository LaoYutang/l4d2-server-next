<script setup lang="ts">
  import { ref, watch } from 'vue';
  import {
    message,
    Modal as AModal,
    Table as ATable,
    Button as AButton,
    Input as AInput,
    Popconfirm as APopconfirm,
    Alert as AAlert,
  } from 'ant-design-vue';
  import {
    PlusOutlined,
    UndoOutlined,
    EditOutlined,
    DeleteOutlined,
    SaveOutlined,
    CloseOutlined,
  } from '@ant-design/icons-vue';
  import { api } from '../services/api';

  interface BackupInfo {
    name: string;
    created_at: number;
    plugin_count: number;
  }

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
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
    },
    {
      title: '插件数',
      dataIndex: 'plugin_count',
      key: 'plugin_count',
      width: 80,
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
    :width="720"
    @cancel="emit('update:open', false)"
  >
    <a-alert
      message="此功能仅备份已启用的插件名称及修改过的配置项，不会备份插件文件本身。若插件被删除，还原时该插件将被跳过。建议在服务器运行一段时间后再备份，确保所有插件配置已自动生成。"
      type="info"
      show-icon
    />

    <div class="mt-4 mb-4 flex items-center gap-2">
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
      </template>
    </div>

    <a-table
      :columns="columns"
      :data-source="backups"
      :loading="loading"
      row-key="name"
      :pagination="false"
      size="small"
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

        <template v-else-if="column.key === 'actions'">
          <div class="flex items-center gap-1">
            <a-popconfirm
              title="还原将重置所有插件状态，确定要继续吗？"
              ok-text="确定"
              cancel-text="取消"
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
              @confirm="handleDelete(record.name)"
            >
              <a-button danger size="small" class="!inline-flex !items-center !justify-center">
                <template #icon><DeleteOutlined /></template>
                删除
              </a-button>
            </a-popconfirm>
          </div>
        </template>
      </template>
    </a-table>
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
