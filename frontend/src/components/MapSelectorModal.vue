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
    try {
      await api.changeMap(mapCode);
      message.success('地图切换指令已发送');
      emit('update:open', false);
      emit('success');
    } catch (e: any) {
      message.error('切换地图失败: ' + e.message);
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
    width="800px"
    centered
  >
    <div class="flex flex-col gap-4">
      <!-- Controls -->
      <div class="flex gap-4 items-center">
        <a-switch
          v-model:checked="showOfficial"
          checked-children="显示官图"
          un-checked-children="隐藏官图"
        />
        <a-input-search
          v-model:value="searchText"
          placeholder="搜索地图名称或文件名..."
          class="flex-1"
        />
        <a-button @click="fetchMaps" :loading="loading" class="!flex !items-center !justify-center"
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
            class="mb-2 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-100 dark:border-gray-700 overflow-hidden"
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

            <a-list :data-source="campaign.Chapters" size="small">
              <template #renderItem="{ item: chapter }">
                <a-list-item class="hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors px-4 py-3">
                  <div class="flex items-center justify-between w-full">
                    <div class="flex flex-col gap-1">
                      <span class="font-medium text-gray-800 dark:text-gray-200">
                        {{ chapter.Title || chapter.Code }}
                      </span>
                      <div class="flex flex-wrap gap-1">
                        <a-tag
                          v-for="mode in chapter.Modes || []"
                          :key="mode"
                          :color="getModeColor(mode)"
                          class="!text-xs !m-0"
                        >
                          {{ formatModes([mode]) }}
                        </a-tag>
                      </div>
                    </div>
                    <a-button
                      type="primary"
                      size="small"
                      ghost
                      @click="handleChangeMap(chapter.Code)"
                      class="!flex !items-center !justify-center"
                    >
                      切换
                    </a-button>
                  </div>
                </a-list-item>
              </template>
            </a-list>
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
</style>
