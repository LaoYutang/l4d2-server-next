package controller

import (
	"fmt"
	"l4d2-manager-next/logic"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ListBackups(c *gin.Context) {
	backups, err := logic.ListBackups()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "获取备份列表失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, backups)
}

func CreateBackup(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	name := c.PostForm("name")
	if name == "" {
		FailWithError(c, http.StatusBadRequest, "备份名称不能为空")
		return
	}
	defer LogOp(c, "创建插件备份: "+name)()

	if err := logic.CreateBackup(name); err != nil {
		FailWithError(c, http.StatusInternalServerError, "创建备份失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "备份创建成功"})
}

func RestoreBackup(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	name := c.PostForm("name")
	if name == "" {
		FailWithError(c, http.StatusBadRequest, "备份名称不能为空")
		return
	}
	defer LogOp(c, "还原插件备份: "+name)()

	result, err := logic.RestoreBackup(name)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "还原备份失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "备份还原成功", "skipped": result.Skipped})
}

type RenameBackupRequest struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

func RenameBackup(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	var req RenameBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.OldName == "" || req.NewName == "" {
		FailWithError(c, http.StatusBadRequest, "备份名称不能为空")
		return
	}
	defer LogOp(c, fmt.Sprintf("重命名插件备份: %s -> %s", req.OldName, req.NewName))()

	if err := logic.RenameBackup(req.OldName, req.NewName); err != nil {
		FailWithError(c, http.StatusInternalServerError, "重命名备份失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "备份重命名成功"})
}

func DeleteBackup(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	name := c.PostForm("name")
	if name == "" {
		FailWithError(c, http.StatusBadRequest, "备份名称不能为空")
		return
	}
	defer LogOp(c, "删除插件备份: "+name)()

	if err := logic.DeleteBackup(name); err != nil {
		FailWithError(c, http.StatusInternalServerError, "删除备份失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "备份删除成功"})
}

func GetBackupPluginsDetail(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		FailWithError(c, http.StatusBadRequest, "备份名称不能为空")
		return
	}
	plugins, err := logic.GetBackupPluginsDetail(name)
	if err != nil {
		FailWithError(c, http.StatusNotFound, "获取备份插件详情失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, plugins)
}

func GetBackupAdminsDetail(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		FailWithError(c, http.StatusBadRequest, "备份名称不能为空")
		return
	}
	admins, err := logic.GetBackupAdminsDetail(name)
	if err != nil {
		FailWithError(c, http.StatusNotFound, "获取备份管理员详情失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, admins)
}

func GetBackupServerInfoDetail(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		FailWithError(c, http.StatusBadRequest, "备份名称不能为空")
		return
	}
	info, err := logic.GetBackupServerInfoDetail(name)
	if err != nil {
		FailWithError(c, http.StatusNotFound, "获取备份服务器信息失败: %v", err)
		return
	}
	if info == nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	c.JSON(http.StatusOK, info)
}

func GetBackupServerConfigDetail(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		FailWithError(c, http.StatusBadRequest, "备份名称不能为空")
		return
	}
	cfg, err := logic.GetBackupServerConfigDetail(name)
	if err != nil {
		FailWithError(c, http.StatusNotFound, "获取备份服务器配置失败: %v", err)
		return
	}
	if cfg == nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

func ExportBackup(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		FailWithError(c, http.StatusBadRequest, "备份名称不能为空")
		return
	}
	data, err := logic.ExportBackup(name)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "导出备份失败: %v", err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.yaml\"", name))
	c.Data(http.StatusOK, "application/x-yaml", data)
}

func ExportAllBackups(c *gin.Context) {
	data, err := logic.ExportAllBackups()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "导出备份失败: %v", err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=\"backups_all.yaml\"")
	c.Data(http.StatusOK, "application/x-yaml", data)
}

func ImportBackup(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		FailWithError(c, http.StatusBadRequest, "请选择文件")
		return
	}
	defer LogOp(c, "导入备份文件: "+file.Filename)()

	f, err := file.Open()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "读取文件失败: %v", err)
		return
	}
	defer f.Close()

	data := make([]byte, file.Size)
	if _, err := f.Read(data); err != nil {
		FailWithError(c, http.StatusInternalServerError, "读取文件失败: %v", err)
		return
	}

	count, err := logic.ImportBackup(data)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "导入备份失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("成功导入 %d 个备份", count), "count": count})
}
