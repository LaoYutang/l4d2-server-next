package logic

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"l4d2-manager-next/consts"
)

type scriptedGameBanRCON struct {
	mu        sync.Mutex
	responses map[string][]string
	errors    map[string]error
	commands  []string
}

func (fake *scriptedGameBanRCON) Execute(command string) (string, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.commands = append(fake.commands, command)
	if err := fake.errors[command]; err != nil {
		return "", err
	}
	queue := fake.responses[command]
	if len(queue) == 0 {
		return "", nil
	}
	fake.responses[command] = queue[1:]
	return queue[0], nil
}

func (fake *scriptedGameBanRCON) Close() error { return nil }

type blockingGameBanRCON struct {
	*scriptedGameBanRCON
	blockCommand string
	started      chan struct{}
	release      chan struct{}
	startOnce    sync.Once
}

func (fake *blockingGameBanRCON) Execute(command string) (string, error) {
	if command == fake.blockCommand {
		fake.mu.Lock()
		fake.commands = append(fake.commands, command)
		fake.mu.Unlock()
		fake.startOnce.Do(func() { close(fake.started) })
		<-fake.release
		return "", nil
	}
	return fake.scriptedGameBanRCON.Execute(command)
}

func setupGameBanLogicTest(t *testing.T) string {
	t.Helper()
	oldGamePath := consts.GamePath
	oldDialer := dialGameBanRCON
	gamePath := t.TempDir()
	cfgDir := filepath.Join(gamePath, "cfg")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "server.cfg"), []byte("hostname test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	consts.GamePath = gamePath
	t.Cleanup(func() {
		consts.GamePath = oldGamePath
		dialGameBanRCON = oldDialer
	})
	return gamePath
}

func TestNormalizeGameBanValue(t *testing.T) {
	tests := []struct {
		name  string
		kind  string
		input string
		want  string
	}{
		{name: "Steam2", kind: GameBanKindSteamID, input: "STEAM_0:1:123", want: "STEAM_1:1:123"},
		{name: "Steam3", kind: GameBanKindSteamID, input: "[U:1:247]", want: "STEAM_1:1:123"},
		{name: "Steam64", kind: GameBanKindSteamID, input: "76561197960265975", want: "STEAM_1:1:123"},
		{name: "IPv4", kind: GameBanKindIP, input: "203.0.113.10", want: "203.0.113.10"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeGameBanValue(test.kind, test.input)
			if err != nil || got != test.want {
				t.Fatalf("NormalizeGameBanValue() = %q, %v; want %q", got, err, test.want)
			}
		})
	}

	invalid := []struct {
		kind  string
		value string
	}{
		{GameBanKindSteamID, "STEAM_1:1:123;quit"},
		{GameBanKindSteamID, "[U:2:247]"},
		{GameBanKindIP, "203.0.113.0/24"},
		{GameBanKindIP, "2001:db8::1"},
		{GameBanKindIP, "203.0.113.10:27015"},
	}
	for _, test := range invalid {
		if _, err := NormalizeGameBanValue(test.kind, test.value); !errors.Is(err, ErrGameBanInvalidInput) {
			t.Errorf("NormalizeGameBanValue(%q) error = %v, want invalid input", test.value, err)
		}
	}
}

func TestParseGameBanLists(t *testing.T) {
	steam, err := parseGameBanIDList("ID filter list: 2 entries\n1 STEAM_0:1:123 : permanent\n2 [U:1:400] : 30.000 min\n")
	if err != nil {
		t.Fatalf("parse SteamID list: %v", err)
	}
	if len(steam) != 2 || steam[0].Value != "STEAM_1:1:123" || !steam[0].Permanent {
		t.Fatalf("unexpected SteamID entries: %#v", steam)
	}
	if steam[1].Value != "STEAM_1:0:200" || steam[1].RemainingMinutes == nil || *steam[1].RemainingMinutes != 30 {
		t.Fatalf("unexpected timed SteamID entry: %#v", steam[1])
	}

	ips, err := parseGameBanIPList("IP filter list: 2 entries\n1  60. 85. 66.  4 : 5.000 min\n2  60. 85. 66. 44 : permanent\n")
	if err != nil {
		t.Fatalf("parse IP list: %v", err)
	}
	if len(ips) != 2 || ips[0].Value != "60.85.66.4" || ips[0].RemainingMinutes == nil || ips[1].Value != "60.85.66.44" || !ips[1].Permanent {
		t.Fatalf("unexpected IP entries: %#v", ips)
	}

	emptySteam, err := parseGameBanIDList("ID filter list: empty")
	if err != nil || len(emptySteam) != 0 {
		t.Fatalf("empty SteamID list = %#v, %v", emptySteam, err)
	}
	emptyIP, err := parseGameBanIPList("IP filter list: empty")
	if err != nil || len(emptyIP) != 0 {
		t.Fatalf("empty IP list = %#v, %v", emptyIP, err)
	}
}

func TestParseGameBanListRejectsCountMismatch(t *testing.T) {
	_, err := parseGameBanIDList("ID filter list: 2 entries\n1 STEAM_1:0:123 : permanent\n")
	if !errors.Is(err, ErrGameBanRCONResponse) {
		t.Fatalf("error = %v, want RCON response error", err)
	}
	_, err = parseGameBanIPList("this is not a list")
	if !errors.Is(err, ErrGameBanRCONResponse) {
		t.Fatalf("error = %v, want RCON response error", err)
	}
}

func TestAddPermanentSteamBanUsesKickPersistsAndVerifies(t *testing.T) {
	setupGameBanLogicTest(t)
	fake := &scriptedGameBanRCON{responses: map[string][]string{
		"listid": {"ID filter list: empty", "ID filter list: 1 entry\n1 STEAM_1:1:123 : permanent\n"},
		"listip": {"IP filter list: empty", "IP filter list: empty"},
	}, errors: map[string]error{}}
	dialGameBanRCON = func() (gameBanRCONSession, error) { return fake, nil }

	result, err := AddGameBan(GameBanChange{
		Kind:            GameBanKindSteamID,
		Value:           "[U:1:247]",
		DurationMinutes: 0,
	})
	if err != nil {
		t.Fatalf("AddGameBan: %v", err)
	}
	if len(result.SteamBans) != 1 || result.SteamBans[0].Value != "STEAM_1:1:123" {
		t.Fatalf("unexpected result: %#v", result)
	}
	wantCommands := []string{
		"listid", "listip",
		"banid 0 STEAM_1:1:123 kick", "writeid",
		"listid", "listip",
	}
	if !reflect.DeepEqual(fake.commands, wantCommands) {
		t.Fatalf("commands = %#v, want %#v", fake.commands, wantCommands)
	}
	if !IsGameBanPersistenceReady() {
		t.Fatal("persistence config was not prepared")
	}
}

func TestAddTimedSteamBanDoesNotWritePermanentList(t *testing.T) {
	setupGameBanLogicTest(t)
	fake := &scriptedGameBanRCON{responses: map[string][]string{
		"listid": {"ID filter list: empty", "ID filter list: 1 entry\n1 STEAM_1:0:200 : 15.000 min\n"},
		"listip": {"IP filter list: empty", "IP filter list: empty"},
	}, errors: map[string]error{}}
	dialGameBanRCON = func() (gameBanRCONSession, error) { return fake, nil }

	_, err := AddGameBan(GameBanChange{Kind: GameBanKindSteamID, Value: "[U:1:400]", DurationMinutes: 15})
	if err != nil {
		t.Fatalf("AddGameBan: %v", err)
	}
	for _, command := range fake.commands {
		if command == "writeid" {
			t.Fatal("timed ban unexpectedly called writeid")
		}
	}
	if !containsGameBanCommand(fake.commands, "banid 15 STEAM_1:0:200 kick") {
		t.Fatalf("commands do not contain timed kick ban: %#v", fake.commands)
	}
}

func TestAddIPBanAlwaysUsesBlacklistCommands(t *testing.T) {
	setupGameBanLogicTest(t)
	fake := &scriptedGameBanRCON{responses: map[string][]string{
		"listid": {"ID filter list: empty", "ID filter list: empty"},
		"listip": {"IP filter list: empty", "IP filter list: 1 entry\n1 203.0.113.10 : permanent\n"},
	}, errors: map[string]error{}}
	dialGameBanRCON = func() (gameBanRCONSession, error) { return fake, nil }

	result, err := AddGameBan(GameBanChange{Kind: GameBanKindIP, Value: "203.0.113.10"})
	if err != nil {
		t.Fatalf("AddGameBan: %v", err)
	}
	if len(result.IPBans) != 1 || result.IPBans[0].Value != "203.0.113.10" {
		t.Fatalf("unexpected result: %#v", result.IPBans)
	}
	if !containsGameBanCommand(fake.commands, "addip 0 203.0.113.10") || !containsGameBanCommand(fake.commands, "writeip") {
		t.Fatalf("commands = %#v", fake.commands)
	}
}

func TestRemoveIPBanPersistsAndVerifies(t *testing.T) {
	setupGameBanLogicTest(t)
	fake := &scriptedGameBanRCON{responses: map[string][]string{
		"listid": {"ID filter list: empty", "ID filter list: empty"},
		"listip": {"IP filter list: 1 entry\n1 203.0.113.10 : permanent\n", "IP filter list: empty"},
	}, errors: map[string]error{}}
	dialGameBanRCON = func() (gameBanRCONSession, error) { return fake, nil }

	result, err := RemoveGameBan(GameBanKindIP, "203.0.113.10")
	if err != nil {
		t.Fatalf("RemoveGameBan: %v", err)
	}
	if len(result.IPBans) != 0 {
		t.Fatalf("IP ban still present: %#v", result.IPBans)
	}
	if !containsGameBanCommand(fake.commands, "removeip 203.0.113.10") || !containsGameBanCommand(fake.commands, "writeip") {
		t.Fatalf("remove commands = %#v", fake.commands)
	}
}

func TestGameBanMutationsAreSerialized(t *testing.T) {
	setupGameBanLogicTest(t)
	releaseFirst := make(chan struct{})
	first := &blockingGameBanRCON{
		scriptedGameBanRCON: &scriptedGameBanRCON{responses: map[string][]string{
			"listid": {"ID filter list: empty", "ID filter list: 1 entry\n1 STEAM_1:1:123 : permanent\n"},
			"listip": {"IP filter list: empty", "IP filter list: empty"},
		}, errors: map[string]error{}},
		blockCommand: "banid 0 STEAM_1:1:123 kick",
		started:      make(chan struct{}),
		release:      releaseFirst,
	}
	second := &scriptedGameBanRCON{responses: map[string][]string{
		"listid": {"ID filter list: empty", "ID filter list: 1 entry\n1 STEAM_1:0:200 : permanent\n"},
		"listip": {"IP filter list: empty", "IP filter list: empty"},
	}, errors: map[string]error{}}

	var dialCount atomic.Int32
	secondDialed := make(chan struct{})
	dialGameBanRCON = func() (gameBanRCONSession, error) {
		if dialCount.Add(1) == 1 {
			return first, nil
		}
		close(secondDialed)
		return second, nil
	}

	firstDone := make(chan error, 1)
	go func() {
		_, err := AddGameBan(GameBanChange{Kind: GameBanKindSteamID, Value: "[U:1:247]"})
		firstDone <- err
	}()
	<-first.started

	secondDone := make(chan error, 1)
	go func() {
		_, err := AddGameBan(GameBanChange{Kind: GameBanKindSteamID, Value: "[U:1:400]"})
		secondDone <- err
	}()
	select {
	case <-secondDialed:
		t.Fatal("second mutation dialed RCON before the first mutation completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first mutation: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second mutation: %v", err)
	}
}

func TestListGameBansReportsRCONUnavailable(t *testing.T) {
	setupGameBanLogicTest(t)
	dialGameBanRCON = func() (gameBanRCONSession, error) {
		return nil, errors.Join(ErrGameBanRCONUnavailable, errors.New("dial failed"))
	}
	_, err := ListGameBans()
	if !errors.Is(err, ErrGameBanRCONUnavailable) {
		t.Fatalf("error = %v, want RCON unavailable", err)
	}
}

func TestEnsureGameBanPersistenceConfigIsHiddenAndIdempotent(t *testing.T) {
	gamePath := setupGameBanLogicTest(t)
	cfgDir := filepath.Join(gamePath, "cfg")
	content := strings.Join([]string{
		"hostname test",
		"exec banned_user.cfg",
		ServerCustomConfigMarker,
		"exec \"banned_ip.cfg\"",
		"sm_cvar custom_value 1",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(cfgDir, "server.cfg"), []byte(content), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "server.cfg.60tick"), []byte("hostname tick\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGameBanPersistenceConfig(); err != nil {
		t.Fatalf("EnsureGameBanPersistenceConfig: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(cfgDir, "server.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(first)
	if strings.Count(text, GameBanConfigMarker) != 1 || strings.Count(text, "exec banned_user.cfg") != 1 || strings.Count(text, "exec banned_ip.cfg") != 1 {
		t.Fatalf("managed block was not canonical:\n%s", text)
	}
	if strings.Index(text, GameBanConfigMarker) > strings.Index(text, ServerCustomConfigMarker) {
		t.Fatalf("managed block appears inside custom config:\n%s", text)
	}
	if !strings.Contains(text, "sm_cvar custom_value 1") {
		t.Fatalf("custom config was lost:\n%s", text)
	}

	if err := EnsureGameBanPersistenceConfig(); err != nil {
		t.Fatalf("second EnsureGameBanPersistenceConfig: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(cfgDir, "server.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("second ensure changed the file:\nfirst=%q\nsecond=%q", first, second)
	}
	if !IsGameBanPersistenceReady() {
		t.Fatal("persistence should be ready")
	}
}

func containsGameBanCommand(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
