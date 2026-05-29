package controller

import (
	"encoding/json"
	"l4d2-manager-next/db"
	"l4d2-manager-next/model"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPlayerStatsTestDB(t *testing.T) {
	t.Helper()

	oldDB := db.PlayerStatsDB
	testDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "player_stats.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := testDB.AutoMigrate(&model.PlayerStatSnapshot{}, &model.PlayerStatPlayer{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}

	db.PlayerStatsDB = testDB
	t.Cleanup(func() {
		sqlDB.Close()
		db.PlayerStatsDB = oldDB
	})
}

func TestCleanupPlayerStatsDeletesExpiredRows(t *testing.T) {
	setupPlayerStatsTestDB(t)

	oldTimestamp := time.Now().Add(-31 * 24 * time.Hour).Unix()
	newTimestamp := time.Now().Unix()
	oldSnapshot := model.PlayerStatSnapshot{Timestamp: oldTimestamp, ServerOnline: true, CollectOK: true}
	newSnapshot := model.PlayerStatSnapshot{Timestamp: newTimestamp, ServerOnline: true, CollectOK: true}

	if err := db.PlayerStatsDB.Create(&oldSnapshot).Error; err != nil {
		t.Fatalf("create old snapshot: %v", err)
	}
	if err := db.PlayerStatsDB.Create(&newSnapshot).Error; err != nil {
		t.Fatalf("create new snapshot: %v", err)
	}
	if err := db.PlayerStatsDB.Create(&[]model.PlayerStatPlayer{
		{SnapshotID: oldSnapshot.ID, Timestamp: oldTimestamp, SteamID: "STEAM_1:0:1"},
		{SnapshotID: newSnapshot.ID, Timestamp: newTimestamp, SteamID: "STEAM_1:0:2"},
	}).Error; err != nil {
		t.Fatalf("create players: %v", err)
	}

	cleanupPlayerStats()

	var oldSnapshotCount int64
	db.PlayerStatsDB.Model(&model.PlayerStatSnapshot{}).Where("id = ?", oldSnapshot.ID).Count(&oldSnapshotCount)
	if oldSnapshotCount != 0 {
		t.Fatalf("old snapshot count = %d, want 0", oldSnapshotCount)
	}

	var oldPlayerCount int64
	db.PlayerStatsDB.Model(&model.PlayerStatPlayer{}).Where("snapshot_id = ?", oldSnapshot.ID).Count(&oldPlayerCount)
	if oldPlayerCount != 0 {
		t.Fatalf("old player count = %d, want 0", oldPlayerCount)
	}

	var newSnapshotCount int64
	db.PlayerStatsDB.Model(&model.PlayerStatSnapshot{}).Where("id = ?", newSnapshot.ID).Count(&newSnapshotCount)
	if newSnapshotCount != 1 {
		t.Fatalf("new snapshot count = %d, want 1", newSnapshotCount)
	}
}

func TestGetPlayerStatsHourlyAggregatesOfflineAndUniquePlayers(t *testing.T) {
	setupPlayerStatsTestDB(t)
	gin.SetMode(gin.TestMode)

	bucket := time.Now().Add(-time.Hour).Truncate(time.Hour).Unix()
	snapshots := []model.PlayerStatSnapshot{
		{Timestamp: bucket + 60, ServerOnline: true, CollectOK: true, PlayerCount: 2},
		{Timestamp: bucket + 600, ServerOnline: true, CollectOK: true, PlayerCount: 4},
		{Timestamp: bucket + 1200, ServerOnline: false, CollectOK: false, ErrorMessage: "offline"},
	}
	if err := db.PlayerStatsDB.Create(&snapshots).Error; err != nil {
		t.Fatalf("create snapshots: %v", err)
	}
	players := []model.PlayerStatPlayer{
		{SnapshotID: snapshots[0].ID, Timestamp: bucket + 60, SteamID: "STEAM_1:0:1"},
		{SnapshotID: snapshots[1].ID, Timestamp: bucket + 600, SteamID: "STEAM_1:0:1"},
		{SnapshotID: snapshots[1].ID, Timestamp: bucket + 600, SteamID: "STEAM_1:0:2"},
	}
	if err := db.PlayerStatsDB.Create(&players).Error; err != nil {
		t.Fatalf("create players: %v", err)
	}

	form := url.Values{}
	form.Set("start", strconv.FormatInt(bucket, 10))
	form.Set("end", strconv.FormatInt(bucket+3599, 10))
	req := httptest.NewRequest(http.MethodPost, "/player-stats/hourly", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("role", "admin")

	GetPlayerStatsHourly(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var results []struct {
		Timestamp      int64    `json:"timestamp"`
		AvgPlayers     *float64 `json:"avg_players"`
		PeakPlayers    *int     `json:"peak_players"`
		UniquePlayers  int64    `json:"unique_players"`
		OfflineSamples int64    `json:"offline_samples"`
		SampleCount    int64    `json:"sample_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	got := results[0]
	if got.AvgPlayers == nil || math.Abs(*got.AvgPlayers-3) > 0.001 {
		t.Fatalf("avg_players = %v, want 3", got.AvgPlayers)
	}
	if got.PeakPlayers == nil || *got.PeakPlayers != 4 {
		t.Fatalf("peak_players = %v, want 4", got.PeakPlayers)
	}
	if got.UniquePlayers != 2 {
		t.Fatalf("unique_players = %d, want 2", got.UniquePlayers)
	}
	if got.OfflineSamples != 1 {
		t.Fatalf("offline_samples = %d, want 1", got.OfflineSamples)
	}
	if got.SampleCount != 3 {
		t.Fatalf("sample_count = %d, want 3", got.SampleCount)
	}
}
