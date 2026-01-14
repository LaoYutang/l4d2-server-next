<script setup lang="ts">
  import { computed, ref } from 'vue';
  import { api } from '../services/api';
  import { message } from 'ant-design-vue';
  import { GAMEMODE_OPTIONS } from '../utils/gameConstants';

  const props = defineProps<{
    open: boolean;
  }>();

  const emit = defineEmits<{
    (e: 'update:open', value: boolean): void;
    (e: 'success'): void;
  }>();

  const loading = ref(false);
  const activeKey = ref('base');

  const baseModes = computed(() => GAMEMODE_OPTIONS.filter((m) => m.type === 'base'));
  const mutationModes = computed(() => GAMEMODE_OPTIONS.filter((m) => m.type === 'mutation'));
  const communityModes = computed(() => GAMEMODE_OPTIONS.filter((m) => m.type === 'community'));

  const handleSetGameMode = async (mode: string) => {
    loading.value = true;
    try {
      await api.setGameMode(mode);
      const modeOption = GAMEMODE_OPTIONS.find((m) => m.value === mode);
      message.success(`模式已设置为 ${modeOption?.label || mode}`);
      emit('update:open', false);
      emit('success');
    } catch (e: any) {
      message.error('设置失败: ' + e.message);
    } finally {
      loading.value = false;
    }
  };
</script>

<template>
  <a-modal
    :open="open"
    title="设置游戏模式"
    @update:open="$emit('update:open', $event)"
    :footer="null"
    centered
    width="800px"
    :bodyStyle="{ maxHeight: '80vh', overflowY: 'auto' }"
  >
    <a-tabs v-model:activeKey="activeKey">
      <a-tab-pane key="base" tab="官方模式">
        <div class="grid grid-cols-2 md:grid-cols-3 gap-4 py-2">
          <div
            v-for="mode in baseModes"
            :key="mode.value"
            class="cursor-pointer border rounded-xl p-4 hover:shadow-md transition-all duration-300 flex flex-col items-center gap-2 group relative overflow-hidden"
            :class="[`hover:border-${mode.color}-400`, `hover:bg-${mode.color}-50`]"
            @click="handleSetGameMode(mode.value)"
          >
            <div
              class="text-4xl mb-2 transform group-hover:scale-110 transition-transform duration-300"
            >
              {{ mode.icon }}
            </div>
            <div class="font-bold text-lg text-gray-800 group-hover:text-gray-900">
              {{ mode.label }}
            </div>
            <div class="text-xs text-gray-500 font-mono uppercase tracking-wider">
              {{ mode.desc }}
            </div>
          </div>
        </div>
      </a-tab-pane>

      <a-tab-pane key="mutation" tab="突变模式">
        <div class="grid grid-cols-2 md:grid-cols-3 gap-4 py-2">
          <div
            v-for="mode in mutationModes"
            :key="mode.value"
            class="cursor-pointer border rounded-xl p-4 hover:shadow-md transition-all duration-300 flex flex-col items-center gap-2 group relative overflow-hidden"
            :class="[`hover:border-${mode.color}-400`, `hover:bg-${mode.color}-50`]"
            @click="handleSetGameMode(mode.value)"
          >
            <div class="font-bold text-lg text-gray-800 group-hover:text-gray-900">
              {{ mode.label }}
            </div>
            <div class="text-xs text-gray-500 font-mono uppercase tracking-wider text-center">
              {{ mode.desc }}
            </div>
          </div>
        </div>
      </a-tab-pane>

      <a-tab-pane key="community" tab="社区模式">
        <div class="grid grid-cols-2 md:grid-cols-3 gap-4 py-2">
          <div
            v-for="mode in communityModes"
            :key="mode.value"
            class="cursor-pointer border rounded-xl p-4 hover:shadow-md transition-all duration-300 flex flex-col items-center gap-2 group relative overflow-hidden"
            :class="[`hover:border-${mode.color}-400`, `hover:bg-${mode.color}-50`]"
            @click="handleSetGameMode(mode.value)"
          >
            <div class="font-bold text-lg text-gray-800 group-hover:text-gray-900">
              {{ mode.label }}
            </div>
            <div class="text-xs text-gray-500 font-mono uppercase tracking-wider text-center">
              {{ mode.desc }}
            </div>
          </div>
        </div>
      </a-tab-pane>
    </a-tabs>

    <!-- Loading overlay -->
    <div
      v-if="loading"
      class="absolute inset-0 bg-white/50 flex items-center justify-center z-50 cursor-not-allowed"
    >
      <a-spin size="large" />
    </div>
  </a-modal>
</template>
