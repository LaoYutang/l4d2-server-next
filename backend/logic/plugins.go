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
	// fileRefs 仅驻留内存，不落文件。进程启动后首次使用时从 enabled_plugins 重建。
	fileRefs map[string][]string
)

type Plugin struct {
	Name        string `json:"name"`
	Status      string `json:"status"` // "enabled" or "disabled"
	Description string `json:"description"`
	Source      string `json:"source"` // "panel", "store", or "upload"
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

	// Read plugin sources map
	sources := configViper.GetStringMapString("plugin_sources")

	pluginMap := make(map[string]Plugin)

	// Add enabled plugins from config
	for _, p := range enabledPlugins {
		source := sources[p.Name]
		if source == "" {
			source = "panel"
		}
		pluginMap[p.Name] = Plugin{
			Name:        p.Name,
			Status:      "enabled",
			Description: "Source missing", // Default description if not found on disk
			Source:      source,
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

		source := sources[name]
		if source == "" {
			source = "panel"
		}

		pluginMap[name] = Plugin{
			Name:        name,
			Status:      status,
			Description: "",
			Source:      source,
		}
	}

	plugins := make([]Plugin, 0, 64)
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

		if err := extractFiles(validFiles, destDir, "", decodedNames); err != nil {
			return err
		}
		writePluginSource(pluginName, "upload")
		return nil
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
		writePluginSource(rootDir, "upload")
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

func writePluginSource(name, source string) {
	loadConfig()
	sources := configViper.GetStringMapString("plugin_sources")
	if sources == nil {
		sources = make(map[string]string)
	}
	sources[name] = source
	configViper.Set("plugin_sources", sources)
	configViper.WriteConfig()
}

// normalizeRelPath 统一路径分隔符并转小写，用于 fileRefs 的 key
func normalizeRelPath(relPath string) string {
	return strings.ToLower(strings.ReplaceAll(relPath, "\\", "/"))
}

// rebuildFileRefs 从 enabled_plugins 重建 fileRefs
func rebuildFileRefs(enabledPlugins []PluginConfig) map[string][]string {
	refs := make(map[string][]string)
	for _, p := range enabledPlugins {
		for _, f := range p.Files {
			normPath := normalizeRelPath(f)
			found := false
			for _, existing := range refs[normPath] {
				if existing == p.Name {
					found = true
					break
				}
			}
			if !found {
				refs[normPath] = append(refs[normPath], p.Name)
			}
		}
	}
	return refs
}

// ensureFileRefs 确保 fileRefs 已初始化；调用前须持有 pluginMutex
func ensureFileRefs(enabledPlugins []PluginConfig) {
	if fileRefs == nil {
		fileRefs = rebuildFileRefs(enabledPlugins)
	}
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

	ensureFileRefs(enabledPlugins)

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
			for i := range enabledPlugins {
				if enabledPlugins[i].Name == name {
					enabledPlugins[i].Files = append(enabledPlugins[i].Files, relPath)
					break
				}
			}
			// 将本插件加入该文件的内存引用列表
			normPath := normalizeRelPath(relPath)
			alreadyRef := false
			for _, p := range fileRefs[normPath] {
				if p == name {
					alreadyRef = true
					break
				}
			}
			if !alreadyRef {
				fileRefs[normPath] = append(fileRefs[normPath], name)
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

	ensureFileRefs(enabledPlugins)

	for _, relPath := range targetPlugin.Files {
		normPath := normalizeRelPath(relPath)
		// 从引用列表中移除本插件
		newRefs := make([]string, 0, len(fileRefs[normPath]))
		for _, p := range fileRefs[normPath] {
			if p != name {
				newRefs = append(newRefs, p)
			}
		}
		if len(newRefs) == 0 {
			// 没有其他插件引用此文件，安全删除
			destPath := filepath.Join(gamePath, relPath)
			os.Remove(destPath)
			delete(fileRefs, normPath)
		} else {
			fileRefs[normPath] = newRefs
		}
	}

	// Remove from list
	enabledPlugins = append(enabledPlugins[:targetIndex], enabledPlugins[targetIndex+1:]...)
	configViper.Set(PluginsKey, enabledPlugins)

	return configViper.WriteConfig()
}

func DeletePlugin(name string) error {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	// 禁止删除当前系统对应的插件平台
	nameLower := strings.ToLower(name)
	if strings.Contains(nameLower, "插件平台") && strings.Contains(nameLower, runtime.GOOS) {
		return fmt.Errorf("不能删除当前系统对应的插件平台，该插件为必要组件")
	}

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

	if err := os.RemoveAll(pluginDir); err != nil {
		return err
	}

	// Clean up source record
	sources := configViper.GetStringMapString("plugin_sources")
	delete(sources, name)
	configViper.Set("plugin_sources", sources)
	configViper.WriteConfig()

	return nil
}

func EnablePlugins(names []string) error {
	var errs []string
	for _, name := range names {
		if err := EnablePlugin(name); err != nil {
			errs = append(errs, fmt.Sprintf("failed to enable %s: %v", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func DisablePlugins(names []string) error {
	var errs []string
	for _, name := range names {
		if err := DisablePlugin(name); err != nil {
			errs = append(errs, fmt.Sprintf("failed to disable %s: %v", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
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
