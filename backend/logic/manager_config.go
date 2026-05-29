package logic

import (
	"encoding/json"
	"l4d2-manager-next/consts"
	"os"
	"sync"
	"time"
)

type ManagerConfig struct {
	EnableSelfService   bool      `json:"enable_self_service"`
	LastSelfServiceTime time.Time `json:"last_self_service_time"`
	EnablePlayerStats   bool      `json:"enable_player_stats"`
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
		EnableSelfService: false,
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

func UpdateLastSelfServiceTime() error {
	managerConfigMutex.Lock()
	defer managerConfigMutex.Unlock()
	managerConfig.LastSelfServiceTime = time.Now()
	return saveManagerConfig()
}
