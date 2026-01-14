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

	// Check multiple possible locations for plugins
	possiblePaths := []string{
		filepath.Join(storePath, pluginName, "left4dead2", "addons", "sourcemod", "plugins"),
		filepath.Join(storePath, pluginName, "addons", "sourcemod", "plugins"),
	}

	var configs []PluginConfigFile
	processedCfgs := make(map[string]bool)

	for _, pluginDir := range possiblePaths {
		entries, err := os.ReadDir(pluginDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".smx") {
				baseName := strings.TrimSuffix(entry.Name(), ".smx")
				cfgName := baseName + ".cfg"

				if processedCfgs[cfgName] {
					continue
				}

				// Check if cfg exists in server cfg/sourcemod
				cfgPath := filepath.Join(consts.GamePath, "cfg", "sourcemod", cfgName)

				if _, err := os.Stat(cfgPath); err == nil {
					// Parse it
					cvars, err := ParseSourceModConfig(cfgPath)
					if err != nil {
						fmt.Printf("Failed to parse config %s: %v\n", cfgPath, err)
						continue
					}

					configs = append(configs, PluginConfigFile{
						FileName: cfgName,
						Cvars:    cvars,
					})
					processedCfgs[cfgName] = true
				}
			}
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
