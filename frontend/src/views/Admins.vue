<script setup lang="ts">
  import { ref, onMounted, computed, reactive } from 'vue';
  import { api } from '../services/api';
  import { useAuthStore } from '../stores/auth';
  import { message, Modal } from 'ant-design-vue';
  import {
    UserOutlined,
    PlusOutlined,
    DeleteOutlined,
    ReloadOutlined,
    CopyOutlined,
    SafetyCertificateOutlined,
  } from '@ant-design/icons-vue';

  interface AdminUser {
    steamid: string;
    remark: string;
  }

  const authStore = useAuthStore();
  const admins = ref<AdminUser[]>([]);
  const loading = ref(false);
  const addModalVisible = ref(false);
  const adding = ref(false);

  const form = reactive({
    steamid: '',
    remark: '',
  });

  const isAdmin = computed(() => authStore.isAdmin);

  const fetchError = ref('');

  const fetchAdmins = async () => {
    loading.value = true;
    fetchError.value = '';
    try {
      const result = await api.getAdmins();
      // Ensure admins is always an array
      admins.value = Array.isArray(result) ? result : [];
    } catch (e: any) {
      // If 404, it might mean file missing
      if (e.message && e.message.includes('SourceMod')) {
        fetchError.value = 'SourceMod 未启用或配置文件不存在，请先检查插件状态';
      } else {
        message.error('获取管理员列表失败: ' + e.message);
      }
      admins.value = [];
    } finally {
      loading.value = false;
    }
  };

  const handleAdd = async () => {
    if (!form.steamid) {
      message.warning('请输入 SteamID');
      return;
    }

    adding.value = true;
    try {
      await api.addAdmin(form.steamid, form.remark);
      message.success('添加管理员成功');
      addModalVisible.value = false;
      form.steamid = '';
      form.remark = '';
      fetchAdmins();
    } catch (e: any) {
      message.error('添加失败: ' + e.message);
    } finally {
      adding.value = false;
    }
  };

  const handleDelete = (admin: AdminUser) => {
    Modal.confirm({
      title: '确定要删除该管理员吗？',
      content: `SteamID: ${admin.steamid}`,
      okText: '删除',
      okType: 'danger',
      onOk: async () => {
        try {
          await api.deleteAdmin(admin.steamid);
          message.success('删除成功');
          fetchAdmins();
        } catch (e: any) {
          message.error('删除失败: ' + e.message);
        }
      },
    });
  };

  const copyToClipboard = async (text: string) => {
    // 优先使用现代 Clipboard API
    if (navigator.clipboard && navigator.clipboard.writeText) {
      try {
        await navigator.clipboard.writeText(text);
        message.success('已复制 SteamID');
        return;
      } catch (err) {
        console.warn('Clipboard API 失败，尝试回退方案...', err);
      }
    }

    // 回退方案：使用 document.execCommand (支持 HTTP 环境)
    try {
      const textArea = document.createElement('textarea');
      textArea.value = text;

      // 避免页面滚动
      textArea.style.position = 'fixed';
      textArea.style.left = '-9999px';
      textArea.style.top = '0';

      document.body.appendChild(textArea);
      textArea.focus();
      textArea.select();

      const successful = document.execCommand('copy');
      document.body.removeChild(textArea);

      if (successful) {
        message.success('已复制 SteamID');
      } else {
        throw new Error('execCommand 失败');
      }
    } catch (err) {
      console.error('复制失败:', err);
      message.error('复制失败，请手动复制');
    }
  };

  onMounted(() => {
    fetchAdmins();
  });
</script>

<template>
  <div class="space-y-6 p-4 md:p-6">
    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
      <div>
        <h1 class="text-2xl font-bold text-gray-800 dark:text-gray-100 flex items-center gap-2">
          <SafetyCertificateOutlined /> 管理员设置
        </h1>
        <p class="text-gray-500 dark:text-gray-400 mt-1">
          管理 SourceMod 管理员 (admins_simple.ini)
        </p>
      </div>
      <div class="flex gap-2">
        <a-button
          @click="fetchAdmins"
          :loading="loading"
          class="!flex !items-center !justify-center"
        >
          <template #icon><ReloadOutlined /></template>
          刷新
        </a-button>
        <a-button
          v-if="isAdmin"
          type="primary"
          @click="addModalVisible = true"
          class="!flex !items-center !justify-center"
        >
          <template #icon><PlusOutlined /></template>
          新增管理员
        </a-button>
      </div>
    </div>

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
              <th class="px-6 py-4 font-medium">SteamID</th>
              <th class="px-6 py-4 font-medium">备注</th>
              <th class="px-6 py-4 font-medium text-right" v-if="isAdmin">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-gray-700">
            <tr v-if="loading" class="text-center">
              <td colspan="3" class="py-8 text-gray-500"><a-spin /> 加载中...</td>
            </tr>
            <tr v-else-if="admins.length === 0" class="text-center">
              <td colspan="3" class="py-8 text-gray-500 dark:text-gray-400">
                <div v-if="fetchError" class="flex flex-col items-center gap-2 text-orange-500">
                  <SafetyCertificateOutlined class="text-2xl" />
                  <span>{{ fetchError }}</span>
                </div>
                <span v-else>暂无管理员</span>
              </td>
            </tr>
            <tr
              v-for="admin in admins"
              :key="admin.steamid"
              class="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
            >
              <td class="px-6 py-4">
                <div class="flex items-center gap-3">
                  <div
                    class="w-10 h-10 rounded-full bg-blue-50 dark:bg-blue-900/20 text-blue-500 flex items-center justify-center"
                  >
                    <UserOutlined />
                  </div>
                  <div>
                    <div
                      class="font-mono font-bold text-gray-800 dark:text-gray-200 cursor-pointer hover:text-blue-500 flex items-center gap-2 group"
                      @click="copyToClipboard(admin.steamid)"
                      title="点击复制"
                    >
                      {{ admin.steamid }}
                      <CopyOutlined
                        class="opacity-0 group-hover:opacity-100 transition-opacity text-xs"
                      />
                    </div>
                  </div>
                </div>
              </td>
              <td class="px-6 py-4 text-gray-700 dark:text-gray-300">
                {{ admin.remark || '-' }}
              </td>
              <td class="px-6 py-4 text-right" v-if="isAdmin">
                <a-button
                  type="primary"
                  danger
                  ghost
                  size="small"
                  class="!inline-flex !items-center !justify-center"
                  @click="handleDelete(admin)"
                >
                  <template #icon><DeleteOutlined /></template>
                  删除
                </a-button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Mobile List -->
      <div class="md:hidden">
        <div v-if="loading" class="p-8 text-center text-gray-500"><a-spin /> 加载中...</div>
        <div
          v-else-if="admins.length === 0"
          class="p-8 text-center text-gray-500 dark:text-gray-400"
        >
          <div v-if="fetchError" class="flex flex-col items-center gap-2 text-orange-500">
            <SafetyCertificateOutlined class="text-2xl" />
            <span>{{ fetchError }}</span>
          </div>
          <span v-else>暂无管理员</span>
        </div>
        <div v-else class="divide-y divide-gray-100 dark:divide-gray-700">
          <div v-for="admin in admins" :key="admin.steamid" class="p-4 flex flex-col gap-3">
            <div class="flex justify-between items-center">
              <div class="flex items-center gap-3">
                <div
                  class="w-10 h-10 rounded-full bg-blue-50 dark:bg-blue-900/20 text-blue-500 flex items-center justify-center shrink-0"
                >
                  <UserOutlined />
                </div>
                <div class="min-w-0">
                  <div
                    class="font-mono font-bold text-gray-800 dark:text-gray-200 truncate"
                    @click="copyToClipboard(admin.steamid)"
                  >
                    {{ admin.steamid }}
                  </div>
                  <div
                    class="text-xs text-gray-500 dark:text-gray-400 mt-0.5 flex items-center gap-2"
                  >
                    <span v-if="admin.remark">{{ admin.remark }}</span>
                  </div>
                </div>
              </div>
              <a-button
                v-if="isAdmin"
                type="primary"
                danger
                ghost
                shape="circle"
                size="small"
                class="!flex !items-center !justify-center shrink-0"
                @click="handleDelete(admin)"
              >
                <DeleteOutlined />
              </a-button>
            </div>
          </div>
        </div>
      </div>
    </a-card>

    <!-- Add Modal -->
    <a-modal
      v-model:open="addModalVisible"
      title="新增管理员"
      @ok="handleAdd"
      :confirmLoading="adding"
      destroyOnClose
    >
      <div class="flex flex-col gap-4 py-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
            >SteamID</label
          >
          <a-input v-model:value="form.steamid" placeholder="例如: STEAM_1:1:123456" allow-clear>
            <template #prefix>
              <UserOutlined class="text-gray-400" />
            </template>
          </a-input>
          <p class="text-xs text-gray-500 mt-1">支持 STEAM_X:X:XXXXX 或 [U:1:XXXXX] 格式</p>
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
            >备注 (可选)</label
          >
          <a-input v-model:value="form.remark" placeholder="例如: 朋友" allow-clear />
        </div>
      </div>
    </a-modal>
  </div>
</template>
