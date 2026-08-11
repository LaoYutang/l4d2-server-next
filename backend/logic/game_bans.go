package logic

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gorcon/rcon"
)

const (
	GameBanKindSteamID = "steam_id"
	GameBanKindIP      = "ip"

	steamID64Base   = uint64(76561197960265728)
	maxSteamAccount = uint64(1<<32 - 1)
	maxBanMinutes   = int64(1<<31 - 1)
)

var (
	ErrGameBanInvalidInput    = errors.New("invalid game ban input")
	ErrGameBanDuplicate       = errors.New("game ban already exists")
	ErrGameBanNotFound        = errors.New("game ban not found")
	ErrGameBanRCONUnavailable = errors.New("game server RCON unavailable")
	ErrGameBanRCONResponse    = errors.New("invalid game server RCON response")
	ErrGameBanPersistence     = errors.New("game ban persistence unavailable")
)

type GameBanEntry struct {
	Kind             string   `json:"kind"`
	Value            string   `json:"value"`
	SteamID64        string   `json:"steam_id64,omitempty"`
	Permanent        bool     `json:"permanent"`
	RemainingMinutes *float64 `json:"remaining_minutes,omitempty"`
}

type GameBanList struct {
	SteamBans        []GameBanEntry `json:"steam_bans"`
	IPBans           []GameBanEntry `json:"ip_bans"`
	PersistenceReady bool           `json:"persistence_ready"`
	Warnings         []string       `json:"warnings"`
}

type GameBanChange struct {
	Kind            string `json:"kind"`
	Value           string `json:"value"`
	DurationMinutes int    `json:"duration_minutes"`
}

type gameBanRCONSession interface {
	Execute(command string) (string, error)
	Close() error
}

var dialGameBanRCON = defaultGameBanRCONDialer

var gameBanMutationMu sync.Mutex

var (
	steam2Pattern       = regexp.MustCompile(`(?i)^STEAM_[0-5]:([01]):([0-9]+)$`)
	steam3Pattern       = regexp.MustCompile(`(?i)^\[U:1:([0-9]+)\]$`)
	listIDHeaderPattern = regexp.MustCompile(`(?i)\bID filter list:\s*(empty|[0-9]+\s+(?:entry|entries))`)
	listIPHeaderPattern = regexp.MustCompile(`(?i)\bIP filter list:\s*(empty|[0-9]+\s+(?:entry|entries))`)
	listIDEntryPattern  = regexp.MustCompile(`(?im)^\s*[0-9]+\s+((?:STEAM_[0-5]:[01]:[0-9]+)|(?:\[U:1:[0-9]+\])|(?:[0-9]{17}))\s*:\s*(permanent|[0-9]+(?:\.[0-9]+)?\s+min)\s*$`)
	listIPEntryPattern  = regexp.MustCompile(`(?im)^\s*[0-9]+\s+([0-9.\t ]+)\s*:\s*(permanent|[0-9]+(?:\.[0-9]+)?\s+min)\s*$`)
	minutesPattern      = regexp.MustCompile(`(?i)^([0-9]+(?:\.[0-9]+)?)\s+min$`)
)

func ListGameBans() (GameBanList, error) {
	session, err := dialGameBanRCON()
	if err != nil {
		return GameBanList{}, err
	}
	defer session.Close()
	return readGameBanList(session)
}

func AddGameBan(change GameBanChange) (GameBanList, error) {
	gameBanMutationMu.Lock()
	defer gameBanMutationMu.Unlock()

	canonical, err := NormalizeGameBanValue(change.Kind, change.Value)
	if err != nil {
		return GameBanList{}, err
	}
	if change.DurationMinutes < 0 || int64(change.DurationMinutes) > maxBanMinutes {
		return GameBanList{}, fmt.Errorf("%w: 封禁分钟数必须在 0 到 %d 之间", ErrGameBanInvalidInput, maxBanMinutes)
	}

	session, err := dialGameBanRCON()
	if err != nil {
		return GameBanList{}, err
	}
	defer session.Close()

	current, err := readGameBanList(session)
	if err != nil {
		return GameBanList{}, err
	}
	if gameBanContains(current, change.Kind, canonical) {
		return GameBanList{}, fmt.Errorf("%w: %s 已在游戏黑名单中", ErrGameBanDuplicate, canonical)
	}
	if err := EnsureGameBanPersistenceConfig(); err != nil {
		return GameBanList{}, fmt.Errorf("%w: %v", ErrGameBanPersistence, err)
	}

	var command string
	switch change.Kind {
	case GameBanKindSteamID:
		command = fmt.Sprintf("banid %d %s kick", change.DurationMinutes, canonical)
	case GameBanKindIP:
		command = fmt.Sprintf("addip %d %s", change.DurationMinutes, canonical)
	default:
		return GameBanList{}, fmt.Errorf("%w: 不支持的封禁类型", ErrGameBanInvalidInput)
	}
	if _, err := executeGameBanRCON(session, command); err != nil {
		return GameBanList{}, err
	}

	if change.DurationMinutes == 0 {
		writeCommand := "writeid"
		if change.Kind == GameBanKindIP {
			writeCommand = "writeip"
		}
		if response, err := executeGameBanRCON(session, writeCommand); err != nil {
			return GameBanList{}, err
		} else if commandResponseLooksFailed(response) {
			return GameBanList{}, fmt.Errorf("%w: %s 持久化失败", ErrGameBanRCONResponse, canonical)
		}
	}

	updated, err := readGameBanList(session)
	if err != nil {
		return GameBanList{}, err
	}
	entry, found := findGameBan(updated, change.Kind, canonical)
	if !found || entry.Permanent != (change.DurationMinutes == 0) {
		return GameBanList{}, fmt.Errorf("%w: 游戏服务器未返回预期的封禁结果", ErrGameBanRCONResponse)
	}
	return updated, nil
}

func RemoveGameBan(kind, value string) (GameBanList, error) {
	gameBanMutationMu.Lock()
	defer gameBanMutationMu.Unlock()

	canonical, err := NormalizeGameBanValue(kind, value)
	if err != nil {
		return GameBanList{}, err
	}

	session, err := dialGameBanRCON()
	if err != nil {
		return GameBanList{}, err
	}
	defer session.Close()

	current, err := readGameBanList(session)
	if err != nil {
		return GameBanList{}, err
	}
	if !gameBanContains(current, kind, canonical) {
		return GameBanList{}, fmt.Errorf("%w: %s 不在游戏黑名单中", ErrGameBanNotFound, canonical)
	}
	if err := EnsureGameBanPersistenceConfig(); err != nil {
		return GameBanList{}, fmt.Errorf("%w: %v", ErrGameBanPersistence, err)
	}

	removeCommand := "removeid " + canonical
	writeCommand := "writeid"
	if kind == GameBanKindIP {
		removeCommand = "removeip " + canonical
		writeCommand = "writeip"
	}
	if _, err := executeGameBanRCON(session, removeCommand); err != nil {
		return GameBanList{}, err
	}
	if response, err := executeGameBanRCON(session, writeCommand); err != nil {
		return GameBanList{}, err
	} else if commandResponseLooksFailed(response) {
		return GameBanList{}, fmt.Errorf("%w: %s 持久化失败", ErrGameBanRCONResponse, canonical)
	}

	updated, err := readGameBanList(session)
	if err != nil {
		return GameBanList{}, err
	}
	if gameBanContains(updated, kind, canonical) {
		return GameBanList{}, fmt.Errorf("%w: 游戏服务器仍返回被删除的封禁", ErrGameBanRCONResponse)
	}
	return updated, nil
}

func NormalizeGameBanValue(kind, value string) (string, error) {
	value = strings.TrimSpace(value)
	switch kind {
	case GameBanKindSteamID:
		steam2, _, err := normalizeSteamID(value)
		return steam2, err
	case GameBanKindIP:
		addr, err := netip.ParseAddr(value)
		if err != nil || !addr.Is4() {
			return "", fmt.Errorf("%w: IP 封禁只支持单个 IPv4 地址", ErrGameBanInvalidInput)
		}
		return addr.String(), nil
	default:
		return "", fmt.Errorf("%w: 不支持的封禁类型", ErrGameBanInvalidInput)
	}
}

func normalizeSteamID(value string) (string, string, error) {
	var accountID uint64
	switch {
	case steam2Pattern.MatchString(value):
		matches := steam2Pattern.FindStringSubmatch(value)
		y, _ := strconv.ParseUint(matches[1], 10, 1)
		z, err := strconv.ParseUint(matches[2], 10, 64)
		if err != nil || z > maxSteamAccount/2 {
			return "", "", fmt.Errorf("%w: Steam2 ID 超出有效范围", ErrGameBanInvalidInput)
		}
		accountID = z*2 + y
	case steam3Pattern.MatchString(value):
		matches := steam3Pattern.FindStringSubmatch(value)
		parsed, err := strconv.ParseUint(matches[1], 10, 64)
		if err != nil || parsed > maxSteamAccount {
			return "", "", fmt.Errorf("%w: Steam3 ID 超出有效范围", ErrGameBanInvalidInput)
		}
		accountID = parsed
	default:
		steam64, err := strconv.ParseUint(value, 10, 64)
		if err != nil || steam64 < steamID64Base || steam64 > steamID64Base+maxSteamAccount {
			return "", "", fmt.Errorf("%w: 请输入有效的 Steam2、Steam3 或 Steam64 ID", ErrGameBanInvalidInput)
		}
		accountID = steam64 - steamID64Base
	}

	steam2 := fmt.Sprintf("STEAM_1:%d:%d", accountID%2, accountID/2)
	steam64 := strconv.FormatUint(steamID64Base+accountID, 10)
	return steam2, steam64, nil
}

func defaultGameBanRCONDialer() (gameBanRCONSession, error) {
	url := strings.TrimSpace(os.Getenv("L4D2_RCON_URL"))
	if url == "" {
		return nil, fmt.Errorf("%w: 服务端未配置 RCON 地址", ErrGameBanRCONUnavailable)
	}
	password := os.Getenv("L4D2_RCON_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("%w: 服务端未配置 RCON 密码", ErrGameBanRCONUnavailable)
	}
	connection, err := rcon.Dial(url, password)
	if err != nil {
		return nil, fmt.Errorf("%w: RCON 连接失败: %v", ErrGameBanRCONUnavailable, err)
	}
	return connection, nil
}

func readGameBanList(session gameBanRCONSession) (GameBanList, error) {
	listIDResponse, err := executeGameBanRCON(session, "listid")
	if err != nil {
		return GameBanList{}, err
	}
	listIPResponse, err := executeGameBanRCON(session, "listip")
	if err != nil {
		return GameBanList{}, err
	}

	steamBans, err := parseGameBanIDList(listIDResponse)
	if err != nil {
		return GameBanList{}, err
	}
	ipBans, err := parseGameBanIPList(listIPResponse)
	if err != nil {
		return GameBanList{}, err
	}
	ready := IsGameBanPersistenceReady()
	warnings := make([]string, 0, 1)
	if !ready {
		warnings = append(warnings, "永久封禁加载配置尚未就绪，下一次修改时会自动修复。")
	}
	return GameBanList{
		SteamBans:        steamBans,
		IPBans:           ipBans,
		PersistenceReady: ready,
		Warnings:         warnings,
	}, nil
}

func executeGameBanRCON(session gameBanRCONSession, command string) (string, error) {
	response, err := session.Execute(command)
	if err != nil {
		return "", fmt.Errorf("%w: 执行 %s 失败: %v", ErrGameBanRCONUnavailable, command, err)
	}
	return response, nil
}

func parseGameBanIDList(response string) ([]GameBanEntry, error) {
	expected, _, err := parseGameBanListHeader(response, listIDHeaderPattern, "SteamID")
	if err != nil {
		return nil, err
	}
	matches := listIDEntryPattern.FindAllStringSubmatch(response, -1)
	entries := make([]GameBanEntry, 0, len(matches))
	for _, match := range matches {
		steam2, steam64, normalizeErr := normalizeSteamID(match[1])
		if normalizeErr != nil {
			return nil, fmt.Errorf("%w: 无法解析游戏服务器返回的 SteamID", ErrGameBanRCONResponse)
		}
		entry, durationErr := newGameBanEntry(GameBanKindSteamID, steam2, match[2])
		if durationErr != nil {
			return nil, durationErr
		}
		entry.SteamID64 = steam64
		entries = append(entries, entry)
	}
	if len(entries) != expected {
		return nil, fmt.Errorf("%w: SteamID 列表声明 %d 条，实际解析 %d 条", ErrGameBanRCONResponse, expected, len(entries))
	}
	return entries, nil
}

func parseGameBanIPList(response string) ([]GameBanEntry, error) {
	expected, _, err := parseGameBanListHeader(response, listIPHeaderPattern, "IP")
	if err != nil {
		return nil, err
	}
	matches := listIPEntryPattern.FindAllStringSubmatch(response, -1)
	entries := make([]GameBanEntry, 0, len(matches))
	for _, match := range matches {
		rawIP := strings.Join(strings.Fields(match[1]), "")
		ip, normalizeErr := NormalizeGameBanValue(GameBanKindIP, rawIP)
		if normalizeErr != nil {
			return nil, fmt.Errorf("%w: 无法解析游戏服务器返回的 IP", ErrGameBanRCONResponse)
		}
		entry, durationErr := newGameBanEntry(GameBanKindIP, ip, match[2])
		if durationErr != nil {
			return nil, durationErr
		}
		entries = append(entries, entry)
	}
	if len(entries) != expected {
		return nil, fmt.Errorf("%w: IP 列表声明 %d 条，实际解析 %d 条", ErrGameBanRCONResponse, expected, len(entries))
	}
	return entries, nil
}

func parseGameBanListHeader(response string, pattern *regexp.Regexp, label string) (int, bool, error) {
	match := pattern.FindStringSubmatch(response)
	if len(match) != 2 {
		return 0, false, fmt.Errorf("%w: 未找到 %s 封禁列表头", ErrGameBanRCONResponse, label)
	}
	value := strings.ToLower(strings.TrimSpace(match[1]))
	if value == "empty" {
		return 0, true, nil
	}
	countText := strings.Fields(value)[0]
	count, err := strconv.Atoi(countText)
	if err != nil {
		return 0, false, fmt.Errorf("%w: %s 封禁数量无效", ErrGameBanRCONResponse, label)
	}
	return count, count == 0, nil
}

func newGameBanEntry(kind, value, duration string) (GameBanEntry, error) {
	entry := GameBanEntry{Kind: kind, Value: value}
	if strings.EqualFold(strings.TrimSpace(duration), "permanent") {
		entry.Permanent = true
		return entry, nil
	}
	match := minutesPattern.FindStringSubmatch(strings.TrimSpace(duration))
	if len(match) != 2 {
		return GameBanEntry{}, fmt.Errorf("%w: 无效的封禁时长", ErrGameBanRCONResponse)
	}
	minutes, err := strconv.ParseFloat(match[1], 64)
	if err != nil || minutes < 0 {
		return GameBanEntry{}, fmt.Errorf("%w: 无效的封禁时长", ErrGameBanRCONResponse)
	}
	entry.RemainingMinutes = &minutes
	return entry, nil
}

func gameBanContains(list GameBanList, kind, value string) bool {
	_, found := findGameBan(list, kind, value)
	return found
}

func findGameBan(list GameBanList, kind, value string) (GameBanEntry, bool) {
	entries := list.SteamBans
	if kind == GameBanKindIP {
		entries = list.IPBans
	}
	for _, entry := range entries {
		if entry.Value == value {
			return entry, true
		}
	}
	return GameBanEntry{}, false
}

func commandResponseLooksFailed(response string) bool {
	lower := strings.ToLower(response)
	for _, marker := range []string{"unknown command", "usage:", "invalid", "couldn't", "cannot", "failed", "error"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
