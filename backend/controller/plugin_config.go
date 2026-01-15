package controller

import (
	"l4d2-manager-next/logic"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetPluginConfig(c *gin.Context) {
	pluginName := c.PostForm("name")
	if pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plugin name is required"})
		return
	}

	configs, err := logic.GetPluginConfigs(pluginName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, configs)
}

type UpdateConfigRequest struct {
	ConfigName string            `json:"config_name"`
	Updates    map[string]string `json:"updates"`
}

func UpdatePluginConfig(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}

	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := logic.SavePluginConfig(req.ConfigName, req.Updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}
