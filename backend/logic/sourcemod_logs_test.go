package logic

import (
	"errors"
	"l4d2-manager-next/consts"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanSourceModLogsClassifiesAndProtectsFiles(t *testing.T) {
	logsDir := setupSourceModLogTestDir(t)
	location := time.Local
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, location)

	writeSourceModLogTestFile(t, logsDir, "L20260701.log", "run", time.Date(2026, 7, 1, 12, 0, 0, 0, location))
	writeSourceModLogTestFile(t, logsDir, "errors_20260702.log", "error", time.Date(2026, 7, 3, 8, 0, 0, 0, location))
	writeSourceModLogTestFile(t, logsDir, "L20261340.log", "invalid", time.Date(2026, 6, 1, 9, 0, 0, 0, location))
	writeSourceModLogTestFile(t, logsDir, "plugin.LOG", "plugin", time.Date(2026, 5, 1, 9, 0, 0, 0, location))
	writeSourceModLogTestFile(t, logsDir, "recent.log", "recent", now.Add(-5*time.Minute))
	writeSourceModLogTestFile(t, logsDir, "L20260813.log", "today", time.Date(2026, 8, 13, 1, 0, 0, 0, location))
	writeSourceModLogTestFile(t, logsDir, "errors_20260814.log", "future", time.Date(2026, 8, 1, 1, 0, 0, 0, location))
	writeSourceModLogTestFile(t, logsDir, "ignored.txt", "ignored", time.Date(2026, 1, 1, 0, 0, 0, 0, location))
	if err := os.Mkdir(filepath.Join(logsDir, "directory.log"), 0755); err != nil {
		t.Fatalf("create directory log: %v", err)
	}

	scan, err := ScanSourceModLogs(now)
	if err != nil {
		t.Fatalf("scan logs: %v", err)
	}
	if !scan.Installed {
		t.Fatal("SourceMod should be installed")
	}
	if len(scan.Files) != 7 {
		t.Fatalf("file count = %d, want 7", len(scan.Files))
	}

	files := sourceModLogFilesByName(scan.Files)
	run := files["L20260701.log"]
	if run.Category != SourceModLogCategoryRun || run.Date != "20260701" || !run.Deletable {
		t.Fatalf("unexpected run log: %+v", run)
	}
	wantRunCleanup := time.Date(2026, 7, 2, 0, 0, 0, 0, location).Add(-time.Nanosecond)
	if !run.CleanupAt.Equal(wantRunCleanup) {
		t.Fatalf("run cleanup_at = %s, want %s", run.CleanupAt, wantRunCleanup)
	}

	errorLog := files["errors_20260702.log"]
	wantErrorCleanup := time.Date(2026, 7, 3, 8, 0, 0, 0, location)
	if errorLog.Category != SourceModLogCategoryErrors || !errorLog.CleanupAt.Equal(wantErrorCleanup) {
		t.Fatalf("unexpected error log: %+v", errorLog)
	}
	if files["L20261340.log"].Category != SourceModLogCategoryOther {
		t.Fatalf("invalid filename date should be other: %+v", files["L20261340.log"])
	}
	if files["plugin.LOG"].Category != SourceModLogCategoryOther {
		t.Fatalf("uppercase extension should be included as other: %+v", files["plugin.LOG"])
	}
	if recent := files["recent.log"]; recent.Deletable || recent.ProtectedReason != "最近 10 分钟内仍有更新" {
		t.Fatalf("recent log should be protected: %+v", recent)
	}
	if today := files["L20260813.log"]; today.Deletable || today.ProtectedReason != "今天或未来日期的标准日志" {
		t.Fatalf("today log should be protected: %+v", today)
	}
	if future := files["errors_20260814.log"]; future.Deletable || future.ProtectedReason != "今天或未来日期的标准日志" {
		t.Fatalf("future log should be protected: %+v", future)
	}
}

func TestPreviewSourceModLogCleanupFiltersByCategoryAndRetention(t *testing.T) {
	logsDir := setupSourceModLogTestDir(t)
	location := time.Local
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, location)

	writeSourceModLogTestFile(t, logsDir, "L20260701.log", "1234567890", time.Date(2026, 7, 1, 1, 0, 0, 0, location))
	writeSourceModLogTestFile(t, logsDir, "errors_20260724.log", "12345678901234567890", time.Date(2026, 7, 24, 1, 0, 0, 0, location))
	writeSourceModLogTestFile(t, logsDir, "plugin.log", "123456789012345678901234567890", time.Date(2026, 5, 1, 1, 0, 0, 0, location))
	writeSourceModLogTestFile(t, logsDir, "L20260813.log", "today", time.Date(2026, 8, 13, 1, 0, 0, 0, location))

	preview, err := PreviewSourceModLogCleanup(now, SourceModLogCleanupFilter{
		Categories:    []string{SourceModLogCategoryRun, SourceModLogCategoryErrors, SourceModLogCategoryOther},
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("preview logs: %v", err)
	}
	if preview.Count != 2 || preview.TotalSize != 40 {
		t.Fatalf("preview = count %d, size %d; want count 2, size 40", preview.Count, preview.TotalSize)
	}
	candidates := sourceModLogFilesByName(preview.Candidates)
	if _, ok := candidates["L20260701.log"]; !ok {
		t.Fatal("30-day preview should include old run log")
	}
	if _, ok := candidates["plugin.log"]; !ok {
		t.Fatal("30-day preview should include old plugin log")
	}
	if len(preview.Protected) != 1 || preview.Protected[0].Name != "L20260813.log" {
		t.Fatalf("protected logs = %+v, want today's run log", preview.Protected)
	}

	errorPreview, err := PreviewSourceModLogCleanup(now, SourceModLogCleanupFilter{
		Categories:    []string{SourceModLogCategoryErrors},
		RetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("preview error logs: %v", err)
	}
	if errorPreview.Count != 1 || errorPreview.Candidates[0].Name != "errors_20260724.log" {
		t.Fatalf("unexpected error preview: %+v", errorPreview)
	}

	ninetyDayPreview, err := PreviewSourceModLogCleanup(now, SourceModLogCleanupFilter{
		Categories:    []string{SourceModLogCategoryRun, SourceModLogCategoryErrors, SourceModLogCategoryOther},
		RetentionDays: 90,
	})
	if err != nil {
		t.Fatalf("preview 90-day logs: %v", err)
	}
	if ninetyDayPreview.Count != 1 || ninetyDayPreview.Candidates[0].Name != "plugin.log" {
		t.Fatalf("unexpected 90-day preview: %+v", ninetyDayPreview)
	}

	allHistoryPreview, err := PreviewSourceModLogCleanup(now, SourceModLogCleanupFilter{
		Categories:    []string{SourceModLogCategoryRun, SourceModLogCategoryErrors, SourceModLogCategoryOther},
		RetentionDays: 0,
	})
	if err != nil {
		t.Fatalf("preview all historical logs: %v", err)
	}
	if allHistoryPreview.Count != 3 || allHistoryPreview.TotalSize != 60 {
		t.Fatalf("unexpected all-history preview: %+v", allHistoryPreview)
	}

	_, err = PreviewSourceModLogCleanup(now, SourceModLogCleanupFilter{
		Categories:    []string{SourceModLogCategoryRun},
		RetentionDays: 14,
	})
	if !errors.Is(err, ErrInvalidSourceModLogCleanupFilter) {
		t.Fatalf("invalid retention error = %v", err)
	}
}

func TestDeleteSourceModLogsHandlesChangedProtectedAndMissingFiles(t *testing.T) {
	logsDir := setupSourceModLogTestDir(t)
	location := time.Local
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, location)
	oldTime := time.Date(2026, 5, 1, 1, 0, 0, 0, location)

	writeSourceModLogTestFile(t, logsDir, "delete.log", "delete-me", oldTime)
	writeSourceModLogTestFile(t, logsDir, "changed.log", "before", oldTime)
	writeSourceModLogTestFile(t, logsDir, "recent.log", "recent", now.Add(-time.Minute))
	if err := os.Mkdir(filepath.Join(logsDir, "directory.log"), 0755); err != nil {
		t.Fatalf("create directory log: %v", err)
	}

	scan, err := ScanSourceModLogs(now)
	if err != nil {
		t.Fatalf("scan logs: %v", err)
	}
	files := sourceModLogFilesByName(scan.Files)
	writeSourceModLogTestFile(t, logsDir, "changed.log", "after-change", oldTime.Add(time.Hour))

	result, err := DeleteSourceModLogs(now, []SourceModLogDeleteTarget{
		{Name: "delete.log", Version: files["delete.log"].Version},
		{Name: "changed.log", Version: files["changed.log"].Version},
		{Name: "recent.log", Version: files["recent.log"].Version},
		{Name: "missing.log", Version: "0:0"},
		{Name: "directory.log", Version: "0:0"},
	})
	if err != nil {
		t.Fatalf("delete logs: %v", err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "delete.log" || result.FreedBytes != int64(len("delete-me")) {
		t.Fatalf("unexpected delete result: %+v", result)
	}
	reasons := sourceModLogDeleteReasons(result.Skipped)
	for name, want := range map[string]string{
		"changed.log":   "changed",
		"recent.log":    "protected",
		"missing.log":   "not_found",
		"directory.log": "invalid_type",
	} {
		if reasons[name] != want {
			t.Fatalf("skip reason for %s = %q, want %q; result: %+v", name, reasons[name], want, result)
		}
	}
	if len(result.Failed) != 0 {
		t.Fatalf("unexpected failures: %+v", result.Failed)
	}
	if _, err := os.Stat(filepath.Join(logsDir, "delete.log")); !os.IsNotExist(err) {
		t.Fatalf("delete.log still exists: %v", err)
	}
}

func TestDeleteSourceModLogsRejectsInvalidTargetsBeforeDeleting(t *testing.T) {
	logsDir := setupSourceModLogTestDir(t)
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.Local)
	oldTime := now.Add(-90 * 24 * time.Hour)
	writeSourceModLogTestFile(t, logsDir, "safe.log", "safe", oldTime)
	outsidePath := filepath.Join(filepath.Dir(logsDir), "outside.log")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	scan, err := ScanSourceModLogs(now)
	if err != nil {
		t.Fatalf("scan logs: %v", err)
	}
	files := sourceModLogFilesByName(scan.Files)
	_, err = DeleteSourceModLogs(now, []SourceModLogDeleteTarget{
		{Name: "safe.log", Version: files["safe.log"].Version},
		{Name: "../outside.log", Version: "0:0"},
	})
	if !errors.Is(err, ErrInvalidSourceModLogName) {
		t.Fatalf("invalid target error = %v", err)
	}
	for _, path := range []string{filepath.Join(logsDir, "safe.log"), outsidePath} {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("file %s changed after invalid request: %v", path, statErr)
		}
	}
}

func TestDeleteSourceModLogsReturnsPartialFailure(t *testing.T) {
	logsDir := setupSourceModLogTestDir(t)
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.Local)
	oldTime := now.Add(-90 * 24 * time.Hour)
	writeSourceModLogTestFile(t, logsDir, "success.log", "success", oldTime)
	writeSourceModLogTestFile(t, logsDir, "failure.log", "failure", oldTime)

	scan, err := ScanSourceModLogs(now)
	if err != nil {
		t.Fatalf("scan logs: %v", err)
	}
	files := sourceModLogFilesByName(scan.Files)
	result, err := deleteSourceModLogs(
		now,
		[]SourceModLogDeleteTarget{
			{Name: "success.log", Version: files["success.log"].Version},
			{Name: "failure.log", Version: files["failure.log"].Version},
		},
		func(root *os.Root, name string) error {
			if name == "failure.log" {
				return errors.New("file is busy")
			}
			return root.Remove(name)
		},
	)
	if err != nil {
		t.Fatalf("delete logs: %v", err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "success.log" {
		t.Fatalf("unexpected deleted files: %+v", result)
	}
	if len(result.Failed) != 1 || result.Failed[0].Name != "failure.log" || result.Failed[0].Reason != "delete_failed" {
		t.Fatalf("unexpected failed files: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(logsDir, "success.log")); !os.IsNotExist(err) {
		t.Fatalf("success.log still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(logsDir, "failure.log")); err != nil {
		t.Fatalf("failure.log should remain: %v", err)
	}
}

func TestDeleteSourceModLogsRejectsSymlink(t *testing.T) {
	logsDir := setupSourceModLogTestDir(t)
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.Local)
	outsidePath := filepath.Join(filepath.Dir(logsDir), "outside.log")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	linkPath := filepath.Join(logsDir, "link.log")
	if err := os.Symlink(outsidePath, linkPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	result, err := DeleteSourceModLogs(now, []SourceModLogDeleteTarget{{Name: "link.log", Version: "0:0"}})
	if err != nil {
		t.Fatalf("delete symlink: %v", err)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Reason != "invalid_type" {
		t.Fatalf("unexpected symlink result: %+v", result)
	}
	if _, err := os.Stat(outsidePath); err != nil {
		t.Fatalf("outside target changed: %v", err)
	}
}

func TestValidateSourceModLogName(t *testing.T) {
	valid := []string{"plugin.log", "插件日志.LOG", "name..part.log"}
	for _, name := range valid {
		if err := ValidateSourceModLogName(name); err != nil {
			t.Errorf("ValidateSourceModLogName(%q) = %v", name, err)
		}
	}
	invalid := []string{"", ".", "..", "log.txt", "../outside.log", `..\outside.log`, "/tmp/x.log", `C:\temp\x.log`, "dir/file.log", "stream:log.log"}
	for _, name := range invalid {
		if err := ValidateSourceModLogName(name); !errors.Is(err, ErrInvalidSourceModLogName) {
			t.Errorf("ValidateSourceModLogName(%q) = %v, want invalid name", name, err)
		}
	}
}

func setupSourceModLogTestDir(t *testing.T) string {
	t.Helper()
	oldGamePath := consts.GamePath
	gamePath := t.TempDir()
	logsDir := filepath.Join(gamePath, "addons", "sourcemod", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("create SourceMod logs directory: %v", err)
	}
	consts.GamePath = gamePath
	t.Cleanup(func() { consts.GamePath = oldGamePath })
	return logsDir
}

func writeSourceModLogTestFile(t *testing.T, logsDir, name, content string, modifiedAt time.Time) {
	t.Helper()
	path := filepath.Join(logsDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write log %s: %v", name, err)
	}
	if err := os.Chtimes(path, modifiedAt, modifiedAt); err != nil {
		t.Fatalf("set log time %s: %v", name, err)
	}
}

func sourceModLogFilesByName(files []SourceModLogFile) map[string]SourceModLogFile {
	result := make(map[string]SourceModLogFile, len(files))
	for _, file := range files {
		result[file.Name] = file
	}
	return result
}

func sourceModLogDeleteReasons(issues []SourceModLogDeleteIssue) map[string]string {
	result := make(map[string]string, len(issues))
	for _, issue := range issues {
		result[issue.Name] = issue.Reason
	}
	return result
}
