<script setup lang="ts">
  import { computed, onUnmounted, ref, watch } from 'vue';
  import { message, Modal } from 'ant-design-vue';
  import { ApiRequestError, type TextConfigDocument, type TextConfigFile } from '../services/api';

  const props = defineProps<{
    open: boolean;
    title: string;
    description?: string;
    saveSuccessMessage?: string;
    listFiles: () => Promise<TextConfigFile[]>;
    readFile: (path: string) => Promise<TextConfigDocument>;
    saveFile?: (path: string, content: string, revision: string) => Promise<TextConfigDocument>;
  }>();
  const emit = defineEmits<{ 'update:open': [value: boolean] }>();
  const files = ref<TextConfigFile[]>([]);
  const activePath = ref('');
  const drafts = ref<Record<string, { document: TextConfigDocument; content: string; conflict: boolean }>>({});
  const loading = ref(false);
  const saving = ref(false);
  const error = ref('');
  let requestId = 0;
  const active = computed(() => drafts.value[activePath.value]);
  const activeFile = computed(() => files.value.find((file) => file.path === activePath.value));
  const dirty = computed(() => active.value && active.value.content !== active.value.document.content);
  const hasDrafts = computed(() => Object.values(drafts.value).some((draft) => draft.content !== draft.document.content));
  const options = computed(() => files.value.map((file) => ({ label: file.path, value: file.path })));

  const selectFile = async (path: string, reload = false) => {
    const id = ++requestId;
    activePath.value = path;
    error.value = '';
    loading.value = false;
    if (!reload && drafts.value[path]) return;
    const file = files.value.find((file) => file.path === path);
    if (props.saveFile && file && !file.editable) {
      error.value = file.error || '该文件不可编辑';
      return;
    }
    loading.value = true;
    try {
      const document = await props.readFile(path);
      if (id !== requestId) return;
      drafts.value[path] = { document, content: document.content, conflict: false };
    } catch (e: any) {
      if (id === requestId) error.value = e.message || '读取文件失败';
    } finally {
      if (id === requestId) loading.value = false;
    }
  };

  const load = async () => {
    const id = ++requestId;
    files.value = [];
    drafts.value = {};
    activePath.value = '';
    error.value = '';
    loading.value = true;
    try {
      const result = await props.listFiles();
      if (id !== requestId) return;
      files.value = result;
      loading.value = false;
      if (result[0]) await selectFile(result[0].path);
    } catch (e: any) {
      if (id === requestId) error.value = e.message || '加载配置列表失败';
    } finally {
      if (id === requestId) loading.value = false;
    }
  };

  const reload = () => {
    if (!activePath.value) return;
    const path = activePath.value;
    if (dirty.value) {
      Modal.confirm({
        title: '重新读取此文件？',
        content: '此文件未保存的草稿将被服务器上的最新内容替换。',
        okText: '重新读取', cancelText: '保留草稿',
        onOk: () => selectFile(path, true),
      });
    } else {
      void selectFile(path, true);
    }
  };

  const save = async () => {
    const draft = active.value;
    if (!draft || !props.saveFile || saving.value) return;
    const path = activePath.value;
    const id = requestId;
    saving.value = true;
    error.value = '';
    try {
      const document = await props.saveFile(path, draft.content, draft.document.revision);
      if (id !== requestId) return;
      drafts.value[path] = { document, content: document.content, conflict: false };
      message.success(props.saveSuccessMessage ?? '配置已保存', 5);
    } catch (e: any) {
      if (id !== requestId) return;
      if (e instanceof ApiRequestError && e.status === 409) draft.conflict = true;
      else error.value = e.message || '保存失败，草稿已保留';
    } finally {
      saving.value = false;
    }
  };

  const close = () => {
    if (saving.value) return;
    const finish = () => emit('update:open', false);
    if (props.saveFile && hasDrafts.value) {
      Modal.confirm({ title: '关闭配置编辑器？', content: '未保存的草稿将被丢弃。', okText: '丢弃并关闭', cancelText: '继续编辑', onOk: finish });
    } else finish();
  };
  watch(() => props.open, (open) => { if (open) void load(); else ++requestId; }, { immediate: true });
  onUnmounted(() => ++requestId);
</script>

<template>
  <a-modal :open="open" :title="title" width="min(960px, 95vw)" centered :body-style="{ maxHeight: 'calc(100dvh - 170px)', overflowY: 'auto' }" :mask-closable="false" :closable="!saving" @cancel="close">
    <p v-if="description" class="text-sm text-gray-500 dark:text-gray-400">{{ description }}</p>
    <a-select
      v-if="files.length" :value="activePath" :options="options" show-search
      class="text-config-file-select" :disabled="saving" aria-label="配置文件"
      @change="(value: any) => selectFile(String(value))"
    />
    <a-alert v-if="error" type="error" show-icon :message="error" class="mb-3" />
    <a-alert
      v-if="active?.conflict" type="warning" show-icon class="mb-3"
      message="服务器上的文件已变化，草稿已保留。请复制需要保留的内容，再重新读取最新版本。"
    />
    <a-alert
      v-if="activeFile?.shared_by?.length" type="warning" show-icon class="mb-3"
      :message="`该配置也被以下插件使用：${activeFile.shared_by.join('、')}`"
    />
    <div v-if="loading" class="flex justify-center py-10"><a-spin /></div>
    <template v-else-if="active">
      <div class="text-config-meta flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span>{{ active.document.encoding.toUpperCase() }}</span>
        <span>{{ active.document.newline.toUpperCase() }}</span>
        <span v-if="active.document.bom">BOM</span>
        <span v-if="dirty" class="text-orange-600 dark:text-orange-400">未保存</span>
      </div>
      <a-textarea
        v-model:value="active.content" :readonly="!saveFile || saving" :spellcheck="false"
        :rows="18" class="text-config-editor" aria-label="配置全文" :maxlength="524288"
      />
    </template>
    <a-empty v-else-if="!files.length && !error" description="暂无配置文件；插件可能尚未生成配置。" />
    <template #footer>
      <div class="flex flex-wrap justify-end gap-2">
        <a-button :disabled="saving" @click="close">关闭</a-button>
        <a-button :disabled="!activePath || loading || saving" @click="reload">重新读取</a-button>
        <a-button v-if="saveFile" type="primary" :loading="saving" :disabled="!dirty || loading || active?.conflict" @click="save">保存当前文件</a-button>
      </div>
    </template>
  </a-modal>
</template>

<style scoped>
  .text-config-file-select { display: block; width: 100%; }
  .text-config-meta { padding-block: 8px; }
  .text-config-editor { font-family: ui-monospace, Consolas, monospace; tab-size: 4; line-height: 1.6; height: clamp(160px, 42vh, 420px); }
</style>
