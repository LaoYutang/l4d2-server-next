package controller

import (
	"context"
	"net/http"
	"sync"
)

// These task guards coordinate requests in one manager process. Each participant
// keeps a reference until its IO and cleanup finish, so a late request cannot
// acquire a fresh guard while cancellation or a merge is still in progress.
var uploadTasks = struct {
	sync.Mutex
	entries map[string]*uploadTask
}{entries: make(map[string]*uploadTask)}

type uploadTask struct {
	mu          sync.Mutex
	references  int // protected by uploadTasks
	ctx         context.Context
	cancel      context.CancelFunc
	writers     int
	writersDone chan struct{}
	operation   chan struct{}
	importing   bool
}

func acquireUploadTask(uploadID string) (*uploadTask, func()) {
	key := getUploadTempPath(uploadID)
	uploadTasks.Lock()
	task := uploadTasks.entries[key]
	if task == nil {
		ctx, cancel := context.WithCancel(context.Background())
		task = &uploadTask{ctx: ctx, cancel: cancel}
		uploadTasks.entries[key] = task
	}
	task.references++
	uploadTasks.Unlock()
	return task, func() {
		uploadTasks.Lock()
		defer uploadTasks.Unlock()
		task.references--
		if task.references == 0 {
			delete(uploadTasks.entries, key)
			task.cancel()
		}
	}
}

func (task *uploadTask) contextError(ctx context.Context) *uploadError {
	if task.ctx.Err() != nil {
		return uploadErrorf(http.StatusConflict, "上传任务已取消，请重新上传")
	}
	if ctx.Err() != nil {
		return uploadErrorf(http.StatusBadRequest, "上传请求已中断")
	}
	return nil
}

func (task *uploadTask) beginChunk() *uploadError {
	task.mu.Lock()
	defer task.mu.Unlock()
	if err := task.contextError(context.Background()); err != nil {
		return err
	}
	if task.operation != nil || task.importing {
		return uploadErrorf(http.StatusConflict, "上传任务正在合并、导入或清理，请勿继续上传分片")
	}
	if task.writers == 0 {
		task.writersDone = make(chan struct{})
	}
	task.writers++
	return nil
}

func (task *uploadTask) finishChunk() {
	task.mu.Lock()
	defer task.mu.Unlock()
	task.writers--
	if task.writers == 0 {
		close(task.writersDone)
		task.writersDone = nil
	}
}

func (task *uploadTask) beginMerge(ctx context.Context) (chan struct{}, *uploadError) {
	task.mu.Lock()
	if err := task.contextError(ctx); err != nil {
		task.mu.Unlock()
		return nil, err
	}
	if task.operation != nil || task.importing {
		task.mu.Unlock()
		return nil, uploadErrorf(http.StatusConflict, "上传任务正在合并、导入或清理，请勿重复合并")
	}
	done := make(chan struct{})
	task.operation = done
	writersDone := task.writersDone
	task.mu.Unlock()
	// A retry can finish before an older request for that same chunk leaves
	// its private file. The existing client immediately merges at this point
	// and treats any merge error as terminal, so wait instead of returning a
	// transient conflict. New writers are excluded by operation while we wait.
	if writersDone != nil {
		select {
		case <-writersDone:
		case <-ctx.Done():
		case <-task.ctx.Done():
		}
	}
	if err := task.contextError(ctx); err != nil {
		task.finishOperation(done)
		return nil, err
	}
	return done, nil
}

// Cancellation prevents publication immediately, then waits outside the mutex
// for writers/the merge to leave their files alone before removing the task.
// Once ProcessFile has started, it must finish before cancellation can clean up;
// installed maps are not rolled back by the upload cancellation endpoint.
func (task *uploadTask) beginCancel() chan struct{} {
	task.stop()
	for {
		task.mu.Lock()
		wait := task.operation
		if wait == nil && task.writers != 0 {
			wait = task.writersDone
		}
		if wait == nil {
			task.operation = make(chan struct{})
			done := task.operation
			task.mu.Unlock()
			return done
		}
		task.mu.Unlock()
		<-wait
	}
}

func (task *uploadTask) stop() {
	task.mu.Lock()
	defer task.mu.Unlock()
	task.cancel()
}

func (task *uploadTask) beginImport(ctx context.Context) *uploadError {
	task.mu.Lock()
	defer task.mu.Unlock()
	if err := task.contextError(ctx); err != nil {
		return err
	}
	task.importing = true
	return nil
}

func (task *uploadTask) finishOperation(done chan struct{}) {
	task.mu.Lock()
	defer task.mu.Unlock()
	if task.operation == done {
		task.operation = nil
		close(done)
	}
}
