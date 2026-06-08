package logic

import (
	"fmt"
	"l4d2-manager-next/consts"
	"os"
	"path/filepath"
	"strings"
)

type pluginConfigPathPair struct {
	PluginDir string
	ConfigDir string
}

func pluginConfigPathPairs(pluginName string) []pluginConfigPathPair {
	storePath := getStorePath()
	return []pluginConfigPathPair{
		{
			PluginDir: filepath.Join(storePath, pluginName, "left4dead2", "addons", "sourcemod", "plugins"),
			ConfigDir: filepath.Join(storePath, pluginName, "left4dead2", "cfg", "sourcemod"),
		},
		{
			PluginDir: filepath.Join(storePath, pluginName, "addons", "sourcemod", "plugins"),
			ConfigDir: filepath.Join(storePath, pluginName, "cfg", "sourcemod"),
		},
	}
}

func getPluginConfigCandidates(pluginName string) map[string]bool {
	candidateConfigs := make(map[string]bool)

	for _, paths := range pluginConfigPathPairs(pluginName) {
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

	return candidateConfigs
}

func readServerPluginConfigCvars(cfgName string) []CvarConfig {
	serverCfgPath := filepath.Join(consts.GamePath, "cfg", "sourcemod", cfgName)
	if _, errStat := os.Stat(serverCfgPath); errStat != nil {
		return nil
	}

	cvars, err := ParseSourceModConfig(serverCfgPath)
	if err != nil {
		fmt.Printf("Failed to parse server config %s: %v\n", serverCfgPath, err)
		return nil
	}
	return cvars
}

func pluginHasConfig(pluginName string) bool {
	for cfgName := range getPluginConfigCandidates(pluginName) {
		if len(readServerPluginConfigCvars(cfgName)) > 0 {
			return true
		}
	}
	return false
}

func GetPluginConfigs(pluginName string) ([]PluginConfigFile, error) {
	configs := make([]PluginConfigFile, 0, 2)
	candidateConfigs := getPluginConfigCandidates(pluginName)

	for cfgName := range candidateConfigs {
		if cvars := readServerPluginConfigCvars(cfgName); len(cvars) > 0 {
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
