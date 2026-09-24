package controller

import (
	"fmt"
	"l4d2-manager-next/consts"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/disk"
)

func Upload(c *gin.Context) {
	if stat, err := disk.Usage(consts.AddonsBasePath); err != nil {
		FailWithError(c, http.StatusInternalServerError, "获取磁盘使用信息失败: %v", err)
		return
	} else if stat.UsedPercent > 90 && c.PostForm("ignoreDiskWarning") != "true" {
		FailWithError(c, http.StatusInsufficientStorage, "磁盘空间不足，当前使用率超过90%%，是否继续上传")
		return
	}

	file, err := c.FormFile("map")
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "文件信息有误")
		return
	}

	vpkReg := regexp.MustCompile(`(?i)\.(vpk|zip|rar|7z)$`)
	zipReg := regexp.MustCompile(`(?i)\.zip$`)
	rarReg := regexp.MustCompile(`(?i)\.rar$`)
	sevenZipReg := regexp.MustCompile(`(?i)\.7z$`)

	if !vpkReg.Match([]byte(file.Filename)) {
		FailWithError(c, http.StatusBadRequest, "错误的文件类型，只支持vpk, zip, rar, 7z文件")
		return
	}

	if file.Size > 2<<30 {
		FailWithError(c, http.StatusBadRequest, "文件超过2GB，禁止上传")
		return
	}

	if zipReg.Match([]byte(file.Filename)) {
		files, err := handleZipFile(c, file)
		if err != nil {
			FailWithError(c, http.StatusInternalServerError, "解压Zip失败: %v", err)
			return
		}
		defer LogOp(c, fmt.Sprintf("上传文件: %s，解压文件数: %d", file.Filename, len(files)))()
		c.String(http.StatusOK, "上传并解压成功！")
		runtime.GC()
		return
	}

	if rarReg.Match([]byte(file.Filename)) {
		files, err := handleRarFile(c, file)
		if err != nil {
			FailWithError(c, http.StatusInternalServerError, "解压Rar失败: %v", err)
			return
		}
		defer LogOp(c, fmt.Sprintf("上传文件: %s，解压文件数: %d", file.Filename, len(files)))()
		c.String(http.StatusOK, "上传并解压成功！")
		runtime.GC()
		return
	}

	if sevenZipReg.Match([]byte(file.Filename)) {
		files, err := handle7zFile(c, file)
		if err != nil {
			FailWithError(c, http.StatusInternalServerError, "解压7z失败: %v", err)
			return
		}
		defer LogOp(c, fmt.Sprintf("上传文件: %s，解压文件数: %d", file.Filename, len(files)))()
		c.String(http.StatusOK, "上传并解压成功！")
		runtime.GC()
		return
	}

	cleanFilename := sanitizeFilename(file.Filename)
	if err := checkMapExists(cleanFilename); err != nil {
		FailWithError(c, http.StatusBadRequest, "检查文件失败: %v", err)
		return
	}

	tempPath := filepath.Join(consts.AddonsBasePath, "temp_"+cleanFilename)
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		FailWithError(c, http.StatusInternalServerError, "文件写入失败: %v", err)
		return
	}

	files, err := ProcessVpkFile(tempPath)
	if err != nil {
		os.Remove(tempPath)
		FailWithError(c, http.StatusInternalServerError, "处理文件失败: %v", err)
		return
	}
	defer LogOp(c, fmt.Sprintf("上传文件: %s，保存文件数: %d", file.Filename, len(files)))()

	c.String(http.StatusOK, "上传成功！")
	runtime.GC()
}

func handleZipFile(c *gin.Context, file *multipart.FileHeader) ([]string, error) {
	tempZipPath := filepath.Join(consts.AddonsBasePath, "temp_"+file.Filename)
	if err := c.SaveUploadedFile(file, tempZipPath); err != nil {
		return nil, err
	}
	defer os.Remove(tempZipPath)
	return ProcessZipFile(tempZipPath)
}

func handleRarFile(c *gin.Context, file *multipart.FileHeader) ([]string, error) {
	tempRarPath := filepath.Join(consts.AddonsBasePath, "temp_"+file.Filename)
	if err := c.SaveUploadedFile(file, tempRarPath); err != nil {
		return nil, err
	}
	defer os.Remove(tempRarPath)
	return ProcessRarFile(tempRarPath)
}

func handle7zFile(c *gin.Context, file *multipart.FileHeader) ([]string, error) {
	temp7zPath := filepath.Join(consts.AddonsBasePath, "temp_"+file.Filename)
	if err := c.SaveUploadedFile(file, temp7zPath); err != nil {
		return nil, err
	}
	defer os.Remove(temp7zPath)
	return Process7zFile(temp7zPath)
}
