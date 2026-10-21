<script setup lang="ts">
  import type { MapImportViolation } from '../services/api';
  defineProps<{ violations: MapImportViolation[] }>();
</script>

<template>
  <div role="alert" class="mt-2 rounded-lg border border-red-200 bg-red-50 p-3 text-xs leading-5 text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300">
    <div class="font-semibold">风险检查未通过，本次导入未安装任何地图</div>
    <ul class="mt-1 space-y-2">
      <li v-for="(violation, index) in violations" :key="index" class="break-all">
        <div>{{ violation.map }}：{{ violation.message }}</div>
        <div v-for="path in violation.paths" :key="path" class="pl-2 font-mono">{{ path }}</div>
      </li>
    </ul>
  </div>
</template>
