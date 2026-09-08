package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type fakeMapQueueReply struct {
	output string
	err    error
}

type fakeMapQueueSession struct {
	replies  []fakeMapQueueReply
	commands []string
	closed   bool
}

func (s *fakeMapQueueSession) Execute(command string) (string, error) {
	s.commands = append(s.commands, command)
	if len(s.replies) == 0 {
		return "", errors.New("unexpected command: " + command)
	}
	reply := s.replies[0]
	s.replies = s.replies[1:]
	return reply.output, reply.err
}

func (s *fakeMapQueueSession) Close() error {
	s.closed = true
	return nil
}

func useFakeMapQueueSession(t *testing.T, session *fakeMapQueueSession) {
	t.Helper()
	original := dialMapQueueRCON
	dialMapQueueRCON = func() (rconSession, error) {
		return session, nil
	}
	t.Cleanup(func() {
		dialMapQueueRCON = original
	})
}

func queueStatus(enabled, supported bool, state MapQueueRunState, active string, pending int) string {
	return fmt.Sprintf(
		"[MapQueue] enabled=%d supported=%d state=%s active=%s pending=%d",
		boolInt(enabled), boolInt(supported), state, active, pending,
	)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func queueMachineList(t *testing.T, state MapQueueRunState, items []MapQueueItem) string {
	t.Helper()
	activeCount := 0
	for _, item := range items {
		if item.Active {
			activeCount++
		}
	}
	records := []map[string]any{{
		"record":        "begin",
		"schema":        1,
		"state":         state,
		"active_count":  activeCount,
		"pending_count": len(items) - activeCount,
		"item_count":    len(items),
	}}
	for _, item := range items {
		records = append(records, map[string]any{
			"record":       "item",
			"schema":       1,
			"index":        item.Index,
			"active":       item.Active,
			"map":          item.Map,
			"mission":      item.Mission,
			"mission_name": item.MissionName,
			"chapter_name": item.ChapterName,
			"official":     item.Official,
		})
	}
	records = append(records, map[string]any{
		"record":     "end",
		"schema":     1,
		"item_count": len(items),
	})

	lines := make([]string, 0, len(records))
	for _, record := range records {
		encoded, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, "MQ_LIST "+string(encoded))
	}
	return strings.Join(lines, "\n")
}

func assertMapQueueErrorKind(t *testing.T, err error, expected MapQueueErrorKind) {
	t.Helper()
	kind, ok := GetMapQueueErrorKind(err)
	if !ok || kind != expected {
		t.Fatalf("error = %v, kind = %q, want %q", err, kind, expected)
	}
}

func TestGetMapQueueSnapshotStates(t *testing.T) {
	tests := []struct {
		name      string
		state     MapQueueRunState
		enabled   bool
		supported bool
		items     []MapQueueItem
	}{
		{name: "stopped empty", state: MapQueueStateStopped, enabled: true, supported: true},
		{
			name:  "armed pending",
			state: MapQueueStateArmed, enabled: true, supported: true,
			items: []MapQueueItem{{Index: 0, Map: "c1m1_hotel", Mission: "L4D2C1", MissionName: "死亡中心", ChapterName: "旅馆", Official: true}},
		},
		{
			name:  "running",
			state: MapQueueStateRunning, enabled: true, supported: true,
			items: []MapQueueItem{
				{Index: 0, Active: true, Map: "c1m1_hotel", Mission: "L4D2C1", MissionName: "死亡中心", ChapterName: "旅馆", Official: true},
				{Index: 1, Map: "custom_map", Mission: "custom", MissionName: "测试 \"战役\"", ChapterName: "第一章\\入口"},
				{Index: 2, Map: "custom_map", Mission: "custom", MissionName: "测试 \"战役\"", ChapterName: "第一章\\入口"},
			},
		},
		{
			name:  "delay unsupported",
			state: MapQueueStateDelay, enabled: false, supported: false,
			items: []MapQueueItem{{Index: 0, Active: true, Map: "c2m1_highway", Mission: "L4D2C2", MissionName: "黑色狂欢节", ChapterName: "公路", Official: true}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			active := "-"
			pending := len(test.items)
			if len(test.items) > 0 && test.items[0].Active {
				active = test.items[0].Map
				pending--
			}
			session := &fakeMapQueueSession{replies: []fakeMapQueueReply{
				{output: queueStatus(test.enabled, test.supported, test.state, active, pending)},
				{output: queueMachineList(t, test.state, test.items)},
			}}
			useFakeMapQueueSession(t, session)

			snapshot, err := GetMapQueueSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			if !snapshot.Installed || snapshot.Enabled != test.enabled || snapshot.Supported != test.supported || snapshot.State == nil || *snapshot.State != test.state || snapshot.Schema != 1 {
				t.Fatalf("unexpected snapshot: %+v", snapshot)
			}
			if !reflect.DeepEqual(session.commands, []string{"sm_mq status", "sm_mq list"}) {
				t.Fatalf("commands = %#v", session.commands)
			}
			if active == "-" {
				if snapshot.Active != nil || len(snapshot.Pending) != len(test.items) {
					t.Fatalf("unexpected active/pending: %+v", snapshot)
				}
				if len(test.items) > 0 && !reflect.DeepEqual(snapshot.Pending, test.items) {
					t.Fatalf("pending = %#v, want %#v", snapshot.Pending, test.items)
				}
			} else {
				if snapshot.Active == nil || snapshot.Active.Map != active || len(snapshot.Pending) != len(test.items)-1 {
					t.Fatalf("unexpected active/pending: %+v", snapshot)
				}
				if !reflect.DeepEqual(*snapshot.Active, test.items[0]) || !reflect.DeepEqual(snapshot.Pending, test.items[1:]) {
					t.Fatalf("active/pending did not preserve machine fields: %+v", snapshot)
				}
			}
		})
	}
}

func TestGetMapQueueSnapshotUnknownCommand(t *testing.T) {
	session := &fakeMapQueueSession{replies: []fakeMapQueueReply{
		{output: `Unknown command "sm_mq"`},
		{output: `Unknown command "sm_mq"`},
	}}
	useFakeMapQueueSession(t, session)

	snapshot, err := GetMapQueueSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Installed || snapshot.Pending == nil || len(snapshot.Pending) != 0 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestGetMapQueueSnapshotLongQueue(t *testing.T) {
	items := make([]MapQueueItem, 100)
	for index := range items {
		items[index] = MapQueueItem{
			Index:       index,
			Map:         fmt.Sprintf("custom_map_%03d", index),
			Mission:     "长战役",
			MissionName: "很长的测试战役",
			ChapterName: fmt.Sprintf("第 %d 章", index+1),
		}
	}
	session := &fakeMapQueueSession{replies: []fakeMapQueueReply{
		{output: queueStatus(true, true, MapQueueStateStopped, "-", len(items))},
		{output: queueMachineList(t, MapQueueStateStopped, items)},
	}}
	useFakeMapQueueSession(t, session)

	snapshot, err := GetMapQueueSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pending) != 100 || snapshot.Pending[99].ChapterName != "第 100 章" {
		t.Fatalf("unexpected long queue: first=%+v last=%+v", snapshot.Pending[0], snapshot.Pending[len(snapshot.Pending)-1])
	}
}

func TestGetMapQueueSnapshotRCONFailure(t *testing.T) {
	original := dialMapQueueRCON
	dialMapQueueRCON = func() (rconSession, error) {
		return nil, errors.New("dial failed")
	}
	t.Cleanup(func() { dialMapQueueRCON = original })

	_, err := GetMapQueueSnapshot()
	assertMapQueueErrorKind(t, err, MapQueueErrorRCON)
}

func TestParseMapQueueListRejectsMalformedProtocols(t *testing.T) {
	begin := `MQ_LIST {"record":"begin","schema":1,"state":"stopped","active_count":0,"pending_count":1,"item_count":1}`
	validItem := `MQ_LIST {"record":"item","schema":1,"index":0,"active":false,"map":"c1m1_hotel","mission":"m","mission_name":"n","chapter_name":"c","official":true}`
	end := `MQ_LIST {"record":"end","schema":1,"item_count":1}`
	tests := []struct {
		name string
		raw  string
	}{
		{name: "human readable legacy", raw: "[MapQueue] 队列为空。"},
		{name: "missing begin", raw: validItem + "\n" + end},
		{name: "missing end", raw: begin + "\n" + validItem},
		{name: "unknown schema", raw: "MQ_LIST {\"record\":\"begin\",\"schema\":2,\"state\":\"stopped\",\"active_count\":0,\"pending_count\":0,\"item_count\":0}\nMQ_LIST {\"record\":\"end\",\"schema\":2,\"item_count\":0}"},
		{name: "count mismatch", raw: `MQ_LIST {"record":"begin","schema":1,"state":"stopped","active_count":0,"pending_count":2,"item_count":2}` + "\n" + validItem + "\n" + end},
		{name: "non-contiguous index", raw: begin + "\n" + strings.Replace(validItem, `"index":0`, `"index":1`, 1) + "\n" + end},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseMapQueueList(test.raw)
			assertMapQueueErrorKind(t, err, MapQueueErrorProtocol)
		})
	}
}

func TestMapQueueActionCommandsAndRefresh(t *testing.T) {
	tests := []struct {
		name    string
		kind    MapQueueActionKind
		mapName string
		command string
		output  string
	}{
		{name: "add front", kind: MapQueueActionAddFront, mapName: "c1m1_hotel", command: "sm_mq addfront c1m1_hotel", output: "[MapQueue] 已添加 1 个地图到队首。"},
		{name: "add back", kind: MapQueueActionAddBack, mapName: "c1m1_hotel", command: "sm_mq add c1m1_hotel", output: "[MapQueue] 已添加 1 个地图到队尾。"},
		{name: "remove", kind: MapQueueActionRemove, mapName: "c1m1_hotel", command: "sm_mq remove c1m1_hotel", output: "[MapQueue] 已删除地图 c1m1_hotel 的全部 2 个待执行项。"},
		{name: "run after", kind: MapQueueActionRunAfter, command: "sm_mq runafter", output: "[MapQueue] 队列已等待，将在当前战役通关后执行。"},
		{name: "clear", kind: MapQueueActionClear, command: "sm_mq clear", output: "[MapQueue] 已清空地图待办并停止执行。"},
		{name: "skip without map change", kind: MapQueueActionSkip, command: "sm_mq skip", output: "[MapQueue] 已跳过 c1m1_hotel。"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session := &fakeMapQueueSession{replies: []fakeMapQueueReply{
				{output: test.output},
				{output: queueStatus(true, true, MapQueueStateStopped, "-", 0)},
				{output: queueMachineList(t, MapQueueStateStopped, nil)},
			}}
			useFakeMapQueueSession(t, session)

			response, err := ExecuteMapQueueAction(test.kind, test.mapName)
			if err != nil {
				t.Fatal(err)
			}
			if !response.OK || response.MapChangeExpected || response.Snapshot == nil || response.RefreshError != "" {
				t.Fatalf("response = %+v", response)
			}
			expected := []string{test.command, "sm_mq status", "sm_mq list"}
			if !reflect.DeepEqual(session.commands, expected) {
				t.Fatalf("commands = %#v, want %#v", session.commands, expected)
			}
		})
	}
}

func TestMapQueueMapChangingActionsDoNotRefresh(t *testing.T) {
	tests := []struct {
		name    string
		kind    MapQueueActionKind
		command string
		output  string
	}{
		{name: "run", kind: MapQueueActionRun, command: "sm_mq run", output: "[MapQueue] 队列已启动，正在切换至 c1m1_hotel。"},
		{name: "skip", kind: MapQueueActionSkip, command: "sm_mq skip", output: "[MapQueue] 已跳过 c1m1_hotel，正在切换至 c2m1_highway。"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session := &fakeMapQueueSession{replies: []fakeMapQueueReply{{output: test.output}}}
			useFakeMapQueueSession(t, session)

			response, err := ExecuteMapQueueAction(test.kind, "")
			if err != nil {
				t.Fatal(err)
			}
			if !response.MapChangeExpected || response.Snapshot != nil || response.RefreshError != "" {
				t.Fatalf("response = %+v", response)
			}
			if !reflect.DeepEqual(session.commands, []string{test.command}) {
				t.Fatalf("commands = %#v", session.commands)
			}
		})
	}
}

func TestMapQueueActionRefreshFailureKeepsSuccess(t *testing.T) {
	session := &fakeMapQueueSession{replies: []fakeMapQueueReply{
		{output: "[MapQueue] 已清空地图待办并停止执行。"},
		{err: errors.New("server restarting")},
	}}
	useFakeMapQueueSession(t, session)

	response, err := ExecuteMapQueueAction(MapQueueActionClear, "")
	if err != nil {
		t.Fatal(err)
	}
	if !response.OK || response.RefreshError == "" || response.Snapshot != nil {
		t.Fatalf("response = %+v", response)
	}
}

func TestMapQueueActionRejectsUnsafeMapAndPluginFailure(t *testing.T) {
	unsafeMaps := []string{"c1m1_hotel;quit", "c1m1 hotel", "地图", strings.Repeat("a", 128)}
	for _, mapName := range unsafeMaps {
		_, err := ExecuteMapQueueAction(MapQueueActionAddBack, mapName)
		assertMapQueueErrorKind(t, err, MapQueueErrorInvalid)
	}

	session := &fakeMapQueueSession{replies: []fakeMapQueueReply{{output: "[MapQueue] 地图不存在。"}}}
	useFakeMapQueueSession(t, session)
	_, err := ExecuteMapQueueAction(MapQueueActionAddBack, "not_found")
	assertMapQueueErrorKind(t, err, MapQueueErrorRejected)
	if err.Error() != "[MapQueue] 地图不存在。" {
		t.Fatalf("error = %v", err)
	}
}

func TestParseMapQueueStatusIsStrict(t *testing.T) {
	tests := []string{
		"[MapQueue] enabled=true supported=1 state=stopped active=- pending=0",
		"prefix [MapQueue] enabled=1 supported=1 state=stopped active=- pending=0",
		"[MapQueue] enabled=1 supported=1 state=paused active=- pending=0",
		"[MapQueue] enabled=1 supported=1 state=stopped active=- pending=0\nunexpected",
	}
	for _, raw := range tests {
		_, err := parseMapQueueStatus(raw)
		assertMapQueueErrorKind(t, err, MapQueueErrorProtocol)
	}
}
