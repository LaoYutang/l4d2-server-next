<script setup lang="ts">
  import { ref, onErrorCaptured } from 'vue';
  import { api } from '../services/api';
  import {
    message,
    Card as ACard,
    Select as ASelect,
    SelectOption as ASelectOption,
    Button as AButton,
    Input as AInput,
    Divider as ADivider,
  } from 'ant-design-vue';
  import {
    KeyOutlined,
    InfoCircleOutlined,
    CheckCircleOutlined,
    CopyOutlined,
    CheckOutlined,
  } from '@ant-design/icons-vue';

  const expiredHours = ref(1);
  const generating = ref(false);
  const generatedCode = ref('');
  const expirationTime = ref('');
  const copied = ref(false);
  const codeInput = ref<any>(null);

  onErrorCaptured((err) => {
    console.error('System.vue Error:', err);
    message.error('系统管理页面发生错误');
    return false;
  });

  const generateCode = async () => {
    generating.value = true;
    generatedCode.value = '';
    copied.value = false;

    try {
      const code = await api.generateTempAuthCode(expiredHours.value);
      generatedCode.value = code;

      // Calculate expiration time
      const date = new Date();
      date.setHours(date.getHours() + Number(expiredHours.value));
      expirationTime.value = date.toLocaleString();
      message.success('授权码生成成功');
    } catch (e: any) {
      message.error(`生成失败: ${e.message}`);
    } finally {
      generating.value = false;
    }
  };

  const copyCode = async () => {
    if (!generatedCode.value) return;

    try {
      await navigator.clipboard.writeText(generatedCode.value);
      copied.value = true;
      message.success('已复制到剪贴板');
      setTimeout(() => {
        copied.value = false;
      }, 2000);
    } catch (e) {
      // Fallback
      if (codeInput.value) {
        codeInput.value.focus();
        // Ant Design Input select() might not be directly available, use document execCommand if needed after focus
        // But usually navigator.clipboard is available.
        message.warning('无法自动复制，请手动复制');
      }
    }
  };
</script>

<template>
  <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
    <!-- Temp Auth Code Section -->
    <a-card class="shadow-xl" :bordered="false">
      <template #title>
        <span class="flex items-center gap-2 text-lg">
          <key-outlined class="text-blue-500" />
          临时授权管理
        </span>
      </template>
      <p class="text-gray-500 mb-6">
        生成临时访问授权码，允许访客在无需管理员密码的情况下访问面板。
      </p>

      <div class="mb-6">
        <label class="block text-sm font-bold mb-2">选择有效期</label>
        <div class="flex gap-2">
          <a-select v-model:value="expiredHours" class="flex-1">
            <a-select-option :value="1">1 小时</a-select-option>
            <a-select-option :value="6">6 小时</a-select-option>
            <a-select-option :value="12">12 小时</a-select-option>
            <a-select-option :value="24">24 小时 (1天)</a-select-option>
            <a-select-option :value="72">72 小时 (3天)</a-select-option>
            <a-select-option :value="168">168 小时 (7天)</a-select-option>
          </a-select>
          <a-button type="primary" @click="generateCode" :loading="generating">
            {{ generating ? '生成中' : '生成授权码' }}
          </a-button>
        </div>
      </div>

      <!-- Result Display -->
      <div
        v-if="generatedCode"
        class="bg-green-50 border border-green-200 rounded-lg p-4 animate-fade-in"
      >
        <div class="flex items-center gap-2 text-green-600 font-bold mb-1">
          <check-circle-outlined />
          生成成功
        </div>
        <div class="text-xs text-gray-500 mb-3">有效期至: {{ expirationTime }}</div>

        <div class="flex gap-2">
          <a-input
            v-model:value="generatedCode"
            readonly
            ref="codeInput"
            class="font-mono text-center text-lg text-blue-600"
          />
          <a-button @click="copyCode" class="!flex !items-center !justify-center">
            <template #icon>
              <check-outlined v-if="copied" />
              <copy-outlined v-else />
            </template>
            {{ copied ? '已复制' : '复制' }}
          </a-button>
        </div>
      </div>
    </a-card>

    <!-- System Info Section -->
    <a-card class="shadow-xl" :bordered="false">
      <template #title>
        <span class="flex items-center gap-2 text-lg">
          <info-circle-outlined class="text-blue-500" />
          关于系统
        </span>
      </template>

      <div class="space-y-6">
        <div>
          <h3 class="font-bold text-gray-800 mb-2 flex items-center gap-2">
            <span>⚖️</span> 开源协议
          </h3>
          <p class="text-gray-600 leading-relaxed">
            本项目基于 Apache License 2.0 开源协议发布<br />
            欢迎贡献代码和提出建议
          </p>
        </div>

        <div>
          <h3 class="font-bold text-gray-800 mb-2 flex items-center gap-2">
            <span>ℹ️</span> 项目信息
          </h3>
          <p class="text-gray-600 leading-relaxed">
            L4D2 服务器管理工具<br />
            作者: LaoYutang<br />
            GitHub:
            <a
              href="https://github.com/LaoYutang/l4d2-server"
              target="_blank"
              class="text-blue-500 hover:underline break-all"
              >https://github.com/LaoYutang/l4d2-server</a
            ><br />
            © 2025 开源社区贡献
          </p>
        </div>

        <a-divider />

        <div class="text-center text-gray-500 text-sm">
          <span>Made with ❤️ by the community | </span>
          <a
            href="https://github.com/LaoYutang/l4d2-server/blob/master/LICENSE"
            target="_blank"
            class="text-blue-500 hover:underline"
          >
            查看许可证
          </a>
        </div>
      </div>
    </a-card>
  </div>
</template>

<style scoped>
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
</style>
