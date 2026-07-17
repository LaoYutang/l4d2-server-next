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
			FailWithError(c, http.StatusNotFound, "%s", err.Error())
			return
		}
		FailWithError(c, http.StatusInternalServerError, "获取管理员列表失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, admins)
}

type AddAdminRequest struct {
	SteamID string `json:"steamid"`
	Remark  string `json:"remark"`
}

func reloadAdmins() error {
	conn, err := getRconConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Execute("sm_reloadadmins")
	return err
}

func AddAdmin(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	var req AddAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, http.StatusBadRequest, "无效的请求格式")
		return
	}
	defer LogOp(c, "添加管理员: "+req.SteamID)()

	if req.SteamID == "" {
		FailWithError(c, http.StatusBadRequest, "SteamID 不能为空")
		return
	}

	if err := logic.AddAdmin(req.SteamID, req.Remark); err != nil {
		FailWithError(c, http.StatusInternalServerError, "添加管理员失败: %v", err)
		return
	}

	if reloadErr := reloadAdmins(); reloadErr != nil {
		c.JSON(http.StatusOK, gin.H{"message": "添加成功", "reload": false, "reload_error": reloadErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "添加成功", "reload": true})
}

type DeleteAdminRequest struct {
	SteamID string `json:"steamid"`
}

func DeleteAdmin(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	var req DeleteAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, http.StatusBadRequest, "无效的请求格式")
		return
	}
	defer LogOp(c, "删除管理员: "+req.SteamID)()

	if req.SteamID == "" {
		FailWithError(c, http.StatusBadRequest, "SteamID 不能为空")
		return
	}

	if err := logic.DeleteAdmin(req.SteamID); err != nil {
		FailWithError(c, http.StatusInternalServerError, "删除管理员失败: %v", err)
		return
	}

	if reloadErr := reloadAdmins(); reloadErr != nil {
		c.JSON(http.StatusOK, gin.H{"message": "删除成功", "reload": false, "reload_error": reloadErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功", "reload": true})
}
