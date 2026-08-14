package logic

import (
	"bytes"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/pkg/valve/vpk"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const VPKScriptEditTempDirName = ".vpk_script_edit_temp"

var (
	ErrMapRevisionConflict           = errors.New("地图文件已发生变化")
	ErrGlobalScriptPathInvalid       = errors.New("全局脚本路径无效")
	ErrGlobalScriptNotFound          = errors.New("全局脚本不存在")
	ErrGlobalScriptNotEditable       = errors.New("全局脚本无法安全编辑")
	ErrGlobalScriptContentTooLarge   = errors.New("全局脚本内容超过 512 KiB")
	ErrGlobalScriptRepackUnsupported = errors.New("当前仅支持 VPK v1 单文件地图重打包")
	ErrGlobalScriptDuplicateEntry    = errors.New("VPK 中存在重复的全局脚本条目")
)

type GlobalScriptUpdateResult struct {
	Map      string              `json:"map"`
	Revision string              `json:"revision"`
	Script   GlobalScriptContent `json:"script"`
}

func CleanVPKScriptEditTemp() {
	os.RemoveAll(filepath.Join(consts.AddonsBasePath, VPKScriptEditTempDirName))
}

func UpdateMapGlobalScript(mapName, scriptPath, content, encoding, expectedRevision string) (GlobalScriptUpdateResult, error) {
	var result GlobalScriptUpdateResult

	mapName, err := NormalizeMapVPKName(mapName)
	if err != nil {
		return result, err
	}
	scriptPath = normalizeVPKEntryPath(scriptPath)
	if !isGlobalScriptEntryPath(scriptPath) {
		return result, ErrGlobalScriptPathInvalid
	}
	if strings.TrimSpace(expectedRevision) == "" {
		return result, ErrMapRevisionConflict
	}
	if strings.ContainsRune(content, '\x00') || !utf8.ValidString(content) {
		return result, fmt.Errorf("%w: 脚本正文必须是有效文本且不能包含 NUL 字符", ErrGlobalScriptNotEditable)
	}

	scripts, currentRevision, err := GetMapGlobalScriptContentsWithRevision(mapName)
	if err != nil {
		return result, err
	}
	if expectedRevision != currentRevision {
		return result, ErrMapRevisionConflict
	}

	var current *GlobalScriptContent
	for index := range scripts {
		if normalizeVPKEntryPath(scripts[index].Path) == scriptPath {
			current = &scripts[index]
			break
		}
	}
	if current == nil {
		return result, ErrGlobalScriptNotFound
	}
	if current.Error != "" || current.Truncated || current.Encoding == "unknown" {
		return result, ErrGlobalScriptNotEditable
	}

	encoding = strings.ToLower(strings.TrimSpace(encoding))
	if encoding != current.Encoding {
		return result, fmt.Errorf("%w: 脚本编码已发生变化，请重新打开后再试", ErrMapRevisionConflict)
	}
	replacement, err := encodeGlobalScriptContent(content, encoding)
	if err != nil {
		return result, err
	}
	if len(replacement) > maxGlobalScriptContentBytes {
		return result, ErrGlobalScriptContentTooLarge
	}

	vpkPath := filepath.Join(consts.AddonsBasePath, mapName)
	beforeInfo, err := os.Stat(vpkPath)
	if err != nil {
		return result, err
	}
	if mapVPKRevision(beforeInfo) != expectedRevision {
		return result, ErrMapRevisionConflict
	}

	tempRoot := filepath.Join(consts.AddonsBasePath, VPKScriptEditTempDirName)
	if err := os.MkdirAll(tempRoot, 0755); err != nil {
		return result, fmt.Errorf("创建脚本编辑临时目录失败: %w", err)
	}
	tempDir, err := os.MkdirTemp(tempRoot, "edit-")
	if err != nil {
		return result, fmt.Errorf("创建脚本编辑任务目录失败: %w", err)
	}
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.RemoveAll(tempDir)
		}
	}()

	repackedPath := filepath.Join(tempDir, "edited.vpk")
	if err := repackSingleFileVPKEntry(vpkPath, repackedPath, scriptPath, replacement); err != nil {
		return result, err
	}
	if err := validateRepackedGlobalScript(repackedPath, scriptPath, replacement); err != nil {
		return result, fmt.Errorf("校验重打包 VPK 失败: %w", err)
	}
	if err := os.Chmod(repackedPath, beforeInfo.Mode().Perm()); err != nil {
		return result, fmt.Errorf("保留地图文件权限失败: %w", err)
	}

	latestInfo, err := os.Stat(vpkPath)
	if err != nil {
		return result, err
	}
	if mapVPKRevision(latestInfo) != expectedRevision {
		return result, ErrMapRevisionConflict
	}

	backupPath := filepath.Join(consts.AddonsBasePath, "."+uuid.NewString()+".script-edit.bak")
	if err := os.Rename(vpkPath, backupPath); err != nil {
		return result, fmt.Errorf("备份原地图文件失败: %w", err)
	}
	backupActive := true
	defer func() {
		if backupActive {
			log.Printf("地图脚本编辑保留了恢复文件: %s", backupPath)
		}
	}()

	if err := os.Rename(repackedPath, vpkPath); err != nil {
		if rollbackErr := os.Rename(backupPath, vpkPath); rollbackErr != nil {
			return result, fmt.Errorf("写入重打包地图失败: %w；原文件回滚失败: %v", err, rollbackErr)
		}
		backupActive = false
		return result, fmt.Errorf("写入重打包地图失败，已恢复原文件: %w", err)
	}

	if err := InspectAndStoreMapVPK(mapName, vpkPath); err != nil {
		failedNewPath := filepath.Join(tempDir, "failed-new.vpk")
		if moveErr := os.Rename(vpkPath, failedNewPath); moveErr != nil {
			cleanupTemp = false
			return result, fmt.Errorf("重新检测地图失败: %w；保留新文件和恢复文件时失败: %v", err, moveErr)
		}
		if rollbackErr := os.Rename(backupPath, vpkPath); rollbackErr != nil {
			if restoreErr := os.Rename(failedNewPath, vpkPath); restoreErr != nil {
				cleanupTemp = false
				return result, fmt.Errorf("重新检测地图失败: %w；原文件回滚失败: %v；恢复新文件失败: %v", err, rollbackErr, restoreErr)
			}
			return result, fmt.Errorf("重新检测地图失败: %w；原文件回滚失败，新文件已恢复: %v", err, rollbackErr)
		}
		backupActive = false
		return result, fmt.Errorf("重新检测地图失败，已恢复原文件: %w", err)
	}

	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		log.Printf("清理地图脚本编辑恢复文件失败（%s）: %v", backupPath, err)
	} else {
		backupActive = false
	}

	updatedInfo, err := os.Stat(vpkPath)
	if err != nil {
		return result, err
	}
	return GlobalScriptUpdateResult{
		Map:      mapName,
		Revision: mapVPKRevision(updatedInfo),
		Script: GlobalScriptContent{
			Path:      scriptPath,
			Size:      int64(len(replacement)),
			Encoding:  encoding,
			Content:   content,
			Truncated: false,
		},
	}, nil
}

func encodeGlobalScriptContent(content, encoding string) ([]byte, error) {
	switch encoding {
	case "utf-8":
		return []byte(content), nil
	case "gbk":
		encoded, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(content))
		if err != nil {
			return nil, fmt.Errorf("脚本包含 GBK 无法表示的字符: %w", err)
		}
		return encoded, nil
	default:
		return nil, ErrGlobalScriptNotEditable
	}
}

func repackSingleFileVPKEntry(sourcePath, destinationPath, targetPath string, replacement []byte) error {
	opener := vpk.Single(sourcePath)
	defer opener.Close()

	archive, err := opener.ReadArchive()
	if err != nil {
		return fmt.Errorf("解析 VPK 失败: %w", err)
	}
	if archive.Version != rawVPKVersion1 {
		return fmt.Errorf("%w: version %d", ErrGlobalScriptRepackUnsupported, archive.Version)
	}

	targetIndex := -1
	for index := range archive.Files {
		archiveFile := &archive.Files[index]
		if normalizeVPKEntryPath(archiveFile.Name()) == targetPath {
			if targetIndex != -1 {
				return ErrGlobalScriptDuplicateEntry
			}
			targetIndex = index
		}
		for _, chunk := range archiveFile.DataLocation {
			if chunk.ArchiveIndex != rawVPKSelfArchive {
				return fmt.Errorf("%w: external archive %d", ErrGlobalScriptRepackUnsupported, chunk.ArchiveIndex)
			}
		}
	}
	if targetIndex == -1 {
		return ErrGlobalScriptNotFound
	}

	dataPath := destinationPath + ".data"
	_ = os.Remove(dataPath)
	defer os.Remove(dataPath)
	dataFile, err := os.OpenFile(dataPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("创建 VPK 数据临时文件失败: %w", err)
	}
	defer dataFile.Close()

	var dataOffset uint64
	for index := range archive.Files {
		archiveFile := &archive.Files[index]
		contentSize := int64(archiveFile.Size())
		if index == targetIndex {
			contentSize = int64(len(replacement))
		}
		if contentSize < 0 || uint64(contentSize) > math.MaxUint32 || dataOffset+uint64(contentSize) > math.MaxUint32 {
			return fmt.Errorf("%w: VPK 数据超过单文件偏移上限", ErrGlobalScriptRepackUnsupported)
		}

		if contentSize > 0 {
			if index == targetIndex {
				if _, err := io.Copy(dataFile, bytes.NewReader(replacement)); err != nil {
					return fmt.Errorf("写入脚本内容失败: %w", err)
				}
			} else {
				reader := io.NewSectionReader(archiveFile.OpenReaderAt(opener), 0, contentSize)
				if _, err := io.CopyN(dataFile, reader, contentSize); err != nil {
					return fmt.Errorf("复制 VPK 条目 %s 失败: %w", archiveFile.Name(), err)
				}
			}
		}

		if index == targetIndex {
			archiveFile.CRC = crc32.ChecksumIEEE(replacement)
		}
		archiveFile.Metadata = nil
		archiveFile.MetadataBytes = 0
		archiveFile.DataLocation = []vpk.DataChunk{{
			ArchiveIndex: rawVPKSelfArchive,
			EntryOffset:  uint32(dataOffset),
			EntryLength:  uint32(contentSize),
		}}
		dataOffset += uint64(contentSize)
	}

	if err := dataFile.Sync(); err != nil {
		return fmt.Errorf("同步 VPK 数据临时文件失败: %w", err)
	}
	if _, err := dataFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("读取 VPK 数据临时文件失败: %w", err)
	}

	out, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("创建重打包 VPK 失败: %w", err)
	}
	if err := vpk.WriteDirectory(out, archive); err != nil {
		_ = out.Close()
		return fmt.Errorf("写入 VPK 目录失败: %w", err)
	}
	if _, err := io.Copy(out, dataFile); err != nil {
		_ = out.Close()
		return fmt.Errorf("写入 VPK 数据失败: %w", err)
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return fmt.Errorf("同步重打包 VPK 失败: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("关闭重打包 VPK 失败: %w", err)
	}
	return nil
}

func validateRepackedGlobalScript(vpkPath, targetPath string, expected []byte) error {
	opener := vpk.Single(vpkPath)
	defer opener.Close()
	archive, err := opener.ReadArchive()
	if err != nil {
		return err
	}

	matched := 0
	for index := range archive.Files {
		archiveFile := &archive.Files[index]
		if normalizeVPKEntryPath(archiveFile.Name()) != targetPath {
			continue
		}
		matched++
		actual, readErr := archiveFile.Bytes(opener)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(actual, expected) {
			return errors.New("重打包后的脚本内容不一致")
		}
	}
	if matched == 0 {
		return ErrGlobalScriptNotFound
	}
	if matched > 1 {
		return ErrGlobalScriptDuplicateEntry
	}
	return nil
}
