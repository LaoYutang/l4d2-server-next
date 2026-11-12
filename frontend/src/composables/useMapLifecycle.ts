import { onBeforeUnmount, ref } from 'vue';
import { api, type MapDeleteResult, type MapLifecycleItem } from '../services/api';
import { mapDeleteSummary } from '../utils/mapLifecycle';

export interface MapRemovalFeedback {
  type: 'success' | 'warning' | 'error';
  title: string;
  description: string;
  result?: MapDeleteResult;
}

export function useMapLifecycle() {
  const maps = ref<MapLifecycleItem[]>([]);
  const selectedRowKeys = ref<string[]>([]);
  const loading = ref(false);
  const loadError = ref('');
  const removing = ref(false);
  const lockingMaps = ref<Record<string, boolean>>({});
  const removalFeedback = ref<MapRemovalFeedback | null>(null);
  let loadRequestId = 0;
  let disposed = false;

  const loadMaps = async () => {
    const requestId = ++loadRequestId;
    loading.value = true;
    loadError.value = '';
    try {
      const result = await api.getMapLifecycleList();
      if (disposed || requestId !== loadRequestId) return;
      maps.value = result;
      const names = new Set(result.map((item) => item.name));
      selectedRowKeys.value = selectedRowKeys.value.filter((name) => names.has(name));
    } catch (error) {
      if (!disposed && requestId === loadRequestId) {
        loadError.value = error instanceof Error ? error.message : String(error);
      }
    } finally {
      if (!disposed && requestId === loadRequestId) loading.value = false;
    }
  };

  const setMapLocked = async (item: MapLifecycleItem, locked: boolean) => {
    if (removing.value || lockingMaps.value[item.name]) return;
    lockingMaps.value = { ...lockingMaps.value, [item.name]: true };
    try {
      const result = await api.setMapLocked(item.name, locked, item.revision);
      if (disposed) return;
      // Invalidate list requests made before this confirmed change.
      loadRequestId += 1;
      maps.value = maps.value.map((map) => map.name === item.name ? result : map);
    } finally {
      if (!disposed) {
        lockingMaps.value = { ...lockingMaps.value, [item.name]: false };
        await loadMaps();
      }
    }
  };

  const removeMaps = async (names?: string[], revisions?: Record<string, string>) => {
    if (removing.value) return;
    removing.value = true;
    removalFeedback.value = null;
    const targets = names ? [...names] : undefined;
    try {
      const result = targets ? await api.deleteMaps(targets, revisions) : await api.clearMaps();
      if (disposed) return;
      loadRequestId += 1;
      const deleted = new Set(result.deleted);
      maps.value = maps.value.filter((item) => !deleted.has(item.name));
      selectedRowKeys.value = selectedRowKeys.value.filter((name) => !deleted.has(name));
      const hasSkipped = result.skipped_locked.length + result.skipped_unknown_time.length + result.skipped_changed.length > 0;
      const hasPendingCleanup = result.pending_cleanup.length > 0;
      removalFeedback.value = {
        type: result.failed.length > 0 || hasSkipped || hasPendingCleanup ? 'warning' : result.deleted.length > 0 ? 'success' : 'warning',
        title: mapDeleteSummary(result),
        description: hasPendingCleanup
          ? '待恢复项已从地图列表移除，文件或检测记录的后续清理尚未完成。服务器会在后续恢复中重试。'
          : result.deleted.length === 0 && result.failed.length === 0
          ? '没有地图被删除。受保护的地图已保留。'
          : '已按服务器返回的实际结果更新列表。',
        result,
      };
    } catch (error) {
      if (!disposed) {
        removalFeedback.value = {
          type: 'warning',
          title: '删除未完成或结果未确认',
          description: `${error instanceof Error ? error.message : String(error)}。正在重新读取列表，请核对地图状态后再重试。`,
        };
      }
    } finally {
      if (!disposed) {
        await loadMaps();
        removing.value = false;
      }
    }
  };

  onBeforeUnmount(() => { disposed = true; loadRequestId += 1; });

  return { maps, selectedRowKeys, loading, loadError, removing, lockingMaps, removalFeedback, loadMaps, setMapLocked, removeMaps };
}
