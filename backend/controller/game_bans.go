package controller

import (
	"errors"
	"fmt"
	"l4d2-manager-next/logic"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type gameBanAddRequest struct {
	Kind            string `json:"kind"`
	Value           string `json:"value"`
	DurationMinutes int    `json:"duration_minutes"`
}

type gameBanRemoveRequest struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

var (
	listGameBans  = logic.ListGameBans
	addGameBan    = logic.AddGameBan
	removeGameBan = logic.RemoveGameBan
)

func ListGameBans(c *gin.Context) {
	if !requireAccessControlAdmin(c) {
		return
	}

	result, err := listGameBans()
	if err != nil {
		writeGameBanError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func AddGameBan(c *gin.Context) {
	if !requireAccessControlAdmin(c) {
		return
	}

	var request gameBanAddRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "参数错误"})
		return
	}
	canonical, err := logic.NormalizeGameBanValue(request.Kind, request.Value)
	if err != nil {
		writeGameBanError(c, err)
		return
	}
	duration := "永久"
	if request.DurationMinutes > 0 {
		duration = fmt.Sprintf("%d 分钟", request.DurationMinutes)
	}
	defer LogOp(c, fmt.Sprintf("添加游戏黑名单，类型: %s，值: %s，时长: %s", request.Kind, canonical, duration))()

	result, err := addGameBan(logic.GameBanChange{
		Kind:            request.Kind,
		Value:           canonical,
		DurationMinutes: request.DurationMinutes,
	})
	if err != nil {
		writeGameBanError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func RemoveGameBan(c *gin.Context) {
	if !requireAccessControlAdmin(c) {
		return
	}

	var request gameBanRemoveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "参数错误"})
		return
	}
	canonical, err := logic.NormalizeGameBanValue(request.Kind, request.Value)
	if err != nil {
		writeGameBanError(c, err)
		return
	}
	defer LogOp(c, fmt.Sprintf("删除游戏黑名单，类型: %s，值: %s", request.Kind, canonical))()

	result, err := removeGameBan(request.Kind, canonical)
	if err != nil {
		writeGameBanError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func writeGameBanError(c *gin.Context, err error) {
	message := gameBanErrorMessage(err)
	switch {
	case errors.Is(err, logic.ErrGameBanInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_game_ban",
			"message": message,
		})
	case errors.Is(err, logic.ErrGameBanDuplicate):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "game_ban_exists",
			"message": message,
		})
	case errors.Is(err, logic.ErrGameBanNotFound):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "game_ban_not_found",
			"message": message,
		})
	case errors.Is(err, logic.ErrGameBanRCONUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "rcon_unavailable",
			"message": message,
		})
	case errors.Is(err, logic.ErrGameBanRCONResponse):
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "invalid_rcon_response",
			"message": message,
		})
	case errors.Is(err, logic.ErrGameBanPersistence):
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "game_ban_persistence_failed",
			"message": message,
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "game_ban_failed",
			"message": message,
		})
	}
}

func gameBanErrorMessage(err error) string {
	message := err.Error()
	for _, sentinel := range []error{
		logic.ErrGameBanInvalidInput,
		logic.ErrGameBanDuplicate,
		logic.ErrGameBanNotFound,
		logic.ErrGameBanRCONUnavailable,
		logic.ErrGameBanRCONResponse,
		logic.ErrGameBanPersistence,
	} {
		message = strings.TrimPrefix(message, sentinel.Error()+": ")
	}
	return message
}
