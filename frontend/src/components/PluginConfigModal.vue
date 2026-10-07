<script setup lang="ts">
  import { computed, ref, watch } from 'vue';
  import { SearchOutlined } from '@ant-design/icons-vue';
  import {
    message,
    Modal as AModal,
    Spin as ASpin,
    Form as AForm,
    FormItem as AFormItem,
    Input as AInput,
    Button as AButton,
    Tag as ATag,
    Collapse as ACollapse,
    CollapsePanel as ACollapsePanel,
  } from 'ant-design-vue';
  import { api } from '../services/api';
  import { useAuthStore } from '../stores/auth';

  const authStore = useAuthStore();

  interface CvarConfig {
    name: string;
    value: string;
    default: string;
    min: string;
    max: string;
    description: string;
  }

  interface PluginConfigFile {
    file_name: string;
    cvars: CvarConfig[];
  }

  const props = defineProps<{
    open: boolean;
    pluginName: string;
  }>();

  const emit = defineEmits<{
    (e: 'update:open', value: boolean): void;
  }>();

  const loading = ref(false);
  const pluginConfigs = ref<PluginConfigFile[]>([]);
  const searchKeyword = ref('');
  const tempApplying = ref<Record<string, boolean>>({});
  const activeKey = ref<string>('');

  const searchQuery = computed(() => searchKeyword.value.trim().toLowerCase());
  const filteredPluginConfigs = computed(() => {
    const query = searchQuery.value;
    if (!query) return pluginConfigs.value;

    return pluginConfigs.value
      .map((file) => ({
        ...file,
        cvars: file.cvars.filter(
          (cvar) =>
            cvar.name.toLowerCase().includes(query) ||
            cvar.description.toLowerCase().includes(query)
        ),
      }))
      .filter((file) => file.cvars.length > 0);
  });

  watch(filteredPluginConfigs, (files) => {
    const firstFile = files[0];
    if (searchQuery.value && firstFile && !files.some((file) => file.file_name === activeKey.value)) {
      activeKey.value = firstFile.file_name;
    }
  });

  const fetchConfigs = async () => {
    if (!props.pluginName) return;
    loading.value = true;
    pluginConfigs.value = [];
    searchKeyword.value = '';
    activeKey.value = '';
    try {
      const configs = await api.getPluginConfigs(props.pluginName);
      pluginConfigs.value = configs;
      // Only auto-expand if there is exactly one config file
      if (configs && configs.length === 1) {
        activeKey.value = configs[0].file_name;
      } else {
        activeKey.value = '';
      }
    } catch (error: any) {
      message.error('加载配置失败: ' + error.message);
    } finally {
      loading.value = false;
    }
  };

  const saveConfig = async (fileName: string, cvar: CvarConfig) => {
    const hide = message.loading('正在保存配置...', 0);
    try {
      await api.updatePluginConfig(fileName, { [cvar.name]: cvar.value });
      message.success('配置已保存');
    } catch (error: any) {
      message.error('保存失败: ' + error.message);
    } finally {
      hide();
    }
  };

  const applyTempConfig = async (cvar: CvarConfig) => {
    tempApplying.value[cvar.name] = true;
    try {
      // Wrap value in quotes to handle spaces
      await api.sendRconCommand(`sm_cvar ${cvar.name} "${cvar.value}"`);
      message.success(`已临时应用 ${cvar.name}`);
    } catch (error: any) {
      message.error('应用失败: ' + error.message);
    } finally {
      tempApplying.value[cvar.name] = false;
    }
  };

  watch(
    () => props.open,
    (newVal) => {
      if (newVal) {
        fetchConfigs();
      }
    }
  );

  const handleCancel = () => {
    emit('update:open', false);
  };
</script>

<template>
  <a-modal
    :open="open"
    :title="`配置插件: ${pluginName}`"
    width="min(800px, 95vw)"
    :footer="null"
    @cancel="handleCancel"
  >
    <div v-if="loading" class="flex justify-center py-8">
      <a-spin />
    </div>
    <div
      v-else-if="!pluginConfigs || pluginConfigs.length === 0"
      class="text-center py-8 text-gray-500 dark:text-gray-400"
    >
      该插件没有找到可配置的文件，请确保插件已启用且生成了配置文件。
    </div>
    <div v-else>
      <a-input
        v-model:value="searchKeyword"
        allow-clear
        class="mb-4"
        placeholder="筛选变量名或注释"
        aria-label="筛选插件配置项"
      >
        <template #prefix><SearchOutlined /></template>
      </a-input>
      <div
        v-if="filteredPluginConfigs.length === 0"
        class="text-center py-8 text-gray-500 dark:text-gray-400"
      >
        未找到匹配的配置项
      </div>
      <a-collapse
        v-else
        v-model:activeKey="activeKey"
        accordion
        class="max-h-[70vh] overflow-y-auto pr-1 sm:pr-2 custom-scrollbar"
      >
        <a-collapse-panel
          v-for="file in filteredPluginConfigs"
          :key="file.file_name"
          :header="`配置文件: ${file.file_name}`"
        >
          <a-form layout="vertical">
            <div
              v-for="cvar in file.cvars"
              :key="cvar.name"
              class="mb-4 p-3 sm:p-4 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-100 dark:border-gray-700 hover:border-blue-100 dark:hover:border-blue-900 transition-colors"
            >
              <div class="flex flex-col sm:flex-row sm:justify-between sm:items-start mb-2 gap-2">
                <label
                  class="text-sm sm:text-base font-medium text-gray-800 dark:text-gray-200 break-all"
                  >{{ cvar.name }}</label
                >
                <div class="flex flex-wrap gap-1 shrink-0">
                  <a-tag v-if="cvar.default" color="blue" class="mr-0"
                    >Def: {{ cvar.default }}</a-tag
                  >
                  <a-tag v-if="cvar.min" color="orange" class="mr-0">Min: {{ cvar.min }}</a-tag>
                  <a-tag v-if="cvar.max" color="red" class="mr-0">Max: {{ cvar.max }}</a-tag>
                </div>
              </div>

              <div
                v-if="cvar.description"
                class="mb-3 text-xs sm:text-sm text-gray-500 dark:text-gray-400 whitespace-pre-wrap bg-white dark:bg-gray-900 p-2 rounded border border-gray-100 dark:border-gray-700"
              >
                {{ cvar.description }}
              </div>

              <a-form-item class="mb-0">
                <div class="flex flex-col sm:flex-row gap-2">
                  <a-input v-model:value="cvar.value" class="flex-1" />
                  <div class="flex gap-2 justify-end sm:justify-start shrink-0">
                    <a-button
                      class="flex-1 sm:flex-none"
                      :loading="tempApplying[cvar.name]"
                      @click="applyTempConfig(cvar)"
                    >
                      临时设置
                    </a-button>
                    <a-button
                      type="primary"
                      class="flex-1 sm:flex-none"
                      @click="saveConfig(file.file_name, cvar)"
                      :disabled="!authStore.isAdmin"
                    >
                      保存
                    </a-button>
                  </div>
                </div>
              </a-form-item>
            </div>
          </a-form>
        </a-collapse-panel>
      </a-collapse>
    </div>
  </a-modal>
</template>
