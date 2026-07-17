package controller

import (
	"fmt"
	"l4d2-manager-next/consts"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

type ServerConfigResponse struct {
	Hidden           bool     `json:"hidden"`
	LobbyConnectOnly bool     `json:"lobby_connect_only"`
	SteamGroup       string   `json:"steam_group"`
	CustomConfig     []string `json:"custom_config"`
}

type UpdateServerConfigRequest struct {
	Hidden           bool     `json:"hidden"`
	LobbyConnectOnly bool     `json:"lobby_connect_only"`
	SteamGroup       string   `json:"steam_group"`
	CustomConfig     []string `json:"custom_config"`
}

const CustomConfigMarker = "// [L4D2-MANAGER-CUSTOM]"

func GetServerConfig(c *gin.Context) {
	configPath := filepath.Join(consts.GamePath, "cfg", "server.cfg")

	resp := ServerConfigResponse{
		CustomConfig: []string{},
	}

	// Read file
	contentBytes, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, resp)
			return
		}
		c.String(http.StatusInternalServerError, "读取配置文件失败: %v", err)
		return
	}

	content := string(contentBytes)
	lines := strings.Split(content, "\n")

	inCustomBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(line, CustomConfigMarker) {
			inCustomBlock = true
			continue
		}

		// Detect managed fields (even if inside CustomBlock in future)
		isManaged := false
		if strings.HasPrefix(trimmed, "sv_tags") {
			isManaged = true
			// Check if "hidden" is in the tags
			args := strings.TrimSpace(strings.TrimPrefix(trimmed, "sv_tags"))
			args = strings.Trim(args, "\"")
			if strings.Contains(args, "hidden") {
				resp.Hidden = true
			}
		} else if strings.HasPrefix(trimmed, "sm_cvar") && strings.Contains(trimmed, "sv_allow_lobby_connect_only") {
			isManaged = true
			// sm_cvar sv_allow_lobby_connect_only "0"
			// Extract value
			re := regexp.MustCompile(`"(\d+)"`)
			matches := re.FindStringSubmatch(trimmed)
			if len(matches) > 1 {
				val := matches[1]
				if val == "1" {
					resp.LobbyConnectOnly = true // "Enable Matching" = 1
				} else {
					resp.LobbyConnectOnly = false
				}
			}
		} else if strings.HasPrefix(trimmed, "sv_steamgroup") {
			isManaged = true
			re := regexp.MustCompile(`"(\d+)"`)
			matches := re.FindStringSubmatch(trimmed)
			if len(matches) > 1 {
				resp.SteamGroup = matches[1]
			}
		}

		if inCustomBlock {
			if trimmed != "" && !isManaged {
				resp.CustomConfig = append(resp.CustomConfig, line)
			}
			continue
		}
	}

	c.JSON(http.StatusOK, resp)
}

func UpdateServerConfig(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return
	}

	var req UpdateServerConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, http.StatusBadRequest, "请求参数错误: %v", err)
		return
	}
	defer LogOp(c, "更新服务器配置")()

	// Update main config
	mainConfigPath := filepath.Join(consts.GamePath, "cfg", "server.cfg")
	if err := updateConfigFile(mainConfigPath, req); err != nil {
		FailWithError(c, http.StatusInternalServerError, "保存 server.cfg 失败: %v", err)
		return
	}

	// Sync to other files independently (preserving their unique top content)
	syncFiles := []string{"server.cfg.128tick", "server.cfg.100tick", "server.cfg.60tick", "server.cfg.30tick"}
	for _, fname := range syncFiles {
		fpath := filepath.Join(consts.GamePath, "cfg", fname)
		if _, err := os.Stat(fpath); err == nil {
			// Ignore errors for sync files, just log/continue
			updateConfigFile(fpath, req)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "保存成功"})
}

func updateConfigFile(configPath string, req UpdateServerConfigRequest) error {
	// Read existing file to preserve other settings
	contentBytes, err := os.ReadFile(configPath)
	var lines []string
	if err == nil {
		lines = strings.Split(string(contentBytes), "\n")
	}

	// Pass 1: Extract original tags from ALL lines of THIS file
	originalTags := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "sv_tags") {
			args := strings.TrimSpace(strings.TrimPrefix(trimmed, "sv_tags"))
			args = strings.Trim(args, "\"")
			if args != "" {
				parts := strings.Split(args, ",")
				for _, p := range parts {
					t := strings.TrimSpace(p)
					if t != "" && t != "hidden" {
						originalTags = append(originalTags, t)
					}
				}
			}
		}
	}

	// Pass 2: Build new content
	// We preserve everything ABOVE the marker (excluding managed fields)
	var newLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(line, CustomConfigMarker) {
			break // Stop reading at marker
		}

		// Skip lines we are managing
		if strings.HasPrefix(trimmed, "sv_tags") {
			continue
		}
		if strings.HasPrefix(trimmed, "sm_cvar") && strings.Contains(trimmed, "sv_allow_lobby_connect_only") {
			continue
		}
		if strings.HasPrefix(trimmed, "sv_steamgroup") {
			continue
		}

		newLines = append(newLines, line)
	}

	// Remove trailing empty lines from preserved content
	for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}

	// Append Custom Config Marker
	newLines = append(newLines, "")
	newLines = append(newLines, CustomConfigMarker)

	// Managed Fields (now below marker)

	// sv_tags
	if req.Hidden {
		originalTags = append(originalTags, "hidden")
	}

	if len(originalTags) > 0 {
		// Deduplicate
		uniqueTags := make([]string, 0, len(originalTags))
		seen := make(map[string]bool)
		for _, t := range originalTags {
			if !seen[t] {
				uniqueTags = append(uniqueTags, t)
				seen[t] = true
			}
		}
		newLines = append(newLines, fmt.Sprintf("sv_tags \"%s\"", strings.Join(uniqueTags, ",")))
	}

	// Lobby Connect
	val := "0"
	if req.LobbyConnectOnly {
		val = "1"
	}
	newLines = append(newLines, fmt.Sprintf("sm_cvar sv_allow_lobby_connect_only \"%s\"", val))

	// Steam Group
	if req.SteamGroup != "" {
		newLines = append(newLines, fmt.Sprintf("sv_steamgroup \"%s\"", req.SteamGroup))
	}

	// User Custom Config
	for _, customLine := range req.CustomConfig {
		if strings.TrimSpace(customLine) != "" {
			newLines = append(newLines, customLine)
		}
	}

	finalContent := strings.Join(newLines, "\n")

	// Write file
	return os.WriteFile(configPath, []byte(finalContent), 0644)
}
