package logic

import (
	"fmt"
	"l4d2-manager-next/consts"
	"os"
	"path/filepath"
	"strings"
)

func GetPluginConfigs(pluginName string) ([]PluginConfigFile, error) {
	// 1. Check if plugin is enabled (optional, but good for validation)
	// We can skip this and just look for files if we assume the UI only calls this for enabled plugins.
	// But let's check the store path to find what SMX files belong to this plugin.

	storePath := getStorePath()

	type PathPair struct {
		PluginDir string
		ConfigDir string
	}

	pathPairs := []PathPair{
		{
			PluginDir: filepath.Join(storePath, pluginName, "left4dead2", "addons", "sourcemod", "plugins"),
			ConfigDir: filepath.Join(storePath, pluginName, "left4dead2", "cfg", "sourcemod"),
		},
		{
			PluginDir: filepath.Join(storePath, pluginName, "addons", "sourcemod", "plugins"),
			ConfigDir: filepath.Join(storePath, pluginName, "cfg", "sourcemod"),
		},
	}

	configs := make([]PluginConfigFile, 0, 2)
	candidateConfigs := make(map[string]bool)

	for _, paths := range pathPairs {
		if entries, err := os.ReadDir(paths.PluginDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".smx") {
					baseName := strings.TrimSuffix(entry.Name(), ".smx")
					candidateConfigs[baseName+".cfg"] = true

					if strings.HasPrefix(baseName, "l4d2_") {
						candidateConfigs["l4d_"+strings.TrimPrefix(baseName, "l4d2_")+".cfg"] = true
					} else if strings.HasPrefix(baseName, "l4d_") {
						candidateConfigs["l4d2_"+strings.TrimPrefix(baseName, "l4d_")+".cfg"] = true
					}
				}
			}
		}

		if entries, err := os.ReadDir(paths.ConfigDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".cfg") {
					candidateConfigs[entry.Name()] = true
				}
			}
		}
	}

	for cfgName := range candidateConfigs {
		serverCfgPath := filepath.Join(consts.GamePath, "cfg", "sourcemod", cfgName)

		var cvars []CvarConfig
		var err error

		if _, errStat := os.Stat(serverCfgPath); errStat == nil {
			cvars, err = ParseSourceModConfig(serverCfgPath)
			if err != nil {
				fmt.Printf("Failed to parse server config %s: %v\n", serverCfgPath, err)
			}
		}

		if len(cvars) > 0 {
			configs = append(configs, PluginConfigFile{
				FileName: cfgName,
				Cvars:    cvars,
			})
		}
	}

	return configs, nil
}

func SavePluginConfig(configName string, updates map[string]string) error {
	// Security check: configName should be just a filename, no paths
	if strings.Contains(configName, "/") || strings.Contains(configName, "\\") {
		return fmt.Errorf("invalid config name")
	}

	cfgPath := filepath.Join(consts.GamePath, "cfg", "sourcemod", configName)
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found")
	}

	return UpdateSourceModConfig(cfgPath, updates)
}
