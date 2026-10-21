package controller

import (
	"errors"
	"fmt"
	"io"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"os"
	"path/filepath"
	"strings"
)

type preparedMapImport struct {
	name       string
	path       string
	info       os.FileInfo
	inspection logic.MapVPKInspection
}

type mapImportBatch struct {
	policy     logic.MapImportPolicy
	trim       bool
	maps       []preparedMapImport
	violations []logic.MapImportViolation
	cleanups   []func()
}

func newMapImportBatch() *mapImportBatch {
	return &mapImportBatch{policy: logic.GetMapImportPolicy(), trim: logic.IsVPKTrimEnabled()}
}

func (batch *mapImportBatch) cleanup() {
	for i := len(batch.cleanups) - 1; i >= 0; i-- {
		batch.cleanups[i]()
	}
}

func (batch *mapImportBatch) add(sourcePath, cleanName string) error {
	if _, err := logic.NormalizeMapVPKName(cleanName); err != nil {
		return err
	}
	for _, candidate := range batch.maps {
		if strings.EqualFold(candidate.name, cleanName) {
			return fmt.Errorf("压缩包内地图清理后重名: %s", cleanName)
		}
	}
	// Own a private candidate on the addons filesystem. No final .vpk name or
	// installed-map record is exposed until every archive member has passed.
	candidatePath, cleanup, err := createVPKProcessingTempFile(cleanName)
	if err != nil {
		return err
	}
	batch.cleanups = append(batch.cleanups, cleanup)
	if err := moveFile(sourcePath, candidatePath); err != nil {
		return fmt.Errorf("暂存地图 %s 失败: %w", cleanName, err)
	}
	if batch.trim {
		trimmed, trimCleanup, err := logic.TrimVPKForServer(candidatePath)
		if err != nil {
			if !logic.IsVPKTrimUnsupported(err) {
				return fmt.Errorf("精简地图 %s 失败: %w", cleanName, err)
			}
		} else {
			batch.cleanups = append(batch.cleanups, trimCleanup)
			candidatePath = trimmed
		}
	}
	before, err := os.Stat(candidatePath)
	if err != nil {
		return err
	}
	inspection := logic.InspectMapVPK(candidatePath)
	after, err := os.Stat(candidatePath)
	if err != nil {
		return err
	}
	if !sameMapImportVersion(before, after) {
		return fmt.Errorf("地图 %s 在检测期间发生变化", cleanName)
	}
	batch.maps = append(batch.maps, preparedMapImport{name: cleanName, path: candidatePath, info: after, inspection: inspection})
	batch.violations = append(batch.violations, logic.EvaluateMapImport(cleanName, inspection, batch.policy)...)
	return nil
}

func sameMapImportVersion(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

// Hooks keep failure-path tests deterministic without depending on disk quotas.
var installMapImportFile = linkOrCopyMapImportFile
var publishMapImportList = atomicWriteMapImportList

func (batch *mapImportBatch) commit() (names []string, returnErr error) {
	if len(batch.violations) > 0 {
		return nil, &logic.MapImportRejectedError{Violations: batch.violations}
	}
	if len(batch.maps) == 0 {
		return nil, errors.New("未找到 VPK 文件")
	}
	// Shared with remove/rename/trim/script editing, and held through rollback.
	mutex.Lock()
	defer mutex.Unlock()
	previousList, err := os.ReadFile(consts.MapListFilePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取地图记录失败: %w", err)
	}
	mode := os.FileMode(0644)
	if info, err := os.Stat(consts.MapListFilePath); err == nil {
		mode = info.Mode().Perm()
	}
	for _, candidate := range batch.maps {
		if err := checkMapExists(candidate.name); err != nil {
			return nil, err
		}
		if _, err := os.Lstat(filepath.Join(consts.AddonsBasePath, candidate.name)); err == nil {
			return nil, fmt.Errorf("地图 %s 已经存在", candidate.name)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		info, err := os.Stat(candidate.path)
		if err != nil {
			return nil, err
		}
		if !sameMapImportVersion(candidate.info, info) {
			return nil, fmt.Errorf("地图 %s 在检测后发生变化", candidate.name)
		}
	}
	installed := make([]string, 0, len(batch.maps))
	defer func() {
		if returnErr == nil {
			return
		}
		for _, path := range installed {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				returnErr = errors.Join(returnErr, fmt.Errorf("回滚新增地图 %s 失败: %w", filepath.Base(path), err))
			}
		}
	}()
	entries := make([]logic.PreparedMapVPKInspection, 0, len(batch.maps))
	content := string(previousList)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	for _, candidate := range batch.maps {
		dest := filepath.Join(consts.AddonsBasePath, candidate.name)
		if err := installMapImportFile(candidate.path, dest); err != nil {
			return nil, fmt.Errorf("安装地图 %s 失败: %w", candidate.name, err)
		}
		installed = append(installed, dest)
		info, err := os.Stat(dest)
		if err != nil {
			return nil, err
		}
		entries = append(entries, logic.PreparedMapVPKInspection{Name: candidate.name, Info: info, Inspection: candidate.inspection})
		content += candidate.name + "\n"
		names = append(names, candidate.name)
	}
	if err := logic.CommitMapVPKInspections(entries, func() error {
		return publishMapImportList(consts.MapListFilePath, []byte(content), mode)
	}); err != nil {
		return nil, fmt.Errorf("记录地图失败: %w", err)
	}
	return names, nil
}

// Hard-linking publishes a complete same-filesystem candidate without replacing
// an existing file. Filesystems without hard links use exclusive creation.
func linkOrCopyMapImportFile(source, destination string) (returnErr error) {
	if err := os.Link(source, destination); err == nil {
		return nil
	} else if os.IsExist(err) {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
		if returnErr != nil {
			returnErr = errors.Join(returnErr, os.Remove(destination))
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	return out.Close()
}

func atomicWriteMapImportList(path string, content []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".maplist-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = temp.Close(); _ = os.Remove(temp.Name()) }()
	if err := temp.Chmod(mode); err != nil {
		return err
	}
	if _, err := temp.Write(content); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}
