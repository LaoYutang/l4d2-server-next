<script setup lang="ts">
  import { ref } from 'vue';
  import { message } from 'ant-design-vue';
  import {
    CloseCircleOutlined,
    InboxOutlined,
    LoadingOutlined,
    PlayCircleOutlined,
  } from '@ant-design/icons-vue';
  import { api, type MapUploadProgressEvent } from '../services/api';

  type MapUploadPhase = 'initializing' | 'uploading' | 'processing';

  interface MapUploadFailure {
    stage: MapUploadPhase;
    message: string;
  }

  const emit = defineEmits<{
    uploaded: [filename: string];
  }>();

  const fileList = ref<any[]>([]);
  const uploadSpeeds = ref<Record<string, string>>({});
  const uploadStates = ref<Record<string, { uploadId: string }>>({});
  const uploadControllers = ref<Record<string, AbortController>>({});
  const uploadPercents = ref<Record<string, number>>({});
  const uploadPhases = ref<Record<string, MapUploadPhase>>({});
  const uploadFailures = ref<Record<string, MapUploadFailure>>({});

  const normalizeUploadErrorMessage = (error: unknown, fallback: string) => {
    const messageText =
      error instanceof Error ? error.message : typeof error === 'string' ? error : '';
    return (messageText || fallback).replace(/\s+/g, ' ').trim() || fallback;
  };

  const getUploadFailureTitle = (stage: MapUploadPhase) => {
    if (stage === 'processing') return '处理失败';
    if (stage === 'uploading') return '上传中断';
    return '上传失败';
  };

  const getUploadErrorSummary = (messageText: string) => {
    const characters = Array.from(messageText);
    return characters.length > 80 ? `${characters.slice(0, 80).join('')}…` : messageText;
  };

  const getUploadFailureLabel = (failure?: MapUploadFailure) =>
    failure
      ? `${getUploadFailureTitle(failure.stage)}：${getUploadErrorSummary(failure.message)}`
      : '上传失败';

  const getUploadFailureTooltip = (failure?: MapUploadFailure) =>
    failure ? `${getUploadFailureTitle(failure.stage)}：${failure.message}` : '上传失败';

  const getTrackedUploadPhase = (fileName: string, fallback: MapUploadPhase): MapUploadPhase =>
    uploadPhases.value[fileName] || fallback;

  const customRequest = async (options: any) => {
    const { file, onSuccess, onError, onProgress } = options;
    const fileName = file.name;
    const previousState = uploadStates.value[fileName];

    if (uploadPhases.value[fileName] === 'processing') {
      const error = new Error('同名文件正在服务端处理中，请等待完成后再上传');
      message.warning(`${fileName} 上传失败：${error.message}`);
      onError(error);
      if (file.uid !== undefined) {
        fileList.value = fileList.value.filter((item: any) => item.uid !== file.uid);
      }
      return;
    }

    const existingController = uploadControllers.value[fileName];
    if (existingController) {
      existingController.abort();
      if (uploadControllers.value[fileName] === existingController) {
        delete uploadControllers.value[fileName];
      }
    }
    if (previousState) {
      delete uploadStates.value[fileName];
      try {
        await api.cancelUpload(previousState.uploadId);
      } catch (e: any) {
        message.warning(`清理 ${fileName} 的上一次上传失败，将由服务器定时清理: ${e.message}`);
      }
    }

    if (file.uid !== undefined) {
      fileList.value = fileList.value.filter(
        (item: any) => item.uid === file.uid || item.name !== fileName
      );
    }

    const controller = new AbortController();
    uploadControllers.value[fileName] = controller;
    let currentUploadId = '';
    let currentStage: MapUploadPhase = 'initializing';
    const isCurrentUpload = () => uploadControllers.value[fileName] === controller;

    uploadPhases.value[fileName] = currentStage;
    delete uploadFailures.value[fileName];
    delete uploadSpeeds.value[fileName];
    delete uploadPercents.value[fileName];

    try {
      const result = await api.uploadMap(
        file,
        (event: MapUploadProgressEvent) => {
          if (!isCurrentUpload()) return;
          currentStage = event.phase;
          uploadPhases.value[fileName] = event.phase;
          uploadPercents.value[fileName] = event.percent;
          if (event.phase === 'uploading') {
            uploadSpeeds.value[fileName] = event.speed;
          } else {
            delete uploadSpeeds.value[fileName];
          }
          onProgress({ percent: event.percent });
        },
        controller.signal,
        (uploadId: string) => {
          if (!isCurrentUpload()) return;
          currentUploadId = uploadId;
          currentStage = 'uploading';
          uploadPhases.value[fileName] = currentStage;
          uploadStates.value[fileName] = { uploadId };
        }
      );

      if (!isCurrentUpload()) {
        onError(new Error('上传任务已被新的同名上传替换'));
        return;
      }

      delete uploadSpeeds.value[fileName];
      delete uploadControllers.value[fileName];
      if (result.success) {
        if (uploadStates.value[fileName]?.uploadId === currentUploadId) {
          delete uploadStates.value[fileName];
        }
        delete uploadPhases.value[fileName];
        delete uploadFailures.value[fileName];
        delete uploadPercents.value[fileName];
        message.success(`${fileName} 上传成功`);
        onSuccess('Ok');
        emit('uploaded', fileName);
      } else {
        const errorMessage = normalizeUploadErrorMessage(result.error, '分片上传失败');
        uploadStates.value[fileName] = { uploadId: result.uploadId };
        uploadPhases.value[fileName] = 'uploading';
        uploadFailures.value[fileName] = { stage: 'uploading', message: errorMessage };
        const currentPercent = uploadPercents.value[fileName] || file.percent || 0;
        message.warning(`${fileName} 上传中断：${errorMessage}；可点击继续上传恢复`);
        onProgress({ percent: currentPercent });
        onError(new Error(errorMessage));
      }
    } catch (error: unknown) {
      const uploadError =
        error instanceof Error
          ? error
          : new Error(normalizeUploadErrorMessage(error, '上传失败'));
      if (!isCurrentUpload()) {
        onError(uploadError);
        return;
      }

      delete uploadSpeeds.value[fileName];
      delete uploadControllers.value[fileName];
      const currentPercent = uploadPercents.value[fileName] || file.percent || 0;
      const errorMessage = normalizeUploadErrorMessage(uploadError, '上传失败');
      if (controller.signal.aborted || errorMessage === '上传已取消') {
        if (uploadStates.value[fileName]?.uploadId === currentUploadId) {
          delete uploadStates.value[fileName];
        }
        delete uploadPhases.value[fileName];
        delete uploadFailures.value[fileName];
        delete uploadPercents.value[fileName];
        onProgress({ percent: currentPercent });
        onError(uploadError);
        return;
      }
      if (uploadStates.value[fileName]?.uploadId === currentUploadId) {
        delete uploadStates.value[fileName];
      }
      uploadPhases.value[fileName] = currentStage;
      uploadFailures.value[fileName] = { stage: currentStage, message: errorMessage };
      message.error(`${fileName} ${getUploadFailureTitle(currentStage)}：${errorMessage}`);
      onProgress({ percent: currentPercent });
      onError(uploadError);
    }
  };

  const resumeUpload = async (fileItem: any) => {
    const fileName = fileItem.name;
    const state = uploadStates.value[fileName];
    if (!state) return;

    const targetFile = fileList.value.find((file: any) => file.uid === fileItem.uid);
    const currentPercent = targetFile?.percent || 0;
    if (targetFile) {
      targetFile.status = 'uploading';
    }

    const controller = new AbortController();
    uploadControllers.value[fileName] = controller;
    let currentStage: MapUploadPhase = 'uploading';
    const isCurrentUpload = () => uploadControllers.value[fileName] === controller;

    uploadPhases.value[fileName] = currentStage;
    delete uploadFailures.value[fileName];
    delete uploadSpeeds.value[fileName];

    try {
      const fileObject = fileItem.originFileObj || fileItem;
      await api.resumeUpload(
        state.uploadId,
        fileObject,
        (event: MapUploadProgressEvent) => {
          if (!isCurrentUpload()) return;
          currentStage = event.phase;
          uploadPhases.value[fileName] = event.phase;
          uploadPercents.value[fileName] = event.percent;
          if (event.phase === 'uploading') {
            uploadSpeeds.value[fileName] = event.speed;
          } else {
            delete uploadSpeeds.value[fileName];
          }
          if (targetFile) {
            targetFile.percent = event.percent;
          }
        },
        controller.signal
      );

      if (!isCurrentUpload()) {
        if (targetFile) targetFile.status = 'error';
        return;
      }

      delete uploadSpeeds.value[fileName];
      delete uploadStates.value[fileName];
      delete uploadControllers.value[fileName];
      delete uploadPercents.value[fileName];
      delete uploadPhases.value[fileName];
      delete uploadFailures.value[fileName];
      message.success(`${fileName} 上传成功`);
      if (targetFile) {
        targetFile.status = 'done';
        targetFile.percent = 100;
      }
      emit('uploaded', fileName);
    } catch (error: unknown) {
      if (!isCurrentUpload()) {
        if (targetFile) targetFile.status = 'error';
        return;
      }

      delete uploadSpeeds.value[fileName];
      delete uploadControllers.value[fileName];
      const failedPercent = uploadPercents.value[fileName] || currentPercent;
      const errorMessage = normalizeUploadErrorMessage(error, '续传失败');
      if (controller.signal.aborted || errorMessage === '上传已取消') {
        if (targetFile) {
          targetFile.status = 'error';
          targetFile.percent = failedPercent;
        }
        return;
      }
      const failureStage = getTrackedUploadPhase(fileName, currentStage);
      if (failureStage === 'processing' && uploadStates.value[fileName]?.uploadId === state.uploadId) {
        delete uploadStates.value[fileName];
      }
      uploadPhases.value[fileName] = failureStage;
      uploadFailures.value[fileName] = { stage: failureStage, message: errorMessage };
      message.error(`${fileName} ${getUploadFailureTitle(failureStage)}：${errorMessage}`);
      if (targetFile) {
        targetFile.status = 'error';
        targetFile.percent = failedPercent;
      }
    }
  };

  const removeUploadFile = async (uid: string) => {
    const file = fileList.value.find((item: any) => item.uid === uid);
    if (!file) return;
    const state = uploadStates.value[file.name];

    if (file.status === 'uploading' && uploadPhases.value[file.name] === 'processing') {
      message.info('服务端正在处理该文件，暂时无法取消');
      return;
    }

    const controller = uploadControllers.value[file.name];
    if (controller) {
      controller.abort();
      if (uploadControllers.value[file.name] === controller) {
        delete uploadControllers.value[file.name];
      }
    }

    delete uploadStates.value[file.name];
    delete uploadSpeeds.value[file.name];
    delete uploadPercents.value[file.name];
    delete uploadPhases.value[file.name];
    delete uploadFailures.value[file.name];

    fileList.value = fileList.value.filter((item: any) => item.uid !== uid);

    if (state) {
      try {
        await api.cancelUpload(state.uploadId);
      } catch (e: any) {
        message.warning(`清理 ${file.name} 的上传临时文件失败，将由服务器定时清理: ${e.message}`);
      }
    }
  };
</script>

<template>
  <div class="space-y-4">
    <a-upload-dragger
      v-model:fileList="fileList"
      name="file"
      :multiple="true"
      :customRequest="customRequest"
      :showUploadList="false"
      accept=".vpk,.zip,.rar,.7z"
    >
      <p class="ant-upload-drag-icon">
        <InboxOutlined />
      </p>
      <p class="ant-upload-text">点击或拖拽上传地图文件</p>
      <p class="ant-upload-hint">支持 .vpk, .zip, .rar, .7z 格式</p>
    </a-upload-dragger>

    <div v-if="fileList.length > 0" class="space-y-2 mt-4">
      <div
        v-for="file in fileList"
        :key="file.uid"
        class="flex flex-col gap-1 p-2 rounded border border-gray-200 dark:border-gray-700"
      >
        <div class="flex items-center justify-between">
          <div class="flex flex-1 items-center gap-2 min-w-0">
            <span class="text-sm truncate" :title="file.name">{{ file.name }}</span>
            <span
              v-if="file.status === 'uploading' && uploadPhases[file.name] === 'processing'"
              class="flex items-center gap-1 text-xs text-blue-500 whitespace-nowrap"
            >
              <LoadingOutlined spin />
              服务端处理中
            </span>
            <span
              v-else-if="file.status === 'uploading' && uploadSpeeds[file.name]"
              class="text-xs text-gray-500 whitespace-nowrap"
            >
              {{ uploadSpeeds[file.name] }}
            </span>
            <a-tooltip
              v-if="file.status === 'error' && uploadFailures[file.name]"
              :title="getUploadFailureTooltip(uploadFailures[file.name])"
            >
              <span
                class="inline-block max-w-[240px] truncate align-bottom text-xs text-red-500 md:max-w-[420px]"
              >
                {{ getUploadFailureLabel(uploadFailures[file.name]) }}
              </span>
            </a-tooltip>
            <span v-else-if="file.status === 'error'" class="text-xs text-red-500 whitespace-nowrap">
              上传失败
            </span>
          </div>
          <a-space>
            <a-button
              v-if="file.status === 'error' && uploadStates[file.name]"
              type="text"
              size="small"
              class="!flex !items-center"
              @click="resumeUpload(file)"
            >
              <template #icon><PlayCircleOutlined /></template>
              继续上传
            </a-button>
            <a-button
              v-if="file.status === 'uploading' && uploadPhases[file.name] !== 'processing'"
              type="text"
              size="small"
              danger
              @click="removeUploadFile(file.uid)"
            >
              取消
            </a-button>
            <a-button
              v-else-if="file.status !== 'uploading'"
              type="text"
              size="small"
              danger
              @click="removeUploadFile(file.uid)"
            >
              <template #icon><CloseCircleOutlined /></template>
            </a-button>
          </a-space>
        </div>
        <a-progress
          v-if="file.status === 'uploading' || file.status === 'done' || file.status === 'error'"
          :percent="Number((uploadPercents[file.name] || file.percent || 0).toFixed(1))"
          size="small"
          :show-info="false"
          :status="
            file.status === 'error'
              ? 'exception'
              : file.status === 'done'
                ? 'success'
                : 'active'
          "
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
  :global(.dark .ant-upload.ant-upload-drag) {
    background-color: #1f2937 !important;
    border-color: #374151 !important;
  }

  :global(.dark .ant-upload.ant-upload-drag .ant-upload-text) {
    color: #e5e7eb !important;
  }

  :global(.dark .ant-upload.ant-upload-drag .ant-upload-hint) {
    color: #9ca3af !important;
  }

  :global(.dark .ant-upload.ant-upload-drag:hover) {
    border-color: #3b82f6 !important;
  }

  :global(.dark .ant-upload.ant-upload-drag .anticon) {
    color: #3b82f6 !important;
  }
</style>
