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
	} else if stat.UsedPercent > 90 {
		FailWithError(c, http.StatusInternalServerError, "磁盘空间不足，当前使用率超过90%%")
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

	// 处理zip文件
	if zipReg.Match([]byte(file.Filename)) {
		files, err := handleZipFile(c, file)
		if err != nil {
			FailWithError(c, http.StatusInternalServerError, "解压Zip失败: %v", err)
			return
		}
		LogOp(c, nil, "上传文件:", file.Filename, "解压文件:", fmt.Sprintf("%v", files))
		c.String(http.StatusOK, "上传并解压成功！")
		runtime.GC()
		return
	}

	// 处理rar文件
	if rarReg.Match([]byte(file.Filename)) {
		files, err := handleRarFile(c, file)
		if err != nil {
			FailWithError(c, http.StatusInternalServerError, "解压Rar失败: %v", err)
			return
		}
		LogOp(c, nil, "上传文件:", file.Filename, "解压文件:", fmt.Sprintf("%v", files))
		c.String(http.StatusOK, "上传并解压成功！")
		runtime.GC()
		return
	}

	// 处理7z文件
	if sevenZipReg.Match([]byte(file.Filename)) {
		files, err := handle7zFile(c, file)
		if err != nil {
			FailWithError(c, http.StatusInternalServerError, "解压7z失败: %v", err)
			return
		}
		LogOp(c, nil, "上传文件:", file.Filename, "解压文件:", fmt.Sprintf("%v", files))
		c.String(http.StatusOK, "上传并解压成功！")
		runtime.GC()
		return
	}

	// 处理vpk文件
	// 清理文件名
	cleanFilename := sanitizeFilename(file.Filename)

	// 检查文件是否已存在
	if err := checkMapExists(cleanFilename); err != nil {
		FailWithError(c, http.StatusBadRequest, "检查文件失败: %v", err)
		return
	}

	// 保存上传的文件
	tempPath := filepath.Join(consts.AddonsBasePath, "temp_"+cleanFilename)
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		FailWithError(c, http.StatusInternalServerError, "文件写入失败: %v", err)
		return
	}

	// 使用共用的文件处理方法
	files, err := ProcessVpkFile(tempPath)
	if err != nil {
		os.Remove(tempPath) // 清理临时文件
		FailWithError(c, http.StatusInternalServerError, "处理文件失败: %v", err)
		return
	}
	LogOp(c, nil, "上传文件:", file.Filename, "保存文件:", fmt.Sprintf("%v", files))

	c.String(http.StatusOK, "上传成功！")
	runtime.GC()
}

func handleZipFile(c *gin.Context, file *multipart.FileHeader) ([]string, error) {
	// 保存临时zip文件
	tempZipPath := filepath.Join(consts.AddonsBasePath, "temp_"+file.Filename)
	if err := c.SaveUploadedFile(file, tempZipPath); err != nil {
		return nil, err
	}
	defer os.Remove(tempZipPath) // 清理临时文件

	// 使用共用的zip文件处理方法
	return ProcessZipFile(tempZipPath)
}

func handleRarFile(c *gin.Context, file *multipart.FileHeader) ([]string, error) {
	// 保存临时rar文件
	tempRarPath := filepath.Join(consts.AddonsBasePath, "temp_"+file.Filename)
	if err := c.SaveUploadedFile(file, tempRarPath); err != nil {
		return nil, err
	}
	defer os.Remove(tempRarPath) // 清理临时文件

	// 使用共用的rar文件处理方法
	return ProcessRarFile(tempRarPath)
}

func handle7zFile(c *gin.Context, file *multipart.FileHeader) ([]string, error) {
	// 保存临时7z文件
	temp7zPath := filepath.Join(consts.AddonsBasePath, "temp_"+file.Filename)
	if err := c.SaveUploadedFile(file, temp7zPath); err != nil {
		return nil, err
	}
	defer os.Remove(temp7zPath) // 清理临时文件

	// 使用共用的7z文件处理方法
	return Process7zFile(temp7zPath)
}
