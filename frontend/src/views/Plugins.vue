<script setup lang="ts">
  import {
    ref,
    computed,
    onMounted,
    onUnmounted,
    onErrorCaptured,
    reactive,
    watch,
    nextTick,
  } from 'vue';
  import {
    message,
    Card as ACard,
    Tabs as ATabs,
    TabPane as ATabPane,
    Input as AInput,
    Button as AButton,
    Table as ATable,
    Popconfirm as APopconfirm,
    Upload as AUpload,
    RadioGroup as ARadioGroup,
    Alert as AAlert,
    Drawer as ADrawer,
    Select as ASelect,
    Tag as ATag,
    Modal as AModal,
    Progress as AProgress,
    InputPassword as AInputPassword,
    Dropdown as ADropdown,
    Menu as AMenu,
    MenuItem as AMenuItem,
  } from 'ant-design-vue';
  import {
    UploadOutlined,
    DeleteOutlined,
    PoweroffOutlined,
    SearchOutlined,
    ReloadOutlined,
    SettingOutlined,
    SyncOutlined,
    AppstoreAddOutlined,
    DownloadOutlined,
    CheckCircleOutlined,
    LinkOutlined,
    FileTextOutlined,
    DownOutlined,
  } from '@ant-design/icons-vue';
  import { api, type PluginExportProgress } from '../services/api';
  import type { UploadProps, TablePaginationConfig } from 'ant-design-vue';
  import PluginConfigModal from '../components/PluginConfigModal.vue';
  import PluginDetailModal from '../components/PluginDetailModal.vue';
  import { useAuthStore } from '../stores/auth';

  const authStore = useAuthStore();
  const isMobile = ref(window.innerWidth < 768);

  const handleResize = () => {
    isMobile.value = window.innerWidth < 768;
  };

  onMounted(() => {
    fetchPlugins();
    window.addEventListener('resize', handleResize);
    // Load saved GitHub token
    const savedToken = localStorage.getItem('l4d2_manager_github_token');
    if (savedToken) {
      githubToken.value = savedToken;
    }
    // Load saved custom repo
    const savedRepo = localStorage.getItem('l4d2_manager_plugin_repo');
    if (customRepo.value.length === 0 && savedRepo) {
      customRepo.value = [savedRepo];
    }
  });

  onUnmounted(() => {
    window.removeEventListener('resize', handleResize);
    storeResizeObserver?.disconnect();
    stopStoreDownloadPolling();
    stopPluginExportPolling();
  });

  const drawerWidth = computed(() => {
    return isMobile.value ? '100%' : 800;
  });

  interface Plugin {
    name: string;
    status: 'enabled' | 'disabled';
    description?: string;
    source: 'panel' | 'store' | 'upload';
    has_smx: boolean;
    has_config: boolean;
  }

  interface StorePlugin {
    name: string;
    file_count: number;
    size: number;
    installed: boolean;
  }

  type StorePluginDownloadStatus = 'pending' | 'downloading' | 'completed' | 'failed' | 'cancelled';

  interface StorePluginDownloadProgress {
    name: string;
    repo: string;
    status: StorePluginDownloadStatus;
    downloaded: number;
    total: number;
    message: string;
  }

  const plugins = ref<Plugin[]>([]);
  const loading = ref(false);
  const uploading = ref(false);
  const fileList = ref<UploadProps['fileList']>([]);
  const activeTab = ref('enabled');
  const selectedRowKeys = ref<string[]>([]);
  const searchText = ref('');
  const filterText = ref('');
  const sourceFilter = ref<string[]>([]);

  const sourceOptions = [
    { label: '预设', value: 'panel' },
    { label: '商店', value: 'store' },
    { label: '上传', value: 'upload' },
  ];

  // Store variables
  const storeVisible = ref(false);
  const storeLoading = ref(false);
  const storePlugins = ref<StorePlugin[]>([]);
  const storeSearchText = ref('');
  const downloadProgress = ref<Record<string, StorePluginDownloadProgress>>({});
  const storeInstallFilter = ref<'all' | 'installed' | 'not-installed'>('all');
  let storeDownloadRefreshInterval: number | null = null;

  // GitHub Token variables
  const githubToken = ref('');
  const tokenModalVisible = ref(false);

  // Custom repo variables
  const repoOptions = [{ label: '官方仓库', value: 'LaoYutang/l4d2-plugins-store' }];
  const savedRepo = localStorage.getItem('l4d2_manager_plugin_repo');
  const customRepo = ref<string[]>(savedRepo ? [savedRepo] : []);

  watch(customRepo, (newVal) => {
    let valToSave = '';
    if (newVal && newVal.length > 0) {
      if (newVal.length > 1) {
        newVal.shift();
      }
      valToSave = newVal[0] || '';
    }
    if (valToSave) {
      localStorage.setItem('l4d2_manager_plugin_repo', valToSave);
    } else {
      localStorage.removeItem('l4d2_manager_plugin_repo');
    }
  });

  // Store drawer layout
  const searchSectionRef = ref<HTMLElement | null>(null);
  const storeContainerRef = ref<HTMLElement | null>(null);
  const tableScrollY = ref(400);
  let storeResizeObserver: ResizeObserver | null = null;

  const computeStoreTableScrollY = () => {
    if (!searchSectionRef.value || !storeContainerRef.value) return;
    const containerHeight = storeContainerRef.value.clientHeight;
    // searchSection offsetHeight + mb-4 (16px) + table thead (~39px) + pagination with margin (~48px)
    const overhead = searchSectionRef.value.offsetHeight + 16 + 39 + 48;
    tableScrollY.value = Math.max(150, containerHeight - overhead);
  };

  const getBody = () => document.body;

  const getModalContainer = () => document.body;

  const proxyOptions = [
    { label: 'laoyutang.cn(仅官方插件库可用)', value: 'https://gh-proxy.laoyutang.cn/' },
    { label: 'gh.dpik.top', value: 'https://gh.dpik.top/' },
    { label: 'gh-proxy.com', value: 'https://gh-proxy.com/' },
    { label: 'hk.gh-proxy.com', value: 'https://hk.gh-proxy.com/' },
    { label: 'gh.llkk.cc', value: 'https://gh.llkk.cc/' },
    { label: 'ghfast.top', value: 'https://ghfast.top/' },
  ];

  const savedProxy = localStorage.getItem('l4d2_manager_plugin_proxy');
  const selectedProxy = ref<string[]>(savedProxy ? [savedProxy] : []);

  watch(selectedProxy, (newVal) => {
    let valToSave = '';
    if (newVal && newVal.length > 0) {
      // Keep only the last selected item to act as single select
      if (newVal.length > 1) {
        newVal.shift();
      }

      valToSave = newVal[0] || '';
      // Auto prepend https:// if it's a custom input and doesn't have protocol
      if (valToSave && !valToSave.startsWith('http://') && !valToSave.startsWith('https://')) {
        valToSave = 'https://' + valToSave;
        // Update the UI immediately to reflect the fixed URL
        selectedProxy.value = [valToSave];
      }
    } else {
      // Explicitly clear proxy
      valToSave = '';
    }
    localStorage.setItem('l4d2_manager_plugin_proxy', valToSave);
  });

  const configModalVisible = ref(false);
  const currentConfigPlugin = ref('');
  const detailModalVisible = ref(false);
  const currentDetailPlugin = ref('');
  const currentDetailIsStore = ref(false);
  const pendingFiles = ref<File[]>([]);
  let uploadTimer: any = null;

  const exportProgressVisible = ref(false);
  const exportingPlugins = ref(false);
  const cancellingPluginExport = ref(false);
  const exportDownloaded = ref(false);
  const exportProgress = ref<PluginExportProgress | null>(null);
  let exportProgressInterval: number | null = null;

  const presetModalVisible = ref(false);
  const presets = ref<any[]>([]);
  const selectedPreset = ref('');
  const applyingPreset = ref(false);
  const footerContainerRef = ref<HTMLElement | null>(null);

  const getPopupContainer = (trigger: HTMLElement) => {
    return footerContainerRef.value || trigger || document.body;
  };

  const getRepoTooltipContainer = () => {
    return storeContainerRef.value || document.body;
  };

  const openPresetModal = async () => {
    try {
      const data = await api.getPresets();
      presets.value = data || [];
      selectedPreset.value = '';
      presetModalVisible.value = true;
    } catch (error: any) {
      message.error('获取预设列表失败: ' + error.message);
    }
  };

  const confirmApplyPreset = async () => {
    if (!selectedPreset.value) {
      message.warning('请选择一个预设');
      return;
    }

    applyingPreset.value = true;
    try {
      await api.applyPreset(selectedPreset.value);
      message.success('预设应用成功');
      presetModalVisible.value = false;
      fetchPlugins();
    } catch (error: any) {
      message.error('应用预设失败: ' + error.message);
    } finally {
      applyingPreset.value = false;
    }
  };

  onErrorCaptured((err) => {
    console.error('Plugins.vue Error:', err);
    message.error('插件管理页面发生错误');
    return false;
  });

  const sourceLabel = (source: string) => {
    switch (source) {
      case 'store':
        return '商店';
      case 'upload':
        return '上传';
      default:
        return '预设';
    }
  };

  const sourceColor = (source: string) => {
    switch (source) {
      case 'store':
        return 'green';
      case 'upload':
        return 'orange';
      default:
        return 'blue';
    }
  };

  const enabledPlugins = computed(() =>
    plugins.value.filter((p) => p.status === 'enabled').sort((a, b) => a.name.localeCompare(b.name))
  );
  const disabledPlugins = computed(() =>
    plugins.value
      .filter((p) => p.status === 'disabled')
      .sort((a, b) => a.name.localeCompare(b.name))
  );

  const filteredDisabledPlugins = computed(() => {
    let list = disabledPlugins.value;
    if (sourceFilter.value.length > 0) {
      list = list.filter((p) => sourceFilter.value.includes(p.source));
    }
    if (!filterText.value) return list;
    const lower = filterText.value.toLowerCase();
    return list.filter(
      (p) =>
        p.name.toLowerCase().includes(lower) ||
        (p.description && p.description.toLowerCase().includes(lower))
    );
  });

  const filteredEnabledPlugins = computed(() => {
    let list = enabledPlugins.value;
    if (sourceFilter.value.length > 0) {
      list = list.filter((p) => sourceFilter.value.includes(p.source));
    }
    if (!filterText.value) return list;
    const lower = filterText.value.toLowerCase();
    return list.filter(
      (p) =>
        p.name.toLowerCase().includes(lower) ||
        (p.description && p.description.toLowerCase().includes(lower))
    );
  });

  const fetchPlugins = async () => {
    loading.value = true;
    try {
      plugins.value = await api.getPlugins();
    } catch (error: any) {
      message.error('加载插件失败: ' + error.message);
    } finally {
      loading.value = false;
    }
  };

  const processPendingUploads = async () => {
    const filesToUpload = [...pendingFiles.value];
    pendingFiles.value = [];

    if (filesToUpload.length === 0) return;

    const hide = message.loading(`正在上传 ${filesToUpload.length} 个插件...`, 0);
    uploading.value = true;
    try {
      await api.uploadPlugin(filesToUpload);
      message.success('插件上传成功');
      fileList.value = [];
      fetchPlugins();
    } catch (error: any) {
      message.error('上传失败: ' + error.message);
    } finally {
      uploading.value = false;
      hide();
    }
  };

  const handleUpload = (file: File) => {
    if (!file.name.endsWith('.zip')) {
      message.error('只允许上传 .zip 格式的文件');
      return false;
    }

    pendingFiles.value.push(file);

    if (uploadTimer) clearTimeout(uploadTimer);
    uploadTimer = setTimeout(() => {
      processPendingUploads();
    }, 100);

    return false; // Prevent default upload behavior
  };

  const isPluginExportActive = computed(() => {
    const status = exportProgress.value?.status;
    return status === 'pending' || status === 'compressing';
  });

  const exportPercent = computed(() => {
    const progress = exportProgress.value;
    if (!progress) return 0;
    if (progress.status === 'completed') return 100;
    if (progress.total <= 0) return 0;
    return Math.min(99, Math.floor((progress.processed / progress.total) * 100));
  });

  const exportProgressStatus = computed(() => {
    const status = exportProgress.value?.status;
    if (status === 'completed') return 'success';
    if (status === 'failed') return 'exception';
    if (status === 'cancelled') return 'normal';
    return 'active';
  });

  const exportProgressText = computed(() => {
    const progress = exportProgress.value;
    if (!progress) return '准备导出插件...';
    if (progress.total <= 0) return progress.message || '正在扫描插件文件...';
    return `${progress.message || '正在压缩插件文件'} (${progress.processed}/${progress.total})`;
  });

  const stopPluginExportPolling = () => {
    if (exportProgressInterval) {
      clearInterval(exportProgressInterval);
      exportProgressInterval = null;
    }
  };

  const downloadCompletedPluginExport = async (taskId: string) => {
    if (exportDownloaded.value) return;
    exportDownloaded.value = true;
    stopPluginExportPolling();

    try {
      await api.downloadExportedPlugins(taskId);
      message.success('插件导出完成，已开始下载');
    } catch (error: any) {
      message.error('下载导出文件失败: ' + error.message);
      exportProgress.value = {
        task_id: taskId,
        status: 'failed',
        processed: exportProgress.value?.processed || 0,
        total: exportProgress.value?.total || 0,
        message: error.message || '下载导出文件失败',
      };
    } finally {
      exportingPlugins.value = false;
    }
  };

  let loadingPluginExportStatus = false;
  const loadPluginExportStatus = async () => {
    if (loadingPluginExportStatus) return;
    const taskId = exportProgress.value?.task_id;
    if (!taskId) return;

    loadingPluginExportStatus = true;
    try {
      const progress = await api.getExportAllPluginsStatus(taskId);
      exportProgress.value = progress;

      if (progress.status === 'completed') {
        await downloadCompletedPluginExport(progress.task_id);
      } else if (progress.status === 'failed') {
        stopPluginExportPolling();
        exportingPlugins.value = false;
        message.error('导出插件失败: ' + (progress.message || '未知错误'));
      } else if (progress.status === 'cancelled') {
        stopPluginExportPolling();
        exportingPlugins.value = false;
        message.info('插件导出已取消');
      }
    } catch (error: any) {
      stopPluginExportPolling();
      exportingPlugins.value = false;
      message.error('获取导出进度失败: ' + error.message);
    } finally {
      loadingPluginExportStatus = false;
    }
  };

  const startPluginExportPolling = () => {
    if (exportProgressInterval) return;
    exportProgressInterval = window.setInterval(loadPluginExportStatus, 1000);
  };

  const handleExportAllPlugins = async () => {
    if (exportingPlugins.value) return;

    exportingPlugins.value = true;
    exportDownloaded.value = false;
    exportProgressVisible.value = true;
    exportProgress.value = {
      task_id: '',
      status: 'pending',
      processed: 0,
      total: 0,
      message: '正在扫描插件文件...',
    };

    try {
      const progress = await api.startExportAllPlugins();
      exportProgress.value = progress;

      if (progress.status === 'completed') {
        await downloadCompletedPluginExport(progress.task_id);
      } else if (progress.status === 'failed') {
        exportingPlugins.value = false;
        message.error('导出插件失败: ' + (progress.message || '未知错误'));
      } else {
        startPluginExportPolling();
      }
    } catch (error: any) {
      exportingPlugins.value = false;
      exportProgress.value = {
        task_id: '',
        status: 'failed',
        processed: 0,
        total: 0,
        message: error.message || '启动导出失败',
      };
      message.error('启动导出失败: ' + error.message);
    }
  };

  const cancelPluginExport = async (closeAfterCancel: boolean = false) => {
    const taskId = exportProgress.value?.task_id;
    if (!taskId || !isPluginExportActive.value) {
      exportProgressVisible.value = false;
      return;
    }

    cancellingPluginExport.value = true;
    stopPluginExportPolling();
    try {
      const progress = await api.cancelExportAllPlugins(taskId);
      exportProgress.value = progress;
      message.info('插件导出已取消');
    } catch (error: any) {
      message.error('取消导出失败: ' + error.message);
    } finally {
      exportingPlugins.value = false;
      cancellingPluginExport.value = false;
      if (closeAfterCancel) {
        exportProgressVisible.value = false;
      }
    }
  };

  const handleExportModalCancel = () => {
    if (isPluginExportActive.value) {
      cancelPluginExport(true);
    } else {
      exportProgressVisible.value = false;
    }
  };

  const togglePlugin = async (plugin: Plugin) => {
    const actionText = plugin.status === 'enabled' ? '禁用' : '启用';
    const hide = message.loading(`正在${actionText}插件...`, 0);

    try {
      if (plugin.status === 'enabled') {
        await api.disablePlugin(plugin.name);
      } else {
        await api.enablePlugin(plugin.name);
      }
      message.success(`插件${actionText}成功`);
      fetchPlugins();
    } catch (error: any) {
      message.error(`${actionText}插件失败: ` + error.message);
    } finally {
      hide();
    }
  };

  const hotTogglePlugin = (plugin: Plugin) => {
    const actionText = plugin.status === 'enabled' ? '禁用并立即卸载 smx' : '启用并立即加载 smx';

    AModal.confirm({
      title: `确定要${actionText}吗？`,
      okText: '确定',
      cancelText: '取消',
      onOk: async () => {
        const hide = message.loading(`正在${actionText}...`, 0);
        try {
          if (plugin.status === 'enabled') {
            await api.disableAndUnloadPlugin(plugin.name);
          } else {
            await api.enableAndLoadPlugin(plugin.name);
          }
          message.success(`插件${actionText}成功`);
          fetchPlugins();
        } catch (error: any) {
          message.error(`${actionText}失败: ` + error.message);
          fetchPlugins();
        } finally {
          hide();
        }
      },
    });
  };

  const deletePlugin = async (plugin: Plugin) => {
    if (plugin.status === 'enabled') {
      message.warning('请先禁用插件');
      return;
    }

    const hide = message.loading('正在删除插件...', 0);
    try {
      await api.deletePlugin(plugin.name);
      message.success('插件删除成功');
      fetchPlugins();
    } catch (error: any) {
      message.error('删除插件失败: ' + error.message);
    } finally {
      hide();
    }
  };

  const handleSearch = () => {
    filterText.value = searchText.value;
  };

  const handleReset = () => {
    searchText.value = '';
    filterText.value = '';
    sourceFilter.value = [];
  };

  const openStore = async () => {
    storeVisible.value = true;

    // Refresh proxy value from localStorage when opening just in case it was empty
    // But don't overwrite if user already has something
    const saved = localStorage.getItem('l4d2_manager_plugin_proxy');
    if (selectedProxy.value.length === 0 && saved) {
      selectedProxy.value = [saved];
    } else if (selectedProxy.value.length === 0 && (saved === '' || saved === null)) {
      // Default behavior if nothing saved or explicitly empty (direct)
      selectedProxy.value = [];
    }

    if (storePlugins.value.length === 0) {
      fetchStorePlugins(false);
    }
  };

  const openTokenModal = () => {
    tokenModalVisible.value = true;
  };

  const saveToken = () => {
    localStorage.setItem('l4d2_manager_github_token', githubToken.value);
    tokenModalVisible.value = false;
    message.success('Token 已保存');
  };

  const clearToken = () => {
    githubToken.value = '';
    localStorage.removeItem('l4d2_manager_github_token');
    tokenModalVisible.value = false;
    message.success('Token 已清除');
  };

  const fetchStorePlugins = async (forceRefresh: boolean = true) => {
    storeLoading.value = true;
    try {
      const proxy = selectedProxy.value.length > 0 ? selectedProxy.value[0] || '' : '';
      const repo = customRepo.value.length > 0 ? customRepo.value[0] || '' : '';
      storePlugins.value = await api.getStorePlugins(forceRefresh, proxy, githubToken.value, repo);
      loadStoreDownloadStatus();
    } catch (error: any) {
      message.error('获取商店列表失败: ' + error.message);
    } finally {
      storeLoading.value = false;
    }
  };

  const defaultStoreRepo = 'LaoYutang/l4d2-plugins-store';

  const getCurrentStoreRepo = () => {
    return customRepo.value.length > 0 ? customRepo.value[0] || '' : '';
  };

  const normalizeStoreRepo = (repo: string = '') => {
    return repo || defaultStoreRepo;
  };

  const getDownloadProgressKey = (name: string, repo: string = getCurrentStoreRepo()) => {
    return `${normalizeStoreRepo(repo)}\n${name}`;
  };

  const isActiveDownloadStatus = (status?: StorePluginDownloadStatus) => {
    return status === 'pending' || status === 'downloading';
  };

  const getPluginDownloadProgress = (plugin: StorePlugin) => {
    return downloadProgress.value[getDownloadProgressKey(plugin.name)];
  };

  const isStoreDownloadActive = (plugin: StorePlugin) => {
    return isActiveDownloadStatus(getPluginDownloadProgress(plugin)?.status);
  };

  const getStoreDownloadLabel = (plugin: StorePlugin) => {
    const progress = getPluginDownloadProgress(plugin);
    if (!progress) return '下载';
    const total = progress.total || plugin.file_count || 0;
    return `${progress.downloaded}/${total}`;
  };

  const hasActiveStoreDownloads = () => {
    return Object.values(downloadProgress.value).some((progress) =>
      isActiveDownloadStatus(progress.status)
    );
  };

  const stopStoreDownloadPolling = () => {
    if (storeDownloadRefreshInterval) {
      clearInterval(storeDownloadRefreshInterval);
      storeDownloadRefreshInterval = null;
    }
  };

  let loadingStoreDownloadStatus = false;
  const loadStoreDownloadStatus = async () => {
    if (loadingStoreDownloadStatus) return;
    loadingStoreDownloadStatus = true;

    const repo = getCurrentStoreRepo();
    const repoKey = normalizeStoreRepo(repo);

    try {
      const tasks = (await api.getStorePluginDownloadStatus(repo)) as StorePluginDownloadProgress[];
      const nextProgress = { ...downloadProgress.value };

      for (const key of Object.keys(nextProgress)) {
        if (key.startsWith(`${repoKey}\n`)) {
          delete nextProgress[key];
        }
      }

      let hasActiveInResponse = false;
      for (const task of tasks) {
        const key = getDownloadProgressKey(task.name, task.repo);
        const previous = downloadProgress.value[key];

        if (isActiveDownloadStatus(task.status)) {
          nextProgress[key] = task;
          hasActiveInResponse = true;
          continue;
        }

        if (previous && isActiveDownloadStatus(previous.status)) {
          if (task.status === 'completed') {
            const idx = storePlugins.value.findIndex((p) => p.name === task.name);
            if (idx !== -1) storePlugins.value[idx]!.installed = true;
            message.success(`插件 ${task.name} 下载成功`);
            fetchPlugins();
          } else if (task.status === 'failed') {
            message.error(`插件 ${task.name} 下载失败: ${task.message || '未知错误'}`);
          } else if (task.status === 'cancelled') {
            message.info(`插件 ${task.name} 下载已取消`);
          }
        }
      }

      downloadProgress.value = nextProgress;

      if (!hasActiveInResponse && !hasActiveStoreDownloads()) {
        stopStoreDownloadPolling();
      }
    } catch (error: any) {
      if (storeVisible.value) {
        message.error('获取下载进度失败: ' + error.message);
      }
    } finally {
      loadingStoreDownloadStatus = false;
    }
  };

  const startStoreDownloadPolling = () => {
    if (storeDownloadRefreshInterval) return;
    loadStoreDownloadStatus();
    storeDownloadRefreshInterval = window.setInterval(loadStoreDownloadStatus, 1000);
  };

  const filteredStorePlugins = computed(() => {
    let list = storePlugins.value;
    if (storeInstallFilter.value === 'installed') {
      list = list.filter((p) => p.installed);
    } else if (storeInstallFilter.value === 'not-installed') {
      list = list.filter((p) => !p.installed);
    }
    if (storeSearchText.value) {
      const lower = storeSearchText.value.toLowerCase();
      list = list.filter((p) => p.name.toLowerCase().includes(lower));
    }
    return [...list].sort((a, b) => a.name.localeCompare(b.name));
  });

  const downloadFromStore = async (plugin: StorePlugin) => {
    if (isStoreDownloadActive(plugin)) return;

    const repo = getCurrentStoreRepo();
    const repoKey = normalizeStoreRepo(repo);
    const key = getDownloadProgressKey(plugin.name, repo);
    downloadProgress.value = {
      ...downloadProgress.value,
      [key]: {
        name: plugin.name,
        repo: repoKey,
        status: 'pending',
        downloaded: 0,
        total: plugin.file_count,
        message: '等待下载',
      },
    };

    try {
      const proxy = selectedProxy.value.length > 0 ? selectedProxy.value[0] || '' : '';
      const progress = (await api.downloadStorePlugin(
        plugin.name,
        proxy,
        githubToken.value,
        repo
      )) as StorePluginDownloadProgress;
      downloadProgress.value = {
        ...downloadProgress.value,
        [getDownloadProgressKey(progress.name, progress.repo)]: progress,
      };
      startStoreDownloadPolling();
    } catch (error: any) {
      const nextProgress = { ...downloadProgress.value };
      delete nextProgress[key];
      downloadProgress.value = nextProgress;
      message.error(`下载失败: ` + error.message);
      if (!hasActiveStoreDownloads()) {
        stopStoreDownloadPolling();
      }
    }
  };

  const cancelStoreDownload = async (plugin: StorePlugin) => {
    const progress = getPluginDownloadProgress(plugin);
    const repo = progress?.repo || getCurrentStoreRepo();
    const key = getDownloadProgressKey(plugin.name, repo);

    try {
      await api.cancelStorePluginDownload(plugin.name, repo);
      const nextProgress = { ...downloadProgress.value };
      delete nextProgress[key];
      downloadProgress.value = nextProgress;
      message.success(`已取消 ${plugin.name} 下载`);
    } catch (error: any) {
      message.error('取消下载失败: ' + error.message);
    } finally {
      if (!hasActiveStoreDownloads()) {
        stopStoreDownloadPolling();
      }
    }
  };

  watch(activeTab, () => {
    selectedRowKeys.value = [];
  });

  watch(storeVisible, (val) => {
    if (val) {
      startStoreDownloadPolling();
      nextTick(() => {
        computeStoreTableScrollY();
        if (storeContainerRef.value && !storeResizeObserver) {
          storeResizeObserver = new ResizeObserver(computeStoreTableScrollY);
          storeResizeObserver.observe(storeContainerRef.value);
        }
      });
    } else {
      storeResizeObserver?.disconnect();
      storeResizeObserver = null;
      if (!hasActiveStoreDownloads()) {
        stopStoreDownloadPolling();
      }
    }
  });

  const handleBatchEnable = async () => {
    if (selectedRowKeys.value.length === 0) return;

    const hide = message.loading('正在批量启用插件...', 0);
    try {
      await api.enablePlugins(selectedRowKeys.value);
      message.success(`成功启用 ${selectedRowKeys.value.length} 个插件`);
      selectedRowKeys.value = [];
      fetchPlugins();
    } catch (error: any) {
      message.error('批量启用失败: ' + error.message);
      fetchPlugins();
    } finally {
      hide();
    }
  };

  const handleBatchDisable = async () => {
    if (selectedRowKeys.value.length === 0) return;

    const hide = message.loading('正在批量禁用插件...', 0);
    try {
      await api.disablePlugins(selectedRowKeys.value);
      message.success(`成功禁用 ${selectedRowKeys.value.length} 个插件`);
      selectedRowKeys.value = [];
      fetchPlugins();
    } catch (error: any) {
      message.error('批量禁用失败: ' + error.message);
      fetchPlugins();
    } finally {
      hide();
    }
  };

  const rowSelection = computed(() => {
    return {
      selectedRowKeys: selectedRowKeys.value,
      onChange: onSelectChange,
    };
  });

  const handleBatchDelete = async () => {
    if (selectedRowKeys.value.length === 0) return;

    const hide = message.loading('正在批量删除插件...', 0);
    try {
      // Execute deletions sequentially to avoid potential conflicts or backend overload
      // Or concurrent if backend supports it. Here sequential for safety.
      for (const name of selectedRowKeys.value) {
        await api.deletePlugin(name);
      }
      message.success(`成功删除 ${selectedRowKeys.value.length} 个插件`);
      selectedRowKeys.value = [];
      fetchPlugins();
    } catch (error: any) {
      message.error('批量删除部分失败: ' + error.message);
      // Refresh to see what was actually deleted
      fetchPlugins();
    } finally {
      hide();
    }
  };

  const onSelectChange = (keys: any[]) => {
    selectedRowKeys.value = keys;
  };

  const openConfig = async (plugin: Plugin) => {
    if (!plugin.has_config) {
      message.info('该插件暂无可配置项');
      return;
    }
    currentConfigPlugin.value = plugin.name;
    configModalVisible.value = true;
  };

  const openDetail = (pluginName: string, isStore: boolean = false) => {
    currentDetailPlugin.value = pluginName;
    currentDetailIsStore.value = isStore;
    detailModalVisible.value = true;
  };

  const localPluginRow = (record: Plugin) => ({
    onDblclick: () => openDetail(record.name),
  });

  const storePluginRow = (record: StorePlugin) => ({
    onDblclick: () => openDetail(record.name, true),
  });

  const enabledColumns = computed(() => {
    const cols = [
      {
        title: '插件名称',
        dataIndex: 'name',
        key: 'name',
        sorter: (a: Plugin, b: Plugin) => a.name.localeCompare(b.name),
      },
      {
        title: '来源',
        key: 'source',
        width: 80,
      },
      {
        title: '操作',
        key: 'actions',
        width: 260,
      },
    ];
    return isMobile.value ? cols.filter((c) => c.key !== 'source') : cols;
  });

  const disabledColumns = computed(() => {
    const cols = [
      {
        title: '插件名称',
        dataIndex: 'name',
        key: 'name',
        sorter: (a: Plugin, b: Plugin) => a.name.localeCompare(b.name),
      },
      {
        title: '来源',
        key: 'source',
        width: 80,
      },
      {
        title: '操作',
        key: 'actions',
        width: 260,
      },
    ];
    return isMobile.value ? cols.filter((c) => c.key !== 'source') : cols;
  });

  const storeColumns = computed(() => {
    const cols = [
      {
        title: '插件名称',
        dataIndex: 'name',
        key: 'name',
        ...(isMobile.value ? {} : { ellipsis: true }),
      },
      {
        title: '大小',
        dataIndex: 'size',
        key: 'size',
        width: 100,
      },
      {
        title: '操作',
        key: 'actions',
        width: 200,
      },
    ];
    return isMobile.value ? cols.filter((c) => c.key !== 'size') : cols;
  });

  const formatSize = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const enabledPagination = reactive<TablePaginationConfig>({
    current: 1,
    pageSize: 10,
    showSizeChanger: true,
    pageSizeOptions: ['10', '20', '50', '100'],
    showTotal: (total: number) => `共 ${total} 条`,
  });

  const disabledPagination = reactive<TablePaginationConfig>({
    current: 1,
    pageSize: 10,
    showSizeChanger: true,
    pageSizeOptions: ['10', '20', '50', '100'],
    showTotal: (total: number) => `共 ${total} 条`,
  });

  const handleEnabledTableChange = (pag: TablePaginationConfig) => {
    enabledPagination.current = pag.current;
    enabledPagination.pageSize = pag.pageSize;
  };

  const handleDisabledTableChange = (pag: TablePaginationConfig) => {
    disabledPagination.current = pag.current;
    disabledPagination.pageSize = pag.pageSize;
  };

  const storePagination = reactive<TablePaginationConfig>({
    current: 1,
    pageSize: 20,
    showSizeChanger: true,
    pageSizeOptions: ['10', '20', '50', '100'],
    showTotal: (total: number) => `共 ${total} 个插件`,
  });

  const handleStoreTableChange = (pag: TablePaginationConfig) => {
    storePagination.current = pag.current;
    storePagination.pageSize = pag.pageSize;
  };
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
      <div>
        <h1 class="text-2xl font-bold text-gray-800 dark:text-gray-100">插件管理</h1>
        <p class="text-gray-500 dark:text-gray-400 mt-1">管理服务器插件和模组</p>
      </div>
      <div class="flex flex-wrap justify-end gap-2">
        <a-button
          v-if="authStore.isAdmin"
          type="primary"
          ghost
          @click="openPresetModal"
          class="!flex !items-center !justify-center"
        >
          <template #icon><SettingOutlined /></template>
          应用预设
        </a-button>
        <a-popconfirm
          v-if="authStore.isAdmin"
          overlayClassName="plugin-export-popconfirm"
          placement="bottomRight"
          ok-text="确定"
          cancel-text="取消"
          @confirm="handleExportAllPlugins"
        >
          <template #title>
            <div class="max-w-[320px] leading-relaxed whitespace-normal">
              导出会在压缩时占用服务器 CPU，下载时占用服务器带宽，请勿在游戏时进行此操作。确定继续吗？
            </div>
          </template>
          <a-button
            :loading="exportingPlugins"
            :disabled="exportingPlugins"
            class="!flex !items-center !justify-center"
          >
            <template #icon><DownloadOutlined /></template>
            导出所有插件
          </a-button>
        </a-popconfirm>
        <a-button
          type="default"
          @click="fetchPlugins"
          :loading="loading"
          class="!flex !items-center !justify-center"
        >
          <template #icon><SyncOutlined /></template>
          刷新列表
        </a-button>
      </div>
    </div>

    <a-card :bordered="false" class="shadow-sm dark:bg-gray-800">
      <a-tabs v-model:activeKey="activeTab">
        <!-- Enabled Plugins Tab -->
        <a-tab-pane key="enabled" tab="已启用插件">
          <div
            class="mb-4 flex flex-col lg:flex-row justify-between items-start lg:items-center gap-4"
          >
            <div class="flex flex-col sm:flex-row gap-2 w-full lg:w-auto">
              <a-input
                v-model:value="searchText"
                placeholder="搜索插件..."
                class="w-full sm:w-[200px]"
                allow-clear
                @pressEnter="handleSearch"
              />
              <a-select
                v-model:value="sourceFilter"
                mode="multiple"
                :options="sourceOptions"
                placeholder="来源筛选"
                allow-clear
                class="w-full sm:w-[200px]"
                :max-tag-count="'responsive'"
              />
              <div class="flex gap-2 w-full sm:w-auto">
                <a-button
                  type="primary"
                  @click="handleSearch"
                  class="!flex !items-center !justify-center flex-1 sm:flex-none"
                >
                  <template #icon><SearchOutlined /></template>
                  搜索
                </a-button>
                <a-button
                  @click="handleReset"
                  class="!flex !items-center !justify-center flex-1 sm:flex-none"
                >
                  <template #icon><ReloadOutlined /></template>
                  重置
                </a-button>
              </div>
            </div>

            <div v-if="selectedRowKeys.length > 0 && authStore.isAdmin" class="flex gap-2">
              <div>
                <a-popconfirm
                  title="确定要禁用选中的插件吗？"
                  ok-text="确定"
                  cancel-text="取消"
                  @confirm="handleBatchDisable"
                >
                  <a-button danger>批量禁用 ({{ selectedRowKeys.length }})</a-button>
                </a-popconfirm>
              </div>
            </div>
          </div>

          <a-table
            :columns="enabledColumns"
            :data-source="filteredEnabledPlugins"
            :loading="loading"
            :pagination="enabledPagination"
            @change="handleEnabledTableChange"
            row-key="name"
            :scroll="{ x: 'max-content' }"
            :row-selection="rowSelection"
            :customRow="localPluginRow"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <div class="font-medium text-gray-700 dark:text-gray-200">
                  <a-tag v-if="isMobile" :color="sourceColor(record.source)" class="!mr-1">{{
                    sourceLabel(record.source)
                  }}</a-tag
                  >{{ record.name }}
                </div>
                <div v-if="record.description" class="text-xs text-gray-400 dark:text-gray-500">
                  {{ record.description }}
                </div>
              </template>

              <template v-else-if="column.key === 'source'">
                <a-tag :color="sourceColor(record.source)" class="!cursor-default">
                  {{ sourceLabel(record.source) }}
                </a-tag>
              </template>

              <template v-else-if="column.key === 'actions'">
                <div class="flex gap-2">
                  <div v-if="authStore.isAdmin" class="inline-flex w-[104px]">
                    <a-popconfirm
                      title="确定要禁用这个插件吗？"
                      ok-text="确定"
                      cancel-text="取消"
                      @confirm="togglePlugin(record as Plugin)"
                    >
                      <a-button
                        type="default"
                        danger
                        size="small"
                        class="!flex !items-center !justify-center"
                        :class="
                          (record as Plugin).has_smx
                            ? '!w-[72px] !rounded-r-none'
                            : '!w-[104px]'
                        "
                      >
                        <template #icon><PoweroffOutlined /></template>
                        禁用
                      </a-button>
                    </a-popconfirm>
                    <a-dropdown
                      v-if="(record as Plugin).has_smx"
                      :trigger="['click']"
                      placement="bottomRight"
                      :getPopupContainer="getBody"
                      :overlayStyle="{ width: '172px' }"
                    >
                      <a-button
                        type="default"
                        danger
                        size="small"
                        class="!flex !w-[32px] !items-center !justify-center !rounded-l-none !border-l-0 !px-0"
                      >
                        <template #icon><DownOutlined /></template>
                      </a-button>
                      <template #overlay>
                        <a-menu>
                          <a-menu-item key="unload" @click="hotTogglePlugin(record as Plugin)">
                            禁用并立即卸载 smx
                          </a-menu-item>
                        </a-menu>
                      </template>
                    </a-dropdown>
                  </div>
                  <a-tooltip
                    :title="(record as Plugin).has_config ? '' : '暂无可配置项'"
                    :getPopupContainer="getBody"
                  >
                    <span class="inline-flex">
                      <a-button
                        type="default"
                        size="small"
                        class="!flex !items-center !justify-center"
                        :disabled="!(record as Plugin).has_config"
                        @click="openConfig(record as Plugin)"
                      >
                        <template #icon><SettingOutlined /></template>
                        配置
                      </a-button>
                    </span>
                  </a-tooltip>
                  <a-button
                    type="default"
                    size="small"
                    class="!flex !items-center !justify-center"
                    @click="openDetail((record as Plugin).name)"
                  >
                    <template #icon><FileTextOutlined /></template>
                    详情
                  </a-button>
                </div>
              </template>
            </template>
          </a-table>
        </a-tab-pane>

        <!-- Disabled Plugins Tab -->
        <a-tab-pane key="disabled" tab="未启用插件" v-if="authStore.isAdmin">
          <div
            class="mb-4 flex flex-col lg:flex-row justify-between items-start lg:items-center gap-4"
          >
            <div class="flex flex-col sm:flex-row gap-2 w-full lg:w-auto">
              <a-input
                v-model:value="searchText"
                placeholder="搜索插件..."
                class="w-full sm:w-[200px]"
                allow-clear
                @pressEnter="handleSearch"
              />
              <a-select
                v-model:value="sourceFilter"
                mode="multiple"
                :options="sourceOptions"
                placeholder="来源筛选"
                allow-clear
                class="w-full sm:w-[200px]"
                :max-tag-count="'responsive'"
              />
              <div class="flex gap-2 w-full sm:w-auto">
                <a-button
                  type="primary"
                  @click="handleSearch"
                  class="!flex !items-center !justify-center flex-1 sm:flex-none"
                >
                  <template #icon><SearchOutlined /></template>
                  搜索
                </a-button>
                <a-button
                  @click="handleReset"
                  class="!flex !items-center !justify-center flex-1 sm:flex-none"
                >
                  <template #icon><ReloadOutlined /></template>
                  重置
                </a-button>
              </div>
            </div>

            <div class="flex flex-row sm:flex-row gap-2 w-full lg:w-auto lg:items-center">
              <div
                v-if="selectedRowKeys.length > 0 && authStore.isAdmin"
                class="flex gap-2 w-full sm:w-auto"
              >
                <div>
                  <a-popconfirm
                    title="确定要启用选中的插件吗？"
                    ok-text="确定"
                    cancel-text="取消"
                    @confirm="handleBatchEnable"
                  >
                    <a-button type="primary">批量启用 ({{ selectedRowKeys.length }})</a-button>
                  </a-popconfirm>
                </div>
                <div>
                  <a-popconfirm
                    title="确定要删除选中的插件吗？"
                    ok-text="确定"
                    cancel-text="取消"
                    @confirm="handleBatchDelete"
                  >
                    <a-button danger>批量删除 ({{ selectedRowKeys.length }})</a-button>
                  </a-popconfirm>
                </div>
              </div>

              <div class="flex flex-col sm:flex-row gap-2 w-full lg:w-auto">
                <a-upload
                  v-if="authStore.isAdmin"
                  v-model:file-list="fileList"
                  :before-upload="handleUpload"
                  accept=".zip"
                  :show-upload-list="false"
                  :disabled="uploading"
                  multiple
                  class="flex-1"
                >
                  <a-button :loading="uploading" class="!flex !items-center !justify-center w-full">
                    <template #icon><UploadOutlined /></template>
                    上传插件 (.zip)
                  </a-button>
                </a-upload>

                <a-button
                  v-if="authStore.isAdmin"
                  type="primary"
                  @click="openStore"
                  class="flex-1 !flex !items-center !justify-center bg-green-600 hover:bg-green-500 border-green-600 hover:border-green-500"
                >
                  <template #icon><AppstoreAddOutlined /></template>
                  插件商店
                </a-button>
              </div>
            </div>
          </div>

          <a-table
            :columns="disabledColumns"
            :data-source="filteredDisabledPlugins"
            :loading="loading"
            :pagination="disabledPagination"
            @change="handleDisabledTableChange"
            row-key="name"
            :scroll="{ x: 'max-content' }"
            :row-selection="rowSelection"
            :customRow="localPluginRow"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <div class="font-medium text-gray-700 dark:text-gray-200">
                  <a-tag v-if="isMobile" :color="sourceColor(record.source)" class="!mr-1">{{
                    sourceLabel(record.source)
                  }}</a-tag
                  >{{ record.name }}
                </div>
                <div v-if="record.description" class="text-xs text-gray-400 dark:text-gray-500">
                  {{ record.description }}
                </div>
              </template>

              <template v-else-if="column.key === 'source'">
                <a-tag :color="sourceColor(record.source)" class="!cursor-default">
                  {{ sourceLabel(record.source) }}
                </a-tag>
              </template>

              <template v-else-if="column.key === 'actions'">
                <div class="flex items-center gap-2">
                  <div class="inline-flex w-[104px]">
                    <a-popconfirm
                      title="确定要启用这个插件吗？"
                      ok-text="确定"
                      cancel-text="取消"
                      @confirm="togglePlugin(record as Plugin)"
                      :disabled="!authStore.isAdmin"
                    >
                      <a-button
                        type="primary"
                        size="small"
                        class="!flex !items-center !justify-center"
                        :class="
                          (record as Plugin).has_smx
                            ? '!w-[72px] !rounded-r-none'
                            : '!w-[104px]'
                        "
                        :disabled="!authStore.isAdmin"
                      >
                        <template #icon><PoweroffOutlined /></template>
                        启用
                      </a-button>
                    </a-popconfirm>
                    <a-dropdown
                      v-if="(record as Plugin).has_smx"
                      :trigger="['click']"
                      placement="bottomRight"
                      :getPopupContainer="getBody"
                      :overlayStyle="{ width: '172px' }"
                    >
                      <a-button
                        type="primary"
                        size="small"
                        class="!flex !w-[32px] !items-center !justify-center !rounded-l-none !border-l !px-0"
                        style="border-left-color: rgba(255, 255, 255, 0.45)"
                        :disabled="!authStore.isAdmin"
                      >
                        <template #icon><DownOutlined /></template>
                      </a-button>
                      <template #overlay>
                        <a-menu>
                          <a-menu-item key="load" @click="hotTogglePlugin(record as Plugin)">
                            启用并立即加载 smx
                          </a-menu-item>
                        </a-menu>
                      </template>
                    </a-dropdown>
                  </div>

                  <a-button
                    type="default"
                    size="small"
                    class="!flex !items-center !justify-center"
                    @click="openDetail((record as Plugin).name)"
                  >
                    <template #icon><FileTextOutlined /></template>
                    详情
                  </a-button>

                  <a-popconfirm
                    v-if="authStore.isAdmin"
                    title="确定要删除这个插件吗？"
                    ok-text="确定"
                    cancel-text="取消"
                    @confirm="deletePlugin(record as Plugin)"
                  >
                    <a-button
                      type="text"
                      danger
                      size="small"
                      class="!flex !items-center !justify-center"
                    >
                      <template #icon><DeleteOutlined /></template>
                      删除
                    </a-button>
                  </a-popconfirm>
                </div>
              </template>
            </template>
          </a-table>
        </a-tab-pane>
      </a-tabs>
    </a-card>

    <a-modal
      v-model:open="presetModalVisible"
      title="选择插件预设"
      :confirm-loading="applyingPreset"
      ok-text="应用"
      cancel-text="取消"
      :width="600"
    >
      <template #footer>
        <div class="flex justify-end gap-2" ref="footerContainerRef">
          <a-button @click="presetModalVisible = false">取消</a-button>
          <a-popconfirm
            title="此操作将重置所有插件状态，确定要继续吗？"
            ok-text="确定"
            cancel-text="取消"
            @confirm="confirmApplyPreset"
            :getPopupContainer="getPopupContainer"
          >
            <a-button type="primary" :loading="applyingPreset">应用</a-button>
          </a-popconfirm>
        </div>
      </template>
      <a-alert
        message="注意"
        description="应用预设将重置所有插件状态，禁用当前所有插件并按预设启用。配置项也会被覆盖。"
        type="warning"
        show-icon
        class="mb-6"
      />
      <div v-if="presets.length === 0" class="text-center py-4 text-gray-500">暂无可用预设</div>
      <div class="flex flex-col gap-2 max-h-[60vh] overflow-y-auto mt-6">
        <a-radio-group v-model:value="selectedPreset" button-style="solid" class="w-full">
          <div class="flex flex-col gap-3">
            <a-radio-button
              v-for="preset in presets"
              :key="preset.name"
              :value="preset.name"
              class="!h-auto !py-3 !px-4 !flex !items-center !rounded-md !border !border-gray-200 dark:!border-gray-700 hover:!border-blue-500 transition-all"
            >
              <div class="flex flex-col text-left w-full">
                <span class="font-medium text-base">{{ preset.name }}</span>
                <span
                  v-if="preset.desc"
                  class="text-sm mt-1 mb-0.5 whitespace-normal opacity-90"
                  :class="
                    selectedPreset === preset.name
                      ? 'text-white'
                      : 'text-gray-500 dark:text-gray-400'
                  "
                >
                  {{ preset.desc }}
                </span>
                <span
                  class="text-xs mt-0.5 opacity-75"
                  :class="
                    selectedPreset === preset.name
                      ? 'text-white'
                      : 'text-gray-400 dark:text-gray-500'
                  "
                >
                  包含 {{ preset.plugin_count || 0 }} 个插件
                </span>
              </div>
            </a-radio-button>
          </div>
        </a-radio-group>
      </div>
    </a-modal>

    <a-modal
      v-model:open="exportProgressVisible"
      title="导出所有插件"
      :width="460"
      :getContainer="getModalContainer"
      @cancel="handleExportModalCancel"
    >
      <div class="space-y-4">
        <a-alert
          message="导出会在压缩时占用服务器 CPU，下载时占用服务器带宽，请勿在游戏时进行此操作。"
          type="warning"
          show-icon
        />
        <a-alert
          v-if="exportProgress?.status === 'failed'"
          :message="exportProgress.message || '导出失败'"
          type="error"
          show-icon
        />
        <a-alert
          v-else-if="exportProgress?.status === 'cancelled'"
          message="导出已取消"
          type="info"
          show-icon
        />
        <a-progress
          :percent="exportPercent"
          :status="exportProgressStatus"
          :stroke-width="10"
        />
        <div class="text-sm text-gray-500 dark:text-gray-400">
          {{ exportProgressText }}
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <a-button
            v-if="isPluginExportActive"
            danger
            :loading="cancellingPluginExport"
            @click="cancelPluginExport(false)"
          >
            取消导出
          </a-button>
          <a-button v-else @click="exportProgressVisible = false">关闭</a-button>
        </div>
      </template>
    </a-modal>

    <a-drawer
      v-model:open="storeVisible"
      placement="right"
      :width="drawerWidth"
      :bodyStyle="{ overflow: 'hidden', display: 'flex', flexDirection: 'column', padding: '24px' }"
    >
      <template #title>
        <div class="flex items-center justify-between">
          <span>插件商店</span>
          <a-button
            type="default"
            size="small"
            @click="openTokenModal"
            class="!flex !items-center !p-1"
          >
            <SettingOutlined />
            Github Token
          </a-button>
        </div>
      </template>
      <div ref="storeContainerRef" class="flex flex-col flex-1 min-h-0">
        <div ref="searchSectionRef" class="mb-4 space-y-4 flex-shrink-0">
          <div class="flex flex-col sm:flex-row sm:items-center gap-2">
            <span
              class="whitespace-nowrap font-medium text-gray-700 dark:text-gray-300 flex items-center gap-1"
              >自定义仓库:
              <a-tooltip placement="topLeft" :getPopupContainer="getRepoTooltipContainer">
                <template #title>
                  <div style="max-width: 280px; word-break: break-all; white-space: normal">
                    仓库必须为公开仓库，且根目录下需包含 plugins/
                    文件夹（每个插件一个子目录），分支固定为 master。
                  </div>
                </template>
                <span
                  class="text-gray-400 cursor-help hover:text-gray-500 dark:hover:text-gray-300 text-xs"
                  >(?)</span
                >
              </a-tooltip>
            </span>
            <a-select
              v-model:value="customRepo"
              class="w-full sm:flex-1"
              :options="repoOptions"
              mode="tags"
              :max-tag-count="1"
              show-search
              allow-clear
              placeholder="默认使用官方仓库"
            />
          </div>

          <div class="flex flex-col sm:flex-row sm:items-center gap-2">
            <span class="whitespace-nowrap font-medium text-gray-700 dark:text-gray-300"
              >加速源:</span
            >
            <a-select
              v-model:value="selectedProxy"
              class="w-full sm:flex-1"
              :options="proxyOptions"
              mode="tags"
              :max-tag-count="1"
              show-search
              allow-clear
              placeholder="默认直连 (不使用加速)"
            />
          </div>

          <div class="flex flex-col sm:flex-row gap-2">
            <a-input
              v-model:value="storeSearchText"
              placeholder="搜索商店插件..."
              allow-clear
              class="w-full sm:flex-1"
            >
              <template #prefix>
                <SearchOutlined class="text-gray-400" />
              </template>
            </a-input>
            <a-button
              @click="fetchStorePlugins(true)"
              :loading="storeLoading"
              class="w-full sm:w-auto !flex !items-center !justify-center"
            >
              <template #icon><SyncOutlined /></template>
              刷新
            </a-button>
          </div>

          <div class="flex items-center gap-2">
            <span class="text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap"
              >安装状态:</span
            >
            <div class="flex gap-1">
              <a-button
                :type="storeInstallFilter === 'all' ? 'primary' : 'default'"
                size="small"
                @click="storeInstallFilter = 'all'"
                >全部</a-button
              >
              <a-button
                :type="storeInstallFilter === 'not-installed' ? 'primary' : 'default'"
                size="small"
                @click="storeInstallFilter = 'not-installed'"
                >未安装</a-button
              >
              <a-button
                :type="storeInstallFilter === 'installed' ? 'primary' : 'default'"
                size="small"
                @click="storeInstallFilter = 'installed'"
                >已安装</a-button
              >
            </div>
          </div>
        </div>

        <a-table
          :columns="storeColumns"
          :data-source="filteredStorePlugins"
          :loading="storeLoading"
          row-key="name"
          :scroll="{ x: isMobile ? 'max-content' : undefined, y: tableScrollY }"
          :pagination="storePagination"
          @change="handleStoreTableChange"
          size="middle"
          :customRow="storePluginRow"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'name'">
              <a-tooltip v-if="!isMobile" placement="topLeft" :getPopupContainer="getBody">
                <template #title>
                  <div style="word-break: break-all; white-space: normal; max-width: 280px">
                    {{ record.name }}
                  </div>
                </template>
                <div class="truncate">{{ record.name }}</div>
              </a-tooltip>
              <span v-else>{{ record.name }}</span>
            </template>

            <template v-else-if="column.key === 'size'">
              {{ formatSize(record.size) }}
            </template>

            <template v-else-if="column.key === 'actions'">
              <div class="flex items-center gap-1">
                <a-button
                  type="default"
                  size="small"
                  class="!flex !items-center !justify-center"
                  @click="openDetail((record as StorePlugin).name, true)"
                >
                  <template #icon><FileTextOutlined /></template>
                  详情
                </a-button>
                <a-tag
                  v-if="(record as StorePlugin).installed"
                  color="success"
                  class="!flex !items-center !w-fit gap-1 !cursor-default"
                >
                  <template #icon><CheckCircleOutlined /></template>
                  已安装
                </a-tag>
                <a-popconfirm
                  v-else-if="isStoreDownloadActive(record as StorePlugin)"
                  title="确定要取消下载这个插件吗？"
                  ok-text="确定"
                  cancel-text="取消"
                  :getPopupContainer="getBody"
                  @confirm="cancelStoreDownload(record as StorePlugin)"
                >
                  <a-button
                    type="primary"
                    danger
                    size="small"
                    class="!flex !items-center !justify-center min-w-[64px]"
                  >
                    <template #icon><SyncOutlined /></template>
                    {{ getStoreDownloadLabel(record as StorePlugin) }}
                  </a-button>
                </a-popconfirm>
                <a-button
                  v-else
                  type="primary"
                  size="small"
                  @click="downloadFromStore(record as StorePlugin)"
                  class="!flex !items-center !justify-center"
                >
                  <template #icon><DownloadOutlined /></template>
                  下载
                </a-button>
              </div>
            </template>
          </template>
        </a-table>
      </div>
    </a-drawer>

    <a-modal
      v-model:open="tokenModalVisible"
      title="GitHub Token 设置"
      :getContainer="getModalContainer"
      :zIndex="2000"
      :width="400"
      @cancel="tokenModalVisible = false"
    >
      <div>
        <a-alert message="设置 GitHub Token 可以提高 API 请求频率限制" type="info" show-icon />
        <a-input-password
          v-model:value="githubToken"
          placeholder="输入 GitHub Token (可选)"
          :visibilityToggle="true"
          class="w-full mt-4"
        />
        <a
          href="https://github.com/settings/tokens/new?description=L4D2%20Plugin%20Store&scopes=public_repo"
          target="_blank"
          class="inline-flex items-center mt-3 text-blue-500 hover:text-blue-600 dark:text-blue-400 dark:hover:text-blue-300"
        >
          <LinkOutlined class="!mr-1" />
          创建 GitHub Token
        </a>
      </div>
      <template #footer>
        <div class="flex justify-between">
          <a-button @click="clearToken" danger>清除</a-button>
          <div class="flex gap-2">
            <a-button @click="tokenModalVisible = false">取消</a-button>
            <a-button type="primary" @click="saveToken">保存</a-button>
          </div>
        </div>
      </template>
    </a-modal>

    <PluginConfigModal v-model:open="configModalVisible" :plugin-name="currentConfigPlugin" />

    <PluginDetailModal
      v-model:open="detailModalVisible"
      :plugin-name="currentDetailPlugin"
      :is-store-plugin="currentDetailIsStore"
      :proxy-url="selectedProxy[0] || ''"
      :github-token="githubToken"
      :repo="customRepo[0] || ''"
    />
  </div>
</template>

<style scoped>
  /* 修复 Popconfirm 按钮在 flex 容器中换行的问题 */
  :deep(.ant-popconfirm-buttons) {
    display: flex;
    justify-content: flex-end; /* 按钮靠右对齐 */
    flex-wrap: nowrap;
    gap: 8px;
    white-space: nowrap;
  }

  :deep(.ant-popconfirm-buttons button) {
    margin-left: 0 !important;
  }

  /* Make Ant Design Upload component expand to fill flex container */
  :deep(.ant-upload-wrapper),
  :deep(.ant-upload) {
    display: block;
    width: 100%;
  }
  :deep(.ant-popconfirm-message) {
    white-space: nowrap;
  }

  :deep(.plugin-export-popconfirm) {
    max-width: min(360px, calc(100vw - 32px));
  }

  :deep(.plugin-export-popconfirm .ant-popconfirm-message) {
    align-items: flex-start;
    white-space: normal;
  }

  :deep(.plugin-export-popconfirm .ant-popconfirm-title) {
    white-space: normal;
    word-break: break-word;
  }

  /* 修复 RadioButton 垂直排列时的边框问题 */
  :deep(.ant-radio-button-wrapper) {
    margin-right: 0 !important;
    border-left-width: 1px !important;
  }

  :deep(.ant-radio-button-wrapper:not(:first-child)::before) {
    display: none !important;
  }
</style>
