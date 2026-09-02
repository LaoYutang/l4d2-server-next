<script setup lang="ts">
  import { h, ref } from 'vue';
  import { message, Modal } from 'ant-design-vue';
  import { ExclamationCircleOutlined, ReloadOutlined } from '@ant-design/icons-vue';
  import { api } from '../services/api';

  const loading = ref(false);

  const executeHotReload = async () => {
    loading.value = true;
    try {
      const result = await api.hotReloadMaps();
      message.success(result.message || '地图热重载指令已发送');
    } catch (error: any) {
      message.error('热重载失败: ' + error.message);
    } finally {
      loading.value = false;
    }
  };

  const confirmHotReload = async () => {
    loading.value = true;
    let usingDefault = false;
    try {
      const status = await api.getMapHotReloadStatus();
      usingDefault = status.using_default;
    } catch (error: any) {
      message.error('获取热重载状态失败: ' + error.message);
      loading.value = false;
      return;
    }
    loading.value = false;

    const content = [
      h('p', '热重载会重新加载地图资源。如果地图过多，会占用 CPU 并影响正在游玩的游戏。'),
    ];
    if (usingDefault) {
      content.push(
        h(
          'p',
          '当前使用默认指令，仅会更新游戏服务器的地图，投票插件的地图缓存不会被刷新。如需同时刷新投票插件缓存，请由管理员自定义地图插件的更新指令。'
        )
      );
    }

    Modal.confirm({
      title: '确认热重载地图？',
      icon: () => h(ExclamationCircleOutlined),
      content: h('div', { class: 'space-y-2' }, content),
      okText: '确认热重载',
      cancelText: '取消',
      onOk: executeHotReload,
    });
  };
</script>

<template>
  <a-button :loading="loading" class="!flex !items-center !justify-center" @click="confirmHotReload">
    <template #icon><ReloadOutlined /></template>
    热重载地图
  </a-button>
</template>
