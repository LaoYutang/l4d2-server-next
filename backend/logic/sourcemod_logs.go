package logic

import (
	"errors"
	"fmt"
	"l4d2-manager-next/consts"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	SourceModLogCategoryRun    = "L"
	SourceModLogCategoryErrors = "errors"
	SourceModLogCategoryOther  = "other"

	SourceModLogRecentProtectionWindow = 10 * time.Minute
)

var (
	ErrInvalidSourceModLogName          = errors.New("invalid SourceMod log filename")
	ErrInvalidSourceModLogCleanupFilter = errors.New("invalid SourceMod log cleanup filter")

	sourceModRunLogPattern   = regexp.MustCompile(`^L(\d{8})\.log$`)
	sourceModErrorLogPattern = regexp.MustCompile(`^errors_(\d{8})\.log$`)
)

type SourceModLogFile struct {
	Name            string    `json:"name"`
	Date            string    `json:"date"`
	Size            int64     `json:"size"`
	Category        string    `json:"category"`
	ModifiedAt      time.Time `json:"modified_at"`
	CleanupAt       time.Time `json:"cleanup_at"`
	Deletable       bool      `json:"deletable"`
	ProtectedReason string    `json:"protected_reason,omitempty"`
	Version         string    `json:"version"`
}

type SourceModLogScanResult struct {
	Installed bool               `json:"installed"`
	Files     []SourceModLogFile `json:"files"`
}

type SourceModLogCleanupFilter struct {
	Categories    []string `json:"categories"`
	RetentionDays int      `json:"retention_days"`
}

type SourceModLogCleanupPreview struct {
	Installed  bool               `json:"installed"`
	Candidates []SourceModLogFile `json:"candidates"`
	Protected  []SourceModLogFile `json:"protected"`
	Count      int                `json:"count"`
	TotalSize  int64              `json:"total_size"`
}

type SourceModLogDeleteTarget struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type SourceModLogDeleteIssue struct {
	Name    string `json:"name"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type SourceModLogDeleteResult struct {
	Deleted    []string                  `json:"deleted"`
	Skipped    []SourceModLogDeleteIssue `json:"skipped"`
	Failed     []SourceModLogDeleteIssue `json:"failed"`
	FreedBytes int64                     `json:"freed_bytes"`
}

func SourceModLogsDir() string {
	return filepath.Join(consts.GamePath, "addons", "sourcemod", "logs")
}

func ValidateSourceModLogName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00:") {
		return ErrInvalidSourceModLogName
	}
	if filepath.IsAbs(name) || filepath.VolumeName(name) != "" || filepath.Base(name) != name {
		return ErrInvalidSourceModLogName
	}
	if !strings.EqualFold(filepath.Ext(name), ".log") {
		return ErrInvalidSourceModLogName
	}
	return nil
}

func ScanSourceModLogs(now time.Time) (SourceModLogScanResult, error) {
	sourceModPath := filepath.Join(consts.GamePath, "addons", "sourcemod")
	info, err := os.Stat(sourceModPath)
	if err != nil {
		if os.IsNotExist(err) {
			return SourceModLogScanResult{Installed: false, Files: []SourceModLogFile{}}, nil
		}
		return SourceModLogScanResult{}, fmt.Errorf("stat SourceMod directory: %w", err)
	}
	if !info.IsDir() {
		return SourceModLogScanResult{}, fmt.Errorf("SourceMod path is not a directory")
	}

	files, err := scanSourceModLogsDir(SourceModLogsDir(), now)
	if err != nil {
		if os.IsNotExist(err) {
			return SourceModLogScanResult{Installed: true, Files: []SourceModLogFile{}}, nil
		}
		return SourceModLogScanResult{}, err
	}
	return SourceModLogScanResult{Installed: true, Files: files}, nil
}

func scanSourceModLogsDir(logsDir string, now time.Time) ([]SourceModLogFile, error) {
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return nil, fmt.Errorf("read SourceMod log directory: %w", err)
	}

	files := make([]SourceModLogFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".log") {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, buildSourceModLogFile(entry.Name(), info, now))
	}

	sortSourceModLogFiles(files)
	return files, nil
}

func buildSourceModLogFile(name string, info os.FileInfo, now time.Time) SourceModLogFile {
	location := now.Location()
	modifiedAt := info.ModTime()
	category := SourceModLogCategoryOther
	date := modifiedAt.In(location).Format("20060102")
	cleanupAt := modifiedAt
	var filenameDate time.Time

	if matches := sourceModRunLogPattern.FindStringSubmatch(name); matches != nil {
		if parsed, err := time.ParseInLocation("20060102", matches[1], location); err == nil {
			category = SourceModLogCategoryRun
			date = matches[1]
			filenameDate = parsed
		}
	} else if matches := sourceModErrorLogPattern.FindStringSubmatch(name); matches != nil {
		if parsed, err := time.ParseInLocation("20060102", matches[1], location); err == nil {
			category = SourceModLogCategoryErrors
			date = matches[1]
			filenameDate = parsed
		}
	}

	protectedReason := ""
	if !filenameDate.IsZero() {
		filenameDateEnd := filenameDate.AddDate(0, 0, 1).Add(-time.Nanosecond)
		if filenameDateEnd.After(cleanupAt) {
			cleanupAt = filenameDateEnd
		}
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
		if !filenameDate.Before(today) {
			protectedReason = "今天或未来日期的标准日志"
		}
	}
	if protectedReason == "" && !modifiedAt.Before(now.Add(-SourceModLogRecentProtectionWindow)) {
		protectedReason = "最近 10 分钟内仍有更新"
	}

	return SourceModLogFile{
		Name:            name,
		Date:            date,
		Size:            info.Size(),
		Category:        category,
		ModifiedAt:      modifiedAt,
		CleanupAt:       cleanupAt,
		Deletable:       protectedReason == "",
		ProtectedReason: protectedReason,
		Version:         sourceModLogVersion(info),
	}
}

func sourceModLogVersion(info os.FileInfo) string {
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
}

func sortSourceModLogFiles(files []SourceModLogFile) {
	categoryOrder := map[string]int{
		SourceModLogCategoryRun:    0,
		SourceModLogCategoryErrors: 1,
		SourceModLogCategoryOther:  2,
	}
	sort.Slice(files, func(i, j int) bool {
		if categoryOrder[files[i].Category] != categoryOrder[files[j].Category] {
			return categoryOrder[files[i].Category] < categoryOrder[files[j].Category]
		}
		if !files[i].CleanupAt.Equal(files[j].CleanupAt) {
			return files[i].CleanupAt.After(files[j].CleanupAt)
		}
		return files[i].Name < files[j].Name
	})
}

func PreviewSourceModLogCleanup(now time.Time, filter SourceModLogCleanupFilter) (SourceModLogCleanupPreview, error) {
	selectedCategories, err := validateSourceModLogCleanupFilter(filter)
	if err != nil {
		return SourceModLogCleanupPreview{}, err
	}

	scan, err := ScanSourceModLogs(now)
	if err != nil {
		return SourceModLogCleanupPreview{}, err
	}
	preview := SourceModLogCleanupPreview{
		Installed:  scan.Installed,
		Candidates: []SourceModLogFile{},
		Protected:  []SourceModLogFile{},
	}
	if !scan.Installed {
		return preview, nil
	}

	cutoff := now
	if filter.RetentionDays > 0 {
		cutoff = now.Add(-time.Duration(filter.RetentionDays) * 24 * time.Hour)
	}
	for _, file := range scan.Files {
		if _, selected := selectedCategories[file.Category]; !selected {
			continue
		}
		if !file.Deletable {
			preview.Protected = append(preview.Protected, file)
			continue
		}
		if filter.RetentionDays > 0 && !file.CleanupAt.Before(cutoff) {
			continue
		}
		preview.Candidates = append(preview.Candidates, file)
		preview.TotalSize += file.Size
	}
	preview.Count = len(preview.Candidates)
	return preview, nil
}

func validateSourceModLogCleanupFilter(filter SourceModLogCleanupFilter) (map[string]struct{}, error) {
	if filter.RetentionDays != 0 && filter.RetentionDays != 7 && filter.RetentionDays != 30 && filter.RetentionDays != 90 {
		return nil, fmt.Errorf("%w: retention_days must be one of 0, 7, 30, or 90", ErrInvalidSourceModLogCleanupFilter)
	}
	if len(filter.Categories) == 0 {
		return nil, fmt.Errorf("%w: select at least one category", ErrInvalidSourceModLogCleanupFilter)
	}

	selected := make(map[string]struct{}, len(filter.Categories))
	for _, category := range filter.Categories {
		switch category {
		case SourceModLogCategoryRun, SourceModLogCategoryErrors, SourceModLogCategoryOther:
			selected[category] = struct{}{}
		default:
			return nil, fmt.Errorf("%w: unknown category %q", ErrInvalidSourceModLogCleanupFilter, category)
		}
	}
	return selected, nil
}

func DeleteSourceModLogs(now time.Time, targets []SourceModLogDeleteTarget) (SourceModLogDeleteResult, error) {
	return deleteSourceModLogs(now, targets, func(root *os.Root, name string) error {
		return root.Remove(name)
	})
}

func deleteSourceModLogs(
	now time.Time,
	targets []SourceModLogDeleteTarget,
	removeFile func(root *os.Root, name string) error,
) (SourceModLogDeleteResult, error) {
	result := SourceModLogDeleteResult{
		Deleted: []string{},
		Skipped: []SourceModLogDeleteIssue{},
		Failed:  []SourceModLogDeleteIssue{},
	}
	if len(targets) == 0 {
		return result, fmt.Errorf("%w: select at least one file", ErrInvalidSourceModLogName)
	}

	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if err := ValidateSourceModLogName(target.Name); err != nil || target.Version == "" {
			return result, fmt.Errorf("%w: %q", ErrInvalidSourceModLogName, target.Name)
		}
		if _, duplicate := seen[target.Name]; duplicate {
			return result, fmt.Errorf("%w: duplicate file %q", ErrInvalidSourceModLogName, target.Name)
		}
		seen[target.Name] = struct{}{}
	}

	root, err := os.OpenRoot(SourceModLogsDir())
	if err != nil {
		if os.IsNotExist(err) {
			for _, target := range targets {
				result.Skipped = append(result.Skipped, SourceModLogDeleteIssue{
					Name: target.Name, Reason: "not_found", Message: "日志文件不存在",
				})
			}
			return result, nil
		}
		return result, fmt.Errorf("open SourceMod log directory: %w", err)
	}
	defer root.Close()

	for _, target := range targets {
		info, statErr := root.Lstat(target.Name)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				result.Skipped = append(result.Skipped, SourceModLogDeleteIssue{
					Name: target.Name, Reason: "not_found", Message: "日志文件不存在",
				})
			} else {
				result.Failed = append(result.Failed, SourceModLogDeleteIssue{
					Name: target.Name, Reason: "stat_failed", Message: fmt.Sprintf("读取日志状态失败: %v", statErr),
				})
			}
			continue
		}
		if !info.Mode().IsRegular() {
			result.Skipped = append(result.Skipped, SourceModLogDeleteIssue{
				Name: target.Name, Reason: "invalid_type", Message: "只允许删除普通日志文件",
			})
			continue
		}

		file := buildSourceModLogFile(target.Name, info, now)
		if !file.Deletable {
			result.Skipped = append(result.Skipped, SourceModLogDeleteIssue{
				Name: target.Name, Reason: "protected", Message: file.ProtectedReason,
			})
			continue
		}
		if file.Version != target.Version {
			result.Skipped = append(result.Skipped, SourceModLogDeleteIssue{
				Name: target.Name, Reason: "changed", Message: "日志文件已发生变化，请刷新后重试",
			})
			continue
		}

		if removeErr := removeFile(root, target.Name); removeErr != nil {
			if os.IsNotExist(removeErr) {
				result.Skipped = append(result.Skipped, SourceModLogDeleteIssue{
					Name: target.Name, Reason: "not_found", Message: "日志文件不存在",
				})
			} else {
				result.Failed = append(result.Failed, SourceModLogDeleteIssue{
					Name: target.Name, Reason: "delete_failed", Message: fmt.Sprintf("删除日志失败，文件可能正在被占用: %v", removeErr),
				})
			}
			continue
		}
		result.Deleted = append(result.Deleted, target.Name)
		result.FreedBytes += file.Size
	}

	return result, nil
}
