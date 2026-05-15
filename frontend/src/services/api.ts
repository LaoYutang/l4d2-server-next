import { useAuthStore } from '../stores/auth';

class ApiService {
  private getPassword() {
    const authStore = useAuthStore();
    return authStore.password;
  }

  private createFormData(data?: Record<string, any>) {
    const fd = new FormData();
    fd.append('password', this.getPassword());
    if (data) {
      Object.entries(data).forEach(([key, value]) => {
        if (value instanceof File) {
          fd.append(key, value);
        } else if (Array.isArray(value)) {
          value.forEach((v) => {
            if (v instanceof File) {
              fd.append(key, v);
            } else {
              fd.append(key, String(v));
            }
          });
        } else {
          fd.append(key, String(value));
        }
      });
    }
    return fd;
  }

  private handleResponseError(status: number) {
    if (status === 401 || status === 429) {
      const authStore = useAuthStore();
      authStore.logout();
      throw new Error('认证失效，请重新登录');
    }

    if (status === 403) {
      throw new Error('没有权限执行此操作');
    }
  }

  async post(url: string, data?: Record<string, any>) {
    const fd = this.createFormData(data);
    const response = await fetch(url, {
      method: 'POST',
      body: fd,
    });

    this.handleResponseError(response.status);

    return response;
  }

  async get(url: string, params?: Record<string, any>) {
    const urlObj = new URL(url, window.location.origin);
    urlObj.searchParams.append('password', this.getPassword());
    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        urlObj.searchParams.append(key, String(value));
      });
    }

    const response = await fetch(urlObj.toString(), {
      method: 'GET',
    });

    this.handleResponseError(response.status);

    return response;
  }

  async postJson(url: string, data: any) {
    const urlObj = new URL(url, window.location.origin);
    urlObj.searchParams.append('password', this.getPassword());

    const response = await fetch(urlObj.toString(), {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(data),
    });

    this.handleResponseError(response.status);

    return response;
  }

  async validatePassword() {
    const fd = new FormData();
    fd.append('password', this.getPassword());
    const response = await fetch('/auth', { method: 'POST', body: fd });
    if (response.ok) return { success: true };
    return { success: false, message: await response.text() };
  }

  async generateTempAuthCode(expiredHours: number) {
    const fd = new FormData();
    fd.append('password', this.getPassword());
    fd.append('expired', expiredHours.toString());

    const response = await fetch('/auth/getTempAuthCode', { method: 'POST', body: fd });
    this.handleResponseError(response.status);
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async getStatus() {
    // Authenticated request
    const response = await this.post('/rcon/getstatus');
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async fetchMapName(mapCode: string) {
    if (!mapCode) return mapCode;
    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 2000);

      const response = await fetch(`http://l4d2-maps.laoyutang.cn/${mapCode}`, {
        signal: controller.signal,
      });
      clearTimeout(timeoutId);

      if (response.ok) {
        const name = await response.text();
        return name.trim() || mapCode;
      }
    } catch (e) {
      console.warn('Map name fetch failed', e);
    }
    return mapCode;
  }

  async restartServer() {
    const response = await this.post('/restart');
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async setMaxPlayers(maxPlayers: number) {
    const response = await this.post('/rcon/setmaxplayers', { maxPlayers });
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async getPlugins() {
    const response = await this.post('/plugins/list');
    if (!response.ok) throw new Error(await response.text());
    const data = await response.json();
    return data || [];
  }

  async uploadPlugin(file: File | File[]) {
    const response = await this.post('/plugins/upload', { file });
    if (!response.ok) throw new Error(await response.text());
  }

  async enablePlugin(name: string) {
    const response = await this.post('/plugins/enable', { name });
    if (!response.ok) throw new Error(await response.text());
  }

  async enablePlugins(names: string[]) {
    const response = await this.postJson('/plugins/enable-batch', { names });
    if (!response.ok) throw new Error(await response.text());
  }

  async disablePlugin(name: string) {
    const response = await this.post('/plugins/disable', { name });
    if (!response.ok) throw new Error(await response.text());
  }

  async disablePlugins(names: string[]) {
    const response = await this.postJson('/plugins/disable-batch', { names });
    if (!response.ok) throw new Error(await response.text());
  }

  async deletePlugin(name: string) {
    const response = await this.post('/plugins/delete', { name });
    if (!response.ok) throw new Error(await response.text());
  }

  async getPluginConfigs(pluginName: string) {
    const response = await this.post('/plugins/config', { name: pluginName });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async updatePluginConfig(configName: string, updates: Record<string, string>) {
    const response = await this.postJson('/plugins/config/update', {
      config_name: configName,
      updates,
    });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async getPresets() {
    const response = await this.post('/plugins/presets');
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async applyPreset(name: string) {
    const response = await this.post('/plugins/apply-preset', { name });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async getStorePlugins(forceRefresh: boolean = false, proxyUrl: string = '', githubToken: string = '', repo: string = '') {
    const response = await this.postJson('/plugins/store/list', {
      force_refresh: forceRefresh,
      proxy_url: proxyUrl,
      github_token: githubToken,
      repo,
    });
    if (!response.ok) throw new Error(await response.text());
    const data = await response.json();
    return data || [];
  }

  async downloadStorePlugin(name: string, proxyUrl: string, githubToken: string = '', repo: string = '') {
    const response = await this.postJson('/plugins/store/download', {
      name,
      proxy_url: proxyUrl,
      github_token: githubToken,
      repo,
    });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async getStorePluginDownloadStatus(repo: string = '') {
    const response = await this.postJson('/plugins/store/download/status', { repo });
    if (!response.ok) throw new Error(await response.text());
    const data = await response.json();
    return data || [];
  }

  async cancelStorePluginDownload(name: string, repo: string = '') {
    const response = await this.postJson('/plugins/store/download/cancel', { name, repo });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async getPluginReadme(name: string, fromStore: boolean = false, proxyUrl: string = '', githubToken: string = '', repo: string = '') {
    const response = await this.postJson('/plugins/readme', {
      name,
      from_store: fromStore,
      proxy_url: proxyUrl,
      github_token: githubToken,
      repo,
    });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async listBackups() {
    const response = await this.post('/plugins/backups/list');
    if (!response.ok) throw new Error(await response.text());
    const data = await response.json();
    return data || [];
  }

  async createBackup(name: string) {
    const response = await this.post('/plugins/backups/create', { name });
    if (!response.ok) throw new Error(await response.text());
  }

  async restoreBackup(name: string): Promise<{ message: string; skipped?: string[] }> {
    const response = await this.post('/plugins/backups/restore', { name });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async renameBackup(oldName: string, newName: string) {
    const response = await this.postJson('/plugins/backups/rename', {
      old_name: oldName,
      new_name: newName,
    });
    if (!response.ok) throw new Error(await response.text());
  }

  async deleteBackup(name: string) {
    const response = await this.post('/plugins/backups/delete', { name });
    if (!response.ok) throw new Error(await response.text());
  }

  async getBackupPluginsDetail(name: string) {
    const response = await this.post('/plugins/backups/detail/plugins', { name });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async getBackupAdminsDetail(name: string) {
    const response = await this.post('/plugins/backups/detail/admins', { name });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async getBackupServerInfoDetail(name: string) {
    const response = await this.post('/plugins/backups/detail/server_info', { name });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async getBackupServerConfigDetail(name: string) {
    const response = await this.post('/plugins/backups/detail/server_config', { name });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async exportBackup(name: string) {
    const response = await this.post('/plugins/backups/export', { name });
    if (!response.ok) throw new Error(await response.text());
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${name}.yaml`;
    a.click();
    URL.revokeObjectURL(url);
  }

  async exportAllBackups() {
    const response = await this.post('/plugins/backups/export-all');
    if (!response.ok) throw new Error(await response.text());
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'backups_all.yaml';
    a.click();
    URL.revokeObjectURL(url);
  }

  async importBackup(file: File): Promise<{ message: string; count: number }> {
    const response = await this.post('/plugins/backups/import', { file });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async clearMaps() {
    const response = await this.post('/clear');
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async getMapList() {
    const response = await this.post('/list');
    if (!response.ok) throw new Error(await response.text());
    const text = await response.text();
    return text
      .split('\n')
      .filter((line) => line.trim())
      .map((line) => {
        const parts = line.split('$$');
        const name = parts[0] || 'unknown';
        const size = parts[1] || 'unknown';
        return { name, size, info: line };
      });
  }

  async getRconMapList() {
    const response = await this.post('/rcon/maplist');
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  private formatSpeed = (bytesPerSecond: number): string => {
    if (bytesPerSecond < 1024) {
      return `${bytesPerSecond.toFixed(2)} B/s`;
    } else if (bytesPerSecond < 1024 * 1024) {
      return `${(bytesPerSecond / 1024).toFixed(2)} KB/s`;
    } else if (bytesPerSecond < 1024 * 1024 * 1024) {
      return `${(bytesPerSecond / (1024 * 1024)).toFixed(2)} MB/s`;
    } else {
      return `${(bytesPerSecond / (1024 * 1024 * 1024)).toFixed(2)} GB/s`;
    }
  };

  private uploadChunkWithSignal(
    uploadId: string,
    chunkIndex: number,
    chunk: Blob,
    signal?: AbortSignal
  ): Promise<void> {
    return new Promise((resolve, reject) => {
      const fd = new FormData();
      fd.append('password', this.getPassword());
      fd.append('uploadId', uploadId);
      fd.append('chunkIndex', chunkIndex.toString());
      fd.append('chunk', new File([chunk], 'chunk'));

      const xhr = new XMLHttpRequest();
      let isAbortedByUser = false;

      const cleanup = () => {
        clearTimeout(timeoutId);
        if (signal) {
          signal.removeEventListener('abort', onAbort);
        }
      };

      const onAbort = () => {
        isAbortedByUser = true;
        xhr.abort();
      };

      if (signal) {
        signal.addEventListener('abort', onAbort);
        if (signal.aborted) {
          cleanup();
          reject(new Error('上传已取消'));
          return;
        }
      }

      // 30 秒超时：只 abort，不直接 reject，由 abort 事件统一处理
      const timeoutId = setTimeout(() => {
        xhr.abort();
      }, 30000);

      xhr.addEventListener('load', () => {
        cleanup();
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve();
        } else {
          try {
            this.handleResponseError(xhr.status);
            reject(new Error(xhr.responseText || '上传失败'));
          } catch (e) {
            reject(e);
          }
        }
      });

      xhr.addEventListener('error', () => {
        cleanup();
        reject(new Error('网络错误'));
      });

      xhr.addEventListener('abort', () => {
        cleanup();
        if (isAbortedByUser) {
          reject(new Error('上传已取消'));
        } else {
          reject(new Error('分片上传超时'));
        }
      });

      xhr.open('POST', '/upload/chunk');
      xhr.send(fd);
    });
  }

  private async uploadChunks(
    uploadId: string,
    file: File,
    pendingIndices: number[],
    totalChunks: number,
    completedCount: number,
    onProgress?: (data: { percent: number; speed: string }) => void,
    signal?: AbortSignal
  ): Promise<number[]> {
    const chunkSize = 5 * 1024 * 1024;
    const concurrency = 3;
    const uploaded: number[] = [];
    let activeCount = 0;
    let doneCount = completedCount;
    let error: Error | null = null;
    const queue = [...pendingIndices];

    const startTime = Date.now();

    return new Promise((resolve, reject) => {
      const checkDone = () => {
        if (activeCount === 0) {
          if (error) {
            reject(error);
          } else if (queue.length === 0) {
            resolve(uploaded);
          }
        }
      };

      const tryNext = () => {
        if (error) return;
        if (signal?.aborted) {
          error = new Error('上传已取消');
          checkDone();
          return;
        }
        while (activeCount < concurrency && queue.length > 0 && !error && !signal?.aborted) {
          const idx = queue.shift()!;
          activeCount++;
          const start = idx * chunkSize;
          const end = Math.min(start + chunkSize, file.size);
          const chunk = file.slice(start, end);

          this.uploadChunkWithSignal(uploadId, idx, chunk, signal)
            .then(() => {
              uploaded.push(idx);
              doneCount++;
              activeCount--;

              if (onProgress && !signal?.aborted) {
                const now = Date.now();
                const elapsed = (now - startTime) / 1000;
                const percent = (doneCount / totalChunks) * 100;
                // 已上传字节数：已完成分片 * chunkSize（最后一个是近似值）
                const bytesUploaded = Math.min(doneCount * chunkSize, file.size);
                const speed = elapsed > 0 ? this.formatSpeed(bytesUploaded / elapsed) : this.formatSpeed(0);
                onProgress({ percent, speed });
              }

              tryNext();
              checkDone();
            })
            .catch((e) => {
              activeCount--;
              if (!error) {
                error = e;
              }
              checkDone();
            });
        }
      };

      tryNext();
    });
  }

  async uploadMap(
    file: File,
    onProgress?: (data: { percent: number; speed: string }) => void,
    signal?: AbortSignal
  ): Promise<{ success: true } | { success: false; uploadId: string; uploadedChunks: number[] }> {
    const chunkSize = 5 * 1024 * 1024;
    const totalChunks = Math.ceil(file.size / chunkSize);

    // 1. init
    const initResponse = await this.post('/upload/init', {
      filename: file.name,
      fileSize: file.size,
      totalChunks,
    });
    if (!initResponse.ok) throw new Error(await initResponse.text());
    const { uploadId } = await initResponse.json();

    // 2. upload all chunks
    const pendingIndices: number[] = [];
    for (let i = 0; i < totalChunks; i++) pendingIndices.push(i);

    try {
      await this.uploadChunks(uploadId, file, pendingIndices, totalChunks, 0, onProgress, signal);
    } catch (e: any) {
      if (signal?.aborted) {
        throw new Error('上传已取消');
      }
      // 超时或网络错误：返回 uploadId 供续传使用
      return { success: false, uploadId, uploadedChunks: [] };
    }

    // 3. merge
    const mergeResponse = await this.post('/upload/merge', {
      uploadId,
      filename: file.name,
    });
    if (!mergeResponse.ok) throw new Error(await mergeResponse.text());
    return { success: true };
  }

  async resumeUpload(
    uploadId: string,
    file: File,
    onProgress?: (data: { percent: number; speed: string }) => void,
    signal?: AbortSignal
  ) {
    const chunkSize = 5 * 1024 * 1024;
    const totalChunks = Math.ceil(file.size / chunkSize);

    // 1. 获取服务端已上传分片
    const statusResponse = await this.post('/upload/status', { uploadId });
    if (!statusResponse.ok) throw new Error(await statusResponse.text());
    const { uploadedChunks: serverChunks } = await statusResponse.json();

    const uploadedSet = new Set(serverChunks);
    const pendingIndices: number[] = [];
    for (let i = 0; i < totalChunks; i++) {
      if (!uploadedSet.has(i)) pendingIndices.push(i);
    }

    if (pendingIndices.length === 0) {
      // 所有分片都已上传，直接 merge
      const mergeResponse = await this.post('/upload/merge', {
        uploadId,
        filename: file.name,
      });
      if (!mergeResponse.ok) throw new Error(await mergeResponse.text());
      return;
    }

    // 2. 上传缺失分片
    const completedCount = totalChunks - pendingIndices.length;
    await this.uploadChunks(uploadId, file, pendingIndices, totalChunks, completedCount, onProgress, signal);

    // 3. merge
    const mergeResponse = await this.post('/upload/merge', {
      uploadId,
      filename: file.name,
    });
    if (!mergeResponse.ok) throw new Error(await mergeResponse.text());
  }

  async cancelUpload(uploadId: string) {
    const response = await this.post('/upload/cancel', { uploadId });
    if (!response.ok) throw new Error(await response.text());
  }

  async deleteMap(mapName: string) {
    const fd = new FormData();
    fd.append('password', this.getPassword());
    fd.append('map', mapName);

    const response = await fetch('/remove', { method: 'POST', body: fd });
    this.handleResponseError(response.status);
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async changeMap(mapName: string) {
    const fd = new FormData();
    fd.append('password', this.getPassword());
    fd.append('mapName', mapName);

    const response = await fetch('/rcon/changemap', { method: 'POST', body: fd });
    this.handleResponseError(response.status);
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async setDifficulty(difficulty: string) {
    const response = await this.post('/rcon/changedifficulty', { difficulty });
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async setGameMode(gameMode: string) {
    const response = await this.post('/rcon/changegamemode', { gameMode });
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async sendRconCommand(cmd: string) {
    const response = await this.post('/rcon', { cmd });
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async kickUser(userName: string, userId: string) {
    const response = await this.post('/rcon/kickuser', { userName, userId });
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async banUser(steamId: string, kick: boolean = true) {
    const response = await this.post('/rcon/banuser', { steamId, kick });
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async getUserPlaytime(steamid: string) {
    const response = await this.post('/getUserPlaytime', { steamid });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async getDownloadTasks() {
    const response = await this.post('/download/list');
    if (!response.ok) throw new Error(await response.text());
    try {
      const data = await response.json();
      return data || [];
    } catch {
      return [];
    }
  }

  async addDownloadTask(url: string) {
    const fd = new FormData();
    fd.append('password', this.getPassword());
    fd.append('url', url);
    const response = await fetch('/download/add', { method: 'POST', body: fd });
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async restartDownloadTask(index: number) {
    const fd = new FormData();
    fd.append('password', this.getPassword());
    fd.append('index', index.toString());
    const response = await fetch('/download/restart', { method: 'POST', body: fd });
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async cancelDownloadTask(index: number) {
    const fd = new FormData();
    fd.append('password', this.getPassword());
    fd.append('index', index.toString());
    const response = await fetch('/download/cancel', { method: 'POST', body: fd });
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async clearDownloadTasks() {
    const response = await this.post('/download/clear');
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async getServerInfo() {
    const response = await this.post('/server-info/get');
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async updateServerInfo(data: { hostname: string; motd: string; host: string }) {
    const response = await this.postJson('/server-info/update', data);
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async getVersion() {
    const response = await this.post('/getVersion');
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async getAdmins() {
    const response = await this.post('/admins/list');
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async addAdmin(steamid: string, remark: string) {
    const response = await this.postJson('/admins/add', { steamid, remark });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async deleteAdmin(steamid: string) {
    const response = await this.postJson('/admins/delete', { steamid });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async getSourceModLogs() {
    const response = await this.post('/logs/list');
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  streamLog(
    filename: string,
    onLine: (line: string) => void,
    onError: (err: string) => void
  ): EventSource {
    const urlObj = new URL('/logs/stream', window.location.origin);
    urlObj.searchParams.append('password', this.getPassword());
    urlObj.searchParams.append('file', filename);

    const es = new EventSource(urlObj.toString());
    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.line !== undefined) {
          onLine(data.line);
        }
      } catch {
        onLine(event.data);
      }
    };
    es.onerror = () => {
      onError('连接中断');
    };
    return es;
  }

  async getSelfServiceStatus() {
    const response = await fetch('/self-service/status', { method: 'POST' });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async generateSelfServiceCode() {
    const response = await fetch('/self-service/generate', { method: 'POST' });
    if (!response.ok) {
      // Return error object if possible, or throw
      const text = await response.text();
      try {
        const json = JSON.parse(text);
        throw new Error(json.error || text);
      } catch (e: any) {
        if (e.message && e.message !== 'Unexpected token') throw e; // Already parsed error
        throw new Error(text);
      }
    }
    return response.json();
  }

  async setSelfServiceConfig(enable: boolean) {
    const response = await this.postJson('/config/self-service', { enable });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }
  async getMonitorConfig() {
    const response = await this.post('/monitor/config');
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async getMonitorHistory(start: number, end: number) {
    const response = await this.post('/monitor/history', { start, end });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async getServerConfig() {
    const response = await this.post('/server-config/get');
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async updateServerConfig(data: {
    hidden: boolean;
    lobby_connect_only: boolean;
    steam_group: string;
    custom_config: string[];
  }) {
    const response = await this.postJson('/server-config/update', data);
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }
}

export const api = new ApiService();
