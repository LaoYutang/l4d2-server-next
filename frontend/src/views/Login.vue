<script setup lang="ts">
  import { ref } from 'vue';
  import { useRouter } from 'vue-router';
  import { useAuthStore } from '../stores/auth';
  import { useThemeStore } from '../stores/theme';
  import { LockOutlined, LoginOutlined } from '@ant-design/icons-vue';

  const password = ref('');
  const loading = ref(false);
  const error = ref('');
  const router = useRouter();
  const authStore = useAuthStore();
  const themeStore = useThemeStore();

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
    </a-card>

    <div class="text-center mt-8 text-sm text-gray-400 dark:text-gray-600">
      &copy; {{ new Date().getFullYear() }} L4D2 Server Manager
    </div>
  </div>
</template>
