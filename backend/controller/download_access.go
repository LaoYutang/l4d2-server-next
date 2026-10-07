package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"l4d2-manager-next/logic"
	"l4d2-manager-next/middlewares"

	"github.com/gin-gonic/gin"
)

var (
	errDownloadTaskNotFound  = errors.New("下载任务不存在或已被清理")
	errDownloadTaskForbidden = errors.New("无权访问其他授权创建的下载任务")
)

// Only the restricted map role is scoped to its own downloads. Existing
// administrator and guest download permissions remain unchanged.
func downloadRequestOwner(c *gin.Context) (string, bool) {
	if c.GetString("role") != middlewares.RoleMapUploader {
		return "", true
	}
	if err := middlewares.RevalidateAuth(c); err != nil {
		authCodeError(c, err)
		return "", false
	}
	return c.GetString("auth_code_id"), true
}

func (dt *downloadTask) validateOwner() error {
	if dt.authCodeID == "" {
		return nil
	}
	_, err := logic.GetAuthCodeStore().ActiveByID(dt.authCodeID)
	return err
}

func downloadRequestTaskID(c *gin.Context) (string, bool) {
	if id := strings.TrimSpace(c.PostForm("id")); id != "" {
		return id, true
	}
	if c.GetString("role") == middlewares.RoleMapUploader {
		FailWithError(c, http.StatusBadRequest, "任务 ID 不能为空")
		return "", false
	}

	// Preserve the old index API for existing clients, resolving to a fixed ID
	// before acting so concurrent record cleanup cannot target a different task.
	index, err := strconv.Atoi(c.PostForm("index"))
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "任务 ID 或有效索引不能为空")
		return "", false
	}
	Downloader.mu.RLock()
	if index < 0 || index >= len(Downloader.tasks) {
		Downloader.mu.RUnlock()
		FailWithError(c, http.StatusBadRequest, "任务索引超出范围")
		return "", false
	}
	taskID := Downloader.tasks[index].id
	Downloader.mu.RUnlock()
	return taskID, true
}

// The caller holds d.mu for reading or writing.
func (d *downloader) taskIndexByID(id string) int {
	for index, task := range d.tasks {
		if task.id == id {
			return index
		}
	}
	return -1
}

func (d *downloader) cancelTaskForOwner(id, ownerID string) error {
	d.mu.RLock()
	index := d.taskIndexByID(id)
	if index < 0 {
		d.mu.RUnlock()
		return errDownloadTaskNotFound
	}
	task := d.tasks[index]
	d.mu.RUnlock()
	if ownerID != "" && task.authCodeID != ownerID {
		return errDownloadTaskForbidden
	}
	task.Cancel()
	return nil
}

func (d *downloader) restartTaskForOwner(id, ownerID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	index := d.taskIndexByID(id)
	if index < 0 {
		return errDownloadTaskNotFound
	}
	if ownerID != "" && d.tasks[index].authCodeID != ownerID {
		return errDownloadTaskForbidden
	}
	d.restartTaskLocked(index)
	return nil
}

func failDownloadTaskAccess(c *gin.Context, err error) {
	status := http.StatusNotFound
	if errors.Is(err, errDownloadTaskForbidden) {
		status = http.StatusForbidden
	}
	FailWithError(c, status, "%v", err)
}
