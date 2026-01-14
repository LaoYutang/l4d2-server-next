<script setup lang="ts">
  import { ref, watch } from 'vue';
  import {
    message,
    Modal as AModal,
    Spin as ASpin,
    Form as AForm,
    FormItem as AFormItem,
    Input as AInput,
    Button as AButton,
    Divider as ADivider,
    Tag as ATag,
  } from 'ant-design-vue';
  import { api } from '../services/api';

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
  const tempApplying = ref<Record<string, boolean>>({});

  const fetchConfigs = async () => {
    if (!props.pluginName) return;
    loading.value = true;
    pluginConfigs.value = [];
    try {
      pluginConfigs.value = await api.getPluginConfigs(props.pluginName);
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
    width="800px"
    :footer="null"
    @cancel="handleCancel"
  >
    <div v-if="loading" class="flex justify-center py-8">
      <a-spin />
    </div>
    <div
      v-else-if="!pluginConfigs || pluginConfigs.length === 0"
      class="text-center py-8 text-gray-500"
    >
      该插件没有找到可配置的文件，请确保插件已启用且生成了配置文件。
    </div>
    <div v-else class="space-y-6 max-h-[60vh] overflow-y-auto pr-2">
      <div v-for="file in pluginConfigs" :key="file.file_name" class="mb-6">
        <h3 class="text-lg font-bold mb-4 flex items-center gap-2">
          <span class="text-gray-400 text-sm">配置文件:</span> {{ file.file_name }}
        </h3>
        <a-form layout="vertical">
          <div
            v-for="cvar in file.cvars"
            :key="cvar.name"
            class="mb-4 p-4 bg-gray-50 rounded-lg border border-gray-100 hover:border-blue-100 transition-colors"
          >
            <div class="flex justify-between items-start mb-2">
              <label class="text-base font-medium text-gray-800 break-all mr-2">{{
                cvar.name
              }}</label>
              <div class="flex flex-wrap gap-1 shrink-0">
                <a-tag v-if="cvar.default" color="blue">Default: {{ cvar.default }}</a-tag>
                <a-tag v-if="cvar.min" color="orange">Min: {{ cvar.min }}</a-tag>
                <a-tag v-if="cvar.max" color="red">Max: {{ cvar.max }}</a-tag>
              </div>
            </div>

            <div
              v-if="cvar.description"
              class="mb-3 text-sm text-gray-500 whitespace-pre-wrap bg-white p-2 rounded border border-gray-100"
            >
              {{ cvar.description }}
            </div>

            <a-form-item class="mb-0">
              <a-input v-model:value="cvar.value">
                <template #addonAfter>
                  <div class="flex items-center gap-2">
                    <a-button
                      type="link"
                      size="small"
                      :loading="tempApplying[cvar.name]"
                      @click="applyTempConfig(cvar)"
                    >
                      临时设置
                    </a-button>
                    <a-divider type="vertical" />
                    <a-button type="link" size="small" @click="saveConfig(file.file_name, cvar)">
                      保存
                    </a-button>
                  </div>
                </template>
              </a-input>
            </a-form-item>
          </div>
        </a-form>
      </div>
    </div>
  </a-modal>
</template>
