package controller

import (
	"l4d2-manager-next/consts"
	"net/http"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/axgle/mahonia"
	"github.com/gin-gonic/gin"
)

type ServerInfoResponse struct {
	Hostname      string `json:"hostname"`
	HostnameError string `json:"hostname_error"`
	Motd          string `json:"motd"`
	Host          string `json:"host"`
}

type UpdateServerInfoRequest struct {
	Hostname string `json:"hostname"`
	Motd     string `json:"motd"`
	Host     string `json:"host"`
}

func readFileContent(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Detect encoding
	if utf8.Valid(content) {
		return string(content), nil
	}

	// Try GBK
	decoder := mahonia.NewDecoder("gbk")
	return decoder.ConvertString(string(content)), nil
}

func writeFileContent(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func GetServerInfo(c *gin.Context) {
	hostnamePath := filepath.Join(consts.GamePath, "addons", "sourcemod", "configs", "l4d2_hostname.txt")
	motdPath := filepath.Join(consts.GamePath, "motd.txt")
	hostPath := filepath.Join(consts.GamePath, "host.txt")

	resp := ServerInfoResponse{}

	// Hostname
	if _, err := os.Stat(hostnamePath); os.IsNotExist(err) {
		resp.HostnameError = "请先在插件管理中启用服务器中文名插件"
	} else {
		content, err := readFileContent(hostnamePath)
		if err != nil {
			resp.HostnameError = "读取文件失败: " + err.Error()
		} else {
			resp.Hostname = content
		}
	}

	// Motd
	if content, err := readFileContent(motdPath); err == nil {
		resp.Motd = content
	} else {
		// If file doesn't exist or error, just return empty, maybe create it later on save
	}

	// Host
	if content, err := readFileContent(hostPath); err == nil {
		resp.Host = content
	}

	c.JSON(http.StatusOK, resp)
}

func UpdateServerInfo(c *gin.Context) {
	var req UpdateServerInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hostnamePath := filepath.Join(consts.GamePath, "addons", "sourcemod", "configs", "l4d2_hostname.txt")
	motdPath := filepath.Join(consts.GamePath, "motd.txt")
	hostPath := filepath.Join(consts.GamePath, "host.txt")

	// Update Hostname if file exists
	if _, err := os.Stat(hostnamePath); err == nil {
		if err := writeFileContent(hostnamePath, req.Hostname); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存服务器名失败: " + err.Error()})
			return
		}
	}

	// For Motd and Host, we create them if they don't exist
	if err := writeFileContent(motdPath, req.Motd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存公告失败: " + err.Error()})
		return
	}

	if err := writeFileContent(hostPath, req.Host); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存标题失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "保存成功"})
}
