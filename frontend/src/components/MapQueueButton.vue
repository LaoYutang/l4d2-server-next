<script setup lang="ts">
  import { ref } from 'vue';
  import { UnorderedListOutlined } from '@ant-design/icons-vue';
  import MapQueueModal from './MapQueueModal.vue';

  const open = ref(false);
</script>

<template>
  <a-button class="!flex !items-center !justify-center" @click="open = true">
    <template #icon><unordered-list-outlined /></template>
    地图待办队列
  </a-button>
  <MapQueueModal v-model:open="open" />
</template>
