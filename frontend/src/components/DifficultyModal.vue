<script setup lang="ts">
  import { ref } from 'vue';
  import { api } from '../services/api';
  import { message } from 'ant-design-vue';
  import { DIFFICULTY_OPTIONS } from '../utils/gameConstants';

  const props = defineProps<{
    open: boolean;
  }>();

  const emit = defineEmits<{
    (e: 'update:open', value: boolean): void;
    (e: 'success'): void;
  }>();

  const loading = ref(false);

  const handleSetDifficulty = async (difficulty: string) => {
    loading.value = true;
    try {
      await api.setDifficulty(difficulty);
      message.success(`难度已设置为 ${difficulty}`);
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
    title="设置游戏难度"
    @update:open="$emit('update:open', $event)"
    :footer="null"
    centered
    width="500px"
  >
    <div class="grid grid-cols-2 gap-4 py-4">
      <div
        v-for="diff in DIFFICULTY_OPTIONS"
        :key="diff.value"
        class="cursor-pointer border dark:border-gray-700 rounded-xl p-4 hover:shadow-md transition-all duration-300 flex flex-col items-center gap-2 group relative overflow-hidden dark:bg-gray-800"
        :class="[
          `hover:border-${diff.color}-400 dark:hover:border-${diff.color}-600`,
          `hover:bg-${diff.color}-50 dark:hover:bg-${diff.color}-900/20`,
        ]"
        @click="handleSetDifficulty(diff.value)"
      >
        <div
          class="text-4xl mb-2 transform group-hover:scale-110 transition-transform duration-300"
        >
          {{ diff.icon }}
        </div>
        <div
          class="font-bold text-lg text-gray-800 dark:text-gray-200 group-hover:text-gray-900 dark:group-hover:text-gray-100"
        >
          {{ diff.label }}
        </div>
        <div class="text-xs text-gray-500 dark:text-gray-400 font-mono uppercase tracking-wider">
          {{ diff.desc }}
        </div>

        <!-- Loading overlay if needed -->
        <div
          v-if="loading"
          class="absolute inset-0 bg-white/50 flex items-center justify-center z-10 cursor-not-allowed"
        ></div>
      </div>
    </div>
  </a-modal>
</template>
