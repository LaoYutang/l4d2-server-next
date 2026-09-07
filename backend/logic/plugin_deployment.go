package logic

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/pkg/valve/vpk"
	"math"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// Refuse symlinks even within the root: otherwise a plugin could edit another
// plugin's config through an alias. Root also enforces containment during IO.
func checkPluginFile(root *os.Root, p string, missingOK bool) error {
	clean, err := cleanPluginPath(p)
	if err != nil {
		return err
	}
	parts := strings.Split(clean, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if missingOK && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("不允许符号链接: %s", p)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("路径不是目录: %s", p)
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return fmt.Errorf("不是普通文件: %s", p)
		}
	}
	return nil
}

type pluginFileChange struct{ path, original string }
type pluginFileTransaction struct {
	root    *os.Root
	dir     string
	changes []pluginFileChange
}

func newPluginFileTransaction(root *os.Root) (*pluginFileTransaction, error) {
	dir := ".manager-plugin-" + uuid.NewString()
	if err := root.Mkdir(dir, 0700); err != nil {
		return nil, err
	}
	return &pluginFileTransaction{root: root, dir: dir}, nil
}

func (t *pluginFileTransaction) remember(p string) error {
	if err := checkPluginFile(t.root, p, true); err != nil {
		return err
	}
	c := pluginFileChange{path: p}
	if _, err := t.root.Stat(p); err == nil {
		c.original = fmt.Sprintf("%s/backup-%d", t.dir, len(t.changes))
		if err := copyRootFile(t.root, p, t.root, c.original); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	t.changes = append(t.changes, c)
	return nil
}

func copyRootFile(src *os.Root, from string, dst *os.Root, to string) error {
	if err := checkPluginFile(src, from, false); err != nil {
		return err
	}
	f, err := src.Open(from)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if err := checkPluginFile(dst, to, true); err != nil {
		return err
	}
	if err := dst.MkdirAll(path.Dir(to), 0755); err != nil {
		return err
	}
	out, err := dst.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if err := out.Chmod(info.Mode().Perm()); err != nil {
		out.Close()
		return err
	}
	_, copyErr := io.Copy(out, f)
	if copyErr == nil {
		copyErr = out.Sync()
	}
	return errors.Join(copyErr, out.Close())
}

func (t *pluginFileTransaction) install(src *os.Root, from, to string) error {
	if err := t.remember(to); err != nil {
		return err
	}
	staged := fmt.Sprintf("%s/new-%d", t.dir, len(t.changes))
	if err := copyRootFile(src, from, t.root, staged); err != nil {
		return err
	}
	if err := t.root.MkdirAll(path.Dir(to), 0755); err != nil {
		return err
	}
	return t.root.Rename(staged, to)
}

func (t *pluginFileTransaction) rollback() error {
	var result error
	for i := len(t.changes) - 1; i >= 0; i-- {
		c := t.changes[i]
		if c.original != "" {
			result = errors.Join(result, t.root.Rename(c.original, c.path))
		} else if err := t.root.Remove(c.path); !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, err)
		}
	}
	return result
}

func (t *pluginFileTransaction) finish(returnErr *error) {
	if *returnErr != nil {
		if err := t.rollback(); err != nil {
			*returnErr = errors.Join(*returnErr, fmt.Errorf("回滚失败，恢复文件保留在 %s: %w", t.dir, err))
			return
		}
	}
	_ = t.root.RemoveAll(t.dir)
}

func deployPlugin(name string) (returnErr error) {
	defer acquirePluginOperation()()
	state, err := readPluginState()
	if err != nil {
		return err
	}
	for _, p := range state.Enabled {
		if p.Name == name {
			return fmt.Errorf("plugin %s is already enabled", name)
		}
	}
	meta, files, err := refreshPluginMetadata(state, name)
	if err != nil {
		return fmt.Errorf("识别插件失败: %w", err)
	}
	source, err := openPluginRoot(name)
	if err != nil {
		return err
	}
	defer source.Close()
	game, err := os.OpenRoot(consts.GamePath)
	if err != nil {
		return err
	}
	defer game.Close()
	if err := checkManagedPluginConflicts(state, meta, files, source, game); err != nil {
		return err
	}
	tx, err := newPluginFileTransaction(game)
	if err != nil {
		return err
	}
	defer tx.finish(&returnErr)
	record := PluginConfig{Name: name, DeploymentMode: "loose", Files: []string{}}
	if meta.Type == "nut" {
		record.DeploymentMode = "vpk"
		record.VPKFile = "addons/manager_nut_" + meta.ID + ".vpk"
		if _, err := game.Lstat(record.VPKFile); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("目标 VPK 已存在或不可访问: %s", record.VPKFile)
		}
		staged := tx.dir + "/plugin.vpk"
		if err := buildPluginVPK(source, files, meta, game, staged); err != nil {
			return err
		}
		if _, err := readPluginVPKDigests(game, staged); err != nil {
			return fmt.Errorf("校验 VPK 失败: %w", err)
		}
		for _, p := range files {
			if !isLooseNUTFile(p) {
				continue
			}
			if err := checkPluginFile(game, p, true); err != nil {
				return err
			}
			if _, err := game.Stat(p); errors.Is(err, os.ErrNotExist) {
				if err := tx.install(source, p, p); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			record.ConfigFiles = append(record.ConfigFiles, p)
		}
		if err := tx.remember(record.VPKFile); err != nil {
			return err
		}
		if err := game.MkdirAll("addons", 0755); err != nil {
			return err
		}
		if err := game.Rename(staged, record.VPKFile); err != nil {
			return err
		}
		record.Files = append(record.Files, record.VPKFile)
	} else {
		for _, p := range files {
			if err := tx.install(source, p, p); err != nil {
				return err
			}
			record.Files = append(record.Files, p)
		}
	}
	state.Enabled = append(state.Enabled, record)
	return state.save()
}

func undeployPlugin(name string) (returnErr error) {
	defer acquirePluginOperation()()
	if err := validatePluginName(name); err != nil {
		return err
	}
	state, err := readPluginState()
	if err != nil {
		return err
	}
	index := -1
	for i := range state.Enabled {
		if state.Enabled[i].Name == name {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("plugin %s is not enabled", name)
	}
	p := state.Enabled[index]
	if p.DeploymentMode != "" && p.DeploymentMode != "loose" && p.DeploymentMode != "vpk" {
		return fmt.Errorf("未知插件部署方式: %s", p.DeploymentMode)
	}
	game, err := os.OpenRoot(consts.GamePath)
	if err != nil {
		return err
	}
	defer game.Close()
	tx, err := newPluginFileTransaction(game)
	if err != nil {
		return err
	}
	defer tx.finish(&returnErr)
	preserveConfig := false
	if m := state.metadata(name); m != nil {
		preserveConfig = m.Type == "nut"
	}
	files := p.Files
	if p.DeploymentMode == "vpk" {
		m := state.metadata(name)
		if m == nil || p.VPKFile != "addons/manager_nut_"+m.ID+".vpk" {
			return fmt.Errorf("NUT 部署记录无效，未删除文件")
		}
		files = []string{p.VPKFile}
	}
	for _, filename := range files {
		file, err := cleanPluginPath(filename)
		if err != nil {
			return err
		}
		if preserveConfig && isLooseNUTFile(file) {
			continue
		}
		shared := false
		for _, other := range state.Enabled {
			if other.Name == name {
				continue
			}
			for _, f := range append(append([]string{}, other.Files...), other.ConfigFiles...) {
				if normalizeRelPath(f) == normalizeRelPath(file) {
					shared = true
				}
			}
		}
		if shared {
			continue
		}
		if _, err := game.Lstat(file); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := tx.remember(file); err != nil {
			return err
		}
		if err := game.Remove(file); err != nil {
			return err
		}
	}
	state.Enabled = append(state.Enabled[:index], state.Enabled[index+1:]...)
	return state.save()
}

func hashPluginFile(root *os.Root, p string) ([32]byte, error) {
	var empty [32]byte
	if err := checkPluginFile(root, p, false); err != nil {
		return empty, err
	}
	f, err := root.Open(p)
	if err != nil {
		return empty, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return empty, err
	}
	copy(empty[:], h.Sum(nil))
	return empty, nil
}

func checkManagedPluginConflicts(state *pluginState, meta PluginMetadata, files []string, source, game *os.Root) error {
	digests := map[string][32]byte{}
	for _, p := range files {
		digest, err := hashPluginFile(source, p)
		if err != nil {
			return err
		}
		digests[normalizeRelPath(p)] = digest
		if meta.Type == "nut" && !isLooseNUTFile(p) && normalizeRelPath(p) != "addoninfo.txt" {
			if _, err := game.Lstat(p); err == nil {
				current, err := hashPluginFile(game, p)
				if err != nil {
					return err
				}
				if isGlobalScriptEntryPath(p) || current != digest {
					return fmt.Errorf("散装文件与 NUT 冲突，请先处理: %s", p)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	for _, other := range state.Enabled {
		if meta.Type != "nut" && other.DeploymentMode != "vpk" {
			continue
		}
		otherDigests := map[string][32]byte{}
		if other.DeploymentMode == "vpk" {
			var err error
			otherDigests, err = readPluginVPKDigests(game, other.VPKFile)
			if err != nil {
				return fmt.Errorf("读取已启用插件 %s: %w", other.Name, err)
			}
		}
		for _, p := range append(append([]string{}, other.Files...), other.ConfigFiles...) {
			key := normalizeRelPath(p)
			if _, ok := digests[key]; !ok {
				continue
			}
			if _, err := game.Stat(p); errors.Is(err, os.ErrNotExist) {
				continue
			}
			digest, err := hashPluginFile(game, p)
			if err != nil {
				return err
			}
			// NUT preserves existing configs. Compare package defaults for two
			// NUT owners so edits to an intentionally shared config do not prevent
			// re-enabling a plugin or restoring a backup of both plugins.
			if om := state.metadata(other.Name); meta.Type == "nut" && isLooseNUTFile(p) && om != nil && om.Type == "nut" {
				if otherSource, openErr := openPluginRoot(other.Name); openErr == nil {
					original, hashErr := hashPluginFile(otherSource, p)
					otherSource.Close()
					if hashErr != nil && !errors.Is(hashErr, os.ErrNotExist) {
						return hashErr
					}
					if hashErr == nil {
						digest = original
					}
				}
			}
			otherDigests[key] = digest
		}
		for key, digest := range digests {
			if key == "addoninfo.txt" || (meta.Type == "nut" && other.DeploymentMode == "vpk" && isGlobalScriptEntryPath(key)) {
				continue
			}
			if old, ok := otherDigests[key]; ok && old != digest {
				return fmt.Errorf("插件 %s 与 %s 文件冲突: %s", meta.Name, other.Name, key)
			}
		}
	}
	return nil
}

func buildPluginVPK(source *os.Root, files []string, meta PluginMetadata, game *os.Root, destination string) error {
	dataPath := destination + ".data"
	data, err := game.OpenFile(dataPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer game.Remove(dataPath)
	defer data.Close()
	a := &vpk.Archive{Header: vpk.Header{Magic: vpk.Magic, Version: 1}}
	var offset uint64
	add := func(p string, r io.Reader) error {
		h := crc32.NewIEEE()
		n, err := io.Copy(io.MultiWriter(data, h), io.LimitReader(r, int64(math.MaxUint32-offset)+1))
		if err != nil {
			return err
		}
		if offset+uint64(n) > math.MaxUint32 {
			return fmt.Errorf("插件超过 VPK v1 单文件上限")
		}
		ext, dir := path.Ext(p), path.Dir(p)
		base := strings.TrimSuffix(path.Base(p), ext)
		if ext == "" || base == "" {
			base = path.Base(p)
			ext = " "
		} else {
			ext = ext[1:]
		}
		if dir == "." {
			dir = " "
		}
		a.Files = append(a.Files, vpk.File{Dir: dir, Base: base, Ext: ext, DirEntry: vpk.DirEntry{CRC: h.Sum32(), DataLocation: []vpk.DataChunk{{ArchiveIndex: 0x7fff, EntryOffset: uint32(offset), EntryLength: uint32(n)}}}})
		offset += uint64(n)
		return nil
	}
	hasInfo := false
	for _, p := range files {
		if isLooseNUTFile(p) {
			continue
		}
		if strings.EqualFold(p, "addoninfo.txt") {
			hasInfo = true
		}
		if err := checkPluginFile(source, p, false); err != nil {
			return err
		}
		f, err := source.Open(p)
		if err != nil {
			return err
		}
		err = add(p, f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if !hasInfo {
		info := fmt.Sprintf("\"AddonInfo\"\n{\n\"addonSteamAppID\" \"550\"\n\"addonTitle\" \"Manager NUT %s\"\n\"addonVersion\" \"1\"\n\"addonAuthor\" \"L4D2 Manager\"\n}\n", meta.ID)
		if err := add("addoninfo.txt", strings.NewReader(info)); err != nil {
			return err
		}
	}
	sort.Slice(a.Files, func(i, j int) bool {
		x, y := a.Files[i], a.Files[j]
		if x.Ext != y.Ext {
			return x.Ext < y.Ext
		}
		if x.Dir != y.Dir {
			return x.Dir < y.Dir
		}
		return x.Base < y.Base
	})
	if _, err := data.Seek(0, io.SeekStart); err != nil {
		return err
	}
	out, err := game.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if err = vpk.WriteDirectory(out, a); err == nil {
		_, err = io.Copy(out, data)
	}
	if err == nil {
		err = out.Sync()
	}
	return errors.Join(err, out.Close())
}

func readPluginVPKDigests(root *os.Root, filename string) (map[string][32]byte, error) {
	if err := checkPluginFile(root, filename, false); err != nil {
		return nil, err
	}
	f, err := root.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	a, err := vpk.ReadArchive(f)
	if err != nil {
		return nil, err
	}
	if a.Version != 1 {
		return nil, fmt.Errorf("受管 NUT VPK 必须为 v1")
	}
	result := map[string][32]byte{}
	for _, entry := range a.Files {
		key := normalizeRelPath(entry.Name())
		if _, err := cleanPluginPath(entry.Name()); err != nil {
			return nil, err
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("VPK 条目重复: %s", key)
		}
		h, crc := sha256.New(), crc32.NewIEEE()
		writer := io.MultiWriter(h, crc)
		_, _ = io.Copy(writer, bytes.NewReader(entry.Metadata))
		for _, chunk := range entry.DataLocation {
			if chunk.ArchiveIndex != 0x7fff {
				return nil, fmt.Errorf("不支持外部 VPK 分卷")
			}
			if _, err := io.CopyN(writer, io.NewSectionReader(f, 12+int64(a.TreeSize)+int64(chunk.EntryOffset), int64(chunk.EntryLength)), int64(chunk.EntryLength)); err != nil {
				return nil, err
			}
		}
		if crc.Sum32() != entry.CRC {
			return nil, fmt.Errorf("VPK 校验失败: %s", key)
		}
		var digest [32]byte
		copy(digest[:], h.Sum(nil))
		result[key] = digest
	}
	return result, nil
}
