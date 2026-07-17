package controller

import (
	"errors"
	"fmt"
	"l4d2-manager-next/db"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/model"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	playerStatsIntervalMinutes = 10
	playerStatsRetentionDays   = 30
)

var playerStatsCollectMutex sync.Mutex

type PlayerStatsTrendResult struct {
	BucketTime     int64    `json:"timestamp" gorm:"column:bucket_time"`
	AvgPlayers     *float64 `json:"avg_players" gorm:"column:avg_players"`
	PeakPlayers    *int     `json:"peak_players" gorm:"column:peak_players"`
	UniquePlayers  int64    `json:"unique_players" gorm:"column:unique_players"`
	OfflineSamples int64    `json:"offline_samples" gorm:"column:offline_samples"`
	SampleCount    int64    `json:"sample_count" gorm:"column:sample_count"`
}

func StartPlayerStatsCollector() {
	cleanupPlayerStats()

	if logic.IsPlayerStatsEnabled() {
		collectPlayerStats()
	}

	collectTicker := time.NewTicker(playerStatsIntervalMinutes * time.Minute)
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer collectTicker.Stop()
	defer cleanupTicker.Stop()

	for {
		select {
		case <-collectTicker.C:
			if logic.IsPlayerStatsEnabled() {
				collectPlayerStats()
			}
		case <-cleanupTicker.C:
			cleanupPlayerStats()
		}
	}
}

func collectPlayerStats() {
	if db.PlayerStatsDB == nil {
		return
	}

	playerStatsCollectMutex.Lock()
	defer playerStatsCollectMutex.Unlock()

	now := time.Now().Unix()
	snapshot := model.PlayerStatSnapshot{Timestamp: now}

	conn, err := getRconConnection()
	if err != nil {
		snapshot.ErrorMessage = err.Error()
		savePlayerStatsSnapshot(snapshot, nil)
		return
	}
	defer conn.Close()

	res, err := conn.Execute("status")
	if err != nil {
		snapshot.ErrorMessage = fmt.Sprintf("RCON命令执行失败: %v", err)
		savePlayerStatsSnapshot(snapshot, nil)
		return
	}

	difficultyRes, err := conn.Execute("z_difficulty")
	if err != nil {
		difficultyRes = "Unknown"
	}

	gameModeRes, err := conn.Execute("sm_cvar mp_gamemode")
	if err != nil {
		gameModeRes = "Unknown"
	}

	status := logic.ParseStatus(res)
	status.Difficulty = logic.ParseDifficulty(difficultyRes)
	status.GameMode = logic.ParseGameMode(gameModeRes)

	playerCount := status.PlayerCount
	if playerCount == 0 && len(status.Users) > 0 {
		playerCount = len(status.Users)
	}

	snapshot.ServerOnline = true
	snapshot.CollectOK = true
	snapshot.PlayerCount = playerCount
	snapshot.MaxPlayers = status.MaxPlayers
	snapshot.Map = status.Map
	snapshot.Hostname = status.Hostname
	snapshot.Difficulty = status.Difficulty
	snapshot.GameMode = status.GameMode

	players := make([]model.PlayerStatPlayer, 0, len(status.Users))
	for _, user := range status.Users {
		players = append(players, model.PlayerStatPlayer{
			Timestamp: now,
			SteamID:   user.SteamId,
			Name:      user.Name,
			IP:        user.Ip,
			Location:  user.Location,
			Status:    user.Status,
			Delay:     user.Delay,
			Loss:      user.Loss,
			Duration:  user.Duration,
			LinkRate:  user.LinkRate,
		})
	}

	savePlayerStatsSnapshot(snapshot, players)
}

func savePlayerStatsSnapshot(snapshot model.PlayerStatSnapshot, players []model.PlayerStatPlayer) {
	err := db.PlayerStatsDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&snapshot).Error; err != nil {
			return err
		}
		if len(players) == 0 {
			return nil
		}
		for i := range players {
			players[i].SnapshotID = snapshot.ID
		}
		return tx.Create(&players).Error
	})
	if err != nil {
		log.Printf("Failed to save player stats snapshot: %v", err)
	}
}

func cleanupPlayerStats() {
	if db.PlayerStatsDB == nil {
		return
	}

	expiration := time.Now().Add(-playerStatsRetentionDays * 24 * time.Hour).Unix()

	var snapshotIDs []uint
	if err := db.PlayerStatsDB.Model(&model.PlayerStatSnapshot{}).
		Where("timestamp < ?", expiration).
		Pluck("id", &snapshotIDs).Error; err != nil {
		log.Printf("Failed to find expired player stats snapshots: %v", err)
		return
	}

	if len(snapshotIDs) > 0 {
		if err := db.PlayerStatsDB.Where("snapshot_id IN ?", snapshotIDs).Delete(&model.PlayerStatPlayer{}).Error; err != nil {
			log.Printf("Failed to cleanup expired player stats players: %v", err)
		}
	}
	if err := db.PlayerStatsDB.Where("timestamp < ?", expiration).Delete(&model.PlayerStatSnapshot{}).Error; err != nil {
		log.Printf("Failed to cleanup expired player stats snapshots: %v", err)
	}
	if err := db.PlayerStatsDB.Where("timestamp < ?", expiration).Delete(&model.PlayerStatPlayer{}).Error; err != nil {
		log.Printf("Failed to cleanup expired orphan player stats players: %v", err)
	}
}

func requirePlayerStatsAdmin(c *gin.Context) bool {
	role, _ := c.Get("role")
	if role != "admin" {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return false
	}
	return true
}

func GetPlayerStatsConfig(c *gin.Context) {
	var lastSnapshot *model.PlayerStatSnapshot
	if db.PlayerStatsDB != nil {
		var snapshot model.PlayerStatSnapshot
		err := db.PlayerStatsDB.Order("timestamp DESC").First(&snapshot).Error
		if err == nil {
			lastSnapshot = &snapshot
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			FailWithError(c, http.StatusInternalServerError, "查询最近采集状态失败: %v", err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":          logic.IsPlayerStatsEnabled(),
		"interval_minutes": playerStatsIntervalMinutes,
		"retention_days":   playerStatsRetentionDays,
		"last_snapshot":    lastSnapshot,
	})
}

func SetPlayerStatsConfig(c *gin.Context) {
	if !requirePlayerStatsAdmin(c) {
		return
	}

	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, http.StatusBadRequest, "参数错误")
		return
	}

	detail := "关闭玩家在线统计"
	if req.Enable {
		detail = "开启玩家在线统计"
	}
	defer LogOp(c, detail)()
	if err := logic.SetPlayerStatsEnable(req.Enable); err != nil {
		FailWithError(c, http.StatusInternalServerError, "保存配置失败: %v", err)
		return
	}

	if req.Enable {
		go collectPlayerStats()
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func GetPlayerStatsHourly(c *gin.Context) {
	if !requirePlayerStatsAdmin(c) {
		return
	}
	if db.PlayerStatsDB == nil {
		FailWithError(c, http.StatusBadRequest, "玩家统计数据库未初始化")
		return
	}

	start, end, ok := parsePlayerStatsRange(c)
	if !ok {
		return
	}

	if c.PostForm("bucket") == "day" {
		results, err := queryPlayerStatsDailyTrend(start, end)
		if err != nil {
			FailWithError(c, http.StatusInternalServerError, "查询每日统计失败: %v", err)
			return
		}
		c.JSON(http.StatusOK, results)
		return
	}

	var results []PlayerStatsTrendResult
	err := db.PlayerStatsDB.Raw(`
		SELECT
			CAST(timestamp / 3600 AS INTEGER) * 3600 AS bucket_time,
			ROUND(AVG(CASE WHEN server_online = 1 AND collect_ok = 1 THEN player_count END), 2) AS avg_players,
			MAX(CASE WHEN server_online = 1 AND collect_ok = 1 THEN player_count END) AS peak_players,
			SUM(CASE WHEN server_online = 0 OR collect_ok = 0 THEN 1 ELSE 0 END) AS offline_samples,
			COUNT(*) AS sample_count
		FROM player_stat_snapshots
		WHERE timestamp >= ? AND timestamp <= ?
		GROUP BY bucket_time
		ORDER BY bucket_time ASC
	`, start, end).Scan(&results).Error
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "查询小时统计失败: %v", err)
		return
	}

	type UniqueResult struct {
		BucketTime    int64 `gorm:"column:bucket_time"`
		UniquePlayers int64 `gorm:"column:unique_players"`
	}
	var uniqueResults []UniqueResult
	err = db.PlayerStatsDB.Raw(`
		SELECT
			CAST(timestamp / 3600 AS INTEGER) * 3600 AS bucket_time,
			COUNT(DISTINCT steam_id) AS unique_players
		FROM player_stat_players
		WHERE timestamp >= ? AND timestamp <= ? AND steam_id <> ''
		GROUP BY bucket_time
	`, start, end).Scan(&uniqueResults).Error
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "查询独立玩家数失败: %v", err)
		return
	}

	uniqueByBucket := make(map[int64]int64, len(uniqueResults))
	for _, item := range uniqueResults {
		uniqueByBucket[item.BucketTime] = item.UniquePlayers
	}
	for i := range results {
		results[i].UniquePlayers = uniqueByBucket[results[i].BucketTime]
	}

	c.JSON(http.StatusOK, results)
}

type playerStatsDailyAgg struct {
	sumPlayers     int
	onlineSamples  int64
	peakPlayers    int
	hasPeak        bool
	offlineSamples int64
	sampleCount    int64
}

func queryPlayerStatsDailyTrend(start, end int64) ([]PlayerStatsTrendResult, error) {
	var snapshots []model.PlayerStatSnapshot
	if err := db.PlayerStatsDB.
		Where("timestamp >= ? AND timestamp <= ?", start, end).
		Order("timestamp ASC").
		Find(&snapshots).Error; err != nil {
		return nil, err
	}

	aggByDay := make(map[int64]*playerStatsDailyAgg)
	dayOrder := make([]int64, 0)
	for _, snapshot := range snapshots {
		bucket := localDayStart(snapshot.Timestamp)
		agg, exists := aggByDay[bucket]
		if !exists {
			agg = &playerStatsDailyAgg{}
			aggByDay[bucket] = agg
			dayOrder = append(dayOrder, bucket)
		}

		agg.sampleCount++
		if !snapshot.ServerOnline || !snapshot.CollectOK {
			agg.offlineSamples++
			continue
		}

		agg.sumPlayers += snapshot.PlayerCount
		agg.onlineSamples++
		if !agg.hasPeak || snapshot.PlayerCount > agg.peakPlayers {
			agg.peakPlayers = snapshot.PlayerCount
			agg.hasPeak = true
		}
	}

	type PlayerUnique struct {
		Timestamp int64
		SteamID   string
	}
	var players []PlayerUnique
	if err := db.PlayerStatsDB.Model(&model.PlayerStatPlayer{}).
		Select("timestamp, steam_id").
		Where("timestamp >= ? AND timestamp <= ? AND steam_id <> ''", start, end).
		Scan(&players).Error; err != nil {
		return nil, err
	}

	uniqueByDay := make(map[int64]map[string]struct{})
	for _, player := range players {
		bucket := localDayStart(player.Timestamp)
		if _, exists := uniqueByDay[bucket]; !exists {
			uniqueByDay[bucket] = make(map[string]struct{})
		}
		uniqueByDay[bucket][player.SteamID] = struct{}{}
	}

	results := make([]PlayerStatsTrendResult, 0, len(dayOrder))
	for _, bucket := range dayOrder {
		agg := aggByDay[bucket]
		result := PlayerStatsTrendResult{
			BucketTime:     bucket,
			UniquePlayers:  int64(len(uniqueByDay[bucket])),
			OfflineSamples: agg.offlineSamples,
			SampleCount:    agg.sampleCount,
		}
		if agg.onlineSamples > 0 {
			avg := toFixed(float64(agg.sumPlayers)/float64(agg.onlineSamples), 2)
			peak := agg.peakPlayers
			result.AvgPlayers = &avg
			result.PeakPlayers = &peak
		}
		results = append(results, result)
	}

	return results, nil
}

func localDayStart(timestamp int64) int64 {
	t := time.Unix(timestamp, 0).In(time.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Unix()
}

func SearchPlayerStatsPlayers(c *gin.Context) {
	if !requirePlayerStatsAdmin(c) {
		return
	}
	if db.PlayerStatsDB == nil {
		FailWithError(c, http.StatusBadRequest, "玩家统计数据库未初始化")
		return
	}

	keyword := strings.TrimSpace(c.PostForm("keyword"))
	start := time.Now().Add(-playerStatsRetentionDays * 24 * time.Hour).Unix()
	if startStr := c.PostForm("start"); startStr != "" {
		if parsed, err := strconv.ParseInt(startStr, 10, 64); err == nil && parsed > 0 {
			start = parsed
		}
	}

	type Candidate struct {
		SteamID  string `gorm:"column:steam_id"`
		Samples  int64  `gorm:"column:samples"`
		LastSeen int64  `gorm:"column:last_seen"`
		Rank     int
	}
	var candidates []Candidate

	matchedSteamIDSet := map[string]struct{}{}
	if keyword != "" {
		var matchedSteamIDs []string
		like := "%" + keyword + "%"
		if err := db.PlayerStatsDB.Model(&model.PlayerStatPlayer{}).
			Distinct("steam_id").
			Where("timestamp >= ? AND steam_id <> ''", start).
			Where("steam_id LIKE ? OR name LIKE ?", like, like).
			Limit(200).
			Pluck("steam_id", &matchedSteamIDs).Error; err != nil {
			FailWithError(c, http.StatusInternalServerError, "搜索玩家失败: %v", err)
			return
		}
		if len(matchedSteamIDs) == 0 {
			c.JSON(http.StatusOK, []gin.H{})
			return
		}
		for _, steamID := range matchedSteamIDs {
			matchedSteamIDSet[steamID] = struct{}{}
		}
	}

	err := db.PlayerStatsDB.Model(&model.PlayerStatPlayer{}).
		Select("steam_id, COUNT(*) AS samples, MAX(timestamp) AS last_seen").
		Where("timestamp >= ? AND steam_id <> ''", start).
		Group("steam_id").
		Order("samples DESC").
		Order("last_seen DESC").
		Order("steam_id ASC").
		Scan(&candidates).Error
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "搜索玩家失败: %v", err)
		return
	}

	type PlayerResult struct {
		SteamID          string `json:"steam_id"`
		Name             string `json:"name"`
		Location         string `json:"location"`
		IP               string `json:"ip"`
		LastSeen         int64  `json:"last_seen"`
		EstimatedMinutes int64  `json:"estimated_minutes"`
		Rank             int    `json:"rank"`
	}
	results := make([]PlayerResult, 0, len(candidates))
	for i, candidate := range candidates {
		candidate.Rank = i + 1
		if keyword != "" {
			if _, ok := matchedSteamIDSet[candidate.SteamID]; !ok {
				continue
			}
		}

		var latest model.PlayerStatPlayer
		err := db.PlayerStatsDB.Where("steam_id = ? AND timestamp >= ?", candidate.SteamID, start).
			Order("timestamp DESC").
			Order("id DESC").
			First(&latest).Error
		if err != nil {
			continue
		}
		results = append(results, PlayerResult{
			SteamID:          candidate.SteamID,
			Name:             latest.Name,
			Location:         latest.Location,
			IP:               latest.IP,
			LastSeen:         candidate.LastSeen,
			EstimatedMinutes: candidate.Samples * playerStatsIntervalMinutes,
			Rank:             candidate.Rank,
		})
		if len(results) >= 50 {
			break
		}
	}

	c.JSON(http.StatusOK, results)
}

func GetPlayerStatsPlayerDays(c *gin.Context) {
	if !requirePlayerStatsAdmin(c) {
		return
	}
	if db.PlayerStatsDB == nil {
		FailWithError(c, http.StatusBadRequest, "玩家统计数据库未初始化")
		return
	}

	steamID := strings.TrimSpace(c.PostForm("steam_id"))
	if steamID == "" {
		FailWithError(c, http.StatusBadRequest, "SteamID不能为空")
		return
	}

	start, end, ok := parsePlayerStatsRange(c)
	if !ok {
		return
	}

	var samples []model.PlayerStatPlayer
	err := db.PlayerStatsDB.
		Where("steam_id = ? AND timestamp >= ? AND timestamp <= ?", steamID, start, end).
		Order("timestamp ASC").
		Find(&samples).Error
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "查询玩家在线日期失败: %v", err)
		return
	}

	type DayResult struct {
		Date          string `json:"date"`
		OnlineMinutes int    `json:"online_minutes"`
		Samples       int    `json:"samples"`
		FirstSeen     int64  `json:"first_seen"`
		LastSeen      int64  `json:"last_seen"`
	}
	type NameAliasResult struct {
		Name             string `json:"name"`
		Samples          int    `json:"samples"`
		EstimatedMinutes int    `json:"estimated_minutes"`
		FirstSeen        int64  `json:"first_seen"`
		LastSeen         int64  `json:"last_seen"`
	}

	dayMap := make(map[string]*DayResult)
	dayOrder := make([]string, 0)
	aliasMap := make(map[string]*NameAliasResult)
	for _, sample := range samples {
		date := time.Unix(sample.Timestamp, 0).Format("2006-01-02")
		day, exists := dayMap[date]
		if !exists {
			day = &DayResult{Date: date, FirstSeen: sample.Timestamp}
			dayMap[date] = day
			dayOrder = append(dayOrder, date)
		}
		day.Samples++
		day.OnlineMinutes += playerStatsIntervalMinutes
		day.LastSeen = sample.Timestamp

		name := strings.TrimSpace(sample.Name)
		if name == "" {
			name = "Unknown"
		}
		alias, exists := aliasMap[name]
		if !exists {
			alias = &NameAliasResult{Name: name, FirstSeen: sample.Timestamp}
			aliasMap[name] = alias
		}
		alias.Samples++
		alias.EstimatedMinutes += playerStatsIntervalMinutes
		alias.LastSeen = sample.Timestamp
	}

	days := make([]DayResult, 0, len(dayOrder))
	for _, date := range dayOrder {
		days = append(days, *dayMap[date])
	}
	aliases := make([]NameAliasResult, 0, len(aliasMap))
	for _, alias := range aliasMap {
		aliases = append(aliases, *alias)
	}
	sort.Slice(aliases, func(i, j int) bool {
		return aliases[i].LastSeen > aliases[j].LastSeen
	})

	var latest model.PlayerStatPlayer
	err = db.PlayerStatsDB.Where("steam_id = ?", steamID).Order("timestamp DESC").First(&latest).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		FailWithError(c, http.StatusInternalServerError, "查询玩家信息失败: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"steam_id": steamID,
		"player":   latest,
		"days":     days,
		"aliases":  aliases,
	})
}

func parsePlayerStatsRange(c *gin.Context) (int64, int64, bool) {
	end := time.Now().Unix()
	start := end - int64(playerStatsRetentionDays*24*3600)

	if startStr := c.PostForm("start"); startStr != "" {
		parsed, err := strconv.ParseInt(startStr, 10, 64)
		if err != nil || parsed <= 0 {
			FailWithError(c, http.StatusBadRequest, "无效的开始时间")
			return 0, 0, false
		}
		start = parsed
	}
	if endStr := c.PostForm("end"); endStr != "" {
		parsed, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || parsed <= 0 {
			FailWithError(c, http.StatusBadRequest, "无效的结束时间")
			return 0, 0, false
		}
		end = parsed
	}
	if start >= end {
		FailWithError(c, http.StatusBadRequest, "开始时间必须早于结束时间")
		return 0, 0, false
	}

	retentionStart := time.Now().Add(-playerStatsRetentionDays * 24 * time.Hour).Unix()
	if start < retentionStart {
		start = retentionStart
	}

	return start, end, true
}
