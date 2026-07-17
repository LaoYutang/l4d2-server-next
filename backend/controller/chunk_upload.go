package controller

import (
	"fmt"
	"io"
	"l4d2-manager-next/consts"
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
	"github.com/shirou/gopsutil/v3/disk"
)

const (
	uploadTempDir      = "upload_temp"
	maxUploadChunkSize = 6 << 20
)

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

func removeUploadTempDir(uploadId string) error {
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
	if stat, err := disk.Usage(consts.AddonsBasePath); err != nil {
		FailWithError(c, http.StatusInternalServerError, "获取磁盘使用信息失败: %v", err)
		return
	} else if stat.UsedPercent > 90 {
		FailWithError(c, http.StatusInternalServerError, "磁盘空间不足，当前使用率超过90%%")
		return
	}

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
	if err != nil || totalChunks <= 0 {
		FailWithError(c, http.StatusBadRequest, "totalChunks 参数无效")
		return
	}

	if fileSize > 2<<30 {
		FailWithError(c, http.StatusBadRequest, "文件超过2GB，禁止上传")
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

	defer LogOp(c, fmt.Sprintf(
		"初始化分片上传: %s，大小: %d，分片数: %d，uploadId: %s",
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

	tempPath := getUploadTempPath(uploadId)
	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		FailWithError(c, http.StatusBadRequest, "上传任务不存在或已过期")
		return
	}

	// 读取元信息校验分片大小
	metaPath := filepath.Join(tempPath, ".meta")
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "上传任务不存在或已过期")
		return
	}
	metaLines := strings.Split(strings.TrimSpace(string(metaBytes)), "\n")
	if len(metaLines) >= 3 {
		fileSize, _ := strconv.ParseInt(metaLines[1], 10, 64)
		totalChunks, _ := strconv.Atoi(metaLines[2])
		if fileSize > 0 && totalChunks > 0 {
			chunkFile, err := c.FormFile("chunk")
			if err != nil {
				FailWithError(c, http.StatusBadRequest, "分片文件读取失败: %v", err)
				return
			}
			if chunkFile.Size > maxUploadChunkSize {
				FailWithError(c, http.StatusBadRequest, "分片大小超过限制 (%d > %d)", chunkFile.Size, maxUploadChunkSize)
				return
			}
			chunkPath := filepath.Join(tempPath, strconv.Itoa(chunkIndex))
			tmpPath := chunkPath + ".tmp"
			if err := c.SaveUploadedFile(chunkFile, tmpPath); err != nil {
				os.Remove(tmpPath)
				FailWithError(c, http.StatusInternalServerError, "保存分片失败: %v", err)
				return
			}
			if err := os.Rename(tmpPath, chunkPath); err != nil {
				os.Remove(tmpPath)
				FailWithError(c, http.StatusInternalServerError, "确认分片失败: %v", err)
				return
			}
			c.String(http.StatusOK, "OK")
			return
		}
	}

	FailWithError(c, http.StatusInternalServerError, "元信息解析失败")
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
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == ".meta" || strings.HasSuffix(name, ".tmp") {
			continue
		}
		if idx, err := strconv.Atoi(name); err == nil {
			uploadedChunks = append(uploadedChunks, idx)
		}
	}

	sort.Ints(uploadedChunks)
	c.JSON(http.StatusOK, gin.H{"uploadedChunks": uploadedChunks})
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

	tempPath := getUploadTempPath(uploadId)
	metaPath := filepath.Join(tempPath, ".meta")

	// 读取元信息
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "上传任务不存在或已过期")
		return
	}

	metaLines := strings.Split(strings.TrimSpace(string(metaBytes)), "\n")
	if len(metaLines) < 3 {
		FailWithError(c, http.StatusInternalServerError, "元信息格式错误")
		return
	}

	totalChunks, err := strconv.Atoi(metaLines[2])
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "元信息解析失败")
		return
	}

	// 检查所有分片是否已上传
	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(tempPath, strconv.Itoa(i))
		if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
			FailWithError(c, http.StatusBadRequest, "分片 %d 缺失，无法合并", i)
			return
		}
	}

	// 合并分片
	cleanFilename := sanitizeFilename(filename)
	mergedPath := filepath.Join(tempPath, "merged_"+cleanFilename)

	mergedFile, err := os.Create(mergedPath)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "创建合并文件失败: %v", err)
		return
	}

	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(tempPath, strconv.Itoa(i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			mergedFile.Close()
			FailWithError(c, http.StatusInternalServerError, "打开分片 %d 失败: %v", i, err)
			return
		}
		_, err = io.Copy(mergedFile, chunkFile)
		chunkFile.Close()
		if err != nil {
			mergedFile.Close()
			FailWithError(c, http.StatusInternalServerError, "合并分片 %d 失败: %v", i, err)
			return
		}
	}
	mergedFile.Close()

	// 校验合并后的文件大小
	mergedInfo, err := os.Stat(mergedPath)
	if err != nil {
		_ = removeUploadTempDir(uploadId)
		FailWithError(c, http.StatusInternalServerError, "读取合并文件信息失败: %v", err)
		return
	}
	expectedSize, _ := strconv.ParseInt(metaLines[1], 10, 64)
	if expectedSize > 0 && mergedInfo.Size() != expectedSize {
		_ = removeUploadTempDir(uploadId)
		FailWithError(c, http.StatusInternalServerError, "合并文件大小不匹配 (期望 %d, 实际 %d)", expectedSize, mergedInfo.Size())
		return
	}

	// 处理合并后的文件
	var files []string
	var processErr error

	vpkReg := regexp.MustCompile(`(?i)\.(vpk|zip|rar|7z)$`)
	zipReg := regexp.MustCompile(`(?i)\.zip$`)
	rarReg := regexp.MustCompile(`(?i)\.rar$`)
	sevenZipReg := regexp.MustCompile(`(?i)\.7z$`)

	if !vpkReg.MatchString(cleanFilename) {
		_ = removeUploadTempDir(uploadId)
		FailWithError(c, http.StatusBadRequest, "错误的文件类型，只支持vpk, zip, rar, 7z文件")
		return
	}

	switch {
	case zipReg.MatchString(cleanFilename):
		files, processErr = ProcessZipFile(mergedPath)
	case rarReg.MatchString(cleanFilename):
		files, processErr = ProcessRarFile(mergedPath)
	case sevenZipReg.MatchString(cleanFilename):
		files, processErr = Process7zFile(mergedPath)
	default:
		// vpk 文件
		files, processErr = ProcessVpkFile(mergedPath)
	}

	if processErr != nil {
		_ = removeUploadTempDir(uploadId)
		FailWithError(c, http.StatusInternalServerError, "处理文件失败: %v", processErr)
		return
	}

	// 清理临时目录
	if err := removeUploadTempDir(uploadId); err != nil {
		FailWithError(c, http.StatusInternalServerError, "清理临时文件失败: %v", err)
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
