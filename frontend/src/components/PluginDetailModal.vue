<script setup lang="ts">
import { ref, watch } from 'vue';
import { Modal as AModal, Spin as ASpin, Empty as AEmpty } from 'ant-design-vue';
import { marked } from 'marked';
import { api } from '../services/api';

const props = defineProps<{
  open: boolean;
  pluginName: string;
  isStorePlugin: boolean;
  proxyUrl?: string;
  githubToken?: string;
  repo?: string;
}>();

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void;
}>();

const loading = ref(false);
const content = ref('');
const fileName = ref('');
const error = ref('');

const fetchReadme = async () => {
  if (!props.pluginName) return;
  loading.value = true;
  error.value = '';
  content.value = '';
  fileName.value = '';
  try {
    const data = await api.getPluginReadme(
      props.pluginName,
      props.isStorePlugin,
      props.proxyUrl || '',
      props.githubToken || '',
      props.repo || ''
    );
    if (data.content) {
      content.value = data.content;
      fileName.value = data.file_name;
    } else {
      error.value = '此插件无说明文件';
    }
  } catch (e: any) {
    error.value = '此插件无说明文件';
  } finally {
    loading.value = false;
  }
};

const renderedHtml = () => {
  if (!content.value) return '';
  return marked(content.value, { breaks: true }) as string;
};

watch(
  () => props.open,
  (newVal) => {
    if (newVal) {
      fetchReadme();
    }
  }
);

const getContainer = () => document.body;

const handleCancel = () => {
  emit('update:open', false);
};
</script>

<template>
  <a-modal
    :open="open"
    :title="`${pluginName} - ${fileName || '详情'}`"
    :width="'min(860px, 95vw)'"
    :footer="null"
    :getContainer="getContainer"
    :zIndex="2000"
    @cancel="handleCancel"
    :bodyStyle="{ padding: '16px 24px', maxHeight: '75vh', overflowY: 'auto' }"
  >
    <div v-if="loading" class="flex justify-center py-12">
      <a-spin size="large" />
    </div>

    <div v-else-if="error" class="text-center py-12">
      <a-empty :description="error">
        <template #description>
          <span class="text-gray-400 dark:text-gray-500">{{ error }}</span>
        </template>
      </a-empty>
    </div>

    <div v-else class="markdown-body" v-html="renderedHtml()" />
  </a-modal>
</template>

<style scoped>
.markdown-body {
  color: #374151;
  line-height: 1.75;
  word-wrap: break-word;
  overflow-wrap: break-word;
}

/* Headings */
.markdown-body :deep(h1) {
  font-size: 1.75rem;
  font-weight: 700;
  margin-top: 1.5rem;
  margin-bottom: 0.75rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid #e5e7eb;
  color: #111827;
}

.markdown-body :deep(h2) {
  font-size: 1.5rem;
  font-weight: 700;
  margin-top: 1.25rem;
  margin-bottom: 0.625rem;
  padding-bottom: 0.375rem;
  border-bottom: 1px solid #e5e7eb;
  color: #1f2937;
}

.markdown-body :deep(h3) {
  font-size: 1.25rem;
  font-weight: 600;
  margin-top: 1rem;
  margin-bottom: 0.5rem;
  color: #374151;
}

.markdown-body :deep(h4) {
  font-size: 1.1rem;
  font-weight: 600;
  margin-top: 0.875rem;
  margin-bottom: 0.5rem;
  color: #374151;
}

.markdown-body :deep(h5),
.markdown-body :deep(h6) {
  font-size: 1rem;
  font-weight: 600;
  margin-top: 0.75rem;
  margin-bottom: 0.5rem;
  color: #4b5563;
}

/* Paragraphs */
.markdown-body :deep(p) {
  margin-top: 0;
  margin-bottom: 0.75rem;
}

.markdown-body :deep(strong),
.markdown-body :deep(em) {
  color: inherit;
}

/* Links */
.markdown-body :deep(a) {
  color: #2563eb;
  text-decoration: none;
}
.markdown-body :deep(a:hover) {
  text-decoration: underline;
}

/* Code blocks */
.markdown-body :deep(pre) {
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 1rem;
  border-radius: 8px;
  overflow-x: auto;
  margin: 0.75rem 0;
  font-size: 0.875rem;
  line-height: 1.6;
  -webkit-overflow-scrolling: touch;
}

.markdown-body :deep(pre code) {
  background: none;
  padding: 0;
  color: inherit;
  font-size: inherit;
  white-space: pre;
}

/* Inline code */
.markdown-body :deep(code) {
  background: #f3f4f6;
  color: #be185d;
  padding: 0.15rem 0.4rem;
  border-radius: 4px;
  font-size: 0.875em;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  word-break: break-word;
}
/* Lists */
.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  padding-left: 1.5rem;
  margin: 0.5rem 0 0.75rem;
}

.markdown-body :deep(li) {
  margin-bottom: 0.25rem;
}

.markdown-body :deep(ul) {
  list-style-type: disc;
}

.markdown-body :deep(ol) {
  list-style-type: decimal;
}

.markdown-body :deep(ul ul) {
  list-style-type: circle;
}

.markdown-body :deep(ul ul ul) {
  list-style-type: square;
}

/* Blockquotes */
.markdown-body :deep(blockquote) {
  border-left: 4px solid #d1d5db;
  padding: 0.5rem 1rem;
  margin: 0.75rem 0;
  background: #f9fafb;
  color: #6b7280;
  border-radius: 0 6px 6px 0;
}

.markdown-body :deep(blockquote p) {
  margin-bottom: 0;
}

/* Tables */
.markdown-body :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 0.75rem 0;
  font-size: 0.9rem;
  display: block;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}

.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid #d1d5db;
  padding: 0.5rem 0.75rem;
  text-align: left;
}
.markdown-body :deep(th) {
  background: #f3f4f6;
  font-weight: 600;
}

.markdown-body :deep(tr:nth-child(even)) {
  background: #f9fafb;
}

.markdown-body :deep(tr:nth-child(odd)) {
  background: #ffffff;
}

/* Horizontal rule */
.markdown-body :deep(hr) {
  border: none;
  border-top: 1px solid #e5e7eb;
  margin: 1.25rem 0;
}

/* Images */
.markdown-body :deep(img) {
  max-width: 100%;
  height: auto;
  border-radius: 6px;
  margin: 0.5rem 0;
}

/* First heading should not have top margin */
.markdown-body :deep(h1:first-child),
.markdown-body :deep(h2:first-child),
.markdown-body :deep(h3:first-child) {
  margin-top: 0;
}

/* Mobile adjustments */
@media (max-width: 640px) {
  .markdown-body {
    font-size: 0.9375rem;
  }

  .markdown-body :deep(h1) {
    font-size: 1.375rem;
  }

  .markdown-body :deep(h2) {
    font-size: 1.25rem;
  }

  .markdown-body :deep(h3) {
    font-size: 1.125rem;
  }

  .markdown-body :deep(pre) {
    padding: 0.75rem;
    font-size: 0.8125rem;
    margin: 0.5rem -0.5rem;
    border-radius: 6px;
  }

  .markdown-body :deep(table) {
    font-size: 0.8125rem;
  }

  .markdown-body :deep(th),
  .markdown-body :deep(td) {
    padding: 0.375rem 0.5rem;
  }
}
</style>
