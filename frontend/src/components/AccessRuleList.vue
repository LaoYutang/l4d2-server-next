<script setup lang="ts">
  import type { AccessControlRule, AccessRuleType } from '../services/api';
  import { Button as AButton, Input as AInput, Select as ASelect, Switch as ASwitch } from 'ant-design-vue';
  import { DeleteOutlined, PlusOutlined } from '@ant-design/icons-vue';

  const props = defineProps<{
    modelValue: AccessControlRule[];
    title: string;
    description: string;
    disabled?: boolean;
  }>();

  const emit = defineEmits<{
    'update:modelValue': [value: AccessControlRule[]];
  }>();

  const typeOptions = [
    { value: 'keyword', label: '地区关键字' },
    { value: 'ip', label: '单 IP' },
    { value: 'cidr', label: 'CIDR 网段' },
  ];

  // The desktop table needs horizontal overflow at its narrowest breakpoint.
  // Mounting Select popups inside that container makes the dropdown participate
  // in its scroll area, so it gets clipped and creates an unnecessary scrollbar.
  const getSelectPopupContainer = () => document.body;

  const placeholderFor = (type: AccessRuleType) => {
    if (type === 'ip') return '例如：203.0.113.10';
    if (type === 'cidr') return '例如：192.168.1.0/24';
    return '例如：中国、广东、电信';
  };

  const updateRule = <K extends keyof AccessControlRule>(
    index: number,
    key: K,
    value: AccessControlRule[K]
  ) => {
    const next = props.modelValue.map((rule, currentIndex) =>
      currentIndex === index ? { ...rule, [key]: value } : rule
    );
    emit('update:modelValue', next);
  };

  const addRule = () => {
    emit('update:modelValue', [
      ...props.modelValue,
      {
        id: '',
        enabled: true,
        type: 'keyword',
        value: '',
        remark: '',
      },
    ]);
  };

  const removeRule = (index: number) => {
    emit(
      'update:modelValue',
      props.modelValue.filter((_, currentIndex) => currentIndex !== index)
    );
  };
</script>

<template>
  <section class="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
    <header
      class="flex flex-col gap-3 border-b border-slate-200 bg-slate-50/80 px-4 py-4 dark:border-slate-700 dark:bg-slate-800/50 sm:flex-row sm:items-center sm:justify-between"
    >
      <div>
        <h3 class="font-semibold text-slate-900 dark:text-slate-100">{{ title }}</h3>
        <p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">{{ description }}</p>
      </div>
      <a-button
        :disabled="disabled"
        class="!inline-flex !items-center !justify-center self-start sm:self-auto"
        @click="addRule"
      >
        <template #icon><PlusOutlined /></template>
        添加规则
      </a-button>
    </header>

    <div v-if="modelValue.length === 0" class="px-4 py-10 text-center text-sm text-slate-500 dark:text-slate-400">
      暂无规则
    </div>

    <div v-else class="hidden overflow-x-auto lg:block">
      <table class="w-full min-w-[820px] table-fixed">
        <thead class="bg-slate-50 text-left text-xs text-slate-500 dark:bg-slate-800/70 dark:text-slate-400">
          <tr>
            <th class="w-20 px-3 py-3">启用</th>
            <th class="w-36 px-3 py-3">类型</th>
            <th class="w-[32%] px-3 py-3">规则值</th>
            <th class="px-3 py-3">备注</th>
            <th class="w-20 px-3 py-3 text-center">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
          <tr v-for="(rule, index) in modelValue" :key="rule.id || `new-${index}`">
            <td class="px-3 py-3">
              <a-switch
                :checked="rule.enabled"
                :disabled="disabled"
                @update:checked="updateRule(index, 'enabled', Boolean($event))"
              />
            </td>
            <td class="px-3 py-3">
              <a-select
                :value="rule.type"
                :options="typeOptions"
                :disabled="disabled"
                :get-popup-container="getSelectPopupContainer"
                class="w-full"
                @update:value="updateRule(index, 'type', $event as AccessRuleType)"
              />
            </td>
            <td class="px-3 py-3">
              <a-input
                :value="rule.value"
                :placeholder="placeholderFor(rule.type)"
                :disabled="disabled"
                class="font-mono"
                @update:value="updateRule(index, 'value', String($event))"
              />
            </td>
            <td class="px-3 py-3">
              <a-input
                :value="rule.remark"
                placeholder="可选备注"
                :maxlength="200"
                :disabled="disabled"
                @update:value="updateRule(index, 'remark', String($event))"
              />
            </td>
            <td class="px-3 py-3 text-center">
              <a-button
                danger
                type="text"
                :disabled="disabled"
                :aria-label="`删除${title}规则`"
                @click="removeRule(index)"
              >
                <template #icon><DeleteOutlined /></template>
              </a-button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="modelValue.length > 0" class="space-y-3 p-3 lg:hidden">
      <article
        v-for="(rule, index) in modelValue"
        :key="rule.id || `mobile-new-${index}`"
        class="rounded-lg border border-slate-200 bg-slate-50/70 p-3 dark:border-slate-700 dark:bg-slate-800/60"
      >
        <div class="mb-3 flex items-center justify-between">
          <span class="text-xs font-medium text-slate-500 dark:text-slate-400">规则 {{ index + 1 }}</span>
          <div class="flex items-center gap-2">
            <a-switch
              :checked="rule.enabled"
              :disabled="disabled"
              @update:checked="updateRule(index, 'enabled', Boolean($event))"
            />
            <a-button
              danger
              size="small"
              type="text"
              :disabled="disabled"
              :aria-label="`删除${title}规则`"
              @click="removeRule(index)"
            >
              <template #icon><DeleteOutlined /></template>
            </a-button>
          </div>
        </div>

        <div class="space-y-3">
          <label class="block">
            <span class="mb-1 block text-xs text-slate-500 dark:text-slate-400">类型</span>
            <a-select
              :value="rule.type"
              :options="typeOptions"
              :disabled="disabled"
              :get-popup-container="getSelectPopupContainer"
              class="w-full"
              @update:value="updateRule(index, 'type', $event as AccessRuleType)"
            />
          </label>
          <label class="block">
            <span class="mb-1 block text-xs text-slate-500 dark:text-slate-400">规则值</span>
            <a-input
              :value="rule.value"
              :placeholder="placeholderFor(rule.type)"
              :disabled="disabled"
              class="font-mono"
              @update:value="updateRule(index, 'value', String($event))"
            />
          </label>
          <label class="block">
            <span class="mb-1 block text-xs text-slate-500 dark:text-slate-400">备注</span>
            <a-input
              :value="rule.remark"
              placeholder="可选备注"
              :maxlength="200"
              :disabled="disabled"
              @update:value="updateRule(index, 'remark', String($event))"
            />
          </label>
        </div>
      </article>
    </div>
  </section>
</template>
