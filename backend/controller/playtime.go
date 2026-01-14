package controller

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
)

const requestUrl = "http://api.steampowered.com/IPlayerService/GetOwnedGames/v0001"

var client = resty.New()

func GetUserPlaytime(c *gin.Context) {
	steamID := c.PostForm("steamid")
	if steamID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未提供SteamID"})
		return
	}

	playtime, err := getUserPlaytime(steamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"playtime": playtime})
}

// Steam API 响应结构体
type SteamGamesResponse struct {
	Response struct {
		Games []struct {
			AppID           int `json:"appid"`
			PlaytimeForever int `json:"playtime_forever"`
		} `json:"games"`
	} `json:"response"`
}

func getUserPlaytime(steamID string) (float64, error) {
	key := os.Getenv("STEAM_API_KEY")
	if key == "" {
		return 0, fmt.Errorf("未设置STEAM_API_KEY，请在https://steamcommunity.com/dev/apikey获取并设置")
	}

	id, err := convertSteamIDToSteam64ID(steamID)
	if err != nil {
		return 0, fmt.Errorf("无效的SteamID: %w", err)
	}

	// 请求服务器获取所有游戏数据，遍历获取id为550的游戏的playtime_forever字段
	res := &SteamGamesResponse{}
	_, err = client.R().SetQueryParams(map[string]string{
		"key":     key,
		"steamid": strconv.FormatUint(id, 10),
	}).SetResult(res).Get(requestUrl)
	if err != nil {
		return 0, fmt.Errorf("请求Steam API失败: %w", err)
	}

	for _, game := range res.Response.Games {
		if game.AppID == 550 {
			return float64(game.PlaytimeForever) / 60, nil
		}
	}

	return 0, errors.New("玩家资料未公开")
}

func convertSteamIDToSteam64ID(steamID string) (uint64, error) {
	// 使用正则表达式匹配 SteamID 的各个部分
	re := regexp.MustCompile(`^STEAM_([0-1]):([0-1]):(\d+)$`)
	matches := re.FindStringSubmatch(steamID)

	if len(matches) != 4 {
		return 0, fmt.Errorf("invalid SteamID format: %s", steamID)
	}

	// 提取 Y 和 Z 的值
	y, err := strconv.ParseUint(matches[2], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid Y value in SteamID: %w", err)
	}
	z, err := strconv.ParseUint(matches[3], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid Z value in SteamID: %w", err)
	}

	// Steam64 ID 的基数
	const steam64Base uint64 = 76561197960265728

	// 计算 Steam64 ID
	steam64ID := z*2 + steam64Base + y

	return steam64ID, nil
}
