package logic

import (
	"fmt"
	"l4d2-manager-next/consts"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const BackupFileName = "backups.yaml"

var backupMutex sync.Mutex

type BackupCvar struct {
	Value       string `yaml:"value" json:"value"`
	Default     string `yaml:"default,omitempty" json:"default,omitempty"`
	Min         string `yaml:"min,omitempty" json:"min,omitempty"`
	Max         string `yaml:"max,omitempty" json:"max,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type BackupPluginConfig struct {
	Name  string                `yaml:"name" json:"name"`
	Cvars map[string]BackupCvar `yaml:"cvars" json:"cvars"`
}

type BackupPlugin struct {
	Name    string               `yaml:"name" json:"name"`
	Configs []BackupPluginConfig `yaml:"configs" json:"configs"`
}

type BackupAdmin struct {
	SteamID string `yaml:"steamid" json:"steamid"`
	Remark  string `yaml:"remark,omitempty" json:"remark,omitempty"`
}

type BackupServerInfo struct {
	Hostname string `yaml:"hostname,omitempty" json:"hostname,omitempty"`
	Motd     string `yaml:"motd,omitempty" json:"motd,omitempty"`
	Host     string `yaml:"host,omitempty" json:"host,omitempty"`
}

type BackupConfig struct {
	Backups []BackupEntry `yaml:"backups"`
}

type BackupEntry struct {
	Name       string            `yaml:"name" json:"name"`
	CreatedAt  int64             `yaml:"created_at" json:"created_at"`
	Plugins    []BackupPlugin    `yaml:"plugins" json:"plugins"`
	Admins     []BackupAdmin     `yaml:"admins,omitempty" json:"admins,omitempty"`
	ServerInfo *BackupServerInfo `yaml:"server_info,omitempty" json:"server_info,omitempty"`
}

type BackupInfo struct {
	Name          string `json:"name"`
	CreatedAt     int64  `json:"created_at"`
	PluginCount   int    `json:"plugin_count"`
	AdminCount    int    `json:"admin_count"`
	HasServerInfo bool   `json:"has_server_info"`
}

func getBackupPath() string {
	return filepath.Join(getStorePath(), BackupFileName)
}

func loadBackupConfig() (*BackupConfig, error) {
	data, err := os.ReadFile(getBackupPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &BackupConfig{}, nil
		}
		return nil, err
	}

	var config BackupConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func saveBackupConfig(config *BackupConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(getBackupPath(), data, 0644)
}

func ListBackups() ([]BackupInfo, error) {
	backupMutex.Lock()
	defer backupMutex.Unlock()

	config, err := loadBackupConfig()
	if err != nil {
		return nil, err
	}

	infos := make([]BackupInfo, 0, len(config.Backups))
	for _, b := range config.Backups {
		infos = append(infos, BackupInfo{
			Name:          b.Name,
			CreatedAt:     b.CreatedAt,
			PluginCount:   len(b.Plugins),
			AdminCount:    len(b.Admins),
			HasServerInfo: b.ServerInfo != nil,
		})
	}
	return infos, nil
}

func GetBackupDetail(name string) (*BackupEntry, error) {
	backupMutex.Lock()
	defer backupMutex.Unlock()

	config, err := loadBackupConfig()
	if err != nil {
		return nil, err
	}

	for _, b := range config.Backups {
		if b.Name == name {
			entry := b
			return &entry, nil
		}
	}
	return nil, fmt.Errorf("未找到备份: %s", name)
}

func GetBackupPluginsDetail(name string) ([]BackupPlugin, error) {
	entry, err := GetBackupDetail(name)
	if err != nil {
		return nil, err
	}
	if entry.Plugins == nil {
		return []BackupPlugin{}, nil
	}
	return entry.Plugins, nil
}

func GetBackupAdminsDetail(name string) ([]BackupAdmin, error) {
	entry, err := GetBackupDetail(name)
	if err != nil {
		return nil, err
	}
	if entry.Admins == nil {
		return []BackupAdmin{}, nil
	}
	return entry.Admins, nil
}

func GetBackupServerInfoDetail(name string) (*BackupServerInfo, error) {
	entry, err := GetBackupDetail(name)
	if err != nil {
		return nil, err
	}
	return entry.ServerInfo, nil
}

func CreateBackup(name string) error {
	backupMutex.Lock()
	defer backupMutex.Unlock()

	config, err := loadBackupConfig()
	if err != nil {
		return fmt.Errorf("读取备份配置失败: %v", err)
	}

	// Check duplicate name
	for _, b := range config.Backups {
		if b.Name == name {
			return fmt.Errorf("备份名称 %s 已存在", name)
		}
	}

	// Get currently enabled plugins
	allPlugins, err := GetPlugins()
	if err != nil {
		return fmt.Errorf("获取插件列表失败: %v", err)
	}

	var backupPlugins []BackupPlugin
	for _, p := range allPlugins {
		if p.Status != "enabled" {
			continue
		}

		cfgFiles, err := GetPluginConfigs(p.Name)
		if err != nil {
			fmt.Printf("Warning: failed to get configs for plugin %s: %v\n", p.Name, err)
		}

		// Load original config values from the plugin store for comparison
		origValues := getStoreOriginalConfigs(p.Name)

		var backupCfgs []BackupPluginConfig
		for _, cfgFile := range cfgFiles {
			cvars := make(map[string]BackupCvar)
			for _, cvar := range cfgFile.Cvars {
				// Only record cvars that differ from the original/default
				origVal, hasOrig := origValues[cfgFile.FileName][cvar.Name]
				if hasOrig {
					// Compare against the store's original value
					if cvar.Value == origVal {
						continue
					}
				} else if cvar.Default != "" && cvar.Value == cvar.Default {
					// No store original, compare against Default from comments
					continue
				}

				cvars[cvar.Name] = BackupCvar{
					Value:       cvar.Value,
					Default:     cvar.Default,
					Min:         cvar.Min,
					Max:         cvar.Max,
					Description: cvar.Description,
				}
			}
			if len(cvars) > 0 {
				backupCfgs = append(backupCfgs, BackupPluginConfig{
					Name:  cfgFile.FileName,
					Cvars: cvars,
				})
			}
		}

		backupPlugins = append(backupPlugins, BackupPlugin{
			Name:    p.Name,
			Configs: backupCfgs,
		})
	}

	entry := BackupEntry{
		Name:       name,
		CreatedAt:  time.Now().Unix(),
		Plugins:    backupPlugins,
		Admins:     captureAdmins(),
		ServerInfo: captureServerInfo(),
	}

	config.Backups = append(config.Backups, entry)
	return saveBackupConfig(config)
}

type RestoreResult struct {
	Skipped []string `json:"skipped"`
}

func RestoreBackup(name string) (*RestoreResult, error) {
	backupMutex.Lock()
	defer backupMutex.Unlock()

	config, err := loadBackupConfig()
	if err != nil {
		return nil, fmt.Errorf("读取备份配置失败: %v", err)
	}

	var target *BackupEntry
	for _, b := range config.Backups {
		if b.Name == name {
			target = &b
			break
		}
	}

	if target == nil {
		return nil, fmt.Errorf("未找到备份: %s", name)
	}

	// Determine platform plugin
	platformKey := runtime.GOOS
	var platformPlugin string
	if presetData, err := os.ReadFile("preset.yaml"); err == nil {
		var presetCfg PresetConfig
		if err := yaml.Unmarshal(presetData, &presetCfg); err == nil {
			platformPlugin = presetCfg.Platform[platformKey]
		}
	}

	// Check which plugins are missing and filter them out
	storePath := getStorePath()
	var skipped []string
	var validPlugins []BackupPlugin
	for _, p := range target.Plugins {
		if _, err := os.Stat(filepath.Join(storePath, p.Name)); os.IsNotExist(err) {
			skipped = append(skipped, p.Name)
		} else {
			validPlugins = append(validPlugins, p)
		}
	}

	// Disable all currently enabled plugins
	allPlugins, err := GetPlugins()
	if err != nil {
		return nil, fmt.Errorf("获取插件列表失败: %v", err)
	}

	var toDisable []string
	for _, p := range allPlugins {
		if p.Status == "enabled" {
			toDisable = append(toDisable, p.Name)
		}
	}

	if len(toDisable) > 0 {
		if err := DisablePlugins(toDisable); err != nil {
			return nil, fmt.Errorf("禁用当前插件失败: %v", err)
		}
	}

	// Enable platform plugin first if it's in the backup
	if platformPlugin != "" {
		for _, p := range validPlugins {
			if p.Name == platformPlugin {
				if err := EnablePlugin(platformPlugin); err != nil {
					return nil, fmt.Errorf("启用平台插件 %s 失败: %v", platformPlugin, err)
				}
				break
			}
		}
	}

	// Enable all plugins from backup
	for _, p := range validPlugins {
		if p.Name == platformPlugin {
			continue // Already enabled
		}
		if err := EnablePlugin(p.Name); err != nil {
			return nil, fmt.Errorf("启用插件 %s 失败: %v", p.Name, err)
		}
	}

	// Apply configs
	for _, p := range validPlugins {
		for _, cfg := range p.Configs {
			cfgPath := filepath.Join(consts.GamePath, "cfg", "sourcemod", cfg.Name)
			if err := os.MkdirAll(filepath.Join(consts.GamePath, "cfg", "sourcemod"), 0755); err != nil {
				fmt.Printf("Warning: failed to create cfg directory: %v\n", err)
				continue
			}
			// Convert BackupCvar map to CvarConfig slice for full metadata write
			var cvars []CvarConfig
			for name, bc := range cfg.Cvars {
				cvars = append(cvars, CvarConfig{
					Name:        name,
					Value:       bc.Value,
					Default:     bc.Default,
					Min:         bc.Min,
					Max:         bc.Max,
					Description: bc.Description,
				})
			}
			if err := RestoreSourceModConfig(cfgPath, cvars); err != nil {
				fmt.Printf("Warning: failed to apply config %s: %v\n", cfg.Name, err)
			}
		}
	}

	// Restore admin list
	if len(target.Admins) > 0 {
		restoreAdminsToFile(target.Admins)
	}

	// Restore server info
	if target.ServerInfo != nil {
		restoreServerInfoToFiles(target.ServerInfo)
	}

	return &RestoreResult{Skipped: skipped}, nil
}

func RenameBackup(oldName, newName string) error {
	backupMutex.Lock()
	defer backupMutex.Unlock()

	config, err := loadBackupConfig()
	if err != nil {
		return fmt.Errorf("读取备份配置失败: %v", err)
	}

	// Check new name doesn't exist
	for _, b := range config.Backups {
		if b.Name == newName {
			return fmt.Errorf("备份名称 %s 已存在", newName)
		}
	}

	found := false
	for i, b := range config.Backups {
		if b.Name == oldName {
			config.Backups[i].Name = newName
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到备份: %s", oldName)
	}

	return saveBackupConfig(config)
}

func DeleteBackup(name string) error {
	backupMutex.Lock()
	defer backupMutex.Unlock()

	config, err := loadBackupConfig()
	if err != nil {
		return fmt.Errorf("读取备份配置失败: %v", err)
	}

	found := false
	newBackups := make([]BackupEntry, 0, len(config.Backups))
	for _, b := range config.Backups {
		if b.Name == name {
			found = true
			continue
		}
		newBackups = append(newBackups, b)
	}

	if !found {
		return fmt.Errorf("未找到备份: %s", name)
	}

	config.Backups = newBackups
	return saveBackupConfig(config)
}

// getStoreOriginalConfigs reads original cfg values from the plugin store directory.
// Returns map[cfgFileName]map[cvarName]value
func getStoreOriginalConfigs(pluginName string) map[string]map[string]string {
	storePath := getStorePath()
	result := make(map[string]map[string]string)

	cfgDirs := []string{
		filepath.Join(storePath, pluginName, "left4dead2", "cfg", "sourcemod"),
		filepath.Join(storePath, pluginName, "cfg", "sourcemod"),
	}

	for _, cfgDir := range cfgDirs {
		entries, err := os.ReadDir(cfgDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".cfg") {
				continue
			}
			cvars, err := ParseSourceModConfig(filepath.Join(cfgDir, entry.Name()))
			if err != nil {
				continue
			}
			values := make(map[string]string, len(cvars))
			for _, c := range cvars {
				values[c.Name] = c.Value
			}
			result[entry.Name()] = values
		}
	}

	return result
}

// captureAdmins reads the current admin list for backup. Returns nil on error or
// when the admin file doesn't exist (SourceMod not enabled).
func captureAdmins() []BackupAdmin {
	admins, err := ParseAdminsSimple()
	if err != nil {
		return nil
	}
	if len(admins) == 0 {
		return nil
	}
	result := make([]BackupAdmin, 0, len(admins))
	for _, a := range admins {
		result = append(result, BackupAdmin{
			SteamID: a.SteamID,
			Remark:  a.Remark,
		})
	}
	return result
}

// captureServerInfo reads hostname, motd and host for backup.
func captureServerInfo() *BackupServerInfo {
	info := &BackupServerInfo{}
	hasData := false

	hostnamePath := filepath.Join(consts.GamePath, "addons", "sourcemod", "configs", "l4d2_hostname.txt")
	motdPath := filepath.Join(consts.GamePath, "motd.txt")
	hostPath := filepath.Join(consts.GamePath, "host.txt")

	if data, err := os.ReadFile(hostnamePath); err == nil {
		info.Hostname = string(data)
		hasData = true
	}
	if data, err := os.ReadFile(motdPath); err == nil {
		info.Motd = string(data)
		hasData = true
	}
	if data, err := os.ReadFile(hostPath); err == nil {
		info.Host = string(data)
		hasData = true
	}

	if !hasData {
		return nil
	}
	return info
}

// restoreAdminsToFile overwrites the admins_simple.ini file with backed-up admins,
// preserving any header comment lines at the top of the file.
func restoreAdminsToFile(admins []BackupAdmin) {
	path := getAdminsFilePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println("Warning: admins_simple.ini not found, skipping admin restore")
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Warning: failed to read admins file: %v\n", err)
		return
	}

	lines := strings.Split(string(content), "\n")
	// Collect only leading comment/blank lines as the header to preserve
	var headerLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			headerLines = append(headerLines, line)
		} else {
			break
		}
	}

	var sb strings.Builder
	for _, line := range headerLines {
		sb.WriteString(line + "\n")
	}
	for _, admin := range admins {
		if admin.Remark != "" {
			sb.WriteString(fmt.Sprintf("\"%s\" \"99:z\" // %s\n", admin.SteamID, admin.Remark))
		} else {
			sb.WriteString(fmt.Sprintf("\"%s\" \"99:z\"\n", admin.SteamID))
		}
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		fmt.Printf("Warning: failed to write admins file: %v\n", err)
	}
}

// restoreServerInfoToFiles writes backed-up server info back to the respective files.
func restoreServerInfoToFiles(info *BackupServerInfo) {
	hostnamePath := filepath.Join(consts.GamePath, "addons", "sourcemod", "configs", "l4d2_hostname.txt")
	motdPath := filepath.Join(consts.GamePath, "motd.txt")
	hostPath := filepath.Join(consts.GamePath, "host.txt")

	if info.Hostname != "" {
		if err := os.WriteFile(hostnamePath, []byte(info.Hostname), 0644); err != nil {
			fmt.Printf("Warning: failed to restore hostname: %v\n", err)
		}
	}
	if info.Motd != "" {
		if err := os.WriteFile(motdPath, []byte(info.Motd), 0644); err != nil {
			fmt.Printf("Warning: failed to restore motd: %v\n", err)
		}
	}
	if info.Host != "" {
		if err := os.WriteFile(hostPath, []byte(info.Host), 0644); err != nil {
			fmt.Printf("Warning: failed to restore host: %v\n", err)
		}
	}
}
