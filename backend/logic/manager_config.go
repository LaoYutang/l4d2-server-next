package logic

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

const ManagerConfigPath = "manager_config.json"

type ManagerConfig struct {
	EnableSelfService   bool      `json:"enable_self_service"`
	LastSelfServiceTime time.Time `json:"last_self_service_time"`
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

	if _, err := os.Stat(ManagerConfigPath); os.IsNotExist(err) {
		saveManagerConfig()
		return
	}

	data, err := os.ReadFile(ManagerConfigPath)
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
	return os.WriteFile(ManagerConfigPath, data, 0644)
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

func UpdateLastSelfServiceTime() error {
	managerConfigMutex.Lock()
	defer managerConfigMutex.Unlock()
	managerConfig.LastSelfServiceTime = time.Now()
	return saveManagerConfig()
}
