package logic

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"l4d2-manager-next/consts"
	"os"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const MaxPluginTextConfigBytes = 512 << 10

var (
	ErrPluginConfigUnavailable = errors.New("仅已启用的 NUT 插件可编辑文本配置")
	ErrPluginConfigPath        = errors.New("文件不属于该插件的可编辑配置")
	ErrPluginConfigConflict    = errors.New("配置已被修改，请重新读取；当前草稿已保留")
	ErrPluginConfigNotEditable = errors.New("配置不是可编辑文本或超过 512 KiB")
)

type PluginTextFile struct {
	Path     string   `json:"path"`
	Size     int64    `json:"size"`
	Editable bool     `json:"editable"`
	Error    string   `json:"error,omitempty"`
	SharedBy []string `json:"shared_by,omitempty"`
}

type PluginTextConfig struct {
	PluginTextFile
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	BOM      bool   `json:"bom"`
	Newline  string `json:"newline"`
	Revision string `json:"revision"`
}

type PluginTextBackup struct {
	Path     string `json:"path" yaml:"path"`
	Content  string `json:"content" yaml:"content"`
	Encoding string `json:"encoding" yaml:"encoding"`
	BOM      bool   `json:"bom,omitempty" yaml:"bom,omitempty"`
	Newline  string `json:"newline,omitempty" yaml:"newline,omitempty"`
}

func enabledNUTMetadata(state *pluginState, name string) (*PluginMetadata, error) {
	if err := validatePluginName(name); err != nil {
		return nil, ErrPluginConfigPath
	}
	m := state.metadata(name)
	if m == nil || m.Type != "nut" {
		return nil, ErrPluginConfigUnavailable
	}
	for _, p := range state.Enabled {
		if p.Name == name {
			return m, nil
		}
	}
	return nil, ErrPluginConfigUnavailable
}

func allowsPluginTextPath(m *PluginMetadata, p string) bool {
	if _, err := cleanPluginPath(p); err != nil {
		return false
	}
	if !isTextPluginConfig(p) {
		return false
	}
	key := normalizeRelPath(p)
	for _, file := range m.ConfigFiles {
		if normalizeRelPath(file) == key {
			return true
		}
	}
	for _, dir := range m.EMSRoots {
		if _, err := cleanPluginPath(dir); err == nil && strings.HasPrefix(normalizeRelPath(dir), "ems/") && strings.HasPrefix(key, normalizeRelPath(dir)+"/") {
			return true
		}
	}
	return false
}

func listPluginTextConfigsLocked(state *pluginState, name string) ([]PluginTextFile, error) {
	m, err := enabledNUTMetadata(state, name)
	if err != nil {
		return nil, err
	}
	game, err := os.OpenRoot(consts.GamePath)
	if err != nil {
		return nil, err
	}
	defer game.Close()
	paths := map[string]string{}
	for _, p := range m.ConfigFiles {
		if allowsPluginTextPath(m, p) {
			paths[normalizeRelPath(p)] = p
		}
	}
	for _, dir := range m.EMSRoots {
		if _, err := cleanPluginPath(dir); err != nil || !strings.HasPrefix(normalizeRelPath(dir), "ems/") {
			continue
		}
		// Opening a known EMS root is allowed, but never follow symlinks.
		if err := checkPluginFile(game, dir+"/.manager-owned-check", true); err != nil {
			return nil, err
		}
		err := fs.WalkDir(game.FS(), dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil
				}
				return err
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("EMS 配置范围包含符号链接: %s", p)
			}
			if !d.IsDir() && allowsPluginTextPath(m, p) {
				paths[normalizeRelPath(p)] = p
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	files := make([]PluginTextFile, 0, len(paths))
	for _, p := range paths {
		if _, err := game.Lstat(p); errors.Is(err, os.ErrNotExist) {
			continue
		}
		entry := PluginTextFile{Path: p, Editable: true}
		if err := checkPluginFile(game, p, false); err != nil {
			entry.Editable = false
			entry.Error = err.Error()
		} else if info, err := game.Stat(p); err == nil {
			entry.Size = info.Size()
			if entry.Size > MaxPluginTextConfigBytes {
				entry.Editable = false
				entry.Error = ErrPluginConfigNotEditable.Error()
			}
		}
		for _, other := range state.Enabled {
			if other.Name == name {
				continue
			}
			if om := state.metadata(other.Name); om != nil && allowsPluginTextPath(om, p) {
				entry.SharedBy = append(entry.SharedBy, other.Name)
			}
		}
		sort.Strings(entry.SharedBy)
		files = append(files, entry)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func ListPluginTextConfigs(name string) ([]PluginTextFile, error) {
	defer acquirePluginOperation()()
	state, err := readPluginState()
	if err != nil {
		return nil, err
	}
	return listPluginTextConfigsLocked(state, name)
}

func readPluginTextFile(game *os.Root, p string) (PluginTextConfig, os.FileMode, error) {
	result := PluginTextConfig{PluginTextFile: PluginTextFile{Path: p, Editable: true}, Encoding: "utf-8", Newline: "lf"}
	if err := checkPluginFile(game, p, false); err != nil {
		return result, 0, err
	}
	f, err := game.Open(p)
	if err != nil {
		return result, 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return result, 0, err
	}
	raw, err := io.ReadAll(io.LimitReader(f, MaxPluginTextConfigBytes+1))
	if err != nil {
		return result, 0, err
	}
	if len(raw) > MaxPluginTextConfigBytes {
		return result, 0, ErrPluginConfigNotEditable
	}
	result.Size = int64(len(raw))
	result.Revision = fmt.Sprintf("%x", sha256.Sum256(raw))
	result.BOM = bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	text := raw
	if result.BOM {
		text = text[3:]
	}
	if !utf8.Valid(text) {
		if result.BOM {
			return result, 0, ErrPluginConfigNotEditable
		}
		decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(text)
		if err != nil {
			return result, 0, ErrPluginConfigNotEditable
		}
		roundtrip, err := simplifiedchinese.GBK.NewEncoder().Bytes(decoded)
		if err != nil || !bytes.Equal(text, roundtrip) {
			return result, 0, ErrPluginConfigNotEditable
		}
		result.Encoding = "gbk"
		text = decoded
	}
	for _, r := range string(text) {
		if (unicode.IsControl(r) && r != '\r' && r != '\n' && r != '\t') || r == utf8.RuneError {
			return result, 0, ErrPluginConfigNotEditable
		}
	}
	if bytes.Contains(text, []byte("\r\n")) {
		result.Newline = "crlf"
	} else if bytes.Contains(text, []byte("\r")) {
		result.Newline = "cr"
	}
	result.Content = string(text)
	return result, info.Mode().Perm(), nil
}

func ReadPluginTextConfig(name, p string) (PluginTextConfig, error) {
	defer acquirePluginOperation()()
	state, err := readPluginState()
	if err != nil {
		return PluginTextConfig{}, err
	}
	m, err := enabledNUTMetadata(state, name)
	if err != nil {
		return PluginTextConfig{}, err
	}
	if !allowsPluginTextPath(m, p) {
		return PluginTextConfig{}, ErrPluginConfigPath
	}
	game, err := os.OpenRoot(consts.GamePath)
	if err != nil {
		return PluginTextConfig{}, err
	}
	defer game.Close()
	result, _, err := readPluginTextFile(game, p)
	return result, err
}

func encodePluginText(content, encoding, newline string, bom bool) ([]byte, error) {
	if !utf8.ValidString(content) {
		return nil, ErrPluginConfigNotEditable
	}
	for _, r := range content {
		if unicode.IsControl(r) && r != '\r' && r != '\n' && r != '\t' {
			return nil, ErrPluginConfigNotEditable
		}
	}
	content = strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	switch newline {
	case "crlf":
		content = strings.ReplaceAll(content, "\n", "\r\n")
	case "cr":
		content = strings.ReplaceAll(content, "\n", "\r")
	case "lf", "":
	default:
		return nil, ErrPluginConfigNotEditable
	}
	var raw []byte
	switch encoding {
	case "utf-8":
		raw = []byte(content)
		if bom {
			raw = append([]byte{0xef, 0xbb, 0xbf}, raw...)
		}
	case "gbk":
		if bom {
			return nil, ErrPluginConfigNotEditable
		}
		var err error
		raw, err = simplifiedchinese.GBK.NewEncoder().Bytes([]byte(content))
		if err != nil {
			return nil, fmt.Errorf("配置包含 GBK 无法表示的字符: %w", err)
		}
	default:
		return nil, ErrPluginConfigNotEditable
	}
	if len(raw) > MaxPluginTextConfigBytes {
		return nil, ErrPluginConfigNotEditable
	}
	return raw, nil
}

func atomicPluginTextWrite(game *os.Root, p string, raw []byte, mode os.FileMode) error {
	if err := checkPluginFile(game, p, true); err != nil {
		return err
	}
	if err := game.MkdirAll(path.Dir(p), 0755); err != nil {
		return err
	}
	temp := path.Join(path.Dir(p), ".manager-config-"+uuid.NewString()+".tmp")
	f, err := game.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer game.Remove(temp)
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	_, writeErr := f.Write(raw)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return err
	}
	if err := checkPluginFile(game, p, true); err != nil {
		return err
	}
	return game.Rename(temp, p)
}

func UpdatePluginTextConfig(name, p, content, expectedRevision string) (PluginTextConfig, error) {
	defer acquirePluginOperation()()
	state, err := readPluginState()
	if err != nil {
		return PluginTextConfig{}, err
	}
	m, err := enabledNUTMetadata(state, name)
	if err != nil {
		return PluginTextConfig{}, err
	}
	if !allowsPluginTextPath(m, p) {
		return PluginTextConfig{}, ErrPluginConfigPath
	}
	game, err := os.OpenRoot(consts.GamePath)
	if err != nil {
		return PluginTextConfig{}, err
	}
	defer game.Close()
	current, mode, err := readPluginTextFile(game, p)
	if err != nil {
		return current, err
	}
	if expectedRevision == "" || current.Revision != expectedRevision {
		return current, ErrPluginConfigConflict
	}
	raw, err := encodePluginText(content, current.Encoding, current.Newline, current.BOM)
	if err != nil {
		return current, err
	}
	if err := atomicPluginTextWrite(game, p, raw, mode); err != nil {
		return current, err
	}
	result, _, err := readPluginTextFile(game, p)
	return result, err
}

func capturePluginTextConfigs(name string) ([]PluginTextBackup, error) {
	defer acquirePluginOperation()()
	state, err := readPluginState()
	if err != nil {
		return nil, err
	}
	files, err := listPluginTextConfigsLocked(state, name)
	if err != nil {
		return nil, err
	}
	game, err := os.OpenRoot(consts.GamePath)
	if err != nil {
		return nil, err
	}
	defer game.Close()
	result := make([]PluginTextBackup, 0, len(files))
	for _, f := range files {
		c, _, err := readPluginTextFile(game, f.Path)
		if err != nil {
			return nil, fmt.Errorf("备份配置 %s: %w", f.Path, err)
		}
		result = append(result, PluginTextBackup{Path: c.Path, Content: c.Content, Encoding: c.Encoding, BOM: c.BOM, Newline: c.Newline})
	}
	return result, nil
}

func validatePluginTextBackup(m *PluginMetadata, configs []PluginTextBackup) error {
	if m.Type != "nut" {
		return ErrPluginConfigUnavailable
	}
	seen := map[string]bool{}
	for _, c := range configs {
		key := normalizeRelPath(c.Path)
		if !allowsPluginTextPath(m, c.Path) || seen[key] {
			return ErrPluginConfigPath
		}
		seen[key] = true
		if _, err := encodePluginText(c.Content, c.Encoding, c.Newline, c.BOM); err != nil {
			return err
		}
	}
	return nil
}

func restorePluginTextConfigs(name string, configs []PluginTextBackup) (returnErr error) {
	defer acquirePluginOperation()()
	state, err := readPluginState()
	if err != nil {
		return err
	}
	m, err := enabledNUTMetadata(state, name)
	if err != nil {
		return err
	}
	if err := validatePluginTextBackup(m, configs); err != nil {
		return err
	}
	game, err := os.OpenRoot(consts.GamePath)
	if err != nil {
		return err
	}
	defer game.Close()
	// Validate all files before changing any config.
	encoded := make([][]byte, len(configs))
	for i, c := range configs {
		if !allowsPluginTextPath(m, c.Path) {
			return ErrPluginConfigPath
		}
		if err := checkPluginFile(game, c.Path, true); err != nil {
			return err
		}
		encoded[i], err = encodePluginText(c.Content, c.Encoding, c.Newline, c.BOM)
		if err != nil {
			return err
		}
	}
	tx, err := newPluginFileTransaction(game)
	if err != nil {
		return err
	}
	defer tx.finish(&returnErr)
	for i, c := range configs {
		if err = tx.remember(c.Path); err != nil {
			break
		}
		mode := os.FileMode(0644)
		if info, e := game.Stat(c.Path); e == nil {
			mode = info.Mode().Perm()
		}
		if err = atomicPluginTextWrite(game, c.Path, encoded[i], mode); err != nil {
			break
		}
	}
	if err != nil {
		return err
	}
	return nil
}
