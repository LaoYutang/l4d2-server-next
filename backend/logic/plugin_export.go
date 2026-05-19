package logic

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	ExportTempDir              = ".export_temp"
	PluginExportFileName       = "plugins_all.zip"
	pluginExportTaskExpireTime = 30 * time.Minute
)

type PluginExportStatus string

const (
	PluginExportStatusPending     PluginExportStatus = "pending"
	PluginExportStatusCompressing PluginExportStatus = "compressing"
	PluginExportStatusCompleted   PluginExportStatus = "completed"
	PluginExportStatusFailed      PluginExportStatus = "failed"
	PluginExportStatusCancelled   PluginExportStatus = "cancelled"
)

type PluginExportProgress struct {
	TaskID    string             `json:"task_id"`
	Status    PluginExportStatus `json:"status"`
	Processed int                `json:"processed"`
	Total     int                `json:"total"`
	Message   string             `json:"message"`
}

type pluginExportFile struct {
	srcPath string
	zipName string
	mode    os.FileMode
}

type pluginExportTask struct {
	id      string
	tempDir string
	zipPath string
	ctx     context.Context
	cancel  context.CancelFunc

	mu        sync.RWMutex
	status    PluginExportStatus
	processed int
	total     int
	message   string
	updatedAt time.Time
}

var (
	pluginExportTaskMut sync.Mutex
	pluginExportTasks   = make(map[string]*pluginExportTask)
)

func CleanPluginExportTemp() {
	os.RemoveAll(filepath.Join(getStorePath(), ExportTempDir))
}

func StartPluginExportTask() (PluginExportProgress, error) {
	cleanupExpiredPluginExportTasks()

	pluginExportTaskMut.Lock()
	for _, task := range pluginExportTasks {
		if task.isActive() {
			progress := task.snapshot()
			pluginExportTaskMut.Unlock()
			return progress, nil
		}
	}
	pluginExportTaskMut.Unlock()

	storePath := getStorePath()
	files, err := collectPluginExportFiles(storePath)
	if err != nil {
		return PluginExportProgress{}, err
	}
	if len(files) == 0 {
		return PluginExportProgress{}, fmt.Errorf("没有可导出的插件文件")
	}

	taskID := uuid.New().String()
	tempDir := filepath.Join(storePath, ExportTempDir, taskID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return PluginExportProgress{}, fmt.Errorf("创建导出临时目录失败: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	task := &pluginExportTask{
		id:        taskID,
		tempDir:   tempDir,
		zipPath:   filepath.Join(tempDir, PluginExportFileName),
		ctx:       ctx,
		cancel:    cancel,
		status:    PluginExportStatusPending,
		total:     len(files),
		message:   "等待压缩",
		updatedAt: time.Now(),
	}

	pluginExportTaskMut.Lock()
	for _, existing := range pluginExportTasks {
		if existing.isActive() {
			progress := existing.snapshot()
			pluginExportTaskMut.Unlock()
			cancel()
			os.RemoveAll(tempDir)
			return progress, nil
		}
	}
	pluginExportTasks[taskID] = task
	pluginExportTaskMut.Unlock()

	go task.run(files)

	return task.snapshot(), nil
}

func GetPluginExportProgress(taskID string) (PluginExportProgress, error) {
	task := getPluginExportTask(taskID)
	if task == nil {
		return PluginExportProgress{}, fmt.Errorf("导出任务不存在")
	}
	return task.snapshot(), nil
}

func CancelPluginExportTask(taskID string) (PluginExportProgress, error) {
	task := getPluginExportTask(taskID)
	if task == nil {
		return PluginExportProgress{}, fmt.Errorf("导出任务不存在")
	}
	task.requestCancel()
	return task.snapshot(), nil
}

func GetCompletedPluginExportPath(taskID string) (string, error) {
	task := getPluginExportTask(taskID)
	if task == nil {
		return "", fmt.Errorf("导出任务不存在")
	}
	progress := task.snapshot()
	if progress.Status != PluginExportStatusCompleted {
		return "", fmt.Errorf("导出任务尚未完成")
	}
	if _, err := os.Stat(task.zipPath); err != nil {
		return "", fmt.Errorf("导出文件不存在: %v", err)
	}
	return task.zipPath, nil
}

func CleanupPluginExportTask(taskID string) {
	pluginExportTaskMut.Lock()
	task := pluginExportTasks[taskID]
	delete(pluginExportTasks, taskID)
	pluginExportTaskMut.Unlock()

	if task != nil {
		task.cancel()
		os.RemoveAll(task.tempDir)
	}
}

func getPluginExportTask(taskID string) *pluginExportTask {
	if taskID == "" {
		return nil
	}
	pluginExportTaskMut.Lock()
	defer pluginExportTaskMut.Unlock()
	return pluginExportTasks[taskID]
}

func cleanupExpiredPluginExportTasks() {
	now := time.Now()
	var expired []string

	pluginExportTaskMut.Lock()
	for id, task := range pluginExportTasks {
		progress := task.snapshot()
		if progress.Status == PluginExportStatusPending || progress.Status == PluginExportStatusCompressing {
			continue
		}
		task.mu.RLock()
		isExpired := now.Sub(task.updatedAt) > pluginExportTaskExpireTime
		task.mu.RUnlock()
		if isExpired {
			expired = append(expired, id)
		}
	}
	pluginExportTaskMut.Unlock()

	for _, id := range expired {
		CleanupPluginExportTask(id)
	}
}

func (t *pluginExportTask) snapshot() PluginExportProgress {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return PluginExportProgress{
		TaskID:    t.id,
		Status:    t.status,
		Processed: t.processed,
		Total:     t.total,
		Message:   t.message,
	}
}

func (t *pluginExportTask) isActive() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status == PluginExportStatusPending || t.status == PluginExportStatusCompressing
}

func (t *pluginExportTask) setStatus(status PluginExportStatus, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status = status
	t.message = message
	t.updatedAt = time.Now()
}

func (t *pluginExportTask) incrementProcessed() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.processed++
	t.updatedAt = time.Now()
}

func (t *pluginExportTask) requestCancel() {
	t.cancel()

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.status == PluginExportStatusPending || t.status == PluginExportStatusCompressing {
		t.status = PluginExportStatusCancelled
		t.message = "导出已取消"
		t.updatedAt = time.Now()
	}
}

func (t *pluginExportTask) run(files []pluginExportFile) {
	defer t.cancel()

	if t.ctx.Err() != nil {
		os.RemoveAll(t.tempDir)
		t.setStatus(PluginExportStatusCancelled, "导出已取消")
		return
	}

	t.setStatus(PluginExportStatusCompressing, "正在压缩插件文件")
	if err := t.writeZip(files); err != nil {
		os.RemoveAll(t.tempDir)
		if t.ctx.Err() != nil {
			t.setStatus(PluginExportStatusCancelled, "导出已取消")
			return
		}
		t.setStatus(PluginExportStatusFailed, err.Error())
		return
	}

	if t.ctx.Err() != nil {
		os.RemoveAll(t.tempDir)
		t.setStatus(PluginExportStatusCancelled, "导出已取消")
		return
	}

	t.setStatus(PluginExportStatusCompleted, "导出完成")
}

func (t *pluginExportTask) writeZip(files []pluginExportFile) error {
	out, err := os.Create(t.zipPath)
	if err != nil {
		return fmt.Errorf("创建导出文件失败: %v", err)
	}
	defer out.Close()

	zipWriter := zip.NewWriter(out)

	for _, file := range files {
		if err := t.ctx.Err(); err != nil {
			return err
		}
		if err := writePluginExportFile(zipWriter, file); err != nil {
			return err
		}
		t.incrementProcessed()
	}
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("写入导出压缩包失败: %v", err)
	}
	return nil
}

func collectPluginExportFiles(storePath string) ([]pluginExportFile, error) {
	entries, err := os.ReadDir(storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []pluginExportFile{}, nil
		}
		return nil, fmt.Errorf("读取插件目录失败: %v", err)
	}

	var files []pluginExportFile
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if name == DownloadTempDir || name == ExportTempDir {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("读取插件 %s 信息失败: %v", name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		pluginDir := filepath.Join(storePath, name)
		err = filepath.WalkDir(pluginDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(storePath, path)
			if err != nil {
				return err
			}
			zipName := filepath.ToSlash(relPath)
			if isJunkFile(zipName) {
				return nil
			}

			files = append(files, pluginExportFile{
				srcPath: path,
				zipName: zipName,
				mode:    info.Mode(),
			})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("扫描插件 %s 失败: %v", name, err)
		}
	}

	return files, nil
}

func writePluginExportFile(zipWriter *zip.Writer, file pluginExportFile) error {
	src, err := os.Open(file.srcPath)
	if err != nil {
		return fmt.Errorf("打开文件 %s 失败: %v", file.zipName, err)
	}
	defer src.Close()

	header := &zip.FileHeader{
		Name:   strings.TrimPrefix(file.zipName, "/"),
		Method: zip.Deflate,
	}
	header.SetMode(file.mode)

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("创建压缩条目 %s 失败: %v", file.zipName, err)
	}
	if _, err := io.Copy(writer, src); err != nil {
		return fmt.Errorf("写入压缩条目 %s 失败: %v", file.zipName, err)
	}
	return nil
}
