<script setup lang="ts">
  import { ref, onMounted, onUnmounted } from 'vue';
  import { useRouter } from 'vue-router';
  import { useAuthStore } from '../stores/auth';
  import { useThemeStore } from '../stores/theme';
  import { api } from '../services/api';
  import {
    LockOutlined,
    LoginOutlined,
    KeyOutlined,
    ClockCircleOutlined,
    CheckCircleOutlined,
    CopyOutlined,
    CheckOutlined,
  } from '@ant-design/icons-vue';
  import { message } from 'ant-design-vue';
  import { copyToClipboard } from '../utils/clipboard';

  const password = ref('');
  const loading = ref(false);
  const error = ref('');
  const router = useRouter();
  const authStore = useAuthStore();
  const themeStore = useThemeStore();

  // Self-service Auth
  const selfServiceEnabled = ref(false);
  const selfServiceCooldown = ref(false);
  const selfServiceRemaining = ref(0);
  const selfServiceLoading = ref(false);
  const selfServiceCode = ref('');
  const copied = ref(false);
  let cooldownTimer: number | null = null;
  let syncTimer: number | null = null;

  const handleLogin = async () => {
    if (!password.value) return;

    loading.value = true;
    error.value = '';

    const success = await authStore.login(password.value);

    if (success) {
      router.push('/');
    } else {
      error.value = '鉴权失败，请检查密码或授权码';
    }

    loading.value = false;
  };

  const fetchSelfServiceStatus = async () => {
    try {
      const status = await api.getSelfServiceStatus();
      selfServiceEnabled.value = status.enabled;
      selfServiceCooldown.value = status.in_cooldown;
      selfServiceRemaining.value = status.remaining_seconds;

      if (status.in_cooldown && status.remaining_seconds > 0) {
        if (!cooldownTimer) {
          startCooldownTimer();
        }
      } else {
        if (cooldownTimer) {
          clearInterval(cooldownTimer);
          cooldownTimer = null;
        }
      }
    } catch (e) {
      console.warn('Failed to fetch self service status', e);
    }
  };

  const startCooldownTimer = () => {
    if (cooldownTimer) clearInterval(cooldownTimer);
    cooldownTimer = setInterval(() => {
      if (selfServiceRemaining.value > 0) {
        selfServiceRemaining.value--;
      } else {
        selfServiceCooldown.value = false;
        if (cooldownTimer) {
          clearInterval(cooldownTimer);
          cooldownTimer = null;
        }
      }
    }, 1000);
  };

  const formatTime = (seconds: number) => {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  };

  const copySelfServiceCode = async () => {
    if (!selfServiceCode.value) return;
    
    const success = await copyToClipboard(selfServiceCode.value);
    if (success) {
      copied.value = true;
      message.success('已复制到剪贴板');
      setTimeout(() => {
        copied.value = false;
      }, 2000);
    } else {
      message.error('复制失败，请手动复制');
    }
  };

  const generateSelfServiceCode = async () => {
    selfServiceLoading.value = true;
    copied.value = false;
    try {
      const res = await api.generateSelfServiceCode();
      selfServiceCode.value = res.code;
      // password.value = res.code; // Do not auto-fill

      // Auto copy
      try {
        await navigator.clipboard.writeText(res.code);
        copied.value = true;
        message.success('授权码已复制到剪贴板，请妥善保存！');
        setTimeout(() => {
          copied.value = false;
        }, 2000);
      } catch (err) {
        message.success('授权码获取成功，请手动复制并妥善保存！');
      }

      // Update status immediately
      selfServiceCooldown.value = true;
      selfServiceRemaining.value = 3600; // 1 hour
      startCooldownTimer();
    } catch (e: any) {
      if (e.message.includes('冷却')) {
        message.warning('系统冷却中，请稍后再试');
        fetchSelfServiceStatus(); // Sync status
      } else {
        message.error('获取失败: ' + e.message);
      }
    } finally {
      selfServiceLoading.value = false;
    }
  };

  const handleVisibilityChange = () => {
    if (document.visibilityState === 'visible') {
      fetchSelfServiceStatus();
    }
  };

  onMounted(() => {
    fetchSelfServiceStatus();
    document.addEventListener('visibilitychange', handleVisibilityChange);
    // Sync with backend every 30 seconds
    syncTimer = setInterval(fetchSelfServiceStatus, 30000);
  });

  onUnmounted(() => {
    if (cooldownTimer) clearInterval(cooldownTimer);
    if (syncTimer) clearInterval(syncTimer);
    document.removeEventListener('visibilitychange', handleVisibilityChange);
  });
</script>

<template>
  <div
    class="min-h-screen flex flex-col justify-center items-center p-4 transition-colors duration-300"
    :style="{ background: themeStore.isDark ? '#020617' : '#f3f4f6' }"
  >
    <div class="text-center mb-8 flex flex-col items-center">
      <img src="/logo.png" alt="Logo" class="w-20 h-20 rounded-xl shadow-lg mb-4 object-cover" />
      <h1 class="text-4xl font-bold text-blue-600 mb-2">L4D2 Manager</h1>
      <p class="text-gray-500 dark:text-gray-400">服务器管理面板</p>
    </div>

    <a-card
      class="w-full max-w-md shadow-xl dark:bg-gray-800 dark:border-gray-700"
      :bordered="false"
    >
      <form @submit.prevent="handleLogin">
        <div class="mb-6">
          <label class="block text-gray-700 dark:text-gray-300 text-sm font-bold mb-2">
            访问密码 / 授权码
          </label>
          <a-input-password
            v-model:value="password"
            placeholder="请输入密码..."
            size="large"
            :status="error ? 'error' : ''"
          >
            <template #prefix>
              <LockOutlined class="text-gray-400" />
            </template>
          </a-input-password>
        </div>

        <a-alert v-if="error" :message="error" type="error" show-icon class="mb-6" />

        <a-button
          type="primary"
          html-type="submit"
          block
          size="large"
          :loading="loading"
          class="!flex !items-center !justify-center"
        >
          <template #icon><LoginOutlined /></template>
          {{ loading ? '登录中...' : '登录' }}
        </a-button>
      </form>

      <!-- Self Service Card -->
      <div
        v-if="selfServiceEnabled"
        class="mt-6 pt-6 border-t border-gray-100 dark:border-gray-700"
      >
        <div class="bg-blue-50 dark:bg-blue-900/20 rounded-lg p-4">
          <div
            class="flex items-center gap-2 text-blue-700 dark:text-blue-300 font-bold mb-2 text-sm"
          >
            <KeyOutlined />
            自助授权通道
          </div>

          <div v-if="!selfServiceCooldown && !selfServiceCode">
            <p class="text-xs text-blue-600/80 dark:text-blue-400/80 mb-3">
              可申请 1 小时有效期的临时访问权限。
            </p>
            <a-button
              type="primary"
              ghost
              size="small"
              block
              @click="generateSelfServiceCode"
              :loading="selfServiceLoading"
            >
              获取临时授权码
            </a-button>
          </div>

          <div v-else-if="selfServiceCode">
            <div
              class="flex items-center gap-2 text-green-600 dark:text-green-400 text-xs font-bold mb-2"
            >
              <CheckCircleOutlined /> 获取成功
            </div>
            <div class="flex gap-2 mb-2">
              <p
                class="flex-1 text-xs text-gray-500 dark:text-gray-400 break-all font-mono bg-white dark:bg-gray-800 p-2 rounded border border-gray-200 dark:border-gray-700 flex items-center justify-center text-center"
              >
                {{ selfServiceCode }}
              </p>
              <a-button @click="copySelfServiceCode" class="!flex !items-center !justify-center">
                <template #icon>
                  <check-outlined v-if="copied" />
                  <copy-outlined v-else />
                </template>
              </a-button>
            </div>
            <p class="text-[10px] text-red-500 dark:text-red-400">
              请务必妥善保存！页面刷新后将无法再次查看。
            </p>
          </div>

          <div v-else class="text-center">
            <p class="text-xs text-orange-600 dark:text-orange-400 mb-2 font-medium">
              仍在上次授权时效内
            </p>
            <div
              class="flex items-center justify-center gap-2 text-xl font-mono font-bold text-gray-700 dark:text-gray-300"
            >
              <ClockCircleOutlined />
              {{ formatTime(selfServiceRemaining) }}
            </div>
          </div>
        </div>
      </div>
    </a-card>

    <div class="text-center mt-8 text-sm text-gray-400 dark:text-gray-600">
      &copy; {{ new Date().getFullYear() }} L4D2 Server Manager
    </div>
  </div>
</template>
