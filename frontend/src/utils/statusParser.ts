import { DIFFICULTY_MAP, GAMEMODE_MAP } from './gameConstants';

export interface PlayerInfo {
  id: number | string;
  name: string;
  steamid: string;
  ip: string;
  status: string;
  delay: number;
  loss: number;
  duration: string;
  linkrate?: number;
  [key: string]: any;
}

export interface ParsedUsersItem {
  label: string;
  users: PlayerInfo[];
  icon: string;
  value?: string;
}

export interface ParsedStatusItem {
  label: string;
  value: string;
  icon: string;
}

export interface ParsedServerStatus {
  Hostname?: ParsedStatusItem;
  Map?: ParsedStatusItem;
  GameMode?: ParsedStatusItem;
  Difficulty?: ParsedStatusItem;
  Players?: ParsedStatusItem;
  Users?: ParsedUsersItem;
  [key: string]: ParsedStatusItem | ParsedUsersItem | undefined;
}

export function parseStatus(data: any): ParsedServerStatus {
  const result: ParsedServerStatus = {};
  let parsedData = data;

  // If string, try to parse as JSON or line-by-line
  if (typeof data === 'string') {
    try {
      parsedData = JSON.parse(data);
    } catch (e) {
      // Parse line by line
      const lines = data.split('\n').filter((line: string) => line.trim());
      lines.forEach((line: string) => {
        if (line.includes(':')) {
          const [key, ...valueParts] = line.split(':');
          if (!key) return;
          const value = valueParts.join(':').trim();
          const normalizedKey =
            key.trim().charAt(0).toUpperCase() + key.trim().slice(1).toLowerCase();
          result[normalizedKey] = {
            label: key.trim(),
            value: value,
            icon: '📝',
          };
        }
      });

      // Apply mappings for string parsed results
      if (result.Difficulty) {
        const rawDiff = result.Difficulty.value;
        const diffKey = Object.keys(DIFFICULTY_MAP).find(
          (k) => k.toLowerCase() === rawDiff.toLowerCase()
        );
        if (diffKey) {
          result.Difficulty.value = DIFFICULTY_MAP[diffKey] || rawDiff;
        }
      }

      // Handle GameMode/Mode normalization and mapping
      if (result.Mode || result.GameMode) {
        const modeItem = result.GameMode || result.Mode;
        if (modeItem && modeItem.value) {
          const rawMode = modeItem.value;
          const displayValue =
            GAMEMODE_MAP[rawMode] || GAMEMODE_MAP[rawMode.toLowerCase()] || rawMode;

          // Ensure GameMode exists and has mapped value
          result.GameMode = {
            ...modeItem,
            label: 'Game Mode',
            value: displayValue,
            icon: '🎮',
          };
        }
      }

      return result;
    }
  }

  // Handle object data
  if (typeof parsedData === 'object' && parsedData !== null) {
    const d = parsedData;

    if (d.hostname || d.Hostname) {
      result.Hostname = {
        label: 'Server Name',
        value: d.hostname || d.Hostname,
        icon: '🏠',
      };
    }

    if (d.map || d.Map) {
      result.Map = {
        label: 'Current Map',
        value: d.map || d.Map,
        icon: '🗺️',
      };
    }

    if (d.gameMode || d.GameMode) {
      const rawMode = d.gameMode || d.GameMode;
      // Try to find mapping, fallback to raw value
      let displayValue = GAMEMODE_MAP[rawMode] || GAMEMODE_MAP[rawMode.toLowerCase()] || rawMode;

      // Handle mutation mode specifics
      if ((rawMode === 'mutation' || displayValue === '突变') && (d.mutation || d.Mutation)) {
        const rawMutation = d.mutation || d.Mutation;
        // Try to map mutation code to name
        const mutationDisplay =
          GAMEMODE_MAP[rawMutation] || GAMEMODE_MAP[rawMutation.toLowerCase()];
        if (mutationDisplay) {
          displayValue = mutationDisplay; // Use mutation name directly
        } else {
          displayValue = rawMutation; // Fallback to raw mutation code
        }
      }

      result.GameMode = {
        label: 'Game Mode',
        value: displayValue,
        icon: '🎮',
      };
    }

    if (d.difficulty || d.Difficulty) {
      const rawDiff = d.difficulty || d.Difficulty;
      // Try to find mapping (case insensitive search for key), fallback to raw value
      const diffKey = Object.keys(DIFFICULTY_MAP).find(
        (k) => k.toLowerCase() === rawDiff.toLowerCase()
      );
      const displayValue = diffKey ? DIFFICULTY_MAP[diffKey] : rawDiff;

      result.Difficulty = {
        label: 'Difficulty',
        value: displayValue,
        icon: '💀',
      };
    }

    const players = d.players !== undefined ? d.players : d.Players;
    const maxPlayers = d.maxPlayers !== undefined ? d.maxPlayers : d.MaxPlayers;
    if (players !== undefined) {
      result.Players = {
        label: 'Players',
        value: maxPlayers ? `${players}/${maxPlayers}` : `${players}`,
        icon: '👥',
      };
    }

    const users = d.users || d.Users;
    if (users && Array.isArray(users)) {
      result.Users = {
        label: 'Online Users',
        users: users.map((u: any) => ({
          id: u.id || u.Id,
          name: u.name || u.Name,
          steamid: u.steamid || u.SteamId,
          ip: u.ip || u.Ip,
          status: u.status || u.Status,
          delay: u.delay || u.Delay,
          loss: u.loss || u.Loss,
          duration: u.duration || u.Duration,
          linkrate: u.linkrate || u.LinkRate,
        })),
        icon: '👥',
      };
    }
  }

  return result;
}

export function formatSteamId(steamId: string | number) {
  if (!steamId) return '';
  const str = String(steamId);
  if (str.length > 12) {
    return `${str.slice(0, 4)}...${str.slice(-8)}`;
  }
  return str;
}
