package logic

import (
	"archive/zip"
	"fmt"
	"io"
	"l4d2-manager-next/consts"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/panjf2000/ants/v2"
	"github.com/spf13/viper"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	PluginStorePathEnv = "L4D2_PLUGIN_STORE_PATH"
	DefaultStorePath   = "./plugins"
	ConfigFileName     = "plugins.yaml"
	PluginsKey         = "enabled_plugins"
)

var (
	pluginMutex sync.Mutex
	configViper *viper.Viper
)

type Plugin struct {
	Name        string `json:"name"`
	Status      string `json:"status"` // "enabled" or "disabled"
	Description string `json:"description"`
}

type PluginConfig struct {
	Name  string   `mapstructure:"name"`
	Files []string `mapstructure:"files"`
}

func init() {
	configViper = viper.New()
	configViper.SetConfigType("yaml")
}

func getStorePath() string {
	path := os.Getenv(PluginStorePathEnv)
	if path == "" {
		// Check for local plugins directory for testing
		if _, err := os.Stat("./plugins"); err == nil {
			if abs, err := filepath.Abs("./plugins"); err == nil {
				return abs
			}
		}
		// Check for backend/plugins (if running from project root)
		if _, err := os.Stat("backend/plugins"); err == nil {
			if abs, err := filepath.Abs("backend/plugins"); err == nil {
				return abs
			}
		}
		return DefaultStorePath
	}
	return path
}

func getConfigPath() string {
	// Store config in plugins path as requested
	return filepath.Join(getStorePath(), ConfigFileName)
}

func loadConfig() error {
	configViper.SetConfigFile(getConfigPath())
	// Create file if not exists
	if _, err := os.Stat(getConfigPath()); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(getConfigPath()), 0755)
		os.Create(getConfigPath())
	}
	return configViper.ReadInConfig()
}

func GetPlugins() ([]Plugin, error) {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	if err := loadConfig(); err != nil {
		// It's okay if config doesn't exist or is empty initially
		// fmt.Println("Error loading config:", err)
	}

	storePath := getStorePath()
	entries, err := os.ReadDir(storePath)
	if err != nil {
		// Check if it's just not existing
		if os.IsNotExist(err) {
			return []Plugin{}, nil
		}
		return nil, err
	}

	// Use list structure to avoid key issues with dots and case sensitivity
	var enabledPlugins []PluginConfig
	if err := configViper.UnmarshalKey(PluginsKey, &enabledPlugins); err != nil {
		// fallback or ignore error?
	}

	enabledMap := make(map[string]bool)
	for _, p := range enabledPlugins {
		enabledMap[p.Name] = true
	}

	pluginMap := make(map[string]Plugin)

	// Add enabled plugins from config
	for _, p := range enabledPlugins {
		pluginMap[p.Name] = Plugin{
			Name:        p.Name,
			Status:      "enabled",
			Description: "Source missing", // Default description if not found on disk
		}
	}

	// Add/Update from disk
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Exact match check
		status := "disabled"
		if enabledMap[name] {
			status = "enabled"
		}

		pluginMap[name] = Plugin{
			Name:        name,
			Status:      status,
			Description: "",
		}
	}

	var plugins []Plugin
	for _, p := range pluginMap {
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// Convert zip filename from possible GBK to UTF-8
func decodeZipName(name string) string {
	if utf8.ValidString(name) {
		return name
	}
	// Try GBK
	decoder := simplifiedchinese.GBK.NewDecoder()
	utf8Name, _, err := transform.String(decoder, name)
	if err == nil {
		return utf8Name
	}
	// Fallback to original if conversion fails
	return name
}

func UploadPlugin(file io.ReaderAt, size int64, filename string) error {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	zipReader, err := zip.NewReader(file, size)
	if err != nil {
		return err
	}

	// Filter valid files and fix encoding
	var validFiles []*zip.File
	// Map to store decoded names to avoid re-decoding
	decodedNames := make(map[*zip.File]string)

	for _, f := range zipReader.File {
		decodedName := decodeZipName(f.Name)
		// Normalize path separators to forward slash
		decodedName = strings.ReplaceAll(decodedName, "\\", "/")

		if isJunkFile(decodedName) {
			continue
		}
		decodedNames[f] = decodedName
		validFiles = append(validFiles, f)
	}

	if len(validFiles) == 0 {
		return fmt.Errorf("empty zip file or only junk files")
	}

	// Case A: Single plugin (Root is left4dead2)
	isSinglePlugin := true
	for _, f := range validFiles {
		name := decodedNames[f]
		if !strings.HasPrefix(name, "left4dead2/") {
			isSinglePlugin = false
			break
		}
	}

	storePath := getStorePath()

	if isSinglePlugin {
		pluginName := strings.TrimSuffix(filename, filepath.Ext(filename))
		destDir := filepath.Join(storePath, pluginName)

		if _, err := os.Stat(destDir); !os.IsNotExist(err) {
			return fmt.Errorf("plugin %s already exists", pluginName)
		}

		return extractFiles(validFiles, destDir, "", decodedNames)
	}

	// Case B: Multiple plugins
	// Group by root directory
	pluginDirs := make(map[string][]*zip.File)

	for _, f := range validFiles {
		name := decodedNames[f]
		// Zip uses forward slash
		parts := strings.Split(name, "/")
		if len(parts) < 2 {
			// File at root (e.g. "readme.txt") -> Invalid for multi-plugin
			return fmt.Errorf("invalid structure: file %s at root", name)
		}
		rootDir := parts[0]
		pluginDirs[rootDir] = append(pluginDirs[rootDir], f)
	}

	// Validate each plugin dir
	for rootDir, files := range pluginDirs {
		// Strict check: every file must be either inside rootDir/left4dead2/ OR be the rootDir/left4dead2/ folder itself
		expectedPrefix := rootDir + "/left4dead2/"

		for _, f := range files {
			name := decodedNames[f]

			// Allow the directory itself (rootDir/left4dead2/)
			if name == expectedPrefix || name == strings.TrimSuffix(expectedPrefix, "/") {
				continue
			}

			if !strings.HasPrefix(name, expectedPrefix) {
				// Also allow rootDir/ itself if it's explicitly in the zip
				if name == rootDir || name == rootDir+"/" {
					continue
				}
				return fmt.Errorf("invalid structure in %s: must only contain left4dead2 folder, found %s", rootDir, name)
			}
		}

		// Ensure left4dead2 folder exists (either explicitly or implicitly)
		hasL4D2 := false
		for _, f := range files {
			name := decodedNames[f]
			if strings.HasPrefix(name, expectedPrefix) {
				hasL4D2 = true
				break
			}
		}

		if !hasL4D2 {
			return fmt.Errorf("invalid structure in %s: left4dead2 folder missing", rootDir)
		}

		// Check collision
		destDir := filepath.Join(storePath, rootDir)
		if _, err := os.Stat(destDir); !os.IsNotExist(err) {
			return fmt.Errorf("plugin %s already exists", rootDir)
		}
	}

	// Extract all
	for rootDir, files := range pluginDirs {
		destDir := filepath.Join(storePath, rootDir)
		if err := extractFiles(files, destDir, rootDir+"/", decodedNames); err != nil {
			return err
		}
	}

	return nil
}

func isJunkFile(name string) bool {
	if strings.HasPrefix(name, "__MACOSX/") {
		return true
	}
	if strings.HasSuffix(name, ".DS_Store") {
		return true
	}
	return false
}

func extractFiles(files []*zip.File, destDir string, stripPrefix string, decodedNames map[*zip.File]string) error {
	for _, f := range files {
		// Get decoded name
		name := decodedNames[f]

		// Strip prefix
		relPath := name
		if stripPrefix != "" {
			relPath = strings.TrimPrefix(name, stripPrefix)
		}

		if relPath == "" {
			continue
		}

		fpath := filepath.Join(destDir, relPath)

		// Prevent Zip Slip
		if !strings.HasPrefix(fpath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func EnablePlugin(name string) error {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	if err := loadConfig(); err != nil {
		// ignore
	}

	var enabledPlugins []PluginConfig
	if err := configViper.UnmarshalKey(PluginsKey, &enabledPlugins); err != nil {
		// ignore
	}

	for _, p := range enabledPlugins {
		if p.Name == name {
			return fmt.Errorf("plugin %s is already enabled", name)
		}
	}

	storePath := getStorePath()
	pluginDir := filepath.Join(storePath, name, "left4dead2")
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		return fmt.Errorf("plugin directory not found or invalid structure")
	}

	gamePath := consts.GamePath

	// Initialize plugin config
	newPlugin := PluginConfig{
		Name:  name,
		Files: []string{},
	}
	enabledPlugins = append(enabledPlugins, newPlugin)

	// Save initial state
	configViper.Set(PluginsKey, enabledPlugins)
	if err := configViper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save initial config: %v", err)
	}

	// Create a goroutine pool
	pool, err := ants.NewPool(runtime.NumCPU())
	if err != nil {
		return fmt.Errorf("failed to create goroutine pool: %v", err)
	}
	defer pool.Release()

	var wg sync.WaitGroup
	var configLock sync.Mutex
	var firstErr error
	var errOnce sync.Once

	err = filepath.Walk(pluginDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(pluginDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(gamePath, relPath)

		wg.Add(1)
		err = pool.Submit(func() {
			defer wg.Done()

			// Create dir (mkdirAll is thread safe enough for OS usually, or we can ignore errors if it exists)
			if err = os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				errOnce.Do(func() { firstErr = err })
				return
			}

			// Copy file
			if err = copyFile(path, destPath); err != nil {
				errOnce.Do(func() { firstErr = err })
				return
			}

			// Update config safely
			configLock.Lock()
			// Refresh reference to the plugin in the slice
			for i := range enabledPlugins {
				if enabledPlugins[i].Name == name {
					enabledPlugins[i].Files = append(enabledPlugins[i].Files, relPath)
					break
				}
			}
			configLock.Unlock()
		})

		if err != nil {
			wg.Done() // Decrement if submit fails
			return err
		}

		return nil
	})

	wg.Wait()

	if firstErr != nil {
		return firstErr
	}

	// Save final config once
	configLock.Lock()
	defer configLock.Unlock()
	configViper.Set(PluginsKey, enabledPlugins)
	return configViper.WriteConfig()
}

func DisablePlugin(name string) error {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	if err := loadConfig(); err != nil {
		return err
	}

	var enabledPlugins []PluginConfig
	if err := configViper.UnmarshalKey(PluginsKey, &enabledPlugins); err != nil {
		return err
	}

	var targetPlugin *PluginConfig
	targetIndex := -1

	for i, p := range enabledPlugins {
		if p.Name == name {
			targetPlugin = &enabledPlugins[i]
			targetIndex = i
			break
		}
	}

	if targetPlugin == nil {
		return fmt.Errorf("plugin %s is not enabled", name)
	}

	gamePath := consts.GamePath

	for _, relPath := range targetPlugin.Files {
		destPath := filepath.Join(gamePath, relPath)
		os.Remove(destPath)
		// Clean up empty parent directories?
		// Not strictly necessary but clean
	}

	// Remove from list
	enabledPlugins = append(enabledPlugins[:targetIndex], enabledPlugins[targetIndex+1:]...)
	configViper.Set(PluginsKey, enabledPlugins)

	return configViper.WriteConfig()
}

func DeletePlugin(name string) error {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	if err := loadConfig(); err != nil {
		// ignore
	}

	var enabledPlugins []PluginConfig
	if err := configViper.UnmarshalKey(PluginsKey, &enabledPlugins); err != nil {
		// ignore
	}

	for _, p := range enabledPlugins {
		if p.Name == name {
			return fmt.Errorf("cannot delete enabled plugin, disable it first")
		}
	}

	storePath := getStorePath()
	pluginDir := filepath.Join(storePath, name)

	return os.RemoveAll(pluginDir)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err = io.Copy(destFile, sourceFile); err != nil {
		return err
	}
	return destFile.Sync()
}
