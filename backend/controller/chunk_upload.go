package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/middlewares"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	uploadTempDir      = "upload_temp"
	maxUploadChunkSize = 6 << 20
	uploadImportMarker = ".importing"
)

type uploadError struct {
	status  int
	message string
	discard bool
}

func uploadErrorf(status int, format string, args ...any) *uploadError {
	return &uploadError{status: status, message: fmt.Sprintf(format, args...)}
}

func (err *uploadError) respond(c *gin.Context) {
	FailWithError(c, err.status, "%s", err.message)
}

type uploadMetadata struct {
	fileSize    int64
	totalChunks int
}

func readUploadMetadata(tempPath string) (uploadMetadata, *uploadError) {
	if _, err := os.Lstat(filepath.Join(tempPath, uploadImportMarker)); err == nil {
		return uploadMetadata{}, uploadErrorf(http.StatusConflict, "上传任务已进入导入阶段，不能安全续传或重复合并；请检查地图列表，未导入成功时请重新上传")
	} else if !os.IsNotExist(err) {
		return uploadMetadata{}, uploadErrorf(http.StatusInternalServerError, "读取上传任务状态失败: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(tempPath, ".meta"))
	if err != nil {
		return uploadMetadata{}, uploadErrorf(http.StatusBadRequest, "上传任务不存在或已过期，请重新上传")
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 3 {
		return uploadMetadata{}, uploadErrorf(http.StatusInternalServerError, "元信息格式错误，请重新上传")
	}
	size, sizeErr := strconv.ParseInt(lines[1], 10, 64)
	chunks, chunksErr := strconv.Atoi(lines[2])
	if sizeErr != nil || chunksErr != nil || size <= 0 || size > 2<<30 || chunks <= 0 || int64(chunks) > size {
		return uploadMetadata{}, uploadErrorf(http.StatusInternalServerError, "元信息解析失败，请重新上传")
	}
	return uploadMetadata{fileSize: size, totalChunks: chunks}, nil
}

// Narrow IO hooks allow deterministic interruption and disk-failure tests.
var copyUploadData = io.Copy
var renameUploadFile = os.Rename
var removeUploadTempDir = deleteUploadTempDir

type uploadReader struct {
	reader io.Reader
	task   *uploadTask
	ctx    context.Context
}

func (reader uploadReader) Read(p []byte) (int, error) {
	if err := reader.task.ctx.Err(); err != nil {
		return 0, err
	}
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(p)
}

func saveUploadChunk(ctx context.Context, task *uploadTask, chunk *multipart.FileHeader, tempPath string, index int) (path string, resultErr *uploadError) {
	source, err := chunk.Open()
	if err != nil {
		return "", uploadErrorf(http.StatusBadRequest, "分片文件读取失败: %v", err)
	}
	defer source.Close()
	temp, err := os.CreateTemp(tempPath, fmt.Sprintf(".chunk-%d-*.tmp", index))
	if err != nil {
		return "", uploadErrorf(http.StatusInternalServerError, "创建分片临时文件失败: %v", err)
	}
	path = temp.Name()
	defer func() {
		_ = temp.Close()
		if resultErr != nil {
			_ = os.Remove(temp.Name())
		}
	}()
	reader := uploadReader{reader: source, task: task, ctx: ctx}
	written, err := copyUploadData(temp, io.LimitReader(reader, maxUploadChunkSize+1))
	if ctxErr := task.contextError(ctx); ctxErr != nil {
		return "", ctxErr
	}
	if err != nil {
		return "", uploadErrorf(http.StatusInternalServerError, "保存分片失败: %v", err)
	}
	if written != chunk.Size {
		return "", uploadErrorf(http.StatusBadRequest, "分片大小不匹配 (期望 %d, 实际 %d)，请重新上传", chunk.Size, written)
	}
	if err := temp.Sync(); err != nil {
		return "", uploadErrorf(http.StatusInternalServerError, "保存分片失败: %v", err)
	}
	info, err := temp.Stat()
	if err != nil || info.Size() != chunk.Size {
		return "", uploadErrorf(http.StatusInternalServerError, "校验分片文件失败，请重新上传")
	}
	if err := temp.Close(); err != nil {
		return "", uploadErrorf(http.StatusInternalServerError, "关闭分片文件失败: %v", err)
	}
	return path, nil
}

func (task *uploadTask) publishChunk(ctx context.Context, source, destination string, size int64) *uploadError {
	// Only the final metadata operations are locked; copying and Sync happen in
	// the request's private file without holding the task mutex.
	task.mu.Lock()
	defer task.mu.Unlock()
	if err := task.contextError(ctx); err != nil {
		return err
	}
	if info, err := os.Lstat(destination); err == nil {
		if !info.Mode().IsRegular() || info.Size() != size {
			return uploadErrorf(http.StatusBadRequest, "已完成分片大小或类型不一致，请重新上传")
		}
		// A same-size retry is idempotent. Without a client-provided hash this
		// does not establish that its bytes match the first completed request.
		return nil
	} else if !os.IsNotExist(err) {
		return uploadErrorf(http.StatusInternalServerError, "读取已完成分片失败: %v", err)
	}
	if err := renameUploadFile(source, destination); err != nil {
		return uploadErrorf(http.StatusInternalServerError, "确认分片失败: %v", err)
	}
	return nil
}

// Administrators may manage every upload; other credentials own their tasks by
// immutable authorization ID. Legacy tasks without an owner are admin-only.
func ensureUploadOwner(c *gin.Context, uploadID string, allowMissing bool) bool {
	if c.GetString("role") == middlewares.RoleAdmin {
		return true
	}
	if err := middlewares.RevalidateAuth(c); err != nil {
		authCodeError(c, err)
		return false
	}
	root, err := os.OpenRoot(consts.AddonsBasePath)
	if err != nil {
		FailWithError(c, 400, "上传任务不存在或已过期")
		return false
	}
	defer root.Close()
	taskPath := filepath.Join(uploadTempDir, uploadID)
	if _, err := root.Stat(taskPath); os.IsNotExist(err) && allowMissing {
		return true
	}
	owner, err := root.ReadFile(filepath.Join(taskPath, ".auth_owner"))
	if err != nil || string(owner) != c.GetString("auth_code_id") {
		FailWithError(c, http.StatusForbidden, "无权访问其他授权创建的上传任务")
		return false
	}
	return true
}

func getUploadTempPath(uploadId string) string {
	return filepath.Join(consts.AddonsBasePath, uploadTempDir, uploadId)
}

func validateUploadId(uploadId string) error {
	parsed, err := uuid.Parse(uploadId)
	if err != nil ||
		parsed.String() != uploadId ||
		parsed.Version() != uuid.Version(4) ||
		parsed.Variant() != uuid.RFC4122 {
		return fmt.Errorf("uploadId 参数无效")
	}
	return nil
}

func deleteUploadTempDir(uploadId string) error {
	if err := validateUploadId(uploadId); err != nil {
		return err
	}

	root, err := os.OpenRoot(consts.AddonsBasePath)
	if err != nil {
		return err
	}
	defer root.Close()

	return root.RemoveAll(filepath.Join(uploadTempDir, uploadId))
}

// UploadInit 初始化分片上传
func UploadInit(c *gin.Context) {
	filename := c.PostForm("filename")
	fileSizeStr := c.PostForm("fileSize")
	totalChunksStr := c.PostForm("totalChunks")

	if filename == "" || fileSizeStr == "" || totalChunksStr == "" {
		FailWithError(c, http.StatusBadRequest, "缺少必要参数")
		return
	}

	fileSize, err := strconv.ParseInt(fileSizeStr, 10, 64)
	if err != nil || fileSize <= 0 {
		FailWithError(c, http.StatusBadRequest, "fileSize 参数无效")
		return
	}

	totalChunks, err := strconv.Atoi(totalChunksStr)
	if err != nil || totalChunks <= 0 || int64(totalChunks) > fileSize {
		FailWithError(c, http.StatusBadRequest, "totalChunks 参数无效")
		return
	}

	if fileSize > 2<<30 {
		FailWithError(c, http.StatusBadRequest, "文件超过2GB，禁止上传")
		return
	}

	ok, forced := ensureUploadDiskSpace(c, fileSize, true)
	if !ok {
		return
	}
	if err := middlewares.RevalidateAuth(c); err != nil {
		authCodeError(c, err)
		return
	}

	uploadId := uuid.New().String()
	tempPath := getUploadTempPath(uploadId)
	if err := os.MkdirAll(tempPath, 0755); err != nil {
		FailWithError(c, http.StatusInternalServerError, "创建临时目录失败: %v", err)
		return
	}

	// 保存上传元信息
	metaPath := filepath.Join(tempPath, ".meta")
	metaContent := fmt.Sprintf("%s\n%d\n%d\n", filename, fileSize, totalChunks)
	if err := os.WriteFile(metaPath, []byte(metaContent), 0644); err != nil {
		_ = removeUploadTempDir(uploadId)
		FailWithError(c, http.StatusInternalServerError, "保存元信息失败: %v", err)
		return
	}
	owner := c.GetString("auth_code_id")
	if c.GetString("role") == middlewares.RoleAdmin {
		owner = "admin"
	}
	if err := os.WriteFile(filepath.Join(tempPath, ".auth_owner"), []byte(owner), 0600); err != nil {
		_ = removeUploadTempDir(uploadId)
		FailWithError(c, http.StatusInternalServerError, "保存上传任务授权失败")
		return
	}

	detailPrefix := "初始化分片上传"
	if forced {
		detailPrefix = "管理员确认磁盘超限后初始化分片上传"
	}
	defer LogOp(c, fmt.Sprintf(
		"%s: %s，大小: %d，分片数: %d，uploadId: %s",
		detailPrefix,
		filename,
		fileSize,
		totalChunks,
		uploadId,
	))()
	c.JSON(http.StatusOK, gin.H{"uploadId": uploadId})
}

// UploadChunk 上传单个分片
func UploadChunk(c *gin.Context) {
	uploadId := c.PostForm("uploadId")
	chunkIndexStr := c.PostForm("chunkIndex")

	if uploadId == "" || chunkIndexStr == "" {
		FailWithError(c, http.StatusBadRequest, "缺少必要参数")
		return
	}
	if err := validateUploadId(uploadId); err != nil {
		FailWithError(c, http.StatusBadRequest, "%v", err)
		return
	}

	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil || chunkIndex < 0 {
		FailWithError(c, http.StatusBadRequest, "chunkIndex 参数无效")
		return
	}
	if !ensureUploadOwner(c, uploadId, false) {
		return
	}
	task, release := acquireUploadTask(uploadId)
	defer release()
	if err := task.beginChunk(); err != nil {
		err.respond(c)
		return
	}
	defer task.finishChunk()
	tempPath := getUploadTempPath(uploadId)
	meta, metaErr := readUploadMetadata(tempPath)
	if metaErr != nil {
		metaErr.respond(c)
		return
	}
	if chunkIndex >= meta.totalChunks {
		FailWithError(c, http.StatusBadRequest, "chunkIndex 参数无效")
		return
	}
	chunkFile, err := c.FormFile("chunk")
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "分片文件读取失败: %v", err)
		return
	}
	if chunkFile.Size <= 0 || chunkFile.Size > maxUploadChunkSize || chunkFile.Size > meta.fileSize {
		FailWithError(c, http.StatusBadRequest, "分片大小无效或超过限制 (%d)，请重新上传", chunkFile.Size)
		return
	}
	tmpPath, saveErr := saveUploadChunk(c.Request.Context(), task, chunkFile, tempPath, chunkIndex)
	if saveErr != nil {
		saveErr.respond(c)
		return
	}
	defer os.Remove(tmpPath)
	chunkPath := filepath.Join(tempPath, strconv.Itoa(chunkIndex))
	if err := task.publishChunk(c.Request.Context(), tmpPath, chunkPath, chunkFile.Size); err != nil {
		err.respond(c)
		return
	}
	c.String(http.StatusOK, "OK")
}

// UploadStatus 查询已上传的分片
func UploadStatus(c *gin.Context) {
	uploadId := c.PostForm("uploadId")
	if uploadId == "" {
		FailWithError(c, http.StatusBadRequest, "缺少 uploadId 参数")
		return
	}
	if err := validateUploadId(uploadId); err != nil {
		FailWithError(c, http.StatusBadRequest, "%v", err)
		return
	}

	tempPath := getUploadTempPath(uploadId)
	entries, err := os.ReadDir(tempPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"uploadedChunks": []int{}})
			return
		}
		FailWithError(c, http.StatusInternalServerError, "读取临时目录失败: %v", err)
		return
	}

	var uploadedChunks []int
	if !ensureUploadOwner(c, uploadId, false) {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == ".meta" || strings.HasSuffix(name, ".tmp") {
			continue
		}
		if idx, err := strconv.Atoi(name); err == nil && idx >= 0 && strconv.Itoa(idx) == name && entry.Type().IsRegular() {
			uploadedChunks = append(uploadedChunks, idx)
		}
	}

	sort.Ints(uploadedChunks)
	c.JSON(http.StatusOK, gin.H{"uploadedChunks": uploadedChunks})
}

func validateUploadChunks(tempPath string, meta uploadMetadata) *uploadError {
	var totalSize int64
	for i := 0; i < meta.totalChunks; i++ {
		info, err := os.Lstat(filepath.Join(tempPath, strconv.Itoa(i)))
		if os.IsNotExist(err) {
			return uploadErrorf(http.StatusBadRequest, "分片 %d 缺失，无法合并，请重新上传", i)
		}
		if err != nil {
			return uploadErrorf(http.StatusInternalServerError, "读取分片 %d 失败: %v", i, err)
		}
		if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxUploadChunkSize {
			return &uploadError{status: http.StatusBadRequest, message: fmt.Sprintf("分片 %d 大小或类型无效，请重新上传", i), discard: true}
		}
		totalSize += info.Size()
	}
	if totalSize != meta.fileSize {
		return &uploadError{status: http.StatusInternalServerError, message: fmt.Sprintf("合并文件大小不匹配 (期望 %d, 实际 %d)，请重新上传", meta.fileSize, totalSize), discard: true}
	}
	return nil
}

func mergeUploadChunks(ctx context.Context, task *uploadTask, tempPath, cleanFilename string, meta uploadMetadata) (string, *uploadError) {
	temp, err := os.CreateTemp(tempPath, ".merge-*.tmp")
	if err != nil {
		return "", uploadErrorf(http.StatusInternalServerError, "创建合并文件失败: %v", err)
	}
	defer func() { _ = temp.Close(); _ = os.Remove(temp.Name()) }()
	var written int64
	for i := 0; i < meta.totalChunks; i++ {
		chunk, err := os.Open(filepath.Join(tempPath, strconv.Itoa(i)))
		if err != nil {
			return "", uploadErrorf(http.StatusInternalServerError, "打开分片 %d 失败: %v", i, err)
		}
		n, copyErr := copyUploadData(temp, uploadReader{reader: chunk, task: task, ctx: ctx})
		closeErr := chunk.Close()
		if err := task.contextError(ctx); err != nil {
			return "", err
		}
		if copyErr != nil || closeErr != nil {
			return "", uploadErrorf(http.StatusInternalServerError, "合并分片 %d 失败: %v", i, errors.Join(copyErr, closeErr))
		}
		written += n
	}
	if err := temp.Sync(); err != nil {
		return "", uploadErrorf(http.StatusInternalServerError, "保存合并文件失败: %v", err)
	}
	info, err := temp.Stat()
	if err != nil {
		return "", uploadErrorf(http.StatusInternalServerError, "读取合并文件信息失败: %v", err)
	}
	if written != meta.fileSize || info.Size() != meta.fileSize {
		return "", &uploadError{status: http.StatusInternalServerError, message: fmt.Sprintf("合并文件大小不匹配 (期望 %d, 实际 %d)，请重新上传", meta.fileSize, info.Size()), discard: true}
	}
	if err := temp.Close(); err != nil {
		return "", uploadErrorf(http.StatusInternalServerError, "关闭合并文件失败: %v", err)
	}
	path := filepath.Join(tempPath, "merged_"+cleanFilename)
	if err := renameUploadFile(temp.Name(), path); err != nil {
		return "", uploadErrorf(http.StatusInternalServerError, "确认合并文件失败: %v", err)
	}
	return path, nil
}

// UploadMerge 合并分片并处理文件
func UploadMerge(c *gin.Context) {
	uploadId := c.PostForm("uploadId")
	filename := c.PostForm("filename")
	if uploadId == "" || filename == "" {
		FailWithError(c, http.StatusBadRequest, "缺少必要参数")
		return
	}
	if err := validateUploadId(uploadId); err != nil {
		FailWithError(c, http.StatusBadRequest, "%v", err)
		return
	}
	if !ensureUploadOwner(c, uploadId, false) {
		return
	}
	task, release := acquireUploadTask(uploadId)
	defer release()
	done, operationErr := task.beginMerge(c.Request.Context())
	if operationErr != nil {
		operationErr.respond(c)
		return
	}
	defer task.finishOperation(done)
	tempPath := getUploadTempPath(uploadId)
	meta, metaErr := readUploadMetadata(tempPath)
	if metaErr != nil {
		metaErr.respond(c)
		return
	}
	if err := validateUploadChunks(tempPath, meta); err != nil {
		if err.discard {
			task.stop()
			_ = removeUploadTempDir(uploadId)
		}
		err.respond(c)
		return
	}
	cleanFilename := sanitizeFilename(filename)
	vpkReg := regexp.MustCompile(`(?i)\.(vpk|zip|rar|7z)$`)
	if !vpkReg.MatchString(cleanFilename) {
		_ = removeUploadTempDir(uploadId)
		FailWithError(c, http.StatusBadRequest, "错误的文件类型，只支持vpk, zip, rar, 7z文件")
		return
	}
	mergedPath, mergeErr := mergeUploadChunks(c.Request.Context(), task, tempPath, cleanFilename, meta)
	if mergeErr != nil {
		if mergeErr.discard {
			task.stop()
			_ = removeUploadTempDir(uploadId)
		}
		mergeErr.respond(c)
		return
	}
	defer os.Remove(mergedPath)
	if err := middlewares.RevalidateAuth(c); err != nil {
		authCodeError(c, err)
		return
	}
	// Persist the import boundary before ProcessFile can install anything. If
	// cleanup fails (or the process exits), another request must not import the
	// same task again. The marker records an attempt, not a successful import;
	// interrupted attempts require a fresh task rather than automatic recovery.
	// File Sync does not provide a power-loss transaction with map installation.
	// This does not change the .meta-based expiry policy.
	markerPath := filepath.Join(tempPath, uploadImportMarker)
	marker, err := os.OpenFile(markerPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "记录上传任务导入状态失败，请重新上传: %v", err)
		return
	}
	markerErr := marker.Sync()
	markerErr = errors.Join(markerErr, marker.Close())
	if markerErr != nil {
		// ProcessFile has not run, so this attempt marker is safe to remove.
		if cleanupErr := os.Remove(markerPath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			markerErr = errors.Join(markerErr, cleanupErr)
		}
		FailWithError(c, http.StatusInternalServerError, "保存上传任务导入状态失败，请重新上传: %v", markerErr)
		return
	}
	if err := task.beginImport(c.Request.Context()); err != nil {
		// A live handler knows no import started; unlike an abandoned marker
		// after a crash, this must not permanently block a healthy upload.
		if cleanupErr := os.Remove(markerPath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			FailWithError(c, http.StatusInternalServerError, "清理未开始的导入标记失败，请重新上传: %v", cleanupErr)
			return
		}
		err.respond(c)
		return
	}
	files, processErr := ProcessFile(mergedPath)
	if processErr != nil {
		_ = removeUploadTempDir(uploadId)
		failMapImport(c, processErr)
		return
	}
	if err := removeUploadTempDir(uploadId); err != nil {
		FailWithError(c, http.StatusInternalServerError, "地图已导入成功，但清理上传临时文件失败，请刷新地图列表: %v", err)
		return
	}
	defer LogOp(c, fmt.Sprintf("分片上传合并完成: %s，文件数: %d", filename, len(files)))()
	c.String(http.StatusOK, "上传成功！")
	runtime.GC()
}

// UploadCancel 取消上传并清理临时文件
func UploadCancel(c *gin.Context) {
	uploadId := c.PostForm("uploadId")
	if uploadId == "" {
		FailWithError(c, http.StatusBadRequest, "缺少 uploadId 参数")
		return
	}
	if err := validateUploadId(uploadId); err != nil {
		FailWithError(c, http.StatusBadRequest, "%v", err)
		return
	}
	if !ensureUploadOwner(c, uploadId, true) {
		return
	}
	task, release := acquireUploadTask(uploadId)
	defer release()
	done := task.beginCancel()
	defer task.finishOperation(done)
	if err := removeUploadTempDir(uploadId); err != nil {
		FailWithError(c, http.StatusInternalServerError, "清理临时文件失败: %v", err)
		return
	}

	c.String(http.StatusOK, "已取消")
}

// StartChunkUploadCleaner 启动定时清理长期未活动的上传临时目录
func StartChunkUploadCleaner() {
	go func() {
		cleanStaleUploads()
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			cleanStaleUploads()
		}
	}()
}

func cleanStaleUploads() {
	basePath := filepath.Join(consts.AddonsBasePath, uploadTempDir)
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return
	}
	root, err := os.OpenRoot(consts.AddonsBasePath)
	if err != nil {
		return
	}
	defer root.Close()

	now := time.Now()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(basePath, entry.Name(), ".meta")
		info, err := os.Stat(metaPath)
		if err != nil {
			// 没有 .meta 文件的目录也清理掉
			_ = root.RemoveAll(filepath.Join(uploadTempDir, entry.Name()))
			continue
		}
		if now.Sub(info.ModTime()) > 6*time.Hour {
			_ = root.RemoveAll(filepath.Join(uploadTempDir, entry.Name()))
		}
	}
}
