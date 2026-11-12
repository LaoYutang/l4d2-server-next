<script setup lang="ts">
  import { computed } from 'vue';
  import type { MapRemovalFeedback } from '../composables/useMapLifecycle';
  import { mapDeleteDetails } from '../utils/mapLifecycle';

  const props = defineProps<{ feedback: MapRemovalFeedback }>();
  const emit = defineEmits<{ close: [] }>();
  const details = computed(() => props.feedback.result ? mapDeleteDetails(props.feedback.result) : []);
</script>

<template>
  <a-alert class="map-removal-result" :type="feedback.type" :message="feedback.title" show-icon closable @close="emit('close')">
    <template #description>
      <div class="flex min-w-0 flex-col gap-2">
        <p class="!mb-0 break-words">{{ feedback.description }}</p>
        <details v-if="details.length" class="min-w-0">
          <summary class="cursor-pointer py-2 text-sm leading-6">查看保留、失败与待恢复详情（{{ details.length }}）</summary>
          <ul class="!mb-0 mt-2 flex max-h-48 min-w-0 flex-col gap-2 overflow-auto pl-4 text-xs">
            <li v-for="(item, index) in details" :key="`${item.name}-${index}`" class="break-all">
              <span class="font-medium">{{ item.name }}</span>：{{ item.reason }}
            </li>
          </ul>
        </details>
      </div>
    </template>
  </a-alert>
</template>

<style scoped>
  .map-removal-result,
  .map-removal-result :deep(.ant-alert-content) {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .map-removal-result :deep(.ant-alert-message) {
    white-space: normal;
  }

  .map-removal-result :deep(.ant-alert-close-icon) {
    display: inline-flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: center;
    width: 2.5rem;
    height: 2.5rem;
  }

  .map-removal-result :deep(.ant-alert-close-icon:focus-visible) {
    outline: 2px solid #1677ff;
    outline-offset: 2px;
  }
</style>
