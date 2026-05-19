package logic

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPluginExportTaskCreatesUploadCompatibleZip(t *testing.T) {
	storePath := t.TempDir()
	t.Setenv(PluginStorePathEnv, storePath)
	resetPluginExportTasksForTest(t)

	writeTestFile(t, filepath.Join(storePath, "插件甲", "README.md"), "说明")
	writeTestFile(t, filepath.Join(storePath, "插件甲", "left4dead2", "addons", "sourcemod", "plugins", "甲.smx"), "smx")
	writeTestFile(t, filepath.Join(storePath, "PluginB", "left4dead2", "cfg", "sourcemod", "plugin_b.cfg"), "cfg")
	writeTestFile(t, filepath.Join(storePath, ConfigFileName), "enabled_plugins: []")
	writeTestFile(t, filepath.Join(storePath, DownloadTempDir, "Ignored", "left4dead2", "ignored.smx"), "ignored")
	writeTestFile(t, filepath.Join(storePath, ExportTempDir, "old", PluginExportFileName), "ignored")

	progress, err := StartPluginExportTask()
	if err != nil {
		t.Fatalf("StartPluginExportTask() error = %v", err)
	}

	progress = waitPluginExportTask(t, progress.TaskID)
	if progress.Status != PluginExportStatusCompleted {
		t.Fatalf("status = %s, want %s: %s", progress.Status, PluginExportStatusCompleted, progress.Message)
	}
	if progress.Total != 3 || progress.Processed != 3 {
		t.Fatalf("progress = %d/%d, want 3/3", progress.Processed, progress.Total)
	}

	zipPath, err := GetCompletedPluginExportPath(progress.TaskID)
	if err != nil {
		t.Fatalf("GetCompletedPluginExportPath() error = %v", err)
	}

	names := readZipNames(t, zipPath)
	wantNames := []string{
		"插件甲/README.md",
		"插件甲/left4dead2/addons/sourcemod/plugins/甲.smx",
		"PluginB/left4dead2/cfg/sourcemod/plugin_b.cfg",
	}
	for _, want := range wantNames {
		if !containsString(names, want) {
			t.Fatalf("zip missing %q; entries: %v", want, names)
		}
	}
	for _, name := range names {
		if name == ConfigFileName || strings.HasPrefix(name, DownloadTempDir+"/") || strings.HasPrefix(name, ExportTempDir+"/") {
			t.Fatalf("zip contains ignored entry %q; entries: %v", name, names)
		}
	}

	CleanupPluginExportTask(progress.TaskID)
	if _, err := os.Stat(filepath.Join(storePath, ExportTempDir, progress.TaskID)); !os.IsNotExist(err) {
		t.Fatalf("export temp dir still exists after cleanup: %v", err)
	}
}

func TestStartPluginExportTaskRejectsEmptyStore(t *testing.T) {
	storePath := t.TempDir()
	t.Setenv(PluginStorePathEnv, storePath)
	resetPluginExportTasksForTest(t)

	_, err := StartPluginExportTask()
	if err == nil || !strings.Contains(err.Error(), "没有可导出的插件文件") {
		t.Fatalf("StartPluginExportTask() error = %v, want empty export error", err)
	}
}

func TestPluginExportCancelledTaskCleansTemp(t *testing.T) {
	storePath := t.TempDir()
	tempDir := filepath.Join(storePath, ExportTempDir, "cancelled-task")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("mkdir temp dir: %v", err)
	}

	srcPath := filepath.Join(storePath, "PluginA", "left4dead2", "addons", "sourcemod", "plugins", "a.smx")
	writeTestFile(t, srcPath, "smx")

	ctx, cancel := context.WithCancel(context.Background())
	task := &pluginExportTask{
		id:        "cancelled-task",
		tempDir:   tempDir,
		zipPath:   filepath.Join(tempDir, PluginExportFileName),
		ctx:       ctx,
		cancel:    cancel,
		status:    PluginExportStatusPending,
		total:     1,
		message:   "等待压缩",
		updatedAt: time.Now(),
	}

	cancel()
	task.run([]pluginExportFile{{
		srcPath: srcPath,
		zipName: "PluginA/left4dead2/addons/sourcemod/plugins/a.smx",
		mode:    0644,
	}})

	progress := task.snapshot()
	if progress.Status != PluginExportStatusCancelled {
		t.Fatalf("status = %s, want %s", progress.Status, PluginExportStatusCancelled)
	}
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Fatalf("export temp dir still exists after cancellation: %v", err)
	}
}

func resetPluginExportTasksForTest(t *testing.T) {
	t.Helper()

	pluginExportTaskMut.Lock()
	oldTasks := pluginExportTasks
	pluginExportTasks = make(map[string]*pluginExportTask)
	pluginExportTaskMut.Unlock()

	for _, task := range oldTasks {
		task.cancel()
		os.RemoveAll(task.tempDir)
	}

	t.Cleanup(func() {
		pluginExportTaskMut.Lock()
		tasks := pluginExportTasks
		pluginExportTasks = make(map[string]*pluginExportTask)
		pluginExportTaskMut.Unlock()

		for _, task := range tasks {
			task.cancel()
			os.RemoveAll(task.tempDir)
		}
	})
}

func waitPluginExportTask(t *testing.T, taskID string) PluginExportProgress {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		progress, err := GetPluginExportProgress(taskID)
		if err != nil {
			t.Fatalf("GetPluginExportProgress() error = %v", err)
		}
		if progress.Status == PluginExportStatusCompleted ||
			progress.Status == PluginExportStatusFailed ||
			progress.Status == PluginExportStatusCancelled {
			return progress
		}
		time.Sleep(20 * time.Millisecond)
	}

	progress, err := GetPluginExportProgress(taskID)
	if err != nil {
		t.Fatalf("GetPluginExportProgress() error = %v", err)
	}
	t.Fatalf("export task did not finish before timeout: %+v", progress)
	return progress
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readZipNames(t *testing.T, path string) []string {
	t.Helper()

	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip %s: %v", path, err)
	}
	defer reader.Close()

	names := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	return names
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
