import type { MapDeleteResult } from '../services/api';

export function formatMapUploadTime(value: string | null): string {
  if (!value) return '历史地图 · 无上传时间，不自动清理';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '上传时间异常，不自动清理';
  return `上传于 ${date.toLocaleString('zh-CN', { hour12: false })}`;
}

export function mapDeleteSummary(result: MapDeleteResult): string {
  const kept = result.skipped_locked.length + result.skipped_unknown_time.length + result.skipped_changed.length;
  const parts = [`已删除 ${result.deleted.length} 个`];
  if (kept > 0) parts.push(`保留 ${kept} 个`);
  if (result.failed.length > 0) parts.push(`失败 ${result.failed.length} 个`);
  if (result.pending_cleanup.length > 0) parts.push(`其中待恢复 ${result.pending_cleanup.length} 个`);
  return parts.join('，');
}

export function mapDeleteDetails(result: MapDeleteResult): Array<{ name: string; reason: string }> {
  return [
    ...result.skipped_locked.map((name) => ({ name, reason: '已锁定，保留地图' })),
    ...result.skipped_unknown_time.map((name) => ({ name, reason: '无上传时间，保留地图' })),
    ...result.skipped_changed.map((name) => ({ name, reason: '地图已变化，保留后重新检查' })),
    ...result.failed.map(({ name, error }) => ({ name, reason: error })),
    ...result.pending_cleanup.map((name) => ({ name, reason: '已从地图列表移除，文件或检测记录的后续清理待恢复' })),
  ];
}
