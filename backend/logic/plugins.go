package logic

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	PluginStorePathEnv = "L4D2_PLUGIN_STORE_PATH"
	DefaultStorePath   = "./plugins"
	ConfigFileName     = "plugins.yaml"
	PluginsKey         = "enabled_plugins"
	DownloadTempDir    = ".download_temp"
	SMXPluginRelDir    = "addons/sourcemod/plugins"
)

type Plugin struct {
	Name        string `json:"name"`
	Status      string `json:"status"` // "enabled" or "disabled"
	Description string `json:"description"`
	Source      string `json:"source"` // "panel", "store", or "upload"
	HasSMX      bool   `json:"has_smx"`
	HasConfig   bool   `json:"has_config"`
	Type        string `json:"type"`
	ConfigMode  string `json:"config_mode"`
	TypeError   string `json:"type_error,omitempty"`
}

type PluginConfig struct {
	Name           string         `yaml:"name"`
	Files          []string       `yaml:"files"`
	DeploymentMode string         `yaml:"deployment_mode,omitempty"`
	VPKFile        string         `yaml:"vpk_file,omitempty"`
	ConfigFiles    []string       `yaml:"config_files,omitempty"`
	Extra          map[string]any `yaml:",inline"`
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

func GetPlugins() ([]Plugin, error) {
	defer acquirePluginOperation()()
	state, failures, err := initPluginMetadataLocked()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(getStorePath())
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	names, enabled := map[string]bool{}, map[string]bool{}
	for _, p := range state.Enabled {
		names[p.Name], enabled[p.Name] = true, true
	}
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names[e.Name()] = true
		}
	}
	result := make([]Plugin, 0, len(names))
	for name := range names {
		p := Plugin{Name: name, Status: "disabled", Source: "panel", Type: "unknown", ConfigMode: "none"}
		if enabled[name] {
			p.Status = "enabled"
		}
		if source, ok := state.Sources[name].(string); ok && source != "" {
			p.Source = source
		}
		if m := state.metadata(name); m != nil && validPluginType(m.Type) {
			p.Type = m.Type
		}
		p.TypeError = failures[name]
		root, err := openPluginRoot(name)
		if err != nil {
			p.Description = "Source missing"
		} else {
			root.Close()
		}
		p.HasSMX = pluginHasSMX(name)
		if p.Type == "nut" {
			if enabled[name] {
				if files, err := listPluginTextConfigsLocked(state, name); err == nil {
					p.HasConfig = len(files) > 0
				}
			}
			if p.HasConfig {
				p.ConfigMode = "text"
			}
		} else {
			p.HasConfig = pluginHasConfig(name)
			if p.HasConfig {
				p.ConfigMode = "sm"
			}
		}
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func pluginHasSMX(name string) bool {
	plugins, err := listPluginSMXIDs(name)
	return err == nil && len(plugins) > 0
}

func listPluginSMXIDs(name string) ([]string, error) {
	if err := validatePluginName(name); err != nil {
		return nil, err
	}
	root := filepath.Join(getStorePath(), name, "left4dead2", filepath.FromSlash(SMXPluginRelDir))
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var pluginIDs []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if strings.EqualFold(info.Name(), "disabled") && path != root {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.EqualFold(filepath.Ext(info.Name()), ".smx") {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		pluginID := filepath.ToSlash(relPath)
		pluginID = strings.TrimSuffix(pluginID, filepath.Ext(pluginID))
		pluginIDs = append(pluginIDs, pluginID)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(pluginIDs)
	return pluginIDs, nil
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
	defer acquirePluginOperation()()

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
		if _, err := cleanPluginPath(strings.TrimSuffix(decodedName, "/")); err != nil {
			return err
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("插件 ZIP 不支持符号链接")
		}
		decodedNames[f] = decodedName
		validFiles = append(validFiles, f)
	}

	if len(validFiles) == 0 {
		return fmt.Errorf("empty zip file or only junk files")
	}

	// Case A: Single plugin (Root is left4dead2, with optional markdown docs)
	isSinglePlugin := true
	hasSinglePluginRoot := false
	for _, f := range validFiles {
		name := decodedNames[f]
		if name == "left4dead2" || strings.HasPrefix(name, "left4dead2/") {
			hasSinglePluginRoot = true
			continue
		}
		if isPluginRootMarkdown(name) {
			continue
		}
		if !strings.HasPrefix(name, "left4dead2/") {
			isSinglePlugin = false
			break
		}
	}
	if !hasSinglePluginRoot {
		isSinglePlugin = false
	}

	storePath := getStorePath()

	if isSinglePlugin {
		pluginName := strings.TrimSuffix(filename, filepath.Ext(filename))
		if err := validatePluginName(pluginName); err != nil {
			return err
		}
		destDir := filepath.Join(storePath, pluginName)

		if _, err := os.Stat(destDir); !os.IsNotExist(err) {
			return fmt.Errorf("plugin %s already exists", pluginName)
		}

		if err := extractFiles(validFiles, destDir, "", decodedNames); err != nil {
			rollbackUploadedPlugin(pluginName)
			return err
		}
		if err := registerPluginLocked(pluginName, "upload"); err != nil {
			rollbackUploadedPlugin(pluginName)
			return err
		}
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
			// File at zip root (e.g. "readme.txt") -> Invalid for multi-plugin
			return fmt.Errorf("invalid structure: file %s at root", name)
		}
		rootDir := parts[0]
		pluginDirs[rootDir] = append(pluginDirs[rootDir], f)
	}

	// Validate each plugin dir
	for rootDir, files := range pluginDirs {
		if err := validatePluginName(rootDir); err != nil {
			return err
		}
		// Strict check: every file must be inside rootDir/left4dead2/ or be a markdown doc in the plugin root.
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
				if strings.HasPrefix(name, rootDir+"/") && isPluginRootMarkdown(strings.TrimPrefix(name, rootDir+"/")) {
					continue
				}
				return fmt.Errorf("invalid structure in %s: must only contain left4dead2 folder and root markdown docs, found %s", rootDir, name)
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
			rollbackUploadedPlugin(rootDir)
			return err
		}
		if err := registerPluginLocked(rootDir, "upload"); err != nil {
			rollbackUploadedPlugin(rootDir)
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

func isPluginRootMarkdown(name string) bool {
	name = strings.Trim(name, "/")
	return name != "" && !strings.Contains(name, "/") && strings.HasSuffix(strings.ToLower(name), ".md")
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

func rollbackUploadedPlugin(name string) {
	root, err := os.OpenRoot(getStorePath())
	if err != nil {
		return
	}
	defer root.Close()
	_ = root.RemoveAll(name)
}

func normalizeRelPath(relPath string) string {
	return strings.ToLower(strings.ReplaceAll(relPath, "\\", "/"))
}

func EnablePlugin(name string) error  { return deployPlugin(name) }
func DisablePlugin(name string) error { return undeployPlugin(name) }

func EnableAndLoadPlugin(name string) error {
	smxPlugins, err := listPluginSMXIDs(name)
	if err != nil {
		return fmt.Errorf("failed to scan smx plugins: %v", err)
	}
	if len(smxPlugins) == 0 {
		return fmt.Errorf("plugin %s does not contain smx files", name)
	}

	if err := EnablePlugin(name); err != nil {
		return err
	}

	loadedPlugins := make([]string, 0, len(smxPlugins))
	for _, pluginID := range smxPlugins {
		if err := runSourceModPluginCommand("load", pluginID); err != nil {
			rollbackErr := rollbackLoadedSMXPlugins(loadedPlugins)
			if disableErr := DisablePlugin(name); disableErr != nil {
				rollbackErr = appendRollbackError(rollbackErr, fmt.Errorf("disable rollback failed: %v", disableErr))
			}
			if rollbackErr != nil {
				return fmt.Errorf("load smx plugin %s failed: %v; rollback failed: %v", pluginID, err, rollbackErr)
			}
			return fmt.Errorf("load smx plugin %s failed: %v", pluginID, err)
		}
		loadedPlugins = append(loadedPlugins, pluginID)
	}

	return nil
}

func LoadPlugin(name string) error {
	enabled, err := isPluginEnabled(name)
	if err != nil {
		return fmt.Errorf("failed to check plugin status: %v", err)
	}
	if !enabled {
		return fmt.Errorf("plugin %s is not enabled", name)
	}

	smxPlugins, err := listPluginSMXIDs(name)
	if err != nil {
		return fmt.Errorf("failed to scan smx plugins: %v", err)
	}
	if len(smxPlugins) == 0 {
		return fmt.Errorf("plugin %s does not contain smx files", name)
	}

	loadedPlugins := make([]string, 0, len(smxPlugins))
	for _, pluginID := range smxPlugins {
		if err := runSourceModPluginCommand("load", pluginID); err != nil {
			rollbackErr := rollbackLoadedSMXPlugins(loadedPlugins)
			if rollbackErr != nil {
				return fmt.Errorf("load smx plugin %s failed: %v; rollback failed: %v", pluginID, err, rollbackErr)
			}
			return fmt.Errorf("load smx plugin %s failed: %v", pluginID, err)
		}
		loadedPlugins = append(loadedPlugins, pluginID)
	}

	return nil
}

func isPluginEnabled(name string) (bool, error) {
	defer acquirePluginOperation()()
	state, err := readPluginState()
	if err != nil {
		return false, err
	}
	for _, p := range state.Enabled {
		if p.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func pluginExistsInStore(name string) (bool, error) {
	entries, err := os.ReadDir(getStorePath())
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == name && entry.Name() != DownloadTempDir {
			return true, nil
		}
	}
	return false, nil
}

func UnloadPlugin(name string) error {
	enabled, err := isPluginEnabled(name)
	if err != nil {
		return fmt.Errorf("failed to check plugin status: %v", err)
	}
	if enabled {
		return fmt.Errorf("plugin %s is not disabled", name)
	}

	exists, err := pluginExistsInStore(name)
	if err != nil {
		return fmt.Errorf("failed to check plugin directory: %v", err)
	}
	if !exists {
		return fmt.Errorf("plugin %s does not exist", name)
	}

	smxPlugins, err := listPluginSMXIDs(name)
	if err != nil {
		return fmt.Errorf("failed to scan smx plugins: %v", err)
	}
	if len(smxPlugins) == 0 {
		return fmt.Errorf("plugin %s does not contain smx files", name)
	}

	_, err = unloadSMXPlugins(smxPlugins)
	return err
}

func DisableAndUnloadPlugin(name string) error {
	smxPlugins, err := listPluginSMXIDs(name)
	if err != nil {
		return fmt.Errorf("failed to scan smx plugins: %v", err)
	}
	if len(smxPlugins) == 0 {
		return fmt.Errorf("plugin %s does not contain smx files", name)
	}

	unloadedPlugins, err := unloadSMXPlugins(smxPlugins)
	if err != nil {
		return err
	}

	if err := DisablePlugin(name); err != nil {
		rollbackErr := rollbackUnloadedSMXPlugins(unloadedPlugins)
		if rollbackErr != nil {
			return fmt.Errorf("disable plugin %s failed: %v; rollback failed: %v", name, err, rollbackErr)
		}
		return err
	}

	return nil
}

func unloadSMXPlugins(smxPlugins []string) ([]string, error) {
	unloadedPlugins := make([]string, 0, len(smxPlugins))
	for i := len(smxPlugins) - 1; i >= 0; i-- {
		pluginID := smxPlugins[i]
		if err := runSourceModPluginCommand("unload", pluginID); err != nil {
			rollbackErr := rollbackUnloadedSMXPlugins(unloadedPlugins)
			if rollbackErr != nil {
				return nil, fmt.Errorf("unload smx plugin %s failed: %v; rollback failed: %v", pluginID, err, rollbackErr)
			}
			return nil, fmt.Errorf("unload smx plugin %s failed: %v", pluginID, err)
		}
		unloadedPlugins = append(unloadedPlugins, pluginID)
	}

	return unloadedPlugins, nil
}

func runSourceModPluginCommand(action, pluginID string) error {
	cmd := fmt.Sprintf("sm plugins %s %s", action, quoteSourceModArg(pluginID))
	res, err := executePluginRconCommand(cmd)
	if err != nil {
		return err
	}
	if sourceModPluginCommandFailed(res) {
		return fmt.Errorf("%s", strings.TrimSpace(res))
	}
	return nil
}

func quoteSourceModArg(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func sourceModPluginCommandFailed(output string) bool {
	lower := strings.ToLower(strings.TrimSpace(output))
	if lower == "" {
		return false
	}

	failureMarkers := []string{
		"unknown command",
		"no such command",
		"failed",
		"error",
		"not found",
		"invalid",
		"could not",
		"unable to",
		"is not loaded",
		"no matching plugin",
	}
	for _, marker := range failureMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func rollbackLoadedSMXPlugins(pluginIDs []string) error {
	var errs []string
	for i := len(pluginIDs) - 1; i >= 0; i-- {
		if err := runSourceModPluginCommand("unload", pluginIDs[i]); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", pluginIDs[i], err))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

func rollbackUnloadedSMXPlugins(pluginIDs []string) error {
	var errs []string
	for i := len(pluginIDs) - 1; i >= 0; i-- {
		if err := runSourceModPluginCommand("load", pluginIDs[i]); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", pluginIDs[i], err))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

func appendRollbackError(existing error, next error) error {
	if next == nil {
		return existing
	}
	if existing == nil {
		return next
	}
	return fmt.Errorf("%v; %v", existing, next)
}

func DeletePlugin(name string) error {
	defer acquirePluginOperation()()
	if err := validatePluginName(name); err != nil {
		return err
	}
	state, err := readPluginState()
	if err != nil {
		return err
	}
	for _, p := range state.Enabled {
		if p.Name == name {
			return fmt.Errorf("cannot delete enabled plugin, disable it first")
		}
	}
	root, err := os.OpenRoot(getStorePath())
	if err != nil {
		return err
	}
	defer root.Close()
	tombstone := ".delete-" + name
	if _, err := root.Lstat(tombstone); !os.IsNotExist(err) {
		return fmt.Errorf("上次删除尚未清理: %s", tombstone)
	}
	if err := root.Rename(name, tombstone); err != nil {
		return err
	}
	for i, m := range state.Metadata {
		if m.Name == name {
			state.Metadata = append(state.Metadata[:i], state.Metadata[i+1:]...)
			break
		}
	}
	delete(state.Sources, name)
	if err := state.save(); err != nil {
		return appendRollbackError(err, root.Rename(tombstone, name))
	}
	return root.RemoveAll(tombstone)
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

func GetPluginReadme(name string) (content string, fileName string, err error) {
	storePath := getStorePath()
	pluginDir := filepath.Join(storePath, name)

	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return "", "", fmt.Errorf("插件目录不存在: %s", name)
	}

	var mdFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			mdFiles = append(mdFiles, e.Name())
		}
	}

	if len(mdFiles) == 0 {
		return "", "", fmt.Errorf("插件 %s 没有说明文档", name)
	}

	// Prefer README.md
	selectedFile := mdFiles[0]
	for _, f := range mdFiles {
		if strings.EqualFold(f, "README.md") {
			selectedFile = f
			break
		}
	}

	data, err := os.ReadFile(filepath.Join(pluginDir, selectedFile))
	if err != nil {
		return "", "", fmt.Errorf("读取说明文档失败: %v", err)
	}

	return string(data), selectedFile, nil
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
