package logic

import (
	"encoding/json"
	"fmt"
	"l4d2-manager-next/consts"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const DefaultMapHotReloadCommand = "update_addon_paths; mission_reload"

type ManagerConfig struct {
	EnableSelfService    bool      `json:"enable_self_service"`
	LastSelfServiceTime  time.Time `json:"last_self_service_time"`
	EnablePlayerStats    bool      `json:"enable_player_stats"`
	EnableMonitorHistory bool      `json:"enable_monitor_history"`
	EnableVPKTrim        bool      `json:"enable_vpk_trim"`
	MapHotReloadCommand  string    `json:"map_hot_reload_command"`
}

var (
	managerConfig      *ManagerConfig
	managerConfigMutex sync.RWMutex
)

func init() {
	LoadManagerConfig()
}

func LoadManagerConfig() {
	managerConfigMutex.Lock()
	defer managerConfigMutex.Unlock()

	managerConfig = &ManagerConfig{
		EnableSelfService:    false,
		EnablePlayerStats:    true,
		EnableMonitorHistory: true,
		EnableVPKTrim:        false,
		MapHotReloadCommand:  DefaultMapHotReloadCommand,
	}

	if _, err := os.Stat(consts.ManagerConfigPath); os.IsNotExist(err) {
		return
	}

	data, err := os.ReadFile(consts.ManagerConfigPath)
	if err != nil {
		return
	}

	json.Unmarshal(data, managerConfig)
}

func saveManagerConfig() error {
	data, err := json.MarshalIndent(managerConfig, "", "  ")
	if err != nil {
		return err
	}
	if err := consts.EnsureManagerDataPath(); err != nil {
		return err
	}
	return os.WriteFile(consts.ManagerConfigPath, data, 0644)
}

func GetSelfServiceConfig() ManagerConfig {
	managerConfigMutex.RLock()
	defer managerConfigMutex.RUnlock()
	return *managerConfig
}

func SetSelfServiceEnable(enable bool) error {
	managerConfigMutex.Lock()
	defer managerConfigMutex.Unlock()
	managerConfig.EnableSelfService = enable
	return saveManagerConfig()
}

func IsPlayerStatsEnabled() bool {
	managerConfigMutex.RLock()
	defer managerConfigMutex.RUnlock()
	return managerConfig.EnablePlayerStats
}

func SetPlayerStatsEnable(enable bool) error {
	managerConfigMutex.Lock()
	defer managerConfigMutex.Unlock()
	managerConfig.EnablePlayerStats = enable
	return saveManagerConfig()
}

func IsMonitorHistoryEnabled() bool {
	managerConfigMutex.RLock()
	defer managerConfigMutex.RUnlock()
	return managerConfig.EnableMonitorHistory
}

func SetMonitorHistoryEnable(enable bool) error {
	managerConfigMutex.Lock()
	defer managerConfigMutex.Unlock()
	managerConfig.EnableMonitorHistory = enable
	return saveManagerConfig()
}

func IsVPKTrimEnabled() bool {
	managerConfigMutex.RLock()
	defer managerConfigMutex.RUnlock()
	return managerConfig.EnableVPKTrim
}

func SetVPKTrimEnable(enable bool) error {
	managerConfigMutex.Lock()
	defer managerConfigMutex.Unlock()
	managerConfig.EnableVPKTrim = enable
	return saveManagerConfig()
}

func GetMapHotReloadCommand() string {
	managerConfigMutex.RLock()
	defer managerConfigMutex.RUnlock()
	if strings.TrimSpace(managerConfig.MapHotReloadCommand) == "" {
		return DefaultMapHotReloadCommand
	}
	return managerConfig.MapHotReloadCommand
}

func IsMapHotReloadCommandDefault() bool {
	return GetMapHotReloadCommand() == DefaultMapHotReloadCommand
}

func NormalizeMapHotReloadCommand(command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return DefaultMapHotReloadCommand, nil
	}
	if utf8.RuneCountInString(command) > 512 {
		return "", fmt.Errorf("热重载命令不能超过 512 个字符")
	}
	if strings.ContainsAny(command, "\r\n\x00") {
		return "", fmt.Errorf("热重载命令不能包含换行或空字符")
	}
	return command, nil
}

func SetMapHotReloadCommand(command string) (string, error) {
	normalized, err := NormalizeMapHotReloadCommand(command)
	if err != nil {
		return "", err
	}

	managerConfigMutex.Lock()
	defer managerConfigMutex.Unlock()
	managerConfig.MapHotReloadCommand = normalized
	return normalized, saveManagerConfig()
}

func UpdateLastSelfServiceTime() error {
	managerConfigMutex.Lock()
	defer managerConfigMutex.Unlock()
	managerConfig.LastSelfServiceTime = time.Now()
	return saveManagerConfig()
}
