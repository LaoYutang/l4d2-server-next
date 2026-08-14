<script setup lang="ts">
  import { ref, computed, onMounted, onUnmounted, h, watch, reactive } from 'vue';
  defineOptions({ name: 'Maps' });
  import {
    api,
    type DownloadLinkParseResult,
    type MapDictionaryChapterInspection,
    type MapMissionCampaign,
    type MapSummaryItem,
    type ParsedDownloadItem,
  } from '../services/api';
  import { useAuthStore } from '../stores/auth';
  import MapGlobalScriptsModal from '../components/MapGlobalScriptsModal.vue';
  import MapHotReloadSetting from '../components/settings/MapHotReloadSetting.vue';
  import SteamCDNSetting from '../components/settings/SteamCDNSetting.vue';
  import { message, Modal } from 'ant-design-vue';
  import type { TablePaginationConfig } from 'ant-design-vue';
  import {
    InboxOutlined,
    ReloadOutlined,
    DeleteOutlined,
    FileTextOutlined,
    PlusOutlined,
    ExclamationCircleOutlined,
    CloseCircleOutlined,
    PlayCircleOutlined,
    SearchOutlined,
    DownloadOutlined,
    EditOutlined,
    CompressOutlined,
    SettingOutlined,
  } from '@ant-design/icons-vue';

  const authStore = useAuthStore();
  const isAdmin = computed(() => authStore.isAdmin);
  const activeTab = ref('local');
  const maps = ref<Array<{ name: string; size: string; info: string }>>([]);
  const mapSummaries = ref<Record<string, MapSummaryItem>>({});
  const mapSummaryLoading = ref<Record<string, boolean>>({});
  const downloadTasks = ref<Array<any>>([]);
  const loading = ref(false);
  const searchQuery = ref('');
  const selectedRowKeys = ref<string[]>([]);
  const trimmingMaps = ref<Record<string, boolean>>({});
  const renameVisible = ref(false);
  const renamingMap = ref(false);
  const renameOldName = ref('');
  const renameNewName = ref('');
  const detailVisible = ref(false);
  const detailLoading = ref(false);
  const detailMapName = ref('');
  const detailCampaigns = ref<MapMissionCampaign[]>([]);
  const globalScriptsVisible = ref(false);
  const globalScriptsMapName = ref('');
  const hotReloading = ref(false);
  const hotReloadConfigVisible = ref(false);
  const detailCampaignTitle = computed(() =>
    detailCampaigns.value.map((campaign) => campaign.Title || '未命名战役').join(' / ')
  );
  const detailModalTitle = computed(
    () => `地图详情 - ${detailCampaignTitle.value || detailMapName.value}`
  );

  // Upload
  const fileList = ref<any[]>([]);
  const uploadSpeeds = ref<Record<string, string>>({});
  const uploadStates = ref<Record<string, { uploadId: string }>>({});
  const uploadControllers = ref<Record<string, AbortController>>({});
  const uploadPercents = ref<Record<string, number>>({});
  const newTaskUrl = ref('');
  const addingTask = ref(false);
  const addTaskVisible = ref(false);
  const linkParseVisible = ref(false);
  const linkUrl = ref('');
  const linkParsing = ref(false);
  const linkAddingSelected = ref(false);
  const linkResult = ref<DownloadLinkParseResult | null>(null);
  const addingParsedItems = ref<Record<string, boolean>>({});
  const addedParsedItems = ref<Record<string, boolean>>({});
  const selectedParsedItemKeys = ref<string[]>([]);
  const downloadConfigVisible = ref(false);
  let downloadRefreshInterval: number | null = null;

  // Local Maps Logic
  const loadMaps = async () => {
    loading.value = true;
    try {
      maps.value = await api.getMapList();
    } catch (e) {
      console.error(e);
      message.error('加载地图列表失败');
    } finally {
      loading.value = false;
    }
  };

  const filteredMaps = computed(() => {
    if (!searchQuery.value) return maps.value;
    const q = searchQuery.value.toLowerCase();
    return maps.value.filter((m) => {
      if (m.name.toLowerCase().includes(q)) return true;

      const summary = mapSummaries.value[m.name];
      if (!summary) return false;
      if (summary.title?.toLowerCase().includes(q)) return true;
      return summary.campaigns?.some((campaign) => campaign.toLowerCase().includes(q));
    });
  });

  const getMapSizeColor = (sizeStr: string) => {
    // Expecting size format like "123 MB", "1.5 GB", "500 KB"
    if (!sizeStr) return 'default';

    const size = parseFloat(sizeStr);
    const unit = sizeStr
      .replace(/[0-9.]/g, '')
      .trim()
      .toUpperCase();

    let sizeInMB = 0;
    if (unit.includes('G')) {
      sizeInMB = size * 1024;
    } else if (unit.includes('K')) {
      sizeInMB = size / 1024;
    } else {
      sizeInMB = size;
    }

    if (sizeInMB < 200) return 'green';
    if (sizeInMB < 500) return 'orange';
    return 'red';
  };

  const onSelectChange = (keys: any[]) => {
    selectedRowKeys.value = keys;
  };

  const batchDeleteMaps = async () => {
    if (selectedRowKeys.value.length === 0) return;

    Modal.confirm({
      title: `确定要删除选中的 ${selectedRowKeys.value.length} 个地图吗？`,
      icon: () => h(ExclamationCircleOutlined),
      content: '此操作不可逆。',
      onOk: async () => {
        let successCount = 0;
        let failCount = 0;

        for (const name of selectedRowKeys.value) {
          try {
            await api.deleteMap(name);
            successCount++;
          } catch (e) {
            console.error(`Failed to delete ${name}`, e);
            failCount++;
          }
        }

        if (failCount > 0) {
          message.warning(`删除完成: ${successCount} 个成功, ${failCount} 个失败`);
        } else {
          message.success(`成功删除 ${successCount} 个地图`);
        }

        selectedRowKeys.value = [];
        loadMaps();
      },
    });
  };

  const customRequest = async (options: any) => {
    const { file, onSuccess, onError, onProgress } = options;
    const previousState = uploadStates.value[file.name];

    // 取消同文件的上一次上传（如果有）
    const existingController = uploadControllers.value[file.name];
    if (existingController) {
      existingController.abort();
      if (uploadControllers.value[file.name] === existingController) {
        delete uploadControllers.value[file.name];
      }
    }
    if (previousState) {
      delete uploadStates.value[file.name];
      try {
        await api.cancelUpload(previousState.uploadId);
      } catch (e: any) {
        message.warning(`清理 ${file.name} 的上一次上传失败，将由服务器定时清理: ${e.message}`);
      }
    }

    const controller = new AbortController();
    uploadControllers.value[file.name] = controller;
    let currentUploadId = '';

    try {
      const result = await api.uploadMap(
        file,
        ({ percent, speed }: { percent: number; speed: string }) => {
          uploadSpeeds.value[file.name] = speed;
          uploadPercents.value[file.name] = percent;
          onProgress({ percent });
        },
        controller.signal,
        (uploadId: string) => {
          currentUploadId = uploadId;
          uploadStates.value[file.name] = { uploadId };
        }
      );
      delete uploadSpeeds.value[file.name];
      if (uploadControllers.value[file.name] === controller) {
        delete uploadControllers.value[file.name];
      }
      if ('success' in result && result.success) {
        if (uploadStates.value[file.name]?.uploadId === currentUploadId) {
          delete uploadStates.value[file.name];
        }
        delete uploadPercents.value[file.name];
        message.success(`${file.name} 上传成功`);
        onSuccess('Ok');
        loadMaps();
      } else {
        const failed = result as { success: false; uploadId: string; uploadedChunks: number[] };
        uploadStates.value[file.name] = { uploadId: failed.uploadId };
        const currentPercent = uploadPercents.value[file.name] || file.percent || 0;
        message.warning(`${file.name} 上传中断，可点击继续上传恢复`);
        onProgress({ percent: currentPercent });
        onError(new Error('Upload interrupted'));
      }
    } catch (e: any) {
      delete uploadSpeeds.value[file.name];
      if (uploadControllers.value[file.name] === controller) {
        delete uploadControllers.value[file.name];
      }
      const currentPercent = uploadPercents.value[file.name] || file.percent || 0;
      if (e.message === '上传已取消') {
        if (uploadStates.value[file.name]?.uploadId === currentUploadId) {
          delete uploadStates.value[file.name];
        }
        onProgress({ percent: currentPercent });
        onError(e);
        return;
      }
      if (uploadStates.value[file.name]?.uploadId === currentUploadId) {
        delete uploadStates.value[file.name];
      }
      message.error(`上传 ${file.name} 失败: ${e.message}`);
      onProgress({ percent: currentPercent });
      onError(e);
    }
  };

  const resumeUpload = async (fileItem: any) => {
    const state = uploadStates.value[fileItem.name];
    if (!state) return;

    const targetFile = fileList.value.find((f: any) => f.uid === fileItem.uid);
    // 保持当前进度，不要清零
    const currentPercent = targetFile?.percent || 0;
    if (targetFile) {
      targetFile.status = 'uploading';
    }

    const controller = new AbortController();
    uploadControllers.value[fileItem.name] = controller;

    try {
      const fileObj = fileItem.originFileObj || fileItem;
      await api.resumeUpload(
        state.uploadId,
        fileObj,
        ({ percent, speed }: { percent: number; speed: string }) => {
          uploadSpeeds.value[fileItem.name] = speed;
          uploadPercents.value[fileItem.name] = percent;
          if (targetFile) {
            targetFile.percent = percent;
          }
        },
        controller.signal
      );
      delete uploadSpeeds.value[fileItem.name];
      delete uploadStates.value[fileItem.name];
      delete uploadControllers.value[fileItem.name];
      delete uploadPercents.value[fileItem.name];
      message.success(`${fileItem.name} 上传成功`);
      if (targetFile) {
        targetFile.status = 'done';
        targetFile.percent = 100;
      }
      loadMaps();
    } catch (e: any) {
      delete uploadSpeeds.value[fileItem.name];
      delete uploadControllers.value[fileItem.name];
      const failedPercent = uploadPercents.value[fileItem.name] || currentPercent;
      if (e.message === '上传已取消') {
        if (targetFile) {
          targetFile.status = 'error';
          targetFile.percent = failedPercent;
        }
        return;
      }
      message.error(`续传 ${fileItem.name} 失败: ${e.message}`);
      if (targetFile) {
        targetFile.status = 'error';
        targetFile.percent = failedPercent;
      }
    }
  };

  const removeUploadFile = async (uid: string) => {
    const file = fileList.value.find((f: any) => f.uid === uid);
    if (!file) return;
    const state = uploadStates.value[file.name];

    // 如果有正在进行的上传，先取消
    const controller = uploadControllers.value[file.name];
    if (controller) {
      controller.abort();
      if (uploadControllers.value[file.name] === controller) {
        delete uploadControllers.value[file.name];
      }
    }

    // 清理本地状态
    delete uploadStates.value[file.name];
    delete uploadSpeeds.value[file.name];
    delete uploadPercents.value[file.name];

    fileList.value = fileList.value.filter((f: any) => f.uid !== uid);

    if (state) {
      try {
        await api.cancelUpload(state.uploadId);
      } catch (e: any) {
        message.warning(`清理 ${file.name} 的上传临时文件失败，将由服务器定时清理: ${e.message}`);
      }
    }
  };

  const deleteMap = async (name: string) => {
    Modal.confirm({
      title: `确定要删除地图 ${name} 吗？`,
      icon: () => h(ExclamationCircleOutlined),
      content: '此操作不可逆。',
      onOk: async () => {
        try {
          await api.deleteMap(name);
          message.success('删除成功');
          loadMaps();
        } catch (e: any) {
          message.error('删除失败: ' + e.message);
        }
      },
    });
  };

  const trimMap = async (name: string) => {
    Modal.confirm({
      title: `确定要精简地图 ${name} 吗？`,
      icon: () => h(ExclamationCircleOutlined),
      content: '精简成功后会替换当前VPK文件。精简后的地图仅适合服务端使用，不适合客户端本地使用。',
      okText: '精简',
      onOk: async () => {
        trimmingMaps.value = { ...trimmingMaps.value, [name]: true };
        try {
          const result = await api.trimMap(name);
          if (result.trimmed) {
            message.success(result.message || `精简成功，节省 ${result.saved_size_label}`);
          } else {
            message.info(result.message || '当前地图无需精简');
          }
          loadMaps();
          loadMapSummaries([name]);
        } catch (e: any) {
          message.error('精简失败: ' + e.message);
        } finally {
          trimmingMaps.value = { ...trimmingMaps.value, [name]: false };
        }
      },
    });
  };

  const openRenameModal = (name: string) => {
    renameOldName.value = name;
    renameNewName.value = name;
    renameVisible.value = true;
  };

  const openMapDetail = async (name: string) => {
    if (!canOpenMapDetail(name)) return;

    detailMapName.value = name;
    detailCampaigns.value = [];
    detailVisible.value = true;
    detailLoading.value = true;
    try {
      const detail = await api.getMapMissionDetail(name);
      detailMapName.value = detail.name || name;
      detailCampaigns.value = detail.campaigns || [];
    } catch (e: any) {
      message.error('获取地图详情失败: ' + e.message);
      detailVisible.value = false;
    } finally {
      detailLoading.value = false;
    }
  };

  const submitRenameMap = async () => {
    const newName = renameNewName.value.trim();
    if (!renameOldName.value || !newName) {
      message.error('请输入新的地图名称');
      return;
    }

    renamingMap.value = true;
    try {
      const result = await api.renameMap(renameOldName.value, newName);
      selectedRowKeys.value = selectedRowKeys.value.map((key) =>
        key === renameOldName.value ? result.name : key
      );
      message.success(result.message || '重命名成功');
      renameVisible.value = false;
      loadMaps();
    } catch (e: any) {
      message.error('重命名失败: ' + e.message);
    } finally {
      renamingMap.value = false;
    }
  };

  const confirmClearMaps = async () => {
    Modal.confirm({
      title: '警告：这将删除所有第三方地图文件！',
      icon: () => h(ExclamationCircleOutlined),
      content: '确定继续吗？',
      okType: 'danger',
      onOk: async () => {
        try {
          await api.clearMaps();
          message.success('所有地图已清空');
          loadMaps();
        } catch (e: any) {
          message.error('清空失败: ' + e.message);
        }
      },
    });
  };

  const executeHotReloadMaps = async () => {
    hotReloading.value = true;
    try {
      const result = await api.hotReloadMaps();
      message.success(result.message || '地图热重载指令已发送');
    } catch (e: any) {
      message.error('热重载失败: ' + e.message);
    } finally {
      hotReloading.value = false;
    }
  };

  const confirmHotReloadMaps = async () => {
    hotReloading.value = true;
    let usingDefault = false;
    try {
      const status = await api.getMapHotReloadStatus();
      usingDefault = status.using_default;
    } catch (e: any) {
      message.error('获取热重载状态失败: ' + e.message);
      hotReloading.value = false;
      return;
    }
    hotReloading.value = false;

    const content = [
      h('p', '热重载会重新加载地图资源。如果地图过多，会占用 CPU 并影响正在游玩的游戏。'),
    ];
    if (usingDefault) {
      content.push(
        h(
          'p',
          '当前使用默认指令，仅会更新游戏服务器的地图，投票插件的地图缓存不会被刷新。如需同时刷新投票插件缓存，请自定义地图插件的更新指令。'
        )
      );
    }

    Modal.confirm({
      title: '确认热重载地图？',
      icon: () => h(ExclamationCircleOutlined),
      content: h('div', { class: 'space-y-2' }, content),
      okText: '确认热重载',
      cancelText: '取消',
      onOk: executeHotReloadMaps,
    });
  };

  const openHotReloadConfig = () => {
    if (!isAdmin.value) return;
    hotReloadConfigVisible.value = true;
  };

  // Download Tasks Logic
  const loadDownloadTasks = async () => {
    try {
      downloadTasks.value = await api.getDownloadTasks();
    } catch (e) {
      console.error(e);
    }
  };

  const addDownloadTask = async () => {
    if (!newTaskUrl.value) return;
    addingTask.value = true;
    try {
      await api.addDownloadTask(newTaskUrl.value);
      newTaskUrl.value = '';
      message.success('下载任务已添加');
      loadDownloadTasks();
      addTaskVisible.value = false;
    } catch (e: any) {
      message.error('添加任务失败: ' + e.message);
    } finally {
      addingTask.value = false;
    }
  };

  const parsedItems = computed(() => linkResult.value?.items || []);

  const getParsedItemKey = (item: ParsedDownloadItem | Record<string, any>) => {
    return String(item.id || item.file_url || item.filename || item.title || '');
  };

  const getParsedDownloadFilename = (item: ParsedDownloadItem | Record<string, any>) => {
    return String(item.filename || item.title || item.id || 'downloaded_file').trim();
  };

  const isParsedItemAdded = (item: ParsedDownloadItem | Record<string, any>) => {
    return !!addedParsedItems.value[getParsedItemKey(item)];
  };

  const sourceTypeLabel = computed(() => {
    if (!linkResult.value) return '';
    if (linkResult.value.source_type === 'workshop') return 'Steam 工坊';
    if (linkResult.value.source_type === 'qq_flash_transfer') return 'QQ 闪传';
    return linkResult.value.source_type;
  });

  const selectedPendingParsedItems = computed(() => {
    const selected = new Set(selectedParsedItemKeys.value);
    return parsedItems.value.filter(
      (item) => selected.has(getParsedItemKey(item)) && item.supported && !isParsedItemAdded(item)
    );
  });

  const parsedRowSelection = computed(() => ({
    selectedRowKeys: selectedParsedItemKeys.value,
    onChange: (keys: any[]) => {
      selectedParsedItemKeys.value = keys.map(String);
    },
    getCheckboxProps: (record: ParsedDownloadItem | Record<string, any>) => ({
      disabled: !record.supported || isParsedItemAdded(record),
    }),
  }));

  const formatParsedFileSize = (size: string | number) => {
    if (typeof size === 'string' && size.trim() && !/^\d+(\.\d+)?$/.test(size.trim())) {
      return size.trim();
    }
    const bytes = Number(size);
    if (!Number.isFinite(bytes) || bytes <= 0) return '未知大小';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  };

  const parseDownloadLink = async () => {
    const link = linkUrl.value.trim();
    if (!link) {
      message.error('请输入要解析的链接或工坊 ID');
      return;
    }

    linkParsing.value = true;
    linkResult.value = null;
    selectedParsedItemKeys.value = [];
    addedParsedItems.value = {};
    addingParsedItems.value = {};
    try {
      const result = await api.parseDownloadLink(link);
      linkResult.value = result;
      selectedParsedItemKeys.value = result.items
        .filter((item) => item.supported)
        .map((item) => getParsedItemKey(item));
      message.success(`解析成功，共 ${result.items.length} 个文件`);
    } catch (e: any) {
      message.error('解析失败: ' + e.message);
    } finally {
      linkParsing.value = false;
    }
  };

  const openLinkParseModal = () => {
    linkParseVisible.value = true;
  };

  const openDownloadConfig = () => {
    if (!isAdmin.value) return;
    downloadConfigVisible.value = true;
  };

  const addParsedDownload = async (
    item: ParsedDownloadItem | Record<string, any>,
    refresh = true
  ) => {
    if (!item.supported) {
      message.error(item.disabled_reason || '该文件类型暂不支持加入地图下载任务');
      return false;
    }
    if (!item.file_url) {
      message.error('该条目没有可用下载链接');
      return false;
    }

    const itemID = getParsedItemKey(item);
    addingParsedItems.value = { ...addingParsedItems.value, [itemID]: true };
    try {
      await api.addDownloadTask(item.file_url, getParsedDownloadFilename(item), item.referer);
      addedParsedItems.value = { ...addedParsedItems.value, [itemID]: true };
      if (refresh) {
        message.success(`${getParsedDownloadFilename(item)} 已添加到下载任务`);
        loadDownloadTasks();
      }
      return true;
    } catch (e: any) {
      if (refresh) {
        message.error('添加下载任务失败: ' + e.message);
      }
      return false;
    } finally {
      addingParsedItems.value = { ...addingParsedItems.value, [itemID]: false };
    }
  };

  const downloadSelectedParsedItems = async () => {
    const pendingItems = selectedPendingParsedItems.value;
    if (pendingItems.length === 0) {
      message.warning('没有可添加的选中文件');
      return;
    }

    linkAddingSelected.value = true;
    let successCount = 0;
    let failCount = 0;
    try {
      for (const item of pendingItems) {
        const ok = await addParsedDownload(item, false);
        if (ok) {
          successCount++;
        } else {
          failCount++;
        }
      }

      if (successCount > 0) {
        message.success(`已添加 ${successCount} 个下载任务`);
        loadDownloadTasks();
      }
      if (failCount > 0) {
        message.warning(`${failCount} 个任务添加失败`);
      }
      if (successCount > 0 && failCount === 0) {
        linkParseVisible.value = false;
      }
    } finally {
      linkAddingSelected.value = false;
    }
  };

  const cancelTask = async (index: number) => {
    try {
      await api.cancelDownloadTask(index);
      message.success('任务已取消');
      loadDownloadTasks();
    } catch (e: any) {
      message.error('取消任务失败: ' + e.message);
    }
  };

  const restartTask = async (index: number) => {
    try {
      await api.restartDownloadTask(index);
      message.success('任务已重启');
      loadDownloadTasks();
    } catch (e: any) {
      message.error('重启任务失败: ' + e.message);
    }
  };

  const clearDownloadTasks = async () => {
    Modal.confirm({
      title: '确定要清空所有下载记录吗？',
      icon: () => h(ExclamationCircleOutlined),
      onOk: async () => {
        try {
          await api.clearDownloadTasks();
          message.success('记录已清空');
          loadDownloadTasks();
        } catch (e: any) {
          message.error('清空任务失败: ' + e.message);
        }
      },
    });
  };

  const mapColumns = [
    { title: '地图名称', dataIndex: 'name', key: 'name' },
    { title: '大小', dataIndex: 'size', key: 'size', width: 120 },
    { title: '操作', key: 'action', width: 330, align: 'right' as const },
  ];

  const mapDetailChapterColumns = [
    { title: '章节名', dataIndex: 'Title', key: 'title', width: 180 },
    { title: '地图代码', dataIndex: 'Code', key: 'code', width: 150 },
    { title: '模式', key: 'modes', width: 360 },
  ];

  const taskColumns = [
    { title: '文件/URL', dataIndex: 'filename', key: 'filename' },
    { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
    { title: '大小', dataIndex: 'formattedSize', key: 'size', width: 120 },
    { title: '速度', dataIndex: 'formattedSpeed', key: 'speed', width: 130 },
    { title: '进度', dataIndex: 'progress', key: 'progress', width: 200 },
    { title: '操作', key: 'action', width: 80, align: 'right' as const },
  ];

  const parsedLinkColumns = [
    { title: '预览', key: 'preview', width: 84 },
    { title: '文件', key: 'info' },
    { title: '大小', key: 'size', width: 120 },
    { title: '状态', key: 'support', width: 150 },
    { title: '操作', key: 'action', width: 110 },
  ];

  const getFileNameFromUrl = (url: string) => {
    if (!url) return 'Unknown';
    try {
      const parts = url.split('/');
      const lastPart = parts[parts.length - 1];
      if (!lastPart) return 'Unknown';
      const filename = lastPart.split('?')[0];
      return filename ? decodeURIComponent(filename) : filename;
    } catch {
      return 'Unknown';
    }
  };

  const startPolling = () => {
    if (downloadRefreshInterval) return;
    loadDownloadTasks(); // Immediate load
    downloadRefreshInterval = window.setInterval(loadDownloadTasks, 3000);
  };

  const stopPolling = () => {
    if (downloadRefreshInterval) {
      clearInterval(downloadRefreshInterval);
      downloadRefreshInterval = null;
    }
  };

  watch(activeTab, (newTab) => {
    if (newTab === 'download') {
      startPolling();
    } else {
      stopPolling();
    }
  });

  const paginationConfig = reactive<TablePaginationConfig>({
    current: 1,
    pageSize: 10,
    showSizeChanger: true,
    pageSizeOptions: ['10', '20', '50', '100'],
    showTotal: (total: number) => `共 ${total} 条`,
  });

  const handleTableChange = (pag: TablePaginationConfig) => {
    paginationConfig.current = pag.current;
    paginationConfig.pageSize = pag.pageSize;
  };

  const currentPageMaps = computed(() => {
    const current = paginationConfig.current || 1;
    const pageSize = paginationConfig.pageSize || 10;
    const start = (current - 1) * pageSize;
    return filteredMaps.value.slice(start, start + pageSize);
  });

  const currentPageMapNames = computed(() => currentPageMaps.value.map((map) => map.name));
  const currentPageMapNamesKey = computed(() => currentPageMapNames.value.join('\0'));

  const loadMapSummaries = async (names: string[]) => {
    const uniqueNames = Array.from(new Set(names.filter(Boolean)));

    if (uniqueNames.length === 0) {
      return;
    }

    const loadingPatch = uniqueNames.reduce<Record<string, boolean>>((acc, name) => {
      acc[name] = true;
      return acc;
    }, {});
    mapSummaryLoading.value = { ...mapSummaryLoading.value, ...loadingPatch };

    try {
      const items = await api.getMapSummaries(uniqueNames);
      mapSummaries.value = { ...mapSummaries.value, ...items };
    } catch (e) {
      console.error('Failed to load map summaries', e);
    } finally {
      const nextLoading = { ...mapSummaryLoading.value };
      uniqueNames.forEach((name) => {
        nextLoading[name] = false;
      });
      mapSummaryLoading.value = nextLoading;
    }
  };

  const loadCurrentPageMapSummaries = () => loadMapSummaries(currentPageMapNames.value);

  const loadSearchMapSummaries = () => {
    if (!searchQuery.value.trim()) return;
    loadMapSummaries(maps.value.map((map) => map.name));
  };

  const getMapSummaryTitle = (name: string) => mapSummaries.value[name]?.title || '';
  const getMapSummaryChapterCount = (name: string) => mapSummaries.value[name]?.chapter_count || 0;
  const canOpenMapDetail = (name: string) => getMapSummaryChapterCount(name) > 0;
  const getMapSummaryError = (name: string) => mapSummaries.value[name]?.error || '';
  const isMapSummaryLoading = (name: string) => !!mapSummaryLoading.value[name];

  const getMapInspection = (name: string) => mapSummaries.value[name]?.inspection;

  const getMissingDictionaryChapters = (name: string) =>
    (getMapInspection(name)?.dictionary.chapters || []).filter(
      (chapter) => chapter.status === 'missing'
    );

  const getUnreadableDictionaryChapters = (name: string) =>
    (getMapInspection(name)?.dictionary.chapters || []).filter(
      (chapter) => chapter.status === 'unreadable'
    );

  const hasDictionaryInspectionError = (name: string) =>
    getMapInspection(name)?.dictionary.status === 'unreadable' ||
    getUnreadableDictionaryChapters(name).length > 0;

  const getGlobalScriptCount = (name: string) => {
    const globalScriptsInspection = getMapInspection(name)?.global_scripts;
    if (globalScriptsInspection?.status !== 'detected') return 0;
    return globalScriptsInspection.files?.length || 0;
  };

  const getChapterDisplayName = (chapter: MapDictionaryChapterInspection) => {
    const campaignTitle = chapter.campaign_title?.trim();
    const chapterTitle = chapter.chapter_title?.trim();
    if (campaignTitle && chapterTitle) return `${campaignTitle} / ${chapterTitle}`;
    if (chapterTitle) return chapterTitle;
    if (campaignTitle) return campaignTitle;
    return chapter.chapter_code?.trim() || chapter.bsp_path?.trim() || '未知章节';
  };

  const getChapterDisplayCode = (chapter: MapDictionaryChapterInspection) => {
    if (!chapter.campaign_title?.trim() && !chapter.chapter_title?.trim()) return '';
    return chapter.chapter_code?.trim() || chapter.bsp_path?.trim() || '';
  };

  const activatePopoverTag = (event: KeyboardEvent) => {
    (event.currentTarget as HTMLElement | null)?.click();
  };

  const openGlobalScripts = (mapName: string) => {
    globalScriptsMapName.value = mapName;
    globalScriptsVisible.value = true;
  };

  const handleGlobalScriptsUpdated = (mapName: string) => {
    void Promise.all([loadMaps(), loadMapSummaries([mapName])]);
  };

  watch(searchQuery, () => {
    paginationConfig.current = 1;
    loadSearchMapSummaries();
  });

  watch(maps, () => {
    loadSearchMapSummaries();
  });

  watch(currentPageMapNamesKey, () => {
    loadCurrentPageMapSummaries();
  });

  const taskPaginationConfig = reactive<TablePaginationConfig>({
    current: 1,
    pageSize: 10,
    showSizeChanger: true,
    pageSizeOptions: ['10', '20', '50', '100'],
    showTotal: (total: number) => `共 ${total} 条`,
  });

  const handleTaskTableChange = (pag: TablePaginationConfig) => {
    taskPaginationConfig.current = pag.current;
    taskPaginationConfig.pageSize = pag.pageSize;
  };

  onMounted(() => {
    loadMaps();
    if (activeTab.value === 'download') {
      startPolling();
    }
  });

  onUnmounted(() => {
    stopPolling();
  });
</script>

<template>
  <div class="h-full">
    <a-tabs v-model:activeKey="activeTab" type="card">
      <a-tab-pane key="local" tab="地图管理">
        <div class="space-y-4">
          <!-- Actions Bar -->
          <div class="flex flex-col md:flex-row justify-between gap-4">
            <div class="w-full md:w-1/3">
              <a-input-search v-model:value="searchQuery" placeholder="搜索地图..." allow-clear />
            </div>
            <div class="flex gap-2 flex-wrap">
              <a-button
                v-if="selectedRowKeys.length > 0"
                danger
                @click="batchDeleteMaps"
                class="!flex !items-center !justify-center"
              >
                <template #icon><delete-outlined /></template>
                <span class="hidden sm:inline">删除选中</span>
                <span class="sm:hidden">删除</span>
                ({{ selectedRowKeys.length }})
              </a-button>
              <a-button
                @click="loadMaps"
                :loading="loading"
                class="!flex !items-center !justify-center"
              >
                <template #icon><reload-outlined /></template>
                刷新
              </a-button>
              <a-button-group class="!flex">
                <a-button
                  @click="confirmHotReloadMaps"
                  :loading="hotReloading"
                  class="!flex !items-center !justify-center"
                >
                  <template #icon><reload-outlined /></template>
                  热重载地图
                </a-button>
                <a-button
                  v-if="isAdmin"
                  @click="openHotReloadConfig"
                  class="!flex !items-center !justify-center"
                  title="设置热重载指令"
                >
                  <template #icon><setting-outlined /></template>
                </a-button>
              </a-button-group>
              <a-button
                danger
                @click="confirmClearMaps"
                class="!flex !items-center !justify-center"
              >
                <template #icon><delete-outlined /></template>
                清空地图
              </a-button>
            </div>
          </div>

          <!-- Maps Table -->
          <a-table
            :columns="mapColumns"
            :dataSource="filteredMaps"
            :loading="loading"
            :pagination="paginationConfig"
            @change="handleTableChange"
            rowKey="name"
            :row-selection="{ selectedRowKeys: selectedRowKeys, onChange: onSelectChange }"
            :scroll="{ x: 940 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <div class="min-w-[180px]">
                  <div class="min-w-0 flex flex-col gap-0.5">
                    <span class="font-medium break-all text-sm dark:text-gray-200">{{
                      record.name
                    }}</span>
                    <span
                      v-if="isMapSummaryLoading(record.name)"
                      class="text-xs text-gray-400 dark:text-gray-500"
                    >
                      读取中...
                    </span>
                    <span
                      v-else-if="getMapSummaryTitle(record.name)"
                      class="text-xs text-gray-500 dark:text-gray-400 truncate"
                      :title="getMapSummaryTitle(record.name)"
                    >
                      {{ getMapSummaryTitle(record.name) }}
                      <template v-if="getMapSummaryChapterCount(record.name) > 0">
                        · {{ getMapSummaryChapterCount(record.name) }} 章节
                      </template>
                    </span>
                    <span
                      v-else-if="getMapSummaryError(record.name)"
                      class="text-xs text-gray-400 dark:text-gray-500"
                      :title="getMapSummaryError(record.name)"
                    >
                      未识别
                    </span>
                    <div
                      v-if="!isMapSummaryLoading(record.name)"
                      class="mt-1 flex max-w-full flex-wrap gap-1"
                    >
                      <a-popover
                        v-if="getMissingDictionaryChapters(record.name).length > 0"
                        title="缺失字典的章节"
                        trigger="click"
                        placement="bottomLeft"
                        overlayClassName="map-inspection-popover"
                      >
                        <template #content>
                          <div class="map-inspection-list" role="list">
                            <div
                              v-for="chapter in getMissingDictionaryChapters(record.name)"
                              :key="chapter.bsp_path"
                              class="map-inspection-item"
                              role="listitem"
                            >
                              <div class="map-inspection-primary">
                                {{ getChapterDisplayName(chapter) }}
                              </div>
                              <div
                                v-if="getChapterDisplayCode(chapter)"
                                class="map-inspection-secondary"
                              >
                                {{ getChapterDisplayCode(chapter) }}
                              </div>
                            </div>
                          </div>
                        </template>
                        <a-tag
                          color="red"
                          class="clickable-risk-tag"
                          role="button"
                          tabindex="0"
                          @keydown.enter.prevent.stop="activatePopoverTag"
                          @keydown.space.prevent.stop="activatePopoverTag"
                        >
                          字典缺失 {{ getMissingDictionaryChapters(record.name).length }}
                        </a-tag>
                      </a-popover>

                      <a-popover
                        v-if="hasDictionaryInspectionError(record.name)"
                        title="字典检测异常"
                        trigger="click"
                        placement="bottomLeft"
                        overlayClassName="map-inspection-popover"
                      >
                        <template #content>
                          <div
                            v-if="getUnreadableDictionaryChapters(record.name).length > 0"
                            class="map-inspection-list"
                            role="list"
                          >
                            <div
                              v-for="chapter in getUnreadableDictionaryChapters(record.name)"
                              :key="chapter.bsp_path"
                              class="map-inspection-item"
                              role="listitem"
                            >
                              <div class="map-inspection-primary">{{ chapter.bsp_path }}</div>
                              <div class="map-inspection-secondary">
                                {{ chapter.message || '无法解析 BSP' }}
                              </div>
                            </div>
                          </div>
                          <div v-else class="map-inspection-empty">
                            无法解析 VPK 目录或 BSP 内容。
                          </div>
                        </template>
                        <a-tag
                          color="orange"
                          class="clickable-risk-tag"
                          role="button"
                          tabindex="0"
                          @keydown.enter.prevent.stop="activatePopoverTag"
                          @keydown.space.prevent.stop="activatePopoverTag"
                        >
                          字典检测异常
                        </a-tag>
                      </a-popover>

                      <a-tag
                        v-if="getGlobalScriptCount(record.name) > 0"
                        color="orange"
                        class="clickable-risk-tag"
                        role="button"
                        tabindex="0"
                        @click="openGlobalScripts(record.name)"
                        @keydown.enter.prevent.stop="openGlobalScripts(record.name)"
                        @keydown.space.prevent.stop="openGlobalScripts(record.name)"
                      >
                        存在全局脚本 {{ getGlobalScriptCount(record.name) }}
                      </a-tag>
                    </div>
                  </div>
                </div>
              </template>
              <template v-else-if="column.key === 'size'">
                <a-tag :color="getMapSizeColor(record.size)">
                  {{ record.size }}
                </a-tag>
              </template>
              <template v-else-if="column.key === 'action'">
                <a-space>
                  <a-button
                    size="small"
                    type="text"
                    :disabled="record.size === 'unknown' || !canOpenMapDetail(record.name)"
                    @click="openMapDetail(record.name)"
                    class="!flex !items-center !justify-center"
                    :title="canOpenMapDetail(record.name) ? '详情' : '未识别到章节，详情不可用'"
                  >
                    <template #icon><file-text-outlined /></template>
                    <span class="hidden sm:inline">详情</span>
                  </a-button>
                  <a-button
                    v-if="record.size !== 'unknown'"
                    size="small"
                    type="text"
                    @click="openRenameModal(record.name)"
                    class="!flex !items-center !justify-center"
                    title="重命名"
                  >
                    <template #icon><edit-outlined /></template>
                    <span class="hidden sm:inline">重命名</span>
                  </a-button>
                  <a-button
                    v-if="record.size !== 'unknown'"
                    size="small"
                    type="text"
                    :loading="trimmingMaps[record.name]"
                    :disabled="trimmingMaps[record.name]"
                    @click="trimMap(record.name)"
                    class="!flex !items-center !justify-center"
                    title="精简"
                  >
                    <template #icon><compress-outlined /></template>
                    <span class="hidden sm:inline">精简</span>
                  </a-button>
                  <a-button
                    size="small"
                    danger
                    type="text"
                    @click="deleteMap(record.name)"
                    class="!flex !items-center !justify-center"
                    title="删除"
                  >
                    <template #icon><delete-outlined /></template>
                    <span class="hidden sm:inline">删除</span>
                  </a-button>
                </a-space>
              </template>
            </template>
          </a-table>
        </div>

        <a-modal
          v-model:open="renameVisible"
          title="重命名地图"
          @ok="submitRenameMap"
          :confirmLoading="renamingMap"
        >
          <div class="space-y-3">
            <div class="text-sm text-gray-500 dark:text-gray-400 break-all">
              当前名称: {{ renameOldName }}
            </div>
            <a-input
              v-model:value="renameNewName"
              placeholder="请输入新的地图名称"
              @pressEnter="submitRenameMap"
            />
            <div class="text-xs text-gray-400 dark:text-gray-500">
              保存时会自动清理特殊字符，未填写 .vpk 时会自动补齐。
            </div>
          </div>
        </a-modal>

        <a-modal
          v-model:open="hotReloadConfigVisible"
          title="热重载地图设置"
          :footer="null"
        >
          <MapHotReloadSetting
            :active="hotReloadConfigVisible"
            context="modal"
            @saved="hotReloadConfigVisible = false"
            @cancel="hotReloadConfigVisible = false"
          />
        </a-modal>

        <a-modal
          v-model:open="detailVisible"
          :title="detailModalTitle"
          :footer="null"
          width="820px"
          wrap-class-name="map-detail-modal"
        >
          <div v-if="detailLoading" class="py-10 text-center text-gray-400">加载中...</div>
          <a-empty v-else-if="detailCampaigns.length === 0" description="未解析到战役信息" />
          <div v-else class="space-y-5">
            <div
              class="rounded-lg border border-gray-200 bg-gray-50/80 p-4 dark:border-slate-700 dark:bg-slate-900/30"
            >
              <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div class="flex min-w-0 items-start gap-3">
                  <div
                    class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-500/10 dark:text-blue-300"
                  >
                    <file-text-outlined />
                  </div>
                  <div class="min-w-0">
                    <div class="text-xs font-medium text-gray-500 dark:text-gray-400">战役名</div>
                    <div
                      class="mt-1 break-words text-lg font-semibold text-gray-900 dark:text-gray-100"
                    >
                      {{ detailCampaignTitle }}
                    </div>
                  </div>
                </div>
                <div class="min-w-0 sm:max-w-[45%] sm:text-right">
                  <div class="text-xs font-medium text-gray-500 dark:text-gray-400">VPK 文件</div>
                  <div
                    class="mt-1 truncate text-sm font-medium text-gray-700 dark:text-gray-200"
                    :title="detailMapName"
                  >
                    {{ detailMapName }}
                  </div>
                </div>
              </div>
            </div>

            <div
              v-for="campaign in detailCampaigns"
              :key="`${campaign.VpkName || detailMapName}-${campaign.Title}`"
              class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-slate-700 dark:bg-slate-900/40"
            >
              <div
                v-if="detailCampaigns.length > 1"
                class="border-b border-gray-100 bg-gray-50/70 px-4 py-3 dark:border-slate-700 dark:bg-slate-800/40"
              >
                <div class="break-words text-base font-semibold text-gray-900 dark:text-gray-100">
                  {{ campaign.Title || '未命名战役' }}
                </div>
                <div class="mt-1 break-all text-xs text-gray-500 dark:text-gray-400">
                  {{ campaign.VpkName || detailMapName }}
                </div>
              </div>

              <a-table
                class="map-detail-chapter-table"
                :columns="mapDetailChapterColumns"
                :data-source="campaign.Chapters || []"
                :pagination="false"
                :row-key="(chapter) => chapter.Code || chapter.Title"
                size="small"
                table-layout="fixed"
                :scroll="{ x: 690 }"
              >
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'title'">
                    <span class="block truncate pr-3" :title="record.Title || '-'">
                      {{ record.Title || '-' }}
                    </span>
                  </template>
                  <template v-else-if="column.key === 'code'">
                    <code
                      class="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-700 dark:bg-slate-800 dark:text-gray-200"
                      >{{ record.Code || '-' }}</code
                    >
                  </template>
                  <template v-else-if="column.key === 'modes'">
                    <div v-if="record.Modes?.length" class="flex min-w-[260px] flex-wrap gap-1">
                      <a-tag
                        v-for="mode in record.Modes"
                        :key="mode"
                        color="default"
                        class="mr-0 map-detail-mode-tag"
                      >
                        {{ mode }}
                      </a-tag>
                    </div>
                    <span v-else class="text-gray-400">-</span>
                  </template>
                </template>
              </a-table>
            </div>
          </div>
        </a-modal>

        <MapGlobalScriptsModal
          v-model:open="globalScriptsVisible"
          :map-name="globalScriptsMapName"
          @updated="handleGlobalScriptsUpdated"
        />
      </a-tab-pane>

      <a-tab-pane key="upload" tab="上传地图">
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
              <inbox-outlined />
            </p>
            <p class="ant-upload-text">点击或拖拽上传地图文件</p>
            <p class="ant-upload-hint">支持 .vpk, .zip, .rar, .7z 格式</p>
          </a-upload-dragger>

          <!-- 自定义上传列表 -->
          <div v-if="fileList.length > 0" class="space-y-2 mt-4">
            <div
              v-for="file in fileList"
              :key="file.uid"
              class="flex flex-col gap-1 p-2 rounded border border-gray-200 dark:border-gray-700"
            >
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2 min-w-0">
                  <span class="text-sm truncate" :title="file.name">{{ file.name }}</span>
                  <span
                    v-if="file.status === 'uploading' && uploadSpeeds[file.name]"
                    class="text-xs text-gray-500 whitespace-nowrap"
                  >
                    {{ uploadSpeeds[file.name] }}
                  </span>
                  <span
                    v-if="file.status === 'error'"
                    class="text-xs text-red-500 whitespace-nowrap"
                  >
                    上传中断
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
                    <template #icon><play-circle-outlined /></template>
                    继续上传
                  </a-button>
                  <a-button
                    v-if="file.status === 'uploading'"
                    type="text"
                    size="small"
                    danger
                    @click="removeUploadFile(file.uid)"
                  >
                    取消
                  </a-button>
                  <a-button
                    v-else
                    type="text"
                    size="small"
                    danger
                    @click="removeUploadFile(file.uid)"
                  >
                    <template #icon><close-circle-outlined /></template>
                  </a-button>
                </a-space>
              </div>
              <a-progress
                v-if="
                  file.status === 'uploading' || file.status === 'done' || file.status === 'error'
                "
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
      </a-tab-pane>

      <a-tab-pane key="download" tab="下载任务">
        <div class="space-y-4">
          <!-- Add Task & Actions -->
          <div class="flex flex-wrap justify-between items-center gap-2">
            <div class="flex gap-2 flex-wrap">
              <a-button
                type="primary"
                @click="addTaskVisible = true"
                class="!flex !items-center !justify-center"
              >
                <template #icon><plus-outlined /></template>
                添加任务
              </a-button>
              <a-button-group class="!flex">
                <a-button
                  @click="openLinkParseModal"
                  class="!flex !items-center !justify-center"
                >
                  <template #icon><search-outlined /></template>
                  解析链接
                </a-button>
                <a-button
                  v-if="isAdmin"
                  @click="openDownloadConfig"
                  class="!flex !items-center !justify-center"
                  title="下载设置"
                  aria-label="下载设置"
                >
                  <template #icon><setting-outlined /></template>
                </a-button>
              </a-button-group>
            </div>

            <a-button
              v-if="downloadTasks.length > 0"
              size="small"
              danger
              type="text"
              @click="clearDownloadTasks"
              class="!flex !items-center !justify-center"
            >
              清空记录
            </a-button>
          </div>

          <!-- Task List -->
          <a-table
            :columns="taskColumns"
            :dataSource="downloadTasks"
            :pagination="taskPaginationConfig"
            @change="handleTaskTableChange"
            :rowKey="(_: any, index?: number) => index || 0"
            :scroll="{ x: 500 }"
          >
            <template #bodyCell="{ column, record, index }">
              <template v-if="column.key === 'filename'">
                <div class="flex flex-col gap-1 min-w-[120px]">
                  <div
                    class="font-bold text-sm truncate"
                    :title="record.filename || getFileNameFromUrl(record.url)"
                  >
                    {{ record.filename || getFileNameFromUrl(record.url) }}
                  </div>
                  <div
                    class="text-xs text-gray-400 truncate max-w-[150px] md:max-w-md"
                    :title="record.url"
                  >
                    {{ record.url }}
                  </div>
                  <div v-if="record.status === 3" class="text-xs text-red-500 break-words">
                    失败原因: {{ record.message }}
                  </div>
                </div>
              </template>
              <template v-else-if="column.key === 'status'">
                <div class="min-w-[80px]">
                  <a-tag
                    class="mr-0"
                    :color="
                      record.status === 2
                        ? 'success'
                        : record.status === 1
                          ? 'processing'
                          : record.status === 3
                            ? 'error'
                            : 'default'
                    "
                  >
                    {{
                      record.status === 2
                        ? '已完成'
                        : record.status === 1
                          ? '下载中'
                          : record.status === 3
                            ? '失败'
                            : '等待中'
                    }}
                  </a-tag>
                </div>
              </template>
              <template v-else-if="column.key === 'size'">
                <a-tag class="mr-0">
                  {{ record.formattedSize || '未知大小' }}
                </a-tag>
              </template>
              <template v-else-if="column.key === 'speed'">
                <span class="text-xs text-gray-600 dark:text-gray-300 whitespace-nowrap">
                  {{ record.status === 1 ? record.formattedSpeed || '0 B/s' : '-' }}
                </span>
              </template>
              <template v-else-if="column.key === 'progress'">
                <div class="flex flex-col items-start gap-1 min-w-[100px]">
                  <a-progress
                    :percent="Number((record.progress || 0).toFixed(1))"
                    size="small"
                    :show-info="false"
                    :status="
                      record.status === 3 ? 'exception' : record.status === 2 ? 'success' : 'active'
                    "
                  />
                  <span class="text-xs text-gray-500 dark:text-gray-400">
                    {{ (record.progress || 0).toFixed(1) }}%
                  </span>
                </div>
              </template>
              <template v-else-if="column.key === 'action'">
                <a-space>
                  <a-button
                    v-if="record.status === 0 || record.status === 1"
                    type="text"
                    size="small"
                    danger
                    @click="index !== undefined && cancelTask(index)"
                    class="!flex !items-center !justify-center"
                    title="取消"
                  >
                    <template #icon><close-circle-outlined /></template>
                  </a-button>
                  <a-button
                    v-if="record.status === 3"
                    type="text"
                    size="small"
                    @click="index !== undefined && restartTask(index)"
                    class="!flex !items-center !justify-center"
                    title="重试"
                  >
                    <template #icon><reload-outlined /></template>
                  </a-button>
                </a-space>
              </template>
            </template>
          </a-table>
        </div>

        <a-modal
          v-model:open="addTaskVisible"
          title="添加下载任务"
          @ok="addDownloadTask"
          :confirmLoading="addingTask"
        >
          <a-textarea
            v-model:value="newTaskUrl"
            placeholder="请输入下载链接，支持多个链接（每行一个或空格分隔）
支持 .vpk, .zip, .rar, .7z 格式"
            :rows="6"
          />
        </a-modal>

        <a-modal
          v-model:open="downloadConfigVisible"
          title="下载设置"
          :width="520"
          :footer="null"
          wrap-class-name="download-config-modal"
        >
          <SteamCDNSetting
            :active="downloadConfigVisible"
            context="modal"
            @saved="downloadConfigVisible = false"
            @cancel="downloadConfigVisible = false"
          />
        </a-modal>

        <a-modal v-model:open="linkParseVisible" title="解析链接" width="920px" :footer="null">
          <div class="space-y-4">
            <a-input-search
              v-model:value="linkUrl"
              placeholder="粘贴 Steam 工坊链接、合集链接、工坊 ID 或 QQ 闪传链接"
              enter-button="解析"
              :loading="linkParsing"
              @search="parseDownloadLink"
            />

            <div v-if="linkResult" class="space-y-3">
              <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
                <div class="text-sm text-gray-600 dark:text-gray-400">
                  解析到 {{ parsedItems.length }} 个文件
                  <span class="ml-2 text-xs text-gray-400">
                    来源: {{ sourceTypeLabel }} · ID: {{ linkResult.source_id }}
                  </span>
                </div>
                <a-button
                  type="primary"
                  :loading="linkAddingSelected"
                  :disabled="selectedPendingParsedItems.length === 0"
                  @click="downloadSelectedParsedItems"
                  class="!flex !items-center !justify-center"
                >
                  <template #icon><download-outlined /></template>
                  添加选中
                </a-button>
              </div>

              <a-table
                :columns="parsedLinkColumns"
                :dataSource="parsedItems"
                :pagination="{ pageSize: 8, showSizeChanger: false }"
                :rowKey="(record: any) => getParsedItemKey(record)"
                :row-selection="parsedRowSelection"
                size="small"
                :scroll="{ x: 720 }"
              >
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'preview'">
                    <div
                      class="w-14 h-14 rounded border border-gray-200 dark:border-gray-700 overflow-hidden bg-gray-50 dark:bg-gray-800 flex items-center justify-center"
                    >
                      <img
                        v-if="record.preview_url"
                        :src="record.preview_url"
                        :alt="record.title"
                        class="w-full h-full object-cover"
                        loading="lazy"
                      />
                      <file-text-outlined v-else class="text-xl text-gray-400" />
                    </div>
                  </template>
                  <template v-else-if="column.key === 'info'">
                    <div class="min-w-[260px]">
                      <div class="font-medium text-sm break-words dark:text-gray-100">
                        {{ record.title || record.filename || record.id }}
                      </div>
                      <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                        {{ record.filename }}
                      </div>
                    </div>
                  </template>
                  <template v-else-if="column.key === 'size'">
                    <a-tag>{{ formatParsedFileSize(record.file_size) }}</a-tag>
                  </template>
                  <template v-else-if="column.key === 'support'">
                    <a-tag v-if="record.supported" color="success" class="mr-0">可添加</a-tag>
                    <a-tag v-else color="default" class="mr-0">
                      {{ record.disabled_reason || '不支持' }}
                    </a-tag>
                  </template>
                  <template v-else-if="column.key === 'action'">
                    <a-button
                      size="small"
                      type="primary"
                      :loading="addingParsedItems[getParsedItemKey(record)]"
                      :disabled="!record.supported || isParsedItemAdded(record)"
                      @click="addParsedDownload(record)"
                      class="!flex !items-center !justify-center"
                    >
                      <template #icon><download-outlined /></template>
                      {{ isParsedItemAdded(record) ? '已添加' : '添加' }}
                    </a-button>
                  </template>
                </template>
              </a-table>
            </div>
          </div>
        </a-modal>
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<style scoped>
  :deep(.clickable-risk-tag) {
    margin-inline-end: 0;
    cursor: pointer;
    font-size: 12px;
    user-select: none;
  }

  :deep(.clickable-risk-tag:focus-visible) {
    outline: 2px solid #1677ff;
    outline-offset: 2px;
  }

  :global(.map-inspection-popover) {
    max-width: min(420px, calc(100vw - 24px));
  }

  :global(.map-inspection-popover .ant-popover-inner) {
    max-width: min(420px, calc(100vw - 24px));
  }

  :global(.map-inspection-list) {
    display: flex;
    max-height: min(420px, 55vh);
    min-width: min(300px, calc(100vw - 64px));
    flex-direction: column;
    gap: 8px;
    overflow-y: auto;
  }

  :global(.map-inspection-item) {
    border-bottom: 1px solid #f0f0f0;
    padding-bottom: 8px;
    overflow-wrap: anywhere;
  }

  :global(.map-inspection-item:last-child) {
    border-bottom: 0;
    padding-bottom: 0;
  }

  :global(.map-inspection-primary) {
    color: #1f2937;
    font-size: 13px;
    font-weight: 600;
    line-height: 1.45;
  }

  :global(.map-inspection-secondary),
  :global(.map-inspection-empty) {
    margin-top: 2px;
    color: #6b7280;
    font-size: 12px;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }

  :global(.dark .map-inspection-item) {
    border-bottom-color: #334155;
  }

  :global(.dark .map-inspection-primary) {
    color: #e2e8f0;
  }

  :global(.dark .map-inspection-secondary),
  :global(.dark .map-inspection-empty) {
    color: #94a3b8;
  }

  :global(.map-detail-modal .ant-modal-body) {
    padding-top: 14px;
  }

  :global(.dark .map-detail-modal) {
    --ant-color-split: #334155;
    --ant-color-border-secondary: #334155;
  }

  :global(.download-config-modal .ant-modal) {
    max-width: calc(100vw - 24px);
  }

  :global(.download-config-modal .ant-modal-body) {
    overflow-wrap: anywhere;
  }

  @media (max-width: 640px) {
    :global(.map-inspection-list) {
      min-width: 0;
      width: calc(100vw - 64px);
    }

    :global(.download-config-modal .ant-modal) {
      top: 12px;
      margin: 0 auto;
      padding-bottom: 12px;
    }

    :global(.download-config-modal .ant-modal-body) {
      max-height: calc(100vh - 170px);
      overflow-y: auto;
    }
  }

  :deep(.map-detail-chapter-table .ant-table) {
    border-radius: 0;
    background: transparent;
  }

  :deep(.map-detail-chapter-table .ant-table-thead > tr > th) {
    background: transparent;
    color: #4b5563;
    font-size: 12px;
    font-weight: 600;
    padding: 10px 16px;
    border-bottom: 0 !important;
  }

  :deep(.map-detail-chapter-table .ant-table-thead),
  :deep(.map-detail-chapter-table .ant-table-thead > tr) {
    border-bottom: 0 !important;
  }

  :deep(.map-detail-chapter-table .ant-table-thead > tr > th::before) {
    display: none !important;
  }

  :deep(.map-detail-chapter-table .ant-table-tbody > tr > td) {
    padding: 10px 16px;
    border-bottom-color: #f1f5f9;
  }

  :deep(.map-detail-chapter-table .ant-table-tbody > tr:last-child > td) {
    border-bottom: 0;
  }

  :deep(.map-detail-mode-tag) {
    line-height: 20px;
    border-radius: 5px;
  }

  :global(.dark .map-detail-chapter-table .ant-table-thead > tr > th) {
    color: #cbd5e1;
    border-bottom: 0 !important;
  }

  :global(.dark .map-detail-chapter-table .ant-table-thead),
  :global(.dark .map-detail-chapter-table .ant-table-thead > tr) {
    border-bottom: 0 !important;
  }

  :global(.dark .map-detail-chapter-table .ant-table-tbody > tr > td) {
    border-bottom: 1px solid #1e293b !important;
  }

  :global(.dark .map-detail-chapter-table .ant-table-header),
  :global(.dark .map-detail-chapter-table .ant-table-container),
  :global(.dark .map-detail-chapter-table .ant-table-content),
  :global(.dark .map-detail-chapter-table table) {
    border-color: #334155 !important;
  }

  /* Dark mode overrides for Upload Dragger */
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

  /* Target the icon specifically */
  :global(.dark .ant-upload.ant-upload-drag .anticon) {
    color: #3b82f6 !important;
  }
</style>
