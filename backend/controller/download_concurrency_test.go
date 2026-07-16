package controller

import (
	"sync"
	"testing"
)

func newDownloadConcurrencyTestTask(status DOWNLOAD_STATUS) *downloadTask {
	return &downloadTask{
		url:       "https://example.invalid/test.vpk",
		status:    status,
		cancel:    make(chan struct{}),
		semaphore: make(chan struct{}, 1),
	}
}

func TestDownloadTaskConcurrentStateAccess(t *testing.T) {
	task := newDownloadConcurrencyTestTask(DOWNLOAD_STATUS_PENDING)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		i := i
		wg.Add(2)

		go func() {
			defer wg.Done()
			switch i % 4 {
			case 0:
				task.markStarted()
			case 1:
				task.updateProgress(int64(i+1), 100)
			case 2:
				task.markFailed("test failure")
			default:
				task.markCompleted()
			}
		}()

		go func() {
			defer wg.Done()
			_ = task.GetStatus()
			_ = task.GetProgress()
			_ = task.GetMessage()
			_ = task.GetDownloadSpeed()
			_ = task.GetTotalSize()
			_ = task.GetFilename()
		}()
	}

	wg.Wait()
}

func TestDownloaderConcurrentTaskListAccess(t *testing.T) {
	downloader := NewDownloader()
	pendingTask := newDownloadConcurrencyTestTask(DOWNLOAD_STATUS_PENDING)
	completedTask := newDownloadConcurrencyTestTask(DOWNLOAD_STATUS_COMPLETED)

	downloader.tasks = append(downloader.tasks, pendingTask, completedTask)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)

		go func() {
			defer wg.Done()
			_ = downloader.GetTasksInfo()
		}()

		go func() {
			defer wg.Done()
			downloader.ClearFinishedTasks()
		}()

		go func() {
			defer wg.Done()
			downloader.CancelTask(0)
		}()
	}

	wg.Wait()

	tasks := downloader.GetTasksInfo()
	if len(tasks) != 1 {
		t.Fatalf("remaining task count = %d, want 1", len(tasks))
	}
}
