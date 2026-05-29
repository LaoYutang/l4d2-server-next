package model

// PlayerStatSnapshot stores one scheduled player-stat collection result.
type PlayerStatSnapshot struct {
	ID           uint   `json:"id" gorm:"primaryKey"`
	Timestamp    int64  `json:"timestamp" gorm:"index:idx_player_stat_snapshot_timestamp"`
	ServerOnline bool   `json:"server_online" gorm:"index"`
	CollectOK    bool   `json:"collect_ok" gorm:"index"`
	PlayerCount  int    `json:"player_count"`
	MaxPlayers   int    `json:"max_players"`
	Map          string `json:"map"`
	Hostname     string `json:"hostname"`
	Difficulty   string `json:"difficulty"`
	GameMode     string `json:"game_mode"`
	ErrorMessage string `json:"error_message"`
}

// PlayerStatPlayer stores players observed in a successful snapshot.
type PlayerStatPlayer struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	SnapshotID uint   `json:"snapshot_id" gorm:"index"`
	Timestamp  int64  `json:"timestamp" gorm:"index:idx_player_stat_player_timestamp"`
	SteamID    string `json:"steam_id" gorm:"index"`
	Name       string `json:"name" gorm:"index"`
	IP         string `json:"ip"`
	Location   string `json:"location"`
	Status     string `json:"status"`
	Delay      int    `json:"delay"`
	Loss       int    `json:"loss"`
	Duration   string `json:"duration"`
	LinkRate   int    `json:"link_rate"`
}
