package controller

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/axgle/mahonia"
	"github.com/bodgit/sevenzip"
	"github.com/google/uuid"
	"github.com/nwaples/rardecode"
)

var chineseDecoder = mahonia.NewDecoder("gbk")

// checkMapExists 检查地图文件是否已存在
func checkMapExists(filename string) error {
	_, statErr := os.Stat(consts.MapListFilePath)
	if !os.IsNotExist(statErr) {
		maps, readErr := os.ReadFile(consts.MapListFilePath)
		if readErr != nil {
			return errors.New("获取地图记录文件失败")
		}
		for _, mapName := range strings.Split(string(maps), "\n") {
			if mapName == filename {
				return errors.New("地图 " + filename + " 已经存在")
			}
		}
	}
	return nil
}

// extractFile 从zip文件中解压单个文件
func extractFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}

// recordMap 将地图文件名记录到maplist.txt
func recordMap(filename string) error {
	mutex.Lock()
	defer mutex.Unlock()

	list, openErr := os.OpenFile(consts.MapListFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if openErr != nil {
		return errors.New("获取地图记录文件句柄失败")
	}
	defer list.Close()

	if _, err := list.WriteString(filename + "\n"); err != nil {
		return errors.New("写入地图记录失败")
	}
	return nil
}

func createVPKProcessingTempFile(cleanName string) (string, func(), error) {
	tempRoot := filepath.Join(consts.AddonsBasePath, logic.VPKTrimTempDirName)
	if err := os.MkdirAll(tempRoot, 0755); err != nil {
		return "", nil, fmt.Errorf("创建VPK处理临时目录失败: %w", err)
	}

	tempDir, err := os.MkdirTemp(tempRoot, "incoming-")
	if err != nil {
		return "", nil, fmt.Errorf("创建VPK处理任务目录失败: %w", err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}
	return filepath.Join(tempDir, cleanName), cleanup, nil
}

func finalizeVpkFile(sourcePath, cleanName string) error {
	if err := checkMapExists(cleanName); err != nil {
		return err
	}

	destPath := filepath.Join(consts.AddonsBasePath, cleanName)
	if _, err := os.Stat(destPath); err == nil {
		return errors.New("地图 " + cleanName + " 已经存在")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查地图文件失败: %w", err)
	}

	saveSourcePath := sourcePath
	var trimCleanup func()
	if logic.IsVPKTrimEnabled() {
		trimmedPath, cleanup, err := logic.TrimVPKForServer(sourcePath)
		if err != nil {
			if !logic.IsVPKTrimUnsupported(err) {
				return fmt.Errorf("精简VPK失败: %w", err)
			}
		} else {
			saveSourcePath = trimmedPath
			trimCleanup = cleanup
			defer trimCleanup()
		}
	}

	tempDestPath := filepath.Join(consts.AddonsBasePath, "."+uuid.NewString()+"."+cleanName+".tmp")
	_ = os.Remove(tempDestPath)
	if err := moveFile(saveSourcePath, tempDestPath); err != nil {
		_ = os.Remove(tempDestPath)
		return fmt.Errorf("保存地图临时文件失败: %w", err)
	}

	saved := false
	defer func() {
		if !saved {
			os.Remove(tempDestPath)
		}
	}()

	if err := os.Rename(tempDestPath, destPath); err != nil {
		return fmt.Errorf("保存地图失败: %w", err)
	}
	saved = true

	recorded := false
	defer func() {
		if !recorded {
			os.Remove(destPath)
		}
	}()

	if err := recordMap(cleanName); err != nil {
		return fmt.Errorf("记录地图失败: %w", err)
	}
	recorded = true

	if saveSourcePath != sourcePath {
		_ = os.Remove(sourcePath)
	}
	return nil
}

// sanitizeFilename 清理文件名中的空格和特殊符号，替换为下划线
func sanitizeFilename(filename string) string {
	filename = strings.TrimSpace(strings.ReplaceAll(filename, "\x00", ""))
	filename = strings.ReplaceAll(filename, "\\", "/")
	filename = filepath.Base(filename)

	// 分离文件名和扩展名
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)
	ext = sanitizeFileExtension(ext)

	// 使用正则表达式匹配需要替换的字符
	// 匹配空格、特殊符号等，但保留中文字符、英文字母、数字、连字符、下划线
	reg := regexp.MustCompile(`[^\p{L}\p{N}\-_]+`)
	cleanName := reg.ReplaceAllString(nameWithoutExt, "_")
	cleanName = strings.Trim(cleanName, "._-")

	// 如果存在myl4d2addons_前缀则去除
	cleanName = strings.TrimPrefix(cleanName, "myl4d2addons_")
	cleanName = strings.Trim(cleanName, "._-")
	if cleanName == "" {
		cleanName = "downloaded_file"
	}
	if isWindowsReservedFilename(cleanName) {
		cleanName += "_"
	}
	cleanName = truncateFilenamePart(cleanName, 180)

	return cleanName + ext
}

func sanitizeFileExtension(ext string) string {
	if ext == "" {
		return ""
	}
	extName := strings.TrimPrefix(strings.ToLower(ext), ".")
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	extName = reg.ReplaceAllString(extName, "")
	if extName == "" {
		return ""
	}
	return "." + truncateFilenamePart(extName, 16)
}

func truncateFilenamePart(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func isWindowsReservedFilename(name string) bool {
	switch strings.ToUpper(name) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
}

// ProcessFile 处理文件（vpk或zip或rar或7z），统一的文件处理入口
func ProcessFile(filePath string) ([]string, error) {
	fileName := filepath.Base(filePath)

	// 检查文件类型
	vpkReg := regexp.MustCompile(`(?i)\.vpk$`)
	zipReg := regexp.MustCompile(`(?i)\.zip$`)
	rarReg := regexp.MustCompile(`(?i)\.rar$`)
	sevenZipReg := regexp.MustCompile(`(?i)\.7z$`)

	if !vpkReg.MatchString(fileName) && !zipReg.MatchString(fileName) && !rarReg.MatchString(fileName) && !sevenZipReg.MatchString(fileName) {
		return nil, errors.New("不支持的文件类型，只支持vpk, zip, rar, 7z文件")
	}

	// 处理zip文件 - 解压并提取vpk文件
	if zipReg.MatchString(fileName) {
		return ProcessZipFile(filePath)
	}

	// 处理rar文件
	if rarReg.MatchString(fileName) {
		return ProcessRarFile(filePath)
	}

	// 处理7z文件
	if sevenZipReg.MatchString(fileName) {
		return Process7zFile(filePath)
	}

	// 处理vpk文件 - 直接移动到目标目录
	if vpkReg.MatchString(fileName) {
		return ProcessVpkFile(filePath)
	}

	return nil, nil
}

// ProcessZipFile 处理zip文件，解压并提取vpk文件
func ProcessZipFile(zipPath string) ([]string, error) {
	// 打开zip文件
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("打开zip文件失败: %v", err)
	}
	defer reader.Close()

	vpkReg := regexp.MustCompile(`(?i)\.vpk$`)
	var extractedFiles []string

	// 解压vpk文件
	for _, f := range reader.File {
		name := f.Name
		if f.NonUTF8 {
			name = chineseDecoder.ConvertString(f.Name)
		}
		if vpkReg.MatchString(name) {
			// 清理文件名
			cleanName := sanitizeFilename(filepath.Base(name))

			tempPath, cleanup, err := createVPKProcessingTempFile(cleanName)
			if err != nil {
				return nil, err
			}

			if err := extractFile(f, tempPath); err != nil {
				cleanup()
				return nil, fmt.Errorf("解压文件失败: %v", err)
			}

			if err := finalizeVpkFile(tempPath, cleanName); err != nil {
				cleanup()
				return nil, err
			}
			cleanup()

			extractedFiles = append(extractedFiles, cleanName)
		}
	}

	if len(extractedFiles) == 0 {
		return nil, errors.New("zip文件中未找到vpk文件")
	}

	return extractedFiles, nil
}

// ProcessRarFile 处理rar文件
func ProcessRarFile(rarPath string) ([]string, error) {
	// 打开rar文件
	file, err := os.Open(rarPath)
	if err != nil {
		return nil, fmt.Errorf("打开rar文件失败: %v", err)
	}
	defer file.Close()

	rr, err := rardecode.NewReader(file, "")
	if err != nil {
		return nil, fmt.Errorf("创建rar读取器失败: %v", err)
	}

	vpkReg := regexp.MustCompile(`(?i)\.vpk$`)
	var extractedFiles []string

	for {
		header, err := rr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取rar内容失败: %v", err)
		}

		if header.IsDir {
			continue
		}

		if vpkReg.MatchString(header.Name) {
			// 清理文件名
			cleanName := sanitizeFilename(filepath.Base(header.Name))

			tempPath, cleanup, err := createVPKProcessingTempFile(cleanName)
			if err != nil {
				return nil, err
			}

			if err := writeReaderToFile(rr, tempPath, 0666); err != nil {
				cleanup()
				return nil, fmt.Errorf("写入文件失败: %v", err)
			}

			if err := finalizeVpkFile(tempPath, cleanName); err != nil {
				cleanup()
				return nil, err
			}
			cleanup()

			extractedFiles = append(extractedFiles, cleanName)
		}
	}

	if len(extractedFiles) == 0 {
		return nil, errors.New("rar文件中未找到vpk文件")
	}

	return extractedFiles, nil
}

// Process7zFile 处理7z文件
func Process7zFile(sevenZipPath string) ([]string, error) {
	r, err := sevenzip.OpenReader(sevenZipPath)
	if err != nil {
		return nil, fmt.Errorf("打开7z文件失败: %v", err)
	}
	defer r.Close()

	vpkReg := regexp.MustCompile(`(?i)\.vpk$`)
	var extractedFiles []string

	for _, f := range r.File {
		if vpkReg.MatchString(f.Name) {
			// 清理文件名
			cleanName := sanitizeFilename(filepath.Base(f.Name))

			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("打开7z内部文件失败: %v", err)
			}

			tempPath, cleanup, err := createVPKProcessingTempFile(cleanName)
			if err != nil {
				rc.Close()
				return nil, err
			}

			if err := writeReaderToFile(rc, tempPath, 0666); err != nil {
				cleanup()
				rc.Close()
				return nil, fmt.Errorf("写入文件失败: %v", err)
			}
			rc.Close()

			if err := finalizeVpkFile(tempPath, cleanName); err != nil {
				cleanup()
				return nil, err
			}
			cleanup()

			extractedFiles = append(extractedFiles, cleanName)
		}
	}

	if len(extractedFiles) == 0 {
		return nil, errors.New("7z文件中未找到vpk文件")
	}

	return extractedFiles, nil
}

// ProcessVpkFile 处理vpk文件，直接移动到目标目录
func ProcessVpkFile(vpkPath string) ([]string, error) {
	fileName := filepath.Base(vpkPath)
	// 移除temp_前缀（如果存在）
	fileName = strings.TrimPrefix(fileName, "temp_")
	// 移除merged_前缀（如果存在）
	fileName = strings.TrimPrefix(fileName, "merged_")
	cleanName := sanitizeFilename(fileName)

	if err := finalizeVpkFile(vpkPath, cleanName); err != nil {
		return nil, err
	}

	return []string{cleanName}, nil
}

func writeReaderToFile(reader io.Reader, dest string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, reader)
	return err
}

func moveFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	if err := os.Rename(src, dest); err == nil {
		return nil
	}

	if err := copyFile(src, dest); err != nil {
		return err
	}
	return os.Remove(src)
}

// copyFile 复制文件的工具函数
func copyFile(src, dest string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
