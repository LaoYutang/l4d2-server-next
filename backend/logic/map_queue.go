package logic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

const mapQueueListSchema = 1

type MapQueueRunState string

const (
	MapQueueStateStopped MapQueueRunState = "stopped"
	MapQueueStateArmed   MapQueueRunState = "armed"
	MapQueueStateRunning MapQueueRunState = "running"
	MapQueueStateDelay   MapQueueRunState = "delay"
	MapQueueStatePaused  MapQueueRunState = "paused"
)

type MapQueueItem struct {
	Index       int    `json:"index"`
	Active      bool   `json:"active"`
	Map         string `json:"map"`
	Mission     string `json:"mission"`
	MissionName string `json:"mission_name"`
	ChapterName string `json:"chapter_name"`
	Official    bool   `json:"official"`
}

type MapQueueSnapshot struct {
	Installed bool              `json:"installed"`
	Enabled   bool              `json:"enabled"`
	Supported bool              `json:"supported"`
	State     *MapQueueRunState `json:"state"`
	Active    *MapQueueItem     `json:"active"`
	Pending   []MapQueueItem    `json:"pending"`
	Schema    int               `json:"schema"`
}

type MapQueueActionResponse struct {
	OK                bool              `json:"ok"`
	Message           string            `json:"message"`
	MapChangeExpected bool              `json:"map_change_expected"`
	Snapshot          *MapQueueSnapshot `json:"snapshot,omitempty"`
	RefreshError      string            `json:"refresh_error,omitempty"`
}

type MapQueueActionKind string

const (
	MapQueueActionAddFront MapQueueActionKind = "add_front"
	MapQueueActionAddBack  MapQueueActionKind = "add_back"
	MapQueueActionRemove   MapQueueActionKind = "remove"
	MapQueueActionRun      MapQueueActionKind = "run"
	MapQueueActionRunAfter MapQueueActionKind = "run_after"
	MapQueueActionPause    MapQueueActionKind = "pause"
	MapQueueActionSkip     MapQueueActionKind = "skip"
	MapQueueActionClear    MapQueueActionKind = "clear"
)

type MapQueueErrorKind string

const (
	MapQueueErrorInvalid  MapQueueErrorKind = "invalid"
	MapQueueErrorRCON     MapQueueErrorKind = "rcon"
	MapQueueErrorProtocol MapQueueErrorKind = "protocol"
	MapQueueErrorRejected MapQueueErrorKind = "rejected"
)

type MapQueueError struct {
	Kind    MapQueueErrorKind
	Message string
	Cause   error
}

func (e *MapQueueError) Error() string {
	return e.Message
}

func (e *MapQueueError) Unwrap() error {
	return e.Cause
}

func GetMapQueueErrorKind(err error) (MapQueueErrorKind, bool) {
	var queueErr *MapQueueError
	if !errors.As(err, &queueErr) {
		return "", false
	}
	return queueErr.Kind, true
}

var (
	mapQueueMapTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	mapQueueStatusPattern   = regexp.MustCompile(`^\[MapQueue\] enabled=([01]) supported=([01]) state=(stopped|armed|running|delay|paused) active=(-|[A-Za-z0-9_.-]+) pending=([0-9]+)$`)
	dialMapQueueRCON        = openRconSession
)

type mapQueueStatus struct {
	Enabled   bool
	Supported bool
	State     MapQueueRunState
	Active    string
	Pending   int
}

type mapQueueMachineRecord struct {
	Record       *string `json:"record"`
	Schema       *int    `json:"schema"`
	State        *string `json:"state"`
	ActiveCount  *int    `json:"active_count"`
	PendingCount *int    `json:"pending_count"`
	ItemCount    *int    `json:"item_count"`
	Index        *int    `json:"index"`
	Active       *bool   `json:"active"`
	Map          *string `json:"map"`
	Mission      *string `json:"mission"`
	MissionName  *string `json:"mission_name"`
	ChapterName  *string `json:"chapter_name"`
	Official     *bool   `json:"official"`
}

type mapQueueList struct {
	State        MapQueueRunState
	ActiveCount  int
	PendingCount int
	Items        []MapQueueItem
}

type mapQueueSnapshotMismatch struct {
	message string
}

func (e *mapQueueSnapshotMismatch) Error() string {
	return e.message
}

func GetMapQueueSnapshot() (MapQueueSnapshot, error) {
	conn, err := dialMapQueueRCON()
	if err != nil {
		return MapQueueSnapshot{}, mapQueueRCONError("RCON连接失败", err)
	}
	defer conn.Close()

	return getMapQueueSnapshotWithSession(conn)
}

func ExecuteMapQueueAction(kind MapQueueActionKind, mapName string) (MapQueueActionResponse, error) {
	command, err := buildMapQueueCommand(kind, mapName)
	if err != nil {
		return MapQueueActionResponse{}, err
	}

	conn, err := dialMapQueueRCON()
	if err != nil {
		return MapQueueActionResponse{}, mapQueueRCONError("RCON连接失败", err)
	}
	defer conn.Close()

	raw, err := conn.Execute(command)
	if err != nil {
		return MapQueueActionResponse{}, mapQueueRCONError("RCON命令执行失败", err)
	}
	if isUnknownMapQueueCommand(raw) {
		return MapQueueActionResponse{}, mapQueueRejectedError("未安装地图待办队列插件")
	}

	reply, message, validReply := parseMapQueueActionReply(raw)
	if !validReply || !isMapQueueActionSuccess(kind, mapName, message) {
		if validReply {
			message = reply
		} else {
			message = strings.TrimSpace(raw)
		}
		if message == "" {
			message = "地图待办队列插件未返回有效结果"
		}
		return MapQueueActionResponse{}, mapQueueRejectedError(message)
	}

	response := MapQueueActionResponse{
		OK:      true,
		Message: message,
	}
	if (kind == MapQueueActionRun && strings.Contains(message, "正在切换至")) ||
		(kind == MapQueueActionSkip && strings.Contains(message, "正在切换至")) {
		response.MapChangeExpected = true
		return response, nil
	}

	snapshot, refreshErr := getMapQueueSnapshotWithSession(conn)
	if refreshErr != nil {
		response.RefreshError = refreshErr.Error()
		return response, nil
	}
	response.Snapshot = &snapshot
	return response, nil
}

func ValidateMapQueueMapToken(mapName string) error {
	if mapName == "" {
		return mapQueueInvalidError("地图代码不能为空")
	}
	if len([]byte(mapName)) > 127 {
		return mapQueueInvalidError("地图代码不能超过 127 字节")
	}
	if !mapQueueMapTokenPattern.MatchString(mapName) {
		return mapQueueInvalidError("地图代码只能包含字母、数字、下划线、点和连字符")
	}
	return nil
}

func buildMapQueueCommand(kind MapQueueActionKind, mapName string) (string, error) {
	switch kind {
	case MapQueueActionAddFront:
		if err := ValidateMapQueueMapToken(mapName); err != nil {
			return "", err
		}
		return "sm_mq addfront " + mapName, nil
	case MapQueueActionAddBack:
		if err := ValidateMapQueueMapToken(mapName); err != nil {
			return "", err
		}
		return "sm_mq add " + mapName, nil
	case MapQueueActionRemove:
		if err := ValidateMapQueueMapToken(mapName); err != nil {
			return "", err
		}
		return "sm_mq remove " + mapName, nil
	case MapQueueActionRun:
		return "sm_mq run", nil
	case MapQueueActionRunAfter:
		return "sm_mq runafter", nil
	case MapQueueActionPause:
		return "sm_mq pause", nil
	case MapQueueActionSkip:
		return "sm_mq skip", nil
	case MapQueueActionClear:
		return "sm_mq clear", nil
	default:
		return "", mapQueueInvalidError("未知的地图待办队列操作")
	}
}

func getMapQueueSnapshotWithSession(conn rconSession) (MapQueueSnapshot, error) {
	for attempt := 0; attempt < 2; attempt++ {
		snapshot, err := readMapQueueSnapshot(conn)
		if err == nil {
			return snapshot, nil
		}
		var mismatch *mapQueueSnapshotMismatch
		if !errors.As(err, &mismatch) || attempt == 1 {
			if errors.As(err, &mismatch) {
				return MapQueueSnapshot{}, mapQueueProtocolError("地图待办队列状态在读取期间持续变化，请手动刷新")
			}
			return MapQueueSnapshot{}, err
		}
	}
	return MapQueueSnapshot{}, mapQueueProtocolError("无法读取地图待办队列状态")
}

func readMapQueueSnapshot(conn rconSession) (MapQueueSnapshot, error) {
	statusRaw, err := conn.Execute("sm_mq status")
	if err != nil {
		return MapQueueSnapshot{}, mapQueueRCONError("RCON命令执行失败", err)
	}
	listRaw, err := conn.Execute("sm_mq list")
	if err != nil {
		return MapQueueSnapshot{}, mapQueueRCONError("RCON命令执行失败", err)
	}

	if isUnknownMapQueueCommand(statusRaw) || isUnknownMapQueueCommand(listRaw) {
		return MapQueueSnapshot{
			Installed: false,
			Pending:   []MapQueueItem{},
		}, nil
	}

	status, err := parseMapQueueStatus(statusRaw)
	if err != nil {
		return MapQueueSnapshot{}, err
	}
	list, err := parseMapQueueList(listRaw)
	if err != nil {
		return MapQueueSnapshot{}, err
	}

	if status.State != list.State || status.Pending != list.PendingCount {
		return MapQueueSnapshot{}, &mapQueueSnapshotMismatch{message: "status 与 list 数量或状态不一致"}
	}
	if (status.Active == "-") != (list.ActiveCount == 0) {
		return MapQueueSnapshot{}, &mapQueueSnapshotMismatch{message: "status 与 list 的执行中地图不一致"}
	}

	pending := make([]MapQueueItem, 0, list.PendingCount)
	var active *MapQueueItem
	for i := range list.Items {
		item := list.Items[i]
		if item.Active {
			itemCopy := item
			active = &itemCopy
			continue
		}
		pending = append(pending, item)
	}
	if active != nil && status.Active != active.Map {
		return MapQueueSnapshot{}, &mapQueueSnapshotMismatch{message: "status 与 list 的执行中地图代码不一致"}
	}

	state := status.State
	return MapQueueSnapshot{
		Installed: true,
		Enabled:   status.Enabled,
		Supported: status.Supported,
		State:     &state,
		Active:    active,
		Pending:   pending,
		Schema:    mapQueueListSchema,
	}, nil
}

func parseMapQueueStatus(raw string) (mapQueueStatus, error) {
	var matched []string
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		match := mapQueueStatusPattern.FindStringSubmatch(line)
		if match == nil || matched != nil {
			return mapQueueStatus{}, mapQueueProtocolError("地图待办队列插件协议不兼容，请更新插件")
		}
		matched = match
	}
	if err := scanner.Err(); err != nil {
		return mapQueueStatus{}, mapQueueProtocolError("无法读取地图待办队列状态")
	}
	if matched == nil {
		return mapQueueStatus{}, mapQueueProtocolError("地图待办队列插件协议不兼容，请更新插件")
	}

	pending, err := strconv.Atoi(matched[5])
	if err != nil {
		return mapQueueStatus{}, mapQueueProtocolError("地图待办队列状态数量无效")
	}
	if matched[4] != "-" && len([]byte(matched[4])) > 127 {
		return mapQueueStatus{}, mapQueueProtocolError("地图待办队列状态中的地图代码无效")
	}
	return mapQueueStatus{
		Enabled:   matched[1] == "1",
		Supported: matched[2] == "1",
		State:     MapQueueRunState(matched[3]),
		Active:    matched[4],
		Pending:   pending,
	}, nil
}

func parseMapQueueList(raw string) (mapQueueList, error) {
	records := make([]mapQueueMachineRecord, 0)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "MQ_LIST ") {
			return mapQueueList{}, mapQueueProtocolError("地图待办队列插件协议不兼容，请更新插件")
		}
		var record mapQueueMachineRecord
		if err := decodeStrictJSON([]byte(strings.TrimPrefix(line, "MQ_LIST ")), &record); err != nil {
			return mapQueueList{}, mapQueueProtocolError("地图待办队列列表协议损坏")
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return mapQueueList{}, mapQueueProtocolError("无法读取地图待办队列列表")
	}
	if len(records) < 2 {
		return mapQueueList{}, mapQueueProtocolError("地图待办队列插件协议不兼容，请更新插件")
	}

	begin := records[0]
	end := records[len(records)-1]
	if begin.Record == nil || *begin.Record != "begin" || end.Record == nil || *end.Record != "end" {
		return mapQueueList{}, mapQueueProtocolError("地图待办队列列表缺少 begin 或 end 记录")
	}
	if !validMapQueueSchema(begin.Schema) || !validMapQueueSchema(end.Schema) {
		return mapQueueList{}, mapQueueProtocolError("地图待办队列插件协议不兼容，请更新插件")
	}
	if begin.State == nil || begin.ActiveCount == nil || begin.PendingCount == nil || begin.ItemCount == nil || end.ItemCount == nil {
		return mapQueueList{}, mapQueueProtocolError("地图待办队列列表记录字段不完整")
	}
	if begin.Index != nil || begin.Active != nil || begin.Map != nil || begin.Mission != nil || begin.MissionName != nil || begin.ChapterName != nil || begin.Official != nil {
		return mapQueueList{}, mapQueueProtocolError("地图待办队列 begin 记录包含多余字段")
	}
	if end.State != nil || end.ActiveCount != nil || end.PendingCount != nil || end.Index != nil || end.Active != nil || end.Map != nil || end.Mission != nil || end.MissionName != nil || end.ChapterName != nil || end.Official != nil {
		return mapQueueList{}, mapQueueProtocolError("地图待办队列 end 记录包含多余字段")
	}
	state := MapQueueRunState(*begin.State)
	if !isValidMapQueueState(state) || *begin.ActiveCount < 0 || *begin.ActiveCount > 1 || *begin.PendingCount < 0 || *begin.ItemCount < 0 {
		return mapQueueList{}, mapQueueProtocolError("地图待办队列列表记录字段无效")
	}
	if *begin.ItemCount != *begin.ActiveCount+*begin.PendingCount {
		return mapQueueList{}, mapQueueProtocolError("地图待办队列列表数量不匹配")
	}

	items := make([]MapQueueItem, 0, len(records)-2)
	activeCount := 0
	for index, record := range records[1 : len(records)-1] {
		if record.Record == nil || *record.Record != "item" || !validMapQueueSchema(record.Schema) {
			return mapQueueList{}, mapQueueProtocolError("地图待办队列列表包含无效 item 记录")
		}
		if record.Index == nil || record.Active == nil || record.Map == nil || record.Mission == nil || record.MissionName == nil || record.ChapterName == nil || record.Official == nil {
			return mapQueueList{}, mapQueueProtocolError("地图待办队列 item 字段不完整")
		}
		if record.State != nil || record.ActiveCount != nil || record.PendingCount != nil || record.ItemCount != nil {
			return mapQueueList{}, mapQueueProtocolError("地图待办队列 item 记录包含多余字段")
		}
		if *record.Index != index || len([]byte(*record.Map)) > 127 || !mapQueueMapTokenPattern.MatchString(*record.Map) {
			return mapQueueList{}, mapQueueProtocolError("地图待办队列 item 的 index 或地图代码无效")
		}
		if *record.Active {
			activeCount++
			if index != 0 {
				return mapQueueList{}, mapQueueProtocolError("执行中地图必须位于列表首项")
			}
		}
		items = append(items, MapQueueItem{
			Index:       *record.Index,
			Active:      *record.Active,
			Map:         *record.Map,
			Mission:     *record.Mission,
			MissionName: *record.MissionName,
			ChapterName: *record.ChapterName,
			Official:    *record.Official,
		})
	}
	if len(items) != *begin.ItemCount || len(items) != *end.ItemCount || activeCount != *begin.ActiveCount || len(items)-activeCount != *begin.PendingCount {
		return mapQueueList{}, mapQueueProtocolError("地图待办队列列表数量不匹配")
	}

	return mapQueueList{
		State:        state,
		ActiveCount:  *begin.ActiveCount,
		PendingCount: *begin.PendingCount,
		Items:        items,
	}, nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("JSON 记录包含多余内容")
	}
	return nil
}

func validMapQueueSchema(schema *int) bool {
	return schema != nil && *schema == mapQueueListSchema
}

func isValidMapQueueState(state MapQueueRunState) bool {
	switch state {
	case MapQueueStateStopped, MapQueueStateArmed, MapQueueStateRunning, MapQueueStateDelay, MapQueueStatePaused:
		return true
	default:
		return false
	}
}

func isUnknownMapQueueCommand(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, "unknown command") && strings.Contains(lower, "sm_mq")
}

func parseMapQueueActionReply(raw string) (string, string, bool) {
	var reply string
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if reply != "" || !strings.HasPrefix(line, "[MapQueue]") {
			return "", "", false
		}
		reply = line
	}
	if reply == "" {
		return "", "", false
	}
	return reply, strings.TrimSpace(strings.TrimPrefix(reply, "[MapQueue]")), true
}

func isMapQueueActionSuccess(kind MapQueueActionKind, mapName, message string) bool {
	switch kind {
	case MapQueueActionAddFront:
		return message == "已添加 1 个地图到队首。"
	case MapQueueActionAddBack:
		return message == "已添加 1 个地图到队尾。"
	case MapQueueActionRemove:
		pattern := regexp.MustCompile(`^已删除地图 ` + regexp.QuoteMeta(mapName) + ` 的全部 [1-9][0-9]* 个待执行项。$`)
		return pattern.MatchString(message)
	case MapQueueActionRun:
		return message == "队列已恢复，当前战役通关后将继续下一项。" ||
			regexp.MustCompile(`^队列已启动，正在切换至 [A-Za-z0-9_.-]+。$`).MatchString(message)
	case MapQueueActionRunAfter:
		return message == "队列已等待，将在当前战役通关后执行。"
	case MapQueueActionPause:
		switch message {
		case "队列已暂停；尚未进入的当前项已放回队首。",
			"队列已暂停；当前战役继续，通关后不会进入下一项。",
			"队列已暂停；后续待办已保留。":
			return true
		default:
			return false
		}
	case MapQueueActionSkip:
		return regexp.MustCompile(`^已跳过 [A-Za-z0-9_.-]+(?:，正在切换至 [A-Za-z0-9_.-]+|；下一项无效，队列已停止并保留该项)?。$`).MatchString(message)
	case MapQueueActionClear:
		return message == "已清空地图待办并停止执行。"
	default:
		return false
	}
}

func mapQueueInvalidError(message string) error {
	return &MapQueueError{Kind: MapQueueErrorInvalid, Message: message}
}

func mapQueueRCONError(message string, cause error) error {
	if cause == nil {
		return &MapQueueError{Kind: MapQueueErrorRCON, Message: message}
	}
	return &MapQueueError{Kind: MapQueueErrorRCON, Message: fmt.Sprintf("%s: %v", message, cause), Cause: cause}
}

func mapQueueProtocolError(message string) error {
	return &MapQueueError{Kind: MapQueueErrorProtocol, Message: message}
}

func mapQueueRejectedError(message string) error {
	return &MapQueueError{Kind: MapQueueErrorRejected, Message: message}
}
