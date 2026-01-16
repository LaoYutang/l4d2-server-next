package controller

import (
	"l4d2-manager-next/logic"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetAdmins(c *gin.Context) {
	admins, err := logic.ParseAdminsSimple()
	if err != nil {
		// Return 200 with error message in body if file missing, or 500
		// Requirement says: "if file not exists, prompt to enable sourcemod"
		// ParseAdminsSimple returns error for this.
		if err.Error() == "SourceMod 未启用或配置文件不存在" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, admins)
}

type AddAdminRequest struct {
	SteamID string `json:"steamid" binding:"required"`
	Remark  string `json:"remark"`
}

func AddAdmin(c *gin.Context) {
	// Check Permission
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，仅管理员可操作"})
		return
	}

	var req AddAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	if err := logic.AddAdmin(req.SteamID, req.Remark); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "添加成功"})
}

type DeleteAdminRequest struct {
	SteamID string `json:"steamid" binding:"required"`
}

func DeleteAdmin(c *gin.Context) {
	// Check Permission
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，仅管理员可操作"})
		return
	}

	var req DeleteAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	if err := logic.DeleteAdmin(req.SteamID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
