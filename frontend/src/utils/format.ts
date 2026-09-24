const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];

/**
 * 把字节数格式化成人类可读文本，与监控页的显示习惯保持一致。
 */
export const formatBytes = (bytes: number): string => {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';

  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < BYTE_UNITS.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }

  const fractionDigits = unitIndex === 0 || value >= 100 ? 0 : 1;
  return `${value.toFixed(fractionDigits)} ${BYTE_UNITS[unitIndex]}`;
};

/**
 * 保留一位小数的百分比文本，用于磁盘使用率等指标。
 */
export const formatPercent = (value: number): string => {
  if (!Number.isFinite(value)) return '0%';
  return `${value.toFixed(1)}%`;
};
