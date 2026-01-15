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

  async post(url: string, data?: Record<string, any>) {
    const fd = this.createFormData(data);
    const response = await fetch(url, {
      method: 'POST',
      body: fd,
    });

    if (response.status === 401 || response.status === 403 || response.status === 429) {
      const authStore = useAuthStore();
      authStore.logout();
      throw new Error('Authentication failed');
    }

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

    if (response.status === 401 || response.status === 403 || response.status === 429) {
      const authStore = useAuthStore();
      authStore.logout();
      throw new Error('Authentication failed');
    }

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

    if (response.status === 401 || response.status === 403 || response.status === 429) {
      const authStore = useAuthStore();
      authStore.logout();
      throw new Error('Authentication failed');
    }

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
    if (response.status === 401 || response.status === 403) {
      const authStore = useAuthStore();
      authStore.logout();
      throw new Error('Authentication failed');
    }
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

  async getPlugins() {
    const response = await this.post('/plugins/list');
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  async uploadPlugin(file: File | File[]) {
    const response = await this.post('/plugins/upload', { file });
    if (!response.ok) throw new Error(await response.text());
  }

  async enablePlugin(name: string) {
    const response = await this.post('/plugins/enable', { name });
    if (!response.ok) throw new Error(await response.text());
  }

  async disablePlugin(name: string) {
    const response = await this.post('/plugins/disable', { name });
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

  async uploadMap(file: File, onProgress?: (percent: number) => void) {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      const fd = new FormData();
      fd.append('password', this.getPassword());
      fd.append('map', file);

      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable && onProgress) {
          const percent = (e.loaded / e.total) * 100;
          onProgress(percent);
        }
      });

      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve(xhr.responseText);
        } else if (xhr.status === 401 || xhr.status === 403) {
          const authStore = useAuthStore();
          authStore.logout();
          reject(new Error('Authentication failed'));
        } else {
          reject(new Error(xhr.responseText));
        }
      });

      xhr.addEventListener('error', () => reject(new Error('Network error')));
      xhr.open('POST', '/upload');
      xhr.send(fd);
    });
  }

  async deleteMap(mapName: string) {
    const fd = new FormData();
    fd.append('password', this.getPassword());
    fd.append('map', mapName);

    const response = await fetch('/remove', { method: 'POST', body: fd });
    if (response.status === 401 || response.status === 403) {
      const authStore = useAuthStore();
      authStore.logout();
      throw new Error('Authentication failed');
    }
    if (!response.ok) throw new Error(await response.text());
    return response.text();
  }

  async changeMap(mapName: string) {
    const fd = new FormData();
    fd.append('password', this.getPassword());
    fd.append('mapName', mapName);

    const response = await fetch('/rcon/changemap', { method: 'POST', body: fd });
    if (response.status === 401 || response.status === 403) {
      const authStore = useAuthStore();
      authStore.logout();
      throw new Error('Authentication failed');
    }
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
}

export const api = new ApiService();
