package logic

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	pluginSchemaVersion        = 2
	pluginTypeDetectionVersion = 2
)

// Serialize complete plugin operations, without holding a state mutex across IO.
// All config snapshots below are local to an operation.
var pluginOperations = make(chan struct{}, 1)

func acquirePluginOperation() func() {
	pluginOperations <- struct{}{}
	return func() { <-pluginOperations }
}

type PluginMetadata struct {
	ID                   string         `yaml:"id"`
	Name                 string         `yaml:"name"`
	Type                 string         `yaml:"type,omitempty"`
	TypeDetectionVersion int            `yaml:"type_detection_version,omitempty"`
	ConfigFiles          []string       `yaml:"config_files,omitempty"`
	EMSRoots             []string       `yaml:"ems_roots,omitempty"`
	Extra                map[string]any `yaml:",inline"`
}

type pluginState struct {
	document yaml.Node
	raw      []byte
	mode     fs.FileMode
	Version  int
	Metadata []PluginMetadata
	Enabled  []PluginConfig
	Sources  map[string]any
}

func readPluginState() (*pluginState, error) {
	s := &pluginState{mode: 0644, Sources: map[string]any{}}
	b, err := os.ReadFile(getConfigPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	s.raw = b
	if info, err := os.Stat(getConfigPath()); err == nil {
		s.mode = info.Mode().Perm()
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		s.document = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	} else if err := yaml.Unmarshal(b, &s.document); err != nil {
		return nil, fmt.Errorf("插件配置解析失败，原文件未修改: %w", err)
	}
	if len(s.document.Content) != 1 || s.document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("插件配置必须是 YAML 对象")
	}
	// Decode the full mapping once to reject duplicate keys.
	var fields map[string]yaml.Node
	if err := s.document.Content[0].Decode(&fields); err != nil {
		return nil, err
	}
	for key, target := range map[string]any{"schema_version": &s.Version, "plugin_metadata": &s.Metadata, "enabled_plugins": &s.Enabled, "plugin_sources": &s.Sources} {
		if n, ok := fields[key]; ok {
			if err := n.Decode(target); err != nil {
				return nil, fmt.Errorf("插件配置 %s 无效: %w", key, err)
			}
		}
	}
	if s.Version < 0 || s.Version > pluginSchemaVersion {
		return nil, fmt.Errorf("不支持插件配置版本 %d，原文件未修改", s.Version)
	}
	if s.Sources == nil {
		s.Sources = map[string]any{}
	}
	names := map[string]bool{}
	ids := map[string]bool{}
	for _, m := range s.Metadata {
		if err := validatePluginName(m.Name); err != nil {
			return nil, err
		}
		if names[m.Name] || (m.ID != "" && ids[m.ID]) {
			return nil, fmt.Errorf("插件元数据重复: %s", m.Name)
		}
		names[m.Name], ids[m.ID] = true, true
	}
	names = map[string]bool{}
	for _, p := range s.Enabled {
		if err := validatePluginName(p.Name); err != nil {
			return nil, err
		}
		if names[p.Name] {
			return nil, fmt.Errorf("启用记录重复: %s", p.Name)
		}
		names[p.Name] = true
	}
	return s, nil
}

func (s *pluginState) save() error {
	if err := os.MkdirAll(getStorePath(), 0755); err != nil {
		return err
	}
	if s.Version < pluginSchemaVersion && s.raw != nil {
		backup := getConfigPath() + ".v1.bak"
		f, err := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, s.mode)
		if err == nil {
			_, err = f.Write(s.raw)
			if err == nil {
				err = f.Sync()
			}
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		} else if !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	root := s.document.Content[0]
	for _, field := range []struct {
		key   string
		value any
	}{{"schema_version", pluginSchemaVersion}, {"plugin_metadata", s.Metadata}, {"enabled_plugins", s.Enabled}, {"plugin_sources", s.Sources}} {
		var n yaml.Node
		if err := n.Encode(field.value); err != nil {
			return err
		}
		found := false
		for i := 0; i < len(root.Content); i += 2 {
			if root.Content[i].Value == field.key {
				root.Content[i+1] = &n
				found = true
				break
			}
		}
		if !found {
			root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: field.key}, &n)
		}
	}
	b, err := yaml.Marshal(&s.document)
	if err != nil {
		return err
	}
	if err := atomicWriteFile(getConfigPath(), b, s.mode); err != nil {
		return err
	}
	s.Version, s.raw = pluginSchemaVersion, b
	return nil
}

func (s *pluginState) metadata(name string) *PluginMetadata {
	for i := range s.Metadata {
		if s.Metadata[i].Name == name {
			return &s.Metadata[i]
		}
	}
	return nil
}

func validPluginType(t string) bool { return t == "nut" || t == "sm" || t == "mix" || t == "other" }

func validatePluginName(name string) error {
	if name == "" || name != strings.TrimSpace(name) || strings.ContainsAny(name, "/\\:") || strings.IndexFunc(name, unicode.IsControl) >= 0 || !filepath.IsLocal(name) || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return fmt.Errorf("无效的插件名称")
	}
	return nil
}

func cleanPluginPath(name string) (string, error) {
	p := strings.ReplaceAll(name, "\\", "/")
	if p == "" || p == "." || path.Clean(p) != p || strings.HasPrefix(p, "/") || !filepath.IsLocal(filepath.FromSlash(p)) || strings.Contains(p, ":") || strings.IndexFunc(p, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("无效的插件文件路径: %s", name)
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." || strings.TrimRight(part, ". ") != part {
			return "", fmt.Errorf("无效的插件文件路径: %s", name)
		}
	}
	return p, nil
}

func openPluginRoot(name string) (*os.Root, error) {
	if err := validatePluginName(name); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(getStorePath())
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("插件目录不是普通目录: %s", name)
	}
	info, err = root.Lstat(filepath.Join(name, "left4dead2"))
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("left4dead2 不是普通目录: %s", name)
	}
	return root.OpenRoot(filepath.Join(name, "left4dead2"))
}

func isLooseNUTFile(p string) bool {
	p = strings.ToLower(p)
	return strings.HasPrefix(p, "cfg/") || strings.HasPrefix(p, "ems/") || strings.HasSuffix(p, ".cfg")
}

func isTextPluginConfig(p string) bool {
	p = strings.ToLower(p)
	if strings.HasSuffix(p, ".cfg") {
		return true
	}
	if !strings.HasPrefix(p, "cfg/") && !strings.HasPrefix(p, "ems/") {
		return false
	}
	switch path.Ext(p) {
	case ".txt", ".ini", ".json", ".yaml", ".yml":
		return true
	}
	return false
}

// These SourceMod build tools ship in Windows platform packages. They are
// development utilities, and do not introduce another plugin runtime.
func isSourceModBuildTool(p string) bool {
	switch p {
	case "addons/sourcemod/scripting/spcomp.exe",
		"addons/sourcemod/scripting/spcomp64.exe",
		"addons/sourcemod/scripting/compile.exe":
		return true
	}
	return false
}

func scanPlugin(name string) (PluginMetadata, []string, error) {
	m := PluginMetadata{Name: name, TypeDetectionVersion: pluginTypeDetectionVersion}
	root, err := openPluginRoot(name)
	if err != nil {
		return m, nil, err
	}
	defer root.Close()
	var files []string
	seen, ems := map[string]bool{}, map[string]bool{}
	hasNUT, hasSM, hasOtherRuntime := false, false, false
	err = fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("插件不支持符号链接: %s", p)
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("插件包含特殊文件: %s", p)
		}
		if _, err := cleanPluginPath(p); err != nil {
			return err
		}
		key := normalizeRelPath(p)
		if seen[key] {
			return fmt.Errorf("插件包含大小写冲突路径: %s", p)
		}
		seen[key] = true
		files = append(files, p)
		ext := path.Ext(key)
		if strings.HasPrefix(key, "scripts/vscripts/") && ext == ".nut" {
			hasNUT = true
		}
		if ext == ".smx" || ((ext == ".dll" || ext == ".so" || ext == ".vdf") && strings.HasPrefix(key, "addons/")) {
			hasSM = true
		} else if ext == ".dll" || ext == ".so" || (ext == ".exe" && !isSourceModBuildTool(key)) || ext == ".vpk" || ext == ".bsp" {
			hasOtherRuntime = true
		}
		if isTextPluginConfig(p) {
			m.ConfigFiles = append(m.ConfigFiles, p)
		}
		parts := strings.Split(p, "/")
		if len(parts) >= 3 && strings.EqualFold(parts[0], "ems") {
			ems[strings.Join(parts[:2], "/")] = true
		}
		return nil
	})
	if err != nil {
		return m, nil, err
	}
	switch {
	case hasNUT && hasSM:
		m.Type = "mix"
	case hasOtherRuntime:
		m.Type = "other"
	case hasNUT:
		m.Type = "nut"
	case hasSM:
		m.Type = "sm"
	default:
		m.Type = "other"
	}
	for p := range ems {
		m.EMSRoots = append(m.EMSRoots, p)
	}
	sort.Strings(files)
	sort.Strings(m.ConfigFiles)
	sort.Strings(m.EMSRoots)
	return m, files, nil
}

func refreshPluginMetadata(s *pluginState, name string) (PluginMetadata, []string, error) {
	m, files, err := scanPlugin(name)
	if err != nil {
		return m, nil, err
	}
	old := s.metadata(name)
	if old != nil {
		m.ID, m.Extra = old.ID, old.Extra
	}
	if _, err := uuid.Parse(m.ID); err != nil {
		m.ID = uuid.NewString()
	}
	if old == nil {
		s.Metadata = append(s.Metadata, m)
	} else {
		*old = m
	}
	return m, files, nil
}

func initPluginMetadataLocked() (*pluginState, map[string]string, error) {
	s, err := readPluginState()
	if err != nil {
		return nil, nil, err
	}
	entries, err := os.ReadDir(getStorePath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	errorsByName := map[string]string{}
	changed := s.Version != pluginSchemaVersion
	for i := range s.Enabled {
		if s.Enabled[i].DeploymentMode == "" {
			s.Enabled[i].DeploymentMode = "loose"
			changed = true
		}
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		m := s.metadata(name)
		if m != nil && validPluginType(m.Type) && m.TypeDetectionVersion >= pluginTypeDetectionVersion {
			if _, err := uuid.Parse(m.ID); err == nil {
				continue
			}
		}
		if _, _, err := refreshPluginMetadata(s, name); err != nil {
			errorsByName[name] = err.Error()
			continue
		}
		changed = true
	}
	if changed {
		if err := s.save(); err != nil {
			return nil, nil, err
		}
	}
	return s, errorsByName, nil
}

func InitPluginMetadata() error {
	defer acquirePluginOperation()()
	_, failures, err := initPluginMetadataLocked()
	if err != nil {
		return err
	}
	if len(failures) > 0 {
		return fmt.Errorf("部分插件识别失败，将在访问时重试: %v", failures)
	}
	return nil
}

func registerPluginLocked(name, source string) error {
	s, err := readPluginState()
	if err != nil {
		return err
	}
	if _, _, err := refreshPluginMetadata(s, name); err != nil {
		return err
	}
	s.Sources[name] = source
	return s.save()
}

func commitDownloadedPlugin(ctx context.Context, tempDir, name string) error {
	defer acquirePluginOperation()()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validatePluginName(name); err != nil {
		return err
	}
	storePath, err := filepath.Abs(getStorePath())
	if err != nil {
		return err
	}
	tempPath, err := filepath.Abs(tempDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(storePath, tempPath)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	if _, err := cleanPluginPath(rel); err != nil || !strings.HasPrefix(rel, DownloadTempDir+"/") {
		return fmt.Errorf("无效的下载暂存目录")
	}
	root, err := os.OpenRoot(storePath)
	if err != nil {
		return err
	}
	defer root.Close()
	if _, err := root.Lstat(name); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("插件 %s 已存在或不可访问", name)
	}
	if err := root.Rename(rel, name); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(err, root.Rename(name, rel))
	}
	if err := registerPluginLocked(name, "store"); err != nil {
		return errors.Join(err, root.Rename(name, rel))
	}
	return nil
}
