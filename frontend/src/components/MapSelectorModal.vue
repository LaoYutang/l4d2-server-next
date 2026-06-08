<script setup lang="ts">
  import { ref, computed, watch } from 'vue';
  import { api } from '../services/api';
  import { officialMaps } from '../data/officialMaps';
  import { message } from 'ant-design-vue';

  const props = defineProps<{
    open: boolean;
  }>();

  const emit = defineEmits<{
    (e: 'update:open', value: boolean): void;
    (e: 'success'): void;
  }>();

  const loading = ref(false);
  const changingMapCode = ref('');
  const searchText = ref('');
  const showOfficial = ref(true);
  const allMaps = ref<any[]>([]);
  const activeKey = ref<string[]>([]); // For collapse

  const fetchMaps = async () => {
    loading.value = true;
    try {
      const serverMaps = await api.getRconMapList();
      mergeMapData(serverMaps);
    } catch (e: any) {
      message.error('获取地图列表失败: ' + e.message);
    } finally {
      loading.value = false;
    }
  };

  const mergeMapData = (serverMaps: any) => {
    // Start with official maps marked as not custom
    const maps = officialMaps.map((officialMap) => ({
      ...officialMap,
      IsCustom: false,
      VpkName: null,
    }));

    const processServerMap = (serverCampaign: any) => {
      // Check if it's an official map
      const isOfficialCampaign = officialMaps.some(
        (officialMap) =>
          officialMap.Chapters &&
          serverCampaign.Chapters &&
          serverCampaign.Chapters.some((serverChapter: any) =>
            officialMap.Chapters.some(
              (officialChapter) => officialChapter.Code === serverChapter.Code
            )
          )
      );

      if (!isOfficialCampaign) {
        maps.push({
          Title: serverCampaign.Title || 'Unknown Campaign',
          Chapters: serverCampaign.Chapters || [],
          IsCustom: true,
          VpkName: serverCampaign.VpkName,
        });
      }
    };

    if (Array.isArray(serverMaps)) {
      serverMaps.forEach(processServerMap);
    } else if (typeof serverMaps === 'object' && serverMaps !== null) {
      if (Array.isArray(serverMaps.campaigns)) {
        serverMaps.campaigns.forEach(processServerMap);
      }
    }

    allMaps.value = maps;
  };

  const filteredMaps = computed(() => {
    let result = allMaps.value;

    // Switch filter: showOfficial means SHOW ALL maps (including official), !showOfficial means HIDE official (show only custom)
    if (!showOfficial.value) {
      result = result.filter((m) => m.IsCustom);
    }
    // If showOfficial is true, we return ALL maps (no filtering), so no else block needed for filtering official only

    // Search filter
    if (searchText.value) {
      const lower = searchText.value.toLowerCase();
      result = result.filter(
        (m) =>
          (m.Title && m.Title.toLowerCase().includes(lower)) ||
          (m.VpkName && m.VpkName.toLowerCase().includes(lower))
      );
    }

    return result;
  });

  const handleChangeMap = async (mapCode: string) => {
    if (changingMapCode.value) return;
    changingMapCode.value = mapCode;
    try {
      await api.changeMap(mapCode);
      message.success('地图切换指令已发送');
      emit('update:open', false);
      emit('success');
    } catch (e: any) {
      message.error('切换地图失败: ' + e.message);
    } finally {
      changingMapCode.value = '';
    }
  };

  watch(
    () => props.open,
    (val) => {
      if (val && allMaps.value.length === 0) {
        fetchMaps();
      }
    }
  );

  // Format modes for display
  const formatModes = (modes: string[]) => {
    if (!modes || modes.length === 0) return 'Unknown';
    const modeMap: Record<string, string> = {
      coop: '战役',
      realism: '写实',
      versus: '对抗',
      survival: '生存',
      scavenge: '清道夫',
    };
    return modes.map((m) => modeMap[m] || m).join(', ');
  };

  const getModeColor = (mode: string) => {
    const colors: Record<string, string> = {
      coop: 'blue',
      realism: 'purple',
      versus: 'red',
      survival: 'orange',
      scavenge: 'green',
    };
    return colors[mode] || 'default';
  };
</script>

<template>
  <a-modal
    :open="open"
    title="切换地图"
    @update:open="$emit('update:open', $event)"
    :footer="null"
    width="min(92vw, 860px)"
    centered
  >
    <div class="flex flex-col gap-4">
      <!-- Controls -->
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
        <a-switch
          v-model:checked="showOfficial"
          checked-children="显示官图"
          un-checked-children="隐藏官图"
          class="self-start sm:self-center"
        />
        <a-input-search
          v-model:value="searchText"
          placeholder="搜索地图名称或文件名..."
          class="w-full sm:flex-1"
        />
        <a-button
          @click="fetchMaps"
          :loading="loading"
          class="!flex !items-center !justify-center w-full sm:!w-auto"
          >刷新</a-button
        >
      </div>

      <!-- Map List -->
      <div class="max-h-[60vh] overflow-y-auto custom-scrollbar">
        <div v-if="loading && allMaps.length === 0" class="py-8 text-center text-gray-500 dark:text-gray-400">
          <a-spin /> 加载地图列表中...
        </div>

        <div v-else-if="filteredMaps.length === 0" class="py-8 text-center text-gray-500 dark:text-gray-400">
          未找到匹配的地图
        </div>

        <a-collapse v-else v-model:activeKey="activeKey" ghost accordion>
          <a-collapse-panel
            v-for="(campaign, index) in filteredMaps"
            :key="index.toString()"
            class="map-collapse-panel mb-2 overflow-hidden rounded-lg bg-gray-50 dark:bg-slate-900/20"
          >
            <template #header>
              <div class="flex items-center gap-2 py-1 w-full">
                <span class="text-xl mr-1">
                  {{ campaign.IsCustom ? '🗺️' : '🏛️' }}
                </span>
                <div class="flex flex-col">
                  <span class="font-bold text-base flex items-center gap-2 dark:text-gray-200">
                    {{ campaign.Title }}
                    <a-tag v-if="campaign.IsCustom" color="purple">三方</a-tag>
                    <a-tag v-else color="blue">官方</a-tag>
                    <a-tag> {{ campaign.Chapters?.length || 0 }} 章 </a-tag>
                  </span>
                  <span v-if="campaign.IsCustom && campaign.VpkName" class="text-xs text-gray-400 dark:text-gray-500">
                    {{ campaign.VpkName }}
                  </span>
                </div>
              </div>
            </template>

            <div class="chapter-grid">
              <button
                v-for="chapter in campaign.Chapters || []"
                :key="chapter.Code"
                type="button"
                class="chapter-card group cursor-pointer border border-gray-200 bg-white text-left transition-all duration-200 hover:-translate-y-0.5 hover:border-blue-300 hover:bg-blue-50/70 hover:shadow-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 dark:border-gray-700 dark:bg-gray-900 dark:hover:border-blue-500 dark:hover:bg-blue-950/30 dark:focus-visible:ring-offset-gray-900"
                :disabled="Boolean(changingMapCode)"
                @click="handleChangeMap(chapter.Code)"
              >
                <div class="flex min-w-0 items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">
                      {{ chapter.Title || chapter.Code }}
                    </div>
                    <div class="mt-1 truncate font-mono text-xs text-gray-500 dark:text-gray-400">
                      {{ chapter.Code }}
                    </div>
                  </div>
                  <a-spin v-if="changingMapCode === chapter.Code" size="small" />
                </div>

                <div class="mt-3 flex flex-wrap gap-1.5">
                  <a-tag
                    v-for="mode in chapter.Modes || []"
                    :key="mode"
                    :color="getModeColor(mode)"
                    class="!m-0 !max-w-full !truncate !text-xs"
                  >
                    {{ formatModes([mode]) }}
                  </a-tag>
                  <a-tag v-if="!chapter.Modes || chapter.Modes.length === 0" class="!m-0 !text-xs">
                    {{ formatModes([]) }}
                  </a-tag>
                </div>
              </button>
            </div>
          </a-collapse-panel>
        </a-collapse>
      </div>
    </div>
  </a-modal>
</template>

<style scoped>
  /* Fix collapse arrow vertical alignment */
  :deep(.ant-collapse-header) {
    align-items: center !important;
  }

  /* Fix search input icon alignment if needed */
  :deep(.ant-input-affix-wrapper) {
    display: flex;
    align-items: center;
  }

  :deep(.ant-input-suffix) {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  :deep(.ant-input-search-icon) {
    display: flex;
    align-items: center;
  }

  /* Make header content expand to fill available width */
  :deep(.ant-collapse-header-text) {
    flex-grow: 1;
  }

  :deep(.ant-collapse-ghost > .ant-collapse-item) {
    border-bottom: 0 !important;
  }

  :deep(.ant-collapse-content) {
    border-top: 0 !important;
    background: transparent !important;
  }

  :deep(.ant-collapse-content-box) {
    padding: 0 16px 18px !important;
  }

  .map-collapse-panel {
    background-clip: padding-box;
  }

  .chapter-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(168px, 1fr));
    gap: 10px;
    padding-top: 8px;
  }

  .chapter-card {
    min-height: 108px;
    border-radius: 8px;
    padding: 12px;
    width: 100%;
  }

  .chapter-card:disabled {
    cursor: wait;
    opacity: 0.72;
    transform: none;
  }

  @media (max-width: 640px) {
    .chapter-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
