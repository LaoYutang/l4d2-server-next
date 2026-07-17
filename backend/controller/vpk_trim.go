package controller

import (
	"l4d2-manager-next/logic"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetVPKTrimConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enabled": logic.IsVPKTrimEnabled(),
	})
}

func SetVPKTrimConfig(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, http.StatusBadRequest, "参数错误")
		return
	}

	detail := "关闭地图自动精简"
	if req.Enable {
		detail = "开启地图自动精简"
	}
	defer LogOp(c, detail)()
	if err := logic.SetVPKTrimEnable(req.Enable); err != nil {
		FailWithError(c, http.StatusInternalServerError, "保存配置失败: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
