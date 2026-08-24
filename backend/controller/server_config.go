package controller

import (
	"errors"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"log"
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
	FixedConfig      string   `json:"fixed_config"`
}

type UpdateServerConfigRequest struct {
	Hidden           bool     `json:"hidden"`
	LobbyConnectOnly bool     `json:"lobby_connect_only"`
	SteamGroup       string   `json:"steam_group"`
	CustomConfig     []string `json:"custom_config"`
}

const CustomConfigMarker = logic.ServerCustomConfigMarker

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
	resp.CustomConfig = logic.ExtractServerCustomConfig(content)
	resp.FixedConfig = logic.ExtractRedactedServerFixedConfig(content)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "sv_tags") {
			// Check if "hidden" is in the tags
			args := strings.TrimSpace(strings.TrimPrefix(trimmed, "sv_tags"))
			args = strings.Trim(args, "\"")
			for _, tag := range strings.Split(args, ",") {
				if strings.EqualFold(strings.TrimSpace(tag), "hidden") {
					resp.Hidden = true
					break
				}
			}
		} else if strings.HasPrefix(trimmed, "sm_cvar") && strings.Contains(trimmed, "sv_allow_lobby_connect_only") {
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
			re := regexp.MustCompile(`"(\d+)"`)
			matches := re.FindStringSubmatch(trimmed)
			if len(matches) > 1 {
				resp.SteamGroup = matches[1]
			}
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
	normalizedCustomConfig, err := logic.NormalizeServerCustomConfig(req.CustomConfig)
	if err != nil {
		if errors.Is(err, logic.ErrUnboundServerConfigComment) {
			FailWithError(c, http.StatusBadRequest, "%v", err)
			return
		}
		FailWithError(c, http.StatusBadRequest, "自定义配置无效: %v", err)
		return
	}
	req.CustomConfig = normalizedCustomConfig
	settings := logic.ServerConfigSettings{
		Hidden:           req.Hidden,
		LobbyConnectOnly: req.LobbyConnectOnly,
		SteamGroup:       req.SteamGroup,
		CustomConfig:     req.CustomConfig,
	}

	// Update main config
	mainConfigPath := filepath.Join(consts.GamePath, "cfg", "server.cfg")
	if err := logic.UpdateServerConfigFile(mainConfigPath, settings); err != nil {
		FailWithError(c, http.StatusInternalServerError, "保存 server.cfg 失败: %v", err)
		return
	}

	// Sync to other files independently (preserving their unique top content)
	syncFiles := []string{"server.cfg.128tick", "server.cfg.100tick", "server.cfg.60tick", "server.cfg.30tick"}
	for _, fname := range syncFiles {
		fpath := filepath.Join(consts.GamePath, "cfg", fname)
		if _, err := os.Stat(fpath); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("failed to inspect server config %s: %v", fname, err)
			}
			continue
		}
		if err := logic.UpdateServerConfigFile(fpath, settings); err != nil {
			log.Printf("failed to sync server config %s: %v", fname, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "保存成功"})
}
