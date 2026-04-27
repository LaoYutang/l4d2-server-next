package controller

import (
	"fmt"
	"l4d2-manager-next/logic"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetPlugins(c *gin.Context) {
	plugins, err := logic.GetPlugins()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "获取插件列表失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, plugins)
}

func UploadPlugin(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		if err = c.Request.ParseMultipartForm(32 << 20); err != nil {
			FailWithError(c, http.StatusBadRequest, "解析表单失败: %v", err)
			return
		}
		form = c.Request.MultipartForm
	}

	files := form.File["file"]
	if len(files) == 0 {
		FailWithError(c, http.StatusBadRequest, "未上传文件")
		return
	}

	var filenames []string
	for _, header := range files {
		filenames = append(filenames, header.Filename)
	}
	LogOp(c, nil, "上传插件:", strings.Join(filenames, ", "))

	var errs []string
	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s 打开失败: %v", header.Filename, err))
			continue
		}

		if err := logic.UploadPlugin(file, header.Size, header.Filename); err != nil {
			errs = append(errs, fmt.Sprintf("%s 上传失败: %v", header.Filename, err))
		}
		file.Close()
	}

	if len(errs) > 0 {
		FailWithError(c, http.StatusInternalServerError, "%s", strings.Join(errs, "; "))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "插件上传成功"})
}

type GetStorePluginsRequest struct {
	ForceRefresh bool   `json:"force_refresh"`
	ProxyUrl     string `json:"proxy_url"`
	GithubToken  string `json:"github_token"`
	Repo         string `json:"repo"`
}

func GetStorePlugins(c *gin.Context) {
	var req GetStorePluginsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// allow empty body
	}

	plugins, err := logic.FetchStorePlugins(req.ForceRefresh, req.ProxyUrl, req.GithubToken, req.Repo)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "获取插件商店列表失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, plugins)
}

type DownloadPluginRequest struct {
	Name        string `json:"name"`
	ProxyUrl    string `json:"proxy_url"`
	GithubToken string `json:"github_token"`
	Repo        string `json:"repo"`
}

func DownloadStorePlugin(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	var req DownloadPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, http.StatusBadRequest, "参数错误: %v", err)
		return
	}

	if req.Name == "" {
		FailWithError(c, http.StatusBadRequest, "插件名称不能为空")
		return
	}

	LogOp(c, req, "从商店下载插件:", req.Name)

	if err := logic.DownloadStorePlugin(req.Name, req.ProxyUrl, req.GithubToken, req.Repo); err != nil {
		FailWithError(c, http.StatusInternalServerError, "下载插件失败: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "插件下载成功"})
}

func EnablePlugin(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	name := c.PostForm("name")
	if name == "" {
		FailWithError(c, http.StatusBadRequest, "插件名称不能为空")
		return
	}
	LogOp(c, nil, "启用插件:", name)

	if err := logic.EnablePlugin(name); err != nil {
		FailWithError(c, http.StatusInternalServerError, "启用插件失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "插件启用成功"})
}

func DisablePlugin(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	name := c.PostForm("name")
	if name == "" {
		FailWithError(c, http.StatusBadRequest, "插件名称不能为空")
		return
	}
	LogOp(c, nil, "禁用插件:", name)

	if err := logic.DisablePlugin(name); err != nil {
		FailWithError(c, http.StatusInternalServerError, "禁用插件失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "插件禁用成功"})
}

func DeletePlugin(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	name := c.PostForm("name")
	if name == "" {
		FailWithError(c, http.StatusBadRequest, "插件名称不能为空")
		return
	}
	LogOp(c, nil, "删除插件:", name)

	if err := logic.DeletePlugin(name); err != nil {
		FailWithError(c, http.StatusInternalServerError, "删除插件失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "插件删除成功"})
}

type BatchPluginRequest struct {
	Names []string `json:"names"`
}

func EnablePlugins(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	var req BatchPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, http.StatusBadRequest, "无效的请求格式")
		return
	}
	LogOp(c, req, "批量启用插件")

	if len(req.Names) == 0 {
		FailWithError(c, http.StatusBadRequest, "插件列表不能为空")
		return
	}

	if err := logic.EnablePlugins(req.Names); err != nil {
		FailWithError(c, http.StatusInternalServerError, "批量启用插件失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "批量启用成功"})
}

func DisablePlugins(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	var req BatchPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, http.StatusBadRequest, "无效的请求格式")
		return
	}
	LogOp(c, req, "批量禁用插件")

	if len(req.Names) == 0 {
		FailWithError(c, http.StatusBadRequest, "插件列表不能为空")
		return
	}

	if err := logic.DisablePlugins(req.Names); err != nil {
		FailWithError(c, http.StatusInternalServerError, "批量禁用插件失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "批量禁用成功"})
}

func GetPresets(c *gin.Context) {
	presets, err := logic.GetPresets()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "获取预设列表失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, presets)
}

func ApplyPreset(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	name := c.PostForm("name")
	if name == "" {
		FailWithError(c, http.StatusBadRequest, "预设名称不能为空")
		return
	}
	LogOp(c, nil, "应用插件预设:", name)

	if err := logic.ApplyPreset(name); err != nil {
		FailWithError(c, http.StatusInternalServerError, "%v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "预设应用成功"})
}
